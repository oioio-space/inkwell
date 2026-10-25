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

// FingerprintCacheLen reports the number of cached fingerprints.
func FingerprintCacheLen() int { return fpState.len() }

// SetFingerprintCompute replaces the Argon2 step and empties the cache. The
// returned function restores the real one and empties the cache again.
func SetFingerprintCompute(f func(ed25519.PublicKey) [4]uint16) (restore func()) {
	fpState.reset(f)
	return func() { fpState.reset(computeIndices) }
}
