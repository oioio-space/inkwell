package blob_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// scripted is a peer that speaks the pull protocol by hand, to serve what a
// test wants: the honest slice, a forgery, a delay, or nothing.
type scripted struct {
	lane  *memnet.Node
	store *blob.Store
	// handle decides the answers to w; def computes the honest one. A nil
	// handle answers honestly.
	handle func(from transport.PeerID, w blob.Want, def func() blob.Message) []blob.Message

	mu    sync.Mutex
	wants []blob.Want
}

// newScripted joins a bulk lane named id and answers its requests from store,
// until the test ends.
func newScripted(t *testing.T, n *netOf, id transport.PeerID, store *blob.Store) *scripted {
	t.Helper()
	s := &scripted{lane: n.net.JoinBulk(id), store: store}
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			ev, err := s.lane.Recv(ctx)
			if err != nil {
				return
			}
			if ev.Kind != transport.Message {
				continue
			}
			wg.Go(func() { s.answer(ctx, ev.Peer, ev.Payload) })
		}
	})
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	return s
}

func (s *scripted) answer(ctx context.Context, from transport.PeerID, payload []byte) {
	m, err := blob.Decode(payload)
	w, ok := m.(blob.Want)
	if err != nil || !ok {
		return
	}
	s.mu.Lock()
	s.wants = append(s.wants, w)
	s.mu.Unlock()
	def := func() blob.Message {
		sl, err := s.store.ReadSlice(w.Hash, w.Offset, w.Length)
		if err != nil {
			return blob.Missing{Hash: w.Hash}
		}
		return sl
	}
	replies := []blob.Message{def()}
	if s.handle != nil {
		replies = s.handle(from, w, def)
	}
	for _, r := range replies {
		b, err := blob.Encode(r)
		if err != nil {
			continue // a forgery the encoder refuses is not sent
		}
		_ = s.lane.Send(ctx, from, b)
	}
}

// requested returns the requests received so far.
func (s *scripted) requested() []blob.Want {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.wants)
}

// link connects the peers of the network a and b, and waits until the
// services have seen each other.
func (n *netOf) link(a, b transport.PeerID) {
	n.net.Link(a, b)
	synctest.Wait()
}

func TestFetchFromPeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		a := n.join("a", blob.Options{}, blob.ServiceOptions{})
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		data := testData(3*blob.MaxSlice+blob.GroupSize+99, 1) // four requests
		h := a.put(t, data)
		n.link("a", "b")

		if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		mustGet(t, b.store, h, data)
		if b.store.Used() != int64(len(data)) {
			t.Errorf("Used = %d, want %d", b.store.Used(), len(data))
		}
		// The fetched blob is served on in turn.
		c := n.join("c", blob.Options{}, blob.ServiceOptions{})
		b.reference(h)
		n.link("b", "c")
		n.net.Partition("a", "b") // c can only get it from b
		synctest.Wait()
		if err := c.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
			t.Fatalf("Fetch from the first fetcher: %v", err)
		}
		mustGet(t, c.store, h, data)
		// Nothing is left in partial/.
		entries, err := os.ReadDir(filepath.Join(b.dir, "partial"))
		if err != nil || len(entries) != 0 {
			t.Errorf("partial/ = %v, %v; want empty after the download", entries, err)
		}
		// Already stored: nothing to do, even with nobody around.
		n.net.Partition("b", "c")
		synctest.Wait()
		if err := c.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
			t.Errorf("Fetch of a stored blob: %v", err)
		}
	})
}

func TestFetchEmptyAndTinyBlobs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		a := n.join("a", blob.Options{}, blob.ServiceOptions{})
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("a", "b")
		for i, data := range [][]byte{{}, []byte("x"), testData(blob.GroupSize, 3), testData(blob.GroupSize+1, 4)} {
			h := a.put(t, data)
			if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
				t.Fatalf("Fetch of %d bytes (case %d): %v", len(data), i, err)
			}
			mustGet(t, b.store, h, data)
		}
	})
}

// A pull-only protocol: only the hashes the project references are served,
// and only to peers the app allows.
func TestServesOnlyReferencedBlobsToAllowedPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		short := blob.ServiceOptions{Timeout: time.Second}
		a := n.join("a", blob.Options{}, short)
		b := n.join("b", blob.Options{}, short)
		n.link("a", "b")
		data := testData(50_000, 5)
		ref := blob.Ref{Hash: blob.HashOf(data), Size: int64(len(data))}
		if _, err := a.store.PutBytes(data); err != nil { // stored, but not referenced
			t.Fatal(err)
		}

		if err := b.svc.Fetch(t.Context(), ref); !errors.Is(err, blob.ErrNoPeer) {
			t.Errorf("Fetch of an unreferenced blob = %v, want ErrNoPeer (answered missing)", err)
		}
		a.reference(ref.Hash)
		a.deny("b")
		if err := b.svc.Fetch(t.Context(), ref); !errors.Is(err, blob.ErrNoPeer) {
			t.Errorf("Fetch from a peer that denies us = %v, want ErrNoPeer (no answer)", err)
		}
		if b.store.Has(ref.Hash) {
			t.Error("the blob arrived although the server refused")
		}
		// The asking side does not ask a peer it does not allow either.
		a.mu.Lock()
		delete(a.denied, "b")
		a.mu.Unlock()
		b.deny("a")
		start := time.Now()
		if err := b.svc.Fetch(t.Context(), ref); !errors.Is(err, blob.ErrNoPeer) || time.Since(start) != 0 {
			t.Errorf("Fetch when we deny the only peer = %v after %v, want ErrNoPeer at once", err, time.Since(start))
		}
		b.mu.Lock()
		delete(b.denied, "a")
		b.mu.Unlock()
		if err := b.svc.Fetch(t.Context(), ref); err != nil {
			t.Errorf("Fetch once allowed: %v", err)
		}
	})
}

func TestFetchWithoutPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		err := b.svc.Fetch(t.Context(), blob.Ref{Hash: blob.HashOf([]byte("x")), Size: 1})
		if !errors.Is(err, blob.ErrNoPeer) {
			t.Errorf("Fetch with nobody connected = %v, want ErrNoPeer", err)
		}
		// PeersChanged fires when a peer comes.
		changed := b.svc.PeersChanged()
		n.join("a", blob.Options{}, blob.ServiceOptions{})
		n.link("a", "b")
		select {
		case <-changed:
		default:
			t.Error("PeersChanged did not fire when a peer connected")
		}
	})
}

func TestQuotaErrorsComeBeforeAnyRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		a := n.join("a", blob.Options{}, blob.ServiceOptions{})
		b := n.join("b", blob.Options{MaxFile: 1 << 20, MaxProject: 2 << 20}, blob.ServiceOptions{})
		sp := newScripted(t, n, "z", a.store)
		n.link("z", "b")
		ctx := t.Context()

		if err := b.svc.Fetch(ctx, blob.Ref{Hash: blob.HashOf([]byte("big")), Size: 1<<20 + 1}); !errors.Is(err, blob.ErrTooLarge) {
			t.Errorf("Fetch of a file over the limit = %v, want ErrTooLarge", err)
		}
		var refs []blob.Ref
		for i := range 3 {
			d := testData(1<<20, uint64(60+i))
			a.put(t, d)
			refs = append(refs, blob.Ref{Hash: blob.HashOf(d), Size: int64(len(d))})
		}
		for _, r := range refs[:2] {
			if err := b.svc.Fetch(ctx, r); err != nil {
				t.Fatalf("Fetch: %v", err)
			}
		}
		before := len(sp.requested())
		err := b.svc.Fetch(ctx, refs[2])
		if !errors.Is(err, blob.ErrProjectQuota) {
			t.Fatalf("Fetch over the project limit = %v, want ErrProjectQuota", err)
		}
		if got := len(sp.requested()); got != before {
			t.Errorf("%d requests sent for a fetch refused by the quota", got-before)
		}
		b.store.Confirm()
		if err := b.svc.Fetch(ctx, refs[2]); err != nil {
			t.Errorf("Fetch after Confirm: %v", err)
		}
	})
}

// PRJ-02: the link is cut half way; the download resumes, even after a
// restart, without asking again for a group that was verified.
func TestResumeAfterCutDoesNotResendVerifiedGroups(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(fmt.Sprintf("PRJ-02 restart=%v", restart), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				n := newNet(t)
				srcStore, _ := openStore(t, "", blob.Options{})
				srv := newScripted(t, n, "srv", srcStore)
				data := testData(16*blob.MaxSlice-1000, 70) // 16 requests, the last one short
				h, err := srcStore.PutBytes(data)
				if err != nil {
					t.Fatal(err)
				}
				ref := blob.Ref{Hash: h, Size: int64(len(data))}
				b := n.join("b", blob.Options{}, blob.ServiceOptions{Timeout: 2 * time.Second})
				n.link("srv", "b")

				// Cut the link once half of the file is served.
				var served atomic.Int32
				srv.handle = func(_ transport.PeerID, _ blob.Want, def func() blob.Message) []blob.Message {
					if served.Add(1) == 8 {
						n.net.Partition("srv", "b")
					}
					return []blob.Message{def()}
				}
				err = b.svc.Fetch(t.Context(), ref)
				if err == nil {
					t.Fatal("Fetch succeeded although the link was cut")
				}
				first := srv.requested()
				verified := len(first) - 1 // the last request got no answer through the cut
				if verified < 7 {
					t.Fatalf("%d requests before the cut, want 8", len(first))
				}
				if b.store.Has(h) {
					t.Fatal("a half-received file is in the store")
				}
				if _, err := os.Stat(filepath.Join(b.dir, "partial", h.String()[len("blake3:"):])); err != nil {
					t.Errorf("no partial file kept: %v", err)
				}

				// A restart: the same directory, a new store and service.
				resumer := b
				if restart {
					lane := n.net.JoinBulk("b2")
					store2, _ := openStore(t, b.dir, blob.Options{})
					resumer = &peer{id: "b2", lane: lane, store: store2, dir: b.dir, refs: map[blob.Hash]bool{}, denied: map[transport.PeerID]bool{}, log: new(logRecorder)}
					resumer.start(t, store2, lane, blob.ServiceOptions{})
					n.net.Link("srv", "b2")
					synctest.Wait()
				} else {
					srv.handle = nil
					n.net.Link("srv", "b")
					synctest.Wait()
				}
				srv.handle = nil
				if err := resumer.svc.Fetch(t.Context(), ref); err != nil {
					t.Fatalf("Fetch after the cut: %v", err)
				}
				mustGet(t, resumer.store, h, data)

				// Nothing verified was asked for again.
				second := srv.requested()[len(first):]
				var resent int
				for _, w := range second {
					for _, f := range first[:verified] {
						if w.Offset < f.Offset+uint64(f.Length) && f.Offset < w.Offset+uint64(w.Length) {
							resent++
						}
					}
				}
				if resent != 0 {
					t.Errorf("%d of %d resumed requests overlap groups that were verified before the cut", resent, len(second))
				}
				var total uint64
				for _, w := range slices.Concat(first[:verified], second) {
					total += uint64(w.Length)
				}
				if total < uint64(len(data)) || total > uint64(len(data))+blob.MaxSlice {
					t.Errorf("%d bytes requested in all for a file of %d: groups were asked for twice", total, len(data))
				}
			})
		})
	}
}

// A map that lies, or garbage in the map, does not give a wrong file: the final
// check catches it, and the next attempt starts over.
func TestResumeWithDamagedMapOrData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		a := n.join("a", blob.Options{}, blob.ServiceOptions{})
		data := testData(6*blob.MaxSlice, 80)
		h := a.put(t, data)
		ref := blob.Ref{Hash: h, Size: int64(len(data))}
		b := n.join("b", blob.Options{}, blob.ServiceOptions{Timeout: time.Second})
		n.link("a", "b")

		// Stop the first attempt after three requests by canceling.
		ctx, cancel := context.WithCancel(t.Context())
		var served atomic.Int32
		sp := newScripted(t, n, "s", a.store)
		sp.handle = func(_ transport.PeerID, _ blob.Want, def func() blob.Message) []blob.Message {
			if served.Add(1) == 3 {
				defer cancel()
			}
			return []blob.Message{def()}
		}
		n.net.Partition("a", "b")
		n.link("s", "b")
		if err := b.svc.Fetch(ctx, ref); !errors.Is(err, context.Canceled) {
			t.Fatalf("first Fetch = %v, want Canceled", err)
		}
		part := filepath.Join(b.dir, "partial", h.String()[len("blake3:"):])

		// 1. The map claims groups the data does not have (bit rot of the partial file).
		raw, err := os.ReadFile(part)
		if err != nil {
			t.Fatal(err)
		}
		raw[10] ^= 0xFF
		if err := os.WriteFile(part, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		sp.handle = nil
		if err := b.svc.Fetch(t.Context(), ref); !errors.Is(err, blob.ErrCorrupt) {
			t.Fatalf("Fetch over a rotten partial file = %v, want ErrCorrupt from the final check", err)
		}
		if b.store.Has(h) {
			t.Fatal("a corrupt file was stored")
		}
		if err := b.svc.Fetch(t.Context(), ref); err != nil {
			t.Fatalf("Fetch after the corrupt download was discarded: %v", err)
		}
		mustGet(t, b.store, h, data)
	})
}

func TestResumeWithUnusableMapStartsOver(t *testing.T) {
	for name, damage := range map[string]func(path string) error{
		"truncated map":  func(p string) error { return os.Truncate(p, 5) },
		"garbage map":    func(p string) error { return os.WriteFile(p, []byte("not a map at all, really not"), 0o600) },
		"flipped bitmap": flipLast,
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				n := newNet(t)
				srcStore, _ := openStore(t, "", blob.Options{})
				data := testData(4*blob.MaxSlice, 81)
				h, _ := srcStore.PutBytes(data)
				ref := blob.Ref{Hash: h, Size: int64(len(data))}
				b := n.join("b", blob.Options{}, blob.ServiceOptions{})
				sp := newScripted(t, n, "s", srcStore)
				n.link("s", "b")
				ctx, cancel := context.WithCancel(t.Context())
				var served atomic.Int32
				sp.handle = func(_ transport.PeerID, _ blob.Want, def func() blob.Message) []blob.Message {
					if served.Add(1) == 2 {
						defer cancel()
					}
					return []blob.Message{def()}
				}
				_ = b.svc.Fetch(ctx, ref)
				synctest.Wait() // the answer to the request given up on arrives, to nobody
				mapFile := filepath.Join(b.dir, "partial", h.String()[len("blake3:"):]+".map")
				if err := damage(mapFile); err != nil {
					t.Fatal(err)
				}
				sp.handle = nil
				before := len(sp.requested())
				if err := b.svc.Fetch(t.Context(), ref); err != nil {
					t.Fatalf("Fetch: %v", err)
				}
				mustGet(t, b.store, h, data)
				if got := len(sp.requested()) - before; got != 4 {
					t.Errorf("%d requests after a damaged map, want all 4: %+v", got, sp.requested())
				}
			})
		})
	}
}

// flipLast flips the last byte of the file at path.
func flipLast(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw[len(raw)-1] ^= 0x01
	return os.WriteFile(path, raw, 0o600)
}

// forgeries are the ways a peer can get a slice wrong; each must be caught.
var forgeries = map[string]func(s blob.Slice) blob.Message{
	"a flipped data byte": func(s blob.Slice) blob.Message {
		s.Bao = slices.Clone(s.Bao)
		s.Bao[len(s.Bao)-1] ^= 1
		return s
	},
	"a flipped byte in a parent hash": func(s blob.Slice) blob.Message {
		s.Bao = slices.Clone(s.Bao)
		if len(s.Bao) > 8+int(s.Length) {
			s.Bao[8+5] ^= 1
		} else {
			s.Bao[len(s.Bao)-1] ^= 1
		}
		return s
	},
	"a lie about the length of the file": func(s blob.Slice) blob.Message {
		s.Bao = slices.Clone(s.Bao)
		binary.LittleEndian.PutUint64(s.Bao, binary.LittleEndian.Uint64(s.Bao)+blob.GroupSize)
		return s
	},
	"a lie of one byte about the length of the file": func(s blob.Slice) blob.Message {
		s.Bao = slices.Clone(s.Bao)
		binary.LittleEndian.PutUint64(s.Bao, binary.LittleEndian.Uint64(s.Bao)+1)
		return s
	},
	"another range": func(s blob.Slice) blob.Message {
		s.Offset += blob.GroupSize
		return s
	},
	"another blob": func(s blob.Slice) blob.Message {
		s.Hash[0] ^= 1
		return s
	},
	"zeros for data": func(s blob.Slice) blob.Message {
		s.Bao = slices.Clone(s.Bao)
		clear(s.Bao[len(s.Bao)-int(s.Length):])
		return s
	},
}

// PRJ-03: a forged slice is dropped, the range asked from another peer, and a
// peer that forges three times is dropped with a security event.
func TestForgedSliceRetriesOtherPeer(t *testing.T) {
	for name, forge := range forgeries {
		t.Run("PRJ-03 "+name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				n := newNet(t)
				good := n.join("good", blob.Options{}, blob.ServiceOptions{})
				data := testData(3*blob.MaxSlice+1234, 90)
				h := good.put(t, data)
				evilStore, _ := openStore(t, "", blob.Options{})
				if _, err := evilStore.PutBytes(data); err != nil {
					t.Fatal(err)
				}
				evil := newScripted(t, n, "evil", evilStore) // sorts before "good": asked first
				evil.handle = func(_ transport.PeerID, _ blob.Want, def func() blob.Message) []blob.Message {
					if sl, ok := def().(blob.Slice); ok {
						return []blob.Message{forge(sl)}
					}
					return nil
				}
				b := n.join("b", blob.Options{}, blob.ServiceOptions{})
				n.link("evil", "b")
				n.link("good", "b")

				if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
					t.Fatalf("Fetch with a forging peer around: %v", err)
				}
				mustGet(t, b.store, h, data)
				// After a fault, or after a silence, the honest peer is preferred. Settling a
				// lie about the length costs a second request to the liar.
				if got := len(evil.requested()); got < 1 || got > 2 {
					t.Errorf("the forger was asked %d times, want once or twice", got)
				}
				wantEvents := 1
				switch name {
				case "another blob":
					wantEvents = 0 // an answer about a blob we did not ask about is no answer: it times out
				case "a lie about the length of the file":
					wantEvents = 0 // the liar stays silent when asked to prove it: unproven, so not blamed
				}
				if got := b.log.security(); got != wantEvents {
					t.Errorf("%d security events, want %d", got, wantEvents)
				}
				if dropped := b.droppedPeers(); len(dropped) != 0 {
					t.Errorf("dropped %v after one fault, want nobody", dropped)
				}
			})
		})
	}
	t.Run("PRJ-03 three faults drop the peer", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			n := newNet(t)
			evilStore, _ := openStore(t, "", blob.Options{})
			data := testData(2*blob.MaxSlice, 91)
			h, _ := evilStore.PutBytes(data)
			ref := blob.Ref{Hash: h, Size: int64(len(data))}
			evil := newScripted(t, n, "evil", evilStore)
			evil.handle = func(_ transport.PeerID, _ blob.Want, def func() blob.Message) []blob.Message {
				if sl, ok := def().(blob.Slice); ok {
					return []blob.Message{forgeries["a flipped data byte"](sl)}
				}
				return nil
			}
			b := n.join("b", blob.Options{}, blob.ServiceOptions{})
			n.link("evil", "b")

			if err := b.svc.Fetch(t.Context(), ref); !errors.Is(err, blob.ErrNoPeer) {
				t.Fatalf("Fetch from a forger alone = %v, want ErrNoPeer once it is dropped", err)
			}
			if got := len(evil.requested()); got != blob.MaxFaults {
				t.Errorf("the forger was asked %d times, want %d before it was dropped", got, blob.MaxFaults)
			}
			if dropped := b.droppedPeers(); !slices.Equal(dropped, []transport.PeerID{"evil"}) {
				t.Errorf("dropped = %v, want [evil]", dropped)
			}
			if got := b.log.security(); got != blob.MaxFaults+1 {
				t.Errorf("%d security events, want %d faults and the drop", got, blob.MaxFaults+1)
			}
			if b.store.Has(h) {
				t.Error("a forged file was stored")
			}

			// An honest peer arrives; the forger stays out even if it comes back.
			good := n.join("good", blob.Options{}, blob.ServiceOptions{})
			good.put(t, data)
			n.link("good", "b")
			asked := len(evil.requested())
			if err := b.svc.Fetch(t.Context(), ref); err != nil {
				t.Fatalf("Fetch once an honest peer is there: %v", err)
			}
			mustGet(t, b.store, h, data)
			if len(evil.requested()) != asked {
				t.Error("the dropped peer was asked again")
			}
		})
	})
}

// A peer that sends garbage frames is counted too.
func TestMalformedFramesAreFaults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		bad := n.net.JoinBulk("bad")
		n.link("bad", "b")
		for range blob.MaxFaults {
			if err := bad.Send(t.Context(), "b", []byte{0xEE, 1, 2, 3}); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
		}
		if dropped := b.droppedPeers(); !slices.Equal(dropped, []transport.PeerID{"bad"}) {
			t.Errorf("dropped = %v after %d malformed frames, want [bad]", dropped, blob.MaxFaults)
		}
	})
}

// The size in the index is not trusted: the first slice authenticates the real
// one, and a lie is told apart from a forgery.
func TestWrongSizeInIndex(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		a := n.join("a", blob.Options{}, blob.ServiceOptions{})
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("a", "b")
		data := testData(3*blob.GroupSize+5, 100)
		h := a.put(t, data)
		for _, size := range []int64{int64(len(data)) + 1, int64(len(data)) - 1, 1, 5 * blob.MaxSlice} {
			err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: size})
			if !errors.Is(err, blob.ErrSizeMismatch) {
				t.Errorf("Fetch with size %d for %d bytes = %v, want ErrSizeMismatch", size, len(data), err)
			}
		}
		if got := b.log.security(); got != 0 {
			t.Errorf("%d security events for an index that lied; the peer is honest", got)
		}
		if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
			t.Errorf("Fetch with the right size: %v", err)
		}
		// The empty file announced as non-empty.
		empty := a.put(t, nil)
		if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: empty, Size: 10}); !errors.Is(err, blob.ErrSizeMismatch) {
			t.Errorf("Fetch of the empty file announced as 10 bytes = %v, want ErrSizeMismatch", err)
		}
	})
}

func TestBusyIsWaitedOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		store, _ := openStore(t, "", blob.Options{})
		data := testData(blob.MaxSlice, 110)
		h, _ := store.PutBytes(data)
		sp := newScripted(t, n, "s", store)
		var calls atomic.Int32
		sp.handle = func(_ transport.PeerID, w blob.Want, def func() blob.Message) []blob.Message {
			if calls.Add(1) <= 2 {
				return []blob.Message{blob.Busy{Hash: w.Hash, Retry: 3 * time.Second}}
			}
			return []blob.Message{def()}
		}
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("s", "b")
		start := time.Now()
		if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if took := time.Since(start); took < 6*time.Second {
			t.Errorf("Fetch took %v, want at least the two 3 s waits the server asked for", took)
		}
		if b.log.security() != 0 {
			t.Error("a Busy answer was counted as a fault")
		}
	})
}

// A server that serves as many as it will answers Busy to the others.
func TestServerAnswersBusy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		a := n.join("a", blob.Options{}, blob.ServiceOptions{MaxServing: 1})
		data := testData(blob.MaxSlice, 111)
		h := a.put(t, data)
		// The server is slow to read from disk: hold it with a request that never ends.
		client := n.net.JoinBulk("client")
		n.link("a", "client")
		var busy, sliceReplies int
		for i := range 40 {
			b, _ := blob.Encode(blob.Want{Hash: h, Offset: 0, Length: blob.MaxSlice})
			if err := client.Send(t.Context(), "a", b); err != nil {
				t.Fatalf("Send %d: %v", i, err)
			}
		}
		synctest.Wait()
		for {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			ev, err := client.Recv(ctx)
			cancel()
			if err != nil {
				break
			}
			if ev.Kind != transport.Message {
				continue
			}
			switch m, _ := blob.Decode(ev.Payload); m.(type) {
			case blob.Busy:
				busy++
			case blob.Slice:
				sliceReplies++
			}
		}
		if busy == 0 || sliceReplies == 0 {
			t.Errorf("%d slices and %d Busy for 40 simultaneous requests with MaxServing 1; want both", sliceReplies, busy)
		}
	})
}

// A peer that disconnects in the middle of a request does not stall the fetch.
func TestPeerGoneMidRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		good := n.join("good", blob.Options{}, blob.ServiceOptions{})
		data := testData(2*blob.MaxSlice, 120)
		h := good.put(t, data)
		store, _ := openStore(t, "", blob.Options{})
		_, _ = store.PutBytes(data)
		flaky := newScripted(t, n, "flaky", store) // asked first
		flaky.handle = func(transport.PeerID, blob.Want, func() blob.Message) []blob.Message {
			n.net.Partition("flaky", "b")
			return nil
		}
		b := n.join("b", blob.Options{}, blob.ServiceOptions{}) // default 30 s timeout
		n.link("flaky", "b")
		n.link("good", "b")
		start := time.Now()
		if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if took := time.Since(start); took >= 30*time.Second {
			t.Errorf("Fetch took %v: the disconnection did not end the request", took)
		}
		mustGet(t, b.store, h, data)
	})
}

// At most four blobs download at once, and concurrent fetches of one blob share
// the download.
func TestConcurrentTransfers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		store, _ := openStore(t, "", blob.Options{})
		sp := newScripted(t, n, "s", store)
		var mu sync.Mutex
		inFlight := map[blob.Hash]int{}
		peak := 0
		sp.handle = func(_ transport.PeerID, w blob.Want, def func() blob.Message) []blob.Message {
			mu.Lock()
			inFlight[w.Hash]++
			peak = max(peak, len(inFlight))
			mu.Unlock()
			time.Sleep(time.Second)
			mu.Lock()
			if inFlight[w.Hash]--; inFlight[w.Hash] == 0 {
				delete(inFlight, w.Hash)
			}
			mu.Unlock()
			return []blob.Message{def()}
		}
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("s", "b")
		var refs []blob.Ref
		var datas [][]byte
		for i := range 9 {
			d := testData(blob.GroupSize*3, uint64(130+i))
			h, _ := store.PutBytes(d)
			refs = append(refs, blob.Ref{Hash: h, Size: int64(len(d))})
			datas = append(datas, d)
		}
		var wg sync.WaitGroup
		for i, r := range refs {
			for range 2 { // each blob twice at once
				wg.Go(func() {
					if err := b.svc.Fetch(t.Context(), r); err != nil {
						t.Errorf("Fetch %d: %v", i, err)
					}
				})
			}
		}
		wg.Wait()
		if peak > blob.MaxTransfers || peak < 2 {
			t.Errorf("peak of %d blobs downloading at once, want between 2 and %d", peak, blob.MaxTransfers)
		}
		for i, r := range refs {
			mustGet(t, b.store, r.Hash, datas[i])
		}
		if got := len(sp.requested()); got != len(refs) {
			t.Errorf("%d requests for %d one-request blobs each fetched twice; the duplicates should share", got, len(refs))
		}
	})
}

// A canceled fetch keeps its progress; the context error is returned, not a peer fault.
func TestFetchCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		store, _ := openStore(t, "", blob.Options{})
		data := testData(4*blob.MaxSlice, 140)
		h, _ := store.PutBytes(data)
		sp := newScripted(t, n, "s", store)
		sp.handle = func(transport.PeerID, blob.Want, func() blob.Message) []blob.Message { return nil }
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("s", "b")
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		if err := b.svc.Fetch(ctx, blob.Ref{Hash: h, Size: int64(len(data))}); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Fetch = %v, want DeadlineExceeded", err)
		}
		if b.log.security() != 0 {
			t.Error("a timeout counted as a security fault")
		}
	})
}

func TestNewServiceRequiresPolicies(t *testing.T) {
	s, _ := openStore(t, "", blob.Options{})
	lane := memnet.New(1, memnet.Faults{}).JoinBulk("a")
	if _, err := blob.NewService(lane, s, blob.ServiceOptions{}); err == nil {
		t.Error("NewService without Referenced and Allowed succeeded")
	}
}

// An answer to a request given up on arrives while the next one waits: it
// verifies, so it is kept; the slow peer is not blamed for being slow.
func TestLateAnswerIsNotAFault(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		store, _ := openStore(t, "", blob.Options{})
		data := testData(blob.GroupSize*2, 150)
		h, _ := store.PutBytes(data)
		sp := newScripted(t, n, "s", store)
		var calls atomic.Int32
		sp.handle = func(_ transport.PeerID, _ blob.Want, def func() blob.Message) []blob.Message {
			time.Sleep(time.Duration(4-calls.Add(1)) * 3 * time.Second) // 6 s for the first, 3 s for the second
			return []blob.Message{def()}
		}
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("s", "b")
		ref := blob.Ref{Hash: h, Size: int64(len(data))}

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := b.svc.Fetch(ctx, ref); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("first Fetch = %v, want DeadlineExceeded", err)
		}
		if err := b.svc.Fetch(t.Context(), ref); err != nil {
			t.Fatalf("second Fetch: %v", err)
		}
		mustGet(t, b.store, h, data)
		if got := b.log.security(); got != 0 {
			t.Errorf("%d security events for a slow peer", got)
		}
	})
}

// One member cannot hold every serving slot: at most two of its requests are
// served at once, the others are told to wait, and another member is served.
func TestServingIsFairBetweenPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		a := n.join("a", blob.Options{}, blob.ServiceOptions{MaxServing: 8})
		hx, hy := a.put(t, testData(blob.GroupSize, 1)), a.put(t, testData(blob.GroupSize, 2))
		release := make(chan struct{})
		var mu sync.Mutex
		inFlight, peak := 0, 0
		a.mu.Lock()
		a.gate = func(h blob.Hash) {
			if h != hx {
				return
			}
			mu.Lock()
			inFlight++
			peak = max(peak, inFlight)
			mu.Unlock()
			<-release
			mu.Lock()
			inFlight--
			mu.Unlock()
		}
		a.mu.Unlock()
		greedy, modest := n.net.JoinBulk("greedy"), n.net.JoinBulk("modest")
		n.link("a", "greedy")
		n.link("a", "modest")
		want := func(h blob.Hash) []byte {
			b, _ := blob.Encode(blob.Want{Hash: h, Length: blob.GroupSize})
			return b
		}
		for range 20 {
			if err := greedy.Send(t.Context(), "a", want(hx)); err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		if err := modest.Send(t.Context(), "a", want(hy)); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()

		mu.Lock()
		held := inFlight
		mu.Unlock()
		if held > 2 || peak > 2 {
			t.Errorf("%d requests of one peer served at once (peak %d), want at most 2", held, peak)
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		got := false
		for !got {
			ev, err := modest.Recv(ctx)
			if err != nil {
				break
			}
			if ev.Kind != transport.Message {
				continue
			}
			m, _ := blob.Decode(ev.Payload)
			_, got = m.(blob.Slice)
		}
		if !got {
			t.Error("the second member was not served while the first held its slots")
		}
		close(release)
		synctest.Wait()
	})
}
