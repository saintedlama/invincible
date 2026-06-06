//go:build !windows

package caddy

import (
	"fmt"
	"strings"
)

func (m *Manager) generateCaddyfile() string {
	var sb strings.Builder
	for _, p := range m.sup.Status() {
		if p.Port == 0 {
			continue
		}
		fmt.Fprintf(&sb, "%s.localhost {\n", p.Name)
		fmt.Fprintf(&sb, "    reverse_proxy localhost:%d\n", p.Port)
		fmt.Fprintf(&sb, "}\n\n")
	}
	return sb.String()
}
