package supervisor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/saintedlama/invincible/internal/config"
	"github.com/saintedlama/invincible/internal/graph"
	"github.com/saintedlama/invincible/internal/ports"
)

type State int

const (
	StateStopped State = iota
	StateStarting
	StateProbing
	StateRunning
	StateCrashed
	StateBuilding
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
	case StateBuilding:
		return "building"
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
	Watching  bool
}

type Supervisor struct {
	mu        sync.RWMutex
	processes map[string]*process
	order     []string     // insertion order from config, used for display
	g         *graph.Graph // dependency graph
}

func New(cfgs []config.ProcessConfig) *Supervisor {
	s := &Supervisor{
		processes: make(map[string]*process),
		order:     make([]string, 0, len(cfgs)),
	}
	edges := make([]graph.Edge, len(cfgs))
	for i, c := range cfgs {
		s.processes[c.Name] = &process{cfg: c}
		s.order = append(s.order, c.Name)
		edges[i] = graph.Edge{Name: c.Name, Deps: c.DependsOn}
	}
	s.g = graph.New(edges)
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

	cmd := ShellCommand(p.cfg.Cmd)
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
		KillProcessGroup(cmd)
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

// Build runs a build command for the named process. The process must be
// running. Log output is written to the process's invincible log stream.
// If ctx is cancelled, the build command is killed and the state reverts.
func (s *Supervisor) Build(name, buildCmd, cwd string, ctx context.Context) error {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return fmt.Errorf("unknown process: %s", name)
	}

	p.mu.Lock()
	if p.state != StateRunning && p.state != StateBuilding {
		p.mu.Unlock()
		return fmt.Errorf("cannot build: process is %s", p.state.String())
	}
	p.state = StateBuilding
	p.mu.Unlock()

	cmd := ShellCommand(buildCmd)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Stdout = &logWriter{sup: s, name: name, prefix: "build: "}
	cmd.Stderr = &logWriter{sup: s, name: name, prefix: "build: "}
	if err := cmd.Start(); err != nil {
		p.mu.Lock()
		if p.state == StateBuilding {
			p.state = StateRunning
		}
		p.mu.Unlock()
		s.Log(name, "build: failed to start: "+err.Error())
		return err
	}
	s.Log(name, "build: running...")

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var buildErr error
	select {
	case buildErr = <-done:
	case <-ctx.Done():
		s.Log(name, "build: cancelled")
		KillProcessGroup(cmd)
		<-done
		buildErr = ctx.Err()
	}

	p.mu.Lock()
	if p.state == StateBuilding {
		p.state = StateRunning
	}
	p.mu.Unlock()
	return buildErr
}

// logWriter implements io.Writer, splitting output into lines and writing
// each to the supervisor's invincible log stream.
type logWriter struct {
	sup    *Supervisor
	name   string
	prefix string
	buf    []byte
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := indexByte(w.buf, '\n')
		if i == -1 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		if line != "" {
			w.sup.Log(w.name, w.prefix+line)
		}
	}
	return len(p), nil
}

// indexByte returns the index of the first occurrence of b in s, or -1.
func indexByte(s []byte, b byte) int {
	for i, c := range s {
		if c == b {
			return i
		}
	}
	return -1
}

func (s *Supervisor) StartAll() {
	s.mu.RLock()
	processes := s.processes
	levels := s.g.StartLevels()
	s.mu.RUnlock()

	for _, level := range levels {
		var wg sync.WaitGroup
		for _, name := range level {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				s.startProcess(processes[name]) //nolint
			}(name)
		}
		wg.Wait()

		// Wait for every process in this level to reach running (or time out).
		for _, name := range level {
			p := processes[name]
			p.mu.Lock()
			running := p.running
			state := p.state
			p.mu.Unlock()
			if state == StateRunning || state == StateCrashed {
				continue
			}
			if running != nil {
				select {
				case <-running:
				case <-time.After(30 * time.Second):
				}
			}
		}
	}
}

// RestartAll restarts every process in dependency order. Each level starts
// after the previous level has reached running.
func (s *Supervisor) RestartAll() {
	s.mu.RLock()
	processes := s.processes
	levels := s.g.StartLevels()
	s.mu.RUnlock()

	for _, level := range levels {
		var wg sync.WaitGroup
		for _, name := range level {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				s.stopProcess(processes[name], "restart") //nolint
				s.startProcess(processes[name])           //nolint
			}(name)
		}
		wg.Wait()

		for _, name := range level {
			p := processes[name]
			p.mu.Lock()
			running := p.running
			state := p.state
			p.mu.Unlock()
			if state == StateRunning || state == StateCrashed {
				continue
			}
			if running != nil {
				select {
				case <-running:
				case <-time.After(30 * time.Second):
				}
			}
		}
	}
}

func (s *Supervisor) StopAll() {
	s.mu.RLock()
	processes := s.processes
	levels := s.g.StopLevels()
	s.mu.RUnlock()

	for _, level := range levels {
		var wg sync.WaitGroup
		for _, name := range level {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				s.stopProcess(processes[name], "") //nolint
			}(name)
		}
		wg.Wait()
	}
}

// cascadePortChange BFS-restarts every process that (transitively) depends on
// changedName, but only if each restarted process also gets a new port.
func (s *Supervisor) cascadePortChange(changedName string) {
	s.mu.RLock()
	processes := s.processes
	s.mu.RUnlock()

	s.g.WalkDependents(changedName, func(depName string) bool {
		p := processes[depName]
		p.mu.Lock()
		oldPort := p.assignedPort
		p.mu.Unlock()

		s.stopProcess(p, "dependency port changed") //nolint
		s.startProcess(p)                           //nolint

		p.mu.Lock()
		newPort := p.assignedPort
		p.mu.Unlock()

		// Only cascade further if this process also got a new port.
		return oldPort != 0 && newPort != 0 && newPort != oldPort
	})
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
			Watching:  len(p.cfg.Watch) > 0 && p.cfg.Build != "",
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

// Log writes a message to a process's invincible log stream.
func (s *Supervisor) Log(name, message string) {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return
	}
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
