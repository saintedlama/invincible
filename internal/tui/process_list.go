package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m *model) renderProcessPanel(contentW, totalH int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", lipgloss.NewStyle().Bold(true).Render("Processes"))
	for i, s := range m.statuses {
		dot := stateIndicator(s.State)
		name := s.Name
		prefix := "   "
		if i == m.cursor {
			name = styleSelected.Render(name)
			prefix = " > "
		}
		port := ""
		if s.Port > 0 {
			port = " " + styleInfo.Render(fmt.Sprintf(":%d", s.Port))
		}
		fmt.Fprintf(&sb, "%s%s %s%s\n", prefix, dot, name, port)
	}
	return panelStyle.Width(contentW).Height(totalH).Render(sb.String())
}

func stateIndicator(state string) string {
	switch state {
	case "running":
		return styleRunning.Render("●")
	case "building":
		return styleStarting.Render("◎")
	case "starting", "probing":
		return styleStarting.Render("◌")
	case "crashed":
		return styleCrashed.Render("●")
	default:
		return styleStopped.Render("○")
	}
}
