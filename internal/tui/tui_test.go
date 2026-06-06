package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/saintedlama/invincible/internal/supervisor"
)

// stubSup implements supervisorIface without real processes.
type stubSup struct {
	statuses  []supervisor.ProcessStatus
	logs      map[string][]supervisor.LogEntry
	started   string
	stopped   string
	restarted string
}

func (s *stubSup) Start(name string) error                       { s.started = name; return nil }
func (s *stubSup) Stop(name string) error                        { s.stopped = name; return nil }
func (s *stubSup) Restart(name string) error                     { s.restarted = name; return nil }
func (s *stubSup) RestartAll()                                   {}
func (s *stubSup) Status() []supervisor.ProcessStatus            { return s.statuses }
func (s *stubSup) Logs(name string, _ int) []supervisor.LogEntry { return s.logs[name] }

func newTestModel(sup supervisorIface) *model {
	return &model{
		sup:    sup,
		width:  120,
		height: 40,
	}
}

var _ caddyProvider = (*stubCaddy)(nil)

type stubCaddy struct{}

func (s stubCaddy) ListenAddr() string { return "127.0.0.1:8443" }

func key(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// --- WindowSizeMsg ---

func TestUpdate_WindowSize(t *testing.T) {
	m := newTestModel(&stubSup{})
	m.width, m.height = 0, 0

	next, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	nm := next.(*model)
	if nm.width != 200 || nm.height != 50 {
		t.Errorf("got %dx%d, want 200x50", nm.width, nm.height)
	}
}

// --- tickMsg ---

func TestUpdate_Tick_FetchesStatus(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "api", State: "running", PID: 1}},
		logs:     map[string][]supervisor.LogEntry{"api": {{Line: "log line"}}},
	}
	m := newTestModel(stub)

	next, _ := m.Update(tickMsg(time.Now()))
	nm := next.(*model)

	if len(nm.statuses) != 1 {
		t.Errorf("statuses not updated: got %d", len(nm.statuses))
	}
}

// --- cursor navigation ---

func TestUpdate_CursorDown(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{
			{Name: "a"}, {Name: "b"}, {Name: "c"},
		},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses

	next, _ := m.Update(specialKey(tea.KeyDown))
	nm := next.(*model)
	if nm.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", nm.cursor)
	}
}

func TestUpdate_CursorUp_AtZero(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "a"}, {Name: "b"}},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses
	m.cursor = 0

	next, _ := m.Update(specialKey(tea.KeyUp))
	nm := next.(*model)
	if nm.cursor != 0 {
		t.Errorf("cursor: got %d, want 0 (can't go above 0)", nm.cursor)
	}
}

func TestUpdate_CursorDown_AtBottom(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "a"}, {Name: "b"}},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses
	m.cursor = 1

	next, _ := m.Update(specialKey(tea.KeyDown))
	nm := next.(*model)
	if nm.cursor != 1 {
		t.Errorf("cursor: got %d, want 1 (can't go past last)", nm.cursor)
	}
}

func TestUpdate_CursorChange_GoesToBottom(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "a"}, {Name: "b"}},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses

	next, _ := m.Update(specialKey(tea.KeyDown))
	nm := next.(*model)
	if !nm.vp.AtBottom() {
		t.Error("cursor change should reset viewport to bottom")
	}
}

// --- log scroll ---

func TestUpdate_LogScrollBack(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "api"}},
		logs: map[string][]supervisor.LogEntry{
			"api": make([]supervisor.LogEntry, 100), // 100 lines so there's room to scroll
		},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses
	m.vp.SetHeight(20)
	m.vp.SetContentLines(formatLogEntries(stub.logs["api"]))
	m.vp.GotoBottom()

	m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	if m.vp.AtBottom() {
		t.Error("viewport should have scrolled up from bottom")
	}
}

func TestUpdate_LogScrollForward_AtBottom(t *testing.T) {
	m := newTestModel(&stubSup{})
	// already at bottom (no content) → scrolling forward should be a no-op
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	if !m.vp.AtBottom() {
		t.Error("empty viewport should stay at bottom")
	}
}

// --- process actions ---

func TestUpdate_StartKey(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "api"}},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses

	m.Update(key('s'))
	if stub.started != "api" {
		t.Errorf("started: got %q, want api", stub.started)
	}
}

func TestUpdate_StopKey(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "api"}},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses

	m.Update(key('x'))
	if stub.stopped != "api" {
		t.Errorf("stopped: got %q, want api", stub.stopped)
	}
}

func TestUpdate_RestartKey(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "api"}},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses

	m.Update(key('r'))
	if stub.restarted != "api" {
		t.Errorf("restarted: got %q, want api", stub.restarted)
	}
}

func TestUpdate_QuitKey(t *testing.T) {
	m := newTestModel(&stubSup{})
	_, cmd := m.Update(key('q'))
	if cmd == nil {
		t.Error("q should return a quit command")
	}
}

func TestUpdate_FilterToggle(t *testing.T) {
	stub := &stubSup{
		statuses: []supervisor.ProcessStatus{{Name: "api"}},
		logs: map[string][]supervisor.LogEntry{
			"api": {
				{Source: "stdout", Line: "out"},
				{Source: "stderr", Line: "err"},
				{Source: "invincible", Line: "started"},
			},
		},
	}
	m := newTestModel(stub)
	m.statuses = stub.statuses

	if m.filterMode != filterAll {
		t.Error("initial filter should be ALL")
	}

	m.Update(key('f'))
	if m.filterMode != filterStderr {
		t.Errorf("after first f: got %d, want filterStderr", m.filterMode)
	}

	m.Update(key('f'))
	if m.filterMode != filterStdout {
		t.Errorf("after second f: got %d, want filterStdout", m.filterMode)
	}

	m.Update(key('f'))
	if m.filterMode != filterInvincible {
		t.Errorf("after third f: got %d, want filterInvincible", m.filterMode)
	}

	m.Update(key('f'))
	if m.filterMode != filterAll {
		t.Errorf("after fourth f: got %d, want filterAll", m.filterMode)
	}
}
