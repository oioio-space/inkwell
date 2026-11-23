// SPDX-License-Identifier: MPL-2.0

package syncack_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/protocol/conformance"
	"github.com/oioio-space/inkwell/protocol/syncack"
	"github.com/oioio-space/inkwell/protocol/vectors"
)

const sec = time.Second

// P-15 (spec §8.7.4): the corpus vectors/supersede.json is the rule's normative test, run here as well as in
// protocol/conformance, so that mutation testing of this package sees it.
func TestSupersede(t *testing.T) {
	results, err := conformance.Run(conformance.StepFromEnv(), func(v conformance.Vector) error {
		if v.File != "supersede.json" {
			return nil
		}
		return conformance.Reference(v)
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range results {
		if r.Vector.File != "supersede.json" {
			continue
		}
		n++
		if !r.Passed {
			t.Errorf("P-15 %s: %v", r.Vector.ID, r.Err)
		}
	}
	if n == 0 {
		t.Fatal("P-15: no supersede vector")
	}
}

// P-15: the node never closes its newest connection to the peer, hence never its only one; a proof left on the
// newest (its newer connection closed) is dropped and the connection stays open.
func TestSupersedeKeepsSole(t *testing.T) {
	silent := syncack.Conn{ID: 1, Acked: true, Proving: true, Started: sec, For: sec}
	for _, now := range []time.Duration{6 * sec, time.Hour} {
		d := syncack.Supersede(now, []syncack.Conn{silent})
		if len(d.Close) != 0 || len(d.Prove) != 0 || !slices.Equal(d.Drop, []uint64{1}) {
			t.Fatalf("P-15 sole connection closed or kept proving at %v: %+v", now, d)
		}
	}
	// Two connections, both silent, the older acknowledged: neither is closed (only a newer acked one closes).
	older := syncack.Conn{ID: 1, Acked: true, Proving: true, Started: 3 * sec, For: 3 * sec}
	newest := syncack.Conn{ID: 2, Opened: sec, Proving: true, Started: 3 * sec, For: 3 * sec}
	d := syncack.Supersede(time.Hour, []syncack.Conn{newest, older})
	if len(d.Close) != 0 || !slices.Equal(d.Drop, []uint64{2}) {
		t.Fatalf("P-15 newest connection closed, or its proof kept: %+v", d)
	}
}

// P-15: an older connection that answered within T_vie stays open, whatever the time and the newer's ACK, until
// another newer connection registers, which proves it again.
func TestSupersedeKeepsAnswered(t *testing.T) {
	answered := syncack.Conn{ID: 1, Proving: true, Started: sec, For: sec, Heard: true}
	newer := syncack.Conn{ID: 2, Opened: sec, Acked: true}
	for _, now := range []time.Duration{sec, 6 * sec, time.Hour} {
		d := syncack.Supersede(now, []syncack.Conn{answered, newer})
		if len(d.Close) != 0 || len(d.Prove) != 0 || len(d.Drop) != 0 || d.Wake != 0 {
			t.Fatalf("P-15 live connection closed or proved again at %v: %+v", now, d)
		}
		if s := answered.State(now); s != syncack.Answered {
			t.Fatalf("P-15 answered proof reads %v at %v", s, now)
		}
	}
	third := syncack.Conn{ID: 3, Opened: 9 * sec}
	if d := syncack.Supersede(9*sec, []syncack.Conn{answered, newer, third}); !slices.Equal(d.Prove, []uint64{1, 2}) {
		t.Fatalf("P-15 a newer registration does not prove the answered connection again: %+v", d)
	}
}

// P-15, the fallback exemption: a silent older connection stays open while no newer connection carries an
// acknowledged sync stream (the newer opening may still fail, and closing would announce PeerDown wrongly), and is
// closed as soon as one does; before T_vie it waits, with the deadline to wake at.
func TestSupersedeWaitsForNewerAck(t *testing.T) {
	older := syncack.Conn{ID: 1, Proving: true, Started: sec, For: sec}
	newer := syncack.Conn{ID: 2, Opened: sec}
	d := syncack.Supersede(sec+syncack.ProofWindow-time.Millisecond, []syncack.Conn{older, newer})
	if len(d.Close) != 0 || d.Wake != sec+syncack.ProofWindow {
		t.Fatalf("P-15 pending proof: %+v, want no close and a wake at %v", d, sec+syncack.ProofWindow)
	}
	for _, now := range []time.Duration{sec + syncack.ProofWindow, 16 * sec, time.Hour} {
		if d := syncack.Supersede(now, []syncack.Conn{older, newer}); len(d.Close) != 0 || d.Wake != 0 {
			t.Fatalf("P-15 fallback closed before the newer connection's ACK at %v: %+v", now, d)
		}
	}
	newer.Acked = true
	if d := syncack.Supersede(16*sec, []syncack.Conn{older, newer}); !slices.Equal(d.Close, []uint64{1}) {
		t.Fatalf("P-15 dead connection kept after the newer connection's ACK: %+v", d)
	}
	if syncack.ProofWindow != 5*sec || syncack.ProofLen != 32 {
		t.Fatalf("P-15 bounds: T_vie %v, ping %d bytes; want 5 s, 32", syncack.ProofWindow, syncack.ProofLen)
	}
}

// P-15's guarantees on any input: the newest connection is never closed nor proved, an answered or pending
// connection is never closed, a close needs a newer acknowledged connection, nothing is listed twice, and the
// input is left as it was.
func FuzzSupersede(f *testing.F) {
	data, err := vectors.FS.ReadFile("supersede.json")
	if err != nil {
		f.Fatal(err)
	}
	var file struct {
		Vectors []struct {
			Context struct {
				Now   int64 `json:"now_ms"`
				Conns []struct {
					ID     uint64 `json:"id"`
					Opened int64  `json:"opened_ms"`
					Acked  bool   `json:"acked"`
					Proof  *struct {
						Started int64 `json:"started_ms"`
						For     int64 `json:"for_ms"`
						Heard   bool  `json:"heard"`
					} `json:"proof"`
				} `json:"conns"`
			} `json:"context"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		f.Fatal(err)
	}
	for _, v := range file.Vectors {
		var b []byte
		b = append(b, byte(v.Context.Now/1000))
		for _, c := range v.Context.Conns {
			var flags byte
			var started, forMs int64
			if c.Acked {
				flags |= 1
			}
			if p := c.Proof; p != nil {
				flags |= 2
				if p.Heard {
					flags |= 4
				}
				started, forMs = p.Started, p.For
			}
			b = append(b, byte(c.ID), byte(c.Opened/1000), flags, byte(started/1000), byte(forMs/1000))
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) == 0 {
			return
		}
		now := time.Duration(b[0]) * sec
		var conns []syncack.Conn
		for i := 1; i+5 <= len(b); i += 5 {
			c := syncack.Conn{ID: uint64(b[i] % 8), Opened: time.Duration(b[i+1]%16) * sec, Acked: b[i+2]&1 != 0}
			if b[i+2]&2 != 0 {
				c.Proving, c.Heard = true, b[i+2]&4 != 0
				c.Started, c.For = time.Duration(b[i+3]%32)*sec, time.Duration(b[i+4]%16)*sec
			}
			if slices.ContainsFunc(conns, func(x syncack.Conn) bool { return x.ID == c.ID }) {
				continue
			}
			conns = append(conns, c)
		}
		before := slices.Clone(conns)
		d := syncack.Supersede(now, conns)
		if !slices.Equal(before, conns) {
			t.Fatal("P-15 Supersede modified its input")
		}
		if len(conns) == 0 {
			if len(d.Prove)+len(d.Close)+len(d.Drop) != 0 {
				t.Fatalf("P-15 decision without a connection: %+v", d)
			}
			return
		}
		newest := conns[0]
		for _, c := range conns[1:] {
			if c.Opened > newest.Opened || c.Opened == newest.Opened && c.ID > newest.ID {
				newest = c
			}
		}
		byID := map[uint64]syncack.Conn{}
		for _, c := range conns {
			byID[c.ID] = c
		}
		listed := map[uint64]bool{}
		for _, l := range [][]uint64{d.Prove, d.Close, d.Drop} {
			for _, id := range l {
				if listed[id] {
					t.Fatalf("P-15 connection %d listed twice: %+v", id, d)
				}
				listed[id] = true
			}
		}
		if slices.Contains(d.Close, newest.ID) || slices.Contains(d.Prove, newest.ID) {
			t.Fatalf("P-15 newest connection %d closed or proved: %+v", newest.ID, d)
		}
		for _, id := range d.Close {
			c := byID[id]
			if c.State(now) != syncack.Silent {
				t.Fatalf("P-15 connection %d closed in state %v", id, c.State(now))
			}
			if !slices.ContainsFunc(conns, func(x syncack.Conn) bool {
				return x.Acked && (x.Opened > c.Opened || x.Opened == c.Opened && x.ID > c.ID)
			}) {
				t.Fatalf("P-15 connection %d closed with no newer acknowledged connection: %+v", id, conns)
			}
		}
	})
}

// P-15: a proof that ended at this node without reaching the peer gives no verdict. The connection is not closed,
// whatever the time and the newer's ACK; it is proved again only for a newer registration; its state reads void.
func TestSupersedeKeepsUnjudged(t *testing.T) {
	unjudged := syncack.Conn{ID: 1, Proving: true, Started: sec, For: sec, Unjudged: true}
	newer := syncack.Conn{ID: 2, Opened: sec, Acked: true}
	for _, now := range []time.Duration{sec, 6 * sec, time.Hour} {
		d := syncack.Supersede(now, []syncack.Conn{unjudged, newer})
		if len(d.Close) != 0 || len(d.Prove) != 0 || len(d.Drop) != 0 || d.Wake != 0 {
			t.Fatalf("P-15 live connection closed on a local failure, or proved again at %v: %+v", now, d)
		}
		if s := unjudged.State(now); s != syncack.Void || s.String() != "void" {
			t.Fatalf("P-15 unjudged proof reads %v at %v, want void", s, now)
		}
	}
	third := syncack.Conn{ID: 3, Opened: 9 * sec}
	if d := syncack.Supersede(9*sec, []syncack.Conn{unjudged, newer, third}); !slices.Equal(d.Prove, []uint64{1, 2}) {
		t.Fatalf("P-15 a newer registration does not prove the unjudged connection again: %+v", d)
	}
}
