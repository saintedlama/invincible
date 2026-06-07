// PROTOTYPE — watch + build support. Delete or promote after evaluation.
//
// State machine per process:
//
//	idle → (fs event) → debouncing → (timer fires) → building →
//	  build ok  → restarting → idle
//	  build fail → idle  (old binary keeps running)
package watcher

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounce = 500 * time.Millisecond

type Restarter interface {
	Restart(name string) error
}

type Watcher struct {
	name     string
	dirs     []string
	includes []string // file glob patterns; empty = all files
	build    string
	cwd      string
	sup      Restarter
	log      func(string)
}

func New(name string, dirs, includes []string, build, cwd string, sup Restarter, log func(string)) *Watcher {
	return &Watcher{name: name, dirs: dirs, includes: includes, build: build, cwd: cwd, sup: sup, log: log}
}

func (w *Watcher) Run(ctx context.Context) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.log("watch: failed to create watcher: " + err.Error())
		return
	}
	defer fsw.Close()

	for _, dir := range w.dirs {
		if err := fsw.Add(dir); err != nil {
			w.log("watch: cannot watch " + dir + ": " + err.Error())
		} else {
			w.log("watch: watching " + dir)
		}
	}

	var timer *time.Timer
	resetTimer := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, func() {
			w.buildAndRestart()
		})
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-fsw.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				if !w.matchesInclude(event.Name) {
					continue
				}
				w.log("watch: change detected in " + event.Name)
				resetTimer()
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			w.log("watch: " + err.Error())
		}
	}
}

func (w *Watcher) matchesInclude(name string) bool {
	if len(w.includes) == 0 {
		return true
	}
	for _, pattern := range w.includes {
		matched, err := filepath.Match(pattern, filepath.Base(name))
		if err == nil && matched {
			return true
		}
	}
	return false
}

func (w *Watcher) buildAndRestart() {
	w.log("build: running " + w.build)

	cmd := shellCommand(w.build)
	if w.cwd != "" {
		cmd.Dir = w.cwd
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			if line != "" {
				w.log("build: " + line)
			}
		}
		w.log("build: FAILED (" + err.Error() + ") — keeping current binary")
		return
	}

	w.log("build: ok — restarting " + w.name)
	if err := w.sup.Restart(w.name); err != nil {
		w.log("restart: " + err.Error())
	}
}
