package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

func TestResolveTargets(t *testing.T) {
	m, err := sidecar.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	all, err := resolveTargets(m, "all")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"}; !slices.Equal(all, want) {
		t.Errorf("resolveTargets(all) = %v, want %v", all, want)
	}
	one, err := resolveTargets(m, "linux_arm64,linux_amd64")
	if err != nil || !slices.Equal(one, []string{"linux_arm64", "linux_amd64"}) {
		t.Errorf("resolveTargets(list) = %v, %v", one, err)
	}
	if _, err := resolveTargets(m, "plan9_mips"); err == nil || !strings.Contains(err.Error(), "plan9_mips") {
		t.Errorf("resolveTargets(plan9_mips) = %v, want an error naming it", err)
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(t.Context(), []string{"-target", "plan9_mips"}, &stdout, &stderr); err == nil {
		t.Error("run(-target plan9_mips) = nil, want an error")
	}
	if err := run(t.Context(), []string{"-nope"}, &stdout, &stderr); err == nil {
		t.Error("run(-nope) = nil, want an error")
	}
}

func TestRunHelp(t *testing.T) {
	var stderr bytes.Buffer
	if err := run(t.Context(), []string{"-h"}, &bytes.Buffer{}, &stderr); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("run(-h) = %v, want flag.ErrHelp", err)
	}
	for _, name := range []string{"-target", "-out", "-cache"} {
		if !strings.Contains(stderr.String(), name) {
			t.Errorf("usage lacks %s:\n%s", name, &stderr)
		}
	}
}

func TestRunNeedsACacheDirectory(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	err := run(t.Context(), []string{"-out", t.TempDir()}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "user cache dir") {
		t.Errorf("run without a cache directory = %v, want a user cache dir error", err)
	}
}

func TestRunStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("HOME", cacheHome) // macOS reads $HOME/Library/Caches
	out := t.TempDir()
	err := run(ctx, []string{"-target", "linux_amd64,linux_arm64", "-out", out}, &bytes.Buffer{}, &bytes.Buffer{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run on a cancelled context = %v, want context.Canceled", err)
	}
	if entries, err := os.ReadDir(out); err != nil || len(entries) != 0 {
		t.Errorf("out directory = %v, %v, want it left empty", entries, err)
	}
}

func TestRunReportsFetchFailure(t *testing.T) {
	// A cache that cannot be a directory: nothing is downloaded, the error says so.
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-target", "linux_amd64", "-out", t.TempDir(), "-cache", filepath.Join(blocker, "dl")}
	err := run(t.Context(), args, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "linux_amd64") {
		t.Errorf("run with an unusable cache = %v, want an error naming the target", err)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"help", []string{"-h"}, 1, "-target"},
		{"unknown target", []string{"-target", "plan9_mips"}, 1, "sidecar-fetch: no pinned build for plan9_mips"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := exitCode(tc.args, &stdout, &stderr); got != tc.code {
				t.Errorf("exitCode(%v) = %d, want %d", tc.args, got, tc.code)
			}
			if !strings.Contains(stderr.String(), tc.msg) {
				t.Errorf("stderr lacks %q:\n%s", tc.msg, &stderr)
			}
		})
	}
}
