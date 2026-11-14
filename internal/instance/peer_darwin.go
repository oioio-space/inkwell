package instance

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerUID returns the user of the process at the other end of c (LOCAL_PEERCRED at level
// SOL_LOCAL). Built and tested at N35.
func peerUID(c *net.UnixConn) (int, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("instance: peer credentials: %w", err)
	}
	var cred *unix.Xucred
	var cerr error
	if err := raw.Control(func(fd uintptr) {
		cred, cerr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	}); err != nil {
		return 0, fmt.Errorf("instance: peer credentials: %w", err)
	}
	if cerr != nil {
		return 0, fmt.Errorf("instance: peer credentials: %w", cerr)
	}
	return int(cred.Uid), nil
}
