// SPDX-License-Identifier: MPL-2.0

// Package admit is the accounting core of a node's connection admission (spec §16.1, rows « connexions »,
// « connexions non vérifiées » and « connexions par IP (membres) »; block RF1, rules RF-1 to RF-6; model
// docs/protocol/tla/RelayFanout.tla).
//
// It is pure: no I/O, no goroutine, no clock of its own, and nothing of libp2p. The transport
// (internal/transport/p2p) derives the [Origin] of each connection, passes the time in and closes what a
// refusal names. The rules:
//
//   - RF-1: before its handshake, an inbound connection is charged to its origin: the relay of a circuit,
//     named by its authenticated PeerID ([RelayOrigin]), otherwise its IPv4 /32 or IPv6 /56 ([IPOrigin]). An
//     outbound connection is charged to no origin.
//   - RF-2: at its handshake ([Ledger.Identify]) a member's or a configured relay's connection leaves every
//     pending count, and its admission token is given back if no period has paid it yet.
//   - RF-3: unverified connections (before the handshake, or identified as strangers and not yet closed) are
//     bounded per origin and in all; above the bound an opening is refused.
//   - RF-4: an unverified connection holds its place for [protocol.PendingDeadline] at most.
//   - RF-5: each origin has a token bucket; an unverified admission takes a token.
//   - RF-6: member connections are bounded in all, with [protocol.MemberShareConnections] reserved for each
//     member of the roster: a third or fourth connection comes only from what no member's share holds.
//     Configured relays have their own connections.
//
// A [Ledger] is not safe for concurrent use: its caller serialises the calls.
package admit
