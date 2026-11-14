//go:build !linux && !darwin

package instance

import (
	"errors"
	"net"
)

// peerUID fails closed where the peer's credentials are not read yet (Windows:
// SIO_AF_UNIX_GETPEERPID then the token's SID, at N35): no request is sent or read.
func peerUID(*net.UnixConn) (int, error) {
	return 0, errors.New("instance: peer credentials not supported on this system")
}
