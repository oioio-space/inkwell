// Package blob forwards the public protocol API used by the application.
package blob

import (
	p "github.com/oioio-space/inkwell/protocol/blob"
)

// HashSize is defined by the protocol.
const HashSize = p.HashSize

// ErrBadHash is defined by the protocol.
var ErrBadHash = p.ErrBadHash

// Hash is the protocol type.
type Hash = p.Hash

// HashOf forwards to the protocol.
var HashOf = p.HashOf

// ParseHash forwards to the protocol.
var ParseHash = p.ParseHash

// GroupSize is defined by the protocol.
const GroupSize = p.GroupSize

// MaxSlice is defined by the protocol.
const MaxSlice = p.MaxSlice

// ErrMalformed is defined by the protocol.
var ErrMalformed = p.ErrMalformed

// Message is the protocol type.
type Message = p.Message

// Want is the protocol type.
type Want = p.Want

// Slice is the protocol type.
type Slice = p.Slice

// Missing is the protocol type.
type Missing = p.Missing

// Busy is the protocol type.
type Busy = p.Busy

// Encode forwards to the protocol.
var Encode = p.Encode

// Decode forwards to the protocol.
var Decode = p.Decode

const (
	groupLog   = 4
	maxOffset  = 1 << 40
	hashPrefix = p.HashPrefix
)

func checkRange(offset uint64, length uint32, _ bool) error {
	_, err := p.Encode(p.Want{Offset: offset, Length: length})
	return err
}
