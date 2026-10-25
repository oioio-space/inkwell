package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/network"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// ProtocolLobby is the libp2p protocol of the lobby: one request and one
// response on a stream, which is how a person who is not a member yet is let
// in (see package invite).
const ProtocolLobby = "/inkwell/lobby/1.0.0"

const (
	// MaxLobbyRequest bounds a lobby request, in bytes.
	MaxLobbyRequest = 4 << 10
	// MaxLobbyResponse bounds a lobby response, in bytes. It is larger than a
	// request because the answer to a joiner carries the member list.
	MaxLobbyResponse = 256 << 10

	// lobbyReadTimeout bounds how long a request takes to arrive.
	lobbyReadTimeout = 10 * time.Second
	// lobbyHandleTimeout bounds the handler, which may wait for a person.
	lobbyHandleTimeout = 3 * time.Minute
	// maxLobbyStreams bounds the lobby streams of one peer.
	maxLobbyStreams = 2
)

// LobbyHandler answers one lobby request from a peer that is connected but may
// not be a member. It returns ok = false to give no answer at all: the stream
// is reset, which tells a stranger nothing. ctx ends with the node or after
// three minutes.
type LobbyHandler func(ctx context.Context, from transport.PeerID, req []byte) (resp []byte, ok bool)

// HandleLobby sets the handler of lobby requests; nil removes it, and the
// lobby then resets every stream. It may be called at any time.
func (n *Node) HandleLobby(h LobbyHandler) {
	if h == nil {
		n.lobby.Store(nil)
		return
	}
	n.lobby.Store(&h)
}

// handleLobbyStream serves one lobby stream.
func (n *Node) handleLobbyStream(s network.Stream) {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		_ = s.Reset()
		return
	}
	n.wg.Add(1)
	n.mu.Unlock()
	defer n.wg.Done()

	p := s.Conn().RemotePeer()
	name, ok := peerName(p)
	h := n.lobby.Load()
	if !ok || h == nil || !n.admitted(p) {
		_ = s.Reset()
		return
	}
	_ = s.SetReadDeadline(time.Now().Add(lobbyReadTimeout))
	req, err := frame.Read(s, MaxLobbyRequest)
	if err != nil {
		_ = s.Reset()
		return
	}
	ctx, cancel := context.WithTimeout(n.ctx, lobbyHandleTimeout)
	defer cancel()
	resp, ok := (*h)(ctx, name, req)
	if !ok || len(resp) > MaxLobbyResponse {
		_ = s.Reset()
		return
	}
	_ = s.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := frame.Write(s, resp); err != nil {
		_ = s.Reset()
		return
	}
	_ = s.Close()
}

// AskLobby sends req to the lobby of peer to, which must be connected (see
// [Node.Connect]), and returns its response. Canceling ctx abandons the
// request. It fails if the peer does not answer: it has no lobby handler, or
// its handler gave no answer.
func (n *Node) AskLobby(ctx context.Context, to transport.PeerID, req []byte) ([]byte, error) {
	id, ok := parseID(to)
	if !ok {
		return nil, fmt.Errorf("p2p: lobby: %q is not a peer id", to)
	}
	if len(req) > MaxLobbyRequest {
		return nil, transport.ErrFrameTooLarge
	}
	if n.isClosed() {
		return nil, transport.ErrClosed
	}
	sctx := network.WithAllowLimitedConn(ctx, "inkwell lobby")
	sctx = network.WithNoDial(sctx, "the peer is connected")
	s, err := n.h.NewStream(sctx, id, ProtocolLobby)
	if err != nil {
		return nil, fmt.Errorf("p2p: lobby: open stream to %s: %w", to, err)
	}
	stop := context.AfterFunc(ctx, func() { _ = s.Reset() })
	defer stop()
	defer func() { _ = s.Reset() }() // a no-op once the stream is closed in order
	if err := frame.Write(s, req); err != nil {
		return nil, fmt.Errorf("p2p: lobby: %w", err)
	}
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("p2p: lobby: %w", err)
	}
	resp, err := frame.Read(s, MaxLobbyResponse)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, fmt.Errorf("p2p: lobby: no answer from %s: %w", to, err)
	}
	return resp, nil
}
