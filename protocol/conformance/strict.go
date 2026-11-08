// SPDX-License-Identifier: MPL-2.0

package conformance

// Strict is the check required from migration step 2 (ADR 0034, N-1). It
// routes every key and signature decoder to protocol/sig, which enforces the
// exact verdict of spec §3.2-§3.3; decoders that carry embedded signatures
// (trust_op+apply, attestation+verify, trust_fold) use the same path through
// [Reference]. Every other format keeps this release's decoder.
func Strict(v Vector) error {
	c, err := newReferenceCase(v)
	if err != nil {
		return err
	}
	var check func() error
	switch v.Decoder {
	case "public_key":
		check = c.key
	case "peer_id":
		check = c.peer
	case "signature", "signature+key", "":
		check = c.signature
	case "repo_message":
		check = c.repo
	default:
		return Reference(v)
	}
	if err := c.prepare(); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	return c.verify()
}
