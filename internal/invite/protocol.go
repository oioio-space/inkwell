// Package invite forwards the public protocol API used by the application.
package invite

import (
	p "github.com/oioio-space/inkwell/protocol/invite"
)

// MaxRequest is defined by the protocol.
const MaxRequest = p.MaxRequest

// MaxReply is defined by the protocol.
const MaxReply = p.MaxReply

// RequestKind is the protocol type.
type RequestKind = p.RequestKind

// KindHello is defined by the protocol.
const KindHello = p.KindHello

// KindAsk is defined by the protocol.
const KindAsk = p.KindAsk

// KindRedeem is defined by the protocol.
const KindRedeem = p.KindRedeem

// Request is the protocol type.
type Request = p.Request

// DecodeRequest forwards to the protocol.
var DecodeRequest = p.DecodeRequest

// ReplyKind is the protocol type.
type ReplyKind = p.ReplyKind

// ReplyHello is defined by the protocol.
const ReplyHello = p.ReplyHello

// ReplyWelcome is defined by the protocol.
const ReplyWelcome = p.ReplyWelcome

// ReplyRefused is defined by the protocol.
const ReplyRefused = p.ReplyRefused

// Reason is the protocol type.
type Reason = p.Reason

// ReasonInvalid is defined by the protocol.
const ReasonInvalid = p.ReasonInvalid

// ReasonDeclined is defined by the protocol.
const ReasonDeclined = p.ReasonDeclined

// ReasonBusy is defined by the protocol.
const ReasonBusy = p.ReasonBusy

// ReasonFailed is defined by the protocol.
const ReasonFailed = p.ReasonFailed

// ReasonTooLarge is defined by the protocol.
const ReasonTooLarge = p.ReasonTooLarge

// Reply is the protocol type.
type Reply = p.Reply

// DecodeReply forwards to the protocol.
var DecodeReply = p.DecodeReply

// TicketTTL is defined by the protocol.
const TicketTTL = p.TicketTTL

// MaxTextLen is defined by the protocol.
const MaxTextLen = p.MaxTextLen

// ErrMalformed is defined by the protocol.
var ErrMalformed = p.ErrMalformed

// Ticket is the protocol type.
type Ticket = p.Ticket

// ParseText forwards to the protocol.
var ParseText = p.ParseText
