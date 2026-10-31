package bridge

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"strings"
	"sync"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
)

// StoreStreamName is the name of the second Wails stream, which carries the
// webview's storage requests (ADR 0020). It is separate from [StreamName] so
// a large chunk being saved never delays a network frame, and it avoids the
// base64 inflation of a bound []byte.
const StoreStreamName = "store"

// Wire format of the store stream. All integers are big-endian.
//
// Request, webview to Go:
//
//	id u32 | op u8 | nkey u8 | nkey × (len u8 | component) | data
//
// Response, Go to webview:
//
//	id u32 | status u8 | payload
//
// id is chosen by the webview (never 0) and echoed by the response. Requests
// are processed one at a time in arrival order, so a Load sent after a Save
// sees it. Go may also push a response with id 0 and [StatusExternal].
//
// The projection operations (and OpPutBlob) name a file of the project by its
// path, one key component per folder, up to [project.MaxDepth] of them; the
// others have storage keys of up to 4 components.
const (
	storeHeaderLen = 4 + 1 + 1
	respHeaderLen  = 4 + 1
	// maxStoreWire bounds a whole frame (the Wails stream frame limit).
	maxStoreWire = maxWire
)

// StoreOp says what a store request asks for.
type StoreOp uint8

// The operations. Key is the storage key, except for the projection
// operations where it holds the single file name.
const (
	// OpLoad returns the value of Key (StatusNotFound if absent).
	OpLoad StoreOp = iota + 1
	// OpSave stores Data under Key.
	OpSave
	// OpRemove deletes Key.
	OpRemove
	// OpLoadRange returns every value whose key starts with Key (maybe
	// empty), as a payload decoded by [DecodeRange].
	OpLoadRange
	// OpRemoveRange deletes every value whose key starts with Key.
	OpRemoveRange
	// OpWriteProjection writes Data to the project file named by Key[0].
	OpWriteProjection
	// OpReadProjection returns the project file named by Key[0].
	OpReadProjection
	// OpAcknowledge marks the external edit of the project file Key[0] as
	// seen. Its Data, if any, is the sync point to record with it; without
	// Data the file has none.
	OpAcknowledge
	// OpListFiles takes no key and returns the project's files, as a payload
	// decoded by [DecodeFileList].
	OpListFiles
	// OpFileInfo returns how the project file Key[0] is stored on disk
	// (encoding, BOM, line ending), as a payload decoded by [DecodeFileInfo].
	// OpReadProjection and OpWriteProjection always speak UTF-8 with LF.
	OpFileInfo
	// OpTree takes no key; its Data is the project tree the webview
	// materialized from the index, built by [EncodeTree]. Go projects it:
	// folders, the text files it lists, and the visible copies of the blobs.
	// The response payload is decoded by [DecodeTreeResult].
	OpTree
	// OpReferenced takes no key; its Data lists the hashes the index, the
	// trash and the packages refer to, built by [EncodeReferenced]. Go serves
	// those blobs to the members and keeps them.
	OpReferenced
	// OpPutBlob stores the project file named by the Key in the blob store
	// and returns its hash and size, as a payload decoded by [DecodeBlobInfo]:
	// how a file added from outside, or replaced on disk, gets a hash.
	OpPutBlob
	// OpWriteProjectionAt is OpWriteProjection that also records a sync point
	// with the hash it writes. Data is built by [EncodeWriteAt].
	OpWriteProjectionAt
	// OpReadSyncPoint returns the sync point recorded for the project file
	// Key[0] (StatusNotFound if none): the opaque bytes the webview gave to
	// OpWriteProjectionAt or OpAcknowledge.
	OpReadSyncPoint
	// OpSetSyncPoint records Data as the sync point of the project file Key[0]
	// without writing it; refused (StatusEditedOutside) if the file changed.
	OpSetSyncPoint
)

// isPathOp reports whether op names a project file by a path in its key.
func isPathOp(op StoreOp) bool {
	switch op {
	case OpWriteProjection, OpReadProjection, OpAcknowledge, OpFileInfo, OpPutBlob, OpWriteProjectionAt, OpReadSyncPoint, OpSetSyncPoint:
		return true
	}
	return false
}

// StoreStatus is the outcome of a store request.
type StoreStatus uint8

// The statuses.
const (
	// StatusOK means success; the payload is the result, if any.
	StatusOK StoreStatus = iota
	// StatusNotFound means the key or file does not exist.
	StatusNotFound
	// StatusCorrupt means the value was damaged and could not be recovered.
	StatusCorrupt
	// StatusError means the request failed; the payload is a short reason.
	StatusError
	// StatusExternal is pushed with id 0: the payload is the name of a
	// project file that was edited outside Inkwell.
	StatusExternal
	// StatusBinary means the project file is not text and was not decoded.
	StatusBinary
	// StatusInvalidText means the project file is UTF-8 apart from invalid
	// bytes; the payload is the 1-based line of the first one, as a u32.
	StatusInvalidText
	// StatusEditedOutside means a write was refused because the project file
	// was edited outside Inkwell and the edit is not yet acknowledged.
	StatusEditedOutside
	// StatusTooLarge means the file is over the size limit of a blob; the
	// payload is the limit in bytes, as a u64.
	StatusTooLarge
)

// maxStatus is the last status a frame may carry.
const maxStatus = StatusTooLarge

// StoreRequest is one decoded request of the store stream.
type StoreRequest struct {
	// Key is the storage key, or the file name for projection operations.
	Key []string
	// Data is the value for OpSave and OpWriteProjection; empty otherwise.
	Data []byte
	// ID is the request number the response echoes; never 0.
	ID uint32
	// Op is the requested operation.
	Op StoreOp
}

// StoreResponse is one frame from Go on the store stream.
type StoreResponse struct {
	// Payload is the result of the request, or a reason on StatusError.
	Payload []byte
	// ID echoes the request; 0 for a push.
	ID uint32
	// Status is the outcome.
	Status StoreStatus
}

// validate checks the invariants shared by encoding and decoding a request.
func (r StoreRequest) validate() error {
	var err error
	switch r.Op {
	case OpLoad, OpSave, OpRemove, OpRemoveRange:
		err = store.ValidKey(r.Key)
	case OpLoadRange:
		err = store.ValidPrefix(r.Key)
	case OpListFiles:
		if len(r.Key) != 0 {
			err = fmt.Errorf("%d components, want 0", len(r.Key))
		}
	case OpWriteProjection, OpReadProjection, OpAcknowledge, OpFileInfo, OpPutBlob, OpWriteProjectionAt, OpReadSyncPoint, OpSetSyncPoint:
		err = validPathKey(r.Key)
	case OpTree, OpReferenced:
		if len(r.Key) != 0 {
			err = fmt.Errorf("%d components, want 0", len(r.Key))
		}
	default:
		err = fmt.Errorf("op %d", r.Op)
	}
	switch {
	case err != nil:
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	case r.ID == 0:
		return fmt.Errorf("%w: request id 0", ErrMalformed)
	case r.Op != OpSave && r.Op != OpWriteProjection && r.Op != OpWriteProjectionAt && r.Op != OpSetSyncPoint && r.Op != OpAcknowledge && r.Op != OpTree && r.Op != OpReferenced && len(r.Data) > 0:
		return fmt.Errorf("%w: data on op %d", ErrMalformed, r.Op)
	case len(r.Data) > store.MaxValue:
		return fmt.Errorf("%w: %d bytes of data", ErrMalformed, len(r.Data))
	}
	return nil
}

// validPathKey checks a key that names a file of the project: one component
// per folder, and a path that [project.ValidatePath] accepts.
func validPathKey(key []string) error {
	if len(key) == 0 || len(key) > project.MaxDepth {
		return fmt.Errorf("%d components, want 1 to %d", len(key), project.MaxDepth)
	}
	return project.ValidatePath(strings.Join(key, "/"))
}

// appendKey appends nkey and the length-prefixed components; the key has
// been validated, so every length fits a byte.
func appendKey(b []byte, key []string) []byte {
	b = append(b, byte(len(key))) // #nosec G115 -- ValidPrefix bounds len(key) ≤ 4.
	for _, c := range key {
		b = append(b, byte(len(c))) // #nosec G115 -- validComponent bounds len(c) ≤ 128.
		b = append(b, c...)
	}
	return b
}

// parseKey reads nkey and the components from the front of b.
func parseKey(b []byte, maxDepth int) (key []string, rest []byte, err error) {
	if len(b) < 1 {
		return nil, nil, errors.New("truncated key")
	}
	n := int(b[0])
	if n > maxDepth {
		return nil, nil, fmt.Errorf("%d components", n)
	}
	b = b[1:]
	key = make([]string, 0, n)
	for range n {
		if len(b) < 1 || len(b) < 1+int(b[0]) {
			return nil, nil, errors.New("truncated key")
		}
		l := int(b[0])
		key = append(key, string(b[1:1+l]))
		b = b[1+l:]
	}
	return key, b, nil
}

// DecodeStoreRequest parses a frame received from the webview. The returned
// Data aliases b. It returns an error wrapping [ErrMalformed] for a truncated
// or oversized frame, an unknown op, an invalid key, id 0, or data on an op
// that takes none. The ID is still returned (with the error) when the frame
// is long enough to carry it, so the caller can answer.
func DecodeStoreRequest(b []byte) (StoreRequest, error) {
	if len(b) > maxStoreWire {
		return StoreRequest{}, fmt.Errorf("%w: %d bytes", ErrMalformed, len(b))
	}
	if len(b) < storeHeaderLen {
		return StoreRequest{}, fmt.Errorf("%w: truncated header", ErrMalformed)
	}
	r := StoreRequest{ID: binary.BigEndian.Uint32(b), Op: StoreOp(b[4])}
	maxDepth := store.MaxDepth
	if isPathOp(r.Op) {
		maxDepth = project.MaxDepth
	}
	key, rest, err := parseKey(b[5:], maxDepth)
	if err != nil {
		return StoreRequest{ID: r.ID}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	r.Key, r.Data = key, rest
	if err := r.validate(); err != nil {
		return StoreRequest{ID: r.ID}, err
	}
	return r, nil
}

// EncodeStoreResponse serializes r into a slice the caller may hand over.
func EncodeStoreResponse(r StoreResponse) ([]byte, error) {
	if r.Status > maxStatus || len(r.Payload) > maxStoreWire-respHeaderLen {
		return nil, fmt.Errorf("%w: response status %d, %d bytes", ErrMalformed, r.Status, len(r.Payload))
	}
	b := make([]byte, 0, respHeaderLen+len(r.Payload))
	b = binary.BigEndian.AppendUint32(b, r.ID)
	b = append(b, byte(r.Status))
	return append(b, r.Payload...), nil
}

// EncodeRange serializes entries as the payload of an OpLoadRange response:
// count u32, then per entry nkey u8 | components | len(data) u32 | data.
// It returns an error wrapping [ErrMalformed] if the result would not fit a frame.
func EncodeRange(entries []store.Entry) ([]byte, error) {
	size := 4
	for _, e := range entries {
		size += 1 + len(e.Key) + 4 + len(e.Data)
		for _, c := range e.Key {
			size += len(c)
		}
	}
	if size > maxStoreWire-respHeaderLen {
		return nil, fmt.Errorf("%w: range of %d bytes", ErrMalformed, size)
	}
	b := make([]byte, 0, size)
	b = binary.BigEndian.AppendUint32(b, uint32(len(entries))) // #nosec G115 -- size bounds the count.
	for _, e := range entries {
		b = appendKey(b, e.Key)
		b = binary.BigEndian.AppendUint32(b, uint32(len(e.Data))) // #nosec G115 -- ≤ store.MaxValue.
		b = append(b, e.Data...)
	}
	return b, nil
}

// Storage is what the store stream serves: a verified key-value store and
// the project files Inkwell owns. [project.Project] implements it.
// ReadProjection and WriteProjection exchange UTF-8 text with LF line ends;
// ReadText says how the file is encoded on disk and List lists the folder.
type Storage interface {
	Load(key []string) ([]byte, error)
	Save(key []string, data []byte) error
	Remove(key []string) error
	LoadRange(prefix []string) ([]store.Entry, error)
	RemoveRange(prefix []string) error
	WriteProjection(rel string, data []byte) error
	// WriteProjectionAt also records point (opaque; nil for none) with the hash.
	WriteProjectionAt(rel string, data, point []byte) error
	ReadProjection(rel string) ([]byte, error)
	Acknowledge(rel string) error
	AcknowledgeAt(rel string, point []byte) error
	// SetSyncPoint records point without writing the file.
	SetSyncPoint(rel string, point []byte) error
	// SyncPoint returns the point recorded with the hash of rel, nil if none.
	SyncPoint(rel string) ([]byte, error)
	ReadText(rel string) (project.Text, error)
	List() ([]project.File, error)
	// SetTree projects the project tree of the webview (see
	// [project.Project.ApplyTree]) and returns how many entries it refused.
	SetTree(entries []project.TreeEntry) (rejected int, err error)
	// SetReferenced records the blobs the project refers to.
	SetReferenced(hashes [][HashSize]byte) error
	// PutBlob stores the project file rel in the blob store. It returns a
	// [*TooLargeError] for a file over the limit of a blob.
	PutBlob(rel string) (hash [HashSize]byte, size int64, err error)
}

// StoreServer serves one [Storage] to the webview over the store stream.
// Create one with [NewStoreServer].
type StoreServer struct {
	st      Storage
	log     *slog.Logger
	mu      sync.Mutex
	cur     *session
	pending map[string]struct{} // external edits the webview has not acknowledged
}

// NewStoreServer returns a server over st that logs to log. Document bytes
// are never logged.
func NewStoreServer(st Storage, log *slog.Logger) *StoreServer {
	return &StoreServer{st: st, log: log, pending: map[string]struct{}{}}
}

// NotifyExternal tells the webview that the project file rel was edited
// outside Inkwell. The notice is kept, and sent again to every new connection
// (a page reload), until the webview acknowledges it. It is safe to call from
// any goroutine and matches the callback of [project.Project.Watch].
func (s *StoreServer) NotifyExternal(rel string) {
	s.mu.Lock()
	s.pending[rel] = struct{}{}
	cur := s.cur
	s.mu.Unlock()
	if cur != nil {
		s.send(cur, StoreResponse{Status: StatusExternal, Payload: []byte(rel)})
	}
}

// send writes r to cur, logging a failure.
func (s *StoreServer) send(cur *session, r StoreResponse) {
	cur.mu.Lock()
	defer cur.mu.Unlock()
	s.sendLocked(cur, r)
}

func (s *StoreServer) sendLocked(cur *session, r StoreResponse) {
	b, err := EncodeStoreResponse(r)
	if err != nil {
		s.log.Error("store stream: encode response", "status", r.Status, "err", err)
		return
	}
	if err := cur.c.Send(b); err != nil {
		s.log.Warn("store stream: send to webview", "status", r.Status, "err", err)
	}
}

// Serve attaches c, replacing any previous connection (a page reload), sends
// it the pending external-edit notices, then answers its requests one at a
// time until c fails or is replaced. A malformed request is logged without
// its data, answered with StatusError when it carries an id, and skipped.
func (s *StoreServer) Serve(c Conn) {
	cur := &session{c: c}
	cur.mu.Lock() // pushes wait until the pending notices are out
	s.mu.Lock()
	s.cur = cur
	notices := make([]string, 0, len(s.pending))
	for rel := range s.pending {
		notices = append(notices, rel)
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.cur == cur {
			s.cur = nil
		}
		s.mu.Unlock()
	}()
	for _, rel := range notices {
		s.sendLocked(cur, StoreResponse{Status: StatusExternal, Payload: []byte(rel)})
	}
	cur.mu.Unlock()

	for {
		b, err := c.Receive()
		if err != nil {
			return
		}
		s.mu.Lock()
		replaced := s.cur != cur
		s.mu.Unlock()
		if replaced {
			return
		}
		req, err := DecodeStoreRequest(b)
		if err != nil {
			s.log.Warn("store stream: dropped request", "bytes", len(b), "err", err)
			if req.ID != 0 {
				s.send(cur, StoreResponse{ID: req.ID, Status: StatusError, Payload: []byte("malformed request")})
			}
			continue
		}
		s.send(cur, s.handle(req))
	}
}

// handle runs one request.
func (s *StoreServer) handle(req StoreRequest) StoreResponse {
	var (
		payload []byte
		err     error
	)
	rel := strings.Join(req.Key, "/") // for the operations that name a file
	switch req.Op {
	case OpLoad:
		payload, err = s.st.Load(req.Key)
	case OpSave:
		err = s.st.Save(req.Key, req.Data)
	case OpRemove:
		err = s.st.Remove(req.Key)
	case OpLoadRange:
		payload, err = s.loadRange(req.Key)
	case OpRemoveRange:
		err = s.st.RemoveRange(req.Key)
	case OpWriteProjection:
		err = s.st.WriteProjection(rel, req.Data)
	case OpReadProjection:
		payload, err = s.st.ReadProjection(rel)
	case OpListFiles:
		payload, err = s.listFiles()
	case OpFileInfo:
		payload, err = s.fileInfo(rel)
	case OpWriteProjectionAt:
		err = s.writeAt(rel, req.Data)
	case OpSetSyncPoint:
		err = s.st.SetSyncPoint(rel, req.Data)
	case OpReadSyncPoint:
		if payload, err = s.st.SyncPoint(rel); err == nil && len(payload) == 0 {
			err = fs.ErrNotExist
		}
	case OpAcknowledge:
		if len(req.Data) == 0 {
			err = s.st.Acknowledge(rel)
		} else {
			err = s.st.AcknowledgeAt(rel, req.Data)
		}
		if err == nil {
			s.mu.Lock()
			delete(s.pending, rel)
			s.mu.Unlock()
		}
	case OpTree:
		payload, err = s.setTree(req.Data)
	case OpReferenced:
		err = s.setReferenced(req.Data)
	case OpPutBlob:
		var (
			hash [HashSize]byte
			size int64
		)
		if hash, size, err = s.st.PutBlob(rel); err == nil {
			payload = EncodeBlobInfo(hash, size)
		}
	}
	return s.respond(req, payload, err)
}

// writeAt runs OpWriteProjectionAt.
func (s *StoreServer) writeAt(rel string, data []byte) error {
	point, text, err := DecodeWriteAt(data)
	if err != nil {
		return err
	}
	if len(point) == 0 {
		point = nil
	}
	return s.st.WriteProjectionAt(rel, text, point)
}

// DecodeWriteAt parses the Data built by [EncodeWriteAt]; the results alias
// data. It returns an error wrapping [ErrMalformed] for a truncated one.
func DecodeWriteAt(data []byte) (point, text []byte, err error) {
	if len(data) < 2 {
		return nil, nil, fmt.Errorf("%w: truncated write", ErrMalformed)
	}
	n := int(binary.BigEndian.Uint16(data))
	if len(data)-2 < n {
		return nil, nil, fmt.Errorf("%w: sync point of %d bytes in a frame of %d", ErrMalformed, n, len(data))
	}
	return data[2 : 2+n], data[2+n:], nil
}

// setTree runs OpTree.
func (s *StoreServer) setTree(data []byte) ([]byte, error) {
	entries, err := DecodeTree(data)
	if err != nil {
		return nil, err
	}
	rejected, err := s.st.SetTree(entries)
	if err != nil {
		return nil, err
	}
	if rejected > 0 {
		s.log.Warn("store stream: tree entries refused", "rejected", rejected, "of", len(entries))
	}
	return EncodeTreeResult(rejected), nil
}

// setReferenced runs OpReferenced.
func (s *StoreServer) setReferenced(data []byte) error {
	hashes, err := DecodeReferenced(data)
	if err != nil {
		return err
	}
	return s.st.SetReferenced(hashes)
}

// listFiles runs OpListFiles.
func (s *StoreServer) listFiles() ([]byte, error) {
	files, err := s.st.List()
	if err != nil {
		return nil, err
	}
	return EncodeFileList(files)
}

// fileInfo runs OpFileInfo.
func (s *StoreServer) fileInfo(rel string) ([]byte, error) {
	t, err := s.st.ReadText(rel)
	if err != nil {
		return nil, err
	}
	return EncodeFileInfo(InfoOf(t)), nil
}

// loadRange runs OpLoadRange. Entries that could not be recovered are left
// out and logged; the rest are returned.
func (s *StoreServer) loadRange(prefix []string) ([]byte, error) {
	entries, err := s.st.LoadRange(prefix)
	if err != nil && !errors.Is(err, store.ErrCorrupt) {
		return nil, err
	}
	if err != nil {
		s.log.Warn("store stream: unrecoverable values left out of a range", "prefix", prefix)
	}
	return EncodeRange(entries)
}

// respond maps err to a status. Messages carry no document bytes.
func (s *StoreServer) respond(req StoreRequest, payload []byte, err error) StoreResponse {
	switch {
	case err == nil:
		return StoreResponse{ID: req.ID, Status: StatusOK, Payload: payload}
	case errors.Is(err, store.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return StoreResponse{ID: req.ID, Status: StatusNotFound}
	case errors.Is(err, store.ErrCorrupt):
		s.log.Warn("store stream: corrupt value", "op", req.Op, "key", req.Key)
		return StoreResponse{ID: req.ID, Status: StatusCorrupt}
	case errors.Is(err, project.ErrBinary):
		return StoreResponse{ID: req.ID, Status: StatusBinary}
	case errors.Is(err, project.ErrExternalEdit):
		return StoreResponse{ID: req.ID, Status: StatusEditedOutside}
	case errors.Is(err, ErrMalformed):
		return StoreResponse{ID: req.ID, Status: StatusError, Payload: []byte("malformed request")}
	}
	if tl, ok := errors.AsType[*TooLargeError](err); ok {
		return StoreResponse{ID: req.ID, Status: StatusTooLarge, Payload: binary.BigEndian.AppendUint64(nil, uint64(max(tl.Limit, 0)))} // #nosec G115 -- clamped.
	}
	if bad, ok := errors.AsType[*project.InvalidUTF8Error](err); ok {
		line := binary.BigEndian.AppendUint32(nil, uint32(min(bad.Line, math.MaxUint32))) // #nosec G115 -- clamped.
		return StoreResponse{ID: req.ID, Status: StatusInvalidText, Payload: line}
	}
	s.log.Warn("store stream: request failed", "op", req.Op, "key", req.Key, "err", err)
	return StoreResponse{ID: req.ID, Status: StatusError, Payload: []byte(shorten(err.Error()))}
}

// shorten bounds an error message sent to the webview.
func shorten(s string) string {
	const limit = 200
	if len(s) > limit {
		return s[:limit]
	}
	return s
}
