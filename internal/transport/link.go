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

	// LossPct is the percentage of packets lost over the last [LossWindow],
	// or nil while fewer than [MinLossPackets] were sent. LossScope says what
	// it measures.
	LossPct   *float64
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
	// [Link.Quality] ignores them.
	Resyncs10m, Resyncs int
}

// Window and minimum sample of the loss figure.
const (
	// LossWindow is the period a loss percentage covers.
	LossWindow = 30 * time.Second
	// MinLossPackets is the number of packets below which no loss is reported.
	MinLossPackets = 50
	// DownsWindow is the period Downs10m covers.
	DownsWindow = 10 * time.Minute
)

// Quality is a plain grade of a link, for the people popover and the status
// bar. The first proposal of decision Q4; the UX design refines it.
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

// Thresholds of the grades: a link is graded by its worst figure, a figure
// at or above the Poor threshold makes it poor, at or above Fair, fair.
const (
	FairRTT   = 150 * time.Millisecond
	PoorRTT   = 400 * time.Millisecond
	FairLoss  = 3.0 // percent
	PoorLoss  = 10.0
	FairDowns = 1 // outages in the last ten minutes
	PoorDowns = 3
)

// Quality grades the link by its worst figure among RTT, loss and recent
// outages. Resyncs are not graded: a failed write already shows in the loss and
// RTT. It is [QualityUnknown] until an RTT or a loss figure exists.
func (l Link) Quality() Quality {
	if l.RTT == 0 && l.LossPct == nil {
		return QualityUnknown
	}
	loss := 0.0
	if l.LossPct != nil {
		loss = *l.LossPct
	}
	switch {
	case l.RTT >= PoorRTT || loss >= PoorLoss || l.Downs10m >= PoorDowns:
		return QualityPoor
	case l.RTT >= FairRTT || loss >= FairLoss || l.Downs10m >= FairDowns:
		return QualityFair
	default:
		return QualityGood
	}
}

// LinkSource is implemented by a transport that reports the state of its links,
// sorted by peer.
type LinkSource interface{ Links() []Link }
