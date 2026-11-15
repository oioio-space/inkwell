package sidecar

import (
	"slices"

	"golang.org/x/sys/unix"
)

// program describes one sandboxed program (ADR 0044 point 1f): what it may
// call and what inkwell guarantees it can do. The tier decision and every
// seccomp list are per program, so a new one (the plugin host of AD-57) is a
// new entry with its own measured lists, not a special case.
type program struct {
	// name is the manifest name [Store.Resolve] takes.
	name string
	// syscalls is the allow-list per GOARCH; an architecture without one
	// builds no filter.
	syscalls map[string][]uint32
	// proven lists the architectures whose inventory passed; decideTier
	// refuses the others (plan S-7).
	proven []string
	// prctl and ioctl are the only options and requests allowed (plan S-4).
	prctl, ioctl []uint32
	// perimeter names what tier F guarantees (ADR 0044 point 1b).
	perimeter []string
}

// The two sidecars share the trace-derived table of scripts/sidecar-allowlist.py
// today; their prctl and ioctl lists are those measured on the stock runners
// (gates/SB2/MEASUREMENTS.md, runs 3 and 4, Q13).
var (
	sidecarSyscalls = map[string][]uint32{"amd64": allowedSyscallsAMD64, "arm64": allowedSyscallsARM64}

	programs = []program{
		{
			name:      Typst,
			syscalls:  sidecarSyscalls,
			proven:    []string{"amd64"},
			ioctl:     []uint32{unix.TIOCGWINSZ},
			perimeter: []string{"compile-pdf", "compile-svg", "query", "eval"},
		},
		{
			name:      Tinymist,
			syscalls:  sidecarSyscalls,
			proven:    []string{"amd64"},
			prctl:     []uint32{unix.PR_SET_NAME},
			ioctl:     []uint32{unix.TIOCINQ, unix.TCGETS}, // TIOCINQ is FIONREAD
			perimeter: []string{"lsp"},
		},
	}
)

// probeName is the program the tier probe launches, as `--version` and under
// its own descriptor (ADR 0044 point 2): the one every compilation runs.
const probeName = Typst

// lookupProgram returns the registered descriptor named name.
func lookupProgram(name string) (program, bool) {
	i := slices.IndexFunc(programs, func(p program) bool { return p.name == name })
	if i < 0 {
		return program{}, false
	}
	return programs[i], true
}
