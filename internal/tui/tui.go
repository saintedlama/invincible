package tui

import (
	"fmt"
	"slices"
	"strings"
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
	panelStyle      = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8"))
	detailPanelStyle = panelStyle.Padding(0, 1, 1, 1)
)

const (
	leftContentW = 28
	helpPanelH   = 3
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
	vp         viewport.Model
	width      int
	height     int
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
		if mouse.Y >= m.height-helpPanelH {
			return m, nil
		}
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
		} else {
			switch mouse.Button {
			case tea.MouseWheelUp:
				m.vp.ScrollUp(1)
			case tea.MouseWheelDown:
				m.vp.ScrollDown(1)
			}
		}

	case tea.KeyPressMsg:
		shift := msg.Mod&tea.ModShift != 0
		ctrl := msg.Mod&tea.ModCtrl != 0
		switch {
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
		case msg.Code == 's':
			if len(m.statuses) > 0 {
				m.sup.Start(m.statuses[m.cursor].Name) //nolint
			}
		case msg.Code == 'x':
			if len(m.statuses) > 0 {
				m.sup.Stop(m.statuses[m.cursor].Name) //nolint
			}
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

	mainH := m.height - helpPanelH
	rightPanelW := max(m.width-leftContentW, 1)

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderProcessPanel(leftContentW, mainH),
		m.renderDetailPanel(rightPanelW, mainH),
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		top,
		m.renderHelpPanel(m.width),
	)

	v := tea.NewView(body)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = m.windowTitle()
	return v
}

func (m *model) renderProcessPanel(contentW, totalH int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", lipgloss.NewStyle().Bold(true).Render("Processes"))
	for i, s := range m.statuses {
		dot := stateIndicator(s.State)
		name := s.Name
		if i == m.cursor {
			name = styleSelected.Render(name)
			fmt.Fprintf(&sb, " > %s %s\n", dot, name)
		} else {
			fmt.Fprintf(&sb, "   %s %s\n", dot, name)
		}
	}
	return panelStyle.Width(contentW).Height(totalH).Render(sb.String())
}

func (m *model) renderDetailPanel(contentW, totalH int) string {
	if len(m.statuses) == 0 {
		return detailPanelStyle.Width(contentW).Height(totalH).Render("")
	}
	s := m.statuses[m.cursor]

	infoContent := renderProcessInfo(s)
	infoLines := strings.Count(infoContent, "\n") + 1
	// content_lines + border_top(1) + border_bottom(1) + padding_bottom(1) = infoLines+3
	infoPanelRendered := infoLines + 3
	logsPanelH := totalH - infoPanelRendered

	infoPanel := detailPanelStyle.Width(contentW).Height(infoPanelRendered).Render(infoContent)
	logsPanel := m.renderLogsPanel(contentW, logsPanelH)

	return lipgloss.JoinVertical(lipgloss.Left, infoPanel, logsPanel)
}

func renderProcessInfo(s supervisor.ProcessStatus) string {
	var lines []string

	showUptime := !s.StartedAt.IsZero() && (s.State == "running" || s.State == "probing")

	nameLine := lipgloss.NewStyle().Bold(true).Render(s.Name)
	if showUptime {
		nameLine += " " + styleLabel.Render("(up "+time.Since(s.StartedAt).Truncate(time.Second).String()+")")
	}
	lines = append(lines, nameLine)
	lines = append(lines, "") // blank separator

	f := func(key, val string) string {
		return fmt.Sprintf("%s  %s", styleLabel.Render(fmt.Sprintf("%-5s", key)), val)
	}

	kv := func(key, val string) {
		lines = append(lines, f(key, val))
	}

	kv("cmd", s.Cmd)

	if s.Cwd != "" {
		kv("cwd", s.Cwd)
	}

	if s.Watching {
		kv("watch", "on")
	}

	stateStr := s.State
	if s.PID > 0 {
		stateStr = fmt.Sprintf("%s  (PID %d)", s.State, s.PID)
	}
	kv("state", stateStr)

	if s.Port > 0 || s.PortEnv != "" {
		var parts []string
		if s.Port > 0 {
			parts = append(parts, f("port", fmt.Sprintf("%d", s.Port)))
		}
		if s.PortEnv != "" {
			parts = append(parts, f("penv", s.PortEnv))
		}
		lines = append(lines, strings.Join(parts, "    "))
	}

	if s.Restarts > 0 {
		kv("crash", fmt.Sprintf("%d", s.Restarts))
	}

	if len(s.DependsOn) > 0 {
		kv("needs", strings.Join(s.DependsOn, ", "))
	}

	envKeys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		envKeys = append(envKeys, k)
	}
	slices.Sort(envKeys)
	for _, k := range envKeys {
		kv("env", k+"="+s.Env[k])
	}

	return strings.Join(lines, "\n")
}

func (m *model) renderLogsPanel(contentW, totalH int) string {
	// vp.View() returns exactly vpWidth × vpHeight chars (no trailing newline).
	// Panel content = title(1) + blank(1) + vp.View()(vpHeight lines) = vpHeight+2 lines.
	// Panel rendered = max(totalH, (vpHeight+2) + borders(2) + padBottom(1)) = totalH
	// when vpHeight = totalH - 5.
	vpHeight := max(totalH-5, 0)
	vpWidth := max(contentW-4, 0) // contentW - borders(2) - paddingLR(2)
	m.vp.SetWidth(vpWidth)
	m.vp.SetHeight(vpHeight)

	title := lipgloss.NewStyle().Bold(true).Render("Logs")
	if !m.vp.AtBottom() {
		title = styleScrolled.Render("Logs ↑")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", title)
	sb.WriteString(m.vp.View())

	return detailPanelStyle.Width(contentW).Height(totalH).Render(sb.String())
}

func (m *model) renderHelpPanel(contentW int) string {
	var parts []string
	if m.apiAddr != "" {
		parts = append(parts, styleAPI.Render("API http://"+m.apiAddr))
	}
	parts = append(parts, styleHelp.Render("↑/↓ navigate  s start  x stop  r restart  Shift+↑/↓ PgUp/PgDn scroll  q quit"))
	parts = append(parts, styleInvincible.Render("f filter ("+filterLabels[m.filterMode]+")"))
	return panelStyle.Width(contentW).Render(strings.Join(parts, "   "))
}

func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func stateIndicator(state string) string {
	switch state {
	case "running":
		return styleRunning.Render("●")
	case "starting", "probing":
		return styleStarting.Render("◌")
	case "crashed":
		return styleCrashed.Render("●")
	default:
		return styleStopped.Render("○")
	}
}

func formatLogEntries(entries []supervisor.LogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		ts := styleLabel.Render("[" + e.Time.Format("15:04:05") + "]")
		content := e.Line
		switch e.Source {
		case "stderr":
			content = styleStderr.Render(content)
		case "invincible":
			content = styleInvincible.Render(content)
		}
		out[i] = ts + " " + content
	}
	return out
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


