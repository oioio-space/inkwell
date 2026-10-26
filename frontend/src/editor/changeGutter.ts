import { type EditorState, type Extension, StateEffect, StateField } from "@codemirror/state";
import {
  Decoration,
  type DecorationSet,
  EditorView,
  ViewPlugin,
  type ViewUpdate,
} from "@codemirror/view";
import { lineStarts } from "./external";

export type ChangeKind = "added" | "modified" | "deleted";
export type ColorBy = "type" | "person";

/** A change since the last visit, anchored by Automerge cursors so it follows the text (HIS-05). */
export interface ChangeMark {
  kind: ChangeKind;
  /** Automerge cursors of the first and of the last changed char (the same point for a removal). */
  from: string;
  to: string;
  /** Colour slot of the person (1 to 8) and the words of the tooltip, « Marie, 14:10 ». */
  person: number;
  label: string;
}

export interface MarkSet {
  marks: readonly ChangeMark[];
  colorBy: ColorBy;
  /** Turns an Automerge cursor into an offset of the text now, or undefined when it is not known. */
  resolve: (cursor: string) => number | undefined;
}

const setMarks = StateEffect.define<MarkSet | null>();

/** A mark with where it stands in the text now (the minimap reads these). */
export interface Placed {
  kind: ChangeKind;
  from: number;
  to: number;
  person: number;
  colorBy: ColorBy;
  label: string;
}

interface State {
  placed: readonly Placed[];
  deco: DecorationSet;
}

const empty: State = { placed: [], deco: Decoration.none };

function build(state: EditorState, input: MarkSet): State {
  const placed: Placed[] = [];
  const ranges: ReturnType<Decoration["range"]>[] = [];
  for (const m of input.marks) {
    const a = input.resolve(m.from);
    const b = input.resolve(m.to);
    if (a === undefined || b === undefined) continue;
    const from = Math.min(Math.max(a, 0), state.doc.length);
    // `to` is the cursor of the last changed char; a removal has no text.
    const to = m.kind === "deleted" ? from : Math.min(Math.max(b + 1, from), state.doc.length);
    placed.push({
      kind: m.kind,
      from,
      to,
      person: m.person,
      colorBy: input.colorBy,
      label: m.label,
    });
    // The strip is a bar drawn in the margin of the line itself (a decoration of the line, not a
    // gutter of its own: B1's marks of outside edits already ride on the gutters).
    const mark = Decoration.line({
      class: `cm-changed cm-changed-${m.kind}`,
      attributes: {
        title: m.label,
        ...(input.colorBy === "person"
          ? { style: `--change-color: var(--person-${m.person})` }
          : {}),
      },
    });
    // One bar per line the change touches (B1's helper); a removal sits on the line where it was.
    const starts =
      m.kind === "deleted" ? [state.doc.lineAt(from).from] : lineStarts(state, [{ from, to }]);
    for (const at of starts) ranges.push(mark.range(at));
  }
  return { placed, deco: Decoration.set(ranges, true) };
}

const field = StateField.define<State>({
  create: () => empty,
  update(value, tr) {
    for (const e of tr.effects) {
      if (e.is(setMarks)) return e.value ? build(tr.state, e.value) : empty;
    }
    if (!tr.docChanged || value.placed.length === 0) return value;
    return {
      deco: value.deco.map(tr.changes),
      placed: value.placed.map((p) => ({
        ...p,
        from: tr.changes.mapPos(p.from, 1),
        to: tr.changes.mapPos(p.to, -1),
      })),
    };
  },
  provide: (f) => EditorView.decorations.from(f, (s) => s.deco),
});

const theme = EditorView.baseTheme({
  // The bar sits in the margin of the line, between the line numbers and the text: it takes no
  // room of its own, so the line-number column keeps the 56 px of the design.
  ".cm-line.cm-changed": { position: "relative", "--change-color": "var(--accent-bg)" },
  ".cm-line.cm-changed::before": {
    content: '""',
    position: "absolute",
    left: "4px",
    top: "0",
    bottom: "0",
    width: "4px",
    borderRadius: "2px",
    background: "var(--change-color)",
  },
  ".cm-line.cm-changed-added": { "--change-color": "var(--success)" },
  ".cm-line.cm-changed-modified": { "--change-color": "var(--accent-bg)" },
  ".cm-line.cm-changed-deleted": { "--change-color": "var(--destructive)" },
  ".cm-line.cm-changed-deleted::before": {
    bottom: "auto",
    width: "0",
    height: "0",
    left: "5px",
    top: "7px",
    borderRadius: "0",
    background: "transparent",
    borderTop: "5px solid transparent",
    borderBottom: "5px solid transparent",
    borderLeft: "7px solid var(--change-color)",
  },
  ".cm-minimap": {
    position: "absolute",
    top: "0",
    bottom: "0",
    right: "0",
    width: "8px",
    pointerEvents: "none",
    zIndex: "5",
  },
  ".cm-minimap-tick": {
    position: "absolute",
    left: "1px",
    right: "1px",
    minHeight: "3px",
    borderRadius: "2px",
    background: "var(--change-color, var(--accent-bg))",
    pointerEvents: "auto",
    cursor: "pointer",
    border: "0",
    padding: "0",
  },
  ".cm-minimap-tick.cm-change-added": { "--change-color": "var(--success)" },
  ".cm-minimap-tick.cm-change-modified": { "--change-color": "var(--accent-bg)" },
  ".cm-minimap-tick.cm-change-deleted": { "--change-color": "var(--destructive)" },
});

/** Draws a tick per change in a strip along the scrollbar: where in the whole document they are. */
const minimap = ViewPlugin.fromClass(
  class {
    strip: HTMLElement;
    constructor(readonly view: EditorView) {
      this.strip = document.createElement("div");
      this.strip.className = "cm-minimap";
      this.strip.setAttribute("aria-hidden", "true");
      view.dom.append(this.strip);
      this.draw();
    }
    update(u: ViewUpdate) {
      if (
        u.docChanged ||
        u.heightChanged ||
        u.geometryChanged ||
        u.transactions.some((tr) => tr.effects.some((e) => e.is(setMarks)))
      ) {
        this.draw();
      }
    }
    draw() {
      const { placed } = this.view.state.field(field);
      this.strip.replaceChildren();
      const total = this.view.contentHeight || 1;
      for (const p of placed) {
        const top = this.view.lineBlockAt(p.from).top;
        const bottom = p.to > p.from ? this.view.lineBlockAt(p.to).bottom : top + 4;
        const tick = document.createElement("button");
        tick.type = "button";
        tick.tabIndex = -1;
        tick.className = `cm-minimap-tick cm-change-${p.kind}`;
        tick.style.top = `${(top / total) * 100}%`;
        tick.style.height = `${Math.max(0, ((bottom - top) / total) * 100)}%`;
        if (p.colorBy === "person")
          tick.style.setProperty("--change-color", `var(--person-${p.person})`);
        tick.title = p.label;
        tick.addEventListener("click", () =>
          this.view.dispatch({ effects: EditorView.scrollIntoView(p.from, { y: "center" }) }),
        );
        this.strip.append(tick);
      }
      this.strip.hidden = placed.length === 0;
    }
    destroy() {
      this.strip.remove();
    }
  },
);

/** The change bars in the margin and the minimap on the scrollbar; give them marks with {@link showChanges}. */
export const changeGutter: Extension = [field, minimap, theme];

/** Shows these changes (replacing the earlier ones); null clears them. */
export function showChanges(view: EditorView, marks: MarkSet | null): void {
  view.dispatch({ effects: setMarks.of(marks) });
}
