package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/saintedlama/invincible/internal/supervisor"
)

func (m *model) renderInfoPanel(contentW, totalH int) string {
	if len(m.statuses) == 0 {
		return detailPanelStyle.Width(contentW).Height(totalH).Render("")
	}
	s := m.statuses[m.cursor]
	return detailPanelStyle.Width(contentW).Height(totalH).Render(renderProcessInfo(s))
}

func renderProcessInfo(s supervisor.ProcessStatus) string {
	var lines []string

	showUptime := !s.StartedAt.IsZero() && (s.State == "running" || s.State == "probing")

	nameLine := lipgloss.NewStyle().Bold(true).Render(s.Name)
	if showUptime {
		nameLine += " " + styleLabel.Render("(up "+time.Since(s.StartedAt).Truncate(time.Second).String()+")")
	}
	lines = append(lines, nameLine)
	lines = append(lines, "")

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
