package memnet_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// recvN reads n events from node, failing the test if the context expires first.
func recvN(t *testing.T, node transport.Transport, n int) []transport.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var got []transport.Event
	for range n {
		ev, err := node.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() after %d events: %v", len(got), err)
		}
		got = append(got, ev)
	}
	return got
}

func TestLinkEmitsPeerUpBothSides(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")
		n.Link("a", "b")

		if got, want := recvN(t, a, 1), []transport.Event{{Kind: transport.PeerUp, Peer: "b"}}; !cmp.Equal(got, want) {
			t.Errorf("a events: %v", cmp.Diff(want, got))
		}
		if got, want := recvN(t, b, 1), []transport.Event{{Kind: transport.PeerUp, Peer: "a"}}; !cmp.Equal(got, want) {
			t.Errorf("b events: %v", cmp.Diff(want, got))
		}
		if got, want := a.Peers(), []transport.PeerID{"b"}; !slices.Equal(got, want) {
			t.Errorf("a.Peers() = %v, want %v", got, want)
		}
	})
}

func TestSendDeliversToTargetOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b, c := n.Join("a"), n.Join("b"), n.Join("c")
		n.Link("a", "b")
		n.Link("a", "c")
		recvN(t, b, 1)
		recvN(t, c, 1)

		payload := []byte("hello")
		if err := a.Send(t.Context(), "b", payload); err != nil {
			t.Fatalf("Send() = %v", err)
		}
		payload[0] = 'X' // the network must own a copy

		want := []transport.Event{{Kind: transport.Message, Peer: "a", Payload: []byte("hello")}}
		if got := recvN(t, b, 1); !cmp.Equal(got, want) {
			t.Errorf("b events: %v", cmp.Diff(want, got))
		}
		synctest.Wait()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if ev, err := c.Recv(ctx); !errors.Is(err, context.Canceled) {
			t.Errorf("c.Recv() = %v, %v; want no event", ev, err)
		}
	})
}

func TestPartitionEmitsPeerDownAndRejectsSend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")
		n.Link("a", "b")
		recvN(t, a, 1)
		recvN(t, b, 1)

		n.Partition("a", "b")

		if got, want := recvN(t, a, 1), []transport.Event{{Kind: transport.PeerDown, Peer: "b"}}; !cmp.Equal(got, want) {
			t.Errorf("a events: %v", cmp.Diff(want, got))
		}
		if err := a.Send(t.Context(), "b", []byte("x")); !errors.Is(err, transport.ErrUnknownPeer) {
			t.Errorf("Send() after partition = %v, want ErrUnknownPeer", err)
		}
		if got := a.Peers(); len(got) != 0 {
			t.Errorf("a.Peers() = %v, want none", got)
		}
	})
}

// scenario runs a fixed exchange on a faulty network and returns what b saw.
func scenario(t *testing.T, seed uint64) []transport.Event {
	t.Helper()
	var got []transport.Event
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(seed, memnet.Faults{Drop: 0.2, Duplicate: 0.2, MaxDelay: 50 * time.Millisecond})
		a, b := n.Join("a"), n.Join("b")
		n.Link("a", "b")
		recvN(t, a, 1)
		for i := range 50 {
			if err := a.Send(t.Context(), "b", []byte{byte(i)}); err != nil {
				t.Fatalf("Send(%d) = %v", i, err)
			}
		}
		time.Sleep(time.Second) // virtual: lets every delayed delivery land
		synctest.Wait()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		for {
			ev, err := b.Recv(ctx)
			if err != nil {
				break
			}
			got = append(got, ev)
		}
		_ = a.Close()
		_ = b.Close()
	})
	return got
}

func TestFaultsAreDeterministicForSeed(t *testing.T) {
	first, second := scenario(t, 42), scenario(t, 42)
	if !cmp.Equal(first, second) {
		t.Errorf("same seed, different runs: %v", cmp.Diff(first, second))
	}
	if other := scenario(t, 7); cmp.Equal(first, other) {
		t.Error("different seeds produced identical runs; faults are not applied")
	}
}

func TestRecvAfterCloseReturnsErrClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		if err := a.Close(); err != nil {
			t.Fatalf("Close() = %v", err)
		}
		if _, err := a.Recv(t.Context()); !errors.Is(err, transport.ErrClosed) {
			t.Errorf("Recv() after Close = %v, want ErrClosed", err)
		}
		if err := a.Send(t.Context(), "b", nil); !errors.Is(err, transport.ErrClosed) {
			t.Errorf("Send() after Close = %v, want ErrClosed", err)
		}
	})
}

func TestCloseNotifiesLinkedPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")
		n.Link("a", "b")
		recvN(t, b, 1)

		_ = a.Close()

		if got, want := recvN(t, b, 1), []transport.Event{{Kind: transport.PeerDown, Peer: "a"}}; !cmp.Equal(got, want) {
			t.Errorf("b events: %v", cmp.Diff(want, got))
		}
	})
}

func TestRecvBlocksUntilEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")

		done := make(chan transport.Event)
		go func() {
			ev, err := a.Recv(t.Context())
			if err != nil {
				t.Errorf("Recv() = %v", err)
			}
			done <- ev
		}()
		synctest.Wait() // a is durably blocked in Recv
		n.Link("a", "b")
		if ev := <-done; ev.Kind != transport.PeerUp {
			t.Errorf("Recv() = %v, want PeerUp", ev)
		}
		_ = b
	})
}

func TestJoinPanicsOnMisuse(t *testing.T) {
	for _, id := range []transport.PeerID{"", "dup"} {
		t.Run(string(id), func(t *testing.T) {
			n := memnet.New(1, memnet.Faults{})
			n.Join("dup")
			defer func() {
				if recover() == nil {
					t.Errorf("Join(%q) did not panic", id)
				}
			}()
			n.Join(id)
		})
	}
}

func TestLinkIgnoresSelfUnknownAndRepeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, _ := n.Join("a"), n.Join("b")
		n.Link("a", "a")
		n.Link("a", "ghost")
		n.Link("b", "a")
		n.Link("a", "b") // already linked: no second PeerUp
		n.Partition("a", "ghost")

		if got, want := recvN(t, a, 1), []transport.Event{{Kind: transport.PeerUp, Peer: "b"}}; !cmp.Equal(got, want) {
			t.Errorf("a events: %v", cmp.Diff(want, got))
		}
		synctest.Wait()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if ev, err := a.Recv(ctx); err == nil {
			t.Errorf("unexpected extra event %v", ev)
		}
		if got := a.Local(); got != "a" {
			t.Errorf("Local() = %q, want %q", got, "a")
		}
	})
}

func TestSendRejectsOversizeAndCancelled(t *testing.T) {
	n := memnet.New(1, memnet.Faults{})
	a, _ := n.Join("a"), n.Join("b")
	n.Link("a", "b")
	if err := a.Send(t.Context(), "b", make([]byte, transport.MaxFrame+1)); !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("Send(oversize) = %v, want ErrFrameTooLarge", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := a.Send(ctx, "b", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Send(cancelled ctx) = %v, want context.Canceled", err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil", err)
	}
}

func TestRelinkDropsTrafficFromPreviousLink(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{MaxDelay: time.Second})
		a, b := n.Join("a"), n.Join("b")
		n.Link("a", "b")
		recvN(t, b, 1)
		for i := range 20 {
			if err := a.Send(t.Context(), "b", []byte{byte(i)}); err != nil {
				t.Fatal(err)
			}
		}
		n.Partition("a", "b")
		n.Link("a", "b")
		time.Sleep(2 * time.Second) // every delayed delivery is now due
		synctest.Wait()

		want := []transport.Event{{Kind: transport.PeerDown, Peer: "a"}, {Kind: transport.PeerUp, Peer: "a"}}
		if got := recvN(t, b, 2); !cmp.Equal(got, want) {
			t.Errorf("b events: %v", cmp.Diff(want, got))
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if ev, err := b.Recv(ctx); err == nil {
			t.Errorf("stale message delivered across re-link: %v", ev)
		}
	})
}

func TestCloseWakesBlockedRecv(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		done := make(chan error)
		go func() {
			_, err := a.Recv(t.Context())
			done <- err
		}()
		synctest.Wait()
		_ = a.Close()
		if err := <-done; !errors.Is(err, transport.ErrClosed) {
			t.Errorf("blocked Recv() after Close = %v, want ErrClosed", err)
		}
	})
}
