package trust

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"

	"github.com/oioio-space/inkwell/internal/transport"
)

// Wire limits of the sync protocol.
const (
	// MaxMessage is the largest sync message, in bytes.
	MaxMessage = 80 << 10
	// maxHave bounds the hashes of a Have message.
	maxHave = 256
	// chunkBytes is the soft size at which an Ops message is split.
	chunkBytes = 64 << 10

	// haveBudget is how many Have messages of one peer are answered per Tick.
	haveBudget = 2

	msgHave = 1
	msgOps  = 2
)

// Outbound is a message the caller must send on the trust channel.
type Outbound struct {
	To   transport.PeerID
	Data []byte
}

// Sync replicates a [List] between peers. It is a pure state machine: the
// caller feeds it peer events and received messages, and sends what it
// returns; it does no I/O, owns no goroutine and has no clock. Loss, duplication
// and reordering are tolerated: every op is relayed on arrival, and calling
// [Sync.Tick] periodically re-announces the frontier so that anything lost is
// resent. A Have is answered with Ops, never with a Have, and at most twice
// per peer and Tick, so a peer cannot make the replicas echo each other or
// amplify traffic; each side learns what it lacks from its own announcements.
// A sealed list does not announce. A Sync is not safe for concurrent use.
type Sync struct {
	list     *List
	peers    map[transport.PeerID]struct{}
	answered map[transport.PeerID]int // Haves answered since the last Tick.
}

// NewSync returns a Sync that replicates l.
func NewSync(l *List) *Sync {
	return &Sync{list: l, peers: make(map[transport.PeerID]struct{}), answered: make(map[transport.PeerID]int)}
}

// OnPeerUp registers p, resets its Have budget and announces our frontier to
// it, unless the list is sealed.
func (s *Sync) OnPeerUp(p transport.PeerID) []Outbound {
	s.peers[p] = struct{}{}
	delete(s.answered, p)
	return s.have(p)
}

// OnPeerDown forgets p.
func (s *Sync) OnPeerDown(p transport.PeerID) {
	delete(s.peers, p)
	delete(s.answered, p)
}

// Tick re-announces our frontier to every peer (unless the list is sealed),
// resets the Have budgets and expires the old ops waiting for parents. Call it
// periodically.
func (s *Sync) Tick() []Outbound {
	clear(s.answered)
	s.list.prunePending()
	var out []Outbound
	for _, p := range s.sortedPeers() {
		out = append(out, s.have(p)...)
	}
	return out
}

// Broadcast relays locally issued ops to every peer.
func (s *Sync) Broadcast(ops ...Op) []Outbound { return s.relay(ops, "") }

// OnMessage handles one message received from a peer. The returned messages
// are valid even when err is not nil: an op that fails to apply does not stop the others.
// A malformed message is an error and the caller may drop the peer.
func (s *Sync) OnMessage(from transport.PeerID, data []byte) ([]Outbound, error) {
	kind, have, ops, err := decodeMessage(data)
	if err != nil {
		return nil, err
	}
	if kind == msgHave {
		if s.answered[from] >= haveBudget {
			return nil, nil
		}
		s.answered[from]++
		return chunk(from, s.list.Since(have)), nil
	}
	added, err := s.list.ApplyAll(ops)
	return s.relay(added, from), err
}

func (s *Sync) have(to transport.PeerID) []Outbound {
	if s.list.Sealed() {
		return nil
	}
	f := s.list.Frontier()
	if len(f) > maxHave {
		f = f[:maxHave]
	}
	b := []byte{msgHave}
	b = binary.BigEndian.AppendUint16(b, uint16(len(f))) // #nosec G115 -- bounded by maxHave.
	for _, h := range f {
		b = append(b, h[:]...)
	}
	return []Outbound{{To: to, Data: b}}
}

// relay sends ops to every peer but except.
func (s *Sync) relay(ops []Op, except transport.PeerID) []Outbound {
	if len(ops) == 0 {
		return nil
	}
	var out []Outbound
	for _, p := range s.sortedPeers() {
		if p != except {
			out = append(out, chunk(p, ops)...)
		}
	}
	return out
}

func (s *Sync) sortedPeers() []transport.PeerID { return slices.Sorted(maps.Keys(s.peers)) }

// chunk packs ops into Ops messages of about chunkBytes each.
func chunk(to transport.PeerID, ops []Op) []Outbound {
	var out []Outbound
	var cur []byte
	count := 0
	flush := func() {
		if count == 0 {
			return
		}
		b := []byte{msgOps}
		b = binary.BigEndian.AppendUint16(b, uint16(count)) // #nosec G115 -- bounded by chunkBytes / min op size.
		out = append(out, Outbound{To: to, Data: append(b, cur...)})
		cur, count = nil, 0
	}
	for _, o := range ops {
		enc := Encode(o)
		if len(cur)+len(enc)+2 > chunkBytes {
			flush()
		}
		cur = binary.BigEndian.AppendUint16(cur, uint16(len(enc))) // #nosec G115 -- at most MaxOpSize.
		cur = append(cur, enc...)
		count++
	}
	flush()
	return out
}

// decodeMessage parses a sync message strictly.
func decodeMessage(b []byte) (kind byte, have []Hash, ops []Op, err error) {
	if len(b) > MaxMessage {
		return 0, nil, nil, fmt.Errorf("%w: message too large", ErrMalformed)
	}
	r := &reader{b: b}
	kind = r.u8()
	n := r.u16()
	switch kind {
	case msgHave:
		if n > maxHave {
			return 0, nil, nil, fmt.Errorf("%w: too many hashes", ErrMalformed)
		}
		for range n {
			have = append(have, r.hash())
		}
	case msgOps:
		for range n {
			enc := r.take(r.u16())
			if r.err != nil {
				break
			}
			o, derr := Decode(enc)
			if derr != nil {
				return 0, nil, nil, derr
			}
			ops = append(ops, o)
		}
	default:
		if r.err == nil {
			return 0, nil, nil, fmt.Errorf("%w: message kind %d", ErrMalformed, kind)
		}
	}
	if r.err != nil {
		return 0, nil, nil, r.err
	}
	if len(r.b) != 0 {
		return 0, nil, nil, fmt.Errorf("%w: trailing bytes", ErrMalformed)
	}
	return kind, have, ops, nil
}
