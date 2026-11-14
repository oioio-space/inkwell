package instance

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerUID returns the user of the process at the other end of c, as the kernel recorded it when
// the connection was made (SO_PEERCRED).
func peerUID(c *net.UnixConn) (int, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("instance: peer credentials: %w", err)
	}
	var cred *unix.Ucred
	var cerr error
	if err := raw.Control(func(fd uintptr) {
		cred, cerr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, fmt.Errorf("instance: peer credentials: %w", err)
	}
	if cerr != nil {
		return 0, fmt.Errorf("instance: peer credentials: %w", cerr)
	}
	return int(cred.Uid), nil
}
