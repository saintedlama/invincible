package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/saintedlama/invincible/internal/api"
	"github.com/saintedlama/invincible/internal/caddy"
	"github.com/saintedlama/invincible/internal/config"
	"github.com/saintedlama/invincible/internal/supervisor"
	"github.com/saintedlama/invincible/internal/tui"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "invincible",
	Short: "Local development process manager",
	Long: `Invincible keeps processes alive, restarts them on crash, assigns free
ports, and exposes an HTTP API for programmatic control.`,
	SilenceUsage: true,
	RunE:         runRoot,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = buildVersion()
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".invincible.toml", "path to config file")
	rootCmd.Flags().String("api-addr", "", "preferred API address (e.g. :7778); falls back to config api_addr, then :7777")
	rootCmd.Flags().Bool("no-tui", false, "run headless (no terminal UI)")
}

func runRoot(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	addr := ":7777"
	if cmd.Flags().Changed("api-addr") {
		addr, _ = cmd.Flags().GetString("api-addr")
	} else if cfg.Project.APIAddr != "" {
		addr = cfg.Project.APIAddr
	}

	sup := supervisor.New(cfg.Processes)

	srv, err := api.New(sup, addr)
	if err != nil {
		return err
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "api: %v\n", err)
		}
	}()

	go sup.StartAll()

	var caddyMgr *caddy.Manager
	if cfg.Caddy.Enabled {
		caddyMgr, err = caddy.New(cfg.Caddy, sup)
		if err != nil {
			fmt.Fprintf(os.Stderr, "caddy: %v\n", err)
		} else if err := caddyMgr.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "caddy: %v\n", err)
			caddyMgr = nil
		} else {
			go caddyMgr.Watch()
		}
	}

	noTUI, _ := cmd.Flags().GetBool("no-tui")
	if noTUI {
		fmt.Printf("http://%s\n", srv.Addr())
		if caddyMgr != nil {
			fmt.Printf("caddy: %s\n", caddyMgr.ListenAddr())
		}
		select {}
	}

	p := tui.New(sup, srv.Addr(), caddyMgr)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	sup.StopAll()
	if caddyMgr != nil {
		caddyMgr.Cleanup()
	}
	return nil
}
