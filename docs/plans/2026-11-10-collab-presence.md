# Collaborative presence and authorship: study and design (2026-11-10)

Goal (user request): shared editing that feels like typst.app. It must be clear who wrote what,
other people's edits must arrive reliably, you always know where everyone is, it looks like Adwaita,
it never lags at any document size, and its correctness is proven by machine.
Status: study and design only. No code changed. The blocks in §6 still need their gates.

## 1. typst.app: what can be verified

Verified from public sources:
- The web app is "a collaborative environment" with "real-time synchronization with your
  co-authors" (https://typst.app/docs/web-app/). Sharing is a link that lets anyone "view and
  optionally edit". Members keep access after the link is deleted. Teams exist
  (https://typst.app/docs/web-app/concepts/).
- Comments, Git sync, invite by email and presentation mode each have a doc page
  (https://typst.app/docs/web-app/ index).
- There is **no version history, change tracking or blame.** The roadmap lists "Version history",
  "Change tracking" and "Chat-like comments" as open (https://typst.app/docs/roadmap/). A forum reply
  says Git sync is "the only way". The Nov 2025 community call said "A priority, but complex feature
  and still in design stage" (https://forum.typst.app/t/will-changes-be-tracked-in-typst-app/1309).
- User reviews are lukewarm on collaboration at scale. On HN, one user found Overleaf better for
  collaboration. Another found the web app lacking once a team grew beyond a few people
  (https://news.ycombinator.com/item?id=45693978).

**Could not verify** (the docs, the blog, the changelog and the forum say nothing, and the app needs
an account): how remote cursors and name labels look; selection colours; avatars in the header;
any follow or jump-to mode; presence per file in the tree; latency. The launch post
(https://typst.app/blog/2023/beta-oss-launch) does not describe them either. Nothing below is
copied from typst.app. Its "feel" is taken as *an online editor where carets and edits of others
just appear*. On authorship, inkwell already goes further than typst.app does.

## 2. The best of the rest

| Product | What works | Noise or cost |
|---|---|---|
| Google Docs | A caret in the person's colour, matching their avatar (https://drive.googleblog.com/2010/10/online-office-hours-with-google-docs.html). Select text, then « Show editors »: recent editors and timestamps for that range (https://workspaceupdates.googleblog.com/2021/05/view-more-context-on-google-docs-edits-with-show-editors.html). Version history colours by author (https://support.google.com/docs/answer/190843). | « Show editors » only on paid tiers. No live per-character colours. Whether the flag fades is not documented (unverified). |
| Figma | Click an avatar to follow at once. A border in the followed person's colour, and « Stop following ». « Spotlight me » with a few seconds of « Not now ». "Not told that you're following" (https://help.figma.com/hc/en-us/articles/360040322673). | Users complain about being observed (forum, see 2026-11-09-ux-research.md §1.2). |
| Zed | Click an avatar to follow. A following pane is "outlined in their cursor color". Joining starts by following the inviter. "To stop following, simply move your mouse or make an edit" (https://zed.dev/docs/collaboration/channels). Its CRDT anchors are (insertion id, offset), "logical locations rather than absolute offsets" (https://zed.dev/blog/crdts). | Auto-follow on join can surprise people. |
| VS Code Live Share | Name tag visibility: Never, **Activity** (the default: shown on edit, select or move) or Always. Follow is per editor group, so you can follow in one split and work in another. Opening another file stops following. « Focus participants » asks everyone to follow (https://learn.microsoft.com/en-us/visualstudio/liveshare/use/coedit-follow-focus-visual-studio-code). | Auto-follow of the host. |
| Etherpad | The classic text background per author, with a per-user « Authorship colors » switch. « Clear authorship colors » wipes them for everybody, and a plugin exists to forbid that (https://www.npmjs.org/package/ep_authorship_toggle, https://cdn.jsdelivr.net/npm/ep_disable_reset_authorship_colours@0.0.26/README.md). | Always-on colour is noisy. A global wipe destroys information. |
| HedgeDoc | Authorship only in the version view: "time consuming and you cannot edit". The planned styles are underline, background or invisible (https://community.hedgedoc.org/t/autorship-colors/1340). | Shows that blame must be live *and* optional. |
| Quarto q2 (on Automerge) | Replays the history into run-length runs of (actor, time): "4× faster updates, 5× faster queries, 20× smaller" than per character. Cold start in chunks, then incremental (https://net.nhd.workers.dev/quarto-dev/q2/pull/122). | The document shows no authorship until the replay ends. A compacted history forces a rebuild. |

Takeaways:
- Label on activity, then fade (Live Share's default).
- Follow by clicking an avatar, with a coloured frame; it stops on any own action (Zed, Figma).
- Follow is never announced to the person followed (Figma).
- Authorship is a personal toggle that never wipes anything (unlike Etherpad).
- Authorship must work while editing (the HedgeDoc complaint).

## 3. What inkwell has today

| Area | Where | State | Gap |
|---|---|---|---|
| Presence wire | `frontend/src/editor/presence.ts:29-30` | automerge-repo `Presence`, state `{name,file,anchor,head}`. Cursors are Automerge cursors, so they stay stable under concurrent edits. | `file` hard-coded to `"main.typ"` (`:30`). Cursors outside `main.typ` are dropped (`:47`): **multi-file presence is broken**. No activity (typing or idle) and no viewport. |
| Liveness | automerge-repo 2.5.6 `presence/constants.js:1-2` | heartbeat 15 s, TTL 45 s (defaults; not configured at `presence.ts:30`) | A crashed or offline person stays shown up to 45 s. |
| Timing of own cursor | `presence.ts:73-89` | Sent after `whenWritten`, so it never points one character off | none |
| Early cursor | `presence.ts:56-71` | Skipped, then retried on the next `change` | none |
| Remote caret | `editor/remoteCursors.ts:17-37` (mapped through edits), `:39-61` (widget) | Caret and label, with the name always written. `aria-hidden` at `:54`. | Label never fades (spec §4.14 asks for 3 s). On line 1 the label clips (`bottom:100%`, `:102-103`). No handling of collisions or of the same spot. **No off-screen indicator.** Ink `#000` hard-coded (`:109`). |
| Tokens | `theme/tokens.css:101-108` | 8 Okabe-Ito `--person-N` | `--person-N-ink` and `--person-N-line` (spec §4.14) do not exist. Slot 8 `#999` is weak as a caret on a light surface. |
| Colour slot | `ui/people.ts:25-32` | FNV-1a(peerId) mod 8 | Hash collisions: 2 people share a colour with probability 1/8, and among 4 people at least two collide with probability 59 %. The name saves meaning, but not at a glance. |
| Header stack | `ui/PeopleStack.svelte:22-35` | Initials in the person's colour, plus a title. Opens the People dialog. | Spec §4.14: a phrased `<span>` button, not `<ul>`; at most 4 then « +N »; a presence dot. Ink hard-coded (`:69-75`). |
| Per-person file | `ui/people/PeopleDialog.svelte:118`, `workspace.svelte.ts:607-610` | The dialog shows the file each person is in | Always `main.typ` (see above). Nothing in the file tree or on the tabs. |
| Follow | spec §4.47 (full behaviour written) | Only for presentations (`present/`) | **Not built for the editor.** No jump-to. |
| Announcements | spec §4.14 « Paul a rejoint » | not found in `i18n/fr.json` | Not built. |
| Who wrote what | `editor/blameMarks.ts:21-77`, driven by `ui/history/HistoryBars.svelte:57-84` | A toggle colours passages by session author, with a `title` tooltip; a click opens the session | The `title` is hover-only (keyboard and screen readers get nothing). Colour + underline only, no non-colour mark. All spans are built as one RangeSet. Off once the analysis is `limited` or not current. |
| Authorship engine | `history/track.ts:125-166` (Tracker, 8 ms slices, incremental per closed session); `history/analysis.ts:45,53`; `history/analysis.worker.ts` | Replays `A.diff` per session in a Worker | Cost is ops × sessions: off above `BLAME_BUDGET` = 1e8, and above `HUGE_TEXT` (512 Ki chars). **So no authorship at 10 MB.** |
| Actor to person | `history/people.ts:39-69,81-97` | A self-claim change `{peer,name}` per actor. The screen says « selon l'appareil de … ». "Mine" is decided locally. | A claim is unsigned: a current writer can claim any name for its actors. |
| Change since last visit | `editor/changeGutter.ts` | Marks anchored by cursors, coloured by type or by person | Not a live « just arrived » cue. |
| Ephemeral spoofing | `net/WailsNetworkAdapter.ts:198` | Drops ephemeral messages whose `senderId` is not the link's peer | none |
| e2e | `frontend/e2e/collab.spec.ts:49-51` | Checks that the label is *visible* | Does not check the position, the latency, or that one's own caret and scroll stay put. |

## 4. Feasibility of « who wrote what » on Automerge 3.5

**APIs in the installed 3.5.0 package** (`dist/implementation.d.ts`, `wasm_types.d.ts`):
- There is no blame or attribution API.
- Available: `diff`, `diffPath` (experimental), `getChangesMetaSince`, `inspectChange` and `decodeChange` (which carry `actor` and `time`), `marks` and `spans`, `topoHistoryTraversal`.
- New in 3.x: `setAuthor`/`getAuthorForActor`/`getActorsForAuthor`. `Author = string`: an *unsigned* label, no stronger than our claim.
- The docs index (https://automerge.org/llms.txt) has no attribution page.

**Key finding (measured 2026-11-10, Node, `@automerge/automerge` 3.5.0).** `getCursor(doc, ["text"], i)`
returns the element id `counter@actor` of the character at `i`, and that actor is the one who
*inserted* the character. In a merged « >> hello world », for example, the `w` gives `8@bbbb`. This
is the CRDT's own ground truth for "who wrote this character". It needs no history replay, and
offline merges are exact by construction. It costs **7.6 µs per call** on a 1 M-char document
(5,000 calls in 38 ms; `getCursorPosition`, 9.7 µs). That is too slow to call per character on each
frame. It works well when:
- it is the **oracle** of every test;
- it attributes the **visible range lazily** (about 5 k chars, so about 40 ms, sliced to under
  8 ms per idle callback, then cached);
- it handles the inserted ranges of a multi-actor merge.

The `time` and the session come from the change holding op `counter` of `actor`: per actor, the
`getChangesMetaSince` entries are sorted by `startOp`, and a binary search finds it.
Caveat: this cursor format is not documented as stable, so B0 pins it with a test (§6).

**Proposed engine (Worker-free on the hot path):**
- *Live*: on each `handle` change, the new changes' metadata gives their actors. If they all come
  from one actor (ordinary typing or a paste), every inserted range in the patches belongs to that
  actor. This is exact and O(patch). If they come from several actors (a merge after offline),
  each inserted range is resolved with `getCursor` in idle slices. Meanwhile the view shows « en
  cours » for those ranges.
- *Store*: a run-length list (B-tree of runs, like `history/pieces.ts`) maps through
  CodeMirror changes. A local cache keyed by heads avoids a cold replay. It is a projection, not a
  source of truth (the rule « une seule source de vérité »). It is checked against the oracle on
  load by sampling.
- *Render*: a `ViewPlugin` decorates only `view.visibleRanges`, so the work per frame is
  O(visible runs) at any document size. A miss in the cache falls back to the lazy oracle.
- The existing Tracker stays for the History panel (sessions, deletions, the « since last visit »
  marks). Its 512 Ki limit no longer blocks the authorship view.
- 10 MB caveat: Automerge keeps about 128 B per character (`analysis.ts:48-52`). A 10 M-char text
  is bounded by ADR 0028's size gate, not by this view. The 16 ms budget is about rendering, and B4
  measures it at 10 MB of text where the gate allows that size.

**Actor to person (ADR 0018).** An attestation covers *heads*. A writer attests everything it holds,
including other people's changes, so attestation alone does not prove authorship. Proposal (needs
a new ADR and a protocol step under ADR 0034):
- An `Attest` also lists the actors this device writes under, and Go signs (doc, heads, actors).
- A char whose inserting actor X is bound to peer P is **confirmed** once some attestation signed
  by P covers the change. Until then it is **declared** (« selon l'appareil de Marie », today's
  wording).
- An actor nobody has bound is shown as « inconnu ».
- A revoked member's text keeps their name, marked « ancien membre ».

**Honest limits.**
1. *Moves*: Automerge has no move operation. Cut and paste is a delete plus an insert, so moved
   text belongs to the mover. The same goes for copying someone else's text.
2. *External disk edits* and whole-text `put` (`track.ts:47-59`) belong to the importing device.
   They are minimised by `updateText`'s diff, never zero.
3. *Interleaving*: concurrent inserts at the same spot can interleave characters. Authorship then
   follows the characters exactly, so it is fragmented, but it is true.
4. A *deleted* char has no current author. Its removal stays in History.
5. Attribution is per **device actor**, not per human. Several devices of one person merge only
   through the member list, as now (`authorBook`).
6. Spans are anchored by element ids, so later edits move them correctly. Nothing ever has to be
   re-derived from offsets.

## 5. Behaviour spec

### 5.1 Presence wire and state machine

The broadcast state is
`{ name, file, anchor, head, top, bottom, act: "typing"|"active"|"idle", focus: bool }`:
- `top` and `bottom` are cursors of the first and last visible line; follow uses them;
- `typing` means an edit in the last 2 s; `idle` means no input for 60 s or the window unfocused;
- `file` is the **active tab's** path, one Presence per document handle.

Heartbeat 5 s and TTL 15 s are set explicitly. Readers may send presence (ADR 0018), and the
sender is checked as at `WailsNetworkAdapter.ts:198`. Presence is never persisted.

| State of a remote person | Entered by | Caret | Label | Header avatar | File tree or tab |
|---|---|---|---|---|---|
| **Typing** | an edit, or `act=typing` | solid `--person-N-line`, 2 px | shown, with « écrit… » after the name | full colour, presence dot | person dot on the row |
| **Active** | a selection or move within 60 s | solid | shown 3 s after the last activity, then fades (200 ms; none under reduced motion) | full | dot |
| **Idle** | 60 s without activity, or `focus=false` | 1 px at 60 % | hidden; shown on hover or focus | 60 % opacity, title « inactive » | dot hollow |
| **Elsewhere** | `file` ≠ my file | none in my editor | — | full, title « dans chapitre2.typ » | dot on *their* file row |
| **Offline** | `goodbye`, or the TTL expires (15 s) | removed at once | — | removed; the dialog keeps them « hors ligne » | removed |
| **Unresolved** | the cursor's text has not arrived yet | none | — | full | dot |

Every transition happens only on: a presence message, a document change, a timer, or the 60 s idle
clock. The same inputs always give the same state, so it can be modelled (B0).

### 5.2 Carets, labels, collisions, off-screen

- **Label position.** Above the caret. On the first visible line it flips below. It is clamped
  inside the editor's width.
- **Collisions.**
  - Carets at the same offset: one caret, one label « Marie, Paul » with stacked colour chips.
  - Labels that overlap horizontally on one line: the later one is shown on hover only, and the
    caret's chip shows « +1 ».
  - At most one visible label per line.
- **Own caret wins.** A remote label never covers my own caret's line. It hides while my caret is
  on that line.
- **Selection.** Fill: `color-mix(--person-N 22 %)` in light, 30 % in dark. A remote selection never
  paints over my own selection; mine is on top.
- **Off-screen.** When a remote head is above or below the viewport, a pill sits at the top or
  bottom edge of the editor: an arrow, a coloured chip and the name. Several pills stack, at most
  3, then « +N ». A click or Enter jumps (§5.3). The pill is a real button (focusable, a name, not
  `aria-hidden`).
- **Colour slot.** Chosen by **join order in the signed member list**, so it is distinct for the
  first 8 people, with FNV as the fallback for non-members. Every caret, avatar, pill and run also
  carries the **name or initial**: colour alone never carries meaning.
- **Tokens.** Add `--person-N-ink` (AA text on `--person-N`) and `--person-N-line` (a caret colour
  ≥ 3:1 against `--view-bg`, in light and dark). Nothing is hard-coded.

### 5.3 Follow and jump

This is spec §4.47 as written, plus the following:
- **Jump** (a click on a pill, the avatar menu « Aller à Marie », or the palette): opens their file
  in a tab and scrolls their head into the middle. My caret does not move, and a focus ring flashes
  1 s on their line.
- **Follow** (the avatar menu « Suivre Marie »):
  - the view tracks their `top` and `bottom` and their file;
  - a frame of `--person-N-line` around the editor, plus the « Vous suivez Marie » bar;
  - it stops on any own key, scroll, click or tab change, and on Esc;
  - follow is never told to the person followed (Figma);
  - no auto-follow on join (the Zed and Live Share surprise).
- **Scrolling** follows them smoothly, with no animation under reduced motion. It is throttled to
  one scroll per frame and never moves my caret.

### 5.4 Arrival of remote edits

- **Flash.** Inserted ranges from a remote actor get `--person-N` at 18 %, fading over 1.2 s (none
  under reduced motion). It is capped at 2,000 chars per frame: a big paste flashes only its first
  and last lines.
- **My caret and selection** are mapped (CodeMirror `mapPos`, with assoc toward my side), never reset.
- **My scroll.** A remote insert above the viewport must keep my first visible line on screen (a
  scroll anchor on the top visible line's position). The test checks this.
- **No announcement per edit.** Screen readers hear only join, leave, start and stop of follow,
  « Marie écrit dans votre paragraphe » (at most once per 30 s, polite), and the « Où sont les
  personnes ? » command.

### 5.5 Authorship view (« Qui a écrit quoi »)

A personal toggle in the view menu, with the palette command and the shortcut kept from the
existing toggle. It never changes the document and is never shared.

| Mode | Text | Margin | Non-colour cue |
|---|---|---|---|
| off (default) | nothing | nothing | — |
| on | a 2 px underline in the person's line colour, and 12 % background on hover | the initial of the line's main author, in an 18 px gutter | each person also gets an **underline style** (solid, dashed, dotted, double, wavy and so on) and the initial; a popover names them |
| on, focus on a run | as on | as on | Alt+F1 at the caret (or the palette « Qui a écrit ici ? ») opens a popover: « Marie · hier 16:02 · confirmé », plus « Ouvrir la séance ». The same text goes to the polite region. |
| status pending | runs still resolving after a merge | a gray hatched gutter | « Attribution en cours » in the popover |

Each run carries a trust word: **confirmé** (signed binding), **déclaré** (claim only) or
**inconnu**. Remote cursors keep `aria-hidden`. The pills, the popover and the gutter are
reachable by keyboard (F6 cycles the zones: tabs, editor, follow bar). Only Adwaita tokens are used
(`--view-bg`, `--headerbar-bg`, `--accent`, `--person-N*`, the radius and font tokens).

## 6. Budgets, proofs and blocks

**Budgets** (p95, measured, never estimated):

| ID | Measure | Budget |
|---|---|---|
| CP-LAT | Remote keystroke to visible glyph, LAN (netsim `lan` profile), two real instances | ≤ 100 ms |
| CP-CUR | Presence message to caret painted (rAF after dispatch) | ≤ 16 ms |
| CP-AUTH | Authorship view on, scrolling a 10 MB text (or the largest size ADR 0028 allows) | ≤ 16 ms per frame, no long task over 50 ms |
| CP-MERGE | 1 M chars merged from 3 actors | the view shows « en cours » ≤ 16 ms; done ≤ 10 s in slices of 8 ms or less |
| CP-OFF | Offline person removed after a hard kill | ≤ 15 s (TTL); ≤ 1 s on goodbye |

**Executable proofs:**
- **Multi-instance e2e**, extending `collab.spec.ts` and `netsim.spec.ts`:
  - the caret's DOM rect sits within 1 px of `coordsAtPos(head)`;
  - the label text equals the name, and the label is inside the editor box;
  - a remote edit is visible within CP-LAT;
  - my `selection.main` and `scrollDOM.scrollTop` (± 1 line) are unchanged after 200 remote
    inserts above and below the viewport;
  - a pill appears when the remote head is off screen, and jump lands there;
  - follow stops on a key press;
  - the person is offline within CP-OFF after a kill;
  - captures in light and dark.
- **Property tests (fast-check)**:
  - random 2-to-4-actor histories with forks, offline merges, splices, deletes, `updateText` and
    whole puts;
  - for each history: authorship(engine) == oracle (`getCursor` per char), the runs cover
    exactly [0, len), adjacent runs differ, and each run's actor inserted those chars;
  - the live path and the cold path agree;
  - mapping through CodeMirror changes agrees with a recompute;
  - the presence state machine agrees with a small TS reference model (the table in §5.1).
- **Formal**: a small TLA+ model of presence liveness (heartbeat, TTL, goodbye, reconnect). Its
  invariant: no person is shown online more than TTL after their last message, and none is
  shown offline while their messages arrive. Plus an Alloy check of the confirmed/declared rule
  against forged claims.
- **Perf cases**: Vitest bench plus native measurement for CP-CUR, CP-AUTH and CP-MERGE, in the
  manner of `highlight.perf.test.ts`.
- **Mutants**, each of which a test must kill (Stryker on the frontend):
  - drop the `mapPos` of remote carets;
  - use the old offset instead of the cursor;
  - attribute a merge to the last actor;
  - skip the off-screen pill;
  - never fade;
  - TTL ignored;
  - follow not stopped on a key;
  - flash that scrolls;
  - colour slot by hash for members;
  - a declared run shown as confirmed.

**Blocks, ranked** (cost S/M/L; risk to schedule or correctness):

| Rank | Block | Content | Cost | Risk |
|---|---|---|---|---|
| 1 | **CP0 presence model** | §5.1 wire, the real active `file`, TTL and heartbeat, the state machine and its model, plus the TLA+ liveness model; fixes the hard-coded `main.typ` | S-M | low |
| 2 | **CP1 carets** | §5.2: fade, flip, collisions, off-screen pills, `-ink`/`-line` tokens, member-order slots, the header stack per §4.14; the e2e for position | M | low |
| 3 | **CP2 arrival** | §5.4: flash, caret and scroll stability, CP-LAT and CP-CUR in netsim; probably the most visible to the user | M | medium (scroll anchoring in WebKitGTK must be measured) |
| 4 | **CP3 follow and jump** | §5.3 and spec §4.47: avatar menu, palette, follow bar, F6 zone | M | medium |
| 5 | **CP4 live authorship** | §4 engine: oracle, live path, run store, viewport render, gutter, popover, trust words; CP-AUTH and CP-MERGE | L | high (perf at 10 MB; the cursor format is undocumented, so a test pins it) |
| 6 | **CP5 confirmed authorship** | new ADR plus a protocol step: actors in `Attest`, the confirmed/declared rule, vectors, the Alloy check, mutation ≥ 95 % in `protocol/*` | M-L | high (touches the signed protocol) |

CP0 → CP1 → CP2 are what makes it "feel like typst.app". CP4 and CP5 are where inkwell surpasses
it, since typst.app has no blame at all (§1).

Requirements: add the user's request to `docs/exigences.md` and a lot in
`docs/architecture/coherence.md` §5 when the plan is accepted. That is not done here: this file is
the only change.
