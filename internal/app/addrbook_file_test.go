package app

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// openTestBook opens the address book of a new home directory.
func openTestBook(t *testing.T) (*addrBookFile, *os.Root) {
	t.Helper()
	home, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	f, err := openAddrBook(home, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f, home
}

// TestAddrBookFileMode: plan §3/§5, the book is private to the user (0600), and a book made
// readable by others is made private again when loaded.
func TestAddrBookFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix modes")
	}
	f, home := openTestBook(t)
	if data, err := f.Load(); data != nil || err != nil {
		t.Fatalf("Load() of a new home = %q, %v; want nothing", data, err)
	}
	if err := f.Save([]byte(`{"v":1,"peers":[]}`)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	mode := func() fs.FileMode {
		t.Helper()
		info, err := home.Stat(addrBookName)
		if err != nil {
			t.Fatal(err)
		}
		return info.Mode().Perm()
	}
	if got := mode(); got != 0o600 {
		t.Errorf("mode after Save = %v, want 0600", got)
	}
	if err := home.Chmod(addrBookName, 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := f.Load(); string(data) != `{"v":1,"peers":[]}` || err != nil {
		t.Errorf("Load() = %q, %v; want the saved book", data, err)
	}
	if got := mode(); got != 0o600 {
		t.Errorf("mode after loading a 0644 book = %v, want 0600", got)
	}
}

// TestAddrBookFileAtomic: plan §3, a reader running during 2000 saves sees the previous book or
// the new one, never a partial file.
func TestAddrBookFileAtomic(t *testing.T) {
	f, home := openTestBook(t)
	books := [][]byte{bytes.Repeat([]byte("a"), 64<<10), bytes.Repeat([]byte("b"), 32<<10)}
	var (
		done atomic.Bool
		torn atomic.Int64
		wg   sync.WaitGroup
	)
	wg.Go(func() {
		for !done.Load() {
			data, err := home.ReadFile(addrBookName)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil || (!bytes.Equal(data, books[0]) && !bytes.Equal(data, books[1])) {
				torn.Add(1)
			}
		}
	})
	for i := range 2000 {
		if err := f.Save(books[i%2]); err != nil {
			done.Store(true)
			wg.Wait()
			t.Fatalf("Save #%d: %v", i, err)
		}
	}
	done.Store(true)
	wg.Wait()
	if n := torn.Load(); n != 0 {
		t.Errorf("a concurrent reader saw %d partial books during 2000 saves", n)
	}
}

// TestAddrBookCorruptQuarantined: plan §3, a book the node refuses is set aside as .bad and the
// device starts with an empty book.
func TestAddrBookCorruptQuarantined(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/"+addrBookName, []byte("not a book"), 0o600); err != nil {
		t.Fatal(err)
	}
	home, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	proj, _ := newProjectFolder(t)
	rt, err := Start(t.Context(), home, Options{Project: proj, RendezvousURL: NoCodes})
	if err != nil {
		t.Fatalf("Start with a corrupt address book = %v, want it to start", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := home.ReadFile(addrBookName + ".bad"); string(got) != "not a book" || err != nil {
		t.Errorf("quarantined book = %q, %v; want the refused file", got, err)
	}
	if _, err := home.Stat(addrBookName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the refused book is still in place (%v), want it set aside", err)
	}
}

// TestLANProbeKeepsMember: plan §1 (second defect), the LAN probe hangs up on a device that does
// not answer instead of dropping it, which would end a member's reconnection. Architecture test
// on the call: Node.Hangup keeps the peer wanted (p2p TestHangupKeepsWant proves it).
func TestLANProbeKeepsMember(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "invites.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]int{}
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "nearby" || fn.Recv == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					calls[fun.Sel.Name]++
				case *ast.Ident:
					calls[fun.Name]++
				}
			}
			return true
		})
	}
	if calls["Drop"] != 0 || calls["Hangup"] != 1 || calls["probeTargets"] != 1 {
		t.Errorf("invites.nearby calls Drop %d, Hangup %d, probeTargets %d times; want 0, 1, 1", calls["Drop"], calls["Hangup"], calls["probeTargets"])
	}
	// Security review 7: a linked peer is not greeted, so a failed greeting cannot hang up its link.
	found := []p2p.Nearby{{ID: "linked"}, {ID: "stranger"}}
	if got := probeTargets(found, []transport.PeerID{"linked"}); len(got) != 1 || got[0].ID != "stranger" {
		t.Errorf("probeTargets = %v, want the stranger only", got)
	}
}

// TestRestartReconnectsMembers: plan §1, two real runtimes on the loopback, no relay: once a
// member restarts on the same home, the two see each other again within R, with nothing to do.
func TestRestartReconnectsMembers(t *testing.T) {
	a, _ := startRuntime(t, nil, false)
	b, bDirs := startRuntime(t, nil, false)
	inv, err := a.Session.Invite("writer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Session.Join(t.Context(), inv.Text); err != nil {
		t.Fatalf("Join() = %v", err)
	}
	expectRepoUp(t, a, b.Node.Local())
	expectRepoUp(t, b, a.Node.Local())

	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Start(t.Context(), bDirs.home, Options{Project: bDirs.proj, RendezvousURL: NoCodes})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Close() })
	start := time.Now()
	expectRepoUp(t, again, a.Node.Local())
	expectRepoUp(t, a, again.Node.Local())
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("members linked again %v after the restart, want at most 15 s (R)", d)
	}
}
