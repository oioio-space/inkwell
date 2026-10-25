package trust

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Limits of the canonical encoding. Every byte that comes from a peer is
// hostile, so each field is bounded and the whole op fits MaxOpSize.
const (
	// MaxOpSize is the largest canonical encoding of an [Op], in bytes.
	MaxOpSize = 4 << 10
	// MaxPrev is the largest number of parents an op may name.
	MaxPrev = 64
	// MaxName bounds Op.Name, in bytes.
	MaxName = 128
	// MaxRootURL bounds Op.RootURL, in bytes.
	MaxRootURL = 512

	opVersion = 1
	keyLen    = ed25519.PublicKeySize

	domainOp     = "trust-op"
	domainAttest = "attest"
)

// Errors returned by this package. Use [errors.Is] to test them.
var (
	// ErrMalformed marks bytes or fields that violate the canonical encoding.
	ErrMalformed = errors.New("trust: malformed")
	// ErrBadSignature marks a signature that does not verify for this project.
	ErrBadSignature = errors.New("trust: bad signature")
	// ErrWrongProject marks data that belongs to another project.
	ErrWrongProject = errors.New("trust: wrong project")
	// ErrBadLamport marks an op whose Lamport is not 1 + the maximum of its parents.
	ErrBadLamport = errors.New("trust: bad lamport")
	// ErrUnknownAuthor marks an op whose author was never granted admin in its causal past.
	ErrUnknownAuthor = errors.New("trust: unknown author")
	// ErrNotAdmin is returned when a signer without the admin role issues an op.
	ErrNotAdmin = errors.New("trust: signer is not an admin")
	// ErrLastAdmin is returned when the creator would remove or demote itself
	// while no other admin exists.
	ErrLastAdmin = errors.New("trust: would remove the last admin")
	// ErrOutranked is returned when an op targets a key senior to its signer.
	ErrOutranked = errors.New("trust: subject is senior to the signer")
	// ErrSuspended is returned when the signer of an attestation, or of an op
	// to issue, is suspended, and when a device is added to a person one of
	// whose devices is suspended.
	ErrSuspended = errors.New("trust: suspended")
	// ErrNotStanding is returned when an op to issue would be stored but would
	// not stand in the fold, for instance because the frontier is too wide to
	// name all the ops it must see.
	ErrNotStanding = errors.New("trust: the op would have no effect")
	// ErrNotWriter marks an attestation whose signer cannot write.
	ErrNotWriter = errors.New("trust: signer is not a writer")
	// ErrNoGenesis is returned when the list has not received its genesis yet.
	ErrNoGenesis = errors.New("trust: genesis not received")
	// ErrTooManyOps is returned when the list is sealed, or the buffer of ops
	// waiting for their parents is full.
	ErrTooManyOps = errors.New("trust: too many ops")
)

// Role is a member's permission level.
type Role uint8

// The roles, from least to most privileged.
const (
	// RoleReader may read and send comments and cursors.
	RoleReader Role = iota + 1
	// RoleWriter may also edit.
	RoleWriter
	// RoleAdmin may also change the member list.
	RoleAdmin
)

// Valid reports whether r is one of the roles a member can hold.
func (r Role) Valid() bool { return r >= RoleReader && r <= RoleAdmin }

// String returns the role's name, for logs.
func (r Role) String() string {
	switch r {
	case RoleReader:
		return "reader"
	case RoleWriter:
		return "writer"
	case RoleAdmin:
		return "admin"
	default:
		return "none"
	}
}

// Hash is a SHA-256 digest.
type Hash [32]byte

// String returns the hexadecimal form of h.
func (h Hash) String() string { return hex.EncodeToString(h[:]) }

// cmpHash orders hashes bytewise, the order of Prev, Heads and the frontier.
func cmpHash(a, b Hash) int { return bytes.Compare(a[:], b[:]) }

// strictlyAscending reports whether hs is sorted by [cmpHash] without duplicates.
func strictlyAscending(hs []Hash) bool {
	for i := 1; i < len(hs); i++ {
		if cmpHash(hs[i-1], hs[i]) >= 0 {
			return false
		}
	}
	return true
}

// ProjectID identifies a project: the SHA-256 of its canonical Genesis op, so
// it certifies itself.
type ProjectID Hash

// String returns the hexadecimal form of id.
func (id ProjectID) String() string { return Hash(id).String() }

// OpKind says what an [Op] does.
type OpKind uint8

// The op kinds.
const (
	// OpGenesis creates the project; its author becomes the first admin.
	OpGenesis OpKind = iota + 1
	// OpAdd admits Subject with Role.
	OpAdd
	// OpSetRole changes the role of member Subject.
	OpSetRole
	// OpRemove removes Subject.
	OpRemove
	// OpSuspend suspends Subject: its writes are refused until an OpResume.
	// It keeps its role and its rank. Author and Subject differ, and Role,
	// Person and Name are empty.
	OpSuspend
	// OpResume lets a suspended Subject back. Same fields as OpSuspend.
	OpResume
)

// suspension reports whether the op is an OpSuspend or an OpResume. These are
// not role ops: they change neither roles nor ranks.
func (o Op) suspension() bool { return o.Kind == OpSuspend || o.Kind == OpResume }

// Signer signs on behalf of one Ed25519 key. Sign must sign
// "inkwell/<domain>/v1\x00" followed by msg, which is what this package
// verifies; internal/identity's device implements it.
type Signer interface {
	// Public returns the signer's public key.
	Public() ed25519.PublicKey
	// Sign returns the domain-separated signature of msg.
	Sign(domain string, msg []byte) []byte
}

// Op is one signed entry of the member list's hash-DAG.
type Op struct {
	// Name is the member's display name (Add, SetRole when non-empty) or the
	// project title (Genesis); empty for the other kinds.
	Name string
	// RootURL is the automerge document URL of the project; Genesis only.
	RootURL string
	// Prev is the author's frontier when it wrote the op: the hashes of the
	// ops it had seen and that no other seen op follows. Strictly ascending.
	Prev []Hash
	// Subject is the key the op is about; the author for Genesis.
	Subject ed25519.PublicKey
	// Author signed the op.
	Author ed25519.PublicKey
	// Person groups the devices of one person; the zero value on an Add means
	// "a new person".
	Person Hash
	// Lamport is 0 for Genesis and 1 + the maximum parent Lamport otherwise.
	Lamport uint64
	// Sig is the Ed25519 signature, see [Signer].
	Sig [64]byte
	// Kind says what the op does.
	Kind OpKind
	// Role is the role granted by Add and SetRole; RoleAdmin for Genesis.
	Role Role
}

// Hash returns the SHA-256 of the op's canonical encoding.
func (o Op) Hash() Hash { return sha256.Sum256(Encode(o)) }

// validate checks the op's structure against the canonical rules.
func (o Op) validate() error {
	if len(o.Author) != keyLen || len(o.Subject) != keyLen {
		return fmt.Errorf("%w: key length", ErrMalformed)
	}
	if len(o.Name) > MaxName || !utf8.ValidString(o.Name) || len(o.RootURL) > MaxRootURL || !utf8.ValidString(o.RootURL) {
		return fmt.Errorf("%w: text", ErrMalformed)
	}
	if len(o.Prev) > MaxPrev {
		return fmt.Errorf("%w: too many parents", ErrMalformed)
	}
	if !strictlyAscending(o.Prev) {
		return fmt.Errorf("%w: parents not strictly ascending", ErrMalformed)
	}
	switch o.Kind {
	case OpGenesis:
		if !bytes.Equal(o.Author, o.Subject) || o.Role != RoleAdmin || o.Person != (Hash{}) || len(o.Prev) != 0 || o.Lamport != 0 {
			return fmt.Errorf("%w: genesis fields", ErrMalformed)
		}
		return nil
	case OpAdd:
		if !o.Role.Valid() {
			return fmt.Errorf("%w: role", ErrMalformed)
		}
	case OpSetRole:
		if !o.Role.Valid() || o.Person != (Hash{}) {
			return fmt.Errorf("%w: setrole fields", ErrMalformed)
		}
	case OpRemove:
		if o.Role != 0 || o.Person != (Hash{}) || o.Name != "" {
			return fmt.Errorf("%w: remove fields", ErrMalformed)
		}
	case OpSuspend, OpResume:
		if bytes.Equal(o.Author, o.Subject) || o.Role != 0 || o.Person != (Hash{}) || o.Name != "" {
			return fmt.Errorf("%w: suspension fields", ErrMalformed)
		}
	default:
		return fmt.Errorf("%w: kind", ErrMalformed)
	}
	if o.RootURL != "" || len(o.Prev) == 0 || o.Lamport == 0 {
		return fmt.Errorf("%w: non-genesis fields", ErrMalformed)
	}
	return nil
}

// signedBytes is what the author signs: the project, then the body. The
// genesis is signed with the zero project, since its hash defines the project.
func signedBytes(project ProjectID, o Op) []byte {
	return append(bytes.Clone(project[:]), encodeBody(o)...)
}

func encodeBody(o Op) []byte {
	name := o.Name[:min(len(o.Name), MaxName)]
	url := o.RootURL[:min(len(o.RootURL), MaxRootURL)]
	prev := o.Prev[:min(len(o.Prev), MaxPrev)]
	b := make([]byte, 0, 256)
	b = append(b, opVersion, byte(o.Kind), byte(o.Role))
	b = binary.BigEndian.AppendUint64(b, o.Lamport)
	b = appendKey(b, o.Author)
	b = appendKey(b, o.Subject)
	b = append(b, o.Person[:]...)
	b = binary.BigEndian.AppendUint16(b, uint16(len(name))) // #nosec G115 -- bounded by MaxName.
	b = append(b, name...)
	b = binary.BigEndian.AppendUint16(b, uint16(len(url))) // #nosec G115 -- bounded by MaxRootURL.
	b = append(b, url...)
	b = append(b, byte(len(prev))) // #nosec G115 -- bounded by MaxPrev.
	for _, p := range prev {
		b = append(b, p[:]...)
	}
	return b
}

func appendKey(b, k []byte) []byte {
	var fixed [keyLen]byte
	copy(fixed[:], k)
	return append(b, fixed[:]...)
}

// Encode returns the canonical binary encoding of o. It is at most MaxOpSize
// bytes for a valid op.
func Encode(o Op) []byte { return append(encodeBody(o), o.Sig[:]...) }

// reader is a bounds-checked cursor over hostile bytes; the first failure
// sticks and later reads return zero values.
type reader struct {
	err error
	b   []byte
}

func (r *reader) take(n int) []byte {
	if r.err != nil || n > len(r.b) {
		r.err = fmt.Errorf("%w: truncated", ErrMalformed)
		return nil
	}
	s := r.b[:n:n]
	r.b = r.b[n:]
	return s
}

func (r *reader) u8() byte {
	if s := r.take(1); s != nil {
		return s[0]
	}
	return 0
}

func (r *reader) u16() int {
	if s := r.take(2); s != nil {
		return int(binary.BigEndian.Uint16(s))
	}
	return 0
}

func (r *reader) u64() uint64 {
	if s := r.take(8); s != nil {
		return binary.BigEndian.Uint64(s)
	}
	return 0
}

func (r *reader) str(limit int) string {
	n := r.u16()
	if r.err == nil && n > limit {
		r.err = fmt.Errorf("%w: field too long", ErrMalformed)
		return ""
	}
	return string(r.take(n))
}

func (r *reader) hash() (h Hash) {
	copy(h[:], r.take(len(h)))
	return h
}

// hashes reads n hashes; it returns nil when n is 0 or the reader has failed.
func (r *reader) hashes(n int) []Hash {
	if n == 0 || r.err != nil {
		return nil
	}
	hs := make([]Hash, n)
	for i := range hs {
		hs[i] = r.hash()
	}
	return hs
}

// Decode parses the canonical encoding of an op. It is strict: any deviation
// from the canonical form, including trailing bytes, is [ErrMalformed]. The
// signature is not checked; [List.Apply] does.
func Decode(b []byte) (Op, error) {
	if len(b) > MaxOpSize {
		return Op{}, fmt.Errorf("%w: too large", ErrMalformed)
	}
	r := &reader{b: b}
	var o Op
	if v := r.u8(); r.err == nil && v != opVersion {
		return Op{}, fmt.Errorf("%w: version %d", ErrMalformed, v)
	}
	o.Kind, o.Role = OpKind(r.u8()), Role(r.u8())
	o.Lamport = r.u64()
	o.Author = bytes.Clone(r.take(keyLen))
	o.Subject = bytes.Clone(r.take(keyLen))
	o.Person = r.hash()
	o.Name = r.str(MaxName)
	o.RootURL = r.str(MaxRootURL)
	n := int(r.u8())
	if r.err == nil && n > MaxPrev {
		return Op{}, fmt.Errorf("%w: too many parents", ErrMalformed)
	}
	o.Prev = r.hashes(n)
	copy(o.Sig[:], r.take(len(o.Sig)))
	if r.err != nil {
		return Op{}, r.err
	}
	if len(r.b) != 0 {
		return Op{}, fmt.Errorf("%w: trailing bytes", ErrMalformed)
	}
	if err := o.validate(); err != nil {
		return Op{}, err
	}
	return o, nil
}

// signingMessage prefixes msg the way [Signer] implementations do.
func signingMessage(domain string, msg []byte) []byte {
	return append([]byte("inkwell/"+domain+"/v1\x00"), msg...)
}

// verify checks the op's signature for project.
func (o Op) verify(project ProjectID) error {
	if o.Kind == OpGenesis {
		project = ProjectID{}
	}
	if !ed25519.Verify(o.Author, signingMessage(domainOp, signedBytes(project, o)), o.Sig[:]) {
		return ErrBadSignature
	}
	return nil
}

func (o *Op) sign(s Signer, project ProjectID) {
	copy(o.Sig[:], s.Sign(domainOp, signedBytes(project, *o)))
}

// Genesis creates a project signed by s, which becomes its first admin. name
// is the project title and rootURL the automerge document URL; both must obey
// the limits of [Op] or [List.Apply] refuses the op.
func Genesis(s Signer, name, rootURL string) (Op, ProjectID) {
	pub := s.Public()
	o := Op{Kind: OpGenesis, Author: pub, Subject: pub, Role: RoleAdmin, Name: name, RootURL: rootURL}
	o.sign(s, ProjectID{})
	return o, ProjectID(o.Hash())
}
