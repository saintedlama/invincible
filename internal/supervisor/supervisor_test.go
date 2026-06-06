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
		{Name: "proc", Cmd: "sleep 60", NoPort: true},
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
		{Name: "proc", Cmd: "sleep 60", NoPort: true},
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
		{Name: "p1", Cmd: "sleep 60", NoPort: true},
		{Name: "p2", Cmd: "sleep 60", NoPort: true},
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
		{Name: "logger", Cmd: `echo "hello from process" && sleep 60`, NoPort: true},
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

func TestSupervisor_ConfigEnv(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "envtest", Cmd: `echo "MY_VAR=$MY_VAR" && sleep 60`, PortEnv: "PORT", NoPort: true, Env: map[string]string{"MY_VAR": "hello123"}},
	})
	t.Cleanup(sup.StopAll)

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
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT", Port: port},
	})
	t.Cleanup(sup.StopAll)

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
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT", Port: port},
	})
	t.Cleanup(sup.StopAll)

	if err := sup.Start("proc"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	if s := sup.Status()[0].State; s != "probing" {
		t.Errorf("expected probing while port unbound, got %q", s)
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
		{Name: "proc", Cmd: "sleep 60", NoPort: true},
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

func TestSupervisor_StartAll_DependencyOrder(t *testing.T) {
	requireSh(t)

	// worker ← api ← frontend; chain with NoPort so all go to running immediately.
	sup := New([]config.ProcessConfig{
		{Name: "frontend", Cmd: "sleep 60", NoPort: true, PortEnv: "PORT", DependsOn: []string{"api"}},
		{Name: "api", Cmd: "sleep 60", NoPort: true, PortEnv: "PORT", DependsOn: []string{"worker"}},
		{Name: "worker", Cmd: "sleep 60", NoPort: true, PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	sup.StartAll()
	time.Sleep(200 * time.Millisecond)

	for _, s := range sup.Status() {
		if s.State != "running" {
			t.Errorf("%s: got %q, want running", s.Name, s.State)
		}
	}
}

func TestSupervisor_waitForRunning_Timeout(t *testing.T) {
	requireSh(t)

	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", PortEnv: "PORT"},
	})
	t.Cleanup(sup.StopAll)

	// StartAll finds a free port; sleep 60 never binds it so probing never completes.
	sup.StartAll()
	time.Sleep(200 * time.Millisecond)

	if s := sup.Status()[0].State; s != "probing" {
		t.Errorf("expected probing (port never bound), got %q", s)
	}
}

func TestSupervisor_LifecycleEvents(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "proc", Cmd: "sleep 60", NoPort: true},
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

func TestSupervisor_CrashRestart(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		// exit 1 terminates immediately; RestartDelay defaults to 0 in direct config.
		{Name: "crasher", Cmd: "exit 1", NoPort: true},
	})
	t.Cleanup(sup.StopAll)

	if err := sup.Start("crasher"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	if s := sup.Status()[0]; s.Restarts == 0 {
		t.Error("expected at least one crash-triggered restart, got 0")
	}
}

func TestSupervisor_DependencyPortEnv(t *testing.T) {
	requireSh(t)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dbPort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	sup := New([]config.ProcessConfig{
		{Name: "db", Cmd: "sleep 60", Port: dbPort, PortEnv: "PORT"},
		{Name: "api", Cmd: `echo "DB_PORT=$DB_PORT" && sleep 60`, NoPort: true, DependsOn: []string{"db"}},
	})
	t.Cleanup(sup.StopAll)

	// Start db first so its assignedPort is known, then start api.
	if err := sup.Start("db"); err != nil {
		t.Fatal(err)
	}
	if err := sup.Start("api"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	expected := fmt.Sprintf("DB_PORT=%d", dbPort)
	if !logContains(sup.Logs("api", 10), expected, "stdout") {
		t.Errorf("expected %q in api stdout, got: %v", expected, sup.Logs("api", 10))
	}
}

func TestSupervisor_RestartAll(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "db", Cmd: "sleep 60", NoPort: true},
		{Name: "api", Cmd: "sleep 60", NoPort: true, DependsOn: []string{"db"}},
	})
	t.Cleanup(sup.StopAll)

	sup.StartAll()
	time.Sleep(200 * time.Millisecond)

	pidsBefore := make(map[string]int)
	for _, s := range sup.Status() {
		pidsBefore[s.Name] = s.PID
	}

	sup.RestartAll()
	time.Sleep(200 * time.Millisecond)

	for _, s := range sup.Status() {
		if s.State != "running" {
			t.Errorf("%s: got state %q after RestartAll, want running", s.Name, s.State)
		}
		if s.PID == pidsBefore[s.Name] {
			t.Errorf("%s: PID unchanged after RestartAll (%d)", s.Name, s.PID)
		}
	}
}

func TestSupervisor_StopAll_DependencyOrder(t *testing.T) {
	requireSh(t)
	sup := New([]config.ProcessConfig{
		{Name: "db", Cmd: "sleep 60", NoPort: true},
		{Name: "api", Cmd: "sleep 60", NoPort: true, DependsOn: []string{"db"}},
		{Name: "frontend", Cmd: "sleep 60", NoPort: true, DependsOn: []string{"api"}},
	})

	sup.StartAll()
	time.Sleep(200 * time.Millisecond)
	sup.StopAll()

	stopTime := func(name string) time.Time {
		for _, e := range sup.Logs(name, 50) {
			if e.Line == "stopped" && e.Source == "invincible" {
				return e.Time
			}
		}
		return time.Time{}
	}

	frontendStopped := stopTime("frontend")
	apiStopped := stopTime("api")
	dbStopped := stopTime("db")

	if frontendStopped.IsZero() || apiStopped.IsZero() || dbStopped.IsZero() {
		t.Fatal("not all processes logged 'stopped'")
	}
	if frontendStopped.After(apiStopped) {
		t.Errorf("frontend must stop before api: frontend=%v api=%v", frontendStopped, apiStopped)
	}
	if apiStopped.After(dbStopped) {
		t.Errorf("api must stop before db: api=%v db=%v", apiStopped, dbStopped)
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
