package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/saintedlama/invincible/internal/api"
	"github.com/saintedlama/invincible/internal/config"
	"github.com/saintedlama/invincible/internal/supervisor"
	"github.com/saintedlama/invincible/internal/tui"
	"github.com/saintedlama/invincible/internal/watcher"
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

	// api-addr precedence: flag > config file (project.api_addr) > default :7777
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

	watcherCtx, watcherCancel := context.WithCancel(context.Background())
	defer watcherCancel()
	for _, p := range cfg.Processes {
		if len(p.Watch) == 0 || p.Build == "" {
			continue
		}
		proc := p
		name := proc.Name

		watchDirs := proc.Watch
		if proc.Cwd != "" {
			watchDirs = make([]string, len(proc.Watch))
			for i, d := range proc.Watch {
				watchDirs[i] = filepath.Join(proc.Cwd, d)
			}
		}

		onBuild := func(ctx context.Context) error {
			return sup.Build(proc.Name, proc.Build, proc.Cwd, ctx)
		}

		onRestart := func() error {
			return sup.Restart(name)
		}

		logFn := func(msg string) {
			sup.Log(name, msg)
		}

		debounce := proc.WatchDebounceDuration()
		w := watcher.New(watchDirs, proc.WatchInclude, proc.WatchExclude, debounce, onBuild, onRestart, logFn)
		go w.Run(watcherCtx)
	}

	noTUI, _ := cmd.Flags().GetBool("no-tui")
	if noTUI {
		fmt.Printf("http://%s\n", srv.Addr())
		select {}
	}

	p := tui.New(sup, srv.Addr())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	sup.StopAll()
	return nil
}
