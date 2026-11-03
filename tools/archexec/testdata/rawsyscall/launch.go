package fixture

import "syscall"

func Launch() {
	syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	syscall.Syscall(syscall.SYS_EXECVE, 0, 0, 0)
}
