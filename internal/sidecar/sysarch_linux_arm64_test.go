package sidecar

// sysForkNumbers: arm64 has no fork or vfork system call.
func sysForkNumbers() (fork, vfork uint32) { return noSyscall, noSyscall }
