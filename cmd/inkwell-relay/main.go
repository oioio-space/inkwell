// Command inkwell-relay is a circuit relay that anyone can host: it lets
// inkwell users who cannot reach each other directly (each behind their own
// router) exchange their work through it. It carries bytes that are encrypted
// end to end; it learns who talks to whom, never what is said.
//
// One command runs it, on a machine with a public address and UDP and TCP port
// 7419 open:
//
//	inkwell-relay --key relay.key
//
// The first run creates relay.key (readable by its owner only), which is the
// relay's identity: keep it, since the people who use the relay know it by
// that key. --print-addr prints the relay's addresses to share. It is a static
// binary: build it with CGO_ENABLED=0. See docs/relay.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

func main() {
	os.Exit(exitCode(os.Args[1:], os.Stdout, os.Stderr))
}

// exitCode runs the relay until it fails or is interrupted and returns the
// process exit status, printing a failure on stderr.
func exitCode(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, args, stdout, stderr); err != nil {
		_, _ = fmt.Fprintln(stderr, "inkwell-relay:", err)
		return 1
	}
	return 0
}

// list is a flag that can be given several times, or with comma-separated
// values.
type list []string

func (l *list) String() string { return strings.Join(*l, ",") }

func (l *list) Set(s string) error {
	for v := range strings.SplitSeq(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			*l = append(*l, v)
		}
	}
	return nil
}

// run is main with its environment explicit: it serves until ctx is done.
// Addresses requested by --print-addr go to stdout, logs to stderr.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("inkwell-relay", flag.ContinueOnError)
	flags.SetOutput(stderr)
	keyPath := flags.String("key", "relay.key", "`file` holding the relay's key; created, readable by its owner only, if missing")
	printAddr := flags.Bool("print-addr", false, "print the relay's addresses on standard output once it listens")
	var listen, allow list
	flags.Var(&listen, "listen", "multiaddr to listen on, e.g. /ip4/0.0.0.0/udp/7419/quic-v1; repeatable (default: QUIC and TCP on port 7419, IPv4 and IPv6)")
	flags.Var(&allow, "allow", "peer ID allowed to use the relay; repeatable or comma-separated (default: everyone)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	key, err := loadOrCreateKey(*keyPath)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(stderr, nil))
	opts := p2p.RelayOptions{Listen: listen}
	for _, id := range allow {
		opts.Allow = append(opts.Allow, transport.PeerID(id))
	}
	opts.OnReady = func(addrs []string) {
		log.Info("relay listening", "addrs", addrs, "restricted", len(allow) > 0)
		if *printAddr {
			for _, a := range addrs {
				_, _ = fmt.Fprintln(stdout, a)
			}
		}
	}
	return p2p.RunRelay(ctx, key, opts)
}
