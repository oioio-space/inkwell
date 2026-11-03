package unix

func Exec(string, []string, []string) error { return nil }
func RawSyscall6(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) (uintptr, uintptr, error) {
	return 0, 0, nil
}

func ForkExec()     {}
func StartProcess() {}
func Syscall()      {}
func Syscall6()     {}
func RawSyscall()   {}
