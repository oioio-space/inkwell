# Protocol conformance audit (2026-11-14, multi-agent, read-only)

Brief: `docs/plans/2026-11-14-protocol-audit-brief.md`. Eight auditors, two independent refuters per finding batch, a completeness critic. Full results (evidence, scenarios, fixes, refuters' reasons): `2026-11-14-protocol-audit.json`. **Not yet triaged into blocks**: next session places each kept finding in TR1, IV1, AD1, NA1, BR1, an ADR 0034 step or a new block, and replays it with a failing test first.

Run log:

- P1: 7 reported, 5 kept after two refuters
- P2: 10 reported, 2 kept after two refuters
- P3: 14 reported, 10 kept after two refuters
- P7: 6 reported, 2 kept after two refuters
- P5: 18 reported, 15 kept after two refuters
- P6: 12 reported, 11 kept after two refuters
- P8: 13 reported, 7 kept after two refuters
- critic: 2 reported, 1 kept

Totals: {'kept': 53, 'critical': 6, 'major': 9}.

| Part | Severity | Finding | Block proposed |
|---|---|---|---|
| P1 | minor | KeyFromID accepts non-canonical PeerIDs: one member key gives unboundedly many admitted identities | new |
| P1 | minor | Lobby Ask runs a 64 MiB Argon2id per stranger key before any cap: host memory and CPU exhaustion | IV1 |
| P1 | minor | No vector or test pins the R rules of §3.3 for a valid key (cofactorless equation, byte comparison, R not validated) | new |
| P1 | minor | The ticket codec does not validate the host key at decode (§3.2 entry point « hôte d'un ticket ») | IV1 |
| P1 | minor | Vector schema and expectations of the §3 corpus are not reproduced, and the application decoders are not run against them | new |
| P2 | critical | A reader can starve every peer's document sync: costly key validation runs before dedup, with no per-peer budget or sanction, and control frames always go before repo frames | ADR 0034 step 5 |
| P2 | major | The vectors do not pin the rules of §4.1, §4.3 and §4.9: 8 trust, 3 attestation and 7 fold vectors, which a simplified fold passes | TR1 |
| P3 | critical | A reader's sync message makes the webview run Automerge decodeChange/load on unbounded RLE/deflate content before any bounded parse | ADR 0034 step 8 (guard G1), with AD1 |
| P3 | critical | Coverage keeps phantom heads after eviction: a Byzantine writer grows #covered/#open without bound | ADR 0034 step 6 (attestations par signataire, ≤ 256 têtes) |
| P3 | minor | Go and TypeScript CBOR profiles read a leading U+FEFF differently: TextDecoder strips the BOM | AD1 |
| P3 | minor | Coverage tracks attestations for documents outside the admitted set; at the bound it refuses new ones first-come, so the per-signer share is not reserved | AD1 |
| P3 | minor | The not-ready hold accepts a first message of up to 64 MiB per peer, against the 8 MiB bound | ADR 0034 step 6 |
| P3 | minor | The vector runners do not prove the refusal reason or check order: TypeScript ignores it, Go accepts two catch-all reasons | AD1 |
| P3 | minor | Several §9.2.1/§9.2.3 bounds have Go unit tests but no shared vector, so the TypeScript decoder is unproven on them | AD1 |
| P3 | minor | protocol/repo uses its own literals instead of the exported §16 constants that TestLimitsMatchSpecification pins | new |
| P3 | minor | Points a third-party implementer must guess in §9.2: item counting, the nested ephemeral budget, check order for size | AD1 |
| P3 | minor | Undocumented webview behaviour on the wire path: ephemeral count must increase, re-arrive triggers re-attestation of every document, presence envelopes are kept with extra keys | NA1 (count, re-attest), AD1 (envelope) |
| P5 | minor | Bulk frame and stream bounds of §16 are not the ones the code enforces, and protocol/limits.go's bulk constants are dead | ADR 0034 step 5 |
| P5 | major | Bulk stream accepted from a non-member during the invitation window | IV1 |
| P5 | major | Requester sends more than 2 requests at a time to one peer; server answers BUSY instead of treating it as a violation | ADR 0034 step 5 |
| P5 | major | Serving capacity fixed at 8 with no reserved share per peer; the 32-BUSY bound is global, not per peer per minute | ADR 0034 step 5 |
| P5 | minor | A SLICE that does not carry the requested offset and length is accepted, and short answers stall the request with no fault | ADR 0034 step 5 |
| P5 | minor | Size announced by bao checked after allocation and full Bao decoding | ADR 0034 step 5 |
| P5 | major | Size-mismatch handling in code (confirmSize sub-exchange, no fault, discard on final) is not what the spec says | ADR 0034 step 5 |
| P5 | minor | Unsolicited answers are never counted as faults | ADR 0034 step 5 |
| P5 | minor | BUSY handling departs from the spec (wait capped at 5 s, off-by-one count never reset, 8 attempts across peers instead of 3 per holder), and the tests pin these departures | ADR 0034 step 5 |
| P5 | major | Malformed bulk frames are faults, not connection violations, and are ignored entirely while the peer is banned; faults never age | ADR 0034 step 5 |
| P5 | minor | Slice verification and extraction live in the application, not in protocol/blob; internal/blob keeps private copies of codec rules | ADR 0034 step 5 |
| P5 | major | The vectors cover framing only: Bao verification is in no vector, and slice-final-group's expectations are never checked | ADR 0034 step 5 |
| P5 | minor | A peer that may not be served gets silence instead of MISSING | ADR 0034 step 5 |
| P5 | minor | Wire rules and bounds in the code that §8.1 and §16 do not state | ADR 0034 step 5 |
| P5 | minor | No formal model for the stateful bulk part, and no written reason | ADR 0034 step 5 |
| P6 | major | During an invitation window a stranger is fully admitted to the sync and bulk streams, not only the lobby | IV1 |
| P6 | minor | Ask: the joiner never authenticates the host, so a visible LAN device can pose as a project | IV1 |
| P6 | minor | The ticket decoder does not check that the host is a valid key, nor that the addresses are multiaddrs | IV1 |
| P6 | minor | The Go codec and the TypeScript reference accept different lobby and ticket bytes | IV1 |
| P6 | major | The lobby has accept-only vectors, no model and no recorded mutation score | IV1 |
| P6 | minor | The invitation window has no bound or reserved share for strangers: Sybil keys fill the inbound connections and the 8 Ask slots | IV1 |
| P6 | minor | The lobby stream limit is 4 in the spec and in protocol.LobbyStreams, but 2 in a private copy in p2p | new |
| P6 | minor | Lobby behaviour that the spec does not describe: host states and bounds, silent resets, joiner checks | IV1 |
| P6 | minor | Name bounds disagree: CleanName keeps 40 runes, but an op name holds at most 128 bytes, so the Add fails after the ticket is consumed | IV1 |
| P6 | minor | The ticket text is decoded more leniently than §8.3 and §1.3 allow | IV1 |
| P6 | minor | Documentation gaps for the lobby: framing, ticket unit, CHANGES entry, stale paths | new |
| P7 | minor | Relayed link: the first-hop figure is taken from an arbitrary connection to the relay, not a QUIC one | new |
| P7 | minor | Documentation drift in the reference: the PTO doc says 25 ms (spec and constant: 26 ms); the ADR counts 147 vectors (149) | new |
| P8 | critical | An unattested stranger can make Go hold about 40 000 unverified attestations per identity during an invitation window, with no total | IV1 |
| P8 | critical | Kept attestations have no global or per-signer bound: one Byzantine writer exhausts the memory of every member | ADR 0034 step 6 |
| P8 | minor | Vector compat.json#cap-unknown is accepted although its CAPS list is not strictly increasing | ADR 0034 step 4 |
| P8 | minor | The §16 constants are pinned to the table but mostly wired to nothing; implemented bounds use private copies, one with a different value | new |
| P8 | minor | The TypeScript test reference that cross-checks Go accepts and refuses different bytes for trust sync, lobby replies and tickets | new |
| P8 | minor | Vector corpus provenance and descriptions disagree with §21 and with the spec bounds | new |
| P8 | minor | Reloading the member list quarantines only ops with an invalid key; any other failure stops the project from opening | new |
| X | critical | One admitted peer freezes every peer's sync and file reception by announcing a large frame and then sending nothing: the shared receive budget is reserved on the announced length and nothing ever times out | ADR 0034 step 5 (N-5, shared non-blocking loops and per-peer shares); interim fix on sync/1 before step 6 |

Gaps named by the completeness critic:

- P4 (session, classes, flow control and fairness) is unaudited: its auditor delivered nothing. I covered only the sync/1 receive budget, the mux reassembly and the writer timers (2 findings above). Still not covered: §5.1-§5.6 rule by rule, §6.1 SET, §6.3 PRESENCE, §10.3-§10.5 and §10.7, the §11 state machines; the 1-in-4 fairness of the mux gate (mux/fragment.go:105-128) against §10.4 DRR; N-4, the single mux reader that blocks every channel when one queue is full (mux.go:14-17, 287-308; known inventaire row N-4); SyncIndicator.tla against frontend/src/net/syncIndicator.ts and its model test; netsim. P4 needs a fresh auditor.
- Session 2.0 (§5, §6.1 SET with cursors, §7.2-§7.4 LARGE and generations, §10.3 bridge credits, §11) has no code: the live code speaks /inkwell/sync/1.0.0 (p2p/node.go:83), and all 34 session.json vectors are « proposed, step-4 ». So questions 1, 3 and 4 do not apply to these rules yet. The models ConnCredit.tla and RepoGen.tla have no code to match. The P-QUIC startup check required by §5.1 (spec.md:580-583) and §16.1 (spec.md:2084) is absent: grep finds no quic.Config, receive-window setting or P-QUIC check in internal/ or protocol/. It is planned for ADR 0034 step 4, so I did not report it separately.
- R-Evict as an attack, not only an accident: evictStalest (mux/fragment.go:361-376) drops the stalest partial message of any other peer, and the attacker's own partial is exempt while it grows (keep=peer). A peer that keeps its fragments fresh can therefore make an honest writer's large message disappear silently. Inventaire R-Evict (inventaire.md:56) covers this as an open defect, so I did not re-report it. Its threat-model angle (a reader triggers it on purpose) is untested.
- Alloy Decisions.als and Registry.als model §4.4 admin decisions and ADR 0030 D2, but no decision code exists on main: internal/policy is absent (inventaire.md:19 marks it † unmerged), and only policy.json and objects.json vectors exist. Tree.als models ADR 0023 materialisation, which is local and not wire. alloy/README.md:17-19 says the model ↔ code link is « à venir ». Nobody checked these three models' mutants (scripts/alloy_check.py). Not covered: no code exists to compare them with.
- §9.6 history pages and rule H (§5.5) have only a codec: 'history-request' appears only in frontend/packages/protocol/src/repo.ts, with no sender or rate limit in frontend/src/net. The same holds for the §9.7 structural guard. P3 read §9 but kept no finding on them. They are planned for ADR 0034 step 6 and 8, so I am noting them, not reporting them.
- Fuzzing of peer-sourced decoders (CLAUDE.md invariant): I listed every Go Fuzz function. Each Go decoder of peer bytes has one: sig, trust op/attestation/message, repo, blob wire, lobby, ticket text, linkprobe, frame Read, mux fragment/split, addrbook. The TypeScript CBOR decoder has fast-check properties (frontend/packages/protocol/cbor.test.ts). No fuzzer or test exercises the receive budgets under a lying length (finding 1). Fuzz bodies and corpora were not read.
- §18 privacy was not traced. In particular, RelayOnly (relay.go:342-351, which filters advertised addresses to circuits) combined with LAN discovery (mdns.go:39-48, enabled independently by INKWELL_LAN in cmd/inkwell/main.go:79) was not checked for direct addresses leaking to the LAN against « Le mode relais seulement cache l'adresse IP ».
- §1.4.5 support and retirement policy, §1.4.6 GREASE emission and §1.4.7 package versions (N13): P8 read them, but nobody checked that the protocol module's tags and CHANGES.md follow them. §2, §15 and §22 are informative and nobody covered them.
- Wormhole Fetch (internal/invite/wormhole.go:72-90) does not check the ticket's expiry, as its own doc says. Whether every caller does was not traced. P6's coverage does not mention it.
- Nothing was run (read-only). The figures in finding 1 come from the constants as read: cost = 64 MiB - 3 per claim, so 4 claims hold 256 MiB - 12 bytes. They were not measured on a live node.
