import { type Extension, RangeSetBuilder, StateEffect, StateField } from "@codemirror/state";
import { Decoration, type DecorationSet, EditorView } from "@codemirror/view";

/** A stretch of the text and who wrote it, for « Qui a écrit quoi ». */
export interface Written {
  from: number;
  to: number;
  /** Colour slot of the person (1 to 8). */
  person: number;
  /** « Paul · hier 16:02 · lors de la séance de 15:40 ». */
  label: string;
  /** Id of the history entry (the session) to open on a click. */
  entry: string;
}

/** Fired on the editor (and bubbles) when a coloured passage is clicked: `detail` is the entry id. */
export const PICK = "inkwell:written-pick";

const setWritten = StateEffect.define<readonly Written[] | null>();

const field = StateField.define<{ deco: DecorationSet; spans: readonly Written[] }>({
  create: () => ({ deco: Decoration.none, spans: [] }),
  update(value, tr) {
    for (const e of tr.effects) {
      if (!e.is(setWritten)) continue;
      const spans = e.value ?? [];
      const builder = new RangeSetBuilder<Decoration>();
      for (const s of spans) {
        if (s.to <= s.from || s.to > tr.state.doc.length) continue;
        builder.add(
          s.from,
          s.to,
          Decoration.mark({
            class: `cm-who cm-who-${s.person}`,
            attributes: { title: s.label, "data-entry": s.entry },
          }),
        );
      }
      return { deco: builder.finish(), spans };
    }
    return tr.docChanged ? { ...value, deco: value.deco.map(tr.changes) } : value;
  },
  provide: (f) => EditorView.decorations.from(f, (v) => v.deco),
});

const theme = EditorView.baseTheme(
  Object.fromEntries(
    Array.from({ length: 8 }, (_, i) => [
      `.cm-who-${i + 1}`,
      {
        backgroundColor: `color-mix(in srgb, var(--person-${i + 1}) 26%, transparent)`,
        boxShadow: `inset 0 -2px color-mix(in srgb, var(--person-${i + 1}) 70%, transparent)`,
      },
    ]),
  ),
);

/** Colours each passage of the text by its author. Off until {@link showWritten} is given spans. */
export function blameMarks(): Extension {
  return [
    field,
    theme,
    EditorView.domEventHandlers({
      click(event, view) {
        const el = (event.target as HTMLElement | null)?.closest<HTMLElement>(".cm-who");
        const entry = el?.dataset.entry;
        if (entry) view.dom.dispatchEvent(new CustomEvent(PICK, { detail: entry, bubbles: true }));
        return false;
      },
    }),
  ];
}

/** Colours the text by author; null removes the colours. */
export function showWritten(view: EditorView, spans: readonly Written[] | null): void {
  view.dispatch({ effects: setWritten.of(spans) });
}
