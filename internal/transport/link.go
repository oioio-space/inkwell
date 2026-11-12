package transport

import "time"

// LinkKind says how well a peer is connected.
type LinkKind uint8

const (
	// LinkLAN is a direct connection on the local network.
	LinkLAN LinkKind = iota + 1
	// LinkDirect is a direct connection over the internet.
	LinkDirect
	// LinkRelayed is a connection through a relay: slower, and the relay
	// sees who talks to whom, but it works from any network.
	LinkRelayed
)

// String returns the kind's name, for logs.
func (k LinkKind) String() string {
	switch k {
	case LinkLAN:
		return "LinkLAN"
	case LinkDirect:
		return "LinkDirect"
	case LinkRelayed:
		return "LinkRelayed"
	default:
		return "LinkKind(?)"
	}
}

// LossScope says what a loss figure measures. When the peer is reached through
// a relay only the first hop, from us to the relay, can be observed.
type LossScope uint8

const (
	// LossPath is the loss on the whole path to the peer.
	LossPath LossScope = iota + 1
	// LossFirstHop is the loss between us and the relay only.
	LossFirstHop
)

// String returns the scope's name, as the webview reads it.
func (s LossScope) String() string {
	switch s {
	case LossPath:
		return "path"
	case LossFirstHop:
		return "firstHop"
	default:
		return ""
	}
}

// Link describes the connection to a peer (plan W33, contract v1). It carries
// no network address: only the kind of path leaves the transport.
type Link struct {
	// Peer is the peer at the other end.
	Peer PeerID
	// Kind is the best connection the peer is reached on.
	Kind LinkKind
	// PathSince is when Kind last changed, or when the link was first seen.
	PathSince time.Time
	// Upgrades counts relayed to direct changes, Fallbacks the opposite.
	Upgrades, Fallbacks int

	// RTT is the smoothed round-trip time, RTTVar its deviation and RTTAge the
	// time since the last sample. RTT is zero if none was measured yet.
	RTT, RTTVar, RTTAge time.Duration

	// ProbeLevel is the quality of the link a person is shown: the level of
	// the link probe (spec §8.5.5 step 6), [QualityUnknown] while the probe has
	// no figure and on a relayed link, which it does not measure end to end.
	// Only the bridge reads it (TestLinkProbeOnlyBridgeReads).
	ProbeLevel Quality
	// ProbeLoss is the probe's loss figure over the last [LossWindow], in
	// permille (spec §8.5.5 step 5), or nil while it is unknown. It is a
	// diagnostic, never shown as a quality. LossScope says what it measures.
	// Only the bridge reads it.
	ProbeLoss *uint16
	LossScope LossScope

	// UpSince is when the current session began. Downs10m counts the outages
	// of the last ten minutes and DownsTotal all of them; LastDown is the
	// duration of the last one. Only a lost connection is an outage.
	UpSince    time.Time
	Downs10m   int
	DownsTotal int
	LastDown   time.Duration

	// Resyncs10m and Resyncs count the times the session was restarted over a
	// healthy connection because a write failed (spec rule L): the peer leaves
	// and returns, but the connection never went down, so it is no outage.
	Resyncs10m, Resyncs int
}

// Windows of the link figures.
const (
	// LossWindow is the period the loss figure of the probe covers.
	LossWindow = 30 * time.Second
	// DownsWindow is the period Downs10m covers.
	DownsWindow = 10 * time.Minute
)

// Quality is the level of a link, for the people popover and the status bar:
// the probe's level (spec §8.5.5 step 6, ADR 0036 « Levels »).
type Quality uint8

const (
	// QualityUnknown means nothing was measured yet.
	QualityUnknown Quality = iota
	// QualityGood is a link nobody has to think about.
	QualityGood
	// QualityFair is a link that works but is noticeably slow or lossy.
	QualityFair
	// QualityPoor is a link that struggles or keeps dropping.
	QualityPoor
)

// String returns the grade's name, as the webview reads it.
func (q Quality) String() string {
	switch q {
	case QualityGood:
		return "good"
	case QualityFair:
		return "fair"
	case QualityPoor:
		return "poor"
	default:
		return "unknown"
	}
}

// LinkSource is implemented by a transport that reports the state of its links,
// sorted by peer.
type LinkSource interface{ Links() []Link }
