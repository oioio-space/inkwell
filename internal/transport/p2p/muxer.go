package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p"
	libp2pyamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-yamux/v5"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// The yamux timers and window of the TCP path (block SP1; QUIC has its own
// streams). Each is a rule of docs/plans/2026-11-09-send-path-design.md that
// the send-path model proves together with frame.WriteIdle.
const (
	// muxWriteTimeout is yamux's ConnectionWriteTimeout (SP1-4, D-SP1-1): it
	// bounds the write valve and both phases of a keepalive ping, 120 s each,
	// where yamux's default of 10 s closed healthy sessions behind a relay hop.
	muxWriteTimeout = 120 * time.Second
	// muxKeepAlive is the keepalive period (SP1-4, D-SP1-3), libp2p's own,
	// stated here with the timers it must fit.
	muxKeepAlive = 30 * time.Second
	// streamWindow is the pinned yamux stream window (SP1-5, D-SP1-2): initial
	// equals maximum, so there is no auto-tuning and the update quantum stays
	// 128 KiB. 256 KiB is yamux's smallest allowed window.
	streamWindow = 256 << 10
)

// A write may not be cut by the valve before the reader gives up on it; this
// fails to compile otherwise.
const _ = uint(muxWriteTimeout - frame.WriteIdle - 1)

// muxConfig returns the yamux configuration of the node and of the relay: a
// copy of libp2p's own defaults, so that the package-level default is never
// mutated, with the constants above stated.
func muxConfig() *yamux.Config {
	cfg := *libp2pyamux.DefaultTransport.Config()
	cfg.ConnectionWriteTimeout = muxWriteTimeout
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = muxKeepAlive
	cfg.InitialStreamWindowSize = streamWindow
	cfg.MaxStreamWindowSize = streamWindow
	return &cfg
}

// muxer is the libp2p option that installs [muxConfig].
func muxer() libp2p.Option {
	return libp2p.Muxer(libp2pyamux.ID, (*libp2pyamux.Transport)(muxConfig()))
}
