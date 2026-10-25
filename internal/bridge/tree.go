package bridge

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
)

// TooLargeError is returned by [Storage.PutBlob] for a file over the size
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

// EncodeTree serializes entries as the data of an OpTree request:
//
//	version u8 | count u32 | count × (type u8 | flags u8 | size u64 | len u16 | path | rlen u8 | ref)
//
// type is 0 for a folder, 1 for a text file and 2 for a blob; flags bit 0 is
// Main; ref is the document URL of a text file, the 32 bytes of the hash of a
// blob, and empty for a folder. It returns an error wrapping [ErrMalformed] if
// an entry cannot be encoded or the result would not fit a frame. The webview
// has its own encoder; this one serves tests and Go clients.
func EncodeTree(entries []project.TreeEntry) ([]byte, error) {
	size := 1 + 4
	for _, e := range entries {
		if err := checkTreeEntry(e); err != nil {
			return nil, err
		}
		size += treeHead + len(e.Path) + 1 + refLen(e)
	}
	if size > store.MaxValue {
		return nil, fmt.Errorf("%w: tree of %d bytes", ErrMalformed, size)
	}
	b := make([]byte, 0, size)
	b = append(b, filesVersion)
	b = binary.BigEndian.AppendUint32(b, uint32(len(entries))) // #nosec G115 -- bounded by the size check.
	for _, e := range entries {
		var flags byte
		if e.Main {
			flags |= flagTreeMain
		}
		b = append(b, byte(e.Type), flags)
		b = binary.BigEndian.AppendUint64(b, uint64(e.Size))      // #nosec G115 -- checked ≥ 0.
		b = binary.BigEndian.AppendUint16(b, uint16(len(e.Path))) // #nosec G115 -- checked ≤ MaxPathLen.
		b = append(b, e.Path...)
		b = append(b, byte(refLen(e))) // #nosec G115 -- checked ≤ 255.
		switch e.Type {
		case project.NodeText:
			b = append(b, e.URL...)
		case project.NodeBlob:
			b = append(b, e.Hash[:]...)
		}
	}
	return b, nil
}

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

// DecodeTreeResult parses the payload built by [EncodeTreeResult].
func DecodeTreeResult(b []byte) (rejected int, err error) {
	if len(b) != 5 || b[0] != filesVersion {
		return 0, fmt.Errorf("%w: tree result", ErrMalformed)
	}
	return int(binary.BigEndian.Uint32(b[1:])), nil
}

// EncodeReferenced serializes hashes as the data of an OpReferenced request:
//
//	version u8 | count u32 | count × hash (32 bytes)
//
// The hashes are those the project index refers to, with the trash and the
// packages: what Go may serve to the members and must keep. It returns an
// error wrapping [ErrMalformed] if the list would not fit a frame.
func EncodeReferenced(hashes [][hashLen]byte) ([]byte, error) {
	if 5+len(hashes)*hashLen > store.MaxValue {
		return nil, fmt.Errorf("%w: %d referenced hashes", ErrMalformed, len(hashes))
	}
	b := make([]byte, 0, 5+len(hashes)*hashLen)
	b = append(b, filesVersion)
	b = binary.BigEndian.AppendUint32(b, uint32(len(hashes))) // #nosec G115 -- bounded by the size check.
	for _, h := range hashes {
		b = append(b, h[:]...)
	}
	return b, nil
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

// DecodeBlobInfo parses the payload built by [EncodeBlobInfo].
func DecodeBlobInfo(b []byte) (hash [hashLen]byte, size int64, err error) {
	if len(b) != 1+hashLen+8 || b[0] != filesVersion {
		return hash, 0, fmt.Errorf("%w: blob info", ErrMalformed)
	}
	copy(hash[:], b[1:])
	n := binary.BigEndian.Uint64(b[1+hashLen:])
	if n > math.MaxInt64 {
		return hash, 0, fmt.Errorf("%w: blob size", ErrMalformed)
	}
	return hash, int64(n), nil // #nosec G115 -- checked.
}
