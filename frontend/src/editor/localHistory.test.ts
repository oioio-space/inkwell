import { history, undo } from "@codemirror/commands";
import { EditorState } from "@codemirror/state";
import { describe, expect, it } from "vitest";
import { localHistoryOnly } from "./localHistory";

describe("localHistoryOnly", () => {
  it("undo reverts only the local edits, never a remote one", () => {
    let state = EditorState.create({ doc: "", extensions: [history(), localHistoryOnly] });
    // Local typing carries a userEvent; remote patches from automerge-codemirror do not.
    state = state.update({ changes: { from: 0, insert: "moi " }, userEvent: "input.type" }).state;
    state = state.update({ changes: { from: 4, insert: "toi" } }).state;
    expect(state.doc.toString()).toBe("moi toi");

    let undone = state;
    undo({ state, dispatch: (tr) => (undone = tr.state) });
    expect(undone.doc.toString()).toBe("toi");
  });

  it("keeps remote edits out of the undo stack entirely", () => {
    let state = EditorState.create({ doc: "", extensions: [history(), localHistoryOnly] });
    state = state.update({ changes: { from: 0, insert: "toi" } }).state;
    const ran = undo({ state, dispatch: () => {} });
    expect(ran).toBe(false);
  });
});
