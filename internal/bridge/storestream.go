package bridge

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
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
	// OpAcknowledge marks the external edit of the project file Key[0] as seen.
	OpAcknowledge
	// OpListFiles takes no key and returns the project's files, as a payload
	// decoded by [DecodeFileList].
	OpListFiles
	// OpFileInfo returns how the project file Key[0] is stored on disk
	// (encoding, BOM, line ending), as a payload decoded by [DecodeFileInfo].
	// OpReadProjection and OpWriteProjection always speak UTF-8 with LF.
	OpFileInfo
)

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
)

// maxStatus is the last status a frame may carry.
const maxStatus = StatusInvalidText

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
	case OpWriteProjection, OpReadProjection, OpAcknowledge, OpFileInfo:
		if len(r.Key) != 1 {
			err = fmt.Errorf("%d components, want 1", len(r.Key))
		} else {
			err = store.ValidKey(r.Key)
		}
	default:
		err = fmt.Errorf("op %d", r.Op)
	}
	switch {
	case err != nil:
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	case r.ID == 0:
		return fmt.Errorf("%w: request id 0", ErrMalformed)
	case r.Op != OpSave && r.Op != OpWriteProjection && len(r.Data) > 0:
		return fmt.Errorf("%w: data on op %d", ErrMalformed, r.Op)
	case len(r.Data) > store.MaxValue:
		return fmt.Errorf("%w: %d bytes of data", ErrMalformed, len(r.Data))
	}
	return nil
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
func parseKey(b []byte) (key []string, rest []byte, err error) {
	if len(b) < 1 {
		return nil, nil, errors.New("truncated key")
	}
	n := int(b[0])
	if n > store.MaxDepth {
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
	key, rest, err := parseKey(b[5:])
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
		key, rest, err := parseKey(b)
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
	ReadProjection(rel string) ([]byte, error)
	Acknowledge(rel string) error
	ReadText(rel string) (project.Text, error)
	List() ([]project.File, error)
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
		err = s.st.WriteProjection(req.Key[0], req.Data)
	case OpReadProjection:
		payload, err = s.st.ReadProjection(req.Key[0])
	case OpListFiles:
		payload, err = s.listFiles()
	case OpFileInfo:
		payload, err = s.fileInfo(req.Key[0])
	case OpAcknowledge:
		if err = s.st.Acknowledge(req.Key[0]); err == nil {
			s.mu.Lock()
			delete(s.pending, req.Key[0])
			s.mu.Unlock()
		}
	}
	return s.respond(req, payload, err)
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
