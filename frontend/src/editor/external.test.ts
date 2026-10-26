import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, describe, expect, it } from "vitest";
import { externalMarks, markExternal, revealExternal } from "./external";

const views: EditorView[] = [];
function mount(doc: string): EditorView {
  const view = new EditorView({
    parent: document.body,
    state: EditorState.create({ doc, extensions: [externalMarks] }),
  });
  views.push(view);
  return view;
}
afterEach(() => {
  for (const v of views.splice(0)) v.destroy();
});

const marked = (view: EditorView) =>
  [...view.dom.querySelectorAll(".cm-line")]
    .map((l, i) => (l.classList.contains("cm-external-line") ? i + 1 : 0))
    .filter(Boolean);

describe("the lines taken from the disk", () => {
  it("EXT-01 are tinted, whole lines, once", () => {
    const view = mount("a\nbb\nccc\ndddd\n");
    markExternal(view, [
      { from: 2, to: 6 }, // lines 2 and 3
      { from: 3, to: 4 }, // line 2 again
    ]);
    expect(marked(view)).toEqual([2, 3]);
  });

  it("follow the text when it moves, and a span past the end is clamped", () => {
    const view = mount("a\nb\nc\n");
    markExternal(view, [
      { from: 2, to: 3 },
      { from: 100, to: 120 },
    ]);
    view.dispatch({ changes: { from: 0, insert: "new\n" } });
    expect(marked(view)).toEqual([3, 4]);
  });

  it("UX-S9-1 a newline that only joins a new line to an old one does not mark the old line", () => {
    const view = mount("= Titre\nUne ligne\nvenue d'ailleurs\n");
    // The diff added "\nUne ligne\nvenue d'ailleurs\n" after "= Titre": lines 2 and 3 are new.
    markExternal(view, [{ from: 7, to: view.state.doc.length }]);
    expect(marked(view)).toEqual([2, 3]);
  });

  it("UX-S9-1 the place where text was removed is marked, on the line that remains", () => {
    const view = mount("a\nb\nc\n");
    markExternal(view, [{ from: 2, to: 2 }]);
    expect(marked(view)).toEqual([2]);
  });

  it("are cleared", () => {
    const view = mount("a\nb\n");
    markExternal(view, [{ from: 0, to: 1 }]);
    markExternal(view, null);
    expect(marked(view)).toEqual([]);
  });

  it("EXT-03 'View' goes to the first marked line", () => {
    const view = mount("a\nb\nc\nd\n");
    markExternal(view, [{ from: 4, to: 5 }]);
    revealExternal(view);
    expect(view.state.selection.main.head).toBe(4);
  });
});
