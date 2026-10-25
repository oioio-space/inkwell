package identity_test

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/oioio-space/inkwell/internal/identity"
)

func TestLoadOrCreateConcurrent(t *testing.T) {
	root, dir := newRoot(t)

	const n = 16
	pubs := make([]ed25519.PublicKey, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			d, err := identity.LoadOrCreate(root)
			if err != nil {
				errs[i] = err
				return
			}
			pubs[i] = d.Public()
		})
	}
	wg.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Fatalf("LoadOrCreate() #%d = %v", i, errs[i])
		}
		if !bytes.Equal(pubs[i], pubs[0]) {
			t.Errorf("LoadOrCreate() #%d key = %x, want %x", i, pubs[i], pubs[0])
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "device.key" {
		t.Errorf("directory entries = %v, want only device.key", entries)
	}
}

func TestFingerprintRejectsWrongLength(t *testing.T) {
	valid := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(valid, fixedKey()[:16]) // trailing zeros: a zero-padded short key would collide

	var calls atomic.Int32
	restore := identity.SetFingerprintCompute(func(pub ed25519.PublicKey) [4]uint16 {
		calls.Add(1)
		return [4]uint16{1, 2, 3, 4}
	})
	defer restore()

	for _, short := range []ed25519.PublicKey{nil, valid[:16], append(bytes.Clone(valid), 0)} {
		if got := identity.Fingerprint(short, "en"); got != identity.InvalidFingerprint {
			t.Errorf("Fingerprint(len %d) = %q, want %q", len(short), got, identity.InvalidFingerprint)
		}
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("compute called %d times for invalid keys, want 0", got)
	}
	if got := identity.Fingerprint(valid, "en"); got == identity.InvalidFingerprint {
		t.Errorf("Fingerprint(valid key) = %q after invalid calls, want real words", got)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("compute called %d times for the valid key, want 1", got)
	}
}

func TestFingerprintDoesNotSerializeKeys(t *testing.T) {
	keyA := fixedKey()
	keyB := bytes.Clone(keyA)
	keyB[0] ^= 0xff

	started := make(chan struct{})
	release := make(chan struct{})
	restore := identity.SetFingerprintCompute(func(pub ed25519.PublicKey) [4]uint16 {
		if bytes.Equal(pub, keyA) {
			close(started)
			<-release // key A's computation hangs until B has finished
		}
		return [4]uint16{5, 6, 7, 8}
	})
	defer restore()

	var wg sync.WaitGroup
	wg.Go(func() { identity.Fingerprint(keyA, "en") })
	<-started
	identity.Fingerprint(keyB, "en") // deadlocks if A's compute holds a global lock
	close(release)
	wg.Wait()
}

func TestFingerprintSingleflight(t *testing.T) {
	key := fixedKey()
	var calls atomic.Int32
	release := make(chan struct{})
	restore := identity.SetFingerprintCompute(func(ed25519.PublicKey) [4]uint16 {
		calls.Add(1)
		<-release
		return [4]uint16{9, 9, 9, 9}
	})
	defer restore()

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { identity.Fingerprint(key, "en") })
	}
	close(release)
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Errorf("compute called %d times for one key, want 1", got)
	}
}

func TestFingerprintCacheBounded(t *testing.T) {
	var calls int
	restore := identity.SetFingerprintCompute(func(ed25519.PublicKey) [4]uint16 {
		calls++
		return [4]uint16{}
	})
	defer restore()

	key := make(ed25519.PublicKey, ed25519.PublicKeySize)
	for i := range identity.FingerprintCacheMax + 10 {
		key[0], key[1] = byte(i), byte(i>>8)
		identity.Fingerprint(key, "en")
	}
	if got := identity.FingerprintCacheLen(); got > identity.FingerprintCacheMax {
		t.Errorf("cache holds %d entries, want <= %d", got, identity.FingerprintCacheMax)
	}
	// The most recent key survived eviction; the oldest did not.
	before := calls
	identity.Fingerprint(key, "en")
	if calls != before {
		t.Error("most recently used key was evicted")
	}
	key[0], key[1] = 0, 0
	identity.Fingerprint(key, "en")
	if calls != before+1 {
		t.Error("least recently used key was not evicted")
	}
}

func TestCleanName(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"plain", "Alice's laptop", "Alices laptop"},
		{"bidi override", "evil\u202egpj.exe", "evilgpj.exe"},
		{"bidi isolate", "a\u2066b\u2069c", "abc"},
		{"zero width", "ad\u200bmin\u200d\ufeff", "admin"},
		{"control", "line\nbreak\x00\x7f", "linebreak"},
		{"trim", "  spaced  ", "spaced"},
		{"empty stays empty", "\u202e\u200b", ""},
		{"truncate", string(bytes.Repeat([]byte("é"), 60)), string(bytes.Repeat([]byte("é"), 40))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := identity.CleanName(tc.in); got != tc.want {
				t.Errorf("CleanName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
