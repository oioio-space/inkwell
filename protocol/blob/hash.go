// SPDX-License-Identifier: MPL-2.0

package blob

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"lukechampine.com/blake3"
)

// HashSize is the length of a [Hash] in bytes.
const HashSize = 32

// HashPrefix is the prefix of the canonical text form of a [Hash].
// Application blob stores use it to parse hexadecimal filenames.
const HashPrefix = "blake3:"

// ErrBadHash is returned by [ParseHash] for a text that is not a hash.
var ErrBadHash = errors.New("blob: invalid hash")

// Hash is the BLAKE3 hash of a file, which names the blob.
type Hash [HashSize]byte

// HashOf returns the hash of data.
func HashOf(data []byte) Hash { return blake3.Sum256(data) }

// String returns the text form "blake3:<64 lowercase hex digits>".
func (h Hash) String() string { return HashPrefix + hex.EncodeToString(h[:]) }

// Hex returns the 64 lowercase hexadecimal digits of h, without [HashPrefix].
// Application blob stores use it as the filename.
func (h Hash) Hex() string { return hex.EncodeToString(h[:]) }

// Short returns the first eight lowercase hexadecimal digits of h for logs.
// It is a display abbreviation, not a unique blob identifier.
func (h Hash) Short() string { return hex.EncodeToString(h[:4]) }

// ParseHash parses the text form of [Hash.String]: "blake3:" and 64 hex
// digits, lower case only so that a hash has a single spelling.
func ParseHash(s string) (Hash, error) {
	digits, ok := strings.CutPrefix(s, HashPrefix)
	if !ok || len(digits) != 2*HashSize || digits != strings.ToLower(digits) {
		return Hash{}, fmt.Errorf("%w: %d bytes", ErrBadHash, len(s))
	}
	var h Hash
	if _, err := hex.Decode(h[:], []byte(digits)); err != nil {
		return Hash{}, fmt.Errorf("%w: not hexadecimal", ErrBadHash)
	}
	return h, nil
}
