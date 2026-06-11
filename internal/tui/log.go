package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/saintedlama/invincible/internal/supervisor"
)

func (m *model) renderLogsPanel(contentW, totalH int) string {
	vpHeight := max(totalH-5, 0)
	vpWidth := max(contentW-4, 0)
	m.vp.SetWidth(vpWidth)
	m.vp.SetHeight(vpHeight)

	title := lipgloss.NewStyle().Bold(true).Render("Logs")
	if len(m.statuses) > 0 && m.cursor < len(m.statuses) {
		s := m.statuses[m.cursor]
		title = lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Logs - %s (%s)", s.Name, s.State))
	}
	if !m.vp.AtBottom() {
		title = styleScrolled.Render(title + " ↑")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", title)
	sb.WriteString(m.vp.View())

	return detailPanelStyle.Width(contentW).Height(totalH).Render(sb.String())
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
