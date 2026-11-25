package app

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"time"

	"github.com/oioio-space/inkwell/internal/obs"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// statsEvery is how often the statistics are summed and written (plan
// docs/plans/2026-11-24-obs1-observability.md §3): a crash loses at most one.
const statsEvery = time.Minute

// linkKinds are the values the network metrics are split by.
var linkKinds = []string{obs.LinkLAN, obs.LinkDirect, obs.LinkRelayed}

// errNoStats is what the statistics service answers when the store could not
// be opened: the application works without it.
var errNoStats = errors.New("statistics are unavailable")

// netMeter counts the network's bytes and connections by kind of link, from
// the per-link counters of the wiring's channels (mux.Traffic), summed when
// sampled.
type netMeter struct {
	sent, received *obs.Volume
	opened         *obs.Counter
	last           map[transport.PeerID]mux.LinkTraffic
}

func newNetMeter(m *obs.Meter) *netMeter {
	bytes := func(id, label string) *obs.Volume {
		return m.Volume(obs.Descriptor{ID: id, Unit: "bytes", Scope: obs.ScopeApp, Label: label, Max: 1 << 50, Values: linkKinds})
	}
	return &netMeter{
		sent:     bytes("network.sent", "metric.network.sent"),
		received: bytes("network.received", "metric.network.received"),
		opened: m.Counter(obs.Descriptor{
			ID: "network.connections", Scope: obs.ScopeApp,
			Label: "metric.network.connections", Max: 1 << 30, Values: linkKinds,
		}),
		last: map[transport.PeerID]mux.LinkTraffic{},
	}
}

// sample adds what crossed each link since the last sample, under the link's
// kind now; a link whose kind is unknown is skipped until it has one.
func (n *netMeter) sample(states []obs.LinkState) {
	for _, s := range states {
		peer := transport.PeerID(s.Peer)
		was := n.last[peer]
		if s.Kind == "" || s.Sent < was.Sent || s.Received < was.Received || s.Opened < was.Opened {
			continue
		}
		n.sent.AddBy(s.Kind, s.Sent-was.Sent)
		n.received.AddBy(s.Kind, s.Received-was.Received)
		n.opened.AddBy(s.Kind, s.Opened-was.Opened)
		n.last[peer] = mux.LinkTraffic{Peer: peer, Sent: s.Sent, Received: s.Received, Opened: s.Opened}
	}
}

// linkStates returns what each link carried, with its kind and figures: the
// bytes of every channel of the wiring and of each counted lane (the blobs'),
// summed per peer, and the link figures of the node.
func (w *Wiring) linkStates(lanes ...transport.Transport) []obs.LinkState {
	figures := map[transport.PeerID]transport.Link{}
	if src, ok := w.raw.(transport.LinkSource); ok {
		for _, l := range src.Links() {
			figures[l.Peer] = l
		}
	}
	byPeer := map[transport.PeerID]*obs.LinkState{}
	var out []*obs.LinkState
	for _, t := range append([]transport.Transport{w.repoCh}, lanes...) {
		for _, tr := range mux.Traffic(t) {
			s := byPeer[tr.Peer]
			if s == nil {
				l := figures[tr.Peer]
				s = &obs.LinkState{
					Peer: string(tr.Peer), Kind: linkKind(l.Kind), RTT: l.RTT, Downs: l.DownsTotal,
					Resyncs: l.Resyncs, LastDown: l.LastDown,
				}
				byPeer[tr.Peer] = s
				out = append(out, s)
			}
			s.Sent += tr.Sent
			s.Received += tr.Received
			s.Opened += tr.Opened
		}
	}
	states := make([]obs.LinkState, len(out))
	for i, s := range out {
		states[i] = *s
	}
	return states
}

func linkKind(k transport.LinkKind) string {
	switch k {
	case transport.LinkLAN:
		return obs.LinkLAN
	case transport.LinkDirect:
		return obs.LinkDirect
	case transport.LinkRelayed:
		return obs.LinkRelayed
	default:
		return ""
	}
}

// StatsService is the Wails service of the local statistics (rule 10 of
// docs/ux-principles.md): the webview declares its metrics and hands its
// deltas over, and reads, exports, erases or switches off what is kept.
// Nothing it holds leaves the device. Create one with [NewStatsService].
type StatsService struct {
	store *obs.Store
	links func() []obs.LinkState
	salt  []byte // the session's, for the debug snapshot's pseudonyms
}

// NewStatsService returns the service over store, which may be nil when it
// could not be opened; links gives the network debug snapshot its links.
func NewStatsService(store *obs.Store, links func() []obs.LinkState) *StatsService {
	salt := make([]byte, 32)
	_, _ = rand.Read(salt) // never fails (crypto/rand)
	return &StatsService{store: store, links: links, salt: salt}
}

// Declare declares the webview's metrics, checked under rule V.
func (s *StatsService) Declare(ds []obs.Descriptor) error {
	if s.store == nil {
		return errNoStats
	}
	return s.store.Declare(ds)
}

// Descriptors returns every declared metric, Go's own included, so that the
// webview's registry refuses an id Go already holds.
func (s *StatsService) Descriptors() []obs.Descriptor {
	if s.store == nil {
		return nil
	}
	return s.store.Descriptors()
}

// Flush adds the webview's deltas since its last flush, checked under rule V;
// the store writes them with its next minute.
func (s *StatsService) Flush(batch []obs.Delta) error {
	if s.store == nil {
		return errNoStats
	}
	return s.store.Add(batch)
}

// Days returns every kept day, today's included.
func (s *StatsService) Days() ([]obs.Day, error) {
	if s.store == nil {
		return nil, errNoStats
	}
	return s.store.Days()
}

// Export asks the person, in a native save dialog, where to write the
// pseudonymous records, and returns the path written, or "" when they
// cancelled; refused inside an open project. The webview gives no path.
func (s *StatsService) Export() (string, error) {
	if s.store == nil {
		return "", errNoStats
	}
	return s.store.Export()
}

// Erase removes every statistics file and the key.
func (s *StatsService) Erase() error {
	if s.store == nil {
		return errNoStats
	}
	return s.store.Erase()
}

// SetEnabled switches the keeping of statistics on or off.
func (s *StatsService) SetEnabled(on bool) error {
	if s.store == nil {
		return errNoStats
	}
	return s.store.SetEnabled(on)
}

// Enabled reports whether statistics are kept.
func (s *StatsService) Enabled() bool { return s.store != nil && s.store.Enabled() }

// Network returns the network's debug snapshot: each link's kind, figures and
// bytes, the peer as a pseudonym for this session only (rule V-3).
func (s *StatsService) Network() obs.NetworkSnapshot {
	var links []obs.LinkState
	if s.links != nil {
		links = s.links()
	}
	return obs.Network(s.salt, links)
}

// runStats samples the network once a minute and writes the statistics,
// until ctx is done; then it writes them a last time.
func runStats(ctx context.Context, st *obs.Store, nm *netMeter, links func() []obs.LinkState, log *slog.Logger) {
	t := time.NewTicker(statsEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case <-ctx.Done():
			nm.sample(links())
			return
		}
		nm.sample(links())
		if err := st.Flush(); err != nil {
			log.Warn("writing the statistics", "err", err)
		}
	}
}
