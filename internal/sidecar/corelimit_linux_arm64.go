package sidecar

// arm64 has no legacy setrlimit syscall. Unknown numbers are denied.
const setrlimitSyscall = ^uint32(0)
