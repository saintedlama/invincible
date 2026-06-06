package caddy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/saintedlama/invincible/internal/config"
	"github.com/saintedlama/invincible/internal/ports"
	"github.com/saintedlama/invincible/internal/supervisor"
)

type StatusProvider interface {
	Status() []supervisor.ProcessStatus
}

type Manager struct {
	cfg        config.CaddyConfig
	sup        StatusProvider
	caddyPort  int
	listenAddr string

	mu       sync.Mutex
	cmd      *exec.Cmd
	caddyDir string
}

func New(cfg config.CaddyConfig, sup StatusProvider) (*Manager, error) {
	m := &Manager{
		cfg:   cfg,
		sup:   sup,
	}

	port := cfg.Port
	if port == 0 {
		port = 8443
	}
	var err error
	port, err = ports.FindFree(port)
	if err != nil {
		return nil, fmt.Errorf("caddy: %w", err)
	}
	m.caddyPort = port

	dir, err := os.MkdirTemp("", "invincible-caddy")
	if err != nil {
		return nil, fmt.Errorf("caddy: %w", err)
	}
	m.caddyDir = dir

	m.rebuildListenAddr()
	return m, nil
}

func (m *Manager) rebuildListenAddr() {
	m.listenAddr = fmt.Sprintf("127.0.0.1:%d", m.caddyPort)
}

func (m *Manager) ListenAddr() string {
	return m.listenAddr
}

func (m *Manager) CaddyPort() int {
	return m.caddyPort
}

func (m *Manager) writeCaddyfile() error {
	content := m.generateCaddyfile()
	return os.WriteFile(filepath.Join(m.caddyDir, "Caddyfile"), []byte(content), 0644)
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil {
		return nil
	}

	caddyPath, err := exec.LookPath("caddy")
	if err != nil {
		return fmt.Errorf("caddy: binary not found in PATH — install from https://caddyserver.com/download")
	}

	if err := m.writeCaddyfile(); err != nil {
		return fmt.Errorf("caddy: %w", err)
	}

	cmd := exec.Command(caddyPath, "run", "--config", filepath.Join(m.caddyDir, "Caddyfile"))
	cmd.Dir = m.caddyDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("caddy: start: %w", err)
	}
	m.cmd = cmd

	go func() {
		cmd.Wait() //nolint
		m.mu.Lock()
		m.cmd = nil
		m.mu.Unlock()
	}()

	if runtime.GOOS == "windows" {
		fmt.Fprintf(os.Stderr, "caddy: listening on %s (path-based routes)\n", m.listenAddr)
	} else {
		fmt.Fprintf(os.Stderr, "caddy: listening on %s (subdomain routes)\n", m.listenAddr)
	}
	fmt.Fprintln(os.Stderr, "caddy: routes:")
	m.printRoutes()
	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	cmd := m.cmd
	m.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run() //nolint
	} else {
		cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

func (m *Manager) Restart() error {
	m.Stop()
	time.Sleep(200 * time.Millisecond)
	return m.Start()
}

func (m *Manager) printRoutes() {
	for _, p := range m.sup.Status() {
		if p.Port == 0 {
			continue
		}
		if runtime.GOOS == "windows" {
			fmt.Fprintf(os.Stderr, "  /%s/* → localhost:%d\n", p.Name, p.Port)
		} else {
			fmt.Fprintf(os.Stderr, "  %s.localhost:%d → localhost:%d\n", p.Name, m.caddyPort, p.Port)
		}
	}
}

func (m *Manager) Cleanup() {
	m.Stop()
	os.RemoveAll(m.caddyDir)
}

func (m *Manager) Watch() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	lastPorts := make(map[string]int)
	for range ticker.C {
		current := snapshotPorts(m.sup.Status())
		if portsChanged(lastPorts, current) {
			lastPorts = current
			m.writeCaddyfile() //nolint
			m.Restart()        //nolint
			m.printRoutes()
		}
	}
}

func snapshotPorts(statuses []supervisor.ProcessStatus) map[string]int {
	m := make(map[string]int, len(statuses))
	for _, s := range statuses {
		m[s.Name] = s.Port
	}
	return m
}

func portsChanged(a, b map[string]int) bool {
	if len(a) != len(b) {
		return true
	}
	for k, v := range a {
		if b[k] != v {
			return true
		}
	}
	return false
}
