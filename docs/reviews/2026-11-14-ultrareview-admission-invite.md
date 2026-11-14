# Cloud review of the admission frontier and the invitation (2026-11-14)

Scope: the 21 source files of `frontend/src/net`, `internal/invite`, `protocol/invite` and
`frontend/packages/protocol/src` on `main` 4b27a6c (4 293 lines), reviewed by `/code-review ultra` (launched by the
owner). Ten findings. The reviewing session re-read the cited code for nine of them (column « Read »); **none has
been replayed by a test yet**: each owning block writes its failing test first.

| # | Rigour | Where | Finding | Read | Block |
|---|---|---|---|---|---|
| 1 | complete | `internal/invite/flow.go:411-427` | `admit` signs and broadcasts the Add of the new member before checking that the Welcome fits in `MaxReply`; with a list too large the person becomes a signed member, receives a refusal, and the ticket is consumed | yes | IV1 |
| 2 | complete | `internal/invite/flow.go:371-383` | an `Ask` stays acceptable after the visible window ended or the asker left: the admin can sign an Add for a device that will never receive the Welcome | partly: the wait on three channels is confirmed; that the context is the node's (`p2p/lobby.go:75`) is not | IV1 |
| 3 | complete | `frontend/src/net/admission.ts:117-181` | `parsePeerMessage` is a second, private decoder that does not call `decodeRepoMessage` of `@inkwell/protocol`, against the invariant « no private copy of the protocol's codecs » | partly: no call, confirmed; the rule gaps (file channel, name length, history kinds) are not | AD1 |
| 4 | complete | `frontend/src/net/scope.ts:38-39` | the admitted set is capped at 257 documents, shared by everyone and silent: one writer can fill it with fake variants and later legitimate variants stop syncing without notice | yes | AD1 |
| 5 | standard | `frontend/src/net/links.svelte.ts:64-66` | `of()` does not age `oldestPendingMs` as `view()` does, so « Lien lent » may never show on a stuck send; whether Go sends the ages alone (`stableLinks`) is to replay | yes (first part) | NA1 |
| 6 | standard | `frontend/src/net/WailsNetworkAdapter.ts:259` | the reconnect timer is neither cancelled nor guarded by `#closed`: a `disconnect()` during the delay still reopens a stream | yes | NA1 |
| 7 | standard | `frontend/src/net/WailsNetworkAdapter.ts:241`, `:458` | `forgetPeer` is not called on two reset paths: the first message of an honest peer after a reconnection can be wrongly refused | yes | NA1 |
| 8 | complete | `frontend/packages/protocol/src/cbor.ts:234-243` | `encodeCBOR` writes a corrupt map when a property is `undefined` (key counted and written, value missing) | yes | AD1 |
| 9 | complete | `frontend/src/net/conformance.ts:27` | the TypeScript default conformance step is 1 while Go applies `CurrentStep` (3): CI does not enforce the vectors of steps 2 and 3 in TypeScript, and an empty value gives 0; predates the reviewed diff | yes | AD1 |
| 10 | light | `repo.ts:79-81`, `envelope.ts`, `cbor.ts` | `hex()` and the UTF-8 encoders are duplicated | no | AD1 |

## Disposition (integrator, 2026-11-14)

Three blocks, on `main`, in this order, each with its specification and frozen gate first:

- **IV1 — invitation** (1, 2), rigour complete, R1 blocker (first contact with a stranger's bytes and a signed
  membership change): rule « a refusal changes no state » (nothing signed or broadcast before every check that can
  refuse has passed) and « an Ask ends with its window and its stream ». First, replay finding 2's unconfirmed part.
- **AD1 — one decoder** (3, 4, 8, 9, 10), rigour complete, R1 blocker: the adapter calls the protocol's decoder; the
  file channel and a per-author quota of admitted documents enter the public specification (`docs/protocol/`, the
  ten points), with a visible signal when a quota is reached; the conformance step is shared with Go; the CBOR
  encoder refuses `undefined`. First, replay finding 3's unconfirmed rule gaps.
- **NA1 — adapter and links** (5, 6, 7), rigour standard: named tests, no heavy gate. 6 and 7 touch the same code as
  PC1's restart and W58a's lock: NA1 comes after W58a is merged.
