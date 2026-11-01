package identity

import "github.com/oioio-space/inkwell/protocol/sig"

// ErrInvalidPublicKey marks an invalid public key.
var ErrInvalidPublicKey = sig.ErrInvalidPublicKey

// ValidatePublicKey validates a public key.
func ValidatePublicKey(key []byte) error { return sig.ValidatePublicKey(key) }
