package bridge_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
	"github.com/oioio-space/inkwell/internal/transport"
)

// rejectsEveryPrefixAndExtension checks the strictness every decoder here
// promises: of a valid encoding, no strict prefix and no extension by a byte is
// accepted. except lists the prefix lengths that are legitimately valid too.
func rejectsEveryPrefixAndExtension(t *testing.T, name string, valid []byte, decode func([]byte) error, except ...int) {
	t.Helper()
	if err := decode(valid); err != nil {
		t.Errorf("%s: decoding the valid encoding = %v", name, err)
		return
	}
	for n := range len(valid) {
		if slicesContain(except, n) {
			continue
		}
		if err := decode(valid[:n]); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("%s: decoding its first %d of %d bytes = %v, want ErrMalformed", name, n, len(valid), err)
		}
	}
	if err := decode(append(bytes.Clone(valid), 0)); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("%s: decoding it plus a trailing byte = %v, want ErrMalformed", name, err)
	}
}

func slicesContain(s []int, n int) bool {
	for _, x := range s {
		if x == n {
			return true
		}
	}
	return false
}

func sampleFiles() []project.File {
	return []project.File{
		{Path: "main.typ", Kind: project.KindText, Size: 12, Main: true},
		{Path: "sub/é.png", Kind: project.KindData, Size: math.MaxInt64},
		{Path: "z", Kind: 0, Size: 0},
	}
}

func TestFileListStrictness(t *testing.T) {
	enc, err := bridge.EncodeFileList(sampleFiles())
	if err != nil {
		t.Fatal(err)
	}
	rejectsEveryPrefixAndExtension(t, "file list", enc, func(b []byte) error { _, err := bridge.DecodeFileList(b); return err })
	back, err := bridge.DecodeFileList(enc)
	if err != nil || !cmp.Equal(back, sampleFiles()) {
		t.Errorf("round trip = %+v, %v", back, err)
	}

	empty, err := bridge.EncodeFileList(nil)
	if err != nil || len(empty) != 5 {
		t.Fatalf("EncodeFileList(nil) = %x, %v; want the 5-byte header", empty, err)
	}
	if got, err := bridge.DecodeFileList(empty); err != nil || len(got) != 0 {
		t.Errorf("DecodeFileList(empty list) = %v, %v; want an empty list", got, err)
	}

	// Entries, by hand: the limits of each field.
	entry := func(kind, flags byte, size uint64, path string) []byte {
		b := []byte{1, 0, 0, 0, 1, kind, flags}
		b = binary.BigEndian.AppendUint64(b, size)
		b = binary.BigEndian.AppendUint16(b, uint16(len(path)))
		return append(b, path...)
	}
	for _, tc := range []struct {
		name string
		wire []byte
		ok   bool
	}{
		{"plain", entry(0, 0, 5, "a"), true},
		{"largest kind", entry(byte(project.KindData), 0, 5, "a"), true},
		{"kind past the last", entry(byte(project.KindData)+1, 0, 5, "a"), false},
		{"main flag", entry(0, 1, 5, "a"), true},
		{"unknown flag", entry(0, 2, 5, "a"), false},
		{"largest size", entry(0, 0, math.MaxInt64, "a"), true},
		{"size over int64", entry(0, 0, math.MaxInt64+1, "a"), false},
		{"empty path", entry(0, 0, 5, ""), false},
		{"path not utf-8", entry(0, 0, 5, "\xff"), false},
	} {
		if _, err := bridge.DecodeFileList(tc.wire); tc.ok != (err == nil) {
			t.Errorf("DecodeFileList(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
		}
	}
	if _, err := bridge.DecodeFileList([]byte{2, 0, 0, 0, 0}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeFileList(version 2) = %v, want ErrMalformed", err)
	}
	// A count that promises more than the bytes carry.
	if _, err := bridge.DecodeFileList([]byte{1, 0xff, 0xff, 0xff, 0xff}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeFileList(count 2^32-1, no entries) = %v, want ErrMalformed", err)
	}
}

func TestEncodeFileListLimits(t *testing.T) {
	long := strings.Repeat("p", math.MaxUint16)
	for _, tc := range []struct {
		name string
		f    project.File
		ok   bool
	}{
		{"zero size", project.File{Path: "a", Size: 0}, true},
		{"negative size", project.File{Path: "a", Size: -1}, false},
		{"largest kind", project.File{Path: "a", Kind: project.KindData}, true},
		{"unknown kind", project.File{Path: "a", Kind: project.KindData + 1}, false},
		{"longest path", project.File{Path: long}, true},
		{"path one byte too long", project.File{Path: long + "p"}, false},
		{"empty path", project.File{}, false},
	} {
		_, err := bridge.EncodeFileList([]project.File{tc.f})
		if tc.ok != (err == nil) || (err != nil && !errors.Is(err, bridge.ErrMalformed)) {
			t.Errorf("EncodeFileList(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
		}
	}

	// The whole list fits the frame limit exactly or not at all.
	const limit = 64<<20 - 5 // maxStoreWire - respHeaderLen
	perEntry := 12 + math.MaxUint16
	n := (limit - 5) / perEntry
	rest := limit - 5 - n*perEntry // what the last, shorter entry adds
	files := make([]project.File, n, n+1)
	for i := range files {
		files[i] = project.File{Path: long}
	}
	files = append(files, project.File{Path: strings.Repeat("q", rest-12)})
	enc, err := bridge.EncodeFileList(files)
	if err != nil || len(enc) != limit {
		t.Fatalf("EncodeFileList(exactly the limit) = %d bytes, %v; want %d, nil", len(enc), err, limit)
	}
	files[len(files)-1].Path += "q"
	if _, err := bridge.EncodeFileList(files); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeFileList(one byte over the limit) = %v, want ErrMalformed", err)
	}
}

func TestFileInfoStrictness(t *testing.T) {
	info := bridge.FileInfo{Encoding: "windows-1252", EOL: project.CRLF, BOM: true, Mixed: true}
	enc := bridge.EncodeFileInfo(info)
	back, err := bridge.DecodeFileInfo(enc)
	if err != nil || back != info {
		t.Fatalf("round trip = %+v, %v; want %+v", back, err, info)
	}
	rejectsEveryPrefixAndExtension(t, "file info", enc, func(b []byte) error { _, err := bridge.DecodeFileInfo(b); return err })
	// The longest-lived corner: no encoding name at all, and each flag alone.
	for _, i := range []bridge.FileInfo{{}, {BOM: true}, {Mixed: true}, {EOL: project.CR}, {EOL: project.LF}} {
		got, err := bridge.DecodeFileInfo(bridge.EncodeFileInfo(i))
		if err != nil || got != i {
			t.Errorf("round trip of %+v = %+v, %v", i, got, err)
		}
	}
	for name, wire := range map[string][]byte{
		"unknown flag":  {1, 4, 0, 0},
		"mixed EOL":     {1, 0, byte(project.Mixed), 0},
		"eol past CR":   {1, 0, byte(project.CR) + 1, 0},
		"wrong version": {2, 0, 0, 0},
		"length lies":   {1, 0, 0, 3, 'a'},
	} {
		if _, err := bridge.DecodeFileInfo(wire); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("DecodeFileInfo(%s) = %v, want ErrMalformed", name, err)
		}
	}
}

func sampleTree() []project.TreeEntry {
	return []project.TreeEntry{
		{Path: "dir", Type: project.NodeDir},
		{Path: "dir/a.typ", Type: project.NodeText, URL: "automerge:abc", Main: true},
		{Path: "dir/b.png", Type: project.NodeBlob, Size: 7, Hash: [32]byte{1, 2, 3}},
	}
}

func TestTreeStrictness(t *testing.T) {
	enc, err := bridge.EncodeTree(sampleTree())
	if err != nil {
		t.Fatal(err)
	}
	rejectsEveryPrefixAndExtension(t, "tree", enc, func(b []byte) error { _, err := bridge.DecodeTree(b); return err })
	back, err := bridge.DecodeTree(enc)
	if err != nil || !cmp.Equal(back, sampleTree()) {
		t.Errorf("round trip = %+v, %v", back, err)
	}
	empty, _ := bridge.EncodeTree(nil)
	if got, err := bridge.DecodeTree(empty); err != nil || len(got) != 0 {
		t.Errorf("DecodeTree(empty) = %v, %v; want an empty tree", got, err)
	}
}

func TestTreeEntryLimits(t *testing.T) {
	url := strings.Repeat("u", 255)
	path := strings.Repeat("p", project.MaxPathLen)
	for _, tc := range []struct {
		name string
		e    project.TreeEntry
		ok   bool
	}{
		{"text with the longest URL", project.TreeEntry{Path: "a", Type: project.NodeText, URL: url}, true},
		{"text with a URL one byte too long", project.TreeEntry{Path: "a", Type: project.NodeText, URL: url + "u"}, false},
		{"longest path", project.TreeEntry{Path: path, Type: project.NodeDir}, true},
		{"path one byte too long", project.TreeEntry{Path: path + "p", Type: project.NodeDir}, false},
		{"empty path", project.TreeEntry{Type: project.NodeDir}, false},
		{"zero size", project.TreeEntry{Path: "a", Type: project.NodeBlob}, true},
		{"negative size", project.TreeEntry{Path: "a", Type: project.NodeBlob, Size: -1}, false},
		{"largest size", project.TreeEntry{Path: "a", Type: project.NodeBlob, Size: math.MaxInt64}, true},
		{"type past blob", project.TreeEntry{Path: "a", Type: project.NodeBlob + 1}, false},
		{"URL on a blob", project.TreeEntry{Path: "a", Type: project.NodeBlob, URL: "x"}, false},
		{"URL on a folder", project.TreeEntry{Path: "a", Type: project.NodeDir, URL: "x"}, false},
		{"hash on a text", project.TreeEntry{Path: "a", Type: project.NodeText, URL: "x", Hash: [32]byte{1}}, false},
		{"hash on a folder", project.TreeEntry{Path: "a", Type: project.NodeDir, Hash: [32]byte{1}}, false},
		{"path not utf-8", project.TreeEntry{Path: "\xff", Type: project.NodeDir}, false},
	} {
		enc, err := bridge.EncodeTree([]project.TreeEntry{tc.e})
		if tc.ok != (err == nil) || (err != nil && !errors.Is(err, bridge.ErrMalformed)) {
			t.Errorf("EncodeTree(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
			continue
		}
		if tc.ok {
			got, err := bridge.DecodeTree(enc)
			if err != nil || len(got) != 1 || !cmp.Equal(got[0], tc.e) {
				t.Errorf("round trip of %s = %+v, %v", tc.name, got, err)
			}
		}
	}

	// The wire, by hand: references that do not fit their type.
	wire := func(typ, flags byte, path string, ref []byte) []byte {
		b := []byte{1, 0, 0, 0, 1, typ, flags}
		b = binary.BigEndian.AppendUint64(b, 3)
		b = binary.BigEndian.AppendUint16(b, uint16(len(path)))
		b = append(b, path...)
		b = append(b, byte(len(ref)))
		return append(b, ref...)
	}
	h := bytes.Repeat([]byte{9}, 32)
	for _, tc := range []struct {
		name string
		wire []byte
		ok   bool
	}{
		{"folder", wire(0, 0, "a", nil), true},
		{"folder with a reference", wire(0, 0, "a", []byte("x")), false},
		{"text", wire(1, 0, "a", []byte("automerge:x")), true},
		{"text without a document", wire(1, 0, "a", nil), false},
		{"text with a non-UTF-8 document", wire(1, 0, "a", []byte{0xff}), false},
		{"blob", wire(2, 0, "a", h), true},
		{"blob with a short hash", wire(2, 0, "a", h[:31]), false},
		{"blob with a long hash", wire(2, 0, "a", append(bytes.Clone(h), 0)), false},
		{"unknown flag", wire(0, 2, "a", nil), false},
		{"unknown type", wire(3, 0, "a", nil), false},
		{"path of the maximum length", wire(0, 0, path, nil), true},
		{"path one byte over the maximum", wire(0, 0, path+"p", nil), false},
		{"empty path", wire(0, 0, "", nil), false},
	} {
		if _, err := bridge.DecodeTree(tc.wire); tc.ok != (err == nil) {
			t.Errorf("DecodeTree(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
		}
	}
	// A size that does not fit an int64.
	big := wire(0, 0, "a", nil)
	binary.BigEndian.PutUint64(big[7:], math.MaxInt64+1)
	if _, err := bridge.DecodeTree(big); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeTree(size 2^63) = %v, want ErrMalformed", err)
	}
}

func TestTreeCountAndSizeLimits(t *testing.T) {
	dirs := func(n int) []project.TreeEntry {
		es := make([]project.TreeEntry, n)
		for i := range es {
			es[i] = project.TreeEntry{Path: "d", Type: project.NodeDir}
		}
		return es
	}
	for _, tc := range []struct {
		n  int
		ok bool
	}{{project.MaxNodes, true}, {project.MaxNodes + 1, false}} {
		enc, err := bridge.EncodeTree(dirs(tc.n))
		if err != nil {
			t.Fatal(err)
		}
		if got, err := bridge.DecodeTree(enc); tc.ok != (err == nil) || (tc.ok && len(got) != tc.n) {
			t.Errorf("DecodeTree(%d entries) = %d entries, %v; want accepted=%t", tc.n, len(got), err, tc.ok)
		}
	}

	// The tree frame holds exactly store.MaxValue bytes.
	path := strings.Repeat("p", project.MaxPathLen)
	perEntry := 12 + project.MaxPathLen + 1
	n := (store.MaxValue - 5) / perEntry
	rest := store.MaxValue - 5 - n*perEntry
	entries := make([]project.TreeEntry, n, n+1)
	for i := range entries {
		entries[i] = project.TreeEntry{Path: path, Type: project.NodeDir}
	}
	entries = append(entries, project.TreeEntry{Path: strings.Repeat("q", rest-13), Type: project.NodeDir})
	if enc, err := bridge.EncodeTree(entries); err != nil || len(enc) != store.MaxValue {
		t.Fatalf("EncodeTree(exactly MaxValue) = %d bytes, %v; want %d", len(enc), err, store.MaxValue)
	}
	entries[len(entries)-1].Path += "q"
	if _, err := bridge.EncodeTree(entries); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeTree(one byte over MaxValue) = %v, want ErrMalformed", err)
	}
}

func TestReferencedAndSmallPayloads(t *testing.T) {
	hs := [][bridge.HashSize]byte{{1}, {2}, {3}}
	enc, err := bridge.EncodeReferenced(hs)
	if err != nil {
		t.Fatal(err)
	}
	rejectsEveryPrefixAndExtension(t, "referenced", enc, func(b []byte) error { _, err := bridge.DecodeReferenced(b); return err })
	if back, err := bridge.DecodeReferenced(enc); err != nil || !cmp.Equal(back, hs) {
		t.Errorf("round trip = %v, %v", back, err)
	}
	if got, err := bridge.DecodeReferenced([]byte{1, 0, 0, 0, 0}); err != nil || len(got) != 0 {
		t.Errorf("DecodeReferenced(empty list) = %v, %v; want an empty list", got, err)
	}
	// A count that disagrees with the bytes, whichever way.
	for _, count := range []uint32{0, 1, 3, math.MaxUint32} {
		b := binary.BigEndian.AppendUint32([]byte{1}, count)
		if _, err := bridge.DecodeReferenced(append(b, make([]byte, 2*bridge.HashSize)...)); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("DecodeReferenced(count %d with 2 hashes) = %v, want ErrMalformed", count, err)
		}
	}
	limit := (store.MaxValue - 5) / bridge.HashSize
	if _, err := bridge.EncodeReferenced(make([][bridge.HashSize]byte, limit)); err != nil {
		t.Errorf("EncodeReferenced(%d hashes, the most that fit) = %v", limit, err)
	}
	if _, err := bridge.EncodeReferenced(make([][bridge.HashSize]byte, limit+1)); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeReferenced(%d hashes) = %v, want ErrMalformed", limit+1, err)
	}

	// Blob info and tree result: exact lengths.
	info := bridge.EncodeBlobInfo([32]byte{5}, 77)
	rejectsEveryPrefixAndExtension(t, "blob info", info, func(b []byte) error { _, _, err := bridge.DecodeBlobInfo(b); return err })
	if h, size, err := bridge.DecodeBlobInfo(info); err != nil || h != ([32]byte{5}) || size != 77 {
		t.Errorf("DecodeBlobInfo = %x, %d, %v", h, size, err)
	}
	if size := binary.BigEndian.Uint64(bridge.EncodeBlobInfo([32]byte{}, -5)[33:]); size != 0 {
		t.Errorf("EncodeBlobInfo(size -5) wrote %d, want it clamped to 0", size)
	}
	huge := bridge.EncodeBlobInfo([32]byte{}, 1)
	binary.BigEndian.PutUint64(huge[33:], math.MaxInt64+1)
	if _, _, err := bridge.DecodeBlobInfo(huge); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeBlobInfo(size 2^63) = %v, want ErrMalformed", err)
	}
	res := bridge.EncodeTreeResult(41)
	rejectsEveryPrefixAndExtension(t, "tree result", res, func(b []byte) error { _, err := bridge.DecodeTreeResult(b); return err })
	if n, err := bridge.DecodeTreeResult(res); err != nil || n != 41 {
		t.Errorf("DecodeTreeResult = %d, %v", n, err)
	}
	if n, _ := bridge.DecodeTreeResult(bridge.EncodeTreeResult(math.MaxUint32 + 5)); n != math.MaxUint32 {
		t.Errorf("EncodeTreeResult(over uint32) decodes to %d, want it clamped to %d", n, uint32(math.MaxUint32))
	}
	if n, _ := bridge.DecodeTreeResult(bridge.EncodeTreeResult(-3)); n < 0 {
		t.Errorf("EncodeTreeResult(-3) decodes to %d, want a non-negative count", n)
	}
}

func TestHeadsLimitsAreExact(t *testing.T) {
	doc := strings.Repeat("d", bridge.MaxDoc)
	heads := make([][bridge.HashSize]byte, bridge.MaxHeads)
	for i := range heads {
		heads[i][0] = byte(i)
	}
	largest := bridge.Heads{Doc: doc, Heads: heads}
	enc, err := bridge.EncodeHeads(largest)
	if err != nil {
		t.Fatalf("EncodeHeads(largest) = %v", err)
	}
	if back, err := bridge.DecodeHeads(enc); err != nil || back.Doc != doc || len(back.Heads) != bridge.MaxHeads {
		t.Errorf("round trip of the largest Heads = %d-byte doc, %d heads, %v", len(back.Doc), len(back.Heads), err)
	}
	for name, h := range map[string]bridge.Heads{
		"doc one byte too long": {Doc: doc + "d"},
		"one head too many":     {Doc: "d", Heads: make([][bridge.HashSize]byte, bridge.MaxHeads+1)},
		"empty doc":             {},
		"doc not utf-8":         {Doc: "\xff"},
	} {
		if _, err := bridge.EncodeHeads(h); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("EncodeHeads(%s) = %v, want ErrMalformed", name, err)
		}
	}

	// On the wire.
	wire := func(doc string, nHeads int) []byte {
		b := binary.BigEndian.AppendUint16(nil, uint16(len(doc)))
		b = append(b, doc...)
		return append(b, make([]byte, nHeads*bridge.HashSize)...)
	}
	for _, tc := range []struct {
		name string
		wire []byte
		ok   bool
	}{
		{"no heads", wire("d", 0), true},
		{"the most heads", wire("d", bridge.MaxHeads), true},
		{"one head too many", wire("d", bridge.MaxHeads+1), false},
		{"longest doc", wire(doc, 1), true},
		{"doc one byte too long", wire(doc+"d", 1), false},
		{"empty doc", wire("", 1), false},
		{"doc not utf-8", wire("\xff", 1), false},
		{"half a head", append(wire("d", 1), 0), false},
		{"head cut short", wire("d", 1)[:len(wire("d", 1))-1], false},
		{"doc longer than the frame", []byte{0, 9, 'a'}, false},
		{"one byte", []byte{0}, false},
		{"empty", nil, false},
	} {
		if _, err := bridge.DecodeHeads(tc.wire); tc.ok != (err == nil) || (err != nil && !errors.Is(err, bridge.ErrMalformed)) {
			t.Errorf("DecodeHeads(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
		}
	}
}

func TestRolesLimitsAreExact(t *testing.T) {
	member := func(i int) bridge.MemberRole {
		return bridge.MemberRole{Peer: transport.PeerID(strings.Repeat("p", 1+i%64)), Role: uint8(1 + i%3), Suspended: i%2 == 0}
	}
	for _, n := range []int{0, 1, 2, bridge.MaxMembers} {
		rs := make([]bridge.MemberRole, n)
		for i := range rs {
			rs[i] = member(i)
		}
		enc, err := bridge.EncodeRoles(rs)
		if err != nil {
			t.Fatalf("EncodeRoles(%d) = %v", n, err)
		}
		back, err := bridge.DecodeRoles(enc)
		if err != nil || len(back) != n || (n > 0 && !cmp.Equal(back, rs)) {
			t.Errorf("round trip of %d members = %d, %v", n, len(back), err)
		}
		if n == 2 {
			rejectsEveryPrefixAndExtension(t, "roles", enc, func(b []byte) error { _, err := bridge.DecodeRoles(b); return err })
		}
	}
	if _, err := bridge.EncodeRoles(make([]bridge.MemberRole, bridge.MaxMembers+1)); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeRoles(MaxMembers+1) = %v, want ErrMalformed", err)
	}
	over := binary.BigEndian.AppendUint16(nil, bridge.MaxMembers+1)
	if _, err := bridge.DecodeRoles(over); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeRoles(count MaxMembers+1) = %v, want ErrMalformed", err)
	}
	entry := func(peer string, role, flags byte) []byte {
		b := binary.BigEndian.AppendUint16(nil, 1)
		b = append(b, byte(len(peer)))
		b = append(b, peer...)
		return append(b, role, flags)
	}
	for _, tc := range []struct {
		name string
		wire []byte
		ok   bool
	}{
		{"reader", entry("p", 1, 0), true},
		{"admin suspended", entry("p", 3, 1), true},
		{"role 0", entry("p", 0, 0), false},
		{"role 4", entry("p", 4, 0), false},
		{"unknown flag", entry("p", 1, 2), false},
		{"empty peer", entry("", 1, 0), false},
		{"longest peer", entry(strings.Repeat("p", 64), 1, 0), true},
		{"peer one byte too long", entry(strings.Repeat("p", 65), 1, 0), false},
	} {
		if _, err := bridge.DecodeRoles(tc.wire); tc.ok != (err == nil) {
			t.Errorf("DecodeRoles(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
		}
	}
	for name, r := range map[string]bridge.MemberRole{
		"role 0": {Peer: "p", Role: 0}, "role 4": {Peer: "p", Role: 4}, "empty peer": {Role: 1},
		"peer too long": {Peer: transport.PeerID(strings.Repeat("p", 65)), Role: 1},
	} {
		if _, err := bridge.EncodeRoles([]bridge.MemberRole{r}); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("EncodeRoles(%s) = %v, want ErrMalformed", name, err)
		}
	}
}

func TestEnvelopeLimitsAreExact(t *testing.T) {
	peer64 := transport.PeerID(strings.Repeat("p", 64))
	for _, tc := range []struct {
		name string
		e    bridge.Envelope
		ok   bool
	}{
		{"peer up", bridge.Envelope{Kind: bridge.PeerUp, Peer: "a"}, true},
		{"longest peer", bridge.Envelope{Kind: bridge.PeerUp, Peer: peer64}, true},
		{"peer one byte too long", bridge.Envelope{Kind: bridge.PeerUp, Peer: peer64 + "p"}, false},
		{"empty peer", bridge.Envelope{Kind: bridge.PeerUp}, false},
		{"message of no bytes", bridge.Envelope{Kind: bridge.Message, Peer: "a"}, true},
		{"message of MaxFrame", bridge.Envelope{Kind: bridge.Message, Peer: peer64, Payload: make([]byte, transport.MaxFrame)}, true},
		{"message of MaxFrame+1", bridge.Envelope{Kind: bridge.Message, Peer: "a", Payload: make([]byte, transport.MaxFrame+1)}, false},
		{"payload on a peer up", bridge.Envelope{Kind: bridge.PeerUp, Peer: "a", Payload: []byte{1}}, false},
		{"payload on a peer down", bridge.Envelope{Kind: bridge.PeerDown, Peer: "a", Payload: []byte{1}}, false},
		{"suspended 0", bridge.Envelope{Kind: bridge.Suspended, Peer: "a", Payload: []byte{0}}, true},
		{"suspended 1", bridge.Envelope{Kind: bridge.Suspended, Peer: "a", Payload: []byte{1}}, true},
		{"suspended 2", bridge.Envelope{Kind: bridge.Suspended, Peer: "a", Payload: []byte{2}}, false},
		{"suspended without a payload", bridge.Envelope{Kind: bridge.Suspended, Peer: "a"}, false},
		{"suspended with two bytes", bridge.Envelope{Kind: bridge.Suspended, Peer: "a", Payload: []byte{0, 0}}, false},
		{"kind 0", bridge.Envelope{Kind: 0, Peer: "a"}, false},
		{"kind past the last", bridge.Envelope{Kind: bridge.Suspended + 1, Peer: "a"}, false},
	} {
		enc, err := bridge.Encode(tc.e)
		if tc.ok != (err == nil) || (err != nil && !errors.Is(err, bridge.ErrMalformed)) {
			t.Errorf("Encode(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
			continue
		}
		if !tc.ok {
			// What Encode refuses, Decode refuses too, when it can even be written down.
			continue
		}
		back, err := bridge.Decode(enc)
		if err != nil || back.Kind != tc.e.Kind || back.Peer != tc.e.Peer || !bytes.Equal(back.Payload, tc.e.Payload) {
			t.Errorf("round trip of %s = %v bytes, %v", tc.name, len(back.Payload), err)
		}
	}
	// Decode: what is cut short is refused.
	enc, _ := bridge.Encode(bridge.Envelope{Kind: bridge.Message, Peer: "peer", Payload: []byte("hi")})
	for n := range len(enc) - len("hi") { // up to and including the peer, a message without bytes is still valid
		if n >= 3+len("peer") {
			break
		}
		if _, err := bridge.Decode(enc[:n]); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("Decode(first %d of %d bytes) = %v, want ErrMalformed", n, len(enc), err)
		}
	}
	for _, header := range [][]byte{{2, 1, 1, 'a'}, {0, 1, 1, 'a'}} {
		if _, err := bridge.Decode(header); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("Decode(version %d) = %v, want ErrMalformed", header[0], err)
		}
	}
	// A peer length that reaches past the frame, exactly one byte.
	if _, err := bridge.Decode([]byte{1, byte(bridge.PeerUp), 2, 'a'}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("Decode(peer length 2, one byte) = %v, want ErrMalformed", err)
	}
	if e, err := bridge.Decode([]byte{1, byte(bridge.PeerUp), 1, 'a'}); err != nil || e.Peer != "a" {
		t.Errorf("Decode(smallest valid frame) = %+v, %v", e, err)
	}
	// The frame limit: header + peer + MaxFrame is accepted, a byte more is not.
	big := append([]byte{1, byte(bridge.Message), 64}, append([]byte(peer64), make([]byte, transport.MaxFrame)...)...)
	if _, err := bridge.Decode(big); err != nil {
		t.Errorf("Decode(a frame of the largest size) = %v", err)
	}
	if _, err := bridge.Decode(append(big, make([]byte, 64<<20)...)); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("Decode(a frame over the wire limit) = %v, want ErrMalformed", err)
	}
}

func TestStoreCodecStrictness(t *testing.T) {
	// Responses: any length from the header up, every status to the last.
	for status := bridge.StatusOK; status <= bridge.StatusTooLarge; status++ {
		enc, err := bridge.EncodeStoreResponse(bridge.StoreResponse{ID: 7, Status: status, Payload: []byte("p")})
		if err != nil {
			t.Fatal(err)
		}
		if got, err := bridge.DecodeStoreResponse(enc); err != nil || got.Status != status || got.ID != 7 || string(got.Payload) != "p" {
			t.Errorf("round trip of status %d = %+v, %v", status, got, err)
		}
	}
	if _, err := bridge.EncodeStoreResponse(bridge.StoreResponse{Status: bridge.StatusTooLarge + 1}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeStoreResponse(status past the last) = %v, want ErrMalformed", err)
	}
	if _, err := bridge.DecodeStoreResponse([]byte{0, 0, 0, 1, byte(bridge.StatusTooLarge) + 1}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeStoreResponse(status past the last) = %v, want ErrMalformed", err)
	}
	if got, err := bridge.DecodeStoreResponse([]byte{0, 0, 0, 1, 0}); err != nil || len(got.Payload) != 0 {
		t.Errorf("DecodeStoreResponse(header only) = %+v, %v; want an empty payload", got, err)
	}
	for n := range 5 {
		if _, err := bridge.DecodeStoreResponse(make([]byte, n)); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("DecodeStoreResponse(%d bytes) = %v, want ErrMalformed", n, err)
		}
	}
	limit := 64<<20 - 5
	if _, err := bridge.EncodeStoreResponse(bridge.StoreResponse{ID: 1, Payload: make([]byte, limit)}); err != nil {
		t.Errorf("EncodeStoreResponse(the largest payload) = %v", err)
	}
	if _, err := bridge.EncodeStoreResponse(bridge.StoreResponse{ID: 1, Payload: make([]byte, limit+1)}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeStoreResponse(one byte over) = %v, want ErrMalformed", err)
	}
	if _, err := bridge.DecodeStoreResponse(make([]byte, 64<<20+1)); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeStoreResponse(over the wire limit) = %v, want ErrMalformed", err)
	}

	// Range entries.
	entries := []store.Entry{
		{Key: []string{"a", "b"}, Data: []byte("one")},
		{Key: []string{"c"}, Data: nil},
		{Key: []string{strings.Repeat("k", store.MaxComponent)}, Data: []byte("x")},
	}
	enc, err := bridge.EncodeRange(entries)
	if err != nil {
		t.Fatal(err)
	}
	rejectsEveryPrefixAndExtension(t, "range", enc, func(b []byte) error { _, err := bridge.DecodeRange(b); return err })
	if back, err := bridge.DecodeRange(enc); err != nil || len(back) != 3 || string(back[0].Data) != "one" || len(back[2].Key[0]) != store.MaxComponent {
		t.Errorf("range round trip = %+v, %v", back, err)
	}
	if got, err := bridge.DecodeRange([]byte{0, 0, 0, 0}); err != nil || len(got) != 0 {
		t.Errorf("DecodeRange(empty) = %v, %v; want an empty range", got, err)
	}
	// Write-at: the u16 point length.
	for _, point := range [][]byte{nil, []byte("p"), bytes.Repeat([]byte("p"), math.MaxUint16)} {
		w, err := bridge.EncodeWriteAt(point, []byte("text"))
		if err != nil {
			t.Fatal(err)
		}
		p, text, err := bridge.DecodeWriteAt(w)
		if err != nil || !bytes.Equal(p, point) || string(text) != "text" {
			t.Errorf("write-at round trip with a point of %d bytes = %d, %q, %v", len(point), len(p), text, err)
		}
	}
	if _, err := bridge.EncodeWriteAt(make([]byte, math.MaxUint16+1), nil); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeWriteAt(point of 65536 bytes) = %v, want ErrMalformed", err)
	}
	for name, w := range map[string][]byte{"empty": nil, "one byte": {0}, "point cut": {0, 5, 'a'}} {
		if _, _, err := bridge.DecodeWriteAt(w); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("DecodeWriteAt(%s) = %v, want ErrMalformed", name, err)
		}
	}
	if p, text, err := bridge.DecodeWriteAt([]byte{0, 1, 'p'}); err != nil || string(p) != "p" || len(text) != 0 {
		t.Errorf("DecodeWriteAt(point only) = %q, %q, %v; want the point and no text", p, text, err)
	}
}

func TestStoreRequestLimits(t *testing.T) {
	long := strings.Repeat("k", store.MaxComponent)
	for _, tc := range []struct {
		name string
		r    bridge.StoreRequest
		ok   bool
	}{
		{"load of a key of the full depth", bridge.StoreRequest{ID: 1, Op: bridge.OpLoad, Key: []string{"a", "b", "c", "d"}}, true},
		{"load of a key one level too deep", bridge.StoreRequest{ID: 1, Op: bridge.OpLoad, Key: []string{"a", "b", "c", "d", "e"}}, false},
		{"component of the longest size", bridge.StoreRequest{ID: 1, Op: bridge.OpLoad, Key: []string{long}}, true},
		{"component one byte too long", bridge.StoreRequest{ID: 1, Op: bridge.OpLoad, Key: []string{long + "k"}}, false},
		{"request id 0", bridge.StoreRequest{ID: 0, Op: bridge.OpLoad, Key: []string{"a"}}, false},
		{"largest id", bridge.StoreRequest{ID: math.MaxUint32, Op: bridge.OpLoad, Key: []string{"a"}}, true},
		{"data on a load", bridge.StoreRequest{ID: 1, Op: bridge.OpLoad, Key: []string{"a"}, Data: []byte{1}}, false},
		{"save of the largest value", bridge.StoreRequest{ID: 1, Op: bridge.OpSave, Key: []string{"a"}, Data: make([]byte, store.MaxValue)}, true},
		{"save one byte over", bridge.StoreRequest{ID: 1, Op: bridge.OpSave, Key: []string{"a"}, Data: make([]byte, store.MaxValue+1)}, false},
		{"list files with a key", bridge.StoreRequest{ID: 1, Op: bridge.OpListFiles, Key: []string{"a"}}, false},
		{"list files", bridge.StoreRequest{ID: 1, Op: bridge.OpListFiles}, true},
		{"path op of the full depth", bridge.StoreRequest{ID: 1, Op: bridge.OpReadProjection, Key: repeat("d", project.MaxDepth-1, "f.typ")}, true},
		{"path op one level too deep", bridge.StoreRequest{ID: 1, Op: bridge.OpReadProjection, Key: repeat("d", project.MaxDepth, "f.typ")}, false},
		{"path op with no key", bridge.StoreRequest{ID: 1, Op: bridge.OpReadProjection}, false},
		{"unknown op", bridge.StoreRequest{ID: 1, Op: 250}, false},
	} {
		enc, err := bridge.EncodeStoreRequest(tc.r)
		if tc.ok != (err == nil) || (err != nil && !errors.Is(err, bridge.ErrMalformed)) {
			t.Errorf("EncodeStoreRequest(%s) = %v, want accepted=%t", tc.name, err, tc.ok)
			continue
		}
		if tc.ok {
			got, err := bridge.DecodeStoreRequest(enc)
			if err != nil || got.ID != tc.r.ID || got.Op != tc.r.Op || !slices.Equal(got.Key, tc.r.Key) || len(got.Data) != len(tc.r.Data) {
				t.Errorf("round trip of %s = %+v, %v", tc.name, got.ID, err)
			}
		}
	}
	// A refused frame still tells its id, so that it can be answered.
	bad := binary.BigEndian.AppendUint32(nil, 99)
	bad = append(bad, byte(bridge.OpLoad), 9)
	if r, err := bridge.DecodeStoreRequest(bad); !errors.Is(err, bridge.ErrMalformed) || r.ID != 99 {
		t.Errorf("DecodeStoreRequest(bad key) = id %d, %v; want id 99 and ErrMalformed", r.ID, err)
	}
	for n := range 6 {
		if _, err := bridge.DecodeStoreRequest(make([]byte, n)); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("DecodeStoreRequest(%d bytes) = %v, want ErrMalformed", n, err)
		}
	}
	// A key whose component runs past the frame by one byte.
	cut := append(binary.BigEndian.AppendUint32(nil, 1), byte(bridge.OpLoad), 1, 3, 'a', 'b')
	if _, err := bridge.DecodeStoreRequest(cut); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeStoreRequest(component of 3, 2 bytes) = %v, want ErrMalformed", err)
	}
	exact := append(binary.BigEndian.AppendUint32(nil, 1), byte(bridge.OpLoad), 1, 2, 'a', 'b')
	if r, err := bridge.DecodeStoreRequest(exact); err != nil || len(r.Key) != 1 || r.Key[0] != "ab" {
		t.Errorf("DecodeStoreRequest(component of 2, 2 bytes) = %+v, %v", r, err)
	}
}

// repeat returns n copies of dir followed by last.
func repeat(dir string, n int, last string) []string {
	out := make([]string, 0, n+1)
	for range n {
		out = append(out, dir)
	}
	return append(out, last)
}
