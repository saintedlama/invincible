package supervisor

import (
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/saintedlama/invincible/internal/config"
)

func requireSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
}

func TestNew_InitialState(t *testing.T) {
	sup := New([]config.ProcessConfig{
		{Name: "foo", Cmd: "echo foo", PortEnv: "PORT"},
		{Name: "bar", Cmd: "echo bar", PortEnv: "PORT"},
	})

	statuses := sup.Status()
	if len(statuses) != 2 {
		t.Fatalf("got %d processes, want 2", len(statuses))
	}
	for _, s := range statuses {
		if s.State != "stopped" {
			t.Errorf("process %q: got state %q, want stopped", s.Name, s.State)
		}
		if s.PID != 0 {
			t.Errorf("process %q: got PID %d, want 0", s.Name, s.PID)
		}
	}
}

func TestSupervisor_StartStop(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	s := sup.Status()[0]
	if s.State != "running" {
		t.Errorf("after start: got %q, want running", s.State)
	}
	if s.PID == 0 {
		t.Error("PID should be non-zero after start")
	}

	if err := sup.Stop("proc"); err != nil {
		t.Fatal(err)
	}

	s = sup.Status()[0]
	if s.State != "stopped" {
		t.Errorf("after stop: got %q, want stopped", s.State)
	}
	if s.PID != 0 {
		t.Errorf("PID should be 0 after stop, got %d", s.PID)
	}
}

func TestSupervisor_Restart(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	pidBefore := sup.Status()[0].PID

	if err := sup.Restart("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	s := sup.Status()[0]
	if s.State != "running" {
		t.Errorf("after restart: got %q, want running", s.State)
	}
	if s.PID == pidBefore {
		t.Error("PID should change after restart")
	}
}

func TestSupervisor_StartAll_StopAll(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "p1", Cmd: "sleep 60", PortEnv: "PORT"},
		{Name: "p2", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	sup.StartAll()
	time.Sleep(100 * time.Millisecond)

	for _, s := range sup.Status() {
		if s.State != "running" {
			t.Errorf("%s: got %q, want running", s.Name, s.State)
		}
	}

	sup.StopAll()

	for _, s := range sup.Status() {
		if s.State != "stopped" {
			t.Errorf("%s: got %q, want stopped", s.Name, s.State)
		}
	}
}

func TestSupervisor_Logs(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "logger", Cmd: `echo "hello from process" && sleep 60`, PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	if err := sup.Start("logger"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	logs := sup.Logs("logger", 10)
	found := false
	for _, l := range logs {
		if l.Line == "hello from process" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected log line not found, got: %v", logs)
	}
}

func TestSupervisor_SetEnv(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "envtest", Cmd: `echo "MY_VAR=$MY_VAR" && sleep 60`, PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	sup.SetEnv("envtest", []string{"MY_VAR=hello123"})

	if err := sup.Start("envtest"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	logs := sup.Logs("envtest", 10)
	found := false
	for _, l := range logs {
		if l.Line == "MY_VAR=hello123" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected env var in output, got: %v", logs)
	}
}

func TestSupervisor_UnknownProcess(t *testing.T) {
	sup := New([]config.ProcessConfig{})

	if err := sup.Start("nobody"); err == nil {
		t.Error("Start: expected error for unknown process")
	}
	if err := sup.Stop("nobody"); err == nil {
		t.Error("Stop: expected error for unknown process")
	}
}

func TestSupervisor_ProbePort_TransitionsToRunning(t *testing.T) {
	requireSh(t)

	// Find a free port and release it so startProcess sees it as available.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)
	sup.SetPort("proc", port)

	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}

	// Bind the port now to simulate the process having started its server.
	l, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	time.Sleep(200 * time.Millisecond)

	if s := sup.Status()[0].State; s != "running" {
		t.Errorf("expected running after port probe succeeded, got %q", s)
	}
}

func TestSupervisor_ProbePort_StaysProbing(t *testing.T) {
	requireSh(t)

	// Get a free port but don't bind it — sleep 60 won't bind it either.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)
	sup.SetPort("proc", port)

	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	if s := sup.Status()[0].State; s != "probing" {
		t.Errorf("expected probing while port unbound, got %q", s)
	}
}

func TestSupervisor_RestartAll_TopoOrder(t *testing.T) {
	// worker ← api ← frontend; RestartAll must restart worker before api before frontend.
	sup := New([]config.ProcessConfig{
		{Name: "frontend", Cmd: "echo", PortEnv: "PORT", DependsOn: []string{"api"}},
		{Name: "api", Cmd: "echo", PortEnv: "PORT", DependsOn: []string{"worker"}},
		{Name: "worker", Cmd: "echo", PortEnv: "PORT"},
	})

	order := sup.topoOrder()

	idx := func(name string) int {
		for i, n := range order {
			if n == name {
				return i
			}
		}
		return -1
	}

	if idx("worker") >= idx("api") {
		t.Errorf("worker must come before api, got order %v", order)
	}
	if idx("api") >= idx("frontend") {
		t.Errorf("api must come before frontend, got order %v", order)
	}
}

func TestSupervisor_Logs_UnknownProcess(t *testing.T) {
	sup := New([]config.ProcessConfig{})
	if logs := sup.Logs("nobody", 10); logs != nil {
		t.Errorf("expected nil logs for unknown process, got %v", logs)
	}
}

func TestSupervisor_StartIdempotent(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	pid1 := sup.Status()[0].PID

	// Second Start on a running process should be a no-op.
	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}
	pid2 := sup.Status()[0].PID

	if pid1 != pid2 {
		t.Errorf("double start changed PID: %d → %d", pid1, pid2)
	}
}

func TestSupervisor_LifecycleEvents(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	logs := sup.Logs("proc", 50)
	if !logContains(logs, "starting...", "invincible") {
		t.Error("expected 'starting...' event from invincible")
	}
	if !logContains(logs, "started", "invincible") {
		t.Error("expected 'started' event from invincible")
	}

	if err := sup.Restart("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	logs = sup.Logs("proc", 50)
	if !logContains(logs, "stopping...", "invincible") {
		t.Error("expected 'stopping...' event from invincible")
	}
	if !logContains(logs, "stopped (restart)", "invincible") {
		t.Error("expected 'stopped (restart)' event from invincible")
	}

	if err := sup.Stop("proc"); err != nil {
		t.Fatal(err)
	}

	logs = sup.Logs("proc", 50)
	if !logContains(logs, "stopped", "invincible") {
		t.Error("expected 'stopped' event from invincible")
	}
}

func logContains(entries []LogEntry, line, source string) bool {
	for _, e := range entries {
		if e.Line == line && e.Source == source {
			return true
		}
	}
	return false
}
