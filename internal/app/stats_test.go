package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/obs"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// errLeak is what a leakGuard's Send returns for a frame that carries a
// statistic.
var errLeak = errors.New("a frame carries a seeded statistic")

// leakGuard is a node whose Send fails on any frame carrying one of the
// seeded statistics' bytes, and counts the frames it let through.
type leakGuard struct {
	*memnet.Node
	t      *testing.T
	mu     sync.Mutex
	seeds  map[string][]byte
	frames int
}

func (g *leakGuard) seed(what string, b []byte) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seeds[what] = b
}

func (g *leakGuard) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for what, b := range g.seeds {
		if bytes.Contains(payload, b) {
			g.t.Errorf("a frame to %s carries the %s", to, what)
			return errLeak
		}
	}
	g.frames++
	return g.Node.Send(ctx, to, payload)
}

// TestOBS1P3NothingLeaves proves the dynamic Go half of rule P-3: in a
// two-peer session where both peers keep statistics, sync frames and the
// statistics' minute run side by side and no frame on the wire carries a
// seeded statistic (a document id, its pseudonym, a distinctive count, the
// stats key), while the statistics do hold them locally.
func TestOBS1P3NothingLeaves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const doc = "2kVd8QqfX7mRbNpZ3wLcT9hYsJ4e"
		const count = 617283
		net := memnet.New(1, memnet.Faults{})
		devA, devB := newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		if _, err := listA.Issue(devA, trust.OpAdd, devB.Public(), trust.RoleWriter, "b"); err != nil {
			t.Fatal(err)
		}
		seeds := map[string][]byte{"document id": []byte(doc), "count": []byte(strconv.Itoa(count))}
		guards := map[string]*leakGuard{}
		wires := map[string]*Wiring{}
		services := map[string]*StatsService{}
		homes := map[string]string{}
		ctx, stop := context.WithCancel(t.Context())
		var loops sync.WaitGroup
		for name, dev := range map[string]trust.Signer{"a": devA, "b": devB} {
			list := listA
			if name == "b" {
				list = fork(t, listA)
			}
			id := p2p.IDFromKey(dev.Public())
			g := &leakGuard{Node: net.Join(id), t: t, seeds: map[string][]byte{}}
			for what, b := range seeds {
				g.seed(what, b)
			}
			w := Wire(t.Context(), g, dev, list, NewAdmission(list), WiringOptions{})
			t.Cleanup(func() { _ = w.Close() })
			meter := new(obs.Meter)
			nm := newNetMeter(meter)
			homes[name] = t.TempDir()
			st, err := obs.Open(homes[name], obs.Options{Meter: meter})
			if err != nil {
				t.Fatal(err)
			}
			guards[name], wires[name], services[name] = g, w, NewStatsService(st, func() []obs.LinkState { return w.linkStates() })
			loops.Go(func() {
				runStats(ctx, st, nm, func() []obs.LinkState { return w.linkStates() }, slog.New(slog.DiscardHandler))
			})
			t.Cleanup(func() { _ = st.Close() })
		}
		a, b := wires["a"], wires["b"]
		net.Link(a.raw.Local(), b.raw.Local())
		synctest.Wait()
		for _, w := range []*Wiring{a, b} {
			if e := nextEnvelope(t, w.Repo()); e.Kind != bridge.PeerUp {
				t.Fatalf("got %v, want PeerUp", e.Kind)
			}
		}

		words := obs.Descriptor{ID: "activity.words", Kind: obs.KindVolume, Unit: "words", Scope: obs.ScopeDocument, Label: "stats.words", Max: 1 << 40}
		for name, svc := range services {
			if err := svc.Declare([]obs.Descriptor{words}); err != nil {
				t.Fatal(err)
			}
			if err := svc.Flush([]obs.Delta{{M: words.ID, Doc: doc, V: count}}); err != nil {
				t.Fatalf("%s: Flush: %v", name, err)
			}
		}
		// Sync frames both ways through three minutes of statistics.
		for minute := range 3 {
			for i := range 20 {
				frame := []byte("sync " + strconv.Itoa(minute*100+i))
				if err := a.Repo().Send(t.Context(), b.raw.Local(), frame); err != nil {
					t.Fatal(err)
				}
				if err := b.Repo().Send(t.Context(), a.raw.Local(), frame); err != nil {
					t.Fatal(err)
				}
			}
			for _, w := range []*Wiring{a, b} {
				for range 20 {
					if e := nextEnvelope(t, w.Repo()); e.Kind != bridge.Message {
						t.Fatalf("got %v, want a message", e.Kind)
					}
				}
			}
			if minute == 0 {
				time.Sleep(statsEvery + time.Second)
				synctest.Wait()
				// Once written, the key and the pseudonym are seeds too.
				for name := range guards {
					key, err := os.ReadFile(filepath.Join(homes[name], "stats", "key"))
					if err != nil {
						t.Fatalf("%s: the statistics were not written: %v", name, err)
					}
					mac := hmac.New(sha256.New, key)
					mac.Write([]byte(doc))
					for _, other := range guards {
						other.seed(name+"'s stats key", key)
						other.seed(name+"'s pseudonym", []byte(hex.EncodeToString(mac.Sum(nil)[:8])))
					}
				}
				continue
			}
			for _, svc := range services {
				_ = svc.Network()
				if _, err := svc.Days(); err != nil {
					t.Fatal(err)
				}
			}
			time.Sleep(statsEvery)
			synctest.Wait()
		}
		stop()
		loops.Wait()

		for name, g := range guards {
			if g.frames < 60 {
				t.Errorf("%s sent %d frames, want at least the 60 sync frames", name, g.frames)
			}
			days, err := services[name].Days()
			if err != nil {
				t.Fatal(err)
			}
			var words, sent uint64
			for _, r := range days[len(days)-1].Records {
				switch r.M {
				case "activity.words":
					words += r.Sum
				case "network.sent":
					sent += r.Sum
				}
			}
			if words != count || sent == 0 {
				t.Errorf("%s keeps %d words and %d bytes sent, want %d words and the sync frames' bytes", name, words, sent, count)
			}
		}
	})
}
