package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// securityLog keeps the number of security events logged.
type securityLog struct {
	mu sync.Mutex
	n  int
}

func (l *securityLog) Enabled(context.Context, slog.Level) bool { return true }
func (l *securityLog) Handle(_ context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "event" && a.Value.String() == "security" {
			l.mu.Lock()
			l.n++
			l.mu.Unlock()
		}
		return true
	})
	return nil
}
func (l *securityLog) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *securityLog) WithGroup(string) slog.Handler      { return l }
func (l *securityLog) events() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.n
}

// filesPeer is one device with a project folder, serving and fetching blobs.
type filesPeer struct {
	files *Files
	proj  *project.Project
	base  *os.Root
	log   *securityLog
	id    transport.PeerID
}

// newFilesPeer starts a device named id on the bulk lane of net.
func newFilesPeer(t *testing.T, net *memnet.Network, id transport.PeerID, o FilesOptions) *filesPeer {
	t.Helper()
	base, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close() })
	proj, err := project.Create(base, "p", "automerge:root")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proj.Close() })
	p := &filesPeer{proj: proj, base: base, log: new(securityLog), id: id}
	if o.Allowed == nil {
		o.Allowed = func(transport.PeerID) bool { return true }
	}
	o.Log = slog.New(p.log)
	p.files, err = NewFiles(proj, net.JoinBulk(id), o)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() { _ = p.files.Run(ctx) })
	t.Cleanup(func() {
		cancel()
		wg.Wait()
		_ = p.files.Close()
	})
	return p
}

func data(n int, seed uint64) []byte {
	r := rand.New(rand.NewPCG(seed, seed)) // #nosec G404 -- test data.
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Uint32())
	}
	return b
}

// addBlob writes content at rel in the project folder of p, as a user would,
// and has the project store it: it returns the entry the tree lists.
func (p *filesPeer) addBlob(t *testing.T, rel string, content []byte) project.TreeEntry {
	t.Helper()
	if i := strings.LastIndex(rel, "/"); i > 0 {
		if err := p.base.MkdirAll(p.proj.Dir()+"/"+rel[:i], 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.base.WriteFile(p.proj.Dir()+"/"+rel, content, 0o600); err != nil {
		t.Fatal(err)
	}
	h, size, err := p.files.PutBlob(rel)
	if err != nil || size != int64(len(content)) || h != sha256Blake(content) {
		t.Fatalf("PutBlob(%q) = %x, %d, %v", rel, h, size, err)
	}
	return project.TreeEntry{Path: rel, Type: project.NodeBlob, Hash: h, Size: size}
}

func sha256Blake(b []byte) [32]byte { return blob.HashOf(b) }

func hashes(tree []project.TreeEntry) [][bridge.HashSize]byte {
	var out [][bridge.HashSize]byte
	for _, e := range tree {
		if e.Type == project.NodeBlob {
			out = append(out, e.Hash)
		}
	}
	return out
}

// waitStored waits until every blob of the tree is stored at p, in virtual time.
func waitStored(t *testing.T, p *filesPeer, want int) {
	t.Helper()
	for i := 0; i < 100_000; i++ {
		synctest.Wait()
		if st := p.files.Status(); st.Stored == want {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("%s: %+v after waiting", p.id, p.files.Status())
}

func (p *filesPeer) read(t *testing.T, rel string) ([]byte, error) {
	t.Helper()
	return p.base.ReadFile(p.proj.Dir() + "/" + rel)
}

// PRJ-01 (tree and hashes), PRJ-04: a joiner gets every file of a project of 40
// files, with the same tree and the same hashes, and nothing shows in the
// folder before it is verified; a file added later reaches it too.
func TestJoinerReceivesTheWholeTree(t *testing.T) {
	t.Run("PRJ-01 PRJ-04 the joiner gets the whole tree", runJoinerReceivesTheWholeTree)
}

func runJoinerReceivesTheWholeTree(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(7, memnet.Faults{MaxDelay: 30 * time.Millisecond})
		a := newFilesPeer(t, net, "a", FilesOptions{})
		b := newFilesPeer(t, net, "b", FilesOptions{})
		net.Link("a", "b")

		var tree []project.TreeEntry
		want := map[string][]byte{}
		tree = append(tree, project.TreeEntry{Path: "main.typ", Type: project.NodeText, URL: "automerge:main", Main: true})
		for i := range 12 { // texts
			e := project.TreeEntry{Path: fmt.Sprintf("chapters/ch%02d.typ", i), Type: project.NodeText, URL: fmt.Sprintf("automerge:t%d", i)}
			tree = append(tree, e)
		}
		sizes := []int{1, 100, 4096, 16384, 16385, 50_000, 300_000, 1 << 20, 1<<20 + 1, 3 << 20}
		for i := range 27 { // images, PDFs, CSVs, fonts...
			rel := fmt.Sprintf("assets/f%02d.bin", i)
			if i%3 == 0 {
				rel = fmt.Sprintf("assets/sub dir/\u00e9l\u00e9ment %02d.png", i)
			}
			content := data(sizes[i%len(sizes)], uint64(i))
			tree = append(tree, a.addBlob(t, rel, content))
			want[rel] = content
		}
		if len(tree) != 40 {
			t.Fatalf("test tree of %d entries, want 40", len(tree))
		}
		if _, err := a.files.SetTree(tree); err != nil {
			t.Fatal(err)
		}
		if err := a.files.SetReferenced(hashes(tree)); err != nil {
			t.Fatal(err)
		}

		// The joiner is told the tree and what is referenced, by its webview.
		if n, err := b.files.SetTree(tree); err != nil || n != 0 {
			t.Fatalf("joiner SetTree = %d, %v", n, err)
		}
		if err := b.files.SetReferenced(hashes(tree)); err != nil {
			t.Fatal(err)
		}

		// While the files arrive, whatever is in the folder is complete.
		var last int
		for step := 0; ; step++ {
			synctest.Wait()
			for rel, content := range want {
				got, err := b.read(t, rel)
				if err == nil && !bytes.Equal(got, content) {
					t.Fatalf("%s is visible with %d bytes before it was verified (want %d)", rel, len(got), len(content))
				}
			}
			st := b.files.Status()
			if st.Stored < last {
				t.Errorf("progress went back from %d to %d", last, st.Stored)
			}
			last = st.Stored
			if st.Stored == st.Files {
				break
			}
			if step > 100_000 {
				t.Fatalf("not done: %+v", st)
			}
			time.Sleep(5 * time.Millisecond)
		}
		synctest.Wait()

		for rel, content := range want {
			if got, err := b.read(t, rel); err != nil || !bytes.Equal(got, content) {
				t.Errorf("%s at the joiner = %d bytes, %v; want the %d bytes of the original", rel, len(got), err, len(content))
			}
		}
		if st := b.files.Status(); st.Files != 27 || st.Stored != 27 || st.Bytes != st.StoredBytes || st.NeedsConfirmation {
			t.Errorf("Status = %+v, want 27 of 27 files", st)
		}
		// Text files are projected from the webview; the folders and the files
		// are there for it to write to.
		for _, e := range tree {
			if e.Type == project.NodeText {
				if err := b.files.WriteProjection(e.Path, []byte("= "+e.Path+"\n")); err != nil {
					t.Errorf("joiner WriteProjection(%q): %v", e.Path, err)
				}
			}
		}
		if got := b.log.events(); got != 0 {
			t.Errorf("%d security events during an honest transfer", got)
		}

		// PRJ-04: a file added after the arrival reaches the joiner.
		later := a.addBlob(t, "assets/late.pdf", data(70_000, 99))
		tree = append(tree, later)
		_, _ = a.files.SetTree(tree)
		_ = a.files.SetReferenced(hashes(tree))
		_, _ = b.files.SetTree(tree)
		_ = b.files.SetReferenced(hashes(tree))
		waitStored(t, b, 28)
		if got, err := b.read(t, "assets/late.pdf"); err != nil || !bytes.Equal(got, data(70_000, 99)) {
			t.Errorf("late file at the joiner = %d bytes, %v", len(got), err)
		}
	})
}

// A blob is fetched from whoever has it, even a peer that joined the network
// after the tree was known.
func TestFetchWaitsForAPeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		a := newFilesPeer(t, net, "a", FilesOptions{})
		b := newFilesPeer(t, net, "b", FilesOptions{})
		e := a.addBlob(t, "x.bin", data(100_000, 1))
		tree := []project.TreeEntry{e}
		_ = a.files.SetReferenced(hashes(tree))
		_, _ = b.files.SetTree(tree)
		synctest.Wait()
		if st := b.files.Status(); st.Stored != 0 {
			t.Fatalf("stored %d without any peer", st.Stored)
		}
		if got := len(b.files.pending()); got != 1 {
			t.Fatalf("pending = %d, want 1", got)
		}
		net.Link("a", "b") // the peer arrives: no need to wait for the retry timer
		synctest.Wait()
		if st := b.files.Status(); st.Stored != 1 {
			t.Errorf("Status = %+v after a peer arrived, want the file stored", st)
		}
	})
}

// PRJ-11: over the quota the arrival waits for the user; a file over the size
// limit is refused and named.
func TestQuotasAtArrival(t *testing.T) { t.Run("PRJ-11 quotas at arrival", runQuotasAtArrival) }

func runQuotasAtArrival(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		a := newFilesPeer(t, net, "a", FilesOptions{})
		b := newFilesPeer(t, net, "b", FilesOptions{Blob: blob.Options{MaxProject: 3 << 20}})
		net.Link("a", "b")
		var tree []project.TreeEntry
		for i := range 5 {
			tree = append(tree, a.addBlob(t, fmt.Sprintf("big%d.bin", i), data(1<<20, uint64(i))))
		}
		_ = a.files.SetReferenced(hashes(tree))
		// A file the index says is over 100 MB: refused, whoever has it.
		huge := project.TreeEntry{Path: "huge.mov", Type: project.NodeBlob, Hash: sha256.Sum256([]byte("huge")), Size: 101 << 20}
		tree = append(tree, huge)
		_, _ = b.files.SetTree(tree)
		_ = b.files.SetReferenced(hashes(tree))
		synctest.Wait()

		st := b.files.Status()
		if !st.NeedsConfirmation || st.Stored == 0 || st.Stored >= 5 {
			t.Fatalf("Status = %+v, want the arrival paused part way over the 3 MiB quota", st)
		}
		if !slices.Equal(st.Refused, []string{"huge.mov"}) {
			t.Errorf("Refused = %q, want [huge.mov]", st.Refused)
		}
		if st.Limit != 3<<20 {
			t.Errorf("Limit = %d, want 3 MiB", st.Limit)
		}
		stored := st.Stored
		time.Sleep(time.Minute)
		synctest.Wait()
		// A download that was already waiting may use what is left under the quota,
		// but the project never goes over it until the user agrees.
		if got := b.files.Blobs().Used(); got > 3<<20 {
			t.Errorf("the project holds %d bytes, over its quota of 3 MiB, without the user's confirmation", got)
		}
		if st := b.files.Status(); st.Stored < stored || st.Stored >= 5 || !st.NeedsConfirmation {
			t.Errorf("Status = %+v after a minute, want still waiting for the user", st)
		}

		b.files.Confirm()
		synctest.Wait()
		if st := b.files.Status(); st.Stored != 5 || st.NeedsConfirmation {
			t.Errorf("Status = %+v after Confirm, want all 5 files", st)
		}
	})
}

func TestPutBlobLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		a := newFilesPeer(t, net, "a", FilesOptions{Blob: blob.Options{MaxFile: 1 << 20}})
		if err := a.base.WriteFile(a.proj.Dir()+"/big.bin", make([]byte, 2<<20), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := a.files.PutBlob("big.bin")
		tl, ok := errors.AsType[*bridge.TooLargeError](err)
		if !ok || tl.Limit != 1<<20 {
			t.Errorf("PutBlob of 2 MiB with a 1 MiB limit = %v, want a TooLargeError of 1 MiB", err)
		}
		for _, bad := range []string{"../x", "missing.bin", ".inkwell/project.json"} {
			if _, _, err := a.files.PutBlob(bad); err == nil {
				t.Errorf("PutBlob(%q) succeeded", bad)
			}
		}
	})
}

// PRJ-13: an image replaced on disk is a new blob, offered under a new hash and
// propagated; the old blob stays while something refers to it.
func TestReplacedImagePropagates(t *testing.T) {
	t.Run("PRJ-13 a replaced image is a new blob", runReplacedImagePropagates)
}

func runReplacedImagePropagates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		a := newFilesPeer(t, net, "a", FilesOptions{})
		b := newFilesPeer(t, net, "b", FilesOptions{})
		net.Link("a", "b")
		v1 := a.addBlob(t, "img/logo.png", data(60_000, 1))
		_, _ = a.files.SetTree([]project.TreeEntry{v1})
		_ = a.files.SetReferenced(hashes([]project.TreeEntry{v1}))
		_, _ = b.files.SetTree([]project.TreeEntry{v1})
		waitStored(t, b, 1)

		// The user replaces the image outside Inkwell; A's watcher would tell the
		// webview, which asks Go to store the file.
		replacement := data(61_000, 2)
		if err := a.base.WriteFile(a.proj.Dir()+"/img/logo.png", replacement, 0o600); err != nil {
			t.Fatal(err)
		}
		v2h, v2size, err := a.files.PutBlob("img/logo.png")
		if err != nil || v2h == v1.Hash || v2size != int64(len(replacement)) {
			t.Fatalf("PutBlob of the replacement = %x, %d, %v; want a new hash", v2h, v2size, err)
		}
		v2 := project.TreeEntry{Path: "img/logo.png", Type: project.NodeBlob, Hash: v2h, Size: v2size}
		_, _ = a.files.SetTree([]project.TreeEntry{v2})
		_ = a.files.SetReferenced([][bridge.HashSize]byte{v1.Hash, v2.Hash}) // the old version is in the history
		_, _ = b.files.SetTree([]project.TreeEntry{v2})
		_ = b.files.SetReferenced([][bridge.HashSize]byte{v1.Hash, v2.Hash})
		synctest.Wait()

		if got, err := b.read(t, "img/logo.png"); err != nil || !bytes.Equal(got, replacement) {
			t.Errorf("replaced image at B = %d bytes, %v; want the replacement", len(got), err)
		}
		if !b.files.Blobs().Has(blob.Hash(v1.Hash)) || !b.files.Blobs().Has(blob.Hash(v2.Hash)) {
			t.Error("a version of the image is missing from B's store")
		}
	})
}

// PRJ-10: a tree from a peer with hostile names is refused name by name, with a
// security event, and the honest part is applied.
func TestHostileTreeFromAPeer(t *testing.T) {
	t.Run("PRJ-10 hostile tree from a peer", runHostileTreeFromAPeer)
}

func runHostileTreeFromAPeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		b := newFilesPeer(t, net, "b", FilesOptions{})
		tree := []project.TreeEntry{
			{Path: "ok.typ", Type: project.NodeText, URL: "automerge:ok"},
			{Path: "../outside.typ", Type: project.NodeText, URL: "automerge:x"},
			{Path: "dir/NUL", Type: project.NodeBlob, Hash: sha256.Sum256([]byte("x")), Size: 1},
			{Path: "C:\\boot.ini", Type: project.NodeText, URL: "automerge:y"},
			{Path: strings.Repeat("n", 300), Type: project.NodeText, URL: "automerge:z"},
		}
		n, err := b.files.SetTree(tree)
		if err != nil || n != 4 {
			t.Fatalf("SetTree = %d refused, %v; want 4", n, err)
		}
		if got := b.log.events(); got != 4 {
			t.Errorf("%d security events, want one per refused entry", got)
		}
		if err := b.files.WriteProjection("ok.typ", []byte("= ok\n")); err != nil {
			t.Errorf("the honest entry was not applied: %v", err)
		}
		if st := b.files.Status(); st.Files != 0 {
			t.Errorf("Status = %+v: a refused blob is wanted", st)
		}
		if entries, _ := os.ReadDir(b.base.Name()); len(entries) != 1 {
			t.Errorf("the documents folder holds %d entries, want only the project", len(entries))
		}
	})
}

// A device that is not allowed is neither served nor asked.
func TestAllowedGatesServing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		var suspended sync.Map
		a := newFilesPeer(t, net, "a", FilesOptions{
			Allowed: func(id transport.PeerID) bool { _, no := suspended.Load(id); return !no },
			Timeout: time.Second,
		})
		b := newFilesPeer(t, net, "b", FilesOptions{Timeout: time.Second})
		net.Link("a", "b")
		e := a.addBlob(t, "x.bin", data(40_000, 5))
		_ = a.files.SetReferenced(hashes([]project.TreeEntry{e}))
		suspended.Store(transport.PeerID("b"), true)
		_, _ = b.files.SetTree([]project.TreeEntry{e})
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		if st := b.files.Status(); st.Stored != 0 {
			t.Fatalf("a suspended member was served: %+v", st)
		}
		suspended.Delete(transport.PeerID("b"))
		time.Sleep(retryNoPeer + retryNoPeer)
		synctest.Wait()
		if st := b.files.Status(); st.Stored != 1 {
			t.Errorf("Status = %+v after the suspension ended, want the file", st)
		}
	})
}

// An evil peer's forgeries do not reach the folder; the file comes from the honest peer.
func TestForgedBlobNeverVisible(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		good := newFilesPeer(t, net, "good", FilesOptions{})
		b := newFilesPeer(t, net, "b", FilesOptions{})
		content := data(2<<20, 6)
		e := good.addBlob(t, "doc.pdf", content)
		_ = good.files.SetReferenced(hashes([]project.TreeEntry{e}))

		// "evil" answers every request with garbage that looks like the right slice.
		evil := net.JoinBulk("evil")
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		wg.Go(func() {
			for {
				ev, err := evil.Recv(ctx)
				if err != nil {
					return
				}
				m, _ := blob.Decode(ev.Payload)
				if w, ok := m.(blob.Want); ok {
					s, err := good.files.Blobs().ReadSlice(blob.Hash(e.Hash), w.Offset, w.Length)
					if err == nil {
						s.Bao[len(s.Bao)-1] ^= 1
						if out, err := blob.Encode(s); err == nil {
							_ = evil.Send(ctx, ev.Peer, out)
						}
					}
				}
			}
		})
		defer func() {
			cancel()
			wg.Wait()
		}()
		net.Link("evil", "b")
		net.Link("good", "b")
		_, _ = b.files.SetTree([]project.TreeEntry{e})
		waitStored(t, b, 1)
		if got, err := b.read(t, "doc.pdf"); err != nil || !bytes.Equal(got, content) {
			t.Errorf("doc.pdf at B = %d bytes, %v", len(got), err)
		}
		if b.log.events() == 0 {
			t.Error("no security event for a forged slice")
		}
	})
}

// The unreferenced blobs go after 30 days, but only once the webview has said
// what is referenced.
func TestGarbageCollectionWaitsForReferences(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		clock := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
		var mu sync.Mutex
		now := func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
		advance := func(d time.Duration) { mu.Lock(); clock = clock.Add(d); mu.Unlock() }
		a := newFilesPeer(t, net, "a", FilesOptions{Blob: blob.Options{Now: now}})
		keep := a.addBlob(t, "keep.bin", data(5000, 1))
		drop := a.addBlob(t, "drop.bin", data(6000, 2))

		for range 3 { // nothing is known to be referenced yet: nothing is collected
			advance(40 * 24 * time.Hour)
			a.files.poke()
			time.Sleep(time.Second)
			synctest.Wait()
		}
		if !a.files.Blobs().Has(blob.Hash(drop.Hash)) {
			t.Fatal("a blob was collected before the references were known")
		}
		_ = a.files.SetReferenced([][bridge.HashSize]byte{keep.Hash})
		synctest.Wait() // first pass: marks drop
		advance(31 * 24 * time.Hour)
		a.files.poke()
		synctest.Wait() // second pass: 31 days later
		if a.files.Blobs().Has(blob.Hash(drop.Hash)) || !a.files.Blobs().Has(blob.Hash(keep.Hash)) {
			t.Errorf("after 31 days: Has(drop) %v, Has(keep) %v; want false, true",
				a.files.Blobs().Has(blob.Hash(drop.Hash)), a.files.Blobs().Has(blob.Hash(keep.Hash)))
		}
	})
}

func TestNewFilesRequiresAllowed(t *testing.T) {
	base, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close() })
	proj, err := project.Create(base, "p", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proj.Close() })
	if _, err := NewFiles(proj, memnet.New(1, memnet.Faults{}).JoinBulk("a"), FilesOptions{}); err == nil {
		t.Error("NewFiles without Allowed succeeded")
	}
}

// SetReferenced replaces what the index refers to, but a blob just stored here
// stays served until the index has caught up (and then it is in the list).
func TestSetReferencedKeepsFreshBlobs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		clock := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
		var mu sync.Mutex
		now := func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
		a := newFilesPeer(t, net, "a", FilesOptions{Blob: blob.Options{Now: now}})
		fresh := a.addBlob(t, "fresh.bin", data(3000, 1))
		other := blob.HashOf([]byte("other"))

		if err := a.files.SetReferenced(nil); err != nil {
			t.Fatal(err)
		}
		if !a.files.referenced(blob.Hash(fresh.Hash)) {
			t.Error("a blob just stored was dropped by a SetReferenced that does not list it yet")
		}
		if a.files.referenced(other) {
			t.Error("a blob nobody mentioned is referenced")
		}
		mu.Lock()
		clock = clock.Add(2 * time.Hour)
		mu.Unlock()
		if a.files.referenced(blob.Hash(fresh.Hash)) {
			t.Error("a blob the index never listed is still referenced after the grace period")
		}
		// Listed by the index, it stays for good.
		fresh2 := a.addBlob(t, "fresh2.bin", data(3000, 2))
		_ = a.files.SetReferenced([][bridge.HashSize]byte{fresh2.Hash})
		mu.Lock()
		clock = clock.Add(100 * time.Hour)
		mu.Unlock()
		if !a.files.referenced(blob.Hash(fresh2.Hash)) {
			t.Error("a blob the index lists is not referenced")
		}
	})
}

// settled waits until the loop of p has written the visible copy of the blob h.
func (p *filesPeer) settled(t *testing.T, h blob.Hash) {
	t.Helper()
	for range 500 {
		p.files.mu.Lock()
		done := p.files.done[h]
		p.files.mu.Unlock()
		if done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the loop never wrote the visible copy")
}

// The visible copy of a 40 MB blob is streamed from the store, not loaded.
func TestMaterializeStreams(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 40 MB")
	}
	net := memnet.New(1, memnet.Faults{})
	a := newFilesPeer(t, net, "a", FilesOptions{})
	content := data(40<<20, 3)
	e := a.addBlob(t, "big.bin", content)
	if _, err := a.files.SetTree([]project.TreeEntry{e}); err != nil {
		t.Fatal(err)
	}
	a.settled(t, blob.Hash(e.Hash))
	if err := a.base.Remove(a.proj.Dir() + "/big.bin"); err != nil {
		t.Fatal(err)
	}
	// The heap must not grow by the size of the file: sample its peak while the
	// copy is written, with a collector eager enough to drop the garbage.
	defer debug.SetGCPercent(debug.SetGCPercent(10))
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var (
		peak atomic.Uint64
		stop = make(chan struct{})
		done = make(chan struct{})
	)
	go func() {
		defer close(done)
		var m runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(200 * time.Microsecond):
				runtime.ReadMemStats(&m)
				peak.Store(max(peak.Load(), m.HeapAlloc))
			}
		}
	}()
	err := a.files.materialize(blob.Hash(e.Hash))
	close(stop)
	<-done
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if grew := int64(peak.Load()) - int64(base.HeapAlloc); grew > 20<<20 {
		t.Errorf("the heap grew by %d MiB while a 40 MiB blob was written, want it streamed", grew>>20)
	}
	if got, err := a.read(t, "big.bin"); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("visible copy = %d bytes, %v", len(got), err)
	}
}

// A rotten blob in the store is not shown: the copy is refused and the blob dropped.
func TestMaterializeRefusesARottenBlob(t *testing.T) {
	net := memnet.New(1, memnet.Faults{})
	a := newFilesPeer(t, net, "a", FilesOptions{})
	e := a.addBlob(t, "x.bin", data(50_000, 4))
	if _, err := a.files.SetTree([]project.TreeEntry{e}); err != nil {
		t.Fatal(err)
	}
	a.settled(t, blob.Hash(e.Hash))
	if err := a.base.Remove(a.proj.Dir() + "/x.bin"); err != nil {
		t.Fatal(err)
	}
	hexName := strings.TrimPrefix(blob.Hash(e.Hash).String(), "blake3:")
	blobFile := a.proj.Dir() + "/.inkwell/blobs/" + hexName[:2] + "/" + hexName
	if err := a.base.Chmod(blobFile, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.base.WriteFile(blobFile, data(50_000, 5), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.files.materialize(blob.Hash(e.Hash)); !errors.Is(err, blob.ErrCorrupt) {
		t.Errorf("materialize of a rotten blob = %v, want ErrCorrupt", err)
	}
	if _, err := a.read(t, "x.bin"); err == nil {
		t.Error("a rotten blob became visible")
	}
	if a.files.Blobs().Has(blob.Hash(e.Hash)) {
		t.Error("the rotten blob is still stored")
	}
}

// ADR 0021: a suspended member is refused the bulk lane. The seam is
// FilesOptions.Allowed (TestAllowedGatesServing); what is missing is the
// Admission that knows about suspension.
func TestBulkLaneRefusedToSuspendedMember(t *testing.T) {
	t.Skip("TODO ADR 0021 (t11-b3a-suspend, not merged): Wiring.isActive must say false for a suspended member; then drive Wire + Files with a suspend op and assert no slice is served")
}

// Both projection writes tell the preview: the webview writes through
// WriteProjectionAt since the sync point moved to Go, and a missed notice left
// the preview on its first, failed compilation.
func TestProjectionWritesNotifyTheListener(t *testing.T) {
	writes := map[string]func(*Files, string) error{
		"WriteProjection": func(f *Files, rel string) error {
			return f.WriteProjection(rel, []byte("= A"))
		},
		"WriteProjectionAt": func(f *Files, rel string) error {
			return f.WriteProjectionAt(rel, []byte("= A"), nil)
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			p := newFilesPeer(t, memnet.New(1, memnet.Faults{}), "a", FilesOptions{})
			var got []string
			p.files.SetOnWrite(func(rel string) { got = append(got, rel) })
			if err := write(p.files, project.MainFile); err != nil {
				t.Fatal(err)
			}
			if want := []string{project.MainFile}; !slices.Equal(got, want) {
				t.Errorf("%s(%q) notified %q, want %q", name, project.MainFile, got, want)
			}
		})
	}
}
