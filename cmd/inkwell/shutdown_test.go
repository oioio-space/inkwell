package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestShutdownWaitsBeforeClosing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var loops shutdownLoops
		release := make(chan struct{})
		for range 2 {
			loops.Go("watch", func() { <-ctx.Done(); <-release })
		}
		var closed []string
		done := make(chan struct{})
		go func() {
			shutdown(cancel, &loops, func() error { closed = append(closed, "network"); return nil }, func() error { closed = append(closed, "project"); return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
			close(done)
		}()
		synctest.Wait()
		if ctx.Err() == nil {
			t.Fatal("shutdown did not cancel")
		}
		if len(closed) != 0 {
			t.Fatal("closed resources while loops were running")
		}
		close(release)
		<-done
		if len(closed) != 2 || closed[0] != "network" || closed[1] != "project" {
			t.Fatalf("close order = %v", closed)
		}
	})
}

func TestShutdownBoundsStuckLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var loops shutdownLoops
		release := make(chan struct{})
		loops.Go("bridge", func() { <-release })
		loops.Go("watch", func() {})
		synctest.Wait()
		defer close(release)
		var logs bytes.Buffer
		closed := 0
		start := time.Now()
		shutdown(func() {}, &loops, func() error { closed++; return nil }, func() error { closed++; return nil }, slog.New(slog.NewTextHandler(&logs, nil)))
		if elapsed := time.Since(start); elapsed != 5*time.Second {
			t.Errorf("wait = %v, want 5s", elapsed)
		}
		if closed != 2 {
			t.Errorf("closed %d resources", closed)
		}
		if !strings.Contains(logs.String(), "loop=bridge") || strings.Contains(logs.String(), "loop=watch") {
			t.Errorf("timeout logs: %s", &logs)
		}
	})
}
