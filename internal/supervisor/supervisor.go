package supervisor

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/saintedlama/invincible/internal/config"
	"github.com/saintedlama/invincible/internal/ports"
)

type State int

const (
	StateStopped State = iota
	StateStarting
	StateProbing
	StateRunning
	StateCrashed
)

func (s State) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateProbing:
		return "probing"
	case StateRunning:
		return "running"
	case StateCrashed:
		return "crashed"
	default:
		return "stopped"
	}
}

type process struct {
	cfg          config.ProcessConfig
	assignedPort int // resolved free port

	mu          sync.Mutex
	state       State
	pid         int
	cmd         *exec.Cmd
	done        chan struct{} // closed by watch() after cmd.Wait() returns
	running     chan struct{} // closed when process reaches StateRunning
	logs        ringBuffer
	intentional bool
	restarts    int
	startedAt   time.Time
}

type ProcessStatus struct {
	Name      string
	State     string
	PID       int
	Cmd       string
	Cwd       string
	Port      int
	PortEnv   string
	Env       map[string]string
	DependsOn []string
	Restarts  int
	StartedAt time.Time
}

type Supervisor struct {
	mu         sync.RWMutex
	processes  map[string]*process
	order      []string            // insertion order from config, used for display
	dependents map[string][]string // reversed graph: name → processes that depend on it
}

func New(cfgs []config.ProcessConfig) *Supervisor {
	s := &Supervisor{
		processes:  make(map[string]*process),
		order:      make([]string, 0, len(cfgs)),
		dependents: make(map[string][]string, len(cfgs)),
	}
	for _, c := range cfgs {
		s.processes[c.Name] = &process{cfg: c}
		s.order = append(s.order, c.Name)
		if _, ok := s.dependents[c.Name]; !ok {
			s.dependents[c.Name] = nil
		}
	}
	for _, c := range cfgs {
		for _, dep := range c.DependsOn {
			s.dependents[dep] = append(s.dependents[dep], c.Name)
		}
	}
	return s
}

func (s *Supervisor) Start(name string) error {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return fmt.Errorf("unknown process: %s", name)
	}
	return s.startProcess(p)
}

func (s *Supervisor) startProcess(p *process) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StateRunning || p.state == StateStarting || p.state == StateProbing {
		return nil
	}
	p.intentional = false
	p.state = StateStarting
	s.logEvent(p, "starting...")

	// Find or re-verify port.
	if !p.cfg.NoPort {
		port, err := ports.FindFree(p.cfg.Port)
		if err != nil {
			p.state = StateCrashed
			return fmt.Errorf("process %q: %w", p.cfg.Name, err)
		}
		p.assignedPort = port
	}

	cmd := shellCommand(p.cfg.Cmd)
	setProcessGroupAttr(cmd)
	if p.cfg.Cwd != "" {
		cmd.Dir = p.cfg.Cwd
	}

	// Build env: parent + config env + own port + dependency ports.
	env := envFromParent()
	for k, v := range p.cfg.Env {
		env = append(env, k+"="+v)
	}
	if !p.cfg.NoPort && p.assignedPort > 0 {
		env = append(env, fmt.Sprintf("%s=%d", p.cfg.PortEnv, p.assignedPort))
	}

	// Collect dep process pointers while holding s.mu briefly.
	s.mu.RLock()
	depProcs := make([]*process, len(p.cfg.DependsOn))
	for i, depName := range p.cfg.DependsOn {
		depProcs[i] = s.processes[depName]
	}
	s.mu.RUnlock()

	for i, dep := range depProcs {
		dep.mu.Lock()
		depPort := dep.assignedPort
		dep.mu.Unlock()
		if depPort > 0 {
			env = append(env, fmt.Sprintf("%s_PORT=%d", strings.ToUpper(p.cfg.DependsOn[i]), depPort))
		}
	}

	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.state = StateCrashed
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.state = StateCrashed
		return err
	}
	if err := cmd.Start(); err != nil {
		p.state = StateCrashed
		return err
	}

	p.cmd = cmd
	p.pid = cmd.Process.Pid
	p.startedAt = time.Now()
	p.done = make(chan struct{})
	p.running = make(chan struct{})
	port := p.assignedPort

	go pipeToRing(stdout, &p.logs, "stdout")
	go pipeToRing(stderr, &p.logs, "stderr")
	go s.watch(p, cmd)

	if port > 0 {
		p.state = StateProbing
		go s.probePort(p, port)
	} else {
		p.state = StateRunning
		close(p.running)
	}
	s.logEvent(p, "started")
	return nil
}

func (s *Supervisor) Stop(name string) error {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return fmt.Errorf("unknown process: %s", name)
	}
	return s.stopProcess(p, "")
}

func (s *Supervisor) stopProcess(p *process, reason string) error {
	p.mu.Lock()
	if p.state == StateStopped {
		p.mu.Unlock()
		return nil
	}
	p.intentional = true
	cmd := p.cmd
	done := p.done
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	s.logEvent(p, "stopping...")

	termProcessGroup(cmd)
	select {
	case <-done:
	case <-time.After(p.cfg.ShutdownTimeoutDuration()):
		s.logEvent(p, "shutdown timeout, killing")
		killProcessGroup(cmd)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}

	p.mu.Lock()
	p.state = StateStopped
	p.pid = 0
	p.mu.Unlock()

	msg := "stopped"
	if reason != "" {
		msg = fmt.Sprintf("stopped (%s)", reason)
	}
	s.logEvent(p, msg)
	return nil
}

func (s *Supervisor) Restart(name string) error {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return fmt.Errorf("unknown process: %s", name)
	}

	p.mu.Lock()
	oldPort := p.assignedPort
	p.mu.Unlock()

	if err := s.stopProcess(p, "restart"); err != nil {
		return err
	}
	if err := s.startProcess(p); err != nil {
		return err
	}

	p.mu.Lock()
	newPort := p.assignedPort
	p.mu.Unlock()

	if oldPort != 0 && newPort != 0 && newPort != oldPort {
		go s.cascadePortChange(name)
	}
	return nil
}

func (s *Supervisor) StartAll() {
	s.mu.RLock()
	order := s.order
	processes := s.processes
	s.mu.RUnlock()

	// ready[name] is closed when name has reached StateRunning (or failed to start).
	ready := make(map[string]chan struct{}, len(order))
	for _, name := range order {
		ready[name] = make(chan struct{})
	}

	var wg sync.WaitGroup
	for _, name := range order {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			defer close(ready[name])

			p := processes[name]

			for _, depName := range p.cfg.DependsOn {
				<-ready[depName]
				dep := processes[depName]
				dep.mu.Lock()
				state := dep.state
				dep.mu.Unlock()
				if state != StateRunning {
					s.logEvent(p, fmt.Sprintf("dependency %s not ready, skipping", depName))
					return
				}
			}

			if err := s.startProcess(p); err != nil {
				return
			}

			// Wait until StateRunning (push-based via p.running).
			p.mu.Lock()
			running := p.running
			state := p.state
			p.mu.Unlock()

			if state == StateRunning {
				return
			}
			if running != nil {
				select {
				case <-running:
				case <-time.After(30 * time.Second):
				}
			}
		}(name)
	}
	wg.Wait()
}

// RestartAll restarts every process in dependency order, waiting for each
// dependency to become running before restarting its dependents.
func (s *Supervisor) RestartAll() {
	s.mu.RLock()
	order := s.order
	processes := s.processes
	s.mu.RUnlock()

	ready := make(map[string]chan struct{}, len(order))
	for _, name := range order {
		ready[name] = make(chan struct{})
	}

	var wg sync.WaitGroup
	for _, name := range order {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			defer close(ready[name])

			p := processes[name]

			for _, depName := range p.cfg.DependsOn {
				<-ready[depName]
				dep := processes[depName]
				dep.mu.Lock()
				state := dep.state
				dep.mu.Unlock()
				if state != StateRunning {
					s.logEvent(p, fmt.Sprintf("dependency %s not ready, skipping", depName))
					return
				}
			}

			s.stopProcess(p, "restart") //nolint

			if err := s.startProcess(p); err != nil {
				return
			}

			p.mu.Lock()
			running := p.running
			state := p.state
			p.mu.Unlock()

			if state == StateRunning {
				return
			}
			if running != nil {
				select {
				case <-running:
				case <-time.After(30 * time.Second):
				}
			}
		}(name)
	}
	wg.Wait()
}

func (s *Supervisor) StopAll() {
	s.mu.RLock()
	order := s.order
	processes := s.processes
	s.mu.RUnlock()

	var wg sync.WaitGroup
	for _, name := range order {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			s.stopProcess(processes[name], "") //nolint
		}(name)
	}
	wg.Wait()
}

// cascadePortChange BFS-restarts every process that (transitively) depends on
// changedName, but only if each restarted process also gets a new port.
func (s *Supervisor) cascadePortChange(changedName string) {
	s.mu.RLock()
	dependents := s.dependents
	processes := s.processes
	s.mu.RUnlock()

	queue := []string{changedName}
	seen := map[string]bool{changedName: true}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		for _, depName := range dependents[name] {
			if seen[depName] {
				continue
			}
			seen[depName] = true

			p := processes[depName]
			p.mu.Lock()
			oldPort := p.assignedPort
			p.mu.Unlock()

			s.stopProcess(p, fmt.Sprintf("dependency %s port changed", name)) //nolint
			s.startProcess(p)                                                 //nolint

			p.mu.Lock()
			newPort := p.assignedPort
			p.mu.Unlock()

			if oldPort != 0 && newPort != 0 && newPort != oldPort {
				queue = append(queue, depName)
			}
		}
	}
}

func (s *Supervisor) Status() []ProcessStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProcessStatus, 0, len(s.processes))
	for _, name := range s.order {
		p := s.processes[name]
		p.mu.Lock()
		out = append(out, ProcessStatus{
			Name:      p.cfg.Name,
			State:     p.state.String(),
			PID:       p.pid,
			Cmd:       p.cfg.Cmd,
			Cwd:       p.cfg.Cwd,
			Port:      p.assignedPort,
			PortEnv:   p.cfg.PortEnv,
			Env:       p.cfg.Env,
			DependsOn: p.cfg.DependsOn,
			Restarts:  p.restarts,
			StartedAt: p.startedAt,
		})
		p.mu.Unlock()
	}
	return out
}

func (s *Supervisor) Logs(name string, n int) []LogEntry {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return nil
	}
	return p.logs.entries(n)
}

func (s *Supervisor) logEvent(p *process, message string) {
	p.logs.write(message, "invincible")
}

// watch waits for a process to exit and restarts it unless it was stopped intentionally.
func (s *Supervisor) watch(p *process, cmd *exec.Cmd) {
	cmd.Wait() //nolint
	close(p.done)

	p.mu.Lock()
	intentional := p.intentional
	oldPort := p.assignedPort
	if !intentional {
		p.state = StateCrashed
	}
	p.mu.Unlock()

	if intentional {
		return
	}

	p.mu.Lock()
	p.restarts++
	restartN := p.restarts
	p.mu.Unlock()

	s.logEvent(p, fmt.Sprintf("crashed (restart #%d)", restartN))

	time.Sleep(p.cfg.RestartDelayDuration())
	s.startProcess(p) //nolint

	p.mu.Lock()
	newPort := p.assignedPort
	p.mu.Unlock()

	if oldPort != 0 && newPort != 0 && newPort != oldPort {
		s.cascadePortChange(p.cfg.Name)
	}
}

// probePort polls with ports.ProbePort until the process accepts a connection, then sets StateRunning.
func (s *Supervisor) probePort(p *process, port int) {
	for {
		time.Sleep(50 * time.Millisecond)
		p.mu.Lock()
		alive := p.state == StateProbing
		p.mu.Unlock()
		if !alive {
			return
		}
		if ports.ProbePort(port) {
			p.mu.Lock()
			if p.state == StateProbing {
				p.state = StateRunning
				close(p.running)
			}
			p.mu.Unlock()
			return
		}
	}
}

func pipeToRing(r io.Reader, ring *ringBuffer, source string) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		ring.write(sc.Text(), source)
	}
}
