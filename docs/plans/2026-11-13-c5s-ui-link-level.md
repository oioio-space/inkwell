# C5S-UI - the link level in the people panel (W33)

Rigour: standard (method v2: display of an already verified value; no peer bytes, no protocol change). Scope: one line
per person in `PeopleDialog`. Out of scope: the avatar stack, the header's sync indicator (SI1c), any Go change.
Order: after PR1 is committed (both touch `PeopleDialog.svelte`; one block in implementation at a time).

## 1. Need

W33 (`docs/exigences.md:51`): a person sees, simply, how they are linked to each other person (directly or through the
relay) and how good the link is. C5S delivered the measured level (good, fair, poor, unknown; ADR 0036 « Levels »);
`PROGRESS.md` row 2 and line 94 keep one item open: showing it in the people panel.

## 2. Current state (read in code, 2026-11-13)

| # | Fact | Evidence |
|---|---|---|
| S1 | Go computes the level per peer and forces `unknown` on a relayed link | `internal/bridge/links.go:45-54` |
| S2 | It reaches the webview (Links, kind 20) with `path` (`unknown, lan, direct, relayed`) and `quality` (`unknown, good, fair, poor`) | `frontend/packages/protocol/src/envelope.ts:283-311` |
| S3 | The store keeps it (`quality: null` for unknown) | `frontend/src/workspace/workspace.svelte.ts:302` -> `frontend/src/net/links.svelte.ts:19-31,43-47` |
| S4 | **It stops there.** The only reader is the e2e hook `__inkwellLinks`; `links.of()` (`links.svelte.ts:64`) has no caller in `src/` | `frontend/src/App.svelte:290`, `frontend/src/workspace/observe.ts:47-49` |
| S5 | The panel shows « En ligne · main.typ » or « Hors ligne » only; `rowsOf` knows nothing of links | `frontend/src/ui/people/PeopleDialog.svelte:17,40-49,118`, `frontend/src/ui/people/roster.ts:49-69` |
| S6 | Trap: `links.of()` reads a plain `Map`, not reactive; only `links.list` (`$state.raw`) is | `links.svelte.ts:35-38,64-66` |
| S7 | An idle link is `unknown` by design (under 200 packets in 30 s), as is a new one for its first 15-30 s | `docs/adr/0036-link-probe.md:112-113,155,331-332` |
| S8 | The interface already says « Lien lent » (send queue older than 2 s, sync indicator): a separate signal | `links.go:47-48`, ADR 0028 D8 |

Missing: a pure mapping (person -> what to show), its wiring in the row, five texts, tests.

## 3. Research

- Jitsi Meet shows a level per participant on each thumbnail (« Good », « Nonoptimal », « Poor », « Lost »), detail on
  demand: https://jitsi.github.io/handbook/docs/user-guide/client%20connection%20status%20indicators ,
  https://raw.githubusercontent.com/jitsi/jitsi-meet/master/lang/main.json (`connectionindicator.quality`).
- Stream Video SDK: `unspecified, poor, good, excellent` per participant; « consider showing warnings only for
  poor/unspecified to reduce visual noise »: https://getstream.io/video/docs/react/ui-cookbook/network-quality-indicator/
- Discord: green, yellow, red on the « Voice Connected » panel, figures on hover (third-party description, thresholds
  unverified): https://support.discord.com/hc/en-us/community/posts/360049324853-Discord-gives-me-5000ms-ping
- Zoom and Google Meet: no official page found for a per-participant indicator (searched 2026-11-13); they warn about
  one's own connection only. Not used as evidence.
- WCAG 2.2 SC 1.4.1 and technique G14 (information conveyed by colour is also in text):
  https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html
- GNOME HIG, writing style (words familiar to the people using the app; no sentence built across controls):
  https://developer.gnome.org/hig/guidelines/writing-style.html ; libadwaita `.dimmed`, `.warning`:
  https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/style-classes.html

Beyond the existing: call apps draw bars or dots because a video tile has no room for words, and their level is always
known because media always flows. Our row has a text line and our level is often unknown (S7). So: **words instead of
a bar glyph** (no new component, nothing to learn, G14 met by construction), **emphasis only on poor** (Stream's
advice), and **unknown shows nothing about quality** instead of a permanent « unknown » that would read as a fault.

## 4. Design

One more dim line under the state line of a row, same style as `.state` (`--fg-dim`, 13 px), class `.link`,
`data-testid="person-link"`. Segments joined by « · » as the state line already does (UX-LUP-3).

| Key | fr | en |
|---|---|---|
| `people.link.direct` | Directement | Direct |
| `people.link.relayed` | Par un relais | Through a relay |
| `people.link.good` | Bon lien | Good link |
| `people.link.fair` | Lien moyen | Fair link |
| `people.link.poor` | Lien faible : les modifications arrivent en retard, rien n'est perdu | Poor link: changes arrive late, nothing is lost |

« Directement », « par un relais » are the design spec's words (`docs/design/spec.md:941,3464`); « relais » is in the
glossary; « lien » follows « Lien lent » (S8); « rien n'est perdu » follows `status.offline`. `lan` and `direct` both
read « Directement » (the difference is jargon). Poor only: text in `--warning` plus the existing `warn` icon
(`Icon name="warn" size={12}`, `aria-hidden`), as suspended rows already use `--warning`. No new token, icon or
component; no tooltip (not reachable by keyboard); no live region (the level may change every few minutes, ADR 0036:331;
a change is read when the person next visits the row).

## 5. Rules (testable)

Pure function in `roster.ts` (`linkOf(member, connected, links)`, result carried by `Row.link`), fed from `links.list`.

- L-1 A link line exists only for a row that is online, not suspended and not one's own.
- L-2 Device choice: among the person's devices that are connected and have a link, the one with the worst known level
  (poor > fair > good); if none is known, the first in device order. Path and level come from that one device.
- L-3 Path segment: `lan`, `direct` -> direct; `relayed` -> relayed; `unknown` -> none.
- L-4 Level segment: `good`, `fair`, `poor` -> their text; `null` -> none. No smoothing, no memory of a past level:
  the hysteresis is the probe's (one source of truth).
- L-5 A relayed link never shows a level, even if the store held one (defence in depth for S1).
- L-6 No segment -> no line at all (the row is exactly today's).
- L-7 The line follows a new Links report without reopening the panel (S6).
- L-8 Poor is told by words and icon, never by colour alone; the level never changes the state line nor the stack.

Decision table (online, other person): direct+good « Directement · Bon lien »; direct+fair « Directement · Lien
moyen »; direct+poor « Directement · Lien faible : … »; direct+unknown « Directement »; relayed+any « Par un
relais »; unknown path+level: the level alone; unknown+unknown: no line.

## 6. Tests (cheapest layer first)

Pure, `src/ui/people/roster.test.ts`:
- `C5S-UI L-1 only an online, not suspended, other person has a link line`
- `C5S-UI L-2 several devices: the worst known level wins, else the first connected`
- `C5S-UI L-3 L-4 every path and level pair gives the table's segments` (all 4 x 4 pairs, exhaustive)
- `C5S-UI L-5 a relayed link never shows a level`
- `C5S-UI L-6 nothing known gives no line`

Mutants in-test, `src/ui/people/roster.mutants.test.ts` (PR1 pattern: one text edit on a copy, anchor asserted once):
`C5S-UI mutant <M> is caught` for RelayedShowsLevel, BestWins, OfflineShown, PoorReadsFair, LanHidden.

Component, `src/ui/people/PeopleDialog.test.ts`:
- `C5S-UI the row reads « Directement · Bon lien » under the state line`
- `C5S-UI L-7 a new Links report changes the open panel`
- `C5S-UI L-8 a poor link has its words and an aria-hidden icon`
- existing `UX-LUP-3 …` and `ACL-*` unchanged (own row and state line untouched).

Catalogs: existing `UI-10 …` tests of `src/ui/i18n.test.ts` (same keys in both, no text in markup).

e2e, `e2e/people.spec.ts`, one added assertion in S7 (real loopback link; the level depends on traffic, so the level
is not asserted there): `rowOf(alice, "Paul")` has a `person-link` whose text starts with « Directement ».
Baselines: **`people-admin-light-chromium-linux.png` changes** (Paul's row gains a line); mask `.link` like `.avatar`
(its text varies with traffic). `people-writer-dark` (own row) and `people-suspended-dark` (banner) do not change;
no `shell.spec.ts` capture opens the panel. No netsim run: the level itself is proven by C5S.

## 7. Draft gate (`gates/C5S-UI.toml`, to freeze before the code)

1. `rules`: `cd frontend && ../scripts/heavy.sh timeout 600 pnpm exec vitest run --configLoader runner --reporter=json
   --outputFile=../.task/c5sui.json src/ui/people/roster.test.ts src/ui/people/roster.mutants.test.ts
   src/ui/people/PeopleDialog.test.ts src/ui/i18n.test.ts`, then the named tests of section 6 read back as passed.
2. `single-reader`: a Python check that, outside tests, only `roster.ts` and `PeopleDialog.svelte` read `quality` of a
   link, and that no source under `frontend/src/ui` calls `links.of(` (S6).
3. `e2e`: `cd frontend && ../scripts/heavy.sh timeout 900 pnpm exec playwright test e2e/people.spec.ts --retries=0
   --reporter=json`, read back (skip or not-run fails).
4. `regression`: `mise run fe:check`.
5. `ux-walkthrough`: `test -s gates/C5S-UI/UX.md && grep -Eq '^Verdict: ship' gates/C5S-UI/UX.md` (light and dark
   captures of good, fair, poor, relayed, unknown, at 360 px and 600 px wide; a short session with the user, SEQ).
6. `simplify-report`: `test -s gates/C5S-UI/SIMPLIFY.md`.

Documents at the end: `PROGRESS.md` row 2 and line 94, `docs/exigences.md` W33.

## 8. Open product choices (decided here, to log in `docs/autonomous-decisions.md`)

- Unknown shows no level (alternative: « Mesure en cours… »; rejected, an idle link never finishes measuring, S7).
- Nothing on the avatar stack, even for poor (alternative: a badge; rejected for now, a boundary link would flicker
  in the header).
- « Lien faible » beside the existing « Lien lent »: two signals, two close wordings; to confirm in the user session.
