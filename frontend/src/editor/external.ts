import {
  type EditorState,
  type Extension,
  RangeSet,
  StateEffect,
  StateField,
} from "@codemirror/state";
import {
  Decoration,
  type DecorationSet,
  EditorView,
  GutterMarker,
  gutterLineClass,
} from "@codemirror/view";

import { contentOf, type Range as Span } from "../store/spans";

export type { Span };

const mark = StateEffect.define<readonly Span[] | null>();

const lineMark = Decoration.line({ class: "cm-external-line" });

class ExternalGutterMarker extends GutterMarker {
  override elementClass = "cm-external-gutter";
}
const gutterMark = new ExternalGutterMarker();

/** The start of each line the spans touch, in order, once. */
function lineStarts(state: EditorState, spans: readonly Span[]): number[] {
  const starts = new Set<number>();
  const { doc } = state;
  for (const span of spans) {
    // The empty line after a final newline is not one of the file: the last real line is meant.
    const last =
      doc.length > 0 && doc.sliceString(doc.length - 1) === "\n" ? doc.length - 1 : doc.length;
    const from = Math.min(span.from, doc.length);
    const to = Math.min(Math.max(span.to, from), doc.length);
    // Without the newlines at its ends: the lines they join to were not changed.
    const content = contentOf({ from, to }, doc.sliceString(from, to));
    const [start, end] = content
      ? [content.from, content.to - 1]
      : [Math.min(to, last), Math.min(to, last)];
    for (let pos = Math.min(start, last); ; ) {
      const line = doc.lineAt(pos);
      starts.add(line.from);
      if (line.to >= end) break;
      pos = line.to + 1;
    }
  }
  return [...starts].sort((a, b) => a - b);
}

const field = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(value, tr) {
    let next = value.map(tr.changes);
    for (const effect of tr.effects) {
      if (!effect.is(mark)) continue;
      next = effect.value
        ? Decoration.set(lineStarts(tr.state, effect.value).map((from) => lineMark.range(from)))
        : Decoration.none;
    }
    return next;
  },
  provide: (f) => [
    EditorView.decorations.from(f),
    gutterLineClass.from(f, (set) => {
      const marks = [];
      for (const it = set.iter(); it.value; it.next()) marks.push(gutterMark.range(it.from));
      return RangeSet.of(marks);
    }),
  ],
});

const theme = EditorView.baseTheme({
  ".cm-external-line": {
    backgroundColor: "color-mix(in srgb, var(--accent-bg) 11%, transparent)",
  },
  ".cm-gutterElement.cm-external-gutter": {
    backgroundColor: "color-mix(in srgb, var(--accent-bg) 22%, transparent)",
    boxShadow: "inset 3px 0 var(--accent-bg)",
    color: "var(--fg)",
  },
});

/** The lines taken from the disk are tinted, in the text and in the gutter. */
export const externalMarks: Extension = [field, theme];

/** Marks the lines the spans touch, replacing earlier marks; null clears them. */
export function markExternal(view: EditorView, spans: readonly Span[] | null): void {
  view.dispatch({ effects: mark.of(spans) });
}

/** Scrolls to the first marked line and puts the caret there. */
export function revealExternal(view: EditorView): void {
  const first = view.state.field(field).iter();
  if (first.value === null) return;
  view.dispatch({
    selection: { anchor: first.from },
    effects: EditorView.scrollIntoView(first.from, { y: "center" }),
  });
  view.focus();
}
