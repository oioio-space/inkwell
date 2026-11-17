package sidecar

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// tierHelper runs the helper modes of the tier tests (ADR 0044).
func tierHelper(name, arg string) int {
	switch name {
	case "sb2-tree":
		return processTreeHelper()
	case "sb2-fork":
		return forkAttemptHelper()
	case "sb2-third":
		return thirdProgramHelper()
	case "sb2-clone3":
		return cloneThreeHelper()
	case "sb2-parent":
		return sb2Parent(arg)
	case "sb2-fake": // a faulty launcher, for the fail-closed handshake (plan T-4)
		return fakeLauncher(arg)
	case "sb2-fds", "sb2-attack":
		return sb2Helper(name, arg)
	case "target-sleep": // report readiness through the inherited stdout pipe
		fmt.Println("target", os.Getpid())
		time.Sleep(time.Hour)
	case "pidsleep": // write the pid to arg, then sleep
		if err := os.WriteFile(arg, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			fmt.Println("pidfile:", err)
			return 2
		}
		time.Sleep(time.Hour)
	case "sentinel": // the target started: create arg
		if err := os.WriteFile(arg, []byte("started"), 0o600); err != nil {
			fmt.Println("sentinel:", err)
			return 2
		}
	case "disarm": // try to disarm the parent-death signal (plan S-4)
		err := unix.Prctl(unix.PR_SET_PDEATHSIG, 0, 0, 0, 0)
		fmt.Println("disarm:", errnoText(err))
	case "read": // open arg for reading
		f, err := os.Open(arg) // #nosec G304 -- the test names the file.
		if err == nil {
			_ = f.Close()
		}
		fmt.Println("read:", errnoText(err))
	}
	return 0
}

func errnoText(err error) string {
	if err == nil {
		return "ok"
	}
	if e, ok := errors.AsType[unix.Errno](err); ok {
		return unix.ErrnoName(e)
	}
	return err.Error()
}

// fakeLauncher answers the handshake wrongly as fault says; it never runs a
// target, so any « proceed » it gets is written to stdout.
func fakeLauncher(fault string) int {
	fd, err := strconv.Atoi(os.Getenv(handshakeEnv))
	if err != nil {
		return 2
	}
	switch fault {
	case "no-report", "cancel-before-report":
	case "truncated":
		_, _ = unix.Write(fd, []byte("tier=F ns="))
		return 0
	case "invalid":
		_, _ = unix.Write(fd, []byte(strings.Repeat("x", 40)+"\n"))
	case "eof":
		return 0
	case "wrong-tier": // a well-formed report of tier N from a launcher asked for F
		ns, err := ownNamespaces()
		if err != nil {
			return 2
		}
		_, _ = unix.Write(fd, fmt.Appendf(nil, "tier=N ns=%s proc=true landlock=3 seccomp=2\n", strings.Join(ns[:], ",")))
	}
	var b [1]byte
	if n, _ := unix.Read(fd+1, b[:]); n == 1 {
		fmt.Println("proceed received")
	}
	time.Sleep(time.Hour)
	return 0
}
