package sidecar

import (
	"fmt"
	"syscall"
)

// selfExe is the running image itself, whatever became of its path (replaced or deleted by an
// upgrade): the kernel resolves it to the file this process executes.
const selfExe = "/proc/self/exe"

// ExecSelf replaces this process image by the running image itself, with the argument list argv
// and the environment env (rule N-12 of docs/plans/2026-11-14-pc1-project-change-by-restart.md).
// It returns only when it failed. It lives here because this package is the only one that starts
// a process (tools/archexec).
func ExecSelf(argv, env []string) error {
	// #nosec G204 -- the running image itself; the caller rebuilds the arguments from nothing.
	return fmt.Errorf("executing %s: %w", selfExe, syscall.Exec(selfExe, argv, env))
}
