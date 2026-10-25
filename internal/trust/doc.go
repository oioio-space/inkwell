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
// verify. The canonical encoding is binary, at most [MaxOpSize] bytes, and [Decode]
// is strict.
//
// # Admission
//
// [List.Apply] stores an op only if it is canonical and signed for the project,
// its parents are stored (otherwise it waits in a bounded buffer that refuses
// rather than evicts), its Lamport is 1 + the maximum of its parents', and its
// author was granted admin in the op's causal past: the creator, or the subject
// of an Add or SetRole with Role admin among the op's ancestors, effective or
// not ([ErrUnknownAuthor] otherwise). Every op kind is admin-only, so readers
// and writers cannot store anything. Each check reads only the op and its
// ancestors, so every replica reaches the same verdict.
//
// A list holding 1024 ops is sealed: it stores nothing more, and Issue, Apply
// and Verify fail with [ErrTooManyOps]. Role and Members report nobody, so
// the list fails closed. Sealing depends only on the set the replicas exchange:
// if it reaches 1024 ops every replica seals, otherwise every replica ends up
// holding the whole set.
//
// # Fold rules (the specification)
//
// The membership is a function of the stored set. ≺ orders ops by Lamport,
// author key, then hash, and extends causality. pos(o) is o's index in ≺.
// "o sees p" means p is a strict ancestor of o. Two ops are concurrent when
// neither sees the other. A role op on x is an Add, SetRole or Remove whose
// subject is x, or Genesis for the creator. It is a grant if it sets Role
// admin, and a cut if it is a Remove or sets a lower role. Levels go
// Remove < reader < writer < admin.
//
//  1. Seniority. The creator has rank 0. Any other key's rank is pos(g) of the
//     ≺-earliest surviving (rule 4) standing grant g on it. A key without one
//     is unranked. A smaller rank is senior. Ranks are distinct and are kept
//     across demotion.
//  2. Reach. A role op by y on another key x counts only if x is unranked or
//     junior to y. An op on a senior is ignored: no one can cut, demote,
//     re-promote or rename a senior, so nobody but the creator itself can ever
//     act on the creator. A key may act on itself, except that a cut of the
//     creator by itself (a Remove, or a SetRole to a lower role) counts only if
//     some admin other than the creator exists in the membership computed from
//     the ancestors of that op alone; otherwise it is ignored. This is the
//     hand-over: the creator keeps rank 0 and stays senior to everyone, but it
//     is no longer an admin (or a member), and rule 3(a) then kills its later
//     ops. Nobody is senior to the creator, so nobody can re-grant it: it
//     returns only if it re-granted itself before stepping down.
//  3. Standing. An op o by y stands if and only if:
//     (a) y is admin at o: among the role ops on y that o sees and that survive,
//     the ≺-latest is a grant (Genesis stands by itself);
//     (b) no standing cut on y by another key is concurrent to o; and
//     (c) rule 2 allows it.
//     An op that does not stand has no effect at all. By (b), what a revoked
//     admin signs without having seen its cut is dead, even if it is later
//     re-admitted. By (a), what it signs after seeing the cut is dead unless a
//     senior re-granted it in between.
//  4. Survival. A standing role op on x by another key is dropped if a
//     concurrent standing role op on x by another key sets a lower level, so a
//     revocation beats a concurrent grant. All other role ops survive. A key's
//     ops on itself always survive and drop nothing.
//  5. Result. x's role is the one set by the ≺-latest surviving role op on x.
//     If that op is a Remove, or there is none, x is not a member. Add and
//     SetRole both set the role, and the Name when it is non-empty (the Name of
//     Genesis is the project title, not a member name, and a Remove forgets the
//     names set before it). Person comes from the ≺-latest surviving Add after
//     the latest surviving Remove (that Add's hash if Person is zero), or the
//     hash of the op that admitted x if there is none. Members are listed in ≺
//     order of that Add (of the admitting op if there is none), and the creator
//     comes first.
//
// Each rule about y's ops reads only ops by keys senior to y and y's own
// earlier ops, except the creator's cut of itself, which reads the fold of its
// own ancestors, a strictly smaller set. A grant's position is greater than
// its author's rank, because the author's own grant is one of its ancestors.
// So one sweep over keys in rank order decides everything. Cuts only go to
// juniors, so two admins cannot cut each other and no cycle of removals
// exists. The fold costs O(n²/64 + n·e), where e is the largest number of role
// ops on one key, plus one more fold per self-cut of the creator.
//
// The creator leaves only after seeing another admin, and nobody else can
// remove it. Concurrent departures (the creator and the last other admin both
// leaving, or the creator's own concurrent Remove of that admin and of itself)
// can nevertheless leave no admin: the membership is then frozen and fails
// closed, while editing, which never depends on an admin being present, is
// unaffected.
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
// relay what is new. A Have is answered with ops, never with a Have, and a
// sealed list does not announce. See [Sync] for the loss and reordering
// tolerance.
package trust
