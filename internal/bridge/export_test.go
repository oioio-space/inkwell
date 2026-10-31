package bridge

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
)

// DecodeFileInfo parses the payload built by [EncodeFileInfo]. It returns an
// error wrapping [ErrMalformed] on an unknown version or any inconsistency.
func DecodeFileInfo(b []byte) (FileInfo, error) {
	if len(b) < 4 || b[0] != filesVersion || b[1]&^(flagBOM|flagMixed) != 0 ||
		project.EOL(b[2]) > project.CR || len(b) != 4+int(b[3]) {
		return FileInfo{}, fmt.Errorf("%w: file info", ErrMalformed)
	}
	return FileInfo{
		Encoding: string(b[4:]), EOL: project.EOL(b[2]), BOM: b[1]&flagBOM != 0, Mixed: b[1]&flagMixed != 0,
	}, nil
}

// DecodeFileList parses the payload built by [EncodeFileList]. It returns an
// error wrapping [ErrMalformed] on an unknown version or any inconsistency.
func DecodeFileList(b []byte) ([]project.File, error) {
	if len(b) < 5 || b[0] != filesVersion {
		return nil, fmt.Errorf("%w: file list header", ErrMalformed)
	}
	count := binary.BigEndian.Uint32(b[1:])
	b = b[5:]
	var out []project.File
	for range count {
		if len(b) < 12 {
			return nil, fmt.Errorf("%w: truncated file entry", ErrMalformed)
		}
		kind, flags := project.Kind(b[0]), b[1]
		size := binary.BigEndian.Uint64(b[2:])
		n := int(binary.BigEndian.Uint16(b[10:]))
		b = b[12:]
		if kind > project.KindData || flags&^flagMain != 0 || size > math.MaxInt64 ||
			n == 0 || len(b) < n || !utf8.Valid(b[:n]) {
			return nil, fmt.Errorf("%w: bad file entry", ErrMalformed)
		}
		out = append(out, project.File{
			Path: string(b[:n]), Kind: kind, Size: int64(size), Main: flags&flagMain != 0, // #nosec G115 -- checked.
		})
		b = b[n:]
	}
	if len(b) != 0 {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrMalformed, len(b))
	}
	return out, nil
}

// DecodeRange parses the payload built by [EncodeRange]. The returned data
// alias b. It returns an error wrapping [ErrMalformed] on any inconsistency.
func DecodeRange(b []byte) ([]store.Entry, error) {
	if len(b) < 4 {
		return nil, fmt.Errorf("%w: truncated range", ErrMalformed)
	}
	count := binary.BigEndian.Uint32(b)
	b = b[4:]
	var out []store.Entry
	for range count {
		key, rest, err := parseKey(b, store.MaxDepth)
		if err != nil || len(rest) < 4 {
			return nil, fmt.Errorf("%w: bad range entry", ErrMalformed)
		}
		n := binary.BigEndian.Uint32(rest)
		if uint64(n) > uint64(len(rest)-4) { // #nosec G115 -- len(rest) >= 4 is checked just above.
			return nil, fmt.Errorf("%w: truncated range entry", ErrMalformed)
		}
		out = append(out, store.Entry{Key: key, Data: rest[4 : 4+n]})
		b = rest[4+n:]
	}
	if len(b) != 0 {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrMalformed, len(b))
	}
	return out, nil
}

// DecodeStoreResponse parses a frame from Go. The returned Payload aliases b.
// The webview has its own decoder; this one serves tests and Go clients.
func DecodeStoreResponse(b []byte) (StoreResponse, error) {
	if len(b) < respHeaderLen || len(b) > maxStoreWire {
		return StoreResponse{}, fmt.Errorf("%w: response of %d bytes", ErrMalformed, len(b))
	}
	r := StoreResponse{ID: binary.BigEndian.Uint32(b), Status: StoreStatus(b[4]), Payload: b[respHeaderLen:]}
	if r.Status > maxStatus {
		return StoreResponse{}, fmt.Errorf("%w: status %d", ErrMalformed, r.Status)
	}
	return r, nil
}

// EncodeStoreRequest serializes r. It returns an error wrapping
// [ErrMalformed] if r is not a valid request. The webview has its own
// encoder; this one serves tests and Go clients.
func EncodeStoreRequest(r StoreRequest) ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	b := make([]byte, 0, storeHeaderLen+len(r.Data)+len(r.Key)*(1+store.MaxComponent))
	b = binary.BigEndian.AppendUint32(b, r.ID)
	b = append(b, byte(r.Op))
	b = appendKey(b, r.Key)
	return append(b, r.Data...), nil
}

// EncodeWriteAt builds the Data of an OpWriteProjectionAt request: the length
// of point as a u16, point, then text. It returns an error wrapping
// [ErrMalformed] if point does not fit a u16.
func EncodeWriteAt(point, text []byte) ([]byte, error) {
	if len(point) > math.MaxUint16 {
		return nil, fmt.Errorf("%w: sync point of %d bytes", ErrMalformed, len(point))
	}
	b := binary.BigEndian.AppendUint16(make([]byte, 0, 2+len(point)+len(text)), uint16(len(point))) // #nosec G115 -- checked above.
	b = append(b, point...)
	return append(b, text...), nil
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
