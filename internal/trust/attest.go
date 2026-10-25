package trust

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"slices"
	"unicode/utf8"
)

// Limits of the canonical attestation encoding.
const (
	// MaxAttestationSize is the largest canonical encoding of an [Attestation].
	MaxAttestationSize = 4 << 10
	// MaxDoc bounds Attestation.Doc, in bytes.
	MaxDoc = 256
	// MaxHeads bounds the number of heads of an attestation.
	MaxHeads = 64

	attestVersion = 1
)

// Attestation is a writer's signed statement of the heads of one document: a
// peer accepts a change only if the heads attested by current writers cover it.
type Attestation struct {
	// Doc names the document.
	Doc string
	// Heads are the change hashes, strictly ascending.
	Heads []Hash
	// Signer is the writer's key.
	Signer ed25519.PublicKey
	// Seq increases with each attestation of the same (Doc, Signer); receivers
	// keep the highest.
	Seq uint64
	// Project is the project the attestation belongs to.
	Project ProjectID
	// Sig is the signature of Project and the fields above.
	Sig [64]byte
}

func (a Attestation) validate() error {
	if len(a.Signer) != keyLen || a.Doc == "" || len(a.Doc) > MaxDoc || !utf8.ValidString(a.Doc) || len(a.Heads) > MaxHeads {
		return fmt.Errorf("%w: attestation fields", ErrMalformed)
	}
	for i := 1; i < len(a.Heads); i++ {
		if bytes.Compare(a.Heads[i-1][:], a.Heads[i][:]) >= 0 {
			return fmt.Errorf("%w: heads not strictly ascending", ErrMalformed)
		}
	}
	return nil
}

// body encodes everything the signature covers, project included.
func (a Attestation) body() []byte {
	doc := a.Doc[:min(len(a.Doc), MaxDoc)]
	heads := a.Heads[:min(len(a.Heads), MaxHeads)]
	b := make([]byte, 0, 128)
	b = append(b, attestVersion)
	b = append(b, a.Project[:]...)
	b = appendKey(b, a.Signer)
	b = binary.BigEndian.AppendUint64(b, a.Seq)
	b = binary.BigEndian.AppendUint16(b, uint16(len(doc))) // #nosec G115 -- bounded by MaxDoc.
	b = append(b, doc...)
	b = binary.BigEndian.AppendUint16(b, uint16(len(heads))) // #nosec G115 -- bounded by MaxHeads.
	for _, h := range heads {
		b = append(b, h[:]...)
	}
	return b
}

// NewAttestation signs the heads of doc as the writer s. The heads are
// sorted and deduplicated.
func NewAttestation(s Signer, project ProjectID, doc string, heads []Hash, seq uint64) (Attestation, error) {
	hs := slices.Clone(heads)
	slices.SortFunc(hs, func(a, b Hash) int { return bytes.Compare(a[:], b[:]) })
	hs = slices.Compact(hs)
	a := Attestation{Project: project, Doc: doc, Heads: hs, Signer: bytes.Clone(s.Public()), Seq: seq}
	if err := a.validate(); err != nil {
		return Attestation{}, err
	}
	copy(a.Sig[:], s.Sign(domainAttest, a.body()))
	return a, nil
}

// EncodeAttestation returns the canonical encoding of a.
func EncodeAttestation(a Attestation) []byte { return append(a.body(), a.Sig[:]...) }

// DecodeAttestation parses the canonical encoding of an attestation, strictly.
// The signature is not checked; [List.Verify] does.
func DecodeAttestation(b []byte) (Attestation, error) {
	if len(b) > MaxAttestationSize {
		return Attestation{}, fmt.Errorf("%w: too large", ErrMalformed)
	}
	r := &reader{b: b}
	var a Attestation
	if v := r.u8(); r.err == nil && v != attestVersion {
		return Attestation{}, fmt.Errorf("%w: version %d", ErrMalformed, v)
	}
	a.Project = ProjectID(r.hash())
	a.Signer = bytes.Clone(r.take(keyLen))
	a.Seq = r.u64()
	a.Doc = r.str(MaxDoc)
	n := r.u16()
	if r.err == nil && n > MaxHeads {
		return Attestation{}, fmt.Errorf("%w: too many heads", ErrMalformed)
	}
	if n > 0 && r.err == nil {
		a.Heads = make([]Hash, n)
		for i := range a.Heads {
			a.Heads[i] = r.hash()
		}
	}
	copy(a.Sig[:], r.take(len(a.Sig)))
	if r.err != nil {
		return Attestation{}, r.err
	}
	if len(r.b) != 0 {
		return Attestation{}, fmt.Errorf("%w: trailing bytes", ErrMalformed)
	}
	if err := a.validate(); err != nil {
		return Attestation{}, err
	}
	return a, nil
}

// Verify checks that a belongs to this project, is correctly signed, and that
// its signer is currently a writer or an admin.
func (l *List) Verify(a Attestation) error {
	if a.Project != l.id {
		return ErrWrongProject
	}
	if err := a.validate(); err != nil {
		return err
	}
	if !ed25519.Verify(a.Signer, signingMessage(domainAttest, a.body()), a.Sig[:]) {
		return ErrBadSignature
	}
	if l.Role(a.Signer) < RoleWriter {
		return ErrNotWriter
	}
	return nil
}
