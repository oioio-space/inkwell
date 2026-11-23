// SPDX-License-Identifier: MPL-2.0

package syncack

import (
	"cmp"
	"slices"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// The bounds of a superseded connection's liveness proof (spec §8.7.4, §16, block HP1c, rule P-15).
const (
	// ProofWindow is T_vie: an older connection proves it lives by a byte of the peer within this window.
	ProofWindow = transport.SupersedeProofWindow
	// ProofLen is the ping a proof writes, /ipfs/ping/1.0.0's payload.
	ProofLen = transport.SupersedeProofLen
)

// Conn is what rule P-15 knows of one open connection to a peer. Times are on the caller's clock.
type Conn struct {
	ID uint64
	// Opened is when the connection registered and was kept (H-19); with ID it orders the connections, oldest first.
	Opened time.Duration
	// Acked reports that it carries a sync stream acknowledged in either direction (§8.7.3, §8.7.4).
	Acked bool
	// Proving reports a proof on it that has neither been dropped nor ended by its close.
	Proving bool
	// Started is when that proof's ping was written.
	Started time.Duration
	// For is the Opened of the connection whose registration caused that proof.
	For time.Duration
	// Heard reports a byte of the peer on the connection at or before Started + ProofWindow.
	Heard bool
	// Unjudged reports that the proof ended at this node without reaching the peer (the node's own refusal of
	// the stream, for one): no verdict. The connection is not judged by that proof: it is kept and proved again
	// for a newer registration, as an answered one is. A local failure is never the peer's silence.
	Unjudged bool
}

// ProofState is where a connection's proof stands at a given time.
type ProofState uint8

const (
	// Unproved is no proof.
	Unproved ProofState = iota
	// Pending is a ping written, no byte yet, ProofWindow not elapsed.
	Pending
	// Answered is a byte within ProofWindow: the connection lives.
	Answered
	// Silent is ProofWindow elapsed with no byte.
	Silent
	// Void is a proof that ended with no verdict (Conn.Unjudged).
	Void
)

var proofNames = [...]string{"none", "pending", "answered", "silent", "void"}

// String returns the state's name in the vectors.
func (s ProofState) String() string {
	if int(s) < len(proofNames) {
		return proofNames[s]
	}
	return "proof(?)"
}

// State is the state of c's proof at time now.
func (c Conn) State(now time.Duration) ProofState {
	switch {
	case !c.Proving:
		return Unproved
	case c.Heard:
		return Answered
	case c.Unjudged:
		return Void
	case now-c.Started >= ProofWindow:
		return Silent
	default:
		return Pending
	}
}

// Decision is what rule P-15 asks of the node at a given time. Each list holds connection IDs, oldest first.
type Decision struct {
	// Prove: start a proof (write a ping) on these connections.
	Prove []uint64
	// Close: close these connections; their streams end as streams lost with their connection.
	Close []uint64
	// Drop: drop the proof of these connections, which stay open (no newer connection is left).
	Drop []uint64
	// Wake is the earliest time at which a pending proof falls silent; zero when none is pending.
	Wake time.Duration
}

// Supersede is rule P-15 (spec §8.7.4) at time now over the open connections to one peer. Every connection older
// than the newest is proved once per newer registration: one with no proof, or one answered for a registration
// older than the newest's, gets a proof; a proof in progress (pending or silent) is never restarted. A silent
// connection is closed once a newer connection still judged alive (not silent itself; the newest is never judged)
// carries an acknowledged sync stream (until then it is the
// fallback of a newer opening that awaits its ACK); an answered one is never closed; the newest, hence the only
// one, never is, nor one whose proof ended with no verdict (Void: it is proved again, as an answered one, for a
// newer registration). A proof on the newest is dropped: no newer connection is left. conns is not modified.
func Supersede(now time.Duration, conns []Conn) Decision {
	d := Decision{Prove: []uint64{}, Close: []uint64{}, Drop: []uint64{}}
	if len(conns) == 0 {
		return d
	}
	cs := slices.Clone(conns)
	slices.SortFunc(cs, func(a, b Conn) int {
		return cmp.Or(cmp.Compare(a.Opened, b.Opened), cmp.Compare(a.ID, b.ID))
	})
	newest := cs[len(cs)-1]
	if st := newest.State(now); st == Pending || st == Silent {
		d.Drop = append(d.Drop, newest.ID)
	}
	// Some connection newer than older[i], still judged alive, carries an acknowledged stream. The newest is never
	// judged (its proof is dropped); an older one counts unless its own proof is silent (block HP1c2, review 3b: a
	// dead acknowledged middle connection must not lift the oldest one's wait while the newest awaits its ACK).
	newerAcked := newest.Acked
	older := cs[:len(cs)-1]
	closes := make([]bool, len(older))
	waking := false
	for i := len(older) - 1; i >= 0; i-- {
		c := older[i]
		st := c.State(now)
		switch st {
		case Unproved:
			d.Prove = append(d.Prove, c.ID)
		case Answered, Void:
			if c.For < newest.Opened {
				d.Prove = append(d.Prove, c.ID)
			}
		case Pending:
			if w := c.Started + ProofWindow; !waking || w < d.Wake {
				d.Wake, waking = w, true
			}
		case Silent:
			closes[i] = newerAcked
		}
		newerAcked = newerAcked || (c.Acked && st != Silent)
	}
	slices.Reverse(d.Prove)
	for i, c := range older {
		if closes[i] {
			d.Close = append(d.Close, c.ID)
		}
	}
	return d
}
