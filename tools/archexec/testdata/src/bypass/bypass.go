package bypass

// A comment above the import must not hide its identity.
import (
	o "os"
	x "os/exec" // want "forbidden import"
	s "syscall"
	_ "unsafe"

	u "golang.org/x/sys/unix"
)

var (
	a = o.StartProcess // want "forbidden launch"
	b = s.Exec         // want "forbidden launch"
	c = s.ForkExec     // want "forbidden launch"
	d = s.StartProcess // want "forbidden launch"
	e = x.Command
	f = u.Exec        // want "forbidden launch"
	g = u.RawSyscall6 // want "forbidden launch"
	h = s.RawSyscall  // want "forbidden launch"
)

func raw() { s.Syscall(59, 0, 0, 0) } // want "forbidden launch"

// want +2 "forbidden go:linkname"
//
//go:linkname hidden golang.org/x/sys/unix.Exec
func hidden()

// Taking method values and aliased syscall references cannot hide launches.
var (
	commandMethod    = (&x.Cmd{}).Run
	syscall6         = s.Syscall6     // want "forbidden launch"
	rawSyscall6      = s.RawSyscall6  // want "forbidden launch"
	unixForkExec     = u.ForkExec     // want "forbidden launch"
	unixStartProcess = u.StartProcess // want "forbidden launch"
	unixSyscall      = u.Syscall      // want "forbidden launch"
	unixSyscall6     = u.Syscall6     // want "forbidden launch"
	unixRawSyscall   = u.RawSyscall   // want "forbidden launch"
)
