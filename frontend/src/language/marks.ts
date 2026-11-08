import { StateEffect, StateField } from "@codemirror/state";
import { Decoration, type DecorationSet, EditorView } from "@codemirror/view";
export const problemMarks =
  StateEffect.define<readonly { from: number; to: number; severity: string }[]>();
export const diagnosticMarks = [
  StateField.define<DecorationSet>({
    create: () => Decoration.none,
    update(value, tr) {
      value = value.map(tr.changes);
      for (const effect of tr.effects)
        if (effect.is(problemMarks))
          value = Decoration.set(
            effect.value
              .filter((d) => d.to > d.from && d.from >= 0 && d.to <= tr.newDoc.length)
              .map((d) =>
                Decoration.mark({ class: `writing-problem ${d.severity}` }).range(d.from, d.to),
              ),
            true,
          );
      return value;
    },
    provide: (field) => EditorView.decorations.from(field),
  }),
  EditorView.theme({
    ".writing-problem.error": {
      textDecoration: "underline wavy var(--error)",
      textUnderlineOffset: "3px",
    },
    ".writing-problem.warning": {
      textDecoration: "underline wavy var(--warning)",
      textUnderlineOffset: "3px",
    },
  }),
];
