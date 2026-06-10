package watcher

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	dirs      []string
	include   []string
	exclude   []string
	debounce  time.Duration
	onBuild   func(context.Context) error
	onRestart func() error
	log       func(string)
}

func New(dirs, include, exclude []string, debounce time.Duration, onBuild func(context.Context) error, onRestart func() error, log func(string)) *Watcher {
	return &Watcher{
		dirs:      dirs,
		include:   include,
		exclude:   exclude,
		debounce:  debounce,
		onBuild:   onBuild,
		onRestart: onRestart,
		log:       log,
	}
}

func (w *Watcher) Run(ctx context.Context) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.log("watch: failed to create watcher: " + err.Error())
		return
	}
	defer fsw.Close()

	for _, dir := range w.dirs {
		if err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if w.isExcludedDir(p) {
				return filepath.SkipDir
			}
			if err := fsw.Add(p); err != nil {
				w.log("watch: cannot watch " + p + ": " + err.Error())
			}
			return nil
		}); err != nil {
			w.log("watch: walk error in " + dir + ": " + err.Error())
		}
	}

	var timer *time.Timer
	var timerC <-chan time.Time
	var buildCancel context.CancelFunc

	cancelBuild := func() {
		if buildCancel != nil {
			buildCancel()
			buildCancel = nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			cancelBuild()
			return
		case <-timerC:
			timer = nil
			buildCtx, cancel := context.WithCancel(ctx)
			buildCancel = cancel

			if err := w.onBuild(buildCtx); err != nil {
				if buildCancel != nil {
					w.log("build: FAILED — " + err.Error())
				}
			} else {
				w.log("build: ok — restarting")
				if err := w.onRestart(); err != nil {
					w.log("restart: " + err.Error())
				}
			}
			buildCancel = nil

		case event, ok := <-fsw.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if !w.isExcludedDir(event.Name) {
						fsw.Add(event.Name) //nolint
					}
				}
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) {
				continue
			}
			if !w.matchesInclude(event.Name) {
				continue
			}

			if buildCancel != nil {
				buildCancel()
				buildCancel = nil
				w.log("watch: build cancelled")
			}

			if timer == nil {
				timer = time.NewTimer(w.debounce)
				timerC = timer.C
			} else {
				timer.Reset(w.debounce)
			}
			w.log("watch: change detected in " + event.Name)

		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			w.log("watch: " + err.Error())
		}
	}
}

func (w *Watcher) isExcludedDir(path string) bool {
	name := filepath.Base(path)
	for _, e := range w.exclude {
		if matched, _ := filepath.Match(e, name); matched {
			return true
		}
	}
	return false
}

func (w *Watcher) matchesInclude(name string) bool {
	if len(w.include) == 0 {
		return true
	}
	base := filepath.Base(name)
	for _, pattern := range w.include {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}
