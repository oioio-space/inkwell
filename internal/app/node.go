package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// DefaultPort is offered first to listen on, so the same port can be opened
// once in a firewall.
const DefaultPort = 7419

// homeEnv overrides the directory holding this machine's identity; each
// end-to-end instance gets its own.
const homeEnv = "INKWELL_HOME"

// Network settings read from the environment until the settings screen exists
// (stage 2, task 11); the end-to-end and network-simulation tests rely on them.
const (
	// relaysEnv lists the relays to use, as multiaddrs ending in /p2p/<id>,
	// separated by commas or spaces.
	relaysEnv = "INKWELL_RELAYS"
	// netEnv set to relayOnly hides this machine's address: it talks to the
	// others through a relay only.
	netEnv    = "INKWELL_NET"
	relayOnly = "relay-only"
)

// OpenHome opens the directory holding this machine's identity: $INKWELL_HOME
// or, by default, "inkwell" in the user's configuration directory. It is
// created, private to the user, when missing. The caller closes the result.
func OpenHome() (*os.Root, error) {
	dir := os.Getenv(homeEnv)
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("locating configuration directory: %w", err)
		}
		dir = filepath.Join(base, "inkwell")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", dir, err)
	}
	return root, nil
}

// StartNode starts the network node of the device stored in home, listening
// on port, or on a free port when port is busy. Canceling ctx stops the node.
// The relays and the relay-only mode come from $INKWELL_RELAYS and
// $INKWELL_NET=relay-only; an invalid combination is an error, never a silent
// fall back to direct connections.
//
// TODO(Task 7): pass Options.Admit from the signed member list; nil admits
// everyone who holds an invitation.
func StartNode(ctx context.Context, home *os.Root, port int) (*p2p.Node, error) {
	dev, err := identity.LoadOrCreate(home)
	if err != nil {
		return nil, fmt.Errorf("loading identity: %w", err)
	}
	opts := p2p.Options{
		Port:      port,
		Relays:    strings.FieldsFunc(os.Getenv(relaysEnv), func(r rune) bool { return r == ',' || r == ' ' }),
		RelayOnly: os.Getenv(netEnv) == relayOnly,
	}
	node, err := p2p.New(ctx, dev.Libp2pKey(), opts)
	if err != nil && port != 0 {
		opts.Port = 0
		node, err = p2p.New(ctx, dev.Libp2pKey(), opts)
	}
	if err != nil {
		return nil, fmt.Errorf("starting network: %w", err)
	}
	return node, nil
}
