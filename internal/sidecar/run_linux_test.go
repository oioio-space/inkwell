package sidecar

import (
	"strings"
	"testing"
)

// SIDE-19: the limit is on the child's address space, so an allocation that
// would succeed in the child without it fails with it; Inkwell is untouched.
func TestRunMemoryLimit(t *testing.T) {
	s := spec("alloc:1536")
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 {
		t.Fatalf("without a limit: Run = %v exit %d stderr %q, want success", err, out.ExitCode, out.Stderr)
	}

	s.MemoryLimit = 1 << 30 // 1 GiB of address space cannot hold 1.5 GiB
	out, err = Run(t.Context(), s)
	if err != nil {
		t.Fatalf("with a limit: Run = %v, want the failure in Output", err)
	}
	if out.ExitCode == 0 {
		t.Errorf("a 1.5 GiB allocation succeeded under a 1 GiB limit: stdout %q", out.Stdout)
	}
	if !strings.Contains(string(out.Stderr), "out of memory") {
		t.Logf("child stderr: %.200s", out.Stderr) // the wording belongs to the runtime
	}
}
