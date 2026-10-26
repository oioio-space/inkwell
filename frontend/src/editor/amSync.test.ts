import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import { EditorView } from "@codemirror/view";
import { describe, expect, it } from "vitest";
import { CHUNK_CHARS, MAX_GROWN_CHARS } from "../store/limits";
import { amSync } from "./amSync";

type Doc = { text: string };
const repo = new Repo();

function mount(text: string, onRefused: () => void = () => {}) {
  const handle = repo.create<Doc>({ text });
  const view = new EditorView({
    parent: document.body,
    doc: text,
    extensions: [amSync({ handle, path: ["text"], onRefused })],
  });
  return { handle, view };
}
const changes = (h: ReturnType<typeof mount>["handle"]) => A.getAllChanges(h.doc()).length;

describe("amSync: the editor and the document, with the size gate and chunks (ADR 0028)", () => {
  it("typing is one small change, and a remote change shows in the editor", () => {
    const { handle, view } = mount("abc");
    const before = changes(handle);
    view.dispatch({ changes: { from: 3, insert: "d" }, userEvent: "input.type" });
    expect(handle.doc().text).toBe("abcd");
    expect(changes(handle) - before).toBe(1);

    handle.change((d) => A.splice(d, ["text"], 0, 0, ">"));
    expect(view.state.doc.toString()).toBe(">abcd");
    view.destroy();
  });

  it("PERF-04 a paste of 200 Ki is cut in changes of at most 64 Ki and the text is intact", () => {
    const { handle, view } = mount("<>");
    const before = changes(handle);
    const big = "ab".repeat(100 * 1024);
    view.dispatch({
      changes: { from: 1, insert: big },
      userEvent: "input.paste",
    });
    expect(handle.doc().text).toBe(`<${big}>`);
    expect(view.state.doc.toString()).toBe(`<${big}>`);
    expect(changes(handle) - before).toBe(Math.ceil(big.length / CHUNK_CHARS));
    view.destroy();
  });

  it("PERF-03 a paste that would take the text past 4 Mi is refused before the document is touched", () => {
    let refused = 0;
    const { handle, view } = mount("x".repeat(100), () => void refused++);
    const before = changes(handle);
    const heads = A.getHeads(handle.doc());
    view.dispatch({
      changes: { from: 0, insert: "y".repeat(MAX_GROWN_CHARS) },
      userEvent: "input.paste",
    });
    expect(refused).toBe(1);
    expect(view.state.doc.length).toBe(100);
    expect(handle.doc().text).toBe("x".repeat(100));
    expect(changes(handle)).toBe(before);
    expect(A.getHeads(handle.doc())).toEqual(heads);
    view.destroy();
  });

  it("deleting from a text past the limit is still allowed, so a big file can be cut down", () => {
    const { handle, view } = mount("z".repeat(MAX_GROWN_CHARS + 10));
    view.dispatch({ changes: { from: 0, to: 20 }, userEvent: "delete.selection" });
    expect(view.state.doc.length).toBe(MAX_GROWN_CHARS - 10);
    expect(handle.doc().text.length).toBe(MAX_GROWN_CHARS - 10);
    view.destroy();
  }, 60_000);

  it("a change from a peer is applied even when it takes the text past the limit", () => {
    const { handle, view } = mount("x".repeat(MAX_GROWN_CHARS - 5));
    handle.change((d) => A.splice(d, ["text"], 0, 0, "peer growth"));
    expect(view.state.doc.length).toBe(MAX_GROWN_CHARS + 6);
    view.destroy();
  }, 60_000);
});
