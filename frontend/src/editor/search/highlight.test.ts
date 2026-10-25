import { EditorState, type Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, describe, expect, it } from "vitest";
import { searchHighlight, setSearchMatcher } from "./highlight";
import { searchExtension } from "./index";
import { searchKeymap } from "./keymap";
import { inlineRunner } from "./runner";
import { compile } from "./testkit";

let view: EditorView | undefined;
afterEach(() => {
  view?.destroy();
  view = undefined;
});

function editor(doc: string, ...extensions: Extension[]) {
  const parent = document.createElement("div");
  document.body.append(parent);
  view = new EditorView({
    parent,
    state: EditorState.create({ doc, extensions }),
  });
  return view;
}

const marked = (v: EditorView) =>
  [...v.dom.querySelectorAll(".cm-search-hit")].map((e) => e.textContent);

describe("editor integration", () => {
  it("highlights every occurrence of the query and follows query changes", () => {
    const v = editor("foo bar foo", searchHighlight(compile("foo")));
    expect(marked(v)).toEqual(["foo", "foo"]);
    v.dispatch({ effects: setSearchMatcher.of(compile("bar")) });
    expect(marked(v)).toEqual(["bar"]);
    v.dispatch({ effects: setSearchMatcher.of(null) });
    expect(marked(v)).toEqual([]);
    // Typing keeps the marks in place.
    v.dispatch({ effects: setSearchMatcher.of(compile("foo")) });
    v.dispatch({ changes: { from: 0, insert: "x" } });
    expect(marked(v).length).toBeGreaterThan(0);
  });

  it("highlights a regex through the runner", async () => {
    const v = editor(
      "a1 b22 c333",
      searchHighlight(compile("\\d+", { mode: "regex" }), inlineRunner),
    );
    await new Promise((r) => setTimeout(r, 0));
    expect(marked(v)).toEqual(["1", "22", "333"]);
  });

  it("maps Ctrl+F, Ctrl+H, Ctrl+Shift+F, F3, F4 to the host's actions", () => {
    const calls: string[] = [];
    const actions = {
      find: () => void calls.push("find"),
      replace: () => void calls.push("replace"),
      findInProject: () => void calls.push("project"),
      next: (b: boolean) => void calls.push(b ? "prev" : "next"),
      nextResult: (b: boolean) => void calls.push(b ? "rprev" : "rnext"),
      close: () => true,
    };
    const v = editor("x", searchExtension(actions));
    const press = (init: KeyboardEventInit) => {
      const e = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
      v.contentDOM.dispatchEvent(e);
    };
    press({ key: "f", ctrlKey: true });
    press({ key: "h", ctrlKey: true });
    press({ key: "F", keyCode: 70, ctrlKey: true, shiftKey: true });
    press({ key: "F3" });
    press({ key: "F3", shiftKey: true });
    press({ key: "F4" });
    press({ key: "F4", shiftKey: true });
    expect(calls).toEqual(["find", "replace", "project", "next", "prev", "rnext", "rprev"]);
    expect(searchKeymap(actions).map((b) => b.key)).toEqual([
      "Mod-f",
      "Mod-h",
      "Mod-Shift-f",
      "F3",
      "F4",
      "Escape",
    ]);
    // A host that declines lets the key fall through.
    const decline = searchKeymap({ ...actions, find: () => false });
    expect(decline[0]?.run?.(v)).toBe(false);
  });
});
