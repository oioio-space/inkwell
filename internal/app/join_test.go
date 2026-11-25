package app

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// Tests of the join by restart (plan docs/plans/2026-11-15-pc1b-join.md, rules J-1, J-4, J-5 and
// J-6, with N-12, N-13 and N-16 of docs/plans/2026-11-14-pc1-project-change-by-restart.md on the
// join path). Every folder is a temporary one.

// notInkwell skips the private directory of a project folder in [treeFiles].
func notInkwell(rel string) bool { return rel == ".inkwell" || strings.HasPrefix(rel, ".inkwell/") }

// openFolder opens the project folder at path.
func openFolder(t *testing.T, path string) *project.Project {
	t.Helper()
	base, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close() })
	p, err := project.Open(base, filepath.Base(path))
	if err != nil {
		t.Fatalf("the folder %s does not open: %v", path, err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// stagings returns the staging folders left in parent.
func stagings(t *testing.T, parent string) []string {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), project.StagingPrefix) {
			out = append(out, e.Name())
		}
	}
	return out
}

// markerOf returns the staging folder the marker of folder names, or "" without a marker.
func markerOf(t *testing.T, folder string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(folder, ".inkwell", joinMarkerFile))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	var m struct {
		ID string `json:"id"`
		V  int    `json:"v"`
	}
	if err != nil || json.Unmarshal(b, &m) != nil || m.V != 1 {
		t.Fatalf("the marker of %s is unreadable: %v %q", folder, err, b)
	}
	return project.StagingPrefix + m.ID
}

// joinCase is a previous project folder and a verified Welcome into another project, written
// outside a runtime.
type joinCase struct {
	parent, folder string
	home           *os.Root
	joiner, host   *identity.Device
	st             *opStore
	joined         *invite.Joined
	h              *heldJoin
	oldID, newID   trust.ProjectID
	text           map[string]string // the previous folder without .inkwell, before the join
}

const oldRoot = "automerge:oldRoot1"

func newJoinCase(t *testing.T, title, root string) *joinCase {
	t.Helper()
	proj, folder := newProjectFolder(t)
	if err := proj.SetRootURL(oldRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "main.typ"), []byte("= Mon livre\nmon texte\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &joinCase{parent: filepath.Dir(folder), folder: folder, home: newHome(t), host: newDevice(t)}
	var err error
	if c.joiner, err = identity.LoadOrCreate(c.home); err != nil {
		t.Fatal(err)
	}
	old, st, err := openTrust(proj, c.joiner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.close() })
	c.st, c.oldID = st, old.Project()
	g, id := trust.Genesis(c.host, "Roman", root)
	list := trust.New(id)
	if err := list.Apply(g); err != nil {
		t.Fatal(err)
	}
	if _, err := list.Issue(c.host, trust.OpAdd, c.joiner.Public(), trust.RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	c.newID = id
	c.joined = &invite.Joined{List: list, Host: p2p.IDFromKey(c.host.Public()), RootURL: root}
	if c.h, err = newHeldJoin(c.joined, title, []string{"/ip4/127.0.0.1/udp/1/quic-v1"}); err != nil {
		t.Fatal(err)
	}
	c.text = treeFiles(t, folder, notInkwell)
	return c
}

func (c *joinCase) finish() (string, error) {
	return finishJoin(c.st, c.folder, c.h, slog.New(slog.DiscardHandler))
}

// nextStart checks what the next start finds in parent, a copy of the parent folder as a crash
// or the end of a join left it: the previous folder recovers (no staging folder, no marker), with
// its text, its root and its member list as they were; a joined folder is whole, on the new list,
// and waits for its root or holds the one that consumed its record, never the old root. tag
// starts the failures. It returns the joined folders.
func (c *joinCase) nextStart(t *testing.T, tag, what, parent string) []string {
	t.Helper()
	prev := filepath.Join(parent, filepath.Base(c.folder))
	p := openFolder(t, prev)
	if _, err := recoverJoin(p); err != nil {
		t.Fatalf("%s: %s: recovering the previous folder: %v", tag, what, err)
	}
	if left, marker := stagings(t, parent), markerOf(t, prev); len(left) > 0 || marker != "" {
		t.Errorf("%s: %s: after the next start of the previous folder, staging %v and marker %q remain", tag, what, left, marker)
	}
	list, st, err := openTrust(p, c.joiner)
	if err != nil {
		t.Fatalf("%s: %s: the previous member list does not open: %v", tag, what, err)
	}
	_ = st.close()
	if diff := changed(c.text, treeFiles(t, prev, notInkwell)); len(diff) > 0 || list.Project() != c.oldID || p.Meta().RootURL != oldRoot {
		t.Errorf("%s: %s: the previous folder changed: files %v, project %v (was %v), root %q", tag, what, diff, list.Project(), c.oldID, p.Meta().RootURL)
	}
	if _, ok := p.PendingJoin(); ok {
		t.Errorf("%s: %s: the previous folder holds a join record", tag, what)
	}
	base, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	if latest, err := project.Latest(base); err != nil || strings.HasPrefix(latest, project.StagingPrefix) {
		t.Errorf("%s: %s: the most recent project is %q (%v), never a staging folder", tag, what, latest, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	var joined []string
	for _, e := range entries {
		if e.Name() == filepath.Base(c.folder) {
			continue
		}
		dir := filepath.Join(parent, e.Name())
		j := openFolder(t, dir)
		jl, jst, err := openTrust(j, c.joiner)
		if err != nil {
			t.Fatalf("%s: %s: the joined member list does not open: %v", tag, what, err)
		}
		_ = jst.close()
		rec, waiting := j.PendingJoin()
		switch root := j.Meta().RootURL; {
		case jl.Project() != c.newID || standingOf(jl, c.joiner.Public()) != StandingMember:
			t.Errorf("%s: %s: the joined folder %s is on project %v, standing %q; want the joined project, member", tag, what, e.Name(), jl.Project(), standingOf(jl, c.joiner.Public()))
		case waiting && (root != "" || rec.Project != c.newID.String() || rec.Inviter != string(c.joined.Host)):
			t.Errorf("%s: %s: the joined folder waits with root %q and record %+v; want no root and the record of the join", tag, what, root, rec)
		case !waiting && (root == "" || root == oldRoot):
			t.Errorf("%s: %s: the joined folder holds no record and the root %q: an old or a fresh root under the new list", tag, what, root)
		}
		joined = append(joined, dir)
	}
	return joined
}

// crashCopyOf copies the parent folder of c as a crash at this instant leaves it.
func (c *joinCase) crashCopyOf(t *testing.T) string {
	t.Helper()
	return crashCopy(t, c.parent)
}

func TestJoinWritesRecordBeforeTheList(t *testing.T) {
	c := newJoinCase(t, "Roman", "")
	var steps, problems []string
	staged := func() string { return filepath.Join(c.parent, project.StagingPrefix+c.h.record.ID) }
	c.st.step = func(name string) error {
		steps = append(steps, name)
		ops := opFiles(readDirOrNone(filepath.Join(staged(), ".inkwell", trustDir)))
		manifest, _ := os.ReadFile(filepath.Join(staged(), ".inkwell", "project.json"))
		hasRecord := strings.Contains(string(manifest), `"pendingJoin"`)
		switch {
		case name == stepJoinRecord && len(ops) > 0:
			problems = append(problems, fmt.Sprintf("%d ops are staged before the record", len(ops)))
		case name == stepJoinList && !hasRecord:
			problems = append(problems, "an op of the list is staged while project.json holds no record")
		}
		return nil
	}
	joined, err := c.finish()
	if err != nil {
		t.Fatalf("J-1 record before the list: the join failed: %v", err)
	}
	r, l := slices.Index(steps, stepJoinRecord), slices.Index(steps, stepJoinList)
	if r < 0 || l < 0 || r > l || len(problems) > 0 {
		t.Errorf("J-1 record before the list: steps %v, %v", steps, problems)
	}
	want := []string{stepJoinMarker, stepJoinStaging, stepJoinRecord, stepJoinList, stepJoinList, stepJoinInstall, stepJoinInstalled}
	if !slices.Equal(steps, want) {
		t.Errorf("J-1 the steps of a join = %v, want %v", steps, want)
	}
	rec, ok := openFolder(t, joined).PendingJoin()
	if !ok || rec.V != 1 || len(rec.ID) != 32 || rec.Project != c.newID.String() || rec.Inviter != string(c.joined.Host) || len(rec.Addrs) != 1 || rec.Root != "" {
		t.Errorf("J-1 the record of the joined folder = %+v, %v; want v 1, a 128-bit id, the project, the inviter and its addresses, no root", rec, ok)
	}
}

// readDirOrNone is [readDir] of a folder that may not exist yet.
func readDirOrNone(dir string) map[string]string {
	out := map[string]string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if b, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			out[e.Name()] = string(b)
		}
	}
	return out
}

func TestJoinCrashAtEachStep(t *testing.T) {
	c := newJoinCase(t, "Roman", "")
	type snap struct{ name, parent string }
	var snaps []snap
	c.st.step = func(name string) error {
		snaps = append(snaps, snap{fmt.Sprintf("%d-%s", len(snaps), name), c.crashCopyOf(t)})
		return nil
	}
	joined, err := c.finish()
	if err != nil {
		t.Fatalf("J-1 the join failed: %v", err)
	}
	c.st.step = nil
	snaps = append(snaps, snap{"installed", c.crashCopyOf(t)})
	if err := openFolder(t, joined).CompleteJoin("automerge:joinedRoot1"); err != nil {
		t.Fatal(err)
	}
	snaps = append(snaps, snap{"after the consuming rewrite", c.crashCopyOf(t)})
	if len(snaps) < 8 {
		t.Fatalf("J-1 a crash at each step: only %d steps were seen", len(snaps))
	}
	for i, s := range snaps {
		t.Run(s.name, func(t *testing.T) {
			got := c.nextStart(t, "J-1 a crash opens the previous folder unchanged or the joined one waiting", "crash at "+s.name, s.parent)
			if wantJoined := i >= len(snaps)-3; (len(got) == 1) != wantJoined { // from the rename on
				t.Errorf("J-1 a crash at %s leaves joined folders %v, want one: %v", s.name, got, wantJoined)
			}
		})
	}
}

func TestJoinCrashDuringInstall(t *testing.T) {
	c := newJoinCase(t, "Roman", "")
	type snap struct{ name, parent string }
	var snaps []snap
	listed := 0
	c.st.step = func(name string) error {
		// The one way back to a staging folder is the marker, from before the folder exists.
		if left, marker := stagings(t, c.parent), markerOf(t, c.folder); len(left) > 0 && !slices.Equal(left, []string{marker}) {
			t.Errorf("J-6 staging found again: at %s the staging folders are %v and the marker names %q", name, left, marker)
		}
		point := ""
		switch name {
		case stepJoinStaging:
			point = "after the marker"
		case stepJoinList:
			if listed++; listed == 2 {
				point = "the staging folder half written"
			}
		case stepJoinInstall:
			point = "before the rename"
		case stepJoinInstalled:
			point = "after the rename"
		}
		if point != "" {
			snaps = append(snaps, snap{point, c.crashCopyOf(t)})
		}
		return nil
	}
	if _, err := c.finish(); err != nil {
		t.Fatalf("J-6 the join failed: %v", err)
	}
	if len(snaps) != 4 {
		t.Fatalf("J-6 crash points: got %d, want 4", len(snaps))
	}
	for _, s := range snaps {
		t.Run(s.name, func(t *testing.T) {
			prev := filepath.Join(s.parent, filepath.Base(c.folder))
			left, marker := stagings(t, s.parent), markerOf(t, prev)
			if marker == "" || (len(left) > 0 && left[0] != marker) {
				t.Fatalf("J-6 staging found again: a crash %s leaves the marker %q and the staging folders %v", s.name, marker, left)
			}
			wantStaging := s.name == "the staging folder half written" || s.name == "before the rename"
			if (len(left) == 1) != wantStaging {
				t.Errorf("J-6 a crash %s leaves the staging folders %v", s.name, left)
			}
			before := treeFiles(t, prev, func(rel string) bool { return rel == ".inkwell/"+joinMarkerFile })
			joined := c.nextStart(t, "J-6 a crash during the install", "crash "+s.name, s.parent)
			if (len(joined) == 1) != (s.name == "after the rename") {
				t.Errorf("J-6 a crash %s: joined folders %v: the joined folder is complete after the rename, absent before", s.name, joined)
			}
			if diff := changed(before, treeFiles(t, prev, nil)); len(diff) > 0 {
				t.Errorf("J-6 a crash %s: the previous folder changed but for its marker: %v", s.name, diff)
			}
		})
	}

	t.Run("a marker that names no folder of the parent", func(t *testing.T) {
		for _, id := range []string{"../" + filepath.Base(c.folder), "", strings.Repeat("g", 32), strings.Repeat("0", 32)} {
			cc := newJoinCase(t, "Roman", "")
			marker := filepath.Join(cc.folder, ".inkwell", joinMarkerFile)
			b, _ := json.Marshal(map[string]any{"v": 1, "id": id})
			if err := os.WriteFile(marker, b, 0o600); err != nil {
				t.Fatal(err)
			}
			if failed, err := recoverJoin(openFolder(t, cc.folder)); err != nil || failed {
				t.Errorf("J-6 a marker naming a missing folder (id %q) is removed alone: recoverJoin = %v, %v", id, failed, err)
			}
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("J-6 a marker naming a missing folder (id %q) is removed: %v", id, err)
			}
			if diff := changed(cc.text, treeFiles(t, cc.folder, notInkwell)); len(diff) > 0 {
				t.Errorf("J-6 a marker with the id %q made the previous folder change: %v", id, diff)
			}
		}
	})
}

func TestJoinFolderNameConfined(t *testing.T) {
	long := strings.Repeat("é", 500)
	for _, tc := range []struct{ name, title, want string }{
		{"plain", "Roman, tome 2 (brouillon)", "Roman, tome 2 (brouillon)"},
		{"parent traversal", "../x", "x"},
		{"slash", "a/b", "ab"},
		{"backslash", `a\b`, "ab"},
		{"dot", ".", "Document rejoint"},
		{"dot dot", "..", "Document rejoint"},
		{"hidden", ".hidden", "hidden"},
		{"leading spaces and dots", " . ..secret", "secret"},
		{"NUL", "a\x00b", "ab"},
		{"control characters", "a\x01\x1f\u007f\n\tb", "ab"},
		{"500 characters", long, strings.Repeat("é", 64)},
		{"only forbidden characters", `/\:*?"<>|`, "Document rejoint"},
		{"empty", "", "Document rejoint"},
		{"not NFC", "Café", "Café"},
		{"absolute", "/etc/passwd", "etcpasswd"},
		// Combining marks (Unicode category M) stay with their letters: these scripts need them.
		{"Devanagari", "../हिन्दी की किताब", "हिन्दी की किताब"},
		{"Thai", "นิยาย/เล่มที่ 2", "นิยายเล่มที่ 2"},
		{"leading combining mark", "\u0301.hidden", "hidden"}, // a mark with no letter before it goes: still no dot-name
		// The bound holds in bytes too: a file system refuses a name over 255 bytes, and « (99) » must fit.
		{"64 four-byte letters", strings.Repeat("\U00010400", 64), strings.Repeat("\U00010400", 60)},
		// No trailing dot or space: Windows and network shares strip them, so the name asked for would not be the name made.
		{"trailing dot", "Roman.", "Roman"},
		{"trailing spaces and dots", "Roman . .", "Roman"},
		{"cut after a space", strings.Repeat("a", 63) + " suite", strings.Repeat("a", 63)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinFolderName(tc.title); got != tc.want {
				t.Errorf("J-6 folder name confined: joinFolderName(%q) = %q, want %q", tc.title, got, tc.want)
			}
			c := newJoinCase(t, tc.title, "")
			around := readDirNames(t, filepath.Dir(c.parent))
			joined, err := c.finish()
			if err != nil {
				t.Fatalf("J-6 folder name confined: a join titled %q failed: %v", tc.title, err)
			}
			if got, want := readDirNames(t, c.parent), []string{filepath.Base(c.folder), tc.want}; joined != filepath.Join(c.parent, tc.want) || !sameNames(got, want) {
				t.Errorf("J-6 folder name confined: a join titled %q installed %q; the parent holds %q, want the new direct child %q beside the previous folder", tc.title, joined, got, tc.want)
			}
			if got := readDirNames(t, filepath.Dir(c.parent)); !sameNames(got, around) {
				t.Errorf("J-6 folder name confined: a join titled %q wrote outside the parent: %q, was %q", tc.title, got, around)
			}
		})
	}
}

// The default folder name is the sentence of the catalogue (key join.folderDefault): Go names the
// folder, the catalogue is where the wording is reviewed.
func TestJoinFolderDefaultIsTheCatalogueSentence(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "i18n", "fr.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fr map[string]string
	if err := json.Unmarshal(b, &fr); err != nil {
		t.Fatal(err)
	}
	if got := fr["join.folderDefault"]; got != joinFolderDefault || joinFolderName(got) != got {
		t.Errorf("J-6 the default folder name: the catalogue says %q, Go names the folder %q (normalised %q)", got, joinFolderDefault, joinFolderName(got))
	}
}

func readDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func sameNames(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

func TestJoinFolderCollisionGetsSuffix(t *testing.T) {
	for _, kind := range []string{"folder", "file", "symbolic link"} {
		t.Run(kind, func(t *testing.T) {
			c := newJoinCase(t, "Roman", "")
			taken := filepath.Join(c.parent, "Roman")
			target := t.TempDir()
			var err error
			switch kind {
			case "folder":
				err = os.Mkdir(taken, 0o700)
			case "file":
				err = os.WriteFile(taken, []byte("kept"), 0o600)
			case "symbolic link":
				err = os.Symlink(target, taken)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(taken)
			if err != nil {
				t.Fatal(err)
			}
			joined, err := c.finish()
			if err != nil || joined != taken+" (2)" {
				t.Fatalf("J-6 never replaces: with the name taken by a %s the join installed %q (%v), want %q", kind, joined, err, taken+" (2)")
			}
			after, err := os.Lstat(taken)
			if err != nil || !os.SameFile(before, after) || len(readDirNames(t, target)) != 0 {
				t.Errorf("J-6 never replaces: the %s that held the name was replaced or followed (%v)", kind, err)
			}
			if kind == "folder" && len(readDirNames(t, taken)) != 0 {
				t.Errorf("J-6 never replaces: the folder that held the name received %v", readDirNames(t, taken))
			}
			if _, ok := openFolder(t, joined).PendingJoin(); !ok {
				t.Error("J-6 the folder installed under the next name is not the joined one")
			}
		})
	}
	t.Run("99 names taken", func(t *testing.T) {
		c := newJoinCase(t, "Roman", "")
		for n := 1; n <= 99; n++ {
			name := "Roman"
			if n > 1 {
				name = fmt.Sprintf("Roman (%d)", n)
			}
			if err := os.Mkdir(filepath.Join(c.parent, name), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		names := readDirNames(t, c.parent)
		joined, err := c.finish()
		if err == nil || joined != "" {
			t.Fatalf("J-6 past (99) the join fails: installed %q, %v", joined, err)
		}
		if failed, err := recoverJoin(openFolder(t, c.folder)); err != nil || !failed {
			t.Errorf("J-4 the next start says the join failed: recoverJoin = %v, %v", failed, err)
		}
		if got := readDirNames(t, c.parent); !sameNames(got, names) || markerOf(t, c.folder) != "" {
			t.Errorf("J-6 past (99) nothing changed: the parent holds %d entries, was %d; marker %q", len(got), len(names), markerOf(t, c.folder))
		}
		if diff := changed(c.text, treeFiles(t, c.folder, notInkwell)); len(diff) > 0 {
			t.Errorf("J-6 past (99) nothing changed: %v", diff)
		}
	})
}

// invitation returns the text of an invitation of host for a writer.
func invitation(t *testing.T, host *Runtime) string {
	t.Helper()
	inv, err := host.Session.Invite("writer")
	if err != nil || inv.Text == "" {
		t.Fatalf("Invite = %+v, %v", inv, err)
	}
	return inv.Text
}

// holdJoin has guest accept an invitation of host into another project: the answer must say
// restart.
func holdJoin(t *testing.T, guest, host *Runtime) JoinResult {
	t.Helper()
	res, err := guest.Session.Join(t.Context(), invitation(t, host))
	if err != nil || !res.Restart || res.Peer != host.Session.LocalPeer() {
		t.Fatalf("J-1 a join into another project answers restart: Join = %+v, %v", res, err)
	}
	return res
}

// restartAfterJoin finishes the held join of guest, closes it as the application does, and
// starts the next image of the profile home on the folder the restart opens: the joined one, or
// prev when the join installed nothing.
func restartAfterJoin(t *testing.T, guest *Runtime, home *os.Root, prev *project.Project) *Runtime {
	t.Helper()
	if err := guest.Session.FinishJoin(t.Context()); err != nil {
		t.Fatalf("FinishJoin = %v", err)
	}
	_ = guest.Close() // the outcome is what the next start loads, whatever Close said
	if guest.RestartChange() == "" {
		t.Errorf("N-12 a join restarts: no restart recorded after FinishJoin and the shutdown (folder %q)", guest.RestartFolder())
	}
	proj := prev
	if dir := guest.RestartFolder(); dir != "" {
		proj = openFolder(t, dir)
	}
	return startIn(t, home, proj, true)
}

// joinedRuntime joins guest to the project of host and returns the next image of guest, on the
// joined folder.
func joinedRuntime(t *testing.T, guest *Runtime, d dirs, host *Runtime) *Runtime {
	t.Helper()
	holdJoin(t, guest, host)
	next := restartAfterJoin(t, guest, d.home, d.proj)
	if next.inv.folder == d.proj.Path() || next.wiring.list().Project() != host.wiring.list().Project() {
		t.Fatalf("the join did not restart on a joined folder: %s, project %v", next.inv.folder, next.wiring.list().Project())
	}
	return next
}

func TestJoinRequestWritesNothing(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, d := startRuntime(t, nil, false)
	parent := filepath.Dir(d.proj.Path())
	oldID := guest.wiring.list().Project()
	before := treeFiles(t, parent, nil)
	res := holdJoin(t, guest, host)
	if diff := changed(before, treeFiles(t, parent, nil)); len(diff) > 0 {
		t.Errorf("J-1 nothing written at the request: changed %v", diff)
	}
	if guest.wiring.list().Project() != oldID || guest.inv.currentList().Project() != oldID {
		t.Errorf("N-11 the project of a running session is never changed: after Join the session is on %v", guest.wiring.list().Project())
	}
	// The person's own folder is « Mon livre » too: the answer names the folder the install makes.
	if res.Folder != "Mon livre (2)" || res.Title != "Mon livre" || res.Current != "Mon livre" {
		t.Errorf("J-2 the answer names the document, the folder it gets and the open one: %+v, want « Mon livre », « Mon livre (2) », « Mon livre »", res)
	}
	if guest.RestartChange() != "" {
		t.Error("J-1 nothing restarts before FinishJoin: a restart is recorded at the request")
	}
}

func TestRequestsRefusedOnceJoinHeld(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	other, _ := startRuntime(t, nil, false)
	guest, _ := startRuntime(t, nil, false)
	second, third := invitation(t, other), invitation(t, host)
	holdJoin(t, guest, host)
	s := guest.Session
	refused := map[string]error{}
	_, refused["a join into a third project"] = s.Join(t.Context(), second)
	_, refused["a second join into the same project"] = s.Join(t.Context(), third)
	_, refused["TakeOver"] = s.TakeOver(t.Context())
	_, refused["Invite"] = s.Invite("writer")
	_, refused["InviteDevice"] = s.InviteDevice()
	_, refused["Ask"] = s.Ask(t.Context(), "peer", "Ana")
	for name, err := range refused {
		if !errors.Is(err, errLeaving) {
			t.Errorf("N-13 one change per image: %s while a Welcome is held = %v, want refused", name, err)
		}
	}
	if err := s.FinishJoin(t.Context()); err != nil {
		t.Fatalf("FinishJoin = %v", err)
	}
	if err := s.FinishJoin(t.Context()); !errors.Is(err, errNoWelcome) {
		t.Errorf("N-13 one change per image: a second FinishJoin = %v, want refused", err)
	}
	if _, err := s.Join(t.Context(), second); !errors.Is(err, errLeaving) {
		t.Errorf("N-13 one change per image: a join after FinishJoin = %v, want refused", err)
	}
}

// TestCancelChangeDropsTheHeldWelcome: the page that could not flush in time cancels (rule N-15):
// the held Welcome is dropped, nothing was written, FinishJoin has nothing to finish and the
// session accepts requests again.
func TestCancelChangeDropsTheHeldWelcome(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, d := startRuntime(t, nil, false)
	if err := guest.Session.FinishJoin(t.Context()); !errors.Is(err, errNoWelcome) {
		t.Errorf("J-1 FinishJoin with no held Welcome = %v, want refused", err)
	}
	parent := filepath.Dir(d.proj.Path())
	before := treeFiles(t, parent, nil)
	holdJoin(t, guest, host)
	guest.Session.CancelChange()
	if err := guest.Session.FinishJoin(t.Context()); !errors.Is(err, errNoWelcome) {
		t.Errorf("N-15 a cancelled Welcome is dropped: FinishJoin = %v, want refused", err)
	}
	if _, err := guest.Session.Invite("writer"); err != nil {
		t.Errorf("N-15 after the cancellation the session is as before: Invite = %v", err)
	}
	holdJoin(t, guest, host) // the inviter's list names this device: a new invitation works
	guest.Session.CancelChange()
	if err := guest.Close(); err != nil {
		t.Fatal(err)
	}
	if diff := changed(before, treeFiles(t, parent, nil)); len(diff) > 0 || (guest.RestartChange() != "") {
		t.Errorf("N-15 a cancelled join changes nothing: changed %v, restart recorded %v", diff, (guest.RestartChange() != ""))
	}
}

// TestFinishJoinConsumesTheDesktopApproval: on the desktop a join is recorded only with the close
// approval, consumed at FinishJoin (the irreversible drain), never at Join. Without it nothing is
// recorded and the held Welcome is dropped, so that the session is as before (rule N-15); a
// recording that fails releases the desktop hold.
func TestFinishJoinConsumesTheDesktopApproval(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, _ := startRuntime(t, nil, false)
	approved, confirms, rejects := false, 0, 0
	guest.SetProjectClose(func() bool { confirms++; return approved }, func() { rejects++ })
	holdJoin(t, guest, host)
	if confirms != 0 {
		t.Errorf("holding a Welcome consumed the desktop approval %d time(s), want 0", confirms)
	}
	if err := guest.Session.FinishJoin(t.Context()); err == nil || (guest.RestartChange() != "") {
		t.Fatalf("FinishJoin without the desktop approval = %v, restart %v; want refused, nothing recorded", err, (guest.RestartChange() != ""))
	}
	if err := guest.Session.FinishJoin(t.Context()); !errors.Is(err, errNoWelcome) {
		t.Errorf("N-15 a refused FinishJoin drops the held Welcome: second FinishJoin = %v, want errNoWelcome", err)
	}
	if _, err := guest.Session.Invite("writer"); err != nil {
		t.Errorf("N-15 after a refused FinishJoin the session is as before: Invite = %v", err)
	}
	approved = true
	holdJoin(t, guest, host)
	if err := guest.Session.FinishJoin(t.Context()); err != nil || guest.RestartChange() == "" {
		t.Fatalf("FinishJoin with the desktop approval = %v, restart %v; want recorded", err, (guest.RestartChange() != ""))
	}
	if confirms != 2 || rejects != 0 {
		t.Errorf("desktop approval asked %d time(s), released %d time(s); want 2 and 0", confirms, rejects)
	}
}

// TestJoinCancelledInvitationIsUsed: an invitation is used once. The host takes the ticket before it
// admits ([invite.Host]), so after « Annuler » the same text is refused as not valid and nothing
// is held; the page says so and asks for a new one (rule J-2), which works (rule J-4).
func TestJoinCancelledInvitationIsUsed(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, _ := startRuntime(t, nil, false)
	text := invitation(t, host)
	if res, err := guest.Session.Join(t.Context(), text); err != nil || !res.Restart {
		t.Fatalf("J-1 a join into another project answers restart: Join = %+v, %v", res, err)
	}
	guest.Session.CancelChange()
	_, err := guest.Session.Join(t.Context(), text)
	if refused, ok := errors.AsType[*invite.RefusedError](err); !ok || refused.Reason != invite.ReasonInvalid {
		t.Errorf("J-2 an invitation is used once: the same text after « Annuler » = %v, want refused as not valid", err)
	}
	if err := guest.Session.FinishJoin(t.Context()); !errors.Is(err, errNoWelcome) {
		t.Errorf("J-2 a refused invitation holds nothing: FinishJoin = %v, want refused", err)
	}
	holdJoin(t, guest, host) // a new invitation from the same person works
	guest.Session.CancelChange()
}

// TestInvitationStateFollowsTheTicket: the inviter's page learns from Go what became of the
// invitation it shows (rule J-8): open until the guest redeems it, used afterwards even when the
// guest cancels and never connects, expired past its end.
func TestInvitationStateFollowsTheTicket(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, _ := startRuntime(t, nil, false)
	text := invitation(t, host)
	if got, err := host.Session.InvitationState(text); err != nil || got != InvitationOpen {
		t.Fatalf("J-8 an invitation just made is open: %q, %v", got, err)
	}
	if res, err := guest.Session.Join(t.Context(), text); err != nil || !res.Restart {
		t.Fatalf("J-1 a join into another project answers restart: Join = %+v, %v", res, err)
	}
	guest.Session.CancelChange()
	if got, err := host.Session.InvitationState(text); err != nil || got != InvitationUsed {
		t.Errorf("J-8 a used invitation is not offered: after the guest's « Annuler » the state is %q, %v; want %q", got, err, InvitationUsed)
	}
	later := invitation(t, host)
	end := time.Now().Add(invite.TicketTTL + time.Minute)
	host.inv.now = func() time.Time { return end }
	if got, err := host.Session.InvitationState(later); err != nil || got != InvitationExpired {
		t.Errorf("J-8 an invitation past its end is expired: %q, %v", got, err)
	}
	if _, err := host.Session.InvitationState("not an invitation"); err == nil {
		t.Error("J-8 a text that is not an invitation is an error")
	}
}

func TestJoinSameProjectNoRestart(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	first, d := startRuntime(t, nil, false)
	guest := joinedRuntime(t, first, d, host)
	quits := 0
	guest.SetQuit(func() { quits++ })
	parent := filepath.Dir(d.proj.Path())
	skip := func(rel string) bool { return strings.Contains(rel, "/.inkwell/trust/") } // the list syncs by itself
	before := treeFiles(t, parent, skip)
	res, err := guest.Session.Join(t.Context(), invitation(t, host))
	if err != nil || res.Restart || res.Peer != host.Session.LocalPeer() {
		t.Errorf("J-5 no-op: a join into the running project = %+v, %v; want no restart", res, err)
	}
	if err := guest.Session.FinishJoin(t.Context()); !errors.Is(err, errNoWelcome) {
		t.Errorf("J-5 no-op: FinishJoin after it = %v, want nothing to finish", err)
	}
	if _, err := guest.Session.Invite("writer"); errors.Is(err, errLeaving) {
		t.Errorf("J-5 no-op: the session is leaving after a join into its own project: %v", err)
	}
	if err := guest.Close(); err != nil {
		t.Fatal(err)
	}
	if diff := changed(before, treeFiles(t, parent, skip)); len(diff) > 0 || quits != 0 || (guest.RestartChange() != "") {
		t.Errorf("J-5 no-op: changed %v, quit %d times, restart recorded %v", diff, quits, (guest.RestartChange() != ""))
	}
}

func TestJoinKeepsThePreviousFolder(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, d := startRuntime(t, nil, false)
	prev := d.proj.Path()
	if err := guest.Files.WriteProjection(project.MainFile, []byte("= Mon livre\nmon texte\n")); err != nil {
		t.Fatal(err)
	}
	before := treeFiles(t, prev, notInkwell)
	holdJoin(t, guest, host)
	next := restartAfterJoin(t, guest, d.home, d.proj)
	if diff := changed(before, treeFiles(t, prev, notInkwell)); len(diff) > 0 {
		t.Errorf("J-6 previous folder unchanged: after the restart: %v", diff)
	}
	// The first save of the joined document, where the restarted image runs.
	if err := next.Files.WriteProjection(project.MainFile, []byte("= Roman\nle texte de l'autre\n")); err != nil {
		t.Fatalf("J-6 the first save of the joined document: %v", err)
	}
	if diff := changed(before, treeFiles(t, prev, notInkwell)); len(diff) > 0 {
		t.Errorf("J-6 previous folder unchanged: after the first save of the joined document: %v", diff)
	}
	joined := guest.RestartFolder()
	if filepath.Dir(joined) != filepath.Dir(prev) || joined == prev {
		t.Fatalf("J-6 previous folder unchanged: the joined document is in %q, want a new folder beside %q", joined, prev)
	}
	if b, err := os.ReadFile(filepath.Join(joined, project.MainFile)); err != nil || !strings.Contains(string(b), "le texte de l'autre") {
		t.Errorf("J-6 the joined document is saved in its own folder: %v", err)
	}
}

func TestJoinOutcomeFollowsTheLoadedList(t *testing.T) {
	for _, tc := range []struct {
		name, failAt string
		joins        bool
		notice       string
	}{
		{"failure before the install", stepJoinList, false, "join.failed"},
		{"install done", "", true, ""},
		{"install done with a finalisation error", stepJoinInstalled, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := startRuntime(t, nil, false)
			guest, d := startRuntime(t, nil, false)
			oldID := guest.wiring.list().Project()
			guest.inv.store.step = func(name string) error {
				if name == tc.failAt {
					return errors.New("injected failure at " + name)
				}
				return nil
			}
			holdJoin(t, guest, host)
			next := restartAfterJoin(t, guest, d.home, d.proj)
			pending := next.Session.PendingJoin()
			id, standing, notice := next.wiring.list().Project(), next.Session.Standing(), next.Session.StartupNotice()
			wantID := oldID
			if tc.joins {
				wantID = host.wiring.list().Project()
			}
			if id != wantID || standing != StandingMember || pending.Waiting != tc.joins || notice != tc.notice {
				t.Errorf("J-4 outcome from the loaded list: %s: project %v, standing %q, record %v, notice %q; want project %v, member, record %v, notice %q",
					tc.name, id, standing, pending.Waiting, notice, wantID, tc.joins, tc.notice)
			}
			if tc.joins && (pending.Inviter != host.Session.LocalPeer() || pending.Root != "") {
				t.Errorf("J-1 the record waits for the root of the inviter: %+v", pending)
			}
			if again := next.Session.StartupNotice(); again != "" {
				t.Errorf("J-4 the sentence is said once: a second notice %q", again)
			}
			if tc.joins {
				// The inviter's addresses came with the record: the two link again by themselves.
				waitUntil(t, "the link to the inviter", func() bool { return slices.Contains(next.Node.Peers(), host.Node.Local()) })
			}
		})
	}
}

func TestJoinFailedRecordDropped(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, d := startRuntime(t, nil, false)
	prev, parent := d.proj.Path(), filepath.Dir(d.proj.Path())
	if err := guest.Files.WriteProjection(project.MainFile, []byte("= Mon livre\nmon texte\n")); err != nil {
		t.Fatal(err)
	}
	text, list := treeFiles(t, prev, notInkwell), trustFiles(t, prev)
	guest.inv.store.step = func(name string) error {
		if name == stepJoinInstall { // the staging folder is whole, the rename fails
			return errors.New("injected failure at " + name)
		}
		return nil
	}
	holdJoin(t, guest, host)
	if err := guest.Session.FinishJoin(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := guest.Close(); err == nil {
		t.Error("the failed write of a join is reported by Close")
	}
	if left, marker := stagings(t, parent), markerOf(t, prev); len(left) != 1 || left[0] != marker {
		t.Fatalf("setup: a join that failed before the rename leaves its staging folder and marker: %v, %q", left, marker)
	}
	if guest.RestartFolder() != "" || guest.RestartChange() == "" {
		t.Errorf("J-4 a failed join restarts on the previous folder: restart %v on %q", (guest.RestartChange() != ""), guest.RestartFolder())
	}

	next := startIn(t, d.home, d.proj, true)
	left, marker := stagings(t, parent), markerOf(t, prev)
	diff := changed(text, treeFiles(t, prev, notInkwell))
	if len(left) > 0 || marker != "" || len(diff) > 0 || !equalMaps(list, trustFiles(t, prev)) || len(readDirNames(t, parent)) != 1 {
		t.Errorf("J-4 nothing changed: after the next start: staging %v, marker %q, text changed %v, parent %v", left, marker, diff, readDirNames(t, parent))
	}
	if got := next.Session.StartupNotice(); got != "join.failed" {
		t.Errorf("J-4 nothing changed: the next start says %q, want %q", got, "join.failed")
	}
	if got := next.Session.StartupNotice(); got != "" || next.Session.PendingJoin().Waiting {
		t.Errorf("J-4 nothing changed: said again %q, record %v", got, next.Session.PendingJoin().Waiting)
	}
	// The inviter's list already names this device: a new invitation from that person works.
	holdJoin(t, next, host)
}

// completeCase is a runtime started on a joined folder whose record waits, as the image a join
// restarted finds it.
type completeCase struct {
	*joinCase
	rt     *Runtime
	proj   *project.Project
	folder string
}

func newCompleteCase(t *testing.T, root string) *completeCase {
	t.Helper()
	c := newJoinCase(t, "Roman", root)
	joined, err := c.finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.st.close(); err != nil {
		t.Fatal(err)
	}
	p := openFolder(t, joined)
	return &completeCase{joinCase: c, rt: startIn(t, c.home, p, true), proj: p, folder: joined}
}

// store saves a chunk of the document url in the Go storage, as the storage adapter of the page
// does: kind is "snapshot", "incremental" or "sync-state".
func (c *completeCase) store(t *testing.T, url, kind string) {
	t.Helper()
	if err := c.rt.Files.Save([]string{strings.TrimPrefix(url, "automerge:"), kind, "abc123"}, []byte("chunk")); err != nil {
		t.Fatal(err)
	}
}

func (c *completeCase) manifest(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(c.folder, ".inkwell", "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const joinedRoot = "automerge:joinedRoot1"

func TestCompleteJoinConsumesOnce(t *testing.T) {
	c := newCompleteCase(t, "")
	c.store(t, joinedRoot, "snapshot")
	host := string(c.joined.Host)
	if got := c.rt.Session.PendingJoin(); !got.Waiting || got.Inviter != host {
		t.Fatalf("setup: the joined folder waits for its root: %+v", got)
	}
	if err := c.rt.Session.CompleteJoin(joinedRoot, host); err != nil {
		t.Fatalf("CompleteJoin of a stored document announced by a member = %v", err)
	}
	if m := c.manifest(t); strings.Contains(m, "pendingJoin") || c.proj.Meta().RootURL != joinedRoot || c.rt.Session.PendingJoin().Waiting {
		t.Errorf("J-1 consumed once: after CompleteJoin project.json is %s", m)
	}
	c.store(t, "automerge:otherRoot2", "snapshot")
	if err := c.rt.Session.CompleteJoin("automerge:otherRoot2", host); err == nil || c.proj.Meta().RootURL != joinedRoot {
		t.Errorf("J-1 consumed once: a second CompleteJoin = %v, root %q; want refused, the first root kept", err, c.proj.Meta().RootURL)
	}
	if err := c.rt.Close(); err != nil {
		t.Fatal(err)
	}
	again := startIn(t, c.home, openFolder(t, c.folder), false)
	if again.Session.PendingJoin().Waiting || again.Session.StartupNotice() != "" {
		t.Error("J-1 consumed once: the next start still waits, or says something")
	}
}

func TestCompleteJoinRefuses(t *testing.T) {
	stranger := string(p2p.IDFromKey(newDevice(t).Public()))
	for _, tc := range []struct {
		name, tag string
		call      func(t *testing.T) (*completeCase, error)
	}{
		{"non-member", "J-1 CompleteJoin refuses a non-member", func(t *testing.T) (*completeCase, error) {
			// The record states the root: membership is the one rule on who served it (without a
			// root the inviter rule below refuses a stranger first).
			c := newCompleteCase(t, joinedRoot)
			c.store(t, joinedRoot, "snapshot")
			return c, allRefused(c.rt.Session.CompleteJoin(joinedRoot, stranger), c.rt.Session.CompleteJoin(joinedRoot, "not a peer"))
		}},
		{"invalid URL", "J-1 CompleteJoin refuses an invalid URL", func(t *testing.T) (*completeCase, error) {
			c := newCompleteCase(t, "")
			host := string(c.joined.Host)
			if err := c.rt.Files.Save([]string{"..", "snapshot", "abc123"}, []byte("chunk")); err == nil {
				t.Fatal("setup: the storage took a hostile key")
			}
			// Stored under its id: only the form of the URL can refuse the first one.
			c.store(t, joinedRoot, "snapshot")
			return c, allRefused(c.rt.Session.CompleteJoin(strings.TrimPrefix(joinedRoot, "automerge:"), host), c.rt.Session.CompleteJoin("", host),
				c.rt.Session.CompleteJoin("https://example.org/doc", host), c.rt.Session.CompleteJoin("automerge:../x", host))
		}},
		{"other root", "J-1 CompleteJoin refuses another root", func(t *testing.T) (*completeCase, error) {
			c := newCompleteCase(t, joinedRoot)
			c.store(t, "automerge:otherRoot2", "snapshot")
			c.store(t, joinedRoot, "snapshot")
			return c, c.rt.Session.CompleteJoin("automerge:otherRoot2", string(c.joined.Host))
		}},
		{"not the inviter", "J-1 CompleteJoin refuses a root not announced by the inviter", func(t *testing.T) (*completeCase, error) {
			// The record holds no root: a member other than the inviter (this device is one)
			// cannot bind the project to the document it announces.
			c := newCompleteCase(t, "")
			c.store(t, joinedRoot, "snapshot")
			return c, c.rt.Session.CompleteJoin(joinedRoot, c.rt.Session.LocalPeer())
		}},
		{"unstored document", "J-1 consumed after storage", func(t *testing.T) (*completeCase, error) {
			c := newCompleteCase(t, "")
			return c, c.rt.Session.CompleteJoin(joinedRoot, string(c.joined.Host))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := tc.call(t)
			if m := c.manifest(t); err == nil || !strings.Contains(m, "pendingJoin") || c.proj.Meta().RootURL != "" || !c.rt.Session.PendingJoin().Waiting {
				t.Errorf("%s: CompleteJoin = %v; project.json is %s; want refused, the record kept and no root", tc.tag, err, m)
			}
		})
	}
	t.Run("no record", func(t *testing.T) {
		rt, d := startRuntime(t, nil, false)
		self := rt.Session.LocalPeer()
		if err := rt.Files.Save([]string{"joinedRoot1", "snapshot", "abc123"}, []byte("chunk")); err != nil {
			t.Fatal(err)
		}
		before := d.proj.Meta().RootURL
		// For that reason and no other: a project that waits for no join has no inviter either,
		// and must not be refused only because nobody matches an empty record.
		if err := rt.Session.CompleteJoin(joinedRoot, self); !errors.Is(err, errNoJoinRecord) || d.proj.Meta().RootURL != before {
			t.Errorf("J-1 CompleteJoin refuses without a record: CompleteJoin = %v, root %q (was %q); want %v", err, d.proj.Meta().RootURL, before, errNoJoinRecord)
		}
	})
}

// With a root in the record the rule is the root itself: any member of the joined list may be the
// one that served it.
func TestCompleteJoinStatedRootFromAnyMember(t *testing.T) {
	c := newCompleteCase(t, joinedRoot)
	c.store(t, joinedRoot, "snapshot")
	if err := c.rt.Session.CompleteJoin(joinedRoot, c.rt.Session.LocalPeer()); err != nil || c.proj.Meta().RootURL != joinedRoot {
		t.Errorf("J-1 the stated root from any member: CompleteJoin = %v, root %q", err, c.proj.Meta().RootURL)
	}
}

// allRefused returns the refusals joined when every call was refused, nil when one was accepted.
func allRefused(errs ...error) error {
	if slices.Contains(errs, nil) {
		return nil
	}
	return errors.Join(errs...)
}

func TestCompleteJoinAfterTheDocumentIsStored(t *testing.T) {
	c := newCompleteCase(t, "")
	host := string(c.joined.Host)
	waiting := func(when string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(c.manifest(t), "pendingJoin") || c.proj.Meta().RootURL != "" {
			t.Fatalf("J-1 consumed after storage: %s CompleteJoin = %v, root %q; want refused, the record kept and no root", when, err, c.proj.Meta().RootURL)
		}
	}
	waiting("with no chunk of the document in the Go storage,", c.rt.Session.CompleteJoin(joinedRoot, host))
	c.store(t, "automerge:otherRoot2", "snapshot")
	waiting("with a chunk of another document only,", c.rt.Session.CompleteJoin(joinedRoot, host))
	c.store(t, joinedRoot, "sync-state")
	waiting("with a sync state of the document only,", c.rt.Session.CompleteJoin(joinedRoot, host))
	c.store(t, joinedRoot, "snapshot")
	if err := c.rt.Session.CompleteJoin(joinedRoot, host); err != nil || c.proj.Meta().RootURL != joinedRoot {
		t.Errorf("J-1 consumed after storage: once a chunk is stored CompleteJoin = %v, root %q", err, c.proj.Meta().RootURL)
	}
}

// joinConn plays the page on the store stream.
type joinConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     chan []byte
	sent   chan []byte
}

func (c *joinConn) Send(b []byte) error {
	if err := c.ctx.Err(); err != nil { // as the Wails stream: nothing is sent once closed
		return err
	}
	select {
	case c.sent <- b:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *joinConn) Receive() ([]byte, error) {
	select {
	case b := <-c.in:
		return b, nil
	case <-c.ctx.Done():
		return nil, io.EOF
	}
}

func (c *joinConn) Context() context.Context { return c.ctx }
func (c *joinConn) Close() error             { c.cancel(); return nil }

// saveRequest is the frame of the store stream that saves "w" under the one-component key, and
// storeReply reads a reply frame (their codecs for Go live in the tests of package bridge).
func saveRequest(id uint32, key string) []byte {
	b := binary.BigEndian.AppendUint32(nil, id)
	b = append(b, byte(bridge.OpSave), 1, byte(len(key)))
	return append(append(b, key...), 'w')
}

func storeReply(b []byte) (bridge.StoreResponse, error) {
	if len(b) < 5 {
		return bridge.StoreResponse{}, errors.New("short reply")
	}
	return bridge.StoreResponse{ID: binary.BigEndian.Uint32(b), Status: bridge.StoreStatus(b[4]), Payload: b[5:]}, nil
}

// heldKV is the storage of a runtime whose Save of the key « hold » waits until released.
type heldKV struct {
	bridge.KV
	entered, release chan struct{}
	once             sync.Once
}

func (h *heldKV) Save(key []string, data []byte) error {
	if key[0] == "hold" {
		h.once.Do(func() { close(h.entered) })
		<-h.release
	}
	return h.KV.Save(key, data)
}

// TestStorageWriteBeforeFinishJoinLandsOrIsRefused: a storage write sent on the store stream just
// before FinishJoin, held at each point, has landed in the storage of the previous project, with
// its reply, or was refused with « leaving »; it is never under the joined project (rule N-16 on
// the join path).
func TestStorageWriteBeforeFinishJoinLandsOrIsRefused(t *testing.T) {
	for _, point := range []string{"not taken", "in the handler", "after the reply"} {
		t.Run(point, func(t *testing.T) {
			host, _ := startRuntime(t, nil, false)
			guest, d := startRuntime(t, nil, false)
			kv := &heldKV{KV: guest.Files, entered: make(chan struct{}), release: make(chan struct{})}
			unhold := sync.OnceFunc(func() { close(kv.release) })
			srv := bridge.NewStoreServer(kv, guest.Files, guest.Files, slog.New(slog.DiscardHandler))
			srv.UseDrain(guest.Bridge)
			ctx, cancel := context.WithCancel(t.Context())
			conn := &joinConn{ctx: ctx, cancel: cancel, in: make(chan []byte, 16), sent: make(chan []byte, 16)}
			served := make(chan struct{})
			go func() { srv.Serve(conn); close(served) }()
			t.Cleanup(func() { unhold(); cancel(); <-served })
			id := uint32(0)
			send := func(key string) uint32 {
				id++
				conn.in <- saveRequest(id, key)
				return id
			}
			holdJoin(t, guest, host)

			key := "write"
			var write uint32
			switch point {
			case "not taken": // the handler is busy with « hold »; « write » waits in the stream
				send("hold")
				<-kv.entered
				write = send("write")
			case "in the handler":
				key = "hold"
				write = send("hold")
				<-kv.entered
			case "after the reply":
				unhold()
				write = send("write")
				if r, err := storeReply(<-conn.sent); err != nil || r.ID != write || r.Status != bridge.StatusOK {
					t.Fatalf("the reply before the request: %+v, %v", r, err)
				}
			}
			finished := make(chan error, 1)
			go func() { finished <- guest.Session.FinishJoin(t.Context()) }()
			if point != "after the reply" {
				select {
				case err := <-finished:
					t.Fatalf("N-16 no write before the drain: FinishJoin returned (%v) with a handler in flight", err)
				case <-time.After(50 * time.Millisecond):
				}
				unhold()
			}
			if err := <-finished; err != nil {
				t.Fatalf("FinishJoin = %v", err)
			}
			replies := map[uint32]bridge.StoreResponse{}
			if point == "after the reply" {
				replies[write] = bridge.StoreResponse{ID: write, Status: bridge.StatusOK}
			}
			for len(conn.sent) > 0 {
				r, err := storeReply(<-conn.sent)
				if err != nil {
					t.Fatal(err)
				}
				replies[r.ID] = r
			}
			_ = guest.Close()
			joined := guest.RestartFolder()
			if joined == "" {
				t.Fatal("the join installed no folder")
			}
			inRepo := func(p *project.Project) bool {
				_, err := p.Load([]string{key})
				return err == nil
			}
			landed := inRepo(d.proj)
			r, answered := replies[write]
			switch {
			case inRepo(openFolder(t, joined)):
				t.Errorf("N-16 no lost write: the write %q is under the joined project", key)
			case landed && (!answered || r.Status != bridge.StatusOK):
				t.Errorf("N-16 no lost write: %q landed without its reply (%+v, %v)", key, r, answered)
			case !landed && answered && (r.Status != bridge.StatusError || string(r.Payload) != bridge.ReasonLeaving):
				t.Errorf("N-16 no lost write: %q is absent from the storage and was answered %+v, want refused %q", key, r, bridge.ReasonLeaving)
			case !landed && !answered && ctx.Err() == nil:
				t.Errorf("N-16 no lost write: %q is absent from the storage, unanswered, and its stream is still open", key)
			}
			// Not landed, not answered, stream ended: the drain closed the stream before it read the
			// write or before its refusal was out; the page's adapter fails every pending write at the
			// end-of-stream and reports it as refused (onLeaving, Vitest « RT1 A-2 a write cut by a
			// stream close is never sent again »), as TestStorageWriteBeforeRequestLandsOrIsRefused.
			if point == "not taken" && landed {
				t.Errorf("N-16 a write not taken before FinishJoin is refused: %q landed", key)
			}
		})
	}
}

// TestJoinNamesTheFolderCreated: the person reads, before anything restarts, the name of the
// folder the joined document gets (rule J-2): the first free one when the Welcome is held, and
// the install makes that one. When the name was taken meanwhile, the install takes the next and
// the image that waits says which.
func TestJoinNamesTheFolderCreated(t *testing.T) {
	for _, tc := range []struct{ name, taken, want, told string }{
		{"the name announced is the one made", "", "Mon livre (2)", ""},
		{"the name was taken meanwhile", "Mon livre (2)", "Mon livre (3)", "Mon livre (3)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := startRuntime(t, nil, false)
			guest, d := startRuntime(t, nil, false)
			res := holdJoin(t, guest, host)
			if res.Folder != "Mon livre (2)" {
				t.Fatalf("J-2 the folder announced is the first free name: %q, want %q", res.Folder, "Mon livre (2)")
			}
			if tc.taken != "" {
				if err := os.Mkdir(filepath.Join(filepath.Dir(d.proj.Path()), tc.taken), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			next := restartAfterJoin(t, guest, d.home, d.proj)
			if got := filepath.Base(guest.RestartFolder()); got != tc.want {
				t.Errorf("J-2 the folder made: %q, want %q", got, tc.want)
			}
			if got := next.Session.PendingJoin().Folder; got != tc.told {
				t.Errorf("J-2 the image that waits names the folder only when it is not the one announced: %q, want %q", got, tc.told)
			}
		})
	}
}

// TestReturnToPreviousSurvivesARelaunch: while a join waits, the person returns to the document
// that was open before (rule J-2): Inkwell restarts on that folder through the change of the
// image, says nothing there, and the next launch opens it too; the join still waits in its own
// folder.
func TestReturnToPreviousSurvivesARelaunch(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, d := startRuntime(t, nil, false)
	prev, parent := d.proj.Path(), filepath.Dir(d.proj.Path())
	if err := guest.Files.WriteProjection(project.MainFile, []byte("= Mon livre\nmon texte\n")); err != nil {
		t.Fatal(err)
	}
	text := treeFiles(t, prev, notInkwell)
	waiting := joinedRuntime(t, guest, d, host)
	joined := waiting.inv.folder
	quits := make(chan struct{}, 1)
	waiting.SetQuit(func() { quits <- struct{}{} })
	if p := waiting.Session.PendingJoin(); !p.Waiting || !p.CanReturn {
		t.Fatalf("J-2 a waiting join offers the return to the previous document: %+v", p)
	}
	if err := waiting.Session.ReturnToPrevious(t.Context()); err != nil {
		t.Fatalf("J-2 ReturnToPrevious = %v", err)
	}
	<-quits
	if _, err := waiting.Session.Invite("writer"); !errors.Is(err, errLeaving) {
		t.Errorf("N-13 one change per image: Invite after the return = %v, want refused", err)
	}
	if got := waiting.RestartFolder(); got != "" {
		t.Errorf("N-13 nothing is decided before the shutdown: the restart folder is %q", got)
	}
	if err := waiting.Close(); err != nil {
		t.Fatal(err)
	}
	if waiting.RestartChange() == "" || waiting.RestartFolder() != prev {
		t.Fatalf("J-2 the return restarts on the previous folder: restart %v on %q, want %q", (waiting.RestartChange() != ""), waiting.RestartFolder(), prev)
	}
	base, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	if latest, err := project.Latest(base); err != nil || latest != filepath.Base(prev) {
		t.Errorf("J-2 a return to the previous document survives a relaunch: the next launch opens %q (%v), want %q", latest, err, filepath.Base(prev))
	}
	if rec, ok := openFolder(t, joined).PendingJoin(); !ok || rec.Previous != filepath.Base(prev) {
		t.Errorf("J-2 the join still waits in its folder after the return: record %+v, %v", rec, ok)
	}
	if diff := changed(text, treeFiles(t, prev, notInkwell)); len(diff) > 0 {
		t.Errorf("J-6 previous folder unchanged: after the return: %v", diff)
	}
	back := startIn(t, d.home, openFolder(t, prev), true)
	if p, notice := back.Session.PendingJoin(), back.Session.StartupNotice(); p.Waiting || p.CanReturn || notice != "" {
		t.Errorf("J-2 back in the previous document nothing waits and nothing is said: pending %+v, notice %q", p, notice)
	}
	if err := back.Close(); err != nil {
		t.Fatal(err)
	}
	// A later start of that folder, after a take-over for instance, says its sentence again.
	if again := startIn(t, d.home, openFolder(t, prev), true); again.Session.StartupNotice() == "" {
		t.Error("J-2 the silence of a return holds for one start: the next change says nothing either")
	}
}

// TestReturnAfterTheJoin: once the join completed, the way back to the document open before it
// stays (rule J-9): offered at every start of the joined folder, it restarts on the previous
// folder; once that folder is gone it is neither offered nor accepted.
func TestReturnAfterTheJoin(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	guest, d := startRuntime(t, nil, false)
	prev := d.proj.Path()
	waiting := joinedRuntime(t, guest, d, host)
	joined := waiting.inv.folder
	if err := waiting.Files.Save([]string{strings.TrimPrefix(joinedRoot, "automerge:"), "snapshot", "abc123"}, []byte("chunk")); err != nil {
		t.Fatal(err)
	}
	if err := waiting.Session.CompleteJoin(joinedRoot, host.Session.LocalPeer()); err != nil {
		t.Fatalf("CompleteJoin = %v", err)
	}
	if p := waiting.Session.PendingJoin(); p.Waiting || !p.CanReturn {
		t.Errorf("J-9 the way back after a join: once the join completed PendingJoin = %+v, want the return offered", p)
	}
	if err := waiting.Close(); err != nil {
		t.Fatal(err)
	}
	again := startIn(t, d.home, openFolder(t, joined), true)
	if p := again.Session.PendingJoin(); p.Waiting || !p.CanReturn {
		t.Errorf("J-9 the way back after a join survives a relaunch: PendingJoin = %+v", p)
	}
	quits := make(chan struct{}, 1)
	again.SetQuit(func() { quits <- struct{}{} })
	if err := again.Session.ReturnToPrevious(t.Context()); err != nil {
		t.Fatalf("J-9 ReturnToPrevious after the join = %v", err)
	}
	<-quits
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
	if again.RestartChange() == "" || again.RestartFolder() != prev {
		t.Errorf("J-9 the return restarts on the previous folder: restart %v on %q, want %q", (again.RestartChange() != ""), again.RestartFolder(), prev)
	}
	back := startIn(t, d.home, openFolder(t, prev), true)
	if p := back.Session.PendingJoin(); p.CanReturn {
		t.Errorf("J-9 back in the previous folder no way back is offered: %+v", p)
	}
	if err := back.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.proj.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(prev); err != nil {
		t.Fatal(err)
	}
	gone := startIn(t, d.home, openFolder(t, joined), true)
	if p := gone.Session.PendingJoin(); p.CanReturn {
		t.Errorf("J-9 no return is offered once the previous folder is gone: %+v", p)
	}
	if err := gone.Session.ReturnToPrevious(t.Context()); !errors.Is(err, errNoPrevious) {
		t.Errorf("J-9 return refused once the previous folder is gone: ReturnToPrevious = %v", err)
	}
}

// TestReturnToPreviousRefuses: without a join that waits, or once the previous folder is gone,
// nothing restarts and the session stays as it was (rule J-2).
func TestReturnToPreviousRefuses(t *testing.T) {
	t.Run("no join waits", func(t *testing.T) {
		rt, _ := startRuntime(t, nil, false)
		if err := rt.Session.ReturnToPrevious(t.Context()); !errors.Is(err, errNoPrevious) {
			t.Errorf("J-2 return refused without a previous document: with no join waiting ReturnToPrevious = %v", err)
		}
		if _, err := rt.Session.Invite("writer"); err != nil {
			t.Errorf("J-2 a refused return leaves the session as it was: Invite = %v", err)
		}
	})
	t.Run("the previous folder is gone", func(t *testing.T) {
		host, _ := startRuntime(t, nil, false)
		guest, d := startRuntime(t, nil, false)
		prev := d.proj.Path()
		waiting := joinedRuntime(t, guest, d, host)
		if err := d.proj.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(prev); err != nil {
			t.Fatal(err)
		}
		if p := waiting.Session.PendingJoin(); !p.Waiting || p.CanReturn {
			t.Errorf("J-2 no return is offered once the previous folder is gone: %+v", p)
		}
		if err := waiting.Session.ReturnToPrevious(t.Context()); !errors.Is(err, errNoPrevious) {
			t.Errorf("J-2 return refused without a previous document: with the folder gone ReturnToPrevious = %v", err)
		}
		if err := waiting.Close(); err != nil {
			t.Fatal(err)
		}
		if waiting.RestartChange() != "" {
			t.Error("J-2 a refused return restarts nothing")
		}
	})
}
