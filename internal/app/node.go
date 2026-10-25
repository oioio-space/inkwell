package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// DefaultPort is offered first to listen on, so the same port can be opened
// once in a firewall.
const DefaultPort = 7419

// homeEnv overrides the directory holding this machine's identity; each
// end-to-end instance gets its own.
const homeEnv = "INKWELL_HOME"

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
func StartNode(ctx context.Context, home *os.Root, port int) (*p2p.Node, error) {
	dev, err := identity.LoadOrCreate(home)
	if err != nil {
		return nil, fmt.Errorf("loading identity: %w", err)
	}
	return startNode(ctx, dev, port, nil)
}

func startNode(ctx context.Context, dev *identity.Device, port int, admit func(transport.PeerID) bool) (*p2p.Node, error) {
	node, err := p2p.New(ctx, dev.Libp2pKey(), p2p.Options{Port: port, Admit: admit})
	if err != nil && port != 0 {
		node, err = p2p.New(ctx, dev.Libp2pKey(), p2p.Options{Admit: admit})
	}
	if err != nil {
		return nil, fmt.Errorf("starting network: %w", err)
	}
	return node, nil
}

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
	store  *opStore
}

// Start starts the node of the device stored in home and wires it to the local
// project, created with title and this device as its admin on the first run.
// Only the members of the project, the peers this user joins and, once
// [Session.Host] has been called, the holders of the invitation can connect.
// The project's member list is kept in home. Canceling ctx or calling
// [Runtime.Close] stops everything; home may be closed once Start returns.
func Start(ctx context.Context, home *os.Root, port int, title string, log *slog.Logger) (*Runtime, error) {
	dev, err := identity.LoadOrCreate(home)
	if err != nil {
		return nil, fmt.Errorf("loading identity: %w", err)
	}
	list, store, err := openProject(home, dev, title)
	if err != nil {
		return nil, fmt.Errorf("opening project: %w", err)
	}
	adm := NewAdmission(list)
	node, err := startNode(ctx, dev, port, adm.Admit)
	if err != nil {
		return nil, errors.Join(err, store.close())
	}
	w := Wire(ctx, node, dev, list, adm, WiringOptions{Log: log, Save: store.save})
	return &Runtime{Node: node, Session: NewSession(node, WithAdmission(adm)), Repo: w.Repo(), wiring: w, store: store}, nil
}

// Close stops the wiring and the node. It is idempotent.
func (r *Runtime) Close() error {
	return errors.Join(r.wiring.Close(), r.store.close())
}
