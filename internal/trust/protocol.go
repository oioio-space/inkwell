// Package trust forwards the public protocol API used by the application.
package trust

import (
	"cmp"

	p "github.com/oioio-space/inkwell/protocol/trust"
)

// MaxAttestationSize is defined by the protocol.
const MaxAttestationSize = p.MaxAttestationSize

// MaxDoc is defined by the protocol.
const MaxDoc = p.MaxDoc

// MaxHeads is defined by the protocol.
const MaxHeads = p.MaxHeads

// Attestation is the protocol type.
type Attestation = p.Attestation

// NewAttestation forwards to the protocol.
var NewAttestation = p.NewAttestation

// EncodeAttestation forwards to the protocol.
var EncodeAttestation = p.EncodeAttestation

// DecodeAttestation forwards to the protocol.
var DecodeAttestation = p.DecodeAttestation

// Member is the protocol type.
type Member = p.Member

// List is the protocol type.
type List = p.List

// New forwards to the protocol.
var New = p.New

// MaxOpSize is defined by the protocol.
const MaxOpSize = p.MaxOpSize

// MaxPrev is defined by the protocol.
const MaxPrev = p.MaxPrev

// MaxName is defined by the protocol.
const MaxName = p.MaxName

// MaxRootURL is defined by the protocol.
const MaxRootURL = p.MaxRootURL

// ErrMalformed is defined by the protocol.
var ErrMalformed = p.ErrMalformed

// ErrBadSignature is defined by the protocol.
var ErrBadSignature = p.ErrBadSignature

// ErrWrongProject is defined by the protocol.
var ErrWrongProject = p.ErrWrongProject

// ErrBadLamport is defined by the protocol.
var ErrBadLamport = p.ErrBadLamport

// ErrUnknownAuthor is defined by the protocol.
var ErrUnknownAuthor = p.ErrUnknownAuthor

// ErrNotAdmin is defined by the protocol.
var ErrNotAdmin = p.ErrNotAdmin

// ErrLastAdmin is defined by the protocol.
var ErrLastAdmin = p.ErrLastAdmin

// ErrOutranked is defined by the protocol.
var ErrOutranked = p.ErrOutranked

// ErrSuspended is defined by the protocol.
var ErrSuspended = p.ErrSuspended

// ErrNotStanding is defined by the protocol.
var ErrNotStanding = p.ErrNotStanding

// ErrNotWriter is defined by the protocol.
var ErrNotWriter = p.ErrNotWriter

// ErrNoGenesis is defined by the protocol.
var ErrNoGenesis = p.ErrNoGenesis

// ErrTooManyOps is defined by the protocol.
var ErrTooManyOps = p.ErrTooManyOps

// Role is the protocol type.
type Role = p.Role

// RoleReader is defined by the protocol.
const RoleReader = p.RoleReader

// RoleWriter is defined by the protocol.
const RoleWriter = p.RoleWriter

// RoleAdmin is defined by the protocol.
const RoleAdmin = p.RoleAdmin

// Hash is the protocol type.
type Hash = p.Hash

// ProjectID is the protocol type.
type ProjectID = p.ProjectID

// OpKind is the protocol type.
type OpKind = p.OpKind

// OpGenesis is defined by the protocol.
const OpGenesis = p.OpGenesis

// OpAdd is defined by the protocol.
const OpAdd = p.OpAdd

// OpSetRole is defined by the protocol.
const OpSetRole = p.OpSetRole

// OpRemove is defined by the protocol.
const OpRemove = p.OpRemove

// OpSuspend is defined by the protocol.
const OpSuspend = p.OpSuspend

// OpResume is defined by the protocol.
const OpResume = p.OpResume

// Signer is the protocol type.
type Signer = p.Signer

// Op is the protocol type.
type Op = p.Op

// Encode forwards to the protocol.
var Encode = p.Encode

// Decode forwards to the protocol.
var Decode = p.Decode

// Genesis forwards to the protocol.
var Genesis = p.Genesis

// MaxMessage is defined by the protocol.
const MaxMessage = p.MaxMessage

// Outbound is the protocol type.
type Outbound[P cmp.Ordered] = p.Outbound[P]

// Sync is the protocol type.
type Sync[P cmp.Ordered] = p.Sync[P]

// NewSync constructs protocol synchronization state.
func NewSync[P cmp.Ordered](l *List) *Sync[P] { return p.NewSync[P](l) }
