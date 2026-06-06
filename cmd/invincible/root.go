package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	"github.com/saintedlama/invincible/internal/api"
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

	// Config file watcher.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watchConfig(ctx, cfgFile, func() {
		fmt.Fprintln(os.Stderr, "invincible: config changed, reloading...")
		newCfg, err := config.Load(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invincible: reload error: %v\n", err)
			return
		}
		sup.Reload(newCfg.Processes)
	})

	// Handle Ctrl+C gracefully.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		sup.StopAll()
		os.Exit(0)
	}()

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

func watchConfig(ctx context.Context, path string, onChange func()) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invincible: file watcher: %v — config reload disabled\n", err)
		return
	}
	if err := w.Add(path); err != nil {
		fmt.Fprintf(os.Stderr, "invincible: watch %s: %v — config reload disabled\n", path, err)
		w.Close()
		return
	}
	defer w.Close()

	var debounce *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-w.Events:
			if !ok {
				return
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(200*time.Millisecond, onChange)
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "invincible: watcher: %v\n", err)
		}
	}
}
