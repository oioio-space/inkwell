package sidecar

import (
	"fmt"
	"os"
	"strconv"
)

const shell = "/bin/sh"

// withLimits wraps the command so that the limits are in force before the
// program's first instruction: a shell sets them with ulimit, then execs the
// program, which keeps its pid (and so Pdeathsig and the process group).
// Setting them on a process that is already running leaves a window in which a
// hostile document could already allocate or write past them.
//
// ulimit -v counts KiB. ulimit -f counts blocks of 512 bytes, in dash and in
// bash in sh mode alike (POSIX), so the file size limit is exact; the directory
// watch of [Spec.MaxDirBytes] bounds the total.
func withLimits(spec Spec) (path string, args []string, err error) {
	if spec.MemoryLimit == 0 && spec.FileSizeLimit == 0 {
		return spec.Path, spec.Args, nil
	}
	if _, err := os.Stat(shell); err != nil {
		return "", nil, fmt.Errorf("sidecar: limits need %s: %w", shell, err)
	}
	script := ""
	if spec.MemoryLimit > 0 {
		script += "ulimit -v " + strconv.FormatUint((spec.MemoryLimit+1023)/1024, 10) + " || exit 126\n"
	}
	if spec.FileSizeLimit > 0 {
		script += "ulimit -f " + strconv.FormatUint((spec.FileSizeLimit+511)/512, 10) + " || exit 126\n"
	}
	script += `exec "$@"`
	return shell, append([]string{"-c", script, "inkwell-sidecar", spec.Path}, spec.Args...), nil
}
