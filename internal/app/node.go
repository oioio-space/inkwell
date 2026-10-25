package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport"
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
// The node admits every peer; [Start] restricts it to the project's members.
// The relays and the relay-only mode come from $INKWELL_RELAYS and
// $INKWELL_NET=relay-only; an invalid combination is an error, never a silent
// fall back to direct connections.
func StartNode(ctx context.Context, home *os.Root, port int) (*p2p.Node, error) {
	dev, err := identity.LoadOrCreate(home)
	if err != nil {
		return nil, fmt.Errorf("loading identity: %w", err)
	}
	return startNode(ctx, dev, netOptions(port))
}

// netOptions returns the network options that come from the environment
// (relays, relay-only mode) for a node listening on port.
func netOptions(port int) p2p.Options {
	return p2p.Options{
		Port:      port,
		Relays:    strings.FieldsFunc(os.Getenv(relaysEnv), func(r rune) bool { return r == ',' || r == ' ' }),
		RelayOnly: os.Getenv(netEnv) == relayOnly,
	}
}

func startNode(ctx context.Context, dev *identity.Device, o p2p.Options) (*p2p.Node, error) {
	node, err := p2p.New(ctx, dev.Libp2pKey(), o)
	if err != nil && o.Port != 0 {
		o.Port = 0
		node, err = p2p.New(ctx, dev.Libp2pKey(), o)
	}
	if err != nil {
		return nil, fmt.Errorf("starting network: %w", err)
	}
	return node, nil
}

// Options tunes [Start]. The zero value is valid.
type Options struct {
	// Log receives the diagnostics, never payload bytes. Nil discards them.
	Log *slog.Logger
	// Project is the open project the node serves; it must stay open until
	// [Runtime.Close]. It is required.
	Project *project.Project
	// RendezvousURL is the Wormhole rendezvous server that carries the short
	// codes: empty for the public magic-wormhole server, [NoCodes] for none.
	RendezvousURL string
	// Port is offered first to listen on; zero picks a free one.
	Port int
	// LAN turns the local network discovery on (the setting "Découverte sur
	// le réseau local"): this device then announces itself on the local
	// network and sees the others that are visible.
	LAN bool
}

// NoCodes as [Options.RendezvousURL] turns the short codes off: invitations
// are then the text only, and nothing is sent to a code server.
const NoCodes = "off"

// Runtime is a running node with the trust wiring of the local project.
type Runtime struct {
	// Node is the network node.
	Node *p2p.Node
	// Session is the service bound to the frontend.
	Session *Session
	// Repo is the transport for the bridge: the frames of the project's
	// members only (see [Wiring.Repo]).
	Repo transport.Transport

	wiring *Wiring
	inv    *invites
}

// Start starts the node of the device stored in home and wires it to the open
// project o.Project, whose member list gets this device as its admin on the
// first run. Only the members of the project can connect, apart from the
// people who are being invited (see [Session.Invite] and [Session.SetVisible]).
// The member list is kept in the project folder (.inkwell/trust), and so is
// the project the device joins. Canceling ctx or calling [Runtime.Close] stops
// everything; home may be closed once Start returns, and o.Project must stay
// open until Close.
func Start(ctx context.Context, home *os.Root, o Options) (*Runtime, error) {
	dev, err := identity.LoadOrCreate(home)
	if err != nil {
		return nil, fmt.Errorf("loading identity: %w", err)
	}
	list, store, err := openTrust(o.Project, dev)
	if err != nil {
		return nil, fmt.Errorf("opening project: %w", err)
	}
	adm := NewAdmission(list)
	no := netOptions(o.Port)
	no.Admit, no.LAN = adm.Admit, o.LAN
	node, err := startNode(ctx, dev, no)
	if err != nil {
		return nil, errors.Join(err, store.close())
	}
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	w := Wire(ctx, node, dev, list, adm, WiringOptions{Log: log, Save: store.save})
	var c carrier
	if o.RendezvousURL != NoCodes {
		c = invite.Wormhole{RendezvousURL: o.RendezvousURL}
	}
	inv := newInvites(ctx, node, w, dev, list, store, c, log)
	s := NewSession(node)
	s.inv = inv
	return &Runtime{Node: node, Session: s, Repo: w.Repo(), wiring: w, inv: inv}, nil
}

// Close stops the wiring and the node. It is idempotent.
func (r *Runtime) Close() error {
	return errors.Join(r.wiring.Close(), r.inv.close())
}
