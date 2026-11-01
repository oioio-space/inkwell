package blob

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func openInternal(t *testing.T, o Options) *Store {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	s, err := Open(root, o)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// However many downloads ask at once, the holds never add up past the quota.
func TestConcurrentHoldsRespectTheQuota(t *testing.T) {
	const size, fit = 1 << 20, 3
	s := openInternal(t, Options{MaxFile: size, MaxProject: fit * size, FreeSpace: func() (uint64, error) { return 1 << 40, nil }})
	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			switch err := s.hold(fmt.Sprintf("k%d", i), size); {
			case err == nil:
				granted.Add(1)
			case !errors.Is(err, ErrProjectQuota):
				t.Errorf("hold: %v", err)
			}
		})
	}
	wg.Wait()
	if got := granted.Load(); got != fit {
		t.Errorf("%d holds granted for a quota of %d files, want exactly %d", got, fit, fit)
	}
}

// The free space the system reports already lacks what a download has written,
// so only the part still to come is set aside.
func TestFreeSpaceIsNotCountedTwice(t *testing.T) {
	const mib = 1 << 20
	s := openInternal(t, Options{
		MaxFile: 10 * mib, Reserve: 1, MaxProject: 100 * mib,
		FreeSpace: func() (uint64, error) { return 8 * mib, nil },
	})
	if err := s.hold("a", 6*mib); err != nil {
		t.Fatal(err)
	}
	// Only 1 MiB of the 6 is still to come: 3 MiB for the new file and 1 MiB fit in 8.
	s.written("a", 5*mib)
	if err := s.hold("b", 3*mib); err != nil {
		t.Errorf("hold with 5 of 6 MiB already written = %v, want it granted", err)
	}
	s.unhold("b")
	// Nothing written: 6 MiB are still to come, and 3 more do not fit in 8.
	s.written("a", 0)
	if err := s.hold("b", 3*mib); !errors.Is(err, ErrNoSpace) {
		t.Errorf("hold with nothing written = %v, want ErrNoSpace", err)
	}
}

// A restart counts what the map of a stopped download says it has.
func TestScanCountsWrittenGroups(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	s, err := Open(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	h := HashOf([]byte("x"))
	d, err := s.openDownload(h, 4*GroupSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.put(0, make([]byte, 2*GroupSize)); err != nil {
		t.Fatal(err)
	}
	if err := d.suspend(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	again.mu.Lock()
	defer again.mu.Unlock()
	if again.holds[h.Hex()] != 4*GroupSize || again.wrote[h.Hex()] != 2*GroupSize {
		t.Errorf("hold %d, written %d after a restart, want %d and %d",
			again.holds[h.Hex()], again.wrote[h.Hex()], 4*GroupSize, 2*GroupSize)
	}
}
