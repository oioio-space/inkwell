package identity

import (
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

	fpMu    sync.Mutex
	fpCache = map[[ed25519.PublicKeySize]byte][fpWords]uint16{}
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
// indices in both languages. Results are cached per key.
func Fingerprint(pub ed25519.PublicKey, lang string) [4]string {
	idx := indices(pub)
	words := wordlist(lang)
	var out [fpWords]string
	for i, n := range idx {
		out[i] = words[n]
	}
	return out
}

func indices(pub ed25519.PublicKey) [fpWords]uint16 {
	var key [ed25519.PublicKeySize]byte
	copy(key[:], pub)

	fpMu.Lock()
	defer fpMu.Unlock()
	if idx, ok := fpCache[key]; ok {
		return idx
	}
	sum := argon2.IDKey(pub, []byte(fpSalt), fpTime, fpMemoryKiB, fpThreads, fpKeyLen)
	// Keep the top 44 bits of the first 8 bytes.
	v := binary.BigEndian.Uint64(sum) >> (64 - fpWords*fpBitsPerIdx)
	var idx [fpWords]uint16
	for i := range idx {
		shift := (fpWords - 1 - i) * fpBitsPerIdx
		idx[i] = uint16(v >> shift & (1<<fpBitsPerIdx - 1))
	}
	if len(fpCache) >= fpCacheMax {
		clear(fpCache)
	}
	fpCache[key] = idx
	return idx
}
