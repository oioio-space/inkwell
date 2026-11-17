package sidecar

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// userNamespaceLimit reads user.max_user_namespaces; tests replace it.
var userNamespaceLimit = func() (int, error) {
	data, err := os.ReadFile("/proc/sys/user/max_user_namespaces")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// launchOutcome is the result of one real probe launch.
type launchOutcome uint8

const (
	launchOK launchOutcome = iota + 1
	launchRefused
	launchInconclusive
)

func (o launchOutcome) String() string {
	return [...]string{"?", "ok", "refused", "inconclusive"}[o]
}

// tierFacts are the facts of ADR 0044 point 2, gathered once per process by
// the probe sequence.
type tierFacts struct {
	Arch     string        // GOARCH
	Landlock int           // ABI number; 0 when unavailable
	Seccomp  bool          // the real filter installed in the probe child
	NS       launchOutcome // the real N launch of `typst --version`
	F        launchOutcome // the real F launch of the same target
	NSErr    error         // why N was refused or inconclusive, reported
	FErr     error         // why F failed, named in the refusal
	// Protected marks a launch for a protected project, refused in F
	// (ADR 0044 point 1e): a property of the launch, not of the machine.
	Protected bool
}

// minLandlockABI is the lowest ABI either tier accepts (truncate rights).
const minLandlockABI = 3

// decideTier is ADR 0044 point 2's table, total over its facts. A refusal
// wraps [ErrSandboxRefused]; a retry wraps [ErrProbeInconclusive] and must not
// be cached. A retry names its cause as text ([causeText]).
func decideTier(p program, f tierFacts) (tier, error) {
	refuse := func(reason string, args ...any) (tier, error) {
		return tierNone, fmt.Errorf("%w: %s: %s", ErrSandboxRefused, p.name, fmt.Sprintf(reason, args...))
	}
	switch {
	case !slices.Contains(p.proven, f.Arch):
		return refuse("not yet proven on %s", f.Arch)
	case f.Landlock < minLandlockABI:
		return refuse("mandatory Landlock unavailable (ABI %d)", f.Landlock)
	case !f.Seccomp:
		return refuse("mandatory seccomp unavailable")
	case f.NS == launchOK:
		return tierN, nil
	case f.NS != launchRefused:
		return tierNone, fmt.Errorf("%w: namespace launch %v: %s", ErrProbeInconclusive, f.NS, causeText(f.NSErr))
	case f.F == launchOK && f.Protected:
		return tierNone, fmt.Errorf("%w: %s", ErrProtectedRefused, p.name)
	case f.F == launchOK:
		return tierF, nil
	case f.F == launchRefused:
		return tierNone, fmt.Errorf("%w: %s: the launcher cannot run here: %w", ErrSandboxRefused, p.name, f.FErr)
	default:
		return tierNone, fmt.Errorf("%w: namespace-free launch %v: %s", ErrProbeInconclusive, f.F, causeText(f.FErr))
	}
}

// causeText is the cause of an inconclusive launch as text, never wrapped: the
// probe's own timeout or cancellation must not read as the caller's.
func causeText(err error) string {
	if err == nil {
		return "no cause reported"
	}
	return err.Error()
}

// namespaceErrnoClass classes an errno of the namespace launch (clone,
// unshare, mount, the id-map writes, the launcher's execve): EPERM, EACCES,
// EINVAL, and ENOSPC when user.max_user_namespaces is 0, are a refusal;
// every other errno is inconclusive. EACCES counts as a refusal only because
// the F launch follows: a launcher that cannot run at all fails both.
func namespaceErrnoClass(e unix.Errno, userNamespacesDisabled bool) launchOutcome {
	switch e {
	case unix.EPERM, unix.EACCES, unix.EINVAL:
		return launchRefused
	case unix.ENOSPC:
		if userNamespacesDisabled {
			return launchRefused
		}
	}
	return launchInconclusive
}

// namespaceOutcome classes the error of a namespace launch: nil is a success,
// an errno anywhere in the chain follows namespaceErrnoClass, anything else
// (a timeout, a cancellation, an exit status, a crash) is inconclusive.
func namespaceOutcome(err error) launchOutcome {
	if err == nil {
		return launchOK
	}
	e, ok := errors.AsType[unix.Errno](err)
	if !ok {
		return launchInconclusive
	}
	disabled := false
	if e == unix.ENOSPC {
		limit, readErr := userNamespaceLimit()
		disabled = readErr == nil && limit == 0
	}
	return namespaceErrnoClass(e, disabled)
}
