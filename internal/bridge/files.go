package bridge

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/oioio-space/inkwell/internal/project"
)

// filesVersion is the first byte of the OpListFiles and OpFileInfo payloads. A
// layout change bumps it, so a webview and a Go side of different ages fail
// cleanly instead of misreading each other.
const filesVersion = 1

// Flags of a file list entry and of a file info payload.
const (
	flagMain  = 1 << 0 // list entry: the project's main document
	flagBOM   = 1 << 0 // info: the file starts with a byte order mark
	flagMixed = 1 << 1 // info: the file mixed line endings
)

// EncodeFileList serializes files as the payload of an OpListFiles response:
//
//	version u8 | count u32 | count × (kind u8 | flags u8 | size u64 | len u16 | path)
//
// flags bit 0 is Main. It returns an error wrapping [ErrMalformed] if a file
// is invalid or the result would not fit a frame.
func EncodeFileList(files []project.File) ([]byte, error) {
	size := 1 + 4
	for _, f := range files {
		if f.Size < 0 || f.Path == "" || len(f.Path) > math.MaxUint16 || f.Kind > project.KindData {
			return nil, fmt.Errorf("%w: file entry %q", ErrMalformed, f.Path)
		}
		size += 1 + 1 + 8 + 2 + len(f.Path)
	}
	if size > maxStoreWire-respHeaderLen {
		return nil, fmt.Errorf("%w: file list of %d bytes", ErrMalformed, size)
	}
	b := make([]byte, 0, size)
	b = append(b, filesVersion)
	b = binary.BigEndian.AppendUint32(b, uint32(len(files))) // #nosec G115 -- size bounds the count.
	for _, f := range files {
		var flags byte
		if f.Main {
			flags |= flagMain
		}
		b = append(b, byte(f.Kind), flags)
		b = binary.BigEndian.AppendUint64(b, uint64(f.Size))      // #nosec G115 -- checked ≥ 0.
		b = binary.BigEndian.AppendUint16(b, uint16(len(f.Path))) // #nosec G115 -- checked ≤ MaxUint16.
		b = append(b, f.Path...)
	}
	return b, nil
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

// FileInfo says how a text file is stored on disk, for the banner and the
// conversion offer. The webview itself always sees UTF-8 with LF.
type FileInfo struct {
	// Encoding is one of the project.Encoding constants.
	Encoding string
	// EOL is the line ending Inkwell writes back: LF, CRLF or CR, never Mixed.
	EOL project.EOL
	// BOM is true if the file starts with a byte order mark.
	BOM bool
	// Mixed is true if the file mixed line endings, which saving unifies
	// into EOL.
	Mixed bool
}

// InfoOf describes how t is stored.
func InfoOf(t project.Text) FileInfo {
	return FileInfo{Encoding: t.Encoding, EOL: t.LineEnding(), BOM: t.BOM, Mixed: t.MixedEOL()}
}

// EncodeFileInfo serializes i as the payload of an OpFileInfo response:
//
//	version u8 | flags u8 | eol u8 | len u8 | encoding
//
// flags bit 0 is BOM and bit 1 Mixed; eol is 0 LF, 1 CRLF or 2 CR.
func EncodeFileInfo(i FileInfo) []byte {
	var flags byte
	if i.BOM {
		flags |= flagBOM
	}
	if i.Mixed {
		flags |= flagMixed
	}
	b := []byte{filesVersion, flags, byte(i.EOL), byte(len(i.Encoding))} // #nosec G115 -- the encoding names are short.
	return append(b, i.Encoding...)
}

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
