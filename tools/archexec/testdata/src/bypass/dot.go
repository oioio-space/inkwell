package bypass

import . "syscall"

var (
	dotExec    = Exec    // want "forbidden launch"
	dotSyscall = Syscall // want "forbidden launch"
)
