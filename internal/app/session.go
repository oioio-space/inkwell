// Package app wires inkwell's services for the desktop shell. It is the only
// package that knows every other one (docs/architecture.md).
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// joinTimeout bounds how long Join waits for the remote handshake.
const joinTimeout = 10 * time.Second

// Network is what a Session needs from the transport: an identity, a way to
// accept peers and a way to reach one.
type Network interface {
	Local() transport.PeerID
	Listen(ctx context.Context, addr string) (net.Addr, error)
	Dial(ctx context.Context, addr string) (transport.PeerID, error)
	Peers() []transport.PeerID
}

// Session is the Wails service behind "Partager": host this machine or join
// another one. Its exported methods are bound to the frontend.
type Session struct {
	net   Network
	addrs []string // set once hosting
	mu    sync.Mutex
}

// NewSession returns a session over n.
func NewSession(n Network) *Session { return &Session{net: n} }

// LocalPeer returns this machine's peer identity.
func (s *Session) LocalPeer() string { return string(s.net.Local()) }

// Peers returns the connected peers, sorted.
func (s *Session) Peers() []string {
	peers := s.net.Peers()
	out := make([]string, len(peers))
	for i, p := range peers {
		out[i] = string(p)
	}
	return out
}

// Host starts accepting peers on every interface at port (0 picks a free one)
// and returns the "ip:port" addresses others can join, LAN addresses first.
// Hosting again returns the same addresses. Listening lasts until the
// network is closed, not just for this call.
func (s *Session) Host(ctx context.Context, port int) ([]string, error) {
	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("port %d out of range", port)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.addrs != nil {
		return slices.Clone(s.addrs), nil
	}
	ln, err := s.net.Listen(context.WithoutCancel(ctx), net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}
	ap, err := netip.ParseAddrPort(ln.String())
	if err != nil {
		return nil, fmt.Errorf("host: unexpected listen address %q: %w", ln, err)
	}
	s.addrs = reachable(ap.Port())
	return slices.Clone(s.addrs), nil
}

// Join connects to the peer hosting at addr ("host:port") and returns its
// identity, so the caller can open that peer's project. Joining a peer that is
// already connected succeeds and returns the same identity.
func (s *Session) Join(ctx context.Context, addr string) (string, error) {
	if err := validHostPort(addr); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, joinTimeout)
	defer cancel()
	peer, err := s.net.Dial(ctx, addr)
	if err != nil {
		return "", fmt.Errorf("join %s: %w", addr, err)
	}
	return string(peer), nil
}

var errBadAddress = errors.New("address must look like 192.168.1.20:7419")

func validHostPort(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" || strings.ContainsAny(host, " \t") {
		return errBadAddress
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errBadAddress
	}
	return nil
}

// reachable lists "ip:port" for this machine's global and private unicast
// addresses (IPv4 first), falling back to loopback when there is no network.
func reachable(port uint16) []string {
	var v4, v6 []string
	ifaddrs, _ := net.InterfaceAddrs()
	for _, a := range ifaddrs {
		prefix, err := netip.ParsePrefix(a.String())
		if err != nil {
			continue
		}
		ip := prefix.Addr()
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
			continue
		}
		hp := netip.AddrPortFrom(ip, port).String()
		if ip.Is4() {
			v4 = append(v4, hp)
		} else {
			v6 = append(v6, hp)
		}
	}
	if out := slices.Concat(v4, v6); len(out) > 0 {
		return out
	}
	return []string{netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), port).String()}
}
