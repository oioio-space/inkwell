// SPDX-License-Identifier: MPL-2.0

package conformance

// Strict is a thin wrapper around Reference for migration step 2 (ADR 0034, N-1).
// It routes key and signature decoders through the same checks as Reference.
func Strict(v Vector) error {
	return Reference(v)
}
