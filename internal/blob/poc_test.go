package blob_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/transport"
)

// PoC: partial downloads are not counted against MaxProject. A peer that
// serves everything but the last group of each blob fills the disk far past
// the project quota without any confirmation.
func TestPoCPartialsBypassProjectQuota(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		const quota = 2 * blob.MaxSlice
		b := n.join("b", blob.Options{MaxProject: quota}, blob.ServiceOptions{})
		evilStore, _ := openStore(t, "", blob.Options{})
		evil := newScripted(t, n, "evil", evilStore)
		sizes := map[blob.Hash]int64{}
		evil.handle = func(_ transport.PeerID, w blob.Want, def func() blob.Message) []blob.Message {
			if w.Offset+uint64(w.Length) >= uint64(sizes[w.Hash]) {
				return []blob.Message{blob.Missing{Hash: w.Hash}} // withhold the tail
			}
			return []blob.Message{def()}
		}
		n.link("evil", "b")
		var refs []blob.Ref
		for i := range 6 {
			data := testData(quota, uint64(1000+i))
			h, _ := evilStore.PutBytes(data)
			sizes[h] = int64(len(data))
			refs = append(refs, blob.Ref{Hash: h, Size: int64(len(data))})
		}
		for _, r := range refs {
			err := b.svc.Fetch(t.Context(), r)
			if errors.Is(err, blob.ErrProjectQuota) {
				break // the quota is enforced: stop here
			}
		}
		var onDisk int64
		ents, _ := os.ReadDir(filepath.Join(b.dir, "partial"))
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".map") {
				continue // the bookkeeping, not downloaded data
			}
			if fi, err := e.Info(); err == nil {
				onDisk += fi.Size()
			}
		}
		t.Logf("partial bytes = %d, quota = %d, Used() = %d", onDisk, quota, b.store.Used())
		if onDisk > quota {
			t.Error("VULN: partial downloads exceed the project quota unconfirmed")
		}
	})
}

// PoC: when the index size is wrong, an honest peer that answers Busy (or
// times out) to the confirmation request is counted as a forger; three such
// blobs and it is dropped (Node.Drop closes every connection).
func TestPoCHonestBusyPeerFaulted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		st, _ := openStore(t, "", blob.Options{})
		honest := newScripted(t, n, "honest", st)
		seen := map[blob.Hash]int{}
		honest.handle = func(_ transport.PeerID, w blob.Want, def func() blob.Message) []blob.Message {
			seen[w.Hash]++
			if seen[w.Hash] == 1 {
				return []blob.Message{def()} // true slice, true length
			}
			return []blob.Message{blob.Busy{Hash: w.Hash, Retry: time.Second}} // legitimately busy
		}
		n.link("honest", "b")
		for i := range blob.MaxFaults {
			data := testData(2*blob.MaxSlice+5, uint64(2000+i))
			h, _ := st.PutBytes(data)
			// a malicious writer put a wrong size in the index
			_ = b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data)) + 1})
		}
		t.Logf("security events = %d, dropped = %v", b.log.security(), b.droppedPeers())
		if len(b.droppedPeers()) > 0 {
			t.Error("VULN: honest peer dropped for answering Busy under a lying index")
		}
	})
}
