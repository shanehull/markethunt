package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWatchParentStopsWhenReparented(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	getppid := func() int {
		calls++
		if calls == 1 {
			return 42
		}
		return 1 // reparented to init
	}
	stopped := make(chan struct{})
	done := make(chan struct{})

	go func() {
		watchParent(ctx, discardLogger(), 5*time.Millisecond, getppid, func() { close(stopped) })
		close(done)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog did not fire after the parent changed")
	}
	<-done
}

func TestWatchParentSkipsInit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopped := make(chan struct{})
	done := make(chan struct{})
	go func() {
		watchParent(ctx, discardLogger(), 5*time.Millisecond, func() int { return 1 }, func() { close(stopped) })
		close(done)
	}()

	select {
	case <-done: // returned immediately, as intended
	case <-time.After(time.Second):
		t.Fatal("watchdog should return immediately for PID 1")
	}
	select {
	case <-stopped:
		t.Fatal("watchdog must not stop a process launched by init")
	default:
	}
}
