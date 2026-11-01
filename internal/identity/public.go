package identity

import (
	"bytes"
	"errors"
	"fmt"

	"filippo.io/edwards25519"
)

// ErrInvalidPublicKey marks a key outside the nonzero prime-order subgroup.
var ErrInvalidPublicKey = errors.New("identity: invalid Ed25519 public key")

// ValidatePublicKey rejects invalid, non-canonical and non-prime-order Ed25519
// public keys, including the identity. Call it before accepting a peer's key;
// crypto/ed25519 alone does not enforce these restrictions.
func ValidatePublicKey(key []byte) error {
	p, err := new(edwards25519.Point).SetBytes(key)
	if err != nil || !bytes.Equal(p.Bytes(), key) {
		return fmt.Errorf("%w: invalid or non-canonical encoding", ErrInvalidPublicKey)
	}
	if new(edwards25519.Point).MultByCofactor(p).Equal(edwards25519.NewIdentityPoint()) == 1 {
		return fmt.Errorf("%w: small order", ErrInvalidPublicKey)
	}
	// Scalars are reduced modulo L, so compute [L]P as [L-1]P + P.
	orderMinusOne := []byte{0xec, 0xd3, 0xf5, 0x5c, 0x1a, 0x63, 0x12, 0x58, 0xd6, 0x9c, 0xf7, 0xa2, 0xde, 0xf9, 0xde, 0x14, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10}
	scalar, _ := new(edwards25519.Scalar).SetCanonicalBytes(orderMinusOne)
	multiple := new(edwards25519.Point).ScalarMult(scalar, p)
	if multiple.Add(multiple, p).Equal(edwards25519.NewIdentityPoint()) != 1 {
		return fmt.Errorf("%w: mixed order", ErrInvalidPublicKey)
	}
	return nil
}
