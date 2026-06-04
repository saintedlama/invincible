package supervisor

import (
	"bufio"
	"fmt"
	"io"
	"net"
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
	extraEnv     []string // injected by port manager
	assignedPort int      // resolved free port (set before start via SetPort)

	mu          sync.Mutex
	state       State
	pid         int
	cmd         *exec.Cmd
	logs        ringBuffer
	stopOnce    sync.Once
	intentional bool
	restarts    int       // number of crash-triggered restarts
	startedAt   time.Time // time of most recent successful start
}

type ProcessStatus struct {
	Name      string
	State     string
	PID       int
	Cmd       string
	Port      int
	Env       map[string]string
	DependsOn []string
	Restarts  int
	StartedAt time.Time
}

type Supervisor struct {
	mu        sync.RWMutex
	processes map[string]*process
	order     []string // insertion order from config
}

func New(cfgs []config.ProcessConfig) *Supervisor {
	s := &Supervisor{
		processes: make(map[string]*process),
		order:     make([]string, 0, len(cfgs)),
	}
	for _, c := range cfgs {
		s.processes[c.Name] = &process{cfg: c}
		s.order = append(s.order, c.Name)
	}
	return s
}

// AssignPorts finds a free port for every process that needs one, then injects
// PORT= and peer {NAME}_PORT= vars into each process environment. Must be called
// before StartAll.
func (s *Supervisor) AssignPorts() error {
	s.mu.RLock()
	order := s.order
	s.mu.RUnlock()

	// First pass: resolve a free port for every process.
	assigned := make(map[string]int, len(order))
	for _, name := range order {
		s.mu.RLock()
		p := s.processes[name]
		s.mu.RUnlock()
		if p.cfg.NoPort {
			continue
		}
		port, err := ports.FindFree(p.cfg.Port)
		if err != nil {
			return fmt.Errorf("process %q: %w", p.cfg.Name, err)
		}
		p.mu.Lock()
		p.assignedPort = port
		p.mu.Unlock()
		assigned[name] = port
	}

	// Second pass: build env with own port + all peer ports.
	for _, name := range order {
		s.mu.RLock()
		p := s.processes[name]
		s.mu.RUnlock()
		var env []string
		if port, ok := assigned[name]; ok {
			env = append(env, fmt.Sprintf("%s=%d", p.cfg.PortEnv, port))
		}
		for _, other := range order {
			if otherPort, ok := assigned[other]; ok {
				env = append(env, fmt.Sprintf("%s_PORT=%d", strings.ToUpper(other), otherPort))
			}
		}
		if len(env) > 0 {
			p.mu.Lock()
			p.extraEnv = env
			p.mu.Unlock()
		}
	}
	return nil
}

// SetEnv adds extra environment variables for a named process (e.g. PORT=).
func (s *Supervisor) SetEnv(name string, env []string) {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return
	}
	p.mu.Lock()
	p.extraEnv = env
	p.mu.Unlock()
}

// SetPort records the resolved port for a named process so startProcess can
// probe it before declaring the process running.
func (s *Supervisor) SetPort(name string, port int) {
	s.mu.RLock()
	p := s.processes[name]
	s.mu.RUnlock()
	if p == nil {
		return
	}
	p.mu.Lock()
	p.assignedPort = port
	p.mu.Unlock()
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
	p.stopOnce = sync.Once{}
	p.state = StateStarting
	s.logEvent(p, "starting...")

	// Ensure the assigned port is still free; re-find if stolen since last assignment.
	if p.assignedPort > 0 {
		if !ports.IsFree(p.assignedPort) {
			newPort, err := ports.FindFree(p.cfg.Port)
			if err != nil {
				p.state = StateCrashed
				return fmt.Errorf("process %q: %w", p.cfg.Name, err)
			}
			p.assignedPort = newPort
			p.rebuildPortEnv(newPort)
		}
	}

	cmd := exec.Command("sh", "-c", p.cfg.Cmd)
	setProcessGroupAttr(cmd)
	// Build env: inherit + config env + extra (port)
	env := envFromParent()
	for k, v := range p.cfg.Env {
		env = append(env, k+"="+v)
	}
	env = append(env, p.extraEnv...)
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
	port := p.assignedPort

	go pipeToRing(stdout, &p.logs, "stdout")
	go pipeToRing(stderr, &p.logs, "stderr")
	go s.watch(p, cmd)

	if port > 0 {
		p.state = StateProbing
		go s.probePort(p, port)
	} else {
		p.state = StateRunning
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
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	s.logEvent(p, "stopping...")

	done := make(chan struct{})
	go func() {
		cmd.Wait() //nolint
		close(done)
	}()

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
	if err := s.stopProcess(p, "restart"); err != nil {
		return err
	}
	return s.startProcess(p)
}

// RestartAll restarts every process in dependency order, waiting for each
// dependency to become running before restarting its dependents.
func (s *Supervisor) RestartAll() {
	s.mu.RLock()
	ordered := s.topoOrder()
	s.mu.RUnlock()

	ready := make(map[string]bool, len(ordered))

	for _, name := range ordered {
		s.mu.RLock()
		p := s.processes[name]
		s.mu.RUnlock()

		for _, dep := range p.cfg.DependsOn {
			if !ready[dep] {
				s.logEvent(p, fmt.Sprintf("dependency %s not ready, skipping", dep))
				goto skip
			}
		}

		s.Restart(name) //nolint
		if s.waitForRunning(name, 30*time.Second) {
			ready[name] = true
		}
	skip:
	}
}

// topoOrder returns process names with dependencies before dependents.
// Must be called with s.mu held.
func (s *Supervisor) topoOrder() []string {
	visited := make(map[string]bool, len(s.order))
	result := make([]string, 0, len(s.order))

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		for _, dep := range s.processes[name].cfg.DependsOn {
			visit(dep)
		}
		result = append(result, name)
	}

	for _, name := range s.order {
		visit(name)
	}
	return result
}

// waitForRunning polls until the named process reaches StateRunning or the
// timeout expires. Returns true if running, false on timeout or crash.
func (s *Supervisor) waitForRunning(name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.RLock()
		p := s.processes[name]
		s.mu.RUnlock()
		if p == nil {
			return false
		}
		p.mu.Lock()
		state := p.state
		p.mu.Unlock()
		if state == StateRunning {
			return true
		}
		if state == StateCrashed || state == StateStopped {
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Supervisor) StartAll() {
	s.mu.RLock()
	ordered := s.topoOrder()
	s.mu.RUnlock()

	started := make(map[string]bool, len(ordered))

	for _, name := range ordered {
		s.mu.RLock()
		p := s.processes[name]
		s.mu.RUnlock()

		for _, dep := range p.cfg.DependsOn {
			if !started[dep] {
				s.logEvent(p, fmt.Sprintf("dependency %s not started, skipping", dep))
				goto skip
			}
			if !s.waitForRunning(dep, 30*time.Second) {
				s.logEvent(p, fmt.Sprintf("dependency %s not ready, skipping", dep))
				goto skip
			}
		}

		s.startProcess(p) //nolint
		started[name] = true
	skip:
	}
}

func (s *Supervisor) StopAll() {
	s.mu.RLock()
	order := s.order
	s.mu.RUnlock()
	for _, name := range order {
		s.mu.RLock()
		p := s.processes[name]
		s.mu.RUnlock()
		s.stopProcess(p, "") //nolint
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
			Port:      p.assignedPort,
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
		s.restartDependents(p.cfg.Name, newPort)
	}
}

// restartDependents updates the peer port env var and restarts every process
// that declares a depends_on on changedName, because its port changed.
func (s *Supervisor) restartDependents(changedName string, newPort int) {
	portKey := strings.ToUpper(changedName) + "_PORT"

	s.mu.RLock()
	var toRestart []string
	for _, name := range s.order {
		p := s.processes[name]
		for _, dep := range p.cfg.DependsOn {
			if dep != changedName {
				continue
			}
			// Update the peer port entry in extraEnv.
			p.mu.Lock()
			for i, kv := range p.extraEnv {
				if key, _, _ := strings.Cut(kv, "="); key == portKey {
					p.extraEnv[i] = fmt.Sprintf("%s=%d", portKey, newPort)
					break
				}
			}
			p.mu.Unlock()
			toRestart = append(toRestart, name)
			break
		}
	}
	s.mu.RUnlock()

	for _, name := range toRestart {
		go func(n string) {
			s.mu.RLock()
			dp := s.processes[n]
			s.mu.RUnlock()
			if dp != nil {
				s.stopProcess(dp, fmt.Sprintf("dependency %s port changed", changedName)) //nolint
				s.startProcess(dp)                                                        //nolint
			}
		}(name)
	}
}

// probePort polls until the process accepts a connection on its port, then sets StateRunning.
func (s *Supervisor) probePort(p *process, port int) {
	addr := fmt.Sprintf("localhost:%d", port)
	for {
		time.Sleep(50 * time.Millisecond)
		p.mu.Lock()
		alive := p.state == StateProbing
		p.mu.Unlock()
		if !alive {
			return
		}
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			p.mu.Lock()
			if p.state == StateProbing {
				p.state = StateRunning
			}
			p.mu.Unlock()
			return
		}
	}
}

// rebuildPortEnv replaces the own-port entries in extraEnv with newPort,
// preserving all peer port entries. Must be called with p.mu held.
func (p *process) rebuildPortEnv(newPort int) {
	selfKey := strings.ToUpper(p.cfg.Name) + "_PORT"
	rebuilt := make([]string, 0, len(p.extraEnv))
	for _, kv := range p.extraEnv {
		key, _, _ := strings.Cut(kv, "=")
		if key != p.cfg.PortEnv && key != selfKey {
			rebuilt = append(rebuilt, kv)
		}
	}
	p.extraEnv = append(rebuilt,
		fmt.Sprintf("%s=%d", p.cfg.PortEnv, newPort),
		fmt.Sprintf("%s=%d", selfKey, newPort),
	)
}

func pipeToRing(r io.Reader, ring *ringBuffer, source string) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		ring.write(sc.Text(), source)
	}
}
