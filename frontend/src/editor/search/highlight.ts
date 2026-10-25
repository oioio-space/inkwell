import { type Extension, StateEffect, StateField } from "@codemirror/state";
import {
  Decoration,
  type DecorationSet,
  EditorView,
  ViewPlugin,
  type ViewUpdate,
} from "@codemirror/view";
import type { Matcher } from "./query";
import { defaultRunner, type SearchRunner } from "./runner";
import { scanSync } from "./scan";

/** Sets (or, with null, clears) the query highlighted in the editor. */
export const setSearchMatcher = StateEffect.define<Matcher | null>();

const matcherField = StateField.define<Matcher | null>({
  create: () => null,
  update(value, tr) {
    for (const e of tr.effects) if (e.is(setSearchMatcher)) return e.value;
    return value;
  },
});

const hitMark = Decoration.mark({ class: "cm-search-hit" });

const theme = EditorView.baseTheme({
  ".cm-search-hit": {
    backgroundColor: "color-mix(in srgb, var(--warning, #ff7800) 35%, transparent)",
    borderRadius: "2px",
  },
});

/** Most characters scanned around the viewport. */
const MAX_SPAN = 200_000;

function visibleSpans(view: EditorView): Array<{ from: number; to: number }> {
  const spans: Array<{ from: number; to: number }> = [];
  for (const r of view.visibleRanges) {
    const from = view.state.doc.lineAt(r.from).from;
    const to = Math.min(view.state.doc.lineAt(r.to).to, from + MAX_SPAN);
    const last = spans[spans.length - 1];
    if (last && from <= last.to) last.to = Math.max(last.to, to);
    else spans.push({ from, to });
  }
  return spans;
}

function highlighter(runner: SearchRunner | undefined) {
  return ViewPlugin.fromClass(
    class {
      decorations: DecorationSet = Decoration.none;
      private token = 0;

      constructor(readonly view: EditorView) {
        this.refresh();
      }

      update(u: ViewUpdate) {
        const changed = u.transactions.some((t) => t.effects.some((e) => e.is(setSearchMatcher)));
        if (u.docChanged || u.viewportChanged || changed) {
          if (u.docChanged) this.decorations = this.decorations.map(u.changes);
          this.refresh();
        }
      }

      refresh() {
        const matcher = this.view.state.field(matcherField);
        const token = ++this.token;
        if (!matcher || matcher.empty) {
          this.decorations = Decoration.none;
          return;
        }
        const spans = visibleSpans(this.view);
        if (!matcher.guarded) {
          this.decorations = this.build(
            spans.map((s) => ({
              s,
              hits: scanSync(this.view.state.sliceDoc(s.from, s.to), matcher),
            })),
          );
          return;
        }
        // A regex may run away: ask the worker, and drop the answer if the view moved on.
        const jobs = spans.map((s) => ({ s, text: this.view.state.sliceDoc(s.from, s.to) }));
        Promise.all(
          jobs.map(async ({ s, text }) => ({
            s,
            hits: await (runner ?? defaultRunner()).run({ text, matcher, range: {}, limit: 5000 }),
          })),
        )
          .then((found) => {
            if (token !== this.token) return;
            this.decorations = this.build(found);
            this.view.dispatch({}); // repaint with the new decorations
          })
          .catch(() => {
            if (token !== this.token) return;
            this.decorations = Decoration.none;
            this.view.dispatch({});
          });
      }

      build(found: Array<{ s: { from: number }; hits: Array<{ from: number; to: number }> }>) {
        const ranges = [];
        for (const { s, hits } of found) {
          for (const h of hits)
            if (h.to > h.from) ranges.push(hitMark.range(s.from + h.from, s.from + h.to));
        }
        return Decoration.set(ranges, true);
      }
    },
    { decorations: (p) => p.decorations },
  );
}

/**
 * Highlights every occurrence of a query in the editor (visible part only, so it costs the same
 * on a 5 MB file). Change the query with `setSearchMatcher`, or with `view.dispatch({ effects:
 * setSearchMatcher.of(matcher) })`.
 */
export function searchHighlight(matcher: Matcher | null = null, runner?: SearchRunner): Extension {
  return [matcherField.init(() => matcher), highlighter(runner), theme];
}
