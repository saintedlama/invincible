package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/saintedlama/invincible/internal/supervisor"
)

type supervisorIface interface {
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	StartAll()
	StopAll()
	RestartAll()
	Status() []supervisor.ProcessStatus
	Logs(name string, n int) []supervisor.LogEntry
}

var (
	styleRunning    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleStopped    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleCrashed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleStarting   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleSelected   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleHelp       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleScrolled   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleStderr     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleInvincible = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleAPI        = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleLabel      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleInfo       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	panelStyle      = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8"))
	detailPanelStyle = panelStyle.Padding(0, 1, 1, 1)
)

const (
	leftContentW = 28
)

const (
	screenDashboard = iota
	screenLogs
)

const (
	filterAll = iota
	filterStderr
	filterStdout
	filterInvincible
)

var filterLabels = [...]string{"ALL", "STDERR", "STDOUT", "INVINCIBLE"}

type tickMsg time.Time

type model struct {
	sup        supervisorIface
	apiAddr    string
	statuses   []supervisor.ProcessStatus
	cursor     int
	filterMode int
	screen     int
	vp         viewport.Model
	width      int
	height     int
	helpH      int
	lastWheel  time.Time
}

func New(sup supervisorIface, apiAddr string) *tea.Program {
	m := &model{
		sup:     sup,
		apiAddr: apiAddr,
		vp:      viewport.New(),
	}
	return tea.NewProgram(m)
}

func (m *model) Init() tea.Cmd {
	return tick()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.statuses = m.sup.Status()
		if m.cursor >= len(m.statuses) {
			m.cursor = max(len(m.statuses)-1, 0)
		}
		if len(m.statuses) > 0 {
			atBottom := m.vp.AtBottom()
			m.loadLogs()
			if atBottom {
				m.vp.GotoBottom()
			}
		}
		return m, tick()

	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		if time.Since(m.lastWheel) < 50*time.Millisecond {
			return m, nil
		}
		m.lastWheel = time.Now()
		if mouse.Y >= m.height-m.helpH {
			return m, nil
		}

		switch m.screen {
		case screenLogs:
			switch mouse.Button {
			case tea.MouseWheelUp:
				m.vp.ScrollUp(1)
			case tea.MouseWheelDown:
				m.vp.ScrollDown(1)
			}

		default:
			if mouse.X < leftContentW {
				if mouse.Button == tea.MouseWheelUp && m.cursor > 0 {
					m.cursor--
					m.loadLogs()
					m.vp.GotoBottom()
				} else if mouse.Button == tea.MouseWheelDown && m.cursor < len(m.statuses)-1 {
					m.cursor++
					m.loadLogs()
					m.vp.GotoBottom()
				}
			}
		}

	case tea.KeyPressMsg:
		shift := msg.Mod&tea.ModShift != 0
		ctrl := msg.Mod&tea.ModCtrl != 0
		switch {
		case msg.Code == tea.KeyTab:
			m.screen = (m.screen + 1) % 2

		case shift && msg.Code == tea.KeyUp:
			m.vp.ScrollUp(1)
		case shift && msg.Code == tea.KeyDown:
			m.vp.ScrollDown(1)
		case msg.Code == tea.KeyPgUp:
			m.vp.PageUp()
		case msg.Code == tea.KeyPgDown:
			m.vp.PageDown()

		case msg.Code == 'q' || (ctrl && msg.Code == 'c'):
			return m, tea.Quit

		case msg.Code == tea.KeyUp || msg.Code == 'k':
			if m.cursor > 0 {
				m.cursor--
				m.loadLogs()
				m.vp.GotoBottom()
			}
		case msg.Code == tea.KeyDown || msg.Code == 'j':
			if m.cursor < len(m.statuses)-1 {
				m.cursor++
				m.loadLogs()
				m.vp.GotoBottom()
			}

		case shift && msg.Code == 's':
			m.sup.StartAll()
		case msg.Code == 's':
			if len(m.statuses) > 0 {
				m.sup.Start(m.statuses[m.cursor].Name) //nolint
			}

		case shift && msg.Code == 'x':
			m.sup.StopAll()
		case msg.Code == 'x':
			if len(m.statuses) > 0 {
				m.sup.Stop(m.statuses[m.cursor].Name) //nolint
			}

		case shift && msg.Code == 'r':
			m.sup.RestartAll()
		case msg.Code == 'r':
			if len(m.statuses) > 0 {
				m.sup.Restart(m.statuses[m.cursor].Name) //nolint
			}

		case msg.Code == 'f':
			m.filterMode = (m.filterMode + 1) % len(filterLabels)
			m.loadLogs()
			m.vp.GotoBottom()
		}
	}
	return m, nil
}

func (m *model) loadLogs() {
	if m.cursor < len(m.statuses) {
		m.vp.SetContentLines(formatLogEntries(m.filteredLogs()))
	}
}

func (m *model) filteredLogs() []supervisor.LogEntry {
	entries := m.sup.Logs(m.statuses[m.cursor].Name, 500)
	if m.filterMode == filterAll {
		return entries
	}
	filtered := make([]supervisor.LogEntry, 0, len(entries))
	for _, e := range entries {
		keep := false
		switch m.filterMode {
		case filterStderr:
			keep = e.Source == "stderr"
		case filterStdout:
			keep = e.Source == "stdout"
		case filterInvincible:
			keep = e.Source == "invincible"
		}
		if keep {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (m *model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("loading...")
	}

	helpBar := m.renderHelpBar()
	m.helpH = lipgloss.Height(helpBar)
	mainH := m.height - m.helpH

	var body string
	switch m.screen {
	case screenLogs:
		body = m.renderLogsScreen(mainH, helpBar)
	default:
		body = m.renderDashboard(mainH, helpBar)
	}

	v := tea.NewView(body)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = m.windowTitle()
	return v
}

func (m *model) renderDashboard(mainH int, helpBar string) string {
	rightPanelW := max(m.width-leftContentW, 1)

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderProcessPanel(leftContentW, mainH),
		m.renderInfoPanel(rightPanelW, mainH),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		top,
		helpBar,
	)
}

func (m *model) renderLogsScreen(mainH int, helpBar string) string {
	logsPanel := m.renderLogsPanel(m.width, mainH)
	return lipgloss.JoinVertical(lipgloss.Left,
		logsPanel,
		helpBar,
	)
}

func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) windowTitle() string {
	total := len(m.statuses)
	running := 0
	for _, s := range m.statuses {
		if s.State == "running" {
			running++
		}
	}
	return fmt.Sprintf("%d/%d invincible", running, total)
}
