package obs

import (
	"cmp"
	"slices"
	"time"
)

// Link kinds of the network debug snapshot: the kind of path, never an
// address.
const (
	LinkLAN     = "lan"
	LinkDirect  = "direct"
	LinkRelayed = "relayed"
)

// LinkState is what the transport knows of one link, given to [Network]. Peer
// is the raw peer id: it never reaches the snapshot.
type LinkState struct {
	Peer string
	// Kind is [LinkLAN], [LinkDirect] or [LinkRelayed]; another is dropped.
	Kind     string
	RTT      time.Duration
	Downs    int
	Resyncs  int
	LastDown time.Duration
	Sent     uint64
	Received uint64
	Opened   uint64
}

// LinkDebug is one link of the network debug snapshot: every field is a safe
// value under rule V, and the peer a pseudonym.
type LinkDebug struct {
	// Peer is HMAC-SHA-256 of the peer id under the session's salt, 8 bytes
	// in hex: stable for this run only (spec O-2).
	Peer       string `json:"peer"`
	Kind       string `json:"kind"`
	RTTMs      int64  `json:"rttMs"`
	Downs      uint64 `json:"downs"`
	Resyncs    uint64 `json:"resyncs"`
	LastDownMs int64  `json:"lastDownMs"`
	Sent       uint64 `json:"sent"`
	Received   uint64 `json:"received"`
	Opened     uint64 `json:"opened"`
}

// NetworkSnapshot is the network's debug state (rule V-3), read only when it
// is asked for.
type NetworkSnapshot struct {
	Links []LinkDebug `json:"links"`
}

// Network returns the debug snapshot of links, each peer keyed by its
// pseudonym under salt, the session's random salt; sorted by pseudonym.
func Network(salt []byte, links []LinkState) NetworkSnapshot {
	out := NetworkSnapshot{Links: []LinkDebug{}}
	for _, l := range links {
		if l.Kind != LinkLAN && l.Kind != LinkDirect && l.Kind != LinkRelayed {
			continue
		}
		out.Links = append(out.Links, LinkDebug{
			Peer:       pseudonym(salt, l.Peer),
			Kind:       l.Kind,
			RTTMs:      max(l.RTT.Milliseconds(), 0),
			Downs:      uint64(max(l.Downs, 0)),   // #nosec G115 -- not negative.
			Resyncs:    uint64(max(l.Resyncs, 0)), // #nosec G115 -- not negative.
			LastDownMs: max(l.LastDown.Milliseconds(), 0),
			Sent:       l.Sent,
			Received:   l.Received,
			Opened:     l.Opened,
		})
	}
	slices.SortFunc(out.Links, func(a, b LinkDebug) int { return cmp.Compare(a.Peer, b.Peer) })
	return out
}
