# SB2a1 UX walkthrough (2026-11-15)

Scope: plan rows U-1 (About line; refusal sentence in the preview). Tree: worktree sb2a1, `bin/inkwell-server`
(`-tags server`, built from the uncommitted tree, instrumented frontend), Chromium 1100x640, fr-FR, light and dark.
Every launch: INKWELL_HOME, INKWELL_DOCS, INKWELL_EXPORT_DIR and XDG_{DATA,CONFIG,CACHE}_HOME set to a fresh directory
under the session scratchpad and checked non-empty before launch (`launch.sh`); verified in `/proc/<pid>/environ`.

## How a tier is forced

No product seam exists (no variable in `.env.reference` forces a tier; `decideTier` reads facts from the machine). Forced
for real, without patching product code, with a systemd property on the process: `-p RestrictNamespaces=yes` makes the
namespace launch refuse, and the namespace-free launch is then refused too, giving the "no tier" refusal (sentence
`preview.fail.sandbox`). Tier F proper, and the protected-project refusal (`preview.fail.protected`), were not reachable
this way. The About "reduced" line was captured by answering the `NetworkMode` bridge call from the browser
(Playwright route), i.e. real UI with a stubbed Go answer, which `TestIsolationReportsTier` covers on the Go side.

## Blocking defects (severity 4)

**UX-SB2a1-1 (4): on the normal path (tier N, this machine) the preview never starts in the real app binary.**
Capture: `ux/N-light-1-preview.png`, `ux/N-dark-1-preview.png`, `ux/N-light-2-about.png`.
Repro: build `go build -tags server ./cmd/inkwell`, start it with fresh directories, open the page, type text. The
preview stays empty with "Le moteur de mise en page ne démarre pas. Relancez Inkwell ; si cela continue, réinstallez-le."
and About says "L'état de l'isolation réseau n'est pas disponible." The server log repeats, with backoff,
`typst: sidecar: namespace probe inconclusive; launch refused, retry: namespace launch inconclusive`.
Evidence: the probe scope lives 5.2 s (= `probeTimeout`) in `journalctl --user`, i.e. a timeout; the same `probeFacts()`
run from a Go test binary of `internal/sidecar` answers `NS=ok` in 10 ms; the server binary run unconfined or under
`unshare -Urn` with `--inkwell-internal-noop-v1` exits 0 in 30 ms. The probe target is `os.Executable()`; the real
binary is dynamically linked to WebKitGTK/GTK4 (`ldd`), its confined noop run does not finish. The sandbox tests use a
static-ish test binary, so they cannot see it. Expected: tier N, preview shows pages, About says "espace isolé".
Consequence: with this block a person on a supported machine loses the preview altogether; the wording also lies
("réinstallez-le" cannot help). Not walked past this point for tier N (no PDF preview, so no "compile a document").
The cause is a hypothesis (confined launch of the GTK-linked binary), not a proven line; strace is not installed here.

**UX-SB2a1-2 (4 until UX-1 is fixed, otherwise 3): "inconclusive" shows the "réinstallez-le" sentence and no About
information.** A transient probe failure is presented as a broken installation (`preview.fail.unavailable`); the About
dialog shows "non disponible", and the cause (`NSErr`) is dropped from the log line (only "namespace launch
inconclusive"), so nobody can diagnose it.

## Refusal path (rules S-9/T-1), forced with RestrictNamespaces

Captures: `ux/F-light-1-preview.png`, `ux/F-dark-1-preview.png`, `ux/F-light-2-about.png`, `ux/F-dark-2-about.png`.
Sentence shown, where the eye is (centre of the preview pane), with a "Détails" button:
"La mise en page ne peut pas démarrer : ce système ne fournit pas la protection dont Inkwell a besoin."

Personas.
- Writer wanting the PDF preview: understands that the preview will not work, not why, not what to do; no next step
  (restart? update? contact someone?). Walkthrough question 4 ("what changed / what now") is a no.
- Cautious reader of About: clicks "Détails", lands on About which says "L'état de l'isolation réseau n'est pas
  disponible." Dead end (see UX-SB2a1-3).

**UX-SB2a1-3 (3): "Détails" leads to nothing.** In a refusal the Go side reports an empty mode, so About shows the
generic "non disponible" line. The button promises detail and gives less than the sentence. Expected: About (or the
sentence) says what is missing in plain words and what to do (e.g. update the system / the package, or "ce document est
protégé, ouvrez-le sur un autre ordinateur"). Per the severity table this misleads the person and the task cannot be
finished without outside help: severity 3, blocks release.

**UX-SB2a1-4 (2): the sentence offers no action.** "la protection dont Inkwell a besoin" is vague ("quelle
protection ?") and gives no next step. Suggest: say what happens to their text ("Votre texte est conservé"), and one
action. Same remark for the protected sentence ("risquer d'exposer son contenu" is alarming and unexplained);
the protected sentence could not be captured (see end).

**UX-SB2a1-5 (2): the About "reduced" sentence is a five-line wall in the same neutral card as the full tier.**
Captures: `ux/about-reduced-light.png`, `ux/about-reduced-dark.png` vs `ux/about-namespace-light.png`,
`ux/about-namespace-dark.png`. It does not answer the cautious reader's question ("suis-je protégé ?"): it lists three
things the layout cannot do, then a "mais" about "la séparer complètement des autres programmes" (vague, near-jargon),
and no next step. Nothing visually distinguishes the states (same card, same title), so the line is easy to miss or to
over-read. Suggest a short first clause (what is guaranteed), the limit second, and optionally a quiet tint or icon
consistent with the existing banners.

Minor (1).
- After closing About with Escape, focus goes back to the editor, not to the "Détails" button that opened it.
- Focus order is fine: toolbar zoom controls, then "Détails" (4 tabs); the dialog takes focus on its Close button.
- Contrast: message text (55 % alpha) about 4.5:1 on the light pane and 5.3:1 on the dark pane (estimated from the
  blended colours), at the AA limit in light; button text high. Acceptable, no margin.
- The empty-state region is `aria-live="polite"` without a role: announced, OK. Both themes render without overlap at
  1100x640.
- Coherence: "Détails" is a plain grey button in the empty state, unlike the existing preview notice
  (`.notice`) pattern; acceptable but no link to the docs' banner vocabulary.

## What could not be walked

Tier F proper (needs a machine where only the namespace launch is refused and the F probe passes; with this binary it
hits UX-1 first), the protected-project refusal (needs tier F plus a protected project; the WebSocket frames are not
plain text, so rewriting the failure kind was not possible), the PDF preview and the document compile on tier N
(blocked by UX-1), multi-user scenarios (no collaboration is involved in U-1), 200 % zoom and forced-colors.
The new e2e assertion (`preview.spec.ts`, PRV-104) accepts `namespace|reduced` and would fail on this machine for the
reason of UX-1; it was not run (heavy job owned by another agent).

First-walkthrough verdict (superseded by the re-judgement below): fix; blocking UX-SB2a1-1 (4), -2 (4 while -1 stood), -3 (3).

## Re-judgement (2026-11-15, second pass)

Tree: worktree sb2a1m (index of block SB2a1 on main e637035), `bin/inkwell-server` and the frontend rebuilt from this tree
for this pass (vite build with VITE_E2E_OBSERVE=1, `go build -tags server`). Chromium 1100x640, fr-FR, light and dark.
Every launch through a script that sets INKWELL_HOME, INKWELL_DOCS, INKWELL_EXPORT_DIR and XDG_{DATA,CONFIG,CACHE}_HOME to
fresh directories under the session scratchpad, checked non-empty and read back from `/proc/<pid>/environ` before use;
applied to the app process only. Captures: `gates/SB2a1/ux/rejudge/` (prefix N = full tier, F = reduced tier, R = refusal,
P = retry). Every tier below was reached for real; no Go answer was stubbed.

### How each state was reached, without patching product code

| State | Real way | Evidence |
|---|---|---|
| Full tier (N) | plain launch | log `typst: offline network=namespace`, About `data-mode=namespace` |
| Reduced tier (F) | `systemd-run --user --scope`-style property `-p RestrictNamespaces=yes` | log `typst: offline network=reduced`, About `data-mode=reduced`. The refusal of the first pass is gone: this property now gives F, as the ADR says |
| Refusal (R) | `-p RestrictNamespaces=yes -p "SystemCallFilter=~landlock_create_ruleset" -p SystemCallErrorNumber=EPERM` (no namespace, no Landlock) | log `sandbox unavailable on this system; launch refused: ... mandatory Landlock unavailable (ABI 0)`, About `data-mode=refused` |
| Retry (P) | `-p CPUQuota=1%`: the probe's confined launch exceeds its 5 s limit, a real inconclusive probe | log `namespace probe inconclusive; launch refused, retry`, About `data-mode=pending`; lifting the quota with `systemctl --user set-property ... CPUQuota=` makes the next probe conclusive (`offline network=namespace`) |

No variable of `.env.reference` forces a tier or a probe timeout; these are machine conditions, not product seams.

### Walk

1. **Normal path (N), closed.** Document opened, text typed, preview shows the pages in under 1 s (`N-light-1-preview.png`,
   `N-dark-1-preview.png`): UX-SB2a1-1 is gone. Menu > « Exporter en PDF… » writes `Sans titre.pdf` (valid PDF 1.7, text
   « Procédure / Bonjour, voici le texte. / un / deux » extracted) and the toast names the file (`N-light-3-export.png`).
   About: « La mise en page tourne dans un espace isolé : elle ne peut joindre aucun réseau, pas même cet ordinateur. »
   (`N-light-4-about.png`, `N-dark-4-about.png`): calm, one fact. Persona 2 gets her answer.
2. **Reduced tier (F), real.** Preview and PDF export work exactly as in N (`F-light-1-preview.png`, PDF verified). About
   (`F-light-4-about.png`, `F-dark-4-about.png`): « Vos fichiers restent protégés : le moteur de mise en page ne peut ni lire
   vos autres fichiers ni utiliser le réseau. Ce système ne permet pas l'isolation complète : le moteur n'est pas
   entièrement séparé des autres programmes ouverts. » The guarantee comes first, the limit second, and no action is asked
   (none is possible). Better than the first pass (UX-SB2a1-5 closed as a defect; see UX-8 residual).
3. **Refusal (R), real.** Preview centre (`R-light-1-preview.png`, `R-dark-1-preview.png`): « L'aperçu est désactivé : ce
   système ne permet pas d'isoler le moteur de mise en page. Le texte et la collaboration fonctionnent normalement. » + button
   « Détails ». Four Tab presses from the editor reach « Détails »; Enter opens About with focus on its close button
   (`R-light-3-about.png`, `R-dark-3-about.png`); the line now says what is wrong, that Inkwell never runs the engine without
   protection, that text and collaboration work, and two next steps (open the document on another computer; update the system
   and Inkwell). **Escape returns the focus to « Détails »** (UX-SB2a1-3 and the minor focus remark closed). Text contrast
   (message at 55 % alpha blended on the pane): about 4.5:1 light, 5.3:1 dark; button text well above.
4. **Retry (P), real.** `P-light-1-retry.png`, `P-dark-1-retry.png`: « L'aperçu n'a pas pu démarrer cette fois. » + « Réessayer »,
   not the old « réinstallez-le »: UX-SB2a1-2 closed. Tab order: zoom controls then « Réessayer », focus ring visible
   (`P-light-3-focus.png`). About (`P-light-2-about.png`, `P-dark-2-about.png`): « L'aperçu n'a pas pu démarrer cette fois :
   Inkwell n'a pas pu vérifier l'isolation du moteur de mise en page. Choisissez « Réessayer » dans l'aperçu. » Recovery:
   after the cause is gone, « Réessayer » brings the pages (`P-light-4-recovered.png`, typed text intact), About then says
   `namespace`. Seen as « … » only while the Go answer is slow (1 % CPU), then the sentence arrives (30 s there).
5. **Placement.** The sentences sit in the centre of the empty preview, where « Préparation de l'aperçu… » already sits, with the
   same grey text and the shared `.btn` (24 px radius, grey card, same hover). No `.notice` strip is used and none is shown
   in these states; the strip is for a notice over a rendered result (missing package), so the choice is coherent:
   nothing to show above, so the centre is where the eye is. Light and dark render without overlap.

Personas. The writer wanting the PDF: in N and F she gets it; in R and P she is told at once why there is no preview, that
her text is safe, and (P) what to press. The cautious person asking « am I protected? »: yes in N and F in the first words
of the line, no ambiguity in R (the engine does not run), P tells her honestly that it could not be checked.

### Closed from the first pass
UX-SB2a1-1, -2, -3 closed by a real walk (N, P, R). UX-SB2a1-4 and -5 closed as wording defects (sentences now state what
happens to the text and what to do; F line leads with the guarantee). Focus returning to « Détails » closed.

### Blocking defects
None of severity 3 or 4.

### New residuals (severity 2 and below)
- **UX-SB2a1-6 (2): « Réessayer » gives no visible feedback, and a click during the backoff is silently absorbed.**
  Capture `P-light-3-clicked.png` (300 ms after the click: the same sentence, the same button). Repro: `-p CPUQuota=1%`,
  wait for the retry state, lift the quota, click « Réessayer »: the sentence is unchanged for about 40 s (clicks at 0 s and
  10 s; the pages came at 40.6 s), because the probe backoff (1 s doubling to 30 s) answers « inconclusive » again without a
  launch, and nothing retries by itself afterwards. Expected: the button shows it is working (« Nouvelle tentative… », disabled)
  and the preview retries by itself when the backoff ends. `Preview.retry()` already calls `#working()`, but with no result
  the empty state shows nothing for it (heuristic 1; more than 1 s without an indicator). Not blocking: a later click works
  and no data is at risk.
- **UX-SB2a1-7 (2): in the refusal, « Exporter en PDF… » ends in a dead end.** Capture `R-light-6-export.png`: toast « Le
  moteur de mise en page n'est pas disponible : l'export est impossible. » No reason, no next step; the useful advice (open
  the document on another computer) is only in About. The writer who wants the PDF is the persona most hurt. Expected: reuse the
  preview sentence or say « ... ; voir Détails ». Sentence untouched by this block (outside U-1), but U-1 created the reachable case.
- **UX-SB2a1-8 (2): in the refusal the status bar keeps saying « Aucun problème » while a document with an error is shown.**
  Capture `R-light-5-typo.png` (`#unknownfn(1)` typed, red in the editor, « Aucun problème »). The checker also refuses (log
  `language resources unavailable ... Landlock unavailable`), so the green mark is a false reassurance, and « Le texte et la
  collaboration fonctionnent normalement » is slightly generous (no live checking). I did not compare with the same text on tier N
  in this pass. Expected: no verdict mark, or « Vérification indisponible ».
- **UX-SB2a1-9 (1): About card title « Mise en page hors ligne » above sentences about a disabled or unverified preview** (R, P);
  correct for N and F, a little off elsewhere. Cosmetic.
- **UX-SB2a1-10 (1): wording.** « isoler le moteur de mise en page », « l'isolation complète » and « séparé des autres programmes »
  stay on the technical side for a non-technical colleague, and « moteur de mise en page » is not in `docs/glossary.md`. The reduced-tier
  line is still five lines. Consider adding the term to the glossary, or « protéger » in place of « isoler ».
- **UX-SB2a1-11 (1): the muted sentence is at the AA limit in light** (about 4.5:1, estimated from the blended colours); for the
  low-vision persona at 200 % or forced colours this was not walked (not in scope of U-1).
- Observed, not a product defect: typing while the editor was not yet writable (page 100 times slower under 1 % CPU) was lost once;
  with a writable editor the text persisted through the whole retry. Not investigated further.

Not walked: protected-project refusal (`preview.fail.protected`; needs F plus a protected project), multi-user views (U-1 involves
no collaboration), 200 % zoom, forced-colors, WebKit.

Verdict: ship

