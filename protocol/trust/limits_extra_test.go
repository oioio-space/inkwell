// SPDX-License-Identifier: MPL-2.0

package trust

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

// hashes returns n distinct, strictly ascending hashes.
func hashes(n int) []Hash {
	hs := make([]Hash, n)
	for i := range hs {
		hs[i] = Hash{byte(i >> 8), byte(i), 1}
	}
	return hs
}

func TestBitsetAcrossWords(t *testing.T) {
	for _, tc := range []struct{ n, words int }{{0, 0}, {1, 1}, {64, 1}, {65, 2}, {128, 2}, {129, 3}, {200, 4}} {
		if got := len(newBitset(tc.n)); got != tc.words {
			t.Errorf("len(newBitset(%d)) = %d words, want %d", tc.n, got, tc.words)
		}
	}
	b := newBitset(200)
	set := []int{0, 1, 63, 64, 65, 127, 128, 199}
	for _, i := range set {
		b.set(i)
	}
	for i := range 200 {
		if got, want := b.has(i), slices.Contains(set, i); got != want {
			t.Errorf("has(%d) = %t, want %t", i, got, want)
		}
	}
	if got := b.positions(); !slices.Equal(got, set) {
		t.Errorf("positions() = %v, want %v", got, set)
	}
	b.unset(64)
	b.unset(2) // not set: no effect
	if want := []int{0, 1, 63, 65, 127, 128, 199}; !slices.Equal(b.positions(), want) {
		t.Errorf("after unset(64) positions() = %v, want %v", b.positions(), want)
	}
	other := newBitset(200)
	for _, i := range []int{5, 65, 190} {
		other.set(i)
	}
	b.or(other)
	if want := []int{0, 1, 5, 63, 65, 127, 128, 190, 199}; !slices.Equal(b.positions(), want) {
		t.Errorf("after or positions() = %v, want %v", b.positions(), want)
	}
	b.andNot(other)
	if want := []int{0, 1, 63, 127, 128, 199}; !slices.Equal(b.positions(), want) {
		t.Errorf("after andNot positions() = %v, want %v", b.positions(), want)
	}
}

func baseOp() Op {
	a, b := newSigner(1), newSigner(2)
	return Op{Kind: OpAdd, Role: RoleWriter, Author: a.Public(), Subject: b.Public(), Lamport: 1, Prev: []Hash{{1}}}
}

// TestOpTextAndParentLimits: every bound is exact, both from the constructor
// side (validate) and from the wire (Decode of a hostile length).
func TestOpTextAndParentLimits(t *testing.T) {
	ok := baseOp()
	ok.Name = strings.Repeat("n", MaxName)
	ok.Prev = hashes(MaxPrev)
	if err := ok.validate(); err != nil {
		t.Fatalf("validate(MaxName, MaxPrev) = %v, want nil", err)
	}
	back, err := Decode(Encode(ok))
	if err != nil || back.Name != ok.Name || len(back.Prev) != MaxPrev {
		t.Fatalf("Decode(max op) = %d-byte name, %d parents, %v; want a lossless round trip", len(back.Name), len(back.Prev), err)
	}
	if n := len(Encode(ok)); n > MaxOpSize {
		t.Errorf("a maximal op encodes to %d bytes, over MaxOpSize %d", n, MaxOpSize)
	}

	for name, mutate := range map[string]func(*Op){
		"name too long":    func(o *Op) { o.Name = strings.Repeat("n", MaxName+1) },
		"name not utf8":    func(o *Op) { o.Name = "\xff\xfe" },
		"too many parents": func(o *Op) { o.Prev = hashes(MaxPrev + 1) },
		"duplicate parent": func(o *Op) { o.Prev = []Hash{{1}, {1}} },
		"unsorted parents": func(o *Op) { o.Prev = []Hash{{2}, {1}} },
		"short author":     func(o *Op) { o.Author = o.Author[:31] },
		"long subject":     func(o *Op) { o.Subject = append(bytes.Clone(o.Subject), 0) },
	} {
		o := baseOp()
		mutate(&o)
		if err := o.validate(); !errors.Is(err, ErrMalformed) {
			t.Errorf("validate(%s) = %v, want ErrMalformed", name, err)
		}
	}

	// A genesis carries the project title and root URL, bounded too.
	a := newSigner(1)
	for _, tc := range []struct {
		name, url string
		wantErr   bool
	}{
		{strings.Repeat("t", MaxName), strings.Repeat("u", MaxRootURL), false},
		{strings.Repeat("t", MaxName+1), "automerge:x", true},
		{"book", strings.Repeat("u", MaxRootURL+1), true},
		{"book", "\xff", true},
	} {
		g, id := Genesis(a, tc.name, tc.url)
		err := New(id).Apply(g)
		if tc.wantErr != errors.Is(err, ErrMalformed) || (!tc.wantErr && err != nil) {
			t.Errorf("Apply(Genesis(%d-byte name, %d-byte url)) = %v, wantErr %t", len(tc.name), len(tc.url), err, tc.wantErr)
		}
	}
}

// TestDecodeHostileLengths patches the length fields of a valid encoding, as a
// hostile peer would: one past the limit is refused, the limit itself is not.
func TestDecodeHostileLengths(t *testing.T) {
	const (
		nameLenAt = 3 + 8 + 3*keyLen // version, kind, role, lamport, author, subject, person
		nameAt    = nameLenAt + 2
	)
	o := baseOp()
	o.Name = strings.Repeat("n", MaxName)
	o.Prev = hashes(MaxPrev)
	valid := Encode(o)
	if _, err := Decode(valid); err != nil {
		t.Fatalf("Decode(valid) = %v", err)
	}

	longName := bytes.Clone(valid)
	longName[nameLenAt], longName[nameLenAt+1] = 0, MaxName+1
	longName = slices.Insert(longName, nameAt+MaxName, 'n')
	if _, err := Decode(longName); !errors.Is(err, ErrMalformed) {
		t.Errorf("Decode(name of %d bytes) = %v, want ErrMalformed", MaxName+1, err)
	}

	body := len(valid) - len(o.Sig)
	prevCountAt := body - MaxPrev*len(Hash{}) - 1
	if valid[prevCountAt] != MaxPrev {
		t.Fatalf("test layout is off: parent count byte = %d", valid[prevCountAt])
	}
	manyParents := bytes.Clone(valid)
	manyParents[prevCountAt] = MaxPrev + 1
	manyParents = slices.Insert(manyParents, body, bytes.Repeat([]byte{0xff}, len(Hash{}))...)
	if _, err := Decode(manyParents); !errors.Is(err, ErrMalformed) {
		t.Errorf("Decode(%d parents) = %v, want ErrMalformed", MaxPrev+1, err)
	}

	// A length that reaches past the end of the buffer.
	short := bytes.Clone(valid)
	short[nameLenAt], short[nameLenAt+1] = 0, MaxName
	if _, err := Decode(short[:nameAt+MaxName-1]); !errors.Is(err, ErrMalformed) {
		t.Errorf("Decode(truncated name) = %v, want ErrMalformed", err)
	}
}

func TestAttestationLimits(t *testing.T) {
	w := newSigner(2)
	l := project(t, newSigner(1))
	good, err := NewAttestation(w, l.Project(), strings.Repeat("d", MaxDoc), hashes(MaxHeads), 1)
	if err != nil {
		t.Fatalf("NewAttestation(MaxDoc, MaxHeads) = %v, want nil", err)
	}
	enc := EncodeAttestation(good)
	if len(enc) > MaxAttestationSize {
		t.Errorf("a maximal attestation encodes to %d bytes, over %d", len(enc), MaxAttestationSize)
	}
	back, err := DecodeAttestation(enc)
	if err != nil || back.Doc != good.Doc || !slices.Equal(back.Heads, good.Heads) {
		t.Fatalf("DecodeAttestation(max) = %d-byte doc, %d heads, %v; want a lossless round trip", len(back.Doc), len(back.Heads), err)
	}

	for name, tc := range map[string]struct {
		doc   string
		heads []Hash
	}{
		"doc too long":  {strings.Repeat("d", MaxDoc+1), nil},
		"doc not utf8":  {"\xff", nil},
		"empty doc":     {"", nil},
		"too many head": {"doc", hashes(MaxHeads + 1)},
	} {
		if _, err := NewAttestation(w, l.Project(), tc.doc, tc.heads, 1); !errors.Is(err, ErrMalformed) {
			t.Errorf("NewAttestation(%s) = %v, want ErrMalformed", name, err)
		}
	}

	// Hostile lengths on the wire.
	const docLenAt = 1 + len(ProjectID{}) + keyLen + 8
	docAt := docLenAt + 2
	longDoc := bytes.Clone(enc)
	longDoc[docLenAt], longDoc[docLenAt+1] = byte((MaxDoc+1)>>8), byte((MaxDoc+1)&0xff)
	longDoc = slices.Insert(longDoc, docAt+MaxDoc, 'd')
	if _, err := DecodeAttestation(longDoc); !errors.Is(err, ErrMalformed) {
		t.Errorf("DecodeAttestation(doc of %d bytes) = %v, want ErrMalformed", MaxDoc+1, err)
	}
	headsLenAt := docAt + MaxDoc
	manyHeads := bytes.Clone(enc)
	manyHeads[headsLenAt], manyHeads[headsLenAt+1] = 0, MaxHeads+1
	manyHeads = slices.Insert(manyHeads, headsLenAt+2+MaxHeads*len(Hash{}), bytes.Repeat([]byte{0xff}, len(Hash{}))...)
	if _, err := DecodeAttestation(manyHeads); !errors.Is(err, ErrMalformed) {
		t.Errorf("DecodeAttestation(%d heads) = %v, want ErrMalformed", MaxHeads+1, err)
	}
	if _, err := DecodeAttestation(enc[:len(enc)-1]); !errors.Is(err, ErrMalformed) {
		t.Errorf("DecodeAttestation(truncated) = %v, want ErrMalformed", err)
	}
	if _, err := DecodeAttestation(append(bytes.Clone(enc), 0)); !errors.Is(err, ErrMalformed) {
		t.Errorf("DecodeAttestation(trailing byte) = %v, want ErrMalformed", err)
	}
	if _, err := DecodeAttestation(append([]byte{9}, enc[1:]...)); !errors.Is(err, ErrMalformed) {
		t.Errorf("DecodeAttestation(bad version) = %v, want ErrMalformed", err)
	}
}

// TestVerifyRoles: who may attest, in every state a member can be in.
func TestVerifyRoles(t *testing.T) {
	a, adm, w, r, stranger := newSigner(1), newSigner(2), newSigner(3), newSigner(4), newSigner(5)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, adm, RoleAdmin)
	mustIssue(t, l, a, OpAdd, w, RoleWriter)
	mustIssue(t, l, a, OpAdd, r, RoleReader)
	attest := func(s Signer, p ProjectID) Attestation {
		t.Helper()
		att, err := NewAttestation(s, p, "doc", []Hash{{1}}, 1)
		if err != nil {
			t.Fatal(err)
		}
		return att
	}
	for _, tc := range []struct {
		name string
		att  Attestation
		want error
	}{
		{"creator admin", attest(a, l.Project()), nil},
		{"added admin", attest(adm, l.Project()), nil},
		{"writer", attest(w, l.Project()), nil},
		{"reader", attest(r, l.Project()), ErrNotWriter},
		{"stranger", attest(stranger, l.Project()), ErrNotWriter},
		{"other project", attest(w, ProjectID{1}), ErrWrongProject},
		{"invalid fields", Attestation{Project: l.Project(), Signer: w.Public()}, ErrMalformed},
	} {
		if err := l.Verify(tc.att); !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
			t.Errorf("Verify(%s) = %v, want %v", tc.name, err, tc.want)
		}
	}
	forged := attest(w, l.Project())
	forged.Signer = r.Public() // a reader's key under a writer's signature
	if err := l.Verify(forged); !errors.Is(err, ErrBadSignature) {
		t.Errorf("Verify(forged signer) = %v, want ErrBadSignature", err)
	}
	for name, tamper := range map[string]func(*Attestation){
		"doc":   func(a *Attestation) { a.Doc = "other" },
		"seq":   func(a *Attestation) { a.Seq++ },
		"heads": func(a *Attestation) { a.Heads = []Hash{{2}} },
	} {
		att := attest(w, l.Project())
		tamper(&att)
		if err := l.Verify(att); !errors.Is(err, ErrBadSignature) {
			t.Errorf("Verify(tampered %s) = %v, want ErrBadSignature", name, err)
		}
	}
}

// TestApplyAllReports: ApplyAll returns the ops that became part of the list,
// including those it released, and every refusal, while still applying the
// good ones.
func TestApplyAllReports(t *testing.T) {
	a, b, c := newSigner(1), newSigner(2), newSigner(3)
	src := project(t, a)
	genesis := src.Since(nil)
	x := mustIssue(t, src, a, OpAdd, b, RoleWriter)
	y := mustIssue(t, src, a, OpAdd, c, RoleWriter) // follows x

	l := New(src.Project())
	bad := x
	bad.Sig[0] ^= 0xff
	// y arrives before its parent x, then a forged x, then the genuine one.
	added, err := l.ApplyAll(append(slices.Clone(genesis), y, bad, x))
	if !errors.Is(err, ErrBadSignature) {
		t.Errorf("ApplyAll error = %v, want ErrBadSignature among the joined errors", err)
	}
	var got []Hash
	for _, op := range added {
		got = append(got, op.Hash())
	}
	want := []Hash{genesis[0].Hash(), x.Hash(), y.Hash()}
	if !slices.Equal(got, want) {
		t.Errorf("ApplyAll added %v, want genesis, x, then the released y", got)
	}
	if l.Len() != 3 || l.Pending() != 0 || l.Role(c.Public()) != RoleWriter {
		t.Errorf("Len() = %d, Pending() = %d, Role(c) = %v; want 3, 0, writer", l.Len(), l.Pending(), l.Role(c.Public()))
	}
}

// TestBufferedOpThatFailsIsNotReported: an op buffered for a missing parent
// that then fails the causal checks is dropped quietly: it is not reported as
// added (and so not relayed) and not stored.
func TestBufferedOpThatFailsIsNotReported(t *testing.T) {
	a, b, c := newSigner(1), newSigner(2), newSigner(3)
	l := project(t, a)
	src := fork(t, l)
	x := mustIssue(t, src, a, OpAdd, b, RoleWriter)

	badLamport := Op{Kind: OpAdd, Role: RoleReader, Subject: c.Public(), Author: a.Public(), Prev: []Hash{x.Hash()}, Lamport: x.Lamport + 5}
	badLamport.sign(a, l.Project())
	if added, err := l.ApplyAll([]Op{badLamport}); err != nil || len(added) != 0 || l.Pending() != 1 {
		t.Fatalf("ApplyAll(orphan) = %d added, %v, pending %d; want 0, nil, 1", len(added), err, l.Pending())
	}
	added, err := l.ApplyAll([]Op{x})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0].Hash() != x.Hash() {
		t.Errorf("released op with a bad Lamport was reported: added %d ops, want only x", len(added))
	}
	if l.Has(badLamport.Hash()) || l.Pending() != 0 || l.Len() != 2 {
		t.Errorf("Has(bad) = %t, Pending() = %d, Len() = %d; want false, 0, 2", l.Has(badLamport.Hash()), l.Pending(), l.Len())
	}
	if l.Role(c.Public()) != 0 {
		t.Error("the op with a bad Lamport took effect")
	}
}

// TestPendingExpiresAfterTwoTicks: a buffered op waits through one generation
// and is dropped by the second, whatever the age of the list.
func TestPendingExpiresAfterTwoTicks(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	src := fork(t, l)
	x := mustIssue(t, src, a, OpAdd, b, RoleWriter)
	y := mustIssue(t, src, a, OpSetRole, b, RoleReader)

	for range 5 { // the list is old: generations are no longer 0
		l.prunePending()
	}
	if err := l.Apply(y); err != nil || l.Pending() != 1 {
		t.Fatalf("Apply(orphan) = %v, pending %d; want nil, 1", err, l.Pending())
	}
	l.prunePending()
	if l.Pending() != 1 {
		t.Errorf("Pending() = %d one tick after buffering, want 1", l.Pending())
	}
	l.prunePending()
	if l.Pending() != 0 {
		t.Errorf("Pending() = %d two ticks after buffering, want 0", l.Pending())
	}
	// Arriving late, the parent does not resurrect it.
	if err := l.Apply(x); err != nil {
		t.Fatal(err)
	}
	if l.Has(y.Hash()) || l.Role(b.Public()) != RoleWriter {
		t.Error("an expired op came back")
	}

	// And one tick is not enough to lose it.
	l3 := New(l.Project())
	mustApplyAll(t, l3, l.Since(nil)[:1])
	for range 7 {
		l3.prunePending()
	}
	if err := l3.Apply(y); err != nil {
		t.Fatal(err)
	}
	l3.prunePending()
	if err := l3.Apply(x); err != nil {
		t.Fatal(err)
	}
	if !l3.Has(y.Hash()) {
		t.Error("an op buffered one tick ago was dropped before its parent arrived")
	}
}
