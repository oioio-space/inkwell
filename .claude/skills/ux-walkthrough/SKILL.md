---
name: ux-walkthrough
description: Use to test inkwell's interface the way a person would — before merging any UI change, after each wave, and before a release. Drives the real app (two or three instances), plays personas through task scenarios, judges every step with a cognitive walkthrough and Nielsen's heuristics, and turns each defect into a failing e2e test plus a fix. Triggers on "test like a human", "is it natural", "UX review", "walkthrough", "usability".
---

# UX walkthrough — test like a person

Automated tests prove that the code does what we wrote. This skill checks that what we wrote is
what a person expects. A feature that passes every test can still be unusable: the button is
hard to find, it gives no feedback, the wording is wrong, or the flow dead-ends.

## Method (grounded in usability practice)

- **Cognitive walkthrough** (Wharton et al., NN/g): for every step of a task, ask the four
  questions:
  1. Will the person *want* to do this step? Do they know it is needed?
  2. Will they *notice* the control that does it?
  3. Will they *associate* that control with their goal (label, icon, position)?
  4. After acting, will they *see* that it worked, and what changed?
- **Nielsen's 10 heuristics** (NN/g):
  1. Visibility of system status.
  2. Match with the real world (no jargon — « personne », never « pair »).
  3. User control and freedom (undo, cancel, escape).
  4. Consistency and standards (GNOME HIG, Excel/Numbers for the sheet).
  5. Error prevention.
  6. Recognition rather than recall.
  7. Flexibility (keyboard shortcuts for experts).
  8. Aesthetic, minimalist design.
  9. Help users recognise, diagnose and recover from errors.
  10. Help and documentation.
- **First-use, not insider:** the tester must NOT read the source or the plan's UI description
  before a scenario. It discovers the interface from the screen, as a new colleague would. It
  reads the plan only afterwards, to check that nothing promised is missing.

## Personas

| Persona | Who | Lens |
|---------|-----|------|
| **Alice**, créatrice et admin | team lead, comfortable with computers, no Typst | sharing, roles, disconnecting, locks, history |
| **Paul**, écrivain | writes procedures, keyboard-heavy, impatient | formatting, search, sheet, shortcuts, speed |
| **Léa**, lectrice | occasional, cautious, low vision (200 % zoom, high contrast) | can she read and find without breaking anything; accessibility |
| **Karim**, à distance | on a bad connection (relay, offline periods) | sync status, offline edits, conflicts, reassurance |

## Scenarios

Each scenario plays out in the real app with 2–3 instances: `mise run e2e:build`, then the
instances that the e2e config starts, or `mise run app:duo` for the desktop app. Scenarios
follow the plan sections:
- **S1, first launch:** create a project, write a title and a list, save.
- **S2, invite:** invite Paul by code; Paul joins; both type; Paul uses Ctrl+Z.
- **S3, formatting:** headings, bold, a code block, Insérer › image / joindre au PDF / renvoi.
- **S4, search:** find all in the project, jump to a result, replace with regex, undo.
- **S5, sheet:** open a CSV, copy a range to and from LibreOffice, drag the fill handle, insert
  and delete rows, sort and filter, with two people editing at once.
- **S6, files:** create, import, rename, ask to delete, admin accepts, restore from history.
- **S7, people:**
  - change a role;
  - disconnect Paul, then let him back;
  - Paul's experience throughout;
  - Léa as a reader.
- **S8, history and locks:**
  - name a version, revert, create a variant and merge it;
  - lock a zone, Paul proposes a change, Alice accepts.
- **S9, outside edits:** edit `main.typ` in another editor; save mode manual vs automatic.
- **S10, Markdown and layout:** switch layout, preview, export.
- **S11, presenting:** present, presenter view, follow the presenter.
- **S12, offline:** cut the network mid-edit (netsim or kill the relay) and come back.

## For every step, record

- The screenshot (light, and dark for at least one pass), and the time to feedback.
- The four walkthrough answers. Any "no" is a defect.
- Every heuristic violated.
- Keyboard-only feasibility (Tab, arrows, Escape) and focus visibility.
- Wording: plain French, consistent verbs (the button « Publier » produces « Publié »), no
  jargon, no English left over.
- What the *other* person sees at that moment, in multi-user scenarios.

## Severity (NN/g scale)

| Severity | Meaning | Action |
|----------|---------|--------|
| 4, catastrophe | data loss, a blocked task, a security surprise | fix before anything else |
| 3, major | the task succeeds only with outside help, or the person is misled | fix before release |
| 2, minor | slows the person down or confuses them | fix in the wave |
| 1, cosmetic | | fix when touched |

## Output

1. Write `docs/ux/walkthrough-<date>-<scope>.md`. For each scenario: steps, verdicts, and
   defects with id `UX-<scenario>-<n>`, severity, screenshot path, repro and expected
   behaviour.
2. Turn each defect of severity ≥ 2 into a **failing e2e test** first (in `frontend/e2e/ux/`,
   named by the defect id), then fix it, then confirm the test passes.
3. Severity 4 and 3 block a merge or a release; list them at the top of the report.
4. Re-run the affected scenarios after the fixes. A defect is closed only when its scenario is
   re-walked clean.

## Tools

- Playwright, either through scripts or the Playwright MCP browser tools. Use `page.screenshot`
  and look at every screenshot yourself: a test that never looks at the screen is not a
  walkthrough.
- `@axe-core/playwright` for WCAG A/AA; zoom to 200 % and use `forced-colors` for Léa.
- netsim (`mise run e2e:netsim`) and relay kills for Karim.
- Timing: measure time-to-feedback per action. More than 100 ms without visible feedback is a
  defect; more than 1 s without a progress indicator is a defect.

## Sources

- NN/g, [10 usability heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/).
- NN/g, [cognitive walkthroughs](https://www.nngroup.com/articles/cognitive-walkthroughs/).
- NN/g, [severity ratings](https://www.nngroup.com/articles/how-to-rate-the-severity-of-usability-problems/).
- [GNOME HIG](https://developer.gnome.org/hig/).
