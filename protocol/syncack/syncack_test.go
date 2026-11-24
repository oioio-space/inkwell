// SPDX-License-Identifier: MPL-2.0

package syncack_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"testing/iotest"
	"time"

	"github.com/oioio-space/inkwell/protocol/syncack"
	"github.com/oioio-space/inkwell/protocol/vectors"
)

// P-4 (spec §8.7.2): the ACK is exactly one byte 0x01; any other first byte is a violation, an end
// or a reset before it a refusal; ReadAck reads one byte and no more.
func TestReadAck(t *testing.T) {
	refused := errors.New("stream reset")
	for _, c := range []struct {
		name string
		r    io.Reader
		want error
		left int // bytes left unread
	}{
		{"ack", bytes.NewReader([]byte{0x01}), nil, 0},
		{"ack then more", bytes.NewReader([]byte{0x01, 0x01, 0x00}), nil, 2},
		{"eof", bytes.NewReader(nil), syncack.ErrRefused, 0},
		{"reset", iotest.ErrReader(refused), syncack.ErrRefused, 0},
		{"zero", bytes.NewReader([]byte{0x00}), syncack.ErrViolation, 0},
		{"two", bytes.NewReader([]byte{0x02}), syncack.ErrViolation, 0},
		{"ff", bytes.NewReader([]byte{0xff}), syncack.ErrViolation, 0},
		{"frame first", bytes.NewReader([]byte{0x03, 0x00, 'a', 'b', 'c'}), syncack.ErrViolation, 4},
		{"one byte at a time", iotest.OneByteReader(bytes.NewReader([]byte{0x01, 0x02})), nil, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := syncack.ReadAck(c.r)
			if !errors.Is(err, c.want) || (err == nil) != (c.want == nil) {
				t.Fatalf("ReadAck = %v, want %v", err, c.want)
			}
			if rest, _ := io.ReadAll(c.r); len(rest) != c.left {
				t.Errorf("ReadAck left %d bytes, want %d", len(rest), c.left)
			}
		})
	}
	// The cause of a refusal is kept.
	if err := syncack.ReadAck(iotest.ErrReader(refused)); !errors.Is(err, refused) {
		t.Errorf("ReadAck on a reset = %v, want it to wrap %v", err, refused)
	}
	if err := syncack.ReadAck(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Errorf("ReadAck on an end = %v, want it to wrap io.EOF", err)
	}
}

// P-4: the responder writes exactly the byte 0x01, which ReadAck accepts.
func TestWriteAck(t *testing.T) {
	var b bytes.Buffer
	if err := syncack.WriteAck(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes(), []byte{syncack.Ack}) || syncack.Ack != 0x01 {
		t.Fatalf("WriteAck wrote % x, want 01", b.Bytes())
	}
	if err := syncack.ReadAck(&b); err != nil {
		t.Fatalf("ReadAck(WriteAck) = %v", err)
	}
	failed := errors.New("write failed")
	if err := syncack.WriteAck(failWriter{failed}); !errors.Is(err, failed) {
		t.Errorf("WriteAck on a failing writer = %v, want %v", err, failed)
	}
}

type failWriter struct{ err error }

func (w failWriter) Write([]byte) (int, error) { return 0, w.err }

// P-2 (spec §8.7.3): the responder acknowledges only an open node, an admitted peer and a live
// connection; every other combination resets.
func TestResponderAccept(t *testing.T) {
	// want[i] for NodeClosed = i&1, Admitted = i&2, ConnClosed = i&4: only i = 2 acknowledges.
	want := [8]bool{false, false, true, false, false, false, false, false}
	for i := range 8 {
		r := syncack.Responder{NodeClosed: i&1 != 0, Admitted: i&2 != 0, ConnClosed: i&4 != 0}
		if got := r.Accept(); got != want[i] {
			t.Errorf("%+v.Accept() = %v, want %v", r, got, want[i])
		}
	}
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

type step struct {
	e    syncack.Event
	want syncack.Outcome
}

func run(t *testing.T, o *syncack.Opener, steps ...step) {
	t.Helper()
	for i, s := range steps {
		got, err := o.Step(s.e)
		if err != nil {
			t.Fatalf("step %d %+v: %v", i, s.e, err)
		}
		if got != s.want {
			t.Fatalf("step %d %+v = %v, want %v", i, s.e, got, s.want)
		}
	}
}

func ev(op syncack.Op, at int) syncack.Event { return syncack.Event{Op: op, At: ms(at)} }

func byteAt(at int, b byte) syncack.Event {
	return syncack.Event{Op: syncack.OpByte, At: ms(at), Byte: b}
}

// P-1, P-5, P-6 (spec §8.7.4, §8.7.5): the opener's machine.
func TestOpener(t *testing.T) {
	open, ret := ev(syncack.OpOpen, 0), ev(syncack.OpReturn, 0)
	t.Run("up only on the ACK", func(t *testing.T) {
		run(t, syncack.NewOpener(syncack.RoleSession, false),
			step{open, syncack.None}, step{ret, syncack.None}, step{byteAt(100, 0x01), syncack.PeerUp},
			step{byteAt(150, 0x02), syncack.CloseConn})
	})
	t.Run("reopen while announced", func(t *testing.T) {
		run(t, syncack.NewOpener(syncack.RoleSession, true), step{open, syncack.None}, step{ret, syncack.None},
			step{byteAt(10, 0x01), syncack.None}, step{ev(syncack.OpDeadline, 20000), syncack.None})
	})
	t.Run("refusals", func(t *testing.T) {
		for _, op := range []syncack.Op{syncack.OpEOF, syncack.OpReset} {
			run(t, syncack.NewOpener(syncack.RoleSession, false), step{open, syncack.None}, step{ret, syncack.None},
				step{ev(op, 5), syncack.CloseConn})
			run(t, syncack.NewOpener(syncack.RoleMove, true), step{open, syncack.None}, step{ret, syncack.None},
				step{ev(op, 5), syncack.Abandon})
		}
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{open, syncack.None}, step{ret, syncack.None},
			step{byteAt(5, 0x00), syncack.CloseConn})
		run(t, syncack.NewOpener(syncack.RoleMove, true), step{open, syncack.None}, step{ret, syncack.None},
			step{byteAt(5, 0xff), syncack.Abandon})
	})
	t.Run("deadline before the return", func(t *testing.T) {
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{ev(syncack.OpOpen, 7), syncack.None},
			step{ev(syncack.OpDeadline, 15007), syncack.CloseConn})
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{ev(syncack.OpOpen, 7), syncack.None},
			step{ev(syncack.OpReturn, 15007), syncack.CloseConn})
	})
	t.Run("one budget from the open", func(t *testing.T) {
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{ev(syncack.OpOpen, 1000), syncack.None},
			step{ev(syncack.OpReturn, 11000), syncack.None}, step{byteAt(15999, 0x01), syncack.PeerUp})
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{ev(syncack.OpOpen, 1000), syncack.None},
			step{ev(syncack.OpReturn, 11000), syncack.None}, step{byteAt(16000, 0x01), syncack.CloseConn})
		run(t, syncack.NewOpener(syncack.RoleMove, true), step{open, syncack.None}, step{ret, syncack.None},
			step{byteAt(15000, 0x01), syncack.Abandon})
		run(t, syncack.NewOpener(syncack.RoleMove, true), step{open, syncack.None}, step{ret, syncack.None},
			step{byteAt(14999, 0x01), syncack.Swap}, step{byteAt(15000, 0x01), syncack.CloseConn})
	})
	t.Run("connection lost", func(t *testing.T) {
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{open, syncack.None},
			step{ev(syncack.OpConnClosed, 3), syncack.Lost})
		run(t, syncack.NewOpener(syncack.RoleMove, true), step{open, syncack.None}, step{ret, syncack.None},
			step{ev(syncack.OpConnClosed, 3), syncack.Abandon})
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{open, syncack.None}, step{ret, syncack.None},
			step{byteAt(3, 0x01), syncack.PeerUp}, step{ev(syncack.OpConnClosed, 4), syncack.Lost})
		// After the deadline fired, the connection's loss is no news.
		run(t, syncack.NewOpener(syncack.RoleSession, false), step{open, syncack.None},
			step{ev(syncack.OpConnClosed, 15000), syncack.CloseConn})
	})
	t.Run("out of order", func(t *testing.T) {
		for _, steps := range [][]syncack.Event{
			{ret},                   // no open
			{open, open},            // opened twice
			{open, byteAt(1, 0x01)}, // a byte before the return
			{open, ret, ret},        // returned twice
			{open, ev(syncack.OpReturn, 5), ev(syncack.OpReturn, 4)}, // back in time
			{open, ev(syncack.OpDeadline, 14999)},                    // a deadline before its time
			{open, ev(syncack.OpEOF, 1), ev(syncack.OpEOF, 2)},       // an event after the end
			{open, ret, byteAt(1, 0x01), ev(syncack.OpReturn, 2)},    // a return once open
			{open, {Op: syncack.Op(99), At: 1}},                      // an unknown event
		} {
			o := syncack.NewOpener(syncack.RoleSession, false)
			var err error
			for _, e := range steps {
				if _, err = o.Step(e); err != nil {
					break
				}
			}
			if !errors.Is(err, syncack.ErrOrder) {
				t.Errorf("%+v: err %v, want ErrOrder", steps, err)
			}
		}
	})
}

func TestNames(t *testing.T) {
	for o, want := range map[syncack.Outcome]string{
		syncack.None: "none", syncack.PeerUp: "peer_up", syncack.CloseConn: "close_conn", syncack.Lost: "lost",
		syncack.Swap: "swap", syncack.Abandon: "abandon", syncack.Outcome(6): "outcome(6)", syncack.Outcome(99): "outcome(99)",
	} {
		if got := o.String(); got != want {
			t.Errorf("Outcome(%d).String() = %q, want %q", o, got, want)
		}
	}
	for s, want := range map[string]syncack.Op{
		"open": syncack.OpOpen, "return": syncack.OpReturn, "byte": syncack.OpByte, "eof": syncack.OpEOF,
		"reset": syncack.OpReset, "conn_closed": syncack.OpConnClosed, "deadline": syncack.OpDeadline,
	} {
		if got, ok := syncack.ParseOp(s); !ok || got != want {
			t.Errorf("ParseOp(%q) = %v, %v; want %v", s, got, ok, want)
		}
	}
	if _, ok := syncack.ParseOp("ping"); ok {
		t.Error(`ParseOp("ping") accepted`)
	}
	for s, want := range map[string]syncack.Role{"session": syncack.RoleSession, "move": syncack.RoleMove} {
		if got, ok := syncack.ParseRole(s); !ok || got != want {
			t.Errorf("ParseRole(%q) = %v, %v; want %v", s, got, ok, want)
		}
	}
	if _, ok := syncack.ParseRole("relay"); ok {
		t.Error(`ParseRole("relay") accepted`)
	}
}

// FuzzReadAck (spec §8.7.9): no input panics; ReadAck accepts exactly the inputs whose first byte
// is 0x01, refuses the empty one, calls every other a violation, and reads one byte at most.
func FuzzReadAck(f *testing.F) {
	data, err := vectors.FS.ReadFile("syncack.json")
	if err != nil {
		f.Fatal(err)
	}
	var file struct {
		Vectors []struct {
			Input *string `json:"input_hex"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		f.Fatal(err)
	}
	for _, v := range file.Vectors {
		if v.Input != nil {
			b, err := hex.DecodeString(*v.Input)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(b)
		}
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		r := bytes.NewReader(b)
		err := syncack.ReadAck(r)
		switch {
		case len(b) == 0:
			if !errors.Is(err, syncack.ErrRefused) {
				t.Fatalf("empty input: %v, want ErrRefused", err)
			}
		case b[0] == syncack.Ack:
			if err != nil {
				t.Fatalf("% x: %v, want the ACK", b, err)
			}
		default:
			if !errors.Is(err, syncack.ErrViolation) {
				t.Fatalf("% x: %v, want ErrViolation", b, err)
			}
		}
		if read := len(b) - r.Len(); read > 1 {
			t.Fatalf("ReadAck read %d bytes, want at most 1", read)
		}
	})
}
