package mux_test

import (
	"bytes"
	"context"
	"testing"
	"testing/synctest"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
	"github.com/oioio-space/inkwell/internal/transport/transporttest"
)

const (
	chRepo  = 0x01
	chTrust = 0x02
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestContract(t *testing.T) {
	transporttest.Run(t, func(t *testing.T) transporttest.Pair {
		n := memnet.New(1, memnet.Faults{})
		a := mux.Split(n.Join("a"), chRepo)[chRepo]
		b := mux.Split(n.Join("b"), chRepo)[chRepo]
		return transporttest.Pair{
			A:       a,
			B:       b,
			Connect: func(testing.TB) { n.Link("a", "b") },
			Cut:     func(testing.TB) { n.Partition("a", "b") },
		}
	}, transporttest.Options{
		MaxFrame: transport.MaxFrame - mux.Overhead,
		Ordered:  true,
		Synctest: true,
	})
}

// pair returns the raw node of a and the channels of b, linked.
func pair(t *testing.T, channels ...byte) (raw *memnet.Node, subs map[byte]transport.Transport) {
	t.Helper()
	n := memnet.New(1, memnet.Faults{})
	raw = n.Join("a")
	subs = mux.Split(n.Join("b"), channels...)
	t.Cleanup(func() {
		_ = raw.Close()
		for _, s := range subs {
			_ = s.Close()
			break
		}
	})
	n.Link("a", "b")
	return raw, subs
}

// poll returns the queued event of t, if any.
func poll(t transport.Transport) (transport.Event, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ev, err := t.Recv(ctx)
	return ev, err == nil
}

func TestChannelIsolation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		repo, trust := subs[chRepo], subs[chTrust]
		for _, c := range []transport.Transport{repo, trust} {
			if ev, err := c.Recv(t.Context()); err != nil || ev.Kind != transport.PeerUp {
				t.Fatalf("first event = %v, %v; want PeerUp on every channel", ev, err)
			}
		}
		for _, m := range [][]byte{{chRepo, 'r'}, {chTrust, 't'}, {0x09, 'x'}, {}, {chRepo}} {
			if err := raw.Send(t.Context(), "b", m); err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()

		want := map[byte][]string{chRepo: {"r", ""}, chTrust: {"t"}}
		for ch, c := range map[byte]transport.Transport{chRepo: repo, chTrust: trust} {
			var got []string
			for {
				ev, ok := poll(c)
				if !ok {
					break
				}
				got = append(got, string(ev.Payload))
			}
			if len(got) != len(want[ch]) {
				t.Fatalf("channel %#x got %q, want %q", ch, got, want[ch])
			}
			for i := range got {
				if got[i] != want[ch][i] {
					t.Errorf("channel %#x message %d = %q, want %q", ch, i, got[i], want[ch][i])
				}
			}
		}
	})
}

func TestSendPrefixesChannel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		raw := n.Join("a")
		subs := mux.Split(n.Join("b"), chRepo, chTrust)
		defer func() { _ = raw.Close(); _ = subs[chRepo].Close() }()
		n.Link("a", "b")
		if err := subs[chTrust].Send(t.Context(), "a", []byte("hi")); err != nil {
			t.Fatal(err)
		}
		for {
			ev, err := raw.Recv(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if ev.Kind == transport.Message {
				if want := []byte{chTrust, 'h', 'i'}; !bytes.Equal(ev.Payload, want) {
					t.Errorf("wire payload = %v, want %v", ev.Payload, want)
				}
				return
			}
		}
	})
}

func TestWake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		wake := mux.Wake(subs[chTrust])
		<-wake // the PeerUp
		if err := raw.Send(t.Context(), "b", []byte{chRepo, 1}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		select {
		case <-wake:
		default:
			t.Error("no wakeup after a message on another channel")
		}
		if mux.Wake(raw) != nil {
			t.Error("Wake(foreign transport) != nil")
		}
	})
}

func FuzzSplitFrame(f *testing.F) {
	f.Add([]byte{chRepo, 'a'})
	f.Add([]byte{chTrust})
	f.Add([]byte{0x7f, 1, 2, 3})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, frame []byte) {
		if len(frame) > 0 && frame[0] == fragChannel {
			t.Skip("fragments are covered by FuzzFragment")
		}
		synctest.Test(t, func(t *testing.T) {
			raw, subs := pair(t, chRepo, chTrust)
			repo, trust := subs[chRepo], subs[chTrust]
			for _, c := range []transport.Transport{repo, trust} {
				if _, err := c.Recv(t.Context()); err != nil { // PeerUp
					t.Fatal(err)
				}
			}
			for _, m := range [][]byte{frame, {chRepo, 's'}} { // then a sentinel
				if err := raw.Send(t.Context(), "b", m); err != nil {
					t.Fatal(err)
				}
			}
			synctest.Wait()

			var wantRepo, wantTrust []byte
			if len(frame) > 0 {
				switch frame[0] {
				case chRepo:
					wantRepo = frame[1:]
				case chTrust:
					wantTrust = frame[1:]
				}
			}
			if len(frame) > 0 && frame[0] == chRepo {
				ev, ok := poll(repo)
				if !ok || !bytes.Equal(ev.Payload, wantRepo) {
					t.Errorf("repo got %q, %v; want %q", ev.Payload, ok, wantRepo)
				}
			}
			if ev, ok := poll(repo); !ok || string(ev.Payload) != "s" {
				t.Errorf("repo sentinel = %q, %v", ev.Payload, ok)
			}
			if len(frame) > 0 && frame[0] == chTrust {
				if ev, ok := poll(trust); !ok || !bytes.Equal(ev.Payload, wantTrust) {
					t.Errorf("trust got %q, %v; want %q", ev.Payload, ok, wantTrust)
				}
			}
			if ev, ok := poll(trust); ok {
				t.Errorf("trust has an extra event %v %q", ev.Kind, ev.Payload)
			}
			if ev, ok := poll(repo); ok {
				t.Errorf("repo has an extra event %v %q", ev.Kind, ev.Payload)
			}
		})
	})
}
