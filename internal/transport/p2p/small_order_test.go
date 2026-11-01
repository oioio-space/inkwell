package p2p

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/oioio-space/inkwell/internal/transport"
)

func TestSmallOrderSecuredBeforeAdmission(t *testing.T) {
	const id = transport.PeerID("12D3KooW9tGaPdJo5jmCpadQ971nfiq4kLcQjeBPYTfutBTtckPH")
	p, err := peer.Decode(string(id))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := KeyFromID(id); err == nil {
		t.Fatal("accepted neutral peer ID")
	}
	key := make([]byte, 32)
	key[0] = 1
	if IDFromKey(key) != "" {
		t.Fatal("accepted neutral key")
	}
	for _, relay := range []bool{false, true} {
		called := false
		g := gate{admit: func(peer.ID) bool { called = true; return true }}
		if relay {
			g.relays = map[peer.ID]peer.AddrInfo{p: {ID: p}}
		}
		if g.InterceptSecured(network.DirInbound, p, nil) || called {
			t.Fatal("invalid peer reached admission")
		}
	}
}

func TestRelayInstallsKeyGater(t *testing.T) {
	stop := errors.New("inspected relay config")
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := peer.Decode("12D3KooW9tGaPdJo5jmCpadQ971nfiq4kLcQjeBPYTfutBTtckPH")
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := peer.Decode(string(IDFromKey(pub)))
	if err != nil {
		t.Fatal(err)
	}
	err = RunRelay(t.Context(), priv, RelayOptions{extra: []libp2p.Option{func(c *config.Config) error {
		if c.ConnectionGater == nil {
			t.Fatal("missing relay gater")
		}
		for _, direction := range []network.Direction{network.DirInbound, network.DirOutbound} {
			if c.ConnectionGater.InterceptSecured(direction, invalid, nil) {
				t.Fatal("relay admitted small-order peer")
			}
			if !c.ConnectionGater.InterceptSecured(direction, valid, nil) {
				t.Fatal("relay rejected honest peer")
			}
		}
		return stop
	}}})
	if !errors.Is(err, stop) {
		t.Fatalf("RunRelay: %v", err)
	}
}
