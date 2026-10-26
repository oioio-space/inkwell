package project

import (
	"bytes"
	"errors"
	"testing"
)

// The point where the file and the document last agreed is kept with the hash
// it describes: one record, so a restart, a second window or a lost
// localStorage all find the pair intact.
func TestSyncPointKeptWithHash(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if got, err := p.SyncPoint(MainFile); err != nil || got != nil {
		t.Fatalf("SyncPoint before any write = %q, %v; want none", got, err)
	}
	point := []byte(`{"heads":["a1"],"fingerprint":7}`)
	if err := p.WriteProjectionAt(MainFile, []byte("one"), point); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	q, err := Open(base, "p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	if got, err := q.SyncPoint(MainFile); err != nil || !bytes.Equal(got, point) {
		t.Fatalf("SyncPoint after restart = %q, %v; want %q", got, err, point)
	}

	// A write without a point forgets it: it would describe other bytes.
	if err := q.WriteProjection(MainFile, []byte("two")); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.SyncPoint(MainFile); got != nil {
		t.Errorf("SyncPoint after a write without point = %q, want none", got)
	}
	// Same bytes: no rewrite of the file, but the point is still recorded.
	if err := q.WriteProjectionAt(MainFile, []byte("two"), point); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.SyncPoint(MainFile); !bytes.Equal(got, point) {
		t.Errorf("SyncPoint after a write of the same bytes = %q, want %q", got, point)
	}
	if err := base.WriteFile("p/main.typ", []byte("theirs"), 0o600); err != nil {
		t.Fatal(err)
	}
	next := []byte(`{"heads":["b2"],"fingerprint":9}`)
	if err := q.AcknowledgeAt(MainFile, next); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.SyncPoint(MainFile); !bytes.Equal(got, next) {
		t.Errorf("SyncPoint after AcknowledgeAt = %q, want %q", got, next)
	}
	if err := q.Acknowledge(MainFile); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.SyncPoint(MainFile); got != nil {
		t.Errorf("SyncPoint after Acknowledge = %q, want none", got)
	}
}

func TestSyncPointRefused(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	for _, bad := range [][]byte{[]byte("not json"), bytes.Repeat([]byte("1"), MaxSyncPoint+1)} {
		if err := p.WriteProjectionAt(MainFile, []byte("x"), bad); !errors.Is(err, ErrSyncPoint) {
			t.Errorf("WriteProjectionAt(point of %d bytes) = %v, want ErrSyncPoint", len(bad), err)
		}
		if err := p.AcknowledgeAt(MainFile, bad); !errors.Is(err, ErrSyncPoint) {
			t.Errorf("AcknowledgeAt(point of %d bytes) = %v, want ErrSyncPoint", len(bad), err)
		}
	}
	if _, err := p.SyncPoint("other.typ"); !errors.Is(err, ErrNotOwned) {
		t.Errorf("SyncPoint(not owned) = %v, want ErrNotOwned", err)
	}
}

// SetSyncPoint records a point for a file the project has not touched since:
// refused (and nothing recorded) when the file changed outside, as the point
// would then describe other bytes than the hash does.
func TestSetSyncPoint(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("mine")); err != nil {
		t.Fatal(err)
	}
	point := []byte(`{"heads":["a"],"fingerprint":1}`)
	if err := p.SetSyncPoint(MainFile, point); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.SyncPoint(MainFile); !bytes.Equal(got, point) {
		t.Errorf("SyncPoint = %q, want %q", got, point)
	}
	if err := base.WriteFile("p/main.typ", []byte("theirs"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.SetSyncPoint(MainFile, []byte(`{"heads":["b"],"fingerprint":2}`)); !errors.Is(err, ErrExternalEdit) {
		t.Errorf("SetSyncPoint over an unseen edit = %v, want ErrExternalEdit", err)
	}
	if got, _ := p.SyncPoint(MainFile); !bytes.Equal(got, point) {
		t.Errorf("SyncPoint after a refusal = %q, want the old %q", got, point)
	}
	if err := p.SetSyncPoint(MainFile, []byte("nope")); !errors.Is(err, ErrSyncPoint) {
		t.Errorf("SetSyncPoint(not JSON) = %v, want ErrSyncPoint", err)
	}
	if err := p.SetSyncPoint("other.typ", point); !errors.Is(err, ErrNotOwned) {
		t.Errorf("SetSyncPoint(not owned) = %v, want ErrNotOwned", err)
	}
}
