# Git link (W71): design

Status: proposal (2026-11-14), not frozen. It refines HI-5, VA-10 and C17, and it keeps INV-3 (Automerge is the only
truth), INV-12 (no `git` binary) and the rule of no server.

## 0. The decision

1. Each **version record** becomes exactly one Git commit. The commit is a pure, versioned function
   (`Inkwell-Map: 1`) of data **frozen when the version was created**, so any two pushers produce byte-identical
   objects.
2. The only commits that depend on the remote are **joins**. One is made when versions were named at the same time
   and the branch must cover them all. The remote's compare-and-swap on a ref (old-id checked under lock) puts
   pushers in an order, so the branch only fast-forwards.
3. **Pull** brings an outside commit in as an offline edit made at the base version, through the existing
   `integrate()` path. The import is staged first and published only after the importer has won that same
   compare-and-swap, so Git itself is the lock that makes each outside commit imported once.
4. No new truth is created:
   - the link is an admin decision in the signed journal (ADR 0030);
   - credentials stay on each device;
   - `.inkwell/git/` is a cache that can be rebuilt.

## 1. Sources, and what follows from them

- **Object ids.** An id is `SHA-1("<type> <len>\0" ‖ body)`. A tree entry is `<mode> <name>\0<raw id>`. A commit is
  `tree`, `parent`*, `author`, `committer` (`Name <email> <s> ±hhmm`), a blank line, then the message.
  - go-git v5.19.3 refuses unsorted trees, but writes identities and time zones exactly as given (`tree.go`,
    `commit.go`).
  - Hence: **inkwell owns a canonical encoder of about 100 lines**, and uses go-git only to store objects and to move
    them over the network.
- **The server compares and swaps.** A push sends `old-id new-id ref`. The server checks the ref "hasn't changed
  while the request was being processed", or replies `ng` (pack-protocol).
  - Pushing a ref that is already up to date is a no-op (git-push).
  - go-git has `RequireRemoteRefs`, `ForceWithLease` and `Atomic`. **Inkwell never forces.**
- **go-git.** v5.19.3 (2026-10-04) is stable; v6 is still in beta (COMPATIBILITY.md).
  - It does: push, fetch, smart HTTP(S), SSH, atomic push, SHA-256, signature verification.
  - It does not: merge (fast-forward only), rebase, LFS, protocol v2, signing.
  - Its source tree has no hook runner. It uses `sha1cd` (collision detection) and `knownhosts`.
  - Its 2026 CVEs all sit in worktree checkout and on-disk ref storage: CVE-2026-45571, CVE-2026-71556 (writing
    through a symlink), CVE-2026-71557 (ref-name traversal).
  - Hence: never a worktree, explicit refspecs, pin ≥ 5.19.3. No system `git` is needed.
- **Prior art.**
  - Overleaf makes commits "when needed" under the author of the last change. It refuses branches, LFS and
    submodules, turns symlinks into files, and drops exec bits. On a conflict it pushes a dated side branch for a
    human to merge. Imports show as "Anonymous (via GitHub)".
  - HackMD makes named versions into commits, and does not record what it pushed (the branch may change outside).
  - Radicle avoids contention with per-peer ref namespaces and signed refs.
  - Jujutsu keeps a stable change id apart from the commit id, which changes when a commit is rewritten.

  Inkwell takes from them:
  - named versions as the unit of a commit;
  - a stable version key (not the commit id);
  - recognising objects by checking them, not by trusting them.

  Inkwell beats Overleaf on two points:
  - concurrency never creates a conflict branch, because the CRDT has already merged;
  - each version keeps its own author.
- **Limits.**
  - GitHub blocks files over 100 MiB.
  - I found no official GitHub hosting of SHA-256 repositories, so **v1 links only SHA-1 repositories**.
  - SSHSIG has no random field, so an Ed25519 signature over it is deterministic. Signing (block G4) is therefore
    possible.

## 2. Mapping v1

**Prerequisite (G0): project manifests (HI-3, VA-5, VD-1).** Today a `Version` covers one document
(`versions.ts:55`). G0 replaces it with:

- the heads of the index;
- the (DocId, heads) of each file, the blob hashes and the main file;
- `who` (≤ 80 bytes), `note`, and `missing` (blobs absent when the version was created, HI-9).

**Frozen reading.** A record is read **as of the change that created it**. The record's objId gives the actor and the
op, hence the change hash. A later rename, hide or forged edit never moves its commit. Deleting a version only hides
it.

**What becomes a commit.**
- Every version record of a line is a commit: named versions, `auto` versions, and imports.
- **"Envoyer" first adds an auto version at the current heads** (unless they equal a tip), so work that was never
  named reaches Git whenever someone sends.
- Rejected: a commit per session (`sessionsOf` depends on the device's `now`, its set of own actors and its last
  visit), and a commit per day (every device would make one, so they would be concurrent duplicates).

**Tree.** It holds exactly the files of the manifest.
- Paths: the index materialised at its heads. A homonym suffix always uses the hex id, never the member name (the
  member list lies outside the manifest).
- Names: converted to NFC, then checked again with the ADR 0023 validator (it refuses `.git`, `..` and aliases).
- Text: the UTF-8 of the Automerge text, LF, no BOM. The on-disk EOL and encoding are device-local and are never
  used.
- Binaries: the bytes of the BLAKE3 store, as plain blobs. LFS is rejected: other people would have to install a
  client, the server needs the LFS API, and go-git has no support for it.
- Modes: only `100644` and `40000`. Empty folders disappear.
- Left out: trashed files, comments (CM-8), and `.inkwell/`.
- A file over 100 MiB, or listed in `missing`, is left out and named in a trailer. A blob missing only on this device
  means this device does not send the version: it fetches the blob from peers first. It never sends a different
  tree.

**Commit bytes.**
- `author` and `committer` are identical:
  `<who, without <>\n\r\0, or "Inkwell"> <d-<hex16(device key hash)>@inkwell.invalid> <floor(at/1000), clamped to [0, 2^33)> +0000`.
- The identity comes from the creating change, which G12 signs. The time zone is always `+0000`: it is not shared
  data, and this way it leaks nothing.
- The message is: the name (one line, ≤ 200 bytes), a blank line, the note (LF, ≤ 64 KiB), a blank line, then the
  trailers `Inkwell-Version: <key>`, `Inkwell-Map: 1` and `Inkwell-Omitted: <path>`*.
- There is no `encoding` header and no `gpgsig`.

**Parents.** The parents of V are the maximal versions W such that:
- W's creating change lies in the causal past of V's creating change. That past is fixed forever, so a record that
  arrives late cannot re-parent V;
- and W's manifest ⊆ V's manifest.

They are sorted by commit id, and V is a merge commit when there are several. All parents are computed in one causal
pass (budget in P-GIT-BENCH).

| Case | Git |
|---|---|
| linear | `P → A → B` on the branch chosen at link time |
| two versions named offline at the same time | `P → A`, `P → B`; the branch reaches both through a join (§4) |
| restoring a version (HI-2) | "Avant retour à X" (current tree), then the old content: a fast-forward, nothing rewritten |
| naming a past moment | a side commit from the version it contains, joined at the next send |
| variante (G2) | branch `variantes/<id8>-<slug>`, starting from the commit of its starting version |
| « Intégrer » (applied difference) | target version with parents `[target tip, variante's integration-point version]`, plus `Inkwell-Variant` |
| abandoned variante | branch left as it is: **inkwell never deletes or forces a ref** (this departs from VA-10) |
| a file renamed or deleted | a tree change; Git's heuristics show it |

## 3. Determinism: hazards and guards

| Hazard | Guard |
|---|---|
| clocks, time zones | only the frozen `at`, `+0000`, clamping |
| identities drifting | `who` frozen; email taken from the signed key |
| EOL, BOM, encoding, OS | only the Automerge text, never the disk |
| NFC/NFD, case, homonyms | NFC plus ADR 0023 folding; hex suffix |
| modes, symlinks, executables, submodules | two modes; anything else cannot be represented (§5) |
| records that arrive late | frozen reading plus causal parents |
| a blob present on some devices only | `missing` is frozen; otherwise the device does not send |
| format changes | `Inkwell-Map` is per record; an unknown map is not sent ("mettez Inkwell à jour"); map-1 encoders are kept forever |
| hash algorithm | `objectFormat: sha1` in the link decision |

**Proof.**
- Golden vectors in `protocol/vectors/gitmap/`: (manifest, record) → (tree id, commit id), checked three ways:
  - by the Go encoder;
  - by an independent Python encoder (`test:py`);
  - in CI only, by `git mktree`, `git commit-tree` and `git fsck --strict` (a development tool, not a runtime one).
- A property test: the same random history on two simulated devices, delivered in different orders, gives the same
  ids.
- Mutants, each to be killed by a vector: wrong sort order, local time zone, disk EOL, member-name suffix.

## 4. Anyone sends, at the same time

```
push(line): repeat ≤ 5, jittered back-off
  R := advertised refs/heads/<branch> (zero if absent); fetch only it (explicit refspec)
  classify each commit in R..known (≤ 10 000):
     marker of a record not yet received   -> stop: "en attente de synchronisation"
     join whose parents are all known       -> recompute; must match, otherwise outside
     anything else                          -> outside: stop "Récupérer d'abord" (G1) / pull (G3)
  T := R; for t in tips(line) sorted by id:
     t reachable from T: skip; T reachable from t: T := t; else T := join(T, t)
  T == R: done.  Send objects(T) \ objects(R); update old=R new=T (Atomic when tags).  ng: loop
```

**The join.** `join(R, t)` has parents `[R, t]`. Its tree is the union of the two manifests, which is the state the
CRDT has already merged. Its author is `Inkwell <join@inkwell.invalid>`, its time the later of the two parents', its
message `<name R> + <name t>` plus `Inkwell-Join: 1`.

**Why it never diverges.**
- The ref moves only by compare-and-swaps whose new value contains the old one. The remote is therefore a chain of
  fast-forwards, in the total order the server imposes.
- A version is always the same object, so "send only what is missing" is just the pack negotiation.
- Two devices that know the same versions make the same join; the second push is a no-op.
- A retry follows only someone else's success, so the loop always makes progress.

**Why joins at all.** When records can arrive late, no function of shared data alone can pick a tip that
fast-forwards every tip another device may already have pushed. So the remote has to act as the sequencer.

**Cases.**
- Peers not yet synced with each other: X pushes A; Y has only B and waits on A's marker. After sync both make the
  same `J(A, B)`.
- The remote was changed with plain Git, or rewritten: handled by G3, starting from the newest common inkwell
  ancestor. With no common ancestor the history is "unrelated", and the send is refused.

## 5. Pull (G3)

**What is computed.**
- The **base** is the merged manifest of the newest inkwell-known commits reachable from R.
- The import is the tree difference base → R. Commits are not replayed, so a merge or a rebase made outside needs no
  special case.

**How each change goes in.**
- Text files: `integrate(handle, baseHeads, newText)`. The edit is made as of the base and then merged, so live
  typing is never disturbed.
- New files: new nodes.
- Binaries: `PutBlob`, within the quotas.
- A rename is kept (same DocId) only when the blob id is identical. Anything else is a delete plus an add.
- Deletions follow `settings.deletion`. In "approval" mode they become requests, and the screen says the file stays
  in sends until an admin decides.
- A symlink, a submodule, an exec bit, a name refused by ADR 0023, or a file over 100 MiB makes the pull **refused**,
  with the list of paths. It is never silently degraded, because the next send would then delete those entries.

**Imported once, with Git as the lock.**
- The importer stages, durably before anything is sent (D-5):
  - the changes, under a **new epoch** of its own key (signed per G12; it never collides with the seq numbers of
    live typing);
  - an auto version "Récupéré du dépôt", with `fromGit: R` and the Git authors as declared claims.
- Its commit Y has parents `[R]`, plus the derived parents that R does not reach. It is pushed with `old = R`.
  - If the compare-and-swap succeeds, the staging is published.
  - If it fails, the importer fetches again. If another Y′ now covers R, it drops its staging and waits for sync.
- A crash after winning: the staging is replayed at the next start.
- The winner lost for good: an admin chooses "Ignorer l'envoi incomplet", which treats Y as an outside commit.

**Roles.**
- The link is an admin decision `git/link {url without userinfo, branch, objectFormat, map}`.
- Sending and fetching need the writer role: sending may add a version, and fetching writes text.
- Without a link, nothing changes.

## 6. Security

- **A hostile remote.** Its bytes are untrusted, like a peer's.
  - Caps: pack ≤ 1 GiB, object ≤ 100 MiB, depth 32, ≤ 10 000 entries, ≤ 10 000 commits walked.
  - Every path goes through ADR 0023 plus a mode allowlist, and the walker and the classifier are fuzzed.
  - Objects are kept in a bare store only: no checkout, no worktree, no remote ref name ever used as a path, no hooks.
  - A rewrite is imported as a visible change that can be undone; inkwell's own history is never rewritten.
- **A hostile member.**
  - Cannot forge someone else's version, because the creating change is signed.
  - Cannot fork the mapping, because the frozen reading ignores later edits.
  - With their own Git credentials they can do anything to the remote, as they could without inkwell.
  - A forged marker only makes others wait, visibly, until an admin chooses "Ignorer".
- **Credentials.**
  - They are kept per device in the OS secret store (the VK-1 component; Secret Service through the `godbus` already
    in `go.mod`).
  - They are never synced, never stored in the shared document, never logged, and never written as a plain file
    (with no secret store, inkwell asks each time).
  - v1 uses an HTTPS token. G4 adds SSH, with known_hosts and the fingerprint shown on the first contact.
- **Never sent:** keys, members, decisions, attestations, comments, `.inkwell/`. A protected project (§6.10) cannot
  be linked in v1.

## 7. What the person sees

The words "dépôt Git" appear only on this surface, which the person has chosen to use.

- **Project settings** › "Copie dans un dépôt Git (facultatif)": address, branch, and "Vérifier" (reachable, rights,
  SHA-1, contents inkwell can represent). The access key is entered on each computer.
- **Versions tab**: one status line ("Dépôt : à jour", "3 versions à envoyer", "Le dépôt a reçu 2 modifications
  faites ailleurs") and the buttons "Envoyer" and "Récupérer".
- **Toasts**: "Envoyé : 3 versions", "Récupéré : 2 modifications de Jean Dupont".
- **History**: "Récupéré du dépôt — modifications de Jean Dupont (via Marie)", the Git name marked as declared.
- **Failure messages**, each with its action:
  - "Le dépôt refuse l'accès — vérifier la clé d'accès";
  - "Dépôt injoignable — rien n'est perdu, réessayer";
  - "Marie a envoyé des versions que cet ordinateur n'a pas encore reçues — l'envoi reprendra après la
    synchronisation";
  - "Le dépôt contient un lien symbolique (`build/x`) qu'Inkwell ne sait pas garder".
- **Offline**: the buttons say why they are unavailable. Nothing is queued silently.

## 8. Blocks

| Block | Content | Rigour, executable spec |
|---|---|---|
| G0 | project manifests, frozen reading, `who` (planned in HI-3) | standard: TS vectors, frozen-reading property |
| **G1a** | `internal/milestone`: encoder, projection, "Exporter l'historique en dépôt Git" (bare repo or bundle); **no network** | standard: §3 vectors (Go, Python, git oracle), mutants, P-GIT-BENCH |
| **G1b** | link decision, token, push of the main line with joins | **complete**: TLA+ `GitPush` (*FastForwardOnly*, *OneObjectPerVersion*, *EventuallyAllSent*); mutants: force, no old-id, join that depends on the device's clock, no classification; netsim against `git-http-backend` |
| G2 | variantes as branches, integration merges, tags `<branch>/<name>` | standard, on the G1 model |
| G3 | pull | **complete**: TLA+ `GitImportOnce` (crashes, races; mutants: publish before the compare-and-swap, reused epoch, no wait on a marker); fuzzing; vectors for the diff → edits |
| G4 | SSH; signed commits (deterministic SSHSIG at naming, stored in the record, only once enabled, so older ids do not change) | complete, for the keys |

**What would make the idea a bad one.**
- The mapping is a compatibility contract that lasts forever.
- G0 is real work.
- G3 touches the webview store and the G12 epoch budget.

If the G1b model or P-GIT-BENCH misses its budget, keep **G1a only**: an export that the person pushes with their own
tool, and outside edits coming back through the folder (that path already exists).

**Rejected:**
- a commit per session, per day or per change;
- per-person branches (several histories);
- deterministic import changes under a synthetic actor (G12 refuses them, and their content cannot be checked);
- a claim in the CRDT to pick the importer (racy without consensus);
- force or force-with-lease;
- worktrees;
- LFS;
- a system `git`;
- the disk as the source of the tree.

## Open questions for the owner

1. Is pull part of R1? *No: G1a, then G1b after R1, then G3.*
2. W71 says "signed commits". *v1 unsigned (inkwell keeps the proof); G4 adds deterministic SSH signatures for
   versions named after it is turned on.*
3. Who sets the link? *An admin; writers send and fetch.*
4. Should "Envoyer" create an automatic version? *Yes.*
5. A repository with symlinks, exec bits or submodules? *Refuse and list the paths; revisit only if real projects
   need exec bits (that needs a node attribute and an ADR).*

## Sources (read 2026-11-14; "summary" = search result only)

- Git internals: git-scm.com/book/en/v2/Git-Internals-Git-Objects; docs/signature-format (OpenPGP only; the SSH form
  is inferred); docs/git-push; docs/pack-protocol; docs/git-fsck.
- `git-config` `core.*`: truncated, **not read**.
- SSHSIG: openssh-portable/PROTOCOL.sshsig.
- go-git: COMPATIBILITY.md; v5.19.3 `options.go`, `tree.go`, `commit.go`, `go.mod`; releases (GitHub API).
- go-git CVEs (summary): advisories.gitlab.com …/CVE-2026-71556, …/CVE-2026-71557; nvd.nist.gov CVE-2026-45571.
- Overleaf: docs.overleaf.com …/git-integration and …/github-synchronization.
- Radicle: radicle.dev/guides/protocol. Jujutsu: docs.jj-vcs.dev/latest/glossary.
- Summaries: HackMD (hackmd.io/@docs/sync-a-note-with-github), Obsidian Git (github.com/vinzent03/obsidian-git),
  Patchwork notebook, sedimentree notes.
- GitHub: docs.github.com about-large-files-on-github and about-commit-signature-verification. SHA-256 hosting: no
  official source found.
- Git LFS specification: **not read**.
