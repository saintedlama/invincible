package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saintedlama/invincible/internal/supervisor"
)

// mockSup satisfies supervisorIface without spawning real processes.
type mockSup struct {
	statuses  []supervisor.ProcessStatus
	logs      map[string][]supervisor.LogEntry
	started   []string
	stopped   []string
	restarted []string
}

func (m *mockSup) Start(name string) error   { m.started = append(m.started, name); return nil }
func (m *mockSup) Stop(name string) error    { m.stopped = append(m.stopped, name); return nil }
func (m *mockSup) Restart(name string) error { m.restarted = append(m.restarted, name); return nil }
func (m *mockSup) RestartAll()               { m.restarted = append(m.restarted, "*") }
func (m *mockSup) Status() []supervisor.ProcessStatus { return m.statuses }
func (m *mockSup) Logs(name string, n int) []supervisor.LogEntry {
	entries := m.logs[name]
	if len(entries) > n {
		return entries[len(entries)-n:]
	}
	return entries
}

func newFixture(t *testing.T) (*Server, *mockSup) {
	t.Helper()
	mock := &mockSup{
		statuses: []supervisor.ProcessStatus{
			{Name: "api", State: "running", PID: 1234},
			{Name: "worker", State: "stopped", PID: 0},
		},
		logs: map[string][]supervisor.LogEntry{
			"api": {{Line: "line1"}, {Line: "line2"}, {Line: "line3"}},
		},
	}
	srv, err := New(mock, ":0")
	if err != nil {
		t.Fatal(err)
	}
	return srv, mock
}

func TestListProcesses(t *testing.T) {
	srv, _ := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/processes", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var out []supervisor.ProcessStatus
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Errorf("got %d processes, want 2", len(out))
	}
}

func TestGetProcess_Found(t *testing.T) {
	srv, _ := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/processes/api", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var s supervisor.ProcessStatus
	json.NewDecoder(w.Body).Decode(&s) //nolint
	if s.Name != "api" || s.State != "running" {
		t.Errorf("got %+v", s)
	}
}

func TestGetProcess_NotFound(t *testing.T) {
	srv, _ := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/processes/nobody", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetLogs_JSON(t *testing.T) {
	srv, _ := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/processes/api/logs", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var entries []supervisor.LogEntry
	json.NewDecoder(w.Body).Decode(&entries) //nolint
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}
}

func TestGetLogs_PlainText(t *testing.T) {
	srv, _ := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/processes/api/logs?format=text", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type: got %q", ct)
	}
	body, _ := io.ReadAll(w.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
}

func TestGetLogs_NParam(t *testing.T) {
	srv, _ := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/processes/api/logs?n=2", nil))

	var entries []supervisor.LogEntry
	json.NewDecoder(w.Body).Decode(&entries) //nolint
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func TestStartProcess(t *testing.T) {
	srv, mock := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/processes/api/start", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if len(mock.started) != 1 || mock.started[0] != "api" {
		t.Errorf("started: got %v, want [api]", mock.started)
	}
}

func TestStopProcess(t *testing.T) {
	srv, mock := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/processes/api/stop", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if len(mock.stopped) != 1 || mock.stopped[0] != "api" {
		t.Errorf("stopped: got %v, want [api]", mock.stopped)
	}
}

func TestRestartProcess(t *testing.T) {
	srv, mock := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/processes/api/restart", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if len(mock.restarted) != 1 || mock.restarted[0] != "api" {
		t.Errorf("restarted: got %v, want [api]", mock.restarted)
	}
}

func TestRestartAll(t *testing.T) {
	srv, mock := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/processes/restart-all", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if len(mock.restarted) != 1 || mock.restarted[0] != "*" {
		t.Errorf("restart-all not called: got %v", mock.restarted)
	}
}

func TestOpenAPISpec(t *testing.T) {
	srv, _ := newFixture(t)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q", ct)
	}
	var spec map[string]any
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if spec["openapi"] == nil {
		t.Error("spec missing openapi field")
	}
	if spec["paths"] == nil {
		t.Error("spec missing paths field")
	}
}
