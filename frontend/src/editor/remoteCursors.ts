import { type Extension, RangeSetBuilder, StateEffect, StateField } from "@codemirror/state";
import { Decoration, type DecorationSet, EditorView, WidgetType } from "@codemirror/view";
import { PERSON_COLORS, personIndex } from "../ui/people";

/** Where another person's cursor is, already resolved to document offsets. */
export interface RemoteCursor {
  peerId: string;
  name: string;
  anchor: number;
  head: number;
}

/** Replaces the set of remote cursors (from presence updates). */
export const setRemoteCursors = StateEffect.define<RemoteCursor[]>();

/** Remote cursors, mapped through every edit until the next presence update. */
export const remoteCursorField = StateField.define<RemoteCursor[]>({
  create: () => [],
  update(cursors, tr) {
    for (const e of tr.effects) {
      if (e.is(setRemoteCursors)) {
        const max = tr.state.doc.length;
        return e.value.map((c) => ({
          ...c,
          anchor: Math.min(c.anchor, max),
          head: Math.min(c.head, max),
        }));
      }
    }
    if (!tr.docChanged) return cursors;
    return cursors.map((c) => ({
      ...c,
      anchor: tr.changes.mapPos(c.anchor, 1),
      head: tr.changes.mapPos(c.head, 1),
    }));
  },
});

class CaretWidget extends WidgetType {
  constructor(
    readonly name: string,
    readonly slot: number,
  ) {
    super();
  }
  override eq(other: CaretWidget): boolean {
    return other.name === this.name && other.slot === this.slot;
  }
  toDOM(): HTMLElement {
    const caret = document.createElement("span");
    // Classes, not style attributes: the CSP forbids inline style attributes.
    caret.className = `cm-remote-caret cm-who-${this.slot}`;
    // Decoration only: screen readers must read the document, not people's names in it.
    caret.setAttribute("aria-hidden", "true");
    const label = document.createElement("span");
    label.className = "cm-remote-name";
    label.textContent = this.name; // the name is always shown: colour is never the only cue
    caret.append(label);
    return caret;
  }
}

const decorations = EditorView.decorations.compute([remoteCursorField], (state) => {
  const marks: { from: number; to: number; deco: Decoration }[] = [];
  for (const c of state.field(remoteCursorField)) {
    const slot = personIndex(c.peerId);
    const from = Math.min(c.anchor, c.head);
    const to = Math.max(c.anchor, c.head);
    if (from < to) {
      marks.push({
        from,
        to,
        deco: Decoration.mark({ class: `cm-remote-selection cm-who-${slot}` }),
      });
    }
    marks.push({
      from: c.head,
      to: c.head,
      deco: Decoration.widget({ widget: new CaretWidget(c.name, slot), side: 1 }),
    });
  }
  marks.sort((a, b) => a.from - b.from || a.to - b.to);
  const builder = new RangeSetBuilder<Decoration>();
  for (const m of marks) builder.add(m.from, m.to, m.deco);
  return builder.finish() as DecorationSet;
});

const slots = Object.fromEntries(
  PERSON_COLORS.map((color, i) => [`.cm-who-${i + 1}`, { "--who": color }]),
);

const theme = EditorView.baseTheme({
  ...slots,
  ".cm-remote-selection": { backgroundColor: "color-mix(in srgb, var(--who) 25%, transparent)" },
  ".cm-remote-caret": {
    position: "relative",
    borderLeft: "2px solid var(--who)",
    marginLeft: "-1px",
    marginRight: "-1px",
  },
  ".cm-remote-name": {
    position: "absolute",
    bottom: "100%",
    left: "-2px",
    padding: "0 4px",
    borderRadius: "4px 4px 4px 0",
    font: "600 11px/1.6 var(--font-ui)",
    whiteSpace: "nowrap",
    color: "#000",
    backgroundColor: "var(--who)",
    pointerEvents: "none",
  },
});

/** Shows other people's cursors and selections, each labelled with their name. */
export function remoteCursors(): Extension {
  return [remoteCursorField, decorations, theme];
}
