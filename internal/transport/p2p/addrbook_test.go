package p2p

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/record"
	ma "github.com/multiformats/go-multiaddr"
)

// bookNow is the clock of the address-book tests that do not run in a synctest bubble.
var bookNow = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

// bookID is the peer ID of the Ed25519 key of seed.
func bookID(t testing.TB, seed int) peer.ID {
	t.Helper()
	var s [ed25519.SeedSize]byte
	s[0], s[1] = byte(seed), byte(seed>>8)
	pk, err := crypto.UnmarshalEd25519PublicKey(ed25519.NewKeyFromSeed(s[:]).Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func addrs(ss ...string) []ma.Multiaddr {
	out := make([]ma.Multiaddr, len(ss))
	for i, s := range ss {
		out[i] = ma.StringCast(s)
	}
	return out
}

func strs(as []ma.Multiaddr) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.String()
	}
	return out
}

// always is a membership check that accepts.
func always() bool { return true }

// testBook is a book in memory only.
func testBook(t testing.TB) *addrBook {
	t.Helper()
	return newAddrBook(nil, bookID(t, 0), bookNow, discard)
}

// dialable is what the book dials for id, authenticated addresses first.
func (b *addrBook) dialable(id peer.ID) []string {
	auth, lan := b.dialAddrs(id)
	return strs(slices.Concat(auth, lan))
}

// bookText is a stored book of one entry per id, seen at seen, kept for project "p", with the
// addresses given.
func bookText(seen time.Time, entries map[peer.ID][]string) string {
	var b strings.Builder
	b.WriteString(`{"v":1,"peers":[`)
	for i, id := range slices.Sorted(maps.Keys(entries)) {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":%q,"seen":%d,"in":["p"],"addrs":[`, id.String(), seen.Unix())
		for j, a := range entries[id] {
			if j > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, "%q", a)
		}
		b.WriteString("]}")
	}
	b.WriteString("]}")
	return b.String()
}

// TestBookRoundTrip: §3, an encoded book decodes to the same entries.
func TestBookRoundTrip(t *testing.T) {
	self := bookID(t, 0)
	in := map[peer.ID]bookEntry{
		bookID(t, 1): {seen: bookNow.Unix(), in: []string{"p"}, addrs: addrs("/ip4/192.168.1.20/udp/7419/quic-v1", "/ip4/192.168.1.20/tcp/7419")},
		bookID(t, 2): {seen: bookNow.Add(-time.Hour).Unix(), in: []string{"p", "q-2"}, addrs: addrs("/ip6/2001:db8::1/udp/7419/quic-v1")},
		bookID(t, 3): {seen: 0, in: []string{"P_3"}, addrs: addrs("/dns/example.org/tcp/7419", "/ip4/127.0.0.1/tcp/1")},
	}
	data, skipped, err := encodeBook(in)
	if err != nil || skipped != 0 {
		t.Fatalf("encodeBook = %d skipped, %v", skipped, err)
	}
	out, err := decodeBook(data, self)
	if err != nil {
		t.Fatalf("decodeBook(encodeBook(book)) = %v", err)
	}
	if !sameEntries(in, out) {
		got, _, _ := encodeBook(out)
		t.Errorf("decodeBook(encodeBook(book)) = %s, want %s", got, data)
	}
}

// sameEntries reports whether two books hold the same stored entries.
func sameEntries(a, b map[peer.ID]bookEntry) bool {
	stored := func(m map[peer.ID]bookEntry) map[peer.ID]bookEntry {
		return maps.Collect(func(yield func(peer.ID, bookEntry) bool) {
			for id, e := range m {
				if len(e.addrs) > 0 && len(e.in) > 0 && !yield(id, e) {
					return
				}
			}
		})
	}
	a, b = stored(a), stored(b)
	if len(a) != len(b) {
		return false
	}
	for id, e := range a {
		f, ok := b[id]
		if !ok || e.seen != f.seen || !slices.Equal(e.in, f.in) || !slices.EqualFunc(e.addrs, f.addrs, ma.Multiaddr.Equal) {
			return false
		}
	}
	return true
}

// bookRejects is the rejection table of §3: each row breaks one rule, and the whole book is
// refused, for the reason (category) given.
func bookRejects(t testing.TB) map[string]struct{ data, reason string } {
	self, a, b := bookID(t, 0), bookID(t, 1), bookID(t, 2)
	good := "/ip4/192.168.1.20/udp/7419/quic-v1"
	one := func(addrs ...string) string { return bookText(bookNow, map[peer.ID][]string{a: addrs}) }
	nine := make([]string, 9)
	for i := range nine {
		nine[i] = fmt.Sprintf("/ip4/10.0.0.%d/tcp/7419", i+1)
	}
	many := make(map[peer.ID][]string, bookMaxPeers+1)
	for i := range bookMaxPeers + 1 {
		many[bookID(t, 100+i)] = []string{good}
	}
	secp, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secpID, err := peer.IDFromPrivateKey(secp)
	if err != nil {
		t.Fatal(err)
	}
	padded := one(good)
	padded = padded[:len(padded)-1] + strings.Repeat(" ", 97<<10) + "}"
	in := func(list string) string { return strings.Replace(one(good), `"in":["p"]`, `"in":`+list, 1) }
	type row = struct{ data, reason string }
	return map[string]row{
		"version 0":            {strings.Replace(one(good), `"v":1`, `"v":0`, 1), "version"},
		"version 2":            {strings.Replace(one(good), `"v":1`, `"v":2`, 1), "version"},
		"no version":           {strings.Replace(one(good), `"v":1,`, ``, 1), "version"},
		"duplicate name":       {strings.Replace(one(good), `"v":1`, `"v":1,"v":1`, 1), "syntax"},
		"unknown name":         {strings.Replace(one(good), `"v":1`, `"v":1,"x":1`, 1), "syntax"},
		"name in another case": {strings.Replace(one(good), `"v":1`, `"V":1`, 1), "syntax"},
		"bad id":               {strings.Replace(one(good), a.String(), "12D3KooWnotapeerid", 1), "bounds"},
		"non-Ed25519 id":       {strings.Replace(one(good), a.String(), secpID.String(), 1), "bounds"},
		"own id":               {bookText(bookNow, map[peer.ID][]string{self: {good}}), "bounds"},
		"duplicate peer":       {strings.Replace(bookText(bookNow, map[peer.ID][]string{a: {good}, b: {good}}), b.String(), a.String(), 1), "bounds"},
		"no address":           {one(), "bounds"},
		"nine addresses":       {one(nine...), "bounds"},
		"duplicate address":    {one(good, good), "bounds"},
		"circuit":              {one("/ip4/1.2.3.4/udp/7419/quic-v1/p2p/" + b.String() + "/p2p-circuit"), "bounds"},
		"p2p suffix":           {one(good + "/p2p/" + a.String()), "bounds"},
		"unspecified":          {one("/ip4/0.0.0.0/tcp/7419"), "bounds"},
		"unspecified ip6":      {one("/ip6/::/udp/7419/quic-v1"), "bounds"},
		"multicast":            {one("/ip4/224.0.0.251/udp/7419/quic-v1"), "bounds"},
		"port zero":            {one("/ip4/10.0.0.1/tcp/0"), "bounds"},
		"websocket":            {one("/ip4/10.0.0.1/tcp/7419/ws"), "bounds"},
		"quic draft":           {one("/ip4/10.0.0.1/udp/7419/quic"), "bounds"},
		"not canonical":        {one("/ip4/010.0.0.1/tcp/7419"), "bounds"},
		"not a multiaddr":      {one("hello"), "bounds"},
		"dns not a hostname":   {one("/dns/a_b.example/tcp/7419"), "bounds"},
		"dns label hyphen":     {one("/dns/-a.example/tcp/7419"), "bounds"},
		"address of 129 bytes": {one("/dns/" + strings.Repeat("a", 60) + "." + strings.Repeat("a", 129-len("/dns/./tcp/7419")-60) + "/tcp/7419"), "bounds"},
		"negative seen":        {strings.Replace(one(good), fmt.Sprintf(`"seen":%d`, bookNow.Unix()), `"seen":-1`, 1), "bounds"},
		"no project":           {in(`[]`), "bounds"},
		"nine projects":        {in(`["a","b","c","d","e","f","g","h","i"]`), "bounds"},
		"duplicate project":    {in(`["p","p"]`), "bounds"},
		"project name":         {in(`["p q"]`), "bounds"},
		"project of 65 bytes":  {in(`["` + strings.Repeat("p", 65) + `"]`), "bounds"},
		"97 KiB":               {padded, "size"},
		"65 peers":             {bookText(bookNow, many), "bounds"},
		"invalid UTF-8":        {strings.Replace(one(good), good, "/dns/\xff\xfe/tcp/7419", 1), "syntax"},
		"trailing bytes":       {one(good) + "{}", "syntax"},
		"not an object":        {`[1]`, "syntax"},
		"empty":                {``, "syntax"},
	}
}

// TestBookDecodeRejects: §3, one row per rule, each with its reason category; a valid book next
// to each proves the row breaks that rule only. Seen times never refuse a book.
func TestBookDecodeRejects(t *testing.T) {
	self, a := bookID(t, 0), bookID(t, 1)
	valid := bookText(bookNow, map[peer.ID][]string{a: {"/ip4/192.168.1.20/udp/7419/quic-v1"}})
	if _, err := decodeBook([]byte(valid), self); err != nil {
		t.Fatalf("decodeBook(valid) = %v", err)
	}
	edge := "/dns/" + strings.Repeat("a", 60) + "." + strings.Repeat("a", 128-len("/dns/./tcp/7419")-60) + "/tcp/7419"
	for _, seen := range []time.Time{time.Unix(0, 0), bookNow.Add(10 * 365 * 24 * time.Hour)} {
		if _, err := decodeBook([]byte(bookText(seen, map[peer.ID][]string{a: {edge}})), self); err != nil {
			t.Errorf("decodeBook(seen %v, address of 128 bytes) = %v, want accepted", seen, err)
		}
	}
	for name, row := range bookRejects(t) {
		got, err := decodeBook([]byte(row.data), self)
		if err == nil {
			t.Errorf("decodeBook(%s) accepted %d peers, want an error", name, len(got))
			continue
		}
		if r, ok := errors.AsType[*bookRefusal](err); !ok || r.reason != row.reason {
			t.Errorf("decodeBook(%s) refused as %v, want reason %q", name, err, row.reason)
		}
	}
}

// FuzzDecodeBook: the decoder never panics, never accepts a book beyond the bounds, and an
// accepted book re-encodes to the same entries.
func FuzzDecodeBook(f *testing.F) {
	self, a := bookID(f, 0), bookID(f, 1)
	f.Add([]byte(bookText(bookNow, map[peer.ID][]string{a: {"/ip4/192.168.1.20/udp/7419/quic-v1", "/ip6/::1/tcp/7419"}})))
	for _, row := range bookRejects(f) {
		f.Add([]byte(row.data))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeBook(data, self)
		if err != nil {
			return
		}
		if len(data) > MaxBookSize || len(got) > bookMaxPeers {
			t.Fatalf("accepted %d bytes, %d peers", len(data), len(got))
		}
		for id, e := range got {
			if id == self || len(e.addrs) == 0 || len(e.addrs) > bookMaxAddrs || len(e.in) == 0 || len(e.in) > bookMaxScopes {
				t.Fatalf("accepted peer %s with %d addresses, %d projects", id, len(e.addrs), len(e.in))
			}
			for _, x := range e.addrs {
				if !bookAddrOK(x) {
					t.Fatalf("accepted address %q", x.String())
				}
			}
		}
		enc, skipped, err := encodeBook(got)
		if err != nil || skipped != 0 {
			t.Fatalf("encodeBook(accepted) = %d skipped, %v", skipped, err)
		}
		again, err := decodeBook(enc, self)
		if err != nil || !sameEntries(got, again) {
			t.Fatalf("re-decoding the accepted book %s: %d peers, %v; want %d peers", enc, len(again), err, len(got))
		}
	})
}

// FuzzBookNote: whatever address note accepts can be saved and read back (security review 1: a
// DNS name of invalid UTF-8 was recorded, then made every save fail).
func FuzzBookNote(f *testing.F) {
	for _, s := range []string{"/ip4/192.168.1.20/udp/7419/quic-v1", "/ip6/::1/tcp/7419", "/dns/example.org/tcp/1", "/dns4/a-b.c/udp/9/quic-v1"} {
		f.Add(ma.StringCast(s).Bytes())
	}
	f.Add([]byte{ma.P_DNS, 2, 0xff, 0xfe, ma.P_TCP, 0x1c, 0xfb})     // a DNS name of invalid UTF-8
	f.Add([]byte{ma.P_DNS, 3, 'a', '/', 'b', ma.P_TCP, 0x1c, 0xfb})  // a slash in the name
	f.Add([]byte{ma.P_DNS, 3, 'a', 0x00, 'b', ma.P_TCP, 0x1c, 0xfb}) // a NUL in the name
	self, p := bookID(f, 0), bookID(f, 1)
	f.Fuzz(func(t *testing.T, raw []byte) {
		a, err := ma.NewMultiaddrBytes(raw)
		if err != nil {
			return
		}
		b := newAddrBook(nil, self, bookNow, discard)
		b.note(p, []ma.Multiaddr{a}, bookNow, "p", always)
		b.noteLAN(p, []ma.Multiaddr{a}, bookNow, always)
		data, skipped, err := encodeBook(b.peers)
		if err != nil || skipped != 0 {
			t.Fatalf("note accepted %q, which does not encode: %d skipped, %v", a.String(), skipped, err)
		}
		got, err := decodeBook(data, self)
		if err != nil || !sameEntries(b.peers, got) {
			t.Fatalf("note accepted %q, which does not read back: %v", a.String(), err)
		}
	})
}

// TestBookSkipsCircuitAndUnspecified: §2.3, a relay path, an unspecified, multicast or otherwise
// non-dialable address, a DNS name that is not a hostname, or one naming another peer, is never
// stored.
func TestBookSkipsCircuitAndUnspecified(t *testing.T) {
	a, other := bookID(t, 1), bookID(t, 2)
	b := testBook(t)
	bad := addrs(
		"/ip4/1.2.3.4/udp/7419/quic-v1/p2p/"+other.String()+"/p2p-circuit",
		"/ip4/0.0.0.0/udp/7419/quic-v1",
		"/ip6/::/tcp/7419",
		"/ip4/224.0.0.251/udp/5353/quic-v1",
		"/ip4/10.0.0.1/tcp/7419/ws",
		"/ip4/10.0.0.1/udp/7419/webrtc-direct",
		"/ip4/10.0.0.1/tcp/0",
		"/ip4/10.0.0.1/tcp/7419/p2p/"+other.String(),
		"/dns/under_score.example/tcp/7419",
	)
	b.note(a, bad, bookNow, "p", always)
	if got := b.dialable(a); len(got) != 0 {
		t.Fatalf("note(only unusable addresses) stored %v", got)
	}
	b.note(a, append(bad, ma.StringCast("/ip4/10.0.0.1/tcp/7419/p2p/"+a.String()), ma.StringCast("/ip4/10.0.0.2/udp/7419/quic-v1")), bookNow, "p", always)
	if got, want := b.dialable(a), []string{"/ip4/10.0.0.1/tcp/7419", "/ip4/10.0.0.2/udp/7419/quic-v1"}; !slices.Equal(got, want) {
		t.Errorf("note(mixed) stored %v, want %v (own /p2p/ suffix removed)", got, want)
	}
}

// TestBookReachableFrom: security review 6, the addresses identify gave are filtered as libp2p's
// identify does for its peerstore.
func TestBookReachableFrom(t *testing.T) {
	all := addrs("/ip4/127.0.0.1/tcp/1", "/ip4/192.168.1.2/tcp/1", "/ip4/1.2.3.4/tcp/1")
	for _, c := range []struct {
		remote string
		want   []string
	}{
		{"/ip4/127.0.0.1/tcp/9", []string{"/ip4/127.0.0.1/tcp/1", "/ip4/192.168.1.2/tcp/1", "/ip4/1.2.3.4/tcp/1"}},
		{"/ip4/192.168.1.9/tcp/9", []string{"/ip4/192.168.1.2/tcp/1", "/ip4/1.2.3.4/tcp/1"}},
		{"/ip4/5.6.7.8/tcp/9", []string{"/ip4/1.2.3.4/tcp/1"}},
	} {
		if got := strs(reachableFrom(slices.Clone(all), ma.StringCast(c.remote))); !slices.Equal(got, c.want) {
			t.Errorf("reachableFrom(%s) = %v, want %v", c.remote, got, c.want)
		}
	}
}

// TestBookBounds: §4, 64 peers (the one seen longest ago is evicted), 8 addresses (the latest
// announced win), 128 bytes per address, 8 projects.
func TestBookBounds(t *testing.T) {
	b := testBook(t)
	for i := range bookMaxPeers + 1 {
		b.note(bookID(t, 100+i), addrs("/ip4/10.0.0.1/tcp/7419"), bookNow.Add(time.Duration(i)*time.Second), "p", always)
	}
	if got := len(b.snapshot()); got != bookMaxPeers {
		t.Errorf("book of %d peers after %d were noted, want %d", got, bookMaxPeers+1, bookMaxPeers)
	}
	if got := b.dialable(bookID(t, 100)); len(got) != 0 {
		t.Errorf("the peer seen longest ago is still there: %v", got)
	}
	if got := b.dialable(bookID(t, 100+bookMaxPeers)); len(got) != 1 {
		t.Errorf("the newest peer is missing: %v", got)
	}

	p := bookID(t, 1)
	var ten []string
	for i := range 10 {
		ten = append(ten, fmt.Sprintf("/ip4/10.0.1.%d/tcp/7419", i+1))
	}
	b.note(p, addrs(ten...), bookNow, "p", always)
	if got := b.dialable(p); !slices.Equal(got, ten[:bookMaxAddrs]) {
		t.Errorf("note(10 addresses) kept %v, want the first %d", got, bookMaxAddrs)
	}
	b.note(p, addrs("/ip4/10.0.2.1/tcp/7419", "/ip4/10.0.2.2/tcp/7419"), bookNow.Add(time.Second), "p", always)
	if got, want := b.dialable(p), append([]string{"/ip4/10.0.2.1/tcp/7419", "/ip4/10.0.2.2/tcp/7419"}, ten[:bookMaxAddrs-2]...); !slices.Equal(got, want) {
		t.Errorf("note(2 new addresses) kept %v, want %v", got, want)
	}
	for i := range 10 {
		b.note(p, addrs("/ip4/10.0.2.1/tcp/7419"), bookNow, fmt.Sprintf("s%d", i), always)
	}
	if got := b.peers[p].in; len(got) != bookMaxScopes || got[0] != "s9" {
		t.Errorf("projects after 11 = %v, want the latest %d", got, bookMaxScopes)
	}

	q := bookID(t, 2)
	long := "/dns/" + strings.Repeat("a", 60) + "." + strings.Repeat("a", 129-len("/dns/./tcp/7419")-60) + "/tcp/7419"
	edge := "/dns/" + strings.Repeat("a", 60) + "." + strings.Repeat("a", 128-len("/dns/./tcp/7419")-60) + "/tcp/7419"
	b.note(q, addrs(long, edge), bookNow, "p", always)
	if got := b.dialable(q); !slices.Equal(got, []string{edge}) {
		t.Errorf("note(129 B, 128 B) kept %v, want only the 128-byte address", got)
	}
}

// TestBookEvictsOld: §4, an entry seen exactly 90 days ago is kept, one seen longer ago is
// dropped, at load and at each flush.
func TestBookEvictsOld(t *testing.T) {
	a, b := bookID(t, 1), bookID(t, 2)
	addr := []string{"/ip4/10.0.0.1/tcp/7419"}
	store := &MemBook{}
	store.Set([]byte(strings.Replace(
		bookText(bookNow.Add(-bookMaxAge), map[peer.ID][]string{a: addr, b: addr}),
		fmt.Sprintf(`"id":%q,"seen":%d`, b.String(), bookNow.Add(-bookMaxAge).Unix()),
		fmt.Sprintf(`"id":%q,"seen":%d`, b.String(), bookNow.Add(-bookMaxAge-time.Second).Unix()), 1,
	)))
	book := newAddrBook(store, bookID(t, 0), bookNow, discard)
	if got := book.snapshot(); len(got) != 1 || got[0].ID != a.String() {
		t.Fatalf("loaded %v, want only the entry seen exactly 90 days ago", got)
	}
	book.flush(bookNow, false) // the dropped entry leaves the disk
	if got := string(store.Data()); !strings.Contains(got, a.String()) || strings.Contains(got, b.String()) {
		t.Errorf("saved %s after the load, want %s only", got, a)
	}
	book.flush(bookNow.Add(time.Second), false)
	if got := book.snapshot(); len(got) != 0 {
		t.Errorf("after a flush past 90 days the book holds %v, want nothing", got)
	}
	if got := string(store.Data()); strings.Contains(got, a.String()) {
		t.Errorf("saved %s after the flush past 90 days, want no entry", got)
	}
}

// TestBookPerDevice: security review 2, the book belongs to the device: every entry is loaded,
// whatever project is open; forgetting a project keeps the entry for the others; a corrupt book
// is quarantined with its reason category and the book starts empty.
func TestBookPerDevice(t *testing.T) {
	a, b := bookID(t, 1), bookID(t, 2)
	store := &MemBook{}
	store.Set([]byte(bookText(bookNow, map[peer.ID][]string{a: {"/ip4/10.0.0.1/tcp/7419"}, b: {"/ip4/10.0.0.2/tcp/7419"}})))
	book := newAddrBook(store, bookID(t, 0), bookNow, discard)
	if got := book.snapshot(); len(got) != 2 {
		t.Fatalf("loaded %v, want both entries", got)
	}
	book.note(a, addrs("/ip4/10.0.0.1/tcp/7419"), bookNow, "q", always)
	if !book.forget(a, "q") || len(book.dialable(a)) == 0 {
		t.Errorf("forget(project q) removed an entry still kept for project p")
	}
	if !book.forget(a, "p") || len(book.dialable(a)) != 0 {
		t.Errorf("forget(its last project) kept the entry")
	}
	if book.forget(b, "q") {
		t.Errorf("forget(a project the entry is not kept for) changed it")
	}

	store.Set([]byte("garbage"))
	book = newAddrBook(store, bookID(t, 0), bookNow, discard)
	if got := book.snapshot(); len(got) != 0 || string(store.bad) != "garbage" || store.reason != "syntax" || store.Data() != nil {
		t.Errorf("corrupt book: loaded %v, quarantined %q as %q, left %q; want empty, garbage, syntax, nothing", got, store.bad, store.reason, store.Data())
	}
}

// TestBookClockRules: security review 3, a clock set back (a seen time in the future) or far
// forward (every entry older than 90 days) never quarantines nor empties the book: it is kept as
// it is, unsaved, until a member is seen; then the usual rules apply. Second review 4: a
// revocation during that hold is saved at once, the rest of the book as it was loaded, and does
// not come back at the next load.
func TestBookClockRules(t *testing.T) {
	a, b := bookID(t, 1), bookID(t, 2)
	stored := bookText(bookNow, map[peer.ID][]string{a: {"/ip4/10.0.0.1/tcp/7419"}, b: {"/ip4/10.0.0.2/tcp/7419"}})
	for _, c := range []struct {
		name   string
		now    time.Time
		pruned bool // once a member is seen: the other entry is too old
	}{
		{"set back ten years", bookNow.Add(-10 * 365 * 24 * time.Hour), false},
		{"forward 100 days", bookNow.Add(100 * 24 * time.Hour), true},
	} {
		store := &MemBook{}
		store.Set([]byte(stored))
		book := newAddrBook(store, bookID(t, 0), c.now, discard)
		if got := book.snapshot(); len(got) != 2 || store.bad != nil {
			t.Errorf("%s: loaded %v, quarantined %q; want both entries", c.name, got, store.bad)
		}
		book.flush(c.now, true)
		if got := store.Saves(); got != 0 {
			t.Errorf("%s: %d saves before a member was seen, want none", c.name, got)
		}
		revoked := bookID(t, 3)
		book.note(revoked, nil, c.now, "p", always) // no address: not added, the hold stays
		store.Set([]byte(bookText(bookNow, map[peer.ID][]string{a: {"/ip4/10.0.0.1/tcp/7419"}, b: {"/ip4/10.0.0.2/tcp/7419"}, revoked: {"/ip4/10.0.0.3/tcp/7419"}})))
		book = newAddrBook(store, bookID(t, 0), c.now, discard)
		if !book.forget(revoked, "p") || !book.flush(c.now, false) || store.Saves() != 1 {
			t.Fatalf("%s: a revocation during the hold was not saved (%d saves)", c.name, store.Saves())
		}
		saved := string(store.Data())
		if strings.Contains(saved, revoked.String()) || !strings.Contains(saved, b.String()) || !strings.Contains(saved, fmt.Sprintf(`"seen":%d`, bookNow.Unix())) {
			t.Errorf("%s: saved during the hold %s; want the loaded book, unpruned, minus the revoked peer", c.name, saved)
		}
		if again := newAddrBook(store, bookID(t, 0), c.now, discard); len(again.dialable(revoked)) != 0 {
			t.Errorf("%s: the revoked peer came back at the next load", c.name)
		}
		book.note(a, addrs("/ip4/10.0.0.1/tcp/7419"), c.now, "p", always)
		book.flush(c.now, false)
		if got := store.Saves(); got != 2 {
			t.Errorf("%s: %d saves once a member was seen, want 2", c.name, got)
		}
		if got := strings.Contains(string(store.Data()), b.String()); got == c.pruned {
			t.Errorf("%s: other entry stored = %v, want %v", c.name, got, !c.pruned)
		}
	}
}

// TestBookNoteRechecksMembership: security review 8, note checks membership under the book's
// lock, so a note decided before a revocation and its forget cannot bring the entry back.
func TestBookNoteRechecksMembership(t *testing.T) {
	p := bookID(t, 1)
	b := testBook(t)
	admitted := true
	member := func() bool {
		if b.mu.TryLock() {
			b.mu.Unlock()
			t.Error("membership checked outside the book's lock")
		}
		return admitted
	}
	b.note(p, addrs("/ip4/10.0.0.1/tcp/7419"), bookNow, "p", member)
	admitted = false // revoked,
	b.forget(p, "p") // then forgotten,
	b.note(p, addrs("/ip4/10.0.0.1/tcp/7419"), bookNow, "p", member)
	b.noteLAN(p, addrs("/ip4/192.168.1.1/tcp/7419"), bookNow, member)
	if got := b.dialable(p); len(got) != 0 {
		t.Errorf("a note after the revocation stored %v", got)
	}
}

// TestBookSaveCoalesced: §3, at most one save per 10 s; seen-only changes are not saved before
// an hour; the book is saved on Close (the end of run).
func TestBookSaveCoalesced(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &MemBook{}
		p := bookID(t, 1)
		b := newAddrBook(store, bookID(t, 0), time.Now(), discard)
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		wg.Go(func() { b.run(ctx) })

		b.note(p, addrs("/ip4/10.0.0.1/tcp/7419"), time.Now(), "p", always)
		synctest.Wait()
		if got := store.Saves(); got != 1 {
			t.Fatalf("saves after the first change = %d, want 1 (at once)", got)
		}
		for i := range 30 { // a change every second for 30 s
			time.Sleep(time.Second)
			b.note(p, addrs(fmt.Sprintf("/ip4/10.0.1.%d/tcp/7419", i+1)), time.Now(), "p", always)
		}
		synctest.Wait()
		if got := store.Saves(); got < 3 || got > 1+30/10 {
			t.Errorf("saves after 30 changes in 30 s = %d, want one per 10 s (3 to 4)", got)
		}
		time.Sleep(bookSaveEvery)
		synctest.Wait()
		changed := store.Saves()
		if !strings.Contains(string(store.Data()), "/ip4/10.0.1.30/tcp/7419") {
			t.Errorf("the last change was not saved within 10 s: %s", store.Data())
		}

		time.Sleep(time.Minute)
		b.note(p, addrs("/ip4/10.0.1.30/tcp/7419"), time.Now(), "p", always) // seen again, same address
		time.Sleep(time.Minute)
		if got := store.Saves(); got != changed {
			t.Errorf("a seen-only change was saved within a minute (%d saves, want %d)", got, changed)
		}
		cancel()
		wg.Wait()
		if got := store.Saves(); got != changed+1 || !strings.Contains(string(store.Data()), fmt.Sprint(time.Now().Add(-time.Minute).Unix())) {
			t.Errorf("on Close: %d saves, book %s; want %d, with the new seen time", got, store.Data(), changed+1)
		}
	})
}

// TestBookLANCannotOutrankAuthenticated: an mDNS sighting, which anyone on the local network can
// forge, never evicts nor outranks an address announced over an authenticated connection: it
// takes at most bookMaxLAN places, only those left free, is dialed last and is never stored; a
// peer seen by mDNS alone never evicts an authenticated entry from a full book; and only private
// or link-local IP literals are taken from it (security review 5).
func TestBookLANCannotOutrankAuthenticated(t *testing.T) {
	p := bookID(t, 1)
	store := &MemBook{}
	b := newAddrBook(store, bookID(t, 0), bookNow, discard)
	auth := []string{"/ip4/10.0.0.1/udp/7419/quic-v1", "/ip4/10.0.0.1/tcp/7419", "/ip6/fd00::1/udp/7419/quic-v1"}
	b.note(p, addrs(auth...), bookNow, "p", always)
	for i := range 100 { // a spoofer floods the member's entry
		b.noteLAN(p, addrs(fmt.Sprintf("/ip4/192.168.9.%d/udp/7419/quic-v1", i+1)), bookNow.Add(time.Duration(i)*time.Second), always)
	}
	got := b.dialable(p)
	if len(got) != len(auth)+bookMaxLAN || !slices.Equal(got[:len(auth)], auth) {
		t.Errorf("after the flood the entry dials %v, want %v first, then %d mDNS addresses", got, auth, bookMaxLAN)
	}
	b.flush(bookNow, false)
	if data := string(store.Data()); strings.Contains(data, "192.168.9.") || !strings.Contains(data, auth[0]) {
		t.Errorf("stored book %s: want the authenticated addresses only", data)
	}
	for _, s := range []string{"/ip4/1.2.3.4/udp/7419/quic-v1", "/ip4/127.0.0.1/tcp/7419", "/ip6/::1/tcp/7419", "/dns/printer.local/tcp/7419", "/ip6/2001:db8::1/tcp/7419"} {
		if b.noteLAN(bookID(t, 3), addrs(s), bookNow, always) {
			t.Errorf("noteLAN(%s) was taken, want only private or link-local IP literals", s)
		}
	}
	if !b.noteLAN(bookID(t, 3), addrs("/ip4/169.254.1.1/tcp/7419"), bookNow, always) {
		t.Error("noteLAN(link-local) was refused")
	}

	var eight []string
	for i := range bookMaxAddrs {
		eight = append(eight, fmt.Sprintf("/ip4/10.0.1.%d/tcp/7419", i+1))
	}
	b.note(p, addrs(eight...), bookNow, "p", always)
	b.noteLAN(p, addrs("/ip4/192.168.9.200/udp/7419/quic-v1"), bookNow, always)
	// The two mDNS places: the new sighting, then the latest of the flood.
	if got, want := b.dialable(p), slices.Concat(eight, []string{"/ip4/192.168.9.200/udp/7419/quic-v1", "/ip4/192.168.9.100/udp/7419/quic-v1"}); !slices.Equal(got, want) {
		t.Errorf("a full authenticated entry dials %v after a sighting, want %v: the 8, then the mDNS places of their own", got, want)
	}

	full := testBook(t)
	for i := range bookMaxPeers {
		full.note(bookID(t, 100+i), addrs("/ip4/10.0.0.1/tcp/7419"), bookNow, "p", always)
	}
	full.noteLAN(bookID(t, 2), addrs("/ip4/192.168.9.1/tcp/7419"), bookNow.Add(time.Hour), always)
	for i := range bookMaxPeers {
		if len(full.dialable(bookID(t, 100+i))) == 0 {
			t.Fatalf("a peer seen by mDNS alone evicted the authenticated entry of peer %d", i)
		}
	}
}

// TestBookLANWakeBucket: security review 4, mDNS sightings wake the dial loops through a
// node-wide token bucket: wakeBurst at once, then one per wakeEvery.
func TestBookLANWakeBucket(t *testing.T) {
	n := &Node{}
	taken := 0
	for range 10 {
		if n.wakeTokenLocked(bookNow) {
			taken++
		}
	}
	if taken != wakeBurst {
		t.Errorf("%d wakes at once, want %d", taken, wakeBurst)
	}
	if !n.wakeTokenLocked(bookNow.Add(wakeEvery)) || n.wakeTokenLocked(bookNow.Add(wakeEvery)) {
		t.Errorf("after %v: want exactly one more wake", wakeEvery)
	}
}

// fakeConn is a connection that only knows its remote address.
type fakeConn struct {
	network.Conn
	remote ma.Multiaddr
}

func (c fakeConn) RemoteMultiaddr() ma.Multiaddr { return c.remote }

// TestBookPrefersSignedRecord: security review 6, the addresses identify gave are those of the
// peer's signed record when it sent one, not its raw listen addresses; a record of another peer is
// ignored; the result is filtered by reachableFrom.
func TestBookPrefersSignedRecord(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	seal := func(of peer.ID, as ...string) *record.Envelope {
		env, err := record.Seal(&peer.PeerRecord{PeerID: of, Addrs: addrs(as...), Seq: 1}, priv)
		if err != nil {
			t.Fatal(err)
		}
		return env
	}
	e := event.EvtPeerIdentificationCompleted{
		Peer:        id,
		Conn:        fakeConn{remote: ma.StringCast("/ip4/192.168.1.9/udp/7419/quic-v1")},
		ListenAddrs: addrs("/ip4/192.168.1.2/tcp/7419", "/ip4/127.0.0.1/tcp/7419"),
	}
	if got, want := strs(announced(e)), []string{"/ip4/192.168.1.2/tcp/7419"}; !slices.Equal(got, want) {
		t.Errorf("announced(no record) = %v, want %v (listen addresses, no loopback)", got, want)
	}
	e.SignedPeerRecord = seal(id, "/ip4/192.168.1.3/udp/7419/quic-v1", "/ip4/127.0.0.1/udp/7419/quic-v1")
	if got, want := strs(announced(e)), []string{"/ip4/192.168.1.3/udp/7419/quic-v1"}; !slices.Equal(got, want) {
		t.Errorf("announced(signed record) = %v, want %v (the record's, no loopback)", got, want)
	}
	e.SignedPeerRecord = seal(bookID(t, 9), "/ip4/192.168.1.4/tcp/7419")
	if got, want := strs(announced(e)), []string{"/ip4/192.168.1.2/tcp/7419"}; !slices.Equal(got, want) {
		t.Errorf("announced(record of another peer) = %v, want %v", got, want)
	}
}

// TestBookSaveRetried: second review 3, a failed save is retried on its own, after bookSaveEvery
// then twice as long, with no further change nor shutdown.
func TestBookSaveRetried(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &MemBook{}
		store.FailSaves(2)
		b := newAddrBook(store, bookID(t, 0), time.Now(), discard)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go b.run(ctx)
		b.note(bookID(t, 1), addrs("/ip4/10.0.0.1/tcp/7419"), time.Now(), "p", always)
		time.Sleep(bookSaveEvery + time.Second) // the first attempt and its first retry fail
		if got := store.Saves(); got != 0 {
			t.Fatalf("%d saves while the store fails, want none", got)
		}
		time.Sleep(2 * bookSaveEvery) // the second retry succeeds
		if got := store.Saves(); got != 1 || !strings.Contains(string(store.Data()), "/ip4/10.0.0.1/tcp/7419") {
			t.Errorf("after the store recovered: %d saves, book %s; want the change saved", got, store.Data())
		}
		cancel()
		synctest.Wait()
	})
}
