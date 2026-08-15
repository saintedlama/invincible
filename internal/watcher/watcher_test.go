package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestRun_RestartsWithoutBuild verifies that a nil onBuild callback (no
// "build" configured for the process) restarts the process directly on file
// change instead of attempting to run a build step.
func TestRun_RestartsWithoutBuild(t *testing.T) {
	dir := t.TempDir()

	var buildCalled atomic.Bool
	restartedCh := make(chan struct{}, 1)

	w := New([]string{dir}, nil, nil, 20*time.Millisecond, nil, func() error {
		select {
		case restartedCh <- struct{}{}:
		default:
		}
		return nil
	}, func(string) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	time.Sleep(50 * time.Millisecond) // let the initial directory walk register watches

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-restartedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("restart was not triggered")
	}

	if buildCalled.Load() {
		t.Error("build should not be called when onBuild is nil")
	}
}

// TestRun_BuildsThenRestarts verifies that a configured onBuild callback
// still runs before the restart when one is provided.
func TestRun_BuildsThenRestarts(t *testing.T) {
	dir := t.TempDir()

	var buildCalled atomic.Bool
	restartedCh := make(chan struct{}, 1)

	onBuild := func(context.Context) error {
		buildCalled.Store(true)
		return nil
	}
	onRestart := func() error {
		select {
		case restartedCh <- struct{}{}:
		default:
		}
		return nil
	}

	w := New([]string{dir}, nil, nil, 20*time.Millisecond, onBuild, onRestart, func(string) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-restartedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("restart was not triggered")
	}

	if !buildCalled.Load() {
		t.Error("build should be called when onBuild is set")
	}
}
