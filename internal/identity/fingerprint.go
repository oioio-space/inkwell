package identity

import (
	"container/list"
	"crypto/ed25519"
	_ "embed" // wordlists
	"encoding/binary"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

//go:embed wordlist/fr.txt
var frRaw string

//go:embed wordlist/en.txt
var enRaw string

const (
	fpWords      = 4
	fpBitsPerIdx = 11
	fpSalt       = "inkwell-fp-v1"
	fpTime       = 1
	fpMemoryKiB  = 64 * 1024
	fpThreads    = 1
	fpKeyLen     = 8
	fpCacheMax   = 1024
)

var (
	frWords = sync.OnceValue(func() []string { return splitWords(frRaw) })
	enWords = sync.OnceValue(func() []string { return splitWords(enRaw) })

	// invalidFingerprint is what [Fingerprint] returns for a key that is not
	// exactly ed25519.PublicKeySize bytes. "????" is in no wordlist, so it
	// never matches, and never collides with, a real fingerprint.
	invalidFingerprint = [fpWords]string{"????", "????", "????", "????"}
)

func splitWords(raw string) []string {
	var words []string
	for line := range strings.SplitSeq(raw, "\n") {
		if w := strings.TrimSpace(line); w != "" {
			words = append(words, w)
		}
	}
	return words
}

// wordlist returns the embedded BIP39 list for lang ("fr", otherwise English).
func wordlist(lang string) []string {
	if lang == "fr" {
		return frWords()
	}
	return enWords()
}

// Fingerprint returns the four-word fingerprint of pub in lang ("fr" or "en";
// any other value selects English). The words come from the BIP39 lists and
// encode 44 bits derived with Argon2id (t=1, m=64 MiB), so forging a key with
// a look-alike fingerprint is expensive. The same key yields the same word
// indices in both languages. Results are cached per exact key in a bounded
// LRU, and concurrent calls for the same key share one computation.
//
// A pub that is not exactly ed25519.PublicKeySize bytes is malformed: the
// result is four "????" placeholders, never derived from (or cached for) the
// input, so it cannot be mistaken for or poison a valid key's fingerprint.
func Fingerprint(pub ed25519.PublicKey, lang string) [4]string {
	if len(pub) != ed25519.PublicKeySize {
		return invalidFingerprint
	}
	idx := indices(pub)
	words := wordlist(lang)
	var out [fpWords]string
	for i, n := range idx {
		out[i] = words[n]
	}
	return out
}

// fpCall is one in-flight computation shared by every caller of the same key.
type fpCall struct {
	done chan struct{}
	idx  [fpWords]uint16
}

// fingerprints is a bounded LRU cache with per-key singleflight. Its mutex
// guards only the bookkeeping, never the Argon2 computation, so distinct keys
// compute in parallel and concurrent callers of one key compute it once.
type fingerprints struct {
	compute  func(ed25519.PublicKey) [fpWords]uint16
	order    *list.List // front = most recently used; values are fpEntry
	items    map[[ed25519.PublicKeySize]byte]*list.Element
	inflight map[[ed25519.PublicKeySize]byte]*fpCall
	mu       sync.Mutex
}

type fpEntry struct {
	key [ed25519.PublicKeySize]byte
	idx [fpWords]uint16
}

var fpState = newFingerprints(computeIndices)

func newFingerprints(compute func(ed25519.PublicKey) [fpWords]uint16) *fingerprints {
	f := &fingerprints{}
	f.reset(compute)
	return f
}

// reset empties the cache and installs compute.
func (f *fingerprints) reset(compute func(ed25519.PublicKey) [fpWords]uint16) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compute = compute
	f.order = list.New()
	f.items = map[[ed25519.PublicKeySize]byte]*list.Element{}
	f.inflight = map[[ed25519.PublicKeySize]byte]*fpCall{}
}

func (f *fingerprints) len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.order.Len()
}

// get returns the indices of key, which must be a full-length public key.
func (f *fingerprints) get(key [ed25519.PublicKeySize]byte) [fpWords]uint16 {
	f.mu.Lock()
	if el, ok := f.items[key]; ok {
		f.order.MoveToFront(el)
		idx := el.Value.(fpEntry).idx
		f.mu.Unlock()
		return idx
	}
	if c, ok := f.inflight[key]; ok {
		f.mu.Unlock()
		<-c.done
		return c.idx
	}
	c := &fpCall{done: make(chan struct{})}
	f.inflight[key] = c
	compute := f.compute
	f.mu.Unlock()

	c.idx = compute(key[:]) // outside the lock: Argon2 takes 64 MiB and tens of ms

	f.mu.Lock()
	delete(f.inflight, key)
	f.items[key] = f.order.PushFront(fpEntry{key: key, idx: c.idx})
	for f.order.Len() > fpCacheMax {
		oldest := f.order.Back()
		delete(f.items, oldest.Value.(fpEntry).key)
		f.order.Remove(oldest)
	}
	f.mu.Unlock()
	close(c.done)
	return c.idx
}

func indices(pub ed25519.PublicKey) [fpWords]uint16 {
	return fpState.get([ed25519.PublicKeySize]byte(pub))
}

func computeIndices(pub ed25519.PublicKey) [fpWords]uint16 {
	sum := argon2.IDKey(pub, []byte(fpSalt), fpTime, fpMemoryKiB, fpThreads, fpKeyLen)
	// Keep the top 44 bits of the first 8 bytes.
	v := binary.BigEndian.Uint64(sum) >> (64 - fpWords*fpBitsPerIdx)
	var idx [fpWords]uint16
	for i := range idx {
		shift := (fpWords - 1 - i) * fpBitsPerIdx
		idx[i] = uint16(v >> shift & (1<<fpBitsPerIdx - 1))
	}
	return idx
}
