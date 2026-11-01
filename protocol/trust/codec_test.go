// SPDX-License-Identifier: MPL-2.0

package trust

import (
	"bytes"
	"errors"
	"testing"
)

func sampleOps(t testing.TB) []Op {
	t.Helper()
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	if _, err := l.Issue(a, OpAdd, b.Public(), RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	mustIssue(t, l, a, OpSetRole, b, RoleReader)
	mustIssue(t, l, a, OpSuspend, b, 0)
	mustIssue(t, l, a, OpResume, b, 0)
	mustIssue(t, l, a, OpRemove, b, 0)
	return l.Since(nil)
}

func TestDecodeRejectsMalformedSuspension(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	base := Op{Kind: OpSuspend, Author: a.Public(), Subject: b.Public(), Prev: l.Frontier(), Lamport: 1}
	good := base
	good.sign(a, l.Project())
	if _, err := Decode(Encode(good)); err != nil {
		t.Fatalf("Decode(valid suspension) = %v", err)
	}
	cases := map[string]func(*Op){
		"on itself":    func(o *Op) { o.Subject = o.Author },
		"with a role":  func(o *Op) { o.Role = RoleWriter },
		"with a name":  func(o *Op) { o.Name = "x" },
		"with person":  func(o *Op) { o.Person = Hash{1} },
		"without prev": func(o *Op) { o.Prev, o.Lamport = nil, 0 },
	}
	for _, kind := range []OpKind{OpSuspend, OpResume} {
		for name, mutate := range cases {
			o := base
			o.Kind = kind
			mutate(&o)
			o.sign(a, l.Project())
			if _, err := Decode(Encode(o)); !errors.Is(err, ErrMalformed) {
				t.Errorf("Decode(kind %d %s) = %v, want ErrMalformed", kind, name, err)
			}
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	for _, op := range sampleOps(t) {
		enc := Encode(op)
		if len(enc) > MaxOpSize {
			t.Errorf("len(Encode(kind %d)) = %d, want <= %d", op.Kind, len(enc), MaxOpSize)
		}
		got, err := Decode(enc)
		if err != nil {
			t.Fatalf("Decode(Encode(kind %d)) = %v", op.Kind, err)
		}
		if !bytes.Equal(Encode(got), enc) {
			t.Errorf("kind %d: re-encoding differs", op.Kind)
		}
	}
}

func TestDecodeRejects(t *testing.T) {
	ops := sampleOps(t)
	good := Encode(ops[1])
	cases := map[string][]byte{
		"empty":       nil,
		"truncated":   good[:len(good)-1],
		"trailing":    append(bytes.Clone(good), 0),
		"oversized":   make([]byte, MaxOpSize+1),
		"bad version": append([]byte{9}, good[1:]...),
		"bad kind":    append([]byte{good[0], 99}, good[2:]...),
		"bad role":    append([]byte{good[0], good[1], 0}, good[3:]...),
	}
	for name, b := range cases {
		if _, err := Decode(b); !errors.Is(err, ErrMalformed) {
			t.Errorf("Decode(%s) = %v, want ErrMalformed", name, err)
		}
	}
}

func TestMaxSizeOpFitsLimit(t *testing.T) {
	op := Op{Kind: OpAdd, Role: RoleReader, Lamport: 1, Author: newSigner(1).Public(), Subject: newSigner(2).Public()}
	op.Name = string(bytes.Repeat([]byte("n"), MaxName))
	for i := range MaxPrev {
		op.Prev = append(op.Prev, Hash{byte(i + 1)})
	}
	if n := len(Encode(op)); n > MaxOpSize {
		t.Errorf("len(Encode(max op)) = %d, want <= %d", n, MaxOpSize)
	}
	if err := op.validate(); err != nil {
		t.Errorf("validate(max op) = %v", err)
	}
}

func FuzzDecodeOp(f *testing.F) {
	for _, op := range sampleOps(f) {
		f.Add(Encode(op))
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		op, err := Decode(b)
		if err != nil {
			return
		}
		if !bytes.Equal(Encode(op), b) {
			t.Errorf("Decode accepted a non-canonical encoding: %x", b)
		}
	})
}

func FuzzDecodeAttestation(f *testing.F) {
	w := newSigner(2)
	for _, heads := range [][]Hash{nil, {{1}, {2}}} {
		a, err := NewAttestation(w, ProjectID{7}, "doc", heads, 3)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(EncodeAttestation(a))
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		a, err := DecodeAttestation(b)
		if err != nil {
			return
		}
		if !bytes.Equal(EncodeAttestation(a), b) {
			t.Errorf("DecodeAttestation accepted a non-canonical encoding: %x", b)
		}
	})
}

func FuzzDecodeMessage(f *testing.F) {
	s := NewSync[string](New(ProjectID{}))
	f.Add(s.have("p")[0].Data)
	for _, m := range chunk("p", sampleOps(f)) {
		f.Add(m.Data)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _, _, _ = decodeMessage(b)
	})
}
