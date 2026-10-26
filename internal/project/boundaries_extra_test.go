package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestListNaturalOrder: numbers inside names count by value, case is ignored,
// and the order is total (leading zeros, prefixes, equal-up-to-case names).
func TestListNaturalOrder(t *testing.T) {
	p := create(t, newBase(t), "p")
	names := []string{
		"a10.txt", "a9.txt", "a2.txt", "a1.txt", "a100.txt", "a09.txt", "a010.txt",
		"B.txt", "b2.txt", "B10.txt", "c.txt", "Z.txt", "y.txt", "`.txt", "A.txt", "a.txt", "ch2", "ch10", "ch1",
		"x1y2.txt", "x1y10.txt", "x01y3.txt",
	}
	for _, n := range names {
		put(t, p, n, "x")
	}
	files, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	got := paths(files)
	// Folders do not exist here; the order within the folder is what we pin.
	want := []string{
		"`.txt",          // 0x60 sorts before the letters, A included
		"A.txt", "a.txt", // same letters: shorter first, then byte order
		"a1.txt", "a2.txt", "a9.txt", "a09.txt", "a10.txt", "a010.txt", "a100.txt", // by value, zeros after the plain form
		"B.txt", "b2.txt", "B10.txt", // B == b, then 2 < 10
		"c.txt",
		"ch1", "ch2", "ch10",
		"x1y2.txt", "x01y3.txt", "x1y10.txt",
		"y.txt", "Z.txt",
	}
	if !slices.Equal(got, want) {
		t.Errorf("List order =\n%v\nwant\n%v", got, want)
	}
}

// TestListFoldersFirstAndDepth: sub-folders come before the files of a
// folder, and the recursion stops past maxListDepth.
func TestListFoldersFirstAndDepth(t *testing.T) {
	p := create(t, newBase(t), "p")
	put(t, p, "a.txt", "x")
	put(t, p, "z/inner.txt", "x")
	put(t, p, "b.txt", "x")
	files, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := paths(files), []string{"z/inner.txt", "a.txt", "b.txt"}; !slices.Equal(got, want) {
		t.Errorf("List() = %v, want sub-folders first: %v", got, want)
	}

	deep := strings.Repeat("d/", maxListDepth) // a file in the folder at depth maxListDepth
	put(t, p, deep+"at-limit.txt", "x")
	tooDeep := strings.Repeat("d/", maxListDepth+1)
	put(t, p, tooDeep+"past-limit.txt", "x")
	files, err = p.List()
	if err != nil {
		t.Fatal(err)
	}
	has := func(rel string) bool { return slices.Contains(paths(files), rel) }
	if !has(deep + "at-limit.txt") {
		t.Errorf("a file %d folders deep was not listed", maxListDepth)
	}
	if has(tooDeep + "past-limit.txt") {
		t.Errorf("a file %d folders deep was listed", maxListDepth+1)
	}
}

// TestKindOfSniffsTruncatedRunes: a text file whose 4096-byte sniff window
// ends inside a multi-byte character is still text; a really invalid one is not.
func TestKindOfSniffsTruncatedRunes(t *testing.T) {
	p := create(t, newBase(t), "p")
	for cut, n := range map[string]int{"2-byte": 1, "3-byte": 2, "4-byte": 3} {
		r := map[string]string{"2-byte": "é", "3-byte": "€", "4-byte": "😀"}[cut]
		// Position the character so that only its first n... bytes are inside the window.
		body := strings.Repeat("a", sniffLen-n) + r + "tail"
		put(t, p, "cut-"+cut, body)
	}
	put(t, p, "bad", strings.Repeat("a", 100)+"\xff\xfe\xfd"+strings.Repeat("b", 100)+"\x00")
	put(t, p, "binary", "\x00\x01\x02")
	files, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]Kind{}
	for _, f := range files {
		kinds[f.Path] = f.Kind
	}
	for _, name := range []string{"cut-2-byte", "cut-3-byte", "cut-4-byte"} {
		if kinds[name] != KindText {
			t.Errorf("kind of %s = %v, want text (a rune cut by the sniff window)", name, kinds[name])
		}
	}
	for _, name := range []string{"bad", "binary"} {
		if kinds[name] != KindOther {
			t.Errorf("kind of %s = %v, want other", name, kinds[name])
		}
	}
}

func TestTrimPartialRune(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
		want []byte
	}{
		{"empty", nil, nil},
		{"ascii", []byte("abc"), []byte("abc")},
		{"complete 2-byte", []byte("aé"), []byte("aé")},
		{"cut 2-byte", []byte("a\xc3"), []byte("a")},
		{"cut 3-byte after 1", []byte("a\xe2"), []byte("a")},
		{"cut 3-byte after 2", []byte("a\xe2\x82"), []byte("a")},
		{"cut 4-byte after 3", []byte("a\xf0\x9f\x98"), []byte("a")},
		{"complete 4-byte", []byte("a\U0001F600"), []byte("a\U0001F600")},
		{"lone continuation", []byte("a\x80"), []byte("a\x80")},
		{"invalid kept", []byte("a\xffb"), []byte("a\xffb")},
		{"only a lead byte", []byte{0xc3}, nil},
		{"four continuation bytes are not a cut", []byte{'a', 0x80, 0x80, 0x80, 0x80}, []byte{'a', 0x80, 0x80, 0x80, 0x80}},
	} {
		if got := trimPartialRune(tc.in); !bytes.Equal(got, tc.want) {
			t.Errorf("%s: trimPartialRune(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestSyncPointSizeBoundary(t *testing.T) {
	jsonString := func(n int) []byte { return append(append([]byte(`"`), bytes.Repeat([]byte("a"), n-2)...), '"') }
	base := newBase(t)
	p := create(t, base, "p")
	exact := jsonString(MaxSyncPoint)
	if err := p.WriteProjectionAt(MainFile, []byte("x"), exact); err != nil {
		t.Fatalf("WriteProjectionAt(point of exactly MaxSyncPoint bytes) = %v, want nil", err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	q, err := Open(base, "p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	if got, err := q.SyncPoint(MainFile); err != nil || !bytes.Equal(got, exact) {
		t.Errorf("the largest sync point did not survive a restart: %d bytes, %v", len(got), err)
	}
	for name, bad := range map[string][]byte{
		"one byte over": jsonString(MaxSyncPoint + 1),
		"empty":         {},
		"truncated":     []byte(`{"a":`),
		"trailing":      []byte(`{} {}`),
	} {
		if err := q.WriteProjectionAt(MainFile, []byte("y"), bad); !errors.Is(err, ErrSyncPoint) {
			t.Errorf("WriteProjectionAt(%s) = %v, want ErrSyncPoint", name, err)
		}
	}
	if got, _ := q.SyncPoint(MainFile); !bytes.Equal(got, exact) {
		t.Error("a refused point disturbed the recorded one")
	}
	if err := checkSyncPoint(nil); err != nil {
		t.Errorf("checkSyncPoint(nil) = %v, want nil (no point)", err)
	}
}

func TestReadLimited(t *testing.T) {
	base := newBase(t)
	for _, size := range []int{0, 9, 10, 11} {
		if err := base.WriteFile(fmt.Sprintf("f%d", size), bytes.Repeat([]byte("z"), size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		size    int
		tooBig  bool
		content int
	}{{0, false, 0}, {9, false, 9}, {10, false, 10}, {11, true, 0}} {
		got, err := readLimited(base, fmt.Sprintf("f%d", tc.size), 10)
		if tc.tooBig != (err != nil) || len(got) != tc.content {
			t.Errorf("readLimited(%d bytes, limit 10) = %d bytes, %v; want %d bytes, error %t", tc.size, len(got), err, tc.content, tc.tooBig)
		}
	}
	if _, err := readLimited(base, "missing", 10); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("readLimited(missing) = %v, want ErrNotExist", err)
	}
}

func TestMakeFolderCollisions(t *testing.T) {
	base := newBase(t)
	for i, want := range []string{"t", "t (2)", "t (3)"} {
		got, err := makeFolder(base, "t")
		if err != nil || got != want {
			t.Fatalf("makeFolder #%d = %q, %v; want %q", i+1, got, err, want)
		}
	}
	// Up to maxCollisions candidates are tried: the last one is usable, then none.
	for n := 4; n < maxCollisions; n++ {
		if err := base.Mkdir(fmt.Sprintf("t (%d)", n), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	got, err := makeFolder(base, "t")
	if err != nil || got != fmt.Sprintf("t (%d)", maxCollisions) {
		t.Fatalf("makeFolder with one slot left = %q, %v; want t (%d)", got, err, maxCollisions)
	}
	if got, err := makeFolder(base, "t"); err == nil {
		t.Errorf("makeFolder with every slot taken = %q, nil; want an error", got)
	}
}

func TestFolderName(t *testing.T) {
	long := strings.Repeat("é", maxFolderRunes) // 160 bytes, 80 runes
	for _, tc := range []struct{ in, want string }{
		{"My book", "My book"},
		{"", DefaultTitle},
		{"...", DefaultTitle},
		{"  . ", DefaultTitle},
		{" .a. ", "a"},
		{`a/b\c:d*e?f"g<h>i|j`, "a-b-c-d-e-f-g-h-i-j"},
		{"a\x00b\tc\u0085d", "a-b-c-d"},
		{long, long},
		{long + "x", long},
		{strings.Repeat("a", maxFolderRunes-1) + ". . tail", strings.Repeat("a", maxFolderRunes-1)}, // trailing dots and spaces go after the cut too
		{strings.Repeat("a", maxFolderRunes+5), strings.Repeat("a", maxFolderRunes)},
	} {
		if got := folderName(tc.in); got != tc.want {
			t.Errorf("folderName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestCheckTreeLimits: the tree limits are exact, entry by entry.
func TestCheckTreeLimits(t *testing.T) {
	text := func(p string) TreeEntry { return TreeEntry{Path: p, Type: NodeText, URL: "automerge:x"} }
	blob := func(p string, size int64) TreeEntry { return TreeEntry{Path: p, Type: NodeBlob, Size: size} }
	deepest := strings.Repeat("d/", MaxDepth-1) + "f.typ"
	tooDeep := strings.Repeat("d/", MaxDepth) + "f.typ"
	for _, tc := range []struct {
		name    string
		entry   TreeEntry
		wantErr error // nil: accepted
		anyErr  bool
	}{
		{name: "text", entry: text("a.typ")},
		{name: "deepest allowed", entry: text(deepest)},
		{name: "one level too deep", entry: text(tooDeep), wantErr: ErrTreeConflict},
		{name: "empty blob", entry: blob("a.bin", 0)},
		{name: "negative blob", entry: blob("a.bin", -1), anyErr: true},
		{name: "text without document", entry: TreeEntry{Path: "a.typ", Type: NodeText}, anyErr: true},
		{name: "unknown type", entry: TreeEntry{Path: "a.typ", Type: 99}, anyErr: true},
		{name: "folder", entry: TreeEntry{Path: "dir", Type: NodeDir}},
		{name: "bad name", entry: text("a/../b.typ"), wantErr: ErrInvalidName},
	} {
		accepted, rejected := checkTree([]TreeEntry{tc.entry})
		switch {
		case tc.wantErr == nil && !tc.anyErr:
			if len(accepted) != 1 || len(rejected) != 0 {
				t.Errorf("%s: accepted %d, rejected %v; want accepted", tc.name, len(accepted), rejected)
			}
		case len(accepted) != 0 || len(rejected) != 1:
			t.Errorf("%s: accepted %d, rejected %d; want one rejection", tc.name, len(accepted), len(rejected))
		case tc.wantErr != nil && !errors.Is(rejected[0].Err, tc.wantErr):
			t.Errorf("%s: rejected with %v, want %v", tc.name, rejected[0].Err, tc.wantErr)
		}
	}
}

// TestCheckTreeCollisions: the same name up to case or normalization, a file
// where a folder is needed, and a folder where a file stands, in any input
// order; the path that sorts first wins.
func TestCheckTreeCollisions(t *testing.T) {
	text := func(p string) TreeEntry { return TreeEntry{Path: p, Type: NodeText, URL: "automerge:" + p} }
	dir := func(p string) TreeEntry { return TreeEntry{Path: p, Type: NodeDir} }
	for _, tc := range []struct {
		name         string
		in           []TreeEntry
		wantAccepted []string
		wantRejected []string
	}{
		{"case twins", []TreeEntry{text("b/A.typ"), text("b/a.typ")}, []string{"b/A.typ"}, []string{"b/a.typ"}},
		{"normalization twins", []TreeEntry{text("é.typ"), text("é.typ")}, []string{"é.typ"}, []string{"é.typ"}},
		{"folder spelled two ways", []TreeEntry{text("Dir/a.typ"), text("dir/b.typ")}, []string{"Dir/a.typ"}, []string{"dir/b.typ"}},
		{"file where a folder is needed", []TreeEntry{text("a"), text("a/b.typ")}, []string{"a"}, []string{"a/b.typ"}},
		{"folder over a file", []TreeEntry{dir("a"), text("a")}, []string{"a"}, []string{"a"}},
		{"file over a folder entry", []TreeEntry{text("a"), dir("a")}, []string{"a"}, []string{"a"}},
		{"same folder twice is harmless", []TreeEntry{dir("a"), dir("a")}, []string{"a", "a"}, nil},
		{"folder and its files", []TreeEntry{dir("a"), text("a/x.typ"), text("a/y.typ")}, []string{"a", "a/x.typ", "a/y.typ"}, nil},
		{"independent", []TreeEntry{text("z.typ"), text("m/n.typ"), text("a.typ")}, []string{"a.typ", "m/n.typ", "z.typ"}, nil},
	} {
		for _, in := range [][]TreeEntry{tc.in, reversed(tc.in)} {
			accepted, rejected := checkTree(in)
			var gotA, gotR []string
			for _, e := range accepted {
				gotA = append(gotA, e.Path)
			}
			for _, r := range rejected {
				gotR = append(gotR, r.Path)
				if !errors.Is(r.Err, ErrTreeConflict) {
					t.Errorf("%s: %q rejected with %v, want ErrTreeConflict", tc.name, r.Path, r.Err)
				}
			}
			if !slices.Equal(gotA, tc.wantAccepted) || !slices.Equal(gotR, tc.wantRejected) {
				t.Errorf("%s: accepted %q rejected %q; want %q and %q", tc.name, gotA, gotR, tc.wantAccepted, tc.wantRejected)
			}
		}
	}
}

func reversed(es []TreeEntry) []TreeEntry {
	out := slices.Clone(es)
	slices.Reverse(out)
	return out
}
