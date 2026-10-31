package bridge

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/oioio-space/inkwell/internal/project"
)

// TooLargeError is returned by [Tree.PutBlob] for a file over the size
// limit of a blob. The store stream answers it with [StatusTooLarge].
type TooLargeError struct {
	// Limit is the largest size accepted, in bytes.
	Limit int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("file over the limit of %d bytes", e.Limit)
}

// Flags and sizes of the tree payload.
const (
	flagTreeMain = 1 << 0 // entry: the project's main document
	// treeHead is the fixed part of an entry: type, flags, size, path length.
	treeHead = 1 + 1 + 8 + 2
	// maxURL bounds the document URL of a text entry.
	maxURL = 255
)

// hashLen is the length of a blob hash in the tree and the referenced list.
const hashLen = HashSize

func refLen(e project.TreeEntry) int {
	switch e.Type {
	case project.NodeText:
		return len(e.URL)
	case project.NodeBlob:
		return hashLen
	}
	return 0
}

// checkTreeEntry checks what the wire format can carry; whether the path is a
// good one is [project.Project.ApplyTree]'s business.
func checkTreeEntry(e project.TreeEntry) error {
	switch {
	case e.Type > project.NodeBlob, e.Size < 0:
		return fmt.Errorf("%w: tree entry of type %d and size %d", ErrMalformed, e.Type, e.Size)
	case e.Path == "" || len(e.Path) > project.MaxPathLen || !utf8.ValidString(e.Path):
		return fmt.Errorf("%w: tree entry path of %d bytes", ErrMalformed, len(e.Path))
	case len(e.URL) > maxURL || !utf8.ValidString(e.URL) || (e.Type != project.NodeText && e.URL != ""):
		return fmt.Errorf("%w: tree entry URL of %d bytes", ErrMalformed, len(e.URL))
	case e.Type != project.NodeBlob && e.Hash != [hashLen]byte{}:
		return fmt.Errorf("%w: tree entry with a hash but no blob", ErrMalformed)
	}
	return nil
}

// DecodeTree parses the data built by [EncodeTree]. It returns an error
// wrapping [ErrMalformed] on an unknown version, a list over [project.MaxNodes],
// or any inconsistency. It checks the shape of the entries, not their names:
// a path that is not portable comes through, for [project.Project.ApplyTree] to
// refuse and report.
func DecodeTree(b []byte) ([]project.TreeEntry, error) {
	if len(b) < 5 || b[0] != filesVersion {
		return nil, fmt.Errorf("%w: tree header", ErrMalformed)
	}
	count := binary.BigEndian.Uint32(b[1:])
	if count > project.MaxNodes {
		return nil, fmt.Errorf("%w: %d tree entries", ErrMalformed, count)
	}
	b = b[5:]
	out := make([]project.TreeEntry, 0, min(int(count), len(b)/treeHead))
	for range count {
		if len(b) < treeHead {
			return nil, fmt.Errorf("%w: truncated tree entry", ErrMalformed)
		}
		e := project.TreeEntry{
			Type: project.NodeType(b[0]),
			Main: b[1]&flagTreeMain != 0,
		}
		size := binary.BigEndian.Uint64(b[2:])
		n := int(binary.BigEndian.Uint16(b[10:]))
		flags := b[1]
		b = b[treeHead:]
		if e.Type > project.NodeBlob || flags&^flagTreeMain != 0 || size > math.MaxInt64 ||
			n == 0 || n > project.MaxPathLen || len(b) < n+1 || !utf8.Valid(b[:n]) {
			return nil, fmt.Errorf("%w: bad tree entry", ErrMalformed)
		}
		e.Size, e.Path = int64(size), string(b[:n]) // #nosec G115 -- checked.
		rlen := int(b[n])
		b = b[n+1:]
		if len(b) < rlen {
			return nil, fmt.Errorf("%w: truncated tree reference", ErrMalformed)
		}
		ref := b[:rlen]
		b = b[rlen:]
		switch {
		case e.Type == project.NodeText && rlen > 0 && utf8.Valid(ref):
			e.URL = string(ref)
		case e.Type == project.NodeBlob && rlen == hashLen:
			copy(e.Hash[:], ref)
		case e.Type == project.NodeDir && rlen == 0:
		default:
			return nil, fmt.Errorf("%w: tree reference of %d bytes for a %v", ErrMalformed, rlen, e.Type)
		}
		out = append(out, e)
	}
	if len(b) != 0 {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrMalformed, len(b))
	}
	return out, nil
}

// EncodeTreeResult serializes the payload of an OpTree response:
//
//	version u8 | rejected u32
//
// rejected is the number of entries Go refused (hostile or colliding names).
func EncodeTreeResult(rejected int) []byte {
	b := []byte{filesVersion}
	return binary.BigEndian.AppendUint32(b, uint32(min(rejected, math.MaxUint32))) // #nosec G115 -- clamped.
}

// DecodeReferenced parses the data built by [EncodeReferenced].
func DecodeReferenced(b []byte) ([][hashLen]byte, error) {
	if len(b) < 5 || b[0] != filesVersion {
		return nil, fmt.Errorf("%w: referenced header", ErrMalformed)
	}
	count := binary.BigEndian.Uint32(b[1:])
	if uint64(len(b)-5) != uint64(count)*hashLen { // #nosec G115 -- len(b) ≥ 5 was checked above.
		return nil, fmt.Errorf("%w: %d referenced hashes in %d bytes", ErrMalformed, count, len(b)-5)
	}
	out := make([][hashLen]byte, count)
	for i := range out {
		copy(out[i][:], b[5+i*hashLen:])
	}
	return out, nil
}

// EncodeBlobInfo serializes the payload of an OpPutBlob response:
//
//	version u8 | hash (32 bytes) | size u64
func EncodeBlobInfo(hash [hashLen]byte, size int64) []byte {
	b := append([]byte{filesVersion}, hash[:]...)
	return binary.BigEndian.AppendUint64(b, uint64(max(size, 0))) // #nosec G115 -- clamped ≥ 0.
}
