package main

import (
	"bytes"
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
