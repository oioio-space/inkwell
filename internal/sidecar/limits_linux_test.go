package sidecar

import (
	"fmt"
	"syscall"
)

// reportLimits prints the address-space, file-size and core limits of this
// process (getrlimit: the Go runtime touches none of them).
func reportLimits() int {
	var as, fsize, core syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_AS, &as) != nil || syscall.Getrlimit(syscall.RLIMIT_FSIZE, &fsize) != nil || syscall.Getrlimit(syscall.RLIMIT_CORE, &core) != nil {
		return 2
	}
	fmt.Printf("as=%d fsize=%d core=%d/%d\n", as.Cur, fsize.Cur, core.Cur, core.Max)
	return 0
}

// A malicious child must not be able to raise the inherited ceiling.
func hardFileLimit() bool {
	var limit syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit) != nil || limit.Cur != limit.Max {
		return false
	}
	limit.Cur++
	limit.Max++
	return syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit) != nil
}
