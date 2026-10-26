package sidecar

import (
	"fmt"
	"syscall"
)

// reportLimits prints the address-space and file-size limits of this process.
func reportLimits() int {
	var as, fsize syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_AS, &as) != nil || syscall.Getrlimit(syscall.RLIMIT_FSIZE, &fsize) != nil {
		return 2
	}
	fmt.Printf("as=%d fsize=%d\n", as.Cur, fsize.Cur)
	return 0
}
