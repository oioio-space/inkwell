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
// rather than evicts, and that only takes ops by keys that are admins in the
// current membership; [Sync] resends the others in causal order), its Lamport is 1 + the maximum of its parents', and its
// author was granted admin in the op's causal past: the creator, or the subject
// of an Add or SetRole with Role admin among the op's ancestors, effective or
// not ([ErrUnknownAuthor] otherwise). Every op kind is admin-only, so readers
// and writers cannot store anything. Each check reads only the op and its
// ancestors, so every replica reaches the same verdict.
//
// A list holding 1024 ops is sealed: it stores nothing more, so Issue and
// Apply fail with [ErrTooManyOps] and membership is frozen. Role, Members and
// Verify keep answering from the stored ops, so editing goes on. Sealing
// depends only on the set the replicas exchange: if it reaches 1024 ops every
// replica seals, otherwise every replica ends up holding the whole set. Sealed
// replicas may hold different sets, and so disagree on the membership, only
// when an ex-admin flooded them (such an admin can still store backdated ops);
// the application should then tell people to create a new project to manage
// members again.
//
// Any consumer that gates document writes on [List.Role] or [List.Verify]
// MUST also check [List.Sealed] and, when it is true, surface to the user
// "liste de membres saturée — créez un nouveau projet": a sealed replica
// cannot tell whether other replicas hold a different set, so its membership
// may be stale. Role and Verify keep their signatures and do not report it.
//
// There is deliberately no per-author quota on the stored ops of keys that
// were not standing admins when they were integrated. Such a quota would have
// to refuse the op that exceeds it, and which op that is depends on arrival
// order, so replicas would store different sets and diverge. Counting at
// fold time instead is replica-independent, but the dead ops are already
// stored and already count towards the 1024 that seal the list, so it would
// not protect anything.
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
//     re-promote or rename a senior while it is an admin. The one exception
//     is a Remove, or a SetRole or an Add to a lower role that sets no Name
//     (nor a Person), on a senior that is not admin at that op (rule 3(a)
//     evaluated for the senior, from the ops of keys senior to it and its
//     own): an admin that stepped down can be removed, demoted or re-added as
//     a non-admin by a junior, the former creator included. A grant or a
//     rename never counts on a senior, nor does an Add that names, regroups
//     or grants. A key may act on itself,
//     except that a cut of itself (a Remove, or a SetRole to a lower role)
//     counts only if some admin other than that key exists in the membership
//     computed from the ancestors of that op alone; otherwise it is ignored,
//     so the last admin cannot leave. This is also the creator's hand-over:
//     it keeps rank 0 and stays senior to everyone, but it is no longer an
//     admin (or a member), and rule 3(a) then kills its later ops. Nobody is
//     senior to the creator, so nobody can re-grant it: it returns only if it
//     re-granted itself before stepping down.
//  3. Standing. An op o by y stands if and only if:
//     (a) y is admin at o: among the role ops on y that o sees and that survive,
//     the ≺-latest is a grant (Genesis stands by itself);
//     (b) no standing cut on y by another key is concurrent to o, and no exit
//     of y is concurrent to o, unless o is an exit itself or is spared by
//     that exit; and
//     (c) rule 2 allows it.
//     An exit is a cut of y by y such that y is admin, along with another
//     admin, in the membership of its ancestors, and such that no standing
//     cut on y by another key is concurrent to it (an exit that a senior's
//     cut did not see is dead like any other op, and kills nothing). A
//     standing exit ends y's career at that point, wherever it falls in ≺. It
//     spares an op o of y concurrent to it when some op by a key other than y
//     in the set being folded sees o but not the exit: the others had
//     accepted o before the exit was in play, so a backdated exit, by a leaked
//     key for instance, cannot erase it.
//     An op that does not stand has no effect at all. By (b), what a revoked
//     admin signs without having seen its cut is dead, even if it is later
//     re-admitted. By (a), what it signs after seeing the cut is dead unless a
//     senior re-granted it in between.
//  4. Survival. A standing role op on x by another key is dropped if a
//     concurrent standing role op on x by another key sets a lower level, so a
//     revocation beats a concurrent grant. All other role ops survive. A key's
//     ops on itself always survive and drop nothing. The cuts that rule 2
//     allows from a junior on a senior drop nothing either, and a concurrent
//     surviving grant on that senior drops them.
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
// ops, except what a self-cut says and what spares an op from an exit (which
// reads the set being folded, through its ancestor bitsets, in O(n/64)) (honoured, exit), which reads the fold of
// its own ancestors, a strictly smaller set. A grant's position is greater
// than its author's rank, because the author's own grant is one of its
// ancestors. So one sweep over keys in rank order decides everything. Grants
// and cuts only go to juniors, so two admins cannot cut each other and no
// cycle of removals exists. The exception of rule 2 keeps the sweep acyclic
// because a senior is decided before its juniors: whether it is admin at the
// junior's op is read from what was decided for it, and the junior's cut,
// decided afterwards, neither changes the senior's rank, nor its ops, nor
// drops a grant, so nothing already decided is revisited; it only shapes the
// senior's final role (rule 5). The fold costs O(n²/64 + n·e), where e is the
// largest number of role ops on one key, plus one more fold per self-cut.
//
// An admin leaves only after seeing another admin, and nobody can remove
// the creator while it is admin. Concurrent departures of different keys (the
// creator and the last other admin both leaving) can nevertheless leave no
// admin: the membership is then frozen, while editing, which never depends on
// an admin being present, is unaffected.
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
