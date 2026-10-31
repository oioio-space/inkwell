package identity

import "crypto/ed25519"

// SanitizeMachineName exposes sanitizeMachineName to the external tests.
var SanitizeMachineName = sanitizeMachineName

// Wordlist exposes the embedded wordlists to the external tests.
var Wordlist = wordlist

// InvalidFingerprint exposes the result returned for malformed keys.
var InvalidFingerprint = invalidFingerprint

// FingerprintCacheMax exposes the cache capacity.
const FingerprintCacheMax = fpCacheMax

// NewTestFingerprints provides an isolated cache with an injected computation.
func NewTestFingerprints(f func(ed25519.PublicKey) [4]uint16) (func(ed25519.PublicKey, string) [4]string, func() int) {
	cache := newFingerprints(f)
	return cache.fingerprint, cache.len
}
