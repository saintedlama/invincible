package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Create a starter .invincible.toml in the current directory",
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const initTemplate = `[project]
name = "myapp"
# api_addr = ":7777"  # override the Invincible API port

# Each [[process]] block defines one managed service.
# Invincible finds a free port starting from the hint and injects it as PORT=<n>.
# Every process also receives <PEER_NAME>_PORT=<n> for each sibling that has a port.

[[process]]
name = "api"
cmd = "go run ./cmd/api"
port = 8080
# port_env = "PORT"       # env var name for this process's own port (default: PORT)
# restart_delay = "500ms" # wait before restarting a crashed process

[[process]]
name = "worker"
cmd = "go run ./cmd/worker"
# port omitted → Invincible assigns an arbitrary free port
# no_port = true          # uncomment to disable port assignment entirely
`

func runInit(_ *cobra.Command, _ []string) error {
	if _, err := os.Stat(".invincible.toml"); err == nil {
		return fmt.Errorf(".invincible.toml already exists in the current directory")
	}
	if err := os.WriteFile(".invincible.toml", []byte(initTemplate), 0644); err != nil {
		return fmt.Errorf("Error creating .invincible.toml: %w", err)
	}
	fmt.Println("created .invincible.toml")
	return nil
}
