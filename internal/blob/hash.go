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

// hashPrefix starts the text form of a [Hash], as the index of a project
// writes it.
const hashPrefix = "blake3:"

// ErrBadHash is returned by [ParseHash] for a text that is not a hash.
var ErrBadHash = errors.New("blob: invalid hash")

// Hash is the BLAKE3 hash of a file, which names the blob.
type Hash [HashSize]byte

// HashOf returns the hash of data.
func HashOf(data []byte) Hash { return blake3.Sum256(data) }

// String returns the text form "blake3:<64 lowercase hex digits>".
func (h Hash) String() string { return hashPrefix + hex.EncodeToString(h[:]) }

// hex returns the 64 hex digits that name the blob on disk.
func (h Hash) hex() string { return hex.EncodeToString(h[:]) }

// short returns the first 8 hex digits, for logs.
func (h Hash) short() string { return hex.EncodeToString(h[:4]) }

// ParseHash parses the text form of [Hash.String]: "blake3:" and 64 hex
// digits, lower case only so that a hash has a single spelling.
func ParseHash(s string) (Hash, error) {
	digits, ok := strings.CutPrefix(s, hashPrefix)
	if !ok || len(digits) != 2*HashSize || digits != strings.ToLower(digits) {
		return Hash{}, fmt.Errorf("%w: %d bytes", ErrBadHash, len(s))
	}
	var h Hash
	if _, err := hex.Decode(h[:], []byte(digits)); err != nil {
		return Hash{}, fmt.Errorf("%w: not hexadecimal", ErrBadHash)
	}
	return h, nil
}
