// Package bridge forwards the public protocol API used by the application.
package bridge

import (
	p "github.com/oioio-space/inkwell/protocol/bridge"
)

// StreamName is defined by the protocol.
const StreamName = p.StreamName

// ErrMalformed is defined by the protocol.
var ErrMalformed = p.ErrMalformed

// Envelope is the protocol type.
type Envelope = p.Envelope

// Encode forwards to the protocol.
var Encode = p.Encode

// Decode forwards to the protocol.
var Decode = p.Decode

// Kind is the protocol type.
type Kind = p.Kind

// PeerUp is defined by the protocol.
const PeerUp = p.PeerUp

// PeerDown is defined by the protocol.
const PeerDown = p.PeerDown

// Message is defined by the protocol.
const Message = p.Message

// Attest is defined by the protocol.
const Attest = p.Attest

// Attested is defined by the protocol.
const Attested = p.Attested

// Revoked is defined by the protocol.
const Revoked = p.Revoked

// Sealed is defined by the protocol.
const Sealed = p.Sealed

// Roles is defined by the protocol.
const Roles = p.Roles

// Suspended is defined by the protocol.
const Suspended = p.Suspended

// HashSize is defined by the protocol.
const HashSize = p.HashSize

// MaxDoc is defined by the protocol.
const MaxDoc = p.MaxDoc

// MaxHeads is defined by the protocol.
const MaxHeads = p.MaxHeads

// Heads is the protocol type.
type Heads = p.Heads

// EncodeHeads forwards to the protocol.
var EncodeHeads = p.EncodeHeads

// DecodeHeads forwards to the protocol.
var DecodeHeads = p.DecodeHeads

// MaxMembers is defined by the protocol.
const MaxMembers = p.MaxMembers

// MemberRole is the protocol type.
type MemberRole = p.MemberRole

// EncodeRoles forwards to the protocol.
var EncodeRoles = p.EncodeRoles

// DecodeRoles forwards to the protocol.
var DecodeRoles = p.DecodeRoles

const maxWire = 64 << 20
