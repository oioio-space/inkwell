// Package trust is the signed member list of a project and the attestations
// that gate writes. It is pure: no I/O, no clock, no goroutine.
//
// # Model
//
// A project's members form a hash-DAG of signed [Op]s (Genesis, Add, SetRole,
// Remove). Each op names in Prev the frontier its author had seen, carries a
// Lamport number (exactly 1 + the maximum of its parents, 0 for Genesis) and
// is signed over the [ProjectID] and its canonical encoding (domain
// "trust-op"). The ProjectID is the SHA-256 of the canonical Genesis op, so a
// project certifies itself and an op signed for another project does not
// verify. Ops are stored whatever their effect; the fold below decides it.
// The canonical encoding is binary, at most [MaxOpSize] bytes, and [Decode]
// is strict.
//
// # Fold rules (the specification)
//
// The result depends only on the set of ops, never on their arrival order.
// Ops are totally ordered by Lamport, then author key, then op hash; as
// Lamport grows along Prev, this order is also topological. "Past" means the
// strict causal ancestors of an op, "concurrent" means neither op is an
// ancestor of the other.
//
//  1. Authorization. An op is valid only if its author is an admin in the
//     membership obtained by replaying the effective ops of its own causal
//     past. Genesis is valid and makes its author the first admin. Only
//     admins issue ops.
//  2. Effects, replayed in total order. Add admits a key that is not a member
//     (an Add of a member changes nothing). SetRole changes a member's role
//     and, if Name is set, its name. Remove deletes a member. Ops that do
//     not apply are no-ops.
//  3. Never zero admins. An op that would remove or demote the last admin of
//     the replayed state is ineffective.
//  4. Remove beats the concurrent. A valid Remove(S) voids every op written
//     by S that is concurrent to it, which defeats antedating: S cannot
//     backdate ops with an old Prev. It also voids every Add or SetRole of S
//     concurrent to it. Ops of S in the Remove's past were seen by the remover
//     and stay; ops that follow the Remove fail rule 1. Re-admitting S needs
//     an Add whose past contains the Remove.
//  5. Crossed removes. If admin A's Remove(B) and admin B's Remove(A) are
//     concurrent, only the Remove of the older admin applies, the older being
//     the one admitted first in the total order (the creator is the oldest).
//     The younger's Remove is voided like any other op of its author.
//  6. Voids are sticky and cascade. A voided op never takes effect, so ops
//     whose authorization depended on it fail rule 1. The fold replays the
//     whole order until no new op is voided; a void is never withdrawn, even
//     if the Remove that caused it is later voided itself. A Remove triggers
//     rule 4 as soon as it is valid, even if rule 3 blocks its effect.
//
// An op whose author never appears as the subject of an Add (or as the
// creator) in the op's past is refused by [List.Apply] with
// [ErrUnknownAuthor], and the list holds at most 1024 ops and 256 ops waiting
// for unknown parents. The fold costs O(n²) per void found, which these
// bounds keep small.
//
// # Attestations
//
// An [Attestation] is a writer's signature (domain "attest") over the heads
// of a document. [List.Verify] checks the project, the signature and that the
// signer is a current writer or admin.
//
// # Synchronisation
//
// [Sync] replicates a list over any [transport.Transport]: peers exchange
// their frontier, send each other the missing ops in application order, and
// relay what is new. See [Sync] for the loss and reordering tolerance.
package trust
