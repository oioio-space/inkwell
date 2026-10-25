import { EditorState } from "@codemirror/state";
import { describe, expect, it } from "vitest";
import { remoteCursorField, setRemoteCursors } from "./remoteCursors";

describe("remote cursors", () => {
  it("maps a remote cursor through a concurrent local edit", () => {
    let state = EditorState.create({ doc: "bonjour le monde", extensions: [remoteCursorField] });
    state = state.update({
      effects: setRemoteCursors.of([
        { peerId: "b", name: "Paul", color: "var(--person-2)", anchor: 8, head: 10 },
      ]),
    }).state;
    state = state.update({ changes: { from: 0, insert: "Oh, " }, userEvent: "input.type" }).state;

    const [cursor] = state.field(remoteCursorField);
    expect(cursor).toMatchObject({ peerId: "b", anchor: 12, head: 14 });
  });

  it("clamps cursors announced beyond the end of the document", () => {
    let state = EditorState.create({ doc: "abc", extensions: [remoteCursorField] });
    state = state.update({
      effects: setRemoteCursors.of([
        { peerId: "b", name: "Paul", color: "red", anchor: 99, head: 99 },
      ]),
    }).state;
    expect(state.field(remoteCursorField)[0]).toMatchObject({ anchor: 3, head: 3 });
  });
});
