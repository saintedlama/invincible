package caddy

import (
	"fmt"
	"strings"
)

func (m *Manager) generateCaddyfile() string {
	port := m.caddyPort
	var sb strings.Builder
	fmt.Fprintf(&sb, ":%d {\n", port)

	for _, p := range m.sup.Status() {
		if p.Port == 0 {
			continue
		}
		fmt.Fprintf(&sb, "    handle_path /%s/* {\n", p.Name)
		fmt.Fprintf(&sb, "        reverse_proxy localhost:%d\n", p.Port)
		fmt.Fprintf(&sb, "    }\n")
	}
	fmt.Fprintf(&sb, "}\n")
	return sb.String()
}
