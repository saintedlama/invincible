package tui

import (
	"strings"
)

func (m *model) renderHelpBar() string {
	var parts []string
	if m.apiAddr != "" {
		parts = append(parts, styleAPI.Render(m.apiAddr))
	}
	parts = append(parts, styleHelp.Render("↑/↓ sel  s/x/r  S/X/R all"))

	parts = append(parts, styleHelp.Render("p processes  l logs"))

	parts = append(parts, styleInvincible.Render("f:"+filterLabels[m.filterMode]))
	parts = append(parts, styleHelp.Render("q quit"))

	return panelStyle.Width(m.width).Render(strings.Join(parts, "  "))
}
