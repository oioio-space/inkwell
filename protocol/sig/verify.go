// SPDX-License-Identifier: MPL-2.0

package sig

import (
	"crypto/ed25519"
	"crypto/sha512"
	"crypto/subtle"
	"errors"
	"fmt"

	"filippo.io/edwards25519"
)

// ErrInvalidSignature marks a signature that the exact verdict of the protocol
// rejects.
var ErrInvalidSignature = errors.New("sig: invalid Ed25519 signature")

// Verify checks sig as the protocol requires (spec §3.3): "une signature (R, S)
// de M par A est acceptée si et seulement si (a) A est valide (§3.2) ; (b)
// S < L (#ed25519-noncanonical-s) ; (c) avec k = SHA-512(R || A || M) mod L,
// l'encodage du point [S]B − [k]A est octet pour octet égal à R (équation sans
// cofacteur de [RFC 8032 §5.1.7], comparaison des encodages). R n'est pas
// validé autrement."
//
// This is stricter than crypto/ed25519.Verify on the key: a non-canonical,
// mixed-order or small-order A is refused by [ValidatePublicKey] before the
// equation runs. R is accepted exactly when it equals the canonical encoding of
// [S]B − [k]A, so a non-canonical or torsion-bearing R never matches.
func Verify(key, msg, sig []byte) error {
	if err := ValidatePublicKey(key); err != nil {
		return err
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("%w: signature length %d", ErrInvalidSignature, len(sig))
	}
	s, err := new(edwards25519.Scalar).SetCanonicalBytes(sig[32:])
	if err != nil {
		return fmt.Errorf("%w: non-canonical S", ErrInvalidSignature)
	}
	a, err := new(edwards25519.Point).SetBytes(key)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}
	h := sha512.New()
	h.Write(sig[:32])
	h.Write(key)
	h.Write(msg)
	k, err := new(edwards25519.Scalar).SetUniformBytes(h.Sum(nil))
	if err != nil {
		return fmt.Errorf("%w: challenge", ErrInvalidSignature)
	}
	negK := new(edwards25519.Scalar).Negate(k)
	expected := new(edwards25519.Point).VarTimeDoubleScalarBaseMult(negK, a, s)
	if subtle.ConstantTimeCompare(expected.Bytes(), sig[:32]) != 1 {
		return fmt.Errorf("%w: equation mismatch", ErrInvalidSignature)
	}
	return nil
}
