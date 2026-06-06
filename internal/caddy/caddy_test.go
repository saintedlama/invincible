package caddy

import (
	"testing"

	"github.com/saintedlama/invincible/internal/supervisor"
)

type stubProvider struct {
	statuses []supervisor.ProcessStatus
}

func (s *stubProvider) Status() []supervisor.ProcessStatus {
	return s.statuses
}

func TestGenerateCaddyfile_Empty(t *testing.T) {
	p := &stubProvider{statuses: nil}
	m := &Manager{sup: p, caddyPort: 8443}
	content := m.generateCaddyfile()
	if content == "" {
		t.Error("expected non-empty Caddyfile")
	}
}

func TestGenerateCaddyfile_WithProcesses(t *testing.T) {
	p := &stubProvider{
		statuses: []supervisor.ProcessStatus{
			{Name: "api", Port: 8080},
			{Name: "worker", Port: 9000},
			{Name: "cron", Port: 0}, // no port
		},
	}
	m := &Manager{sup: p, caddyPort: 8443}
	content := m.generateCaddyfile()

	if content == "" {
		t.Error("expected non-empty Caddyfile")
	}
	// Processes with port 0 should be skipped.
}

func TestSnapshotPorts(t *testing.T) {
	statuses := []supervisor.ProcessStatus{
		{Name: "api", Port: 8080},
		{Name: "worker", Port: 9000},
	}
	m := snapshotPorts(statuses)
	if m["api"] != 8080 {
		t.Errorf("api port: got %d, want 8080", m["api"])
	}
	if m["worker"] != 9000 {
		t.Errorf("worker port: got %d, want 9000", m["worker"])
	}
}

func TestPortsChanged(t *testing.T) {
	a := map[string]int{"api": 8080}
	b := map[string]int{"api": 8080}
	if portsChanged(a, b) {
		t.Error("identical maps should not report changed")
	}

	b["api"] = 8081
	if !portsChanged(a, b) {
		t.Error("different values should report changed")
	}

	delete(b, "api")
	if !portsChanged(a, b) {
		t.Error("different keys should report changed")
	}
}
