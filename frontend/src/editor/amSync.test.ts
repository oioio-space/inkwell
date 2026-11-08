import { next as A } from "@automerge/automerge";
import { type DocHandle, encodeHeads, Presence, Repo } from "@automerge/automerge-repo";
import { history, undo } from "@codemirror/commands";
import { EditorSelection, EditorState, type Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { describe, expect, it, vi } from "vitest";
import { CHUNK_CHARS, MAX_GROWN_CHARS } from "../store/limits";
import { startProjection } from "../store/projection";
import { amSync, flushTyped, reconcileAnnotationType, whenWritten } from "./amSync";
import { localHistoryOnly } from "./localHistory";
import { presenceSync } from "./presence";

type Doc = { text: string };
const repo = new Repo();

function attach(handle: DocHandle<Doc>, onRefused: () => void = () => {}, extra: Extension = []) {
  return new EditorView({
    parent: document.body,
    doc: handle.doc().text,
    extensions: [extra, amSync({ handle, path: ["text"], onRefused })],
  });
}
function mount(text: string, onRefused: () => void = () => {}, extra: Extension = []) {
  const handle = repo.create<Doc>({ text });
  return { handle, view: attach(handle, onRefused, extra) };
}
/** A second person (their own actor) with the same document, in their own editor. */
function peerOf(other: DocHandle<Doc>) {
  const handle = repo.create<Doc>();
  handle.update(() => A.merge(A.init<Doc>(), other.doc()));
  return { handle, view: attach(handle) };
}
const changes = (h: DocHandle<Doc>) => A.getAllChanges(h.doc()).length;

describe("amSync: the editor and the document, with the size gate and chunks (ADR 0028)", () => {
  it("typing is one small change, and a remote change shows in the editor", () => {
    const { handle, view } = mount("abc");
    const before = changes(handle);
    view.dispatch({
      changes: { from: 3, insert: "d" },
      userEvent: "input.type",
    });
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
    view.dispatch({
      changes: { from: 0, to: 20 },
      userEvent: "delete.selection",
    });
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

  describe("a typed character is written after the next paint (PERF-TYPE-native)", () => {
    // What CodeMirror's DOM input path does for a typed character: ask the input handlers,
    // then dispatch the default insert.
    function type(view: EditorView, text: string, userEvent = "input.type") {
      const at = view.state.selection.main.head;
      const insert = () =>
        view.state.update({
          changes: { from: at, insert: text },
          selection: { anchor: at + text.length },
          userEvent,
        });
      if (!view.state.facet(EditorView.inputHandler).some((h) => h(view, at, at, text, insert)))
        view.dispatch(insert());
    }
    /** The same key typed at every cursor, as CodeMirror does for several selections. */
    function typeAtEach(view: EditorView, text: string) {
      const at = view.state.selection.main.head;
      const insert = () =>
        view.state.update(
          view.state.changeByRange((r) => ({
            changes: { from: r.head, insert: text },
            range: EditorSelection.cursor(r.head + text.length),
          })),
          { userEvent: "input.type" },
        );
      if (!view.state.facet(EditorView.inputHandler).some((h) => h(view, at, at, text, insert)))
        view.dispatch(insert());
    }
    const afterPaint = () =>
      new Promise<void>((done) =>
        requestAnimationFrame(() => setTimeout(() => setTimeout(done, 0), 0)),
      );
    const textOf = (p: { handle: DocHandle<Doc>; view: EditorView }) => ({
      doc: p.handle.doc().text,
      editor: p.view.state.doc.toString(),
    });
    /** Mounted with the caret at the end of the text. */
    function atEnd(text: string, extra: Extension = []) {
      const p = mount(text, () => {}, extra);
      p.view.dispatch({ selection: { anchor: text.length } });
      return p;
    }

    it("keys typed in one frame become one change, after the paint", async () => {
      const { handle, view } = atEnd("abc");
      const before = changes(handle);
      type(view, "d");
      type(view, "e");
      expect(view.state.doc.toString()).toBe("abcde");
      expect(handle.doc().text).toBe("abc");
      await afterPaint();
      expect(handle.doc().text).toBe("abcde");
      expect(changes(handle) - before).toBe(1);
      view.destroy();
    });

    it("a command after a waiting key is written in order, at once", () => {
      const { handle, view } = atEnd("abc");
      type(view, "d");
      view.dispatch({ changes: { from: 0, insert: "#" } });
      expect(handle.doc().text).toBe("#abcd");
      view.destroy();
    });

    it("a change arriving while a key waits keeps both, in the editor and the document", async () => {
      const p = atEnd("abc");
      type(p.view, "d");
      p.handle.change((d) => A.splice(d, ["text"], 1, 0, ">"));
      expect(textOf(p)).toEqual({ doc: "a>bcd", editor: "a>bcd" });
      const settled = changes(p.handle);
      await afterPaint();
      expect(textOf(p)).toEqual({ doc: "a>bcd", editor: "a>bcd" });
      expect(changes(p.handle)).toBe(settled);
      p.view.destroy();
    });

    it("two people typing at once converge to the same text", async () => {
      const a = mount("hello");
      const b = peerOf(a.handle);
      a.view.dispatch({ selection: { anchor: 5 } });
      b.view.dispatch({ selection: { anchor: 0 } });
      type(a.view, "!");
      type(b.view, "<");
      b.handle.update((d) => A.merge(d, a.handle.doc()));
      a.handle.update((d) => A.merge(d, b.handle.doc()));
      await afterPaint();
      b.handle.update((d) => A.merge(d, a.handle.doc()));
      a.handle.update((d) => A.merge(d, b.handle.doc()));
      for (const p of [a, b]) expect(textOf(p)).toEqual({ doc: "<hello!", editor: "<hello!" });
      a.view.destroy();
      b.view.destroy();
    });

    it("a peer's key merged while mine waits: concurrent changes, both texts converge", async () => {
      const a = mount("hello");
      const b = peerOf(a.handle);
      const base = A.getHeads(a.handle.doc());
      a.view.dispatch({ selection: { anchor: 5 } });
      b.view.dispatch({ selection: { anchor: 0 } });
      type(a.view, "!");
      type(b.view, "<");
      b.view.contentDOM.dispatchEvent(new FocusEvent("blur")); // b's key is written, a's waits
      expect(a.handle.doc().text).toBe("hello");
      a.handle.update((d) => A.merge(d, b.handle.doc()));
      expect(textOf(a)).toEqual({ doc: "<hello!", editor: "<hello!" });
      const heads = A.getHeads(a.handle.doc());
      expect(heads).toHaveLength(2);
      for (const h of heads) expect(A.inspectChange(a.handle.doc(), h)?.deps).toEqual(base);
      b.handle.update((d) => A.merge(d, a.handle.doc()));
      await afterPaint();
      for (const p of [a, b]) expect(textOf(p)).toEqual({ doc: "<hello!", editor: "<hello!" });
      expect(A.getHeads(b.handle.doc()).sort()).toEqual([...heads].sort());
      a.view.destroy();
      b.view.destroy();
    });

    it("a key typed after a rebased one, before the paint, is written once at its place", async () => {
      const p = atEnd("abc");
      type(p.view, "d");
      p.handle.change((d) => A.splice(d, ["text"], 1, 0, ">"));
      const settled = changes(p.handle);
      type(p.view, "e");
      expect(textOf(p)).toEqual({ doc: "a>bcd", editor: "a>bcde" });
      await afterPaint();
      expect(textOf(p)).toEqual({ doc: "a>bcde", editor: "a>bcde" });
      expect(changes(p.handle) - settled).toBe(1);
      p.view.destroy();
    });

    it("undoing a rebased key takes it out of the editor and the document", () => {
      const p = atEnd("abc", [history(), localHistoryOnly]);
      type(p.view, "d");
      p.handle.change((d) => A.splice(d, ["text"], 1, 0, ">"));
      expect(undo(p.view)).toBe(true);
      expect(textOf(p)).toEqual({ doc: "a>bc", editor: "a>bc" });
      p.view.destroy();
    });

    it("a key typed at several cursors waits through a peer's change", async () => {
      const p = mount("abc", () => {}, EditorState.allowMultipleSelections.of(true));
      p.view.dispatch({
        selection: EditorSelection.create([EditorSelection.cursor(1), EditorSelection.cursor(3)]),
      });
      typeAtEach(p.view, "x");
      expect(textOf(p)).toEqual({ doc: "abc", editor: "axbcx" });
      p.handle.change((d) => A.splice(d, ["text"], 0, 0, ">"));
      expect(textOf(p)).toEqual({ doc: ">axbcx", editor: ">axbcx" });
      await afterPaint();
      expect(textOf(p)).toEqual({ doc: ">axbcx", editor: ">axbcx" });
      p.view.destroy();
    });

    it("an input method's text (compose) waits through a peer's change", async () => {
      const p = atEnd("abc");
      type(p.view, "é", "input.type.compose");
      expect(textOf(p)).toEqual({ doc: "abc", editor: "abcé" });
      p.handle.change((d) => A.splice(d, ["text"], 0, 0, ">"));
      expect(textOf(p)).toEqual({ doc: ">abcé", editor: ">abcé" });
      await afterPaint();
      expect(textOf(p)).toEqual({ doc: ">abcé", editor: ">abcé" });
      p.view.destroy();
    });

    it("ADR 0028: a big typed text caught by a peer's change is written in changes of 64 Ki", () => {
      const p = atEnd("abc");
      const big = "ab".repeat(80 * 1024);
      type(p.view, big);
      const before = changes(p.handle);
      p.handle.change((d) => A.splice(d, ["text"], 0, 0, ">"));
      expect(textOf(p)).toEqual({ doc: `>abc${big}`, editor: `>abc${big}` });
      expect(changes(p.handle) - before).toBe(1 + Math.ceil(big.length / CHUNK_CHARS));
      p.view.destroy();
    });

    it("a key a later input handler swallowed does not make the next paste wait", () => {
      const p = atEnd(
        "abc",
        EditorView.inputHandler.of(() => true),
      );
      type(p.view, "x");
      expect(textOf(p)).toEqual({ doc: "abc", editor: "abc" });
      p.view.dispatch({ changes: { from: 0, insert: "P" }, userEvent: "input.paste" });
      expect(textOf(p)).toEqual({ doc: "Pabc", editor: "Pabc" });
      p.view.destroy();
    });

    it("a peer's change on the same heads, landing first, is not taken for the waiting key", async () => {
      const handle = repo.create<Doc>({ text: "abc" });
      const q = A.change(A.merge(A.init<Doc>(), handle.doc()), (d) =>
        A.splice(d, ["text"], 0, 0, "Q"),
      );
      let merged = false;
      handle.on("change", () => {
        if (merged) return;
        merged = true;
        handle.update((d) => A.merge(d, q));
      });
      const p = { handle, view: attach(handle) };
      p.view.dispatch({ selection: { anchor: 3 } });
      type(p.view, "d");
      handle.change((d) => A.splice(d, ["text"], 1, 0, ">"));
      expect(textOf(p).editor).toBe(textOf(p).doc);
      await afterPaint();
      expect(textOf(p).editor).toBe(textOf(p).doc);
      expect(textOf(p).doc.match(/d/g)).toHaveLength(1);
      expect(textOf(p).doc).toContain("Q");
      p.view.destroy();
    });

    it("a key waiting when the document is replaced: the editor follows it, and still syncs", async () => {
      const errors = vi.spyOn(console, "error").mockImplementation(() => {});
      const p = atEnd("abc");
      type(p.view, "d");
      p.handle.update(() => A.from<Doc>({ text: "xyz" }));
      expect(textOf(p)).toEqual({ doc: "xyz", editor: "xyz" });
      p.handle.change((d) => A.splice(d, ["text"], 0, 0, ">"));
      expect(textOf(p)).toEqual({ doc: ">xyz", editor: ">xyz" });
      await afterPaint();
      expect(textOf(p)).toEqual({ doc: ">xyz", editor: ">xyz" });
      expect(errors).toHaveBeenCalled(); // the key that could not be written is reported
      errors.mockRestore();
      p.view.destroy();
    });

    it("a waiting key the rebase cannot write: the editor takes the document, which still syncs", () => {
      const errors = vi.spyOn(console, "error").mockImplementation(() => {});
      const p = atEnd("abc");
      // The editor ahead of the document by text the document never had: the key lands past
      // its end, so the concurrent write throws inside Automerge.
      p.view.dispatch({
        changes: { from: 3, insert: "zz" },
        annotations: reconcileAnnotationType.of({}),
      });
      p.view.dispatch({ selection: { anchor: 5 } });
      type(p.view, "d");
      p.handle.change((d) => A.splice(d, ["text"], 0, 0, ">"));
      expect(textOf(p)).toEqual({ doc: ">abc", editor: ">abc" });
      p.handle.change((d) => A.splice(d, ["text"], 0, 0, "<"));
      expect(textOf(p)).toEqual({ doc: "<>abc", editor: "<>abc" });
      expect(errors).toHaveBeenCalled();
      errors.mockRestore();
      p.view.destroy();
    });

    it("leaving the editor writes what waits", () => {
      const { handle, view } = atEnd("abc");
      type(view, "d");
      view.contentDOM.dispatchEvent(new FocusEvent("blur"));
      expect(handle.doc().text).toBe("abcd");
      type(view, "e");
      view.destroy();
      expect(handle.doc().text).toBe("abcde");
    });

    it("destroy with a handle that can no longer be written removes every listener", () => {
      const errors = vi.spyOn(console, "error").mockImplementation(() => {});
      const source = repo.create<Doc>({ text: "abc" });
      const handle = source.view(encodeHeads(A.getHeads(source.doc()))); // view-only: change throws
      const view = attach(handle);
      view.dispatch({ selection: { anchor: 3 } });
      type(view, "d");
      const removed = vi.spyOn(window, "removeEventListener");
      expect(() => view.destroy()).not.toThrow();
      expect(handle.listenerCount("change")).toBe(0);
      expect(removed.mock.calls.map((c) => c[0])).toContain("pagehide");
      expect(errors).toHaveBeenCalled();
      removed.mockRestore();
      errors.mockRestore();
    });

    describe("what reads or writes the document sees what the editor shows (no lag)", () => {
      it("a shortcut flushes the waiting key before its command writes at an editor offset", () => {
        const p = atEnd("abc");
        type(p.view, "d");
        p.view.contentDOM.dispatchEvent(
          new KeyboardEvent("keydown", { key: "s", ctrlKey: true, bubbles: true }),
        );
        expect(p.handle.doc().text).toBe("abcd");
        p.handle.change((d) => A.splice(d, ["text"], p.view.state.doc.length, 0, "\n"));
        expect(textOf(p)).toEqual({ doc: "abcd\n", editor: "abcd\n" });
        p.view.destroy();
      });

      it("a click (toolbar, menu, panel) flushes the waiting key", () => {
        const p = atEnd("abc");
        type(p.view, "d");
        document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
        expect(p.handle.doc().text).toBe("abcd");
        p.view.destroy();
      });

      it("a plain typed key and its modifier do not flush (the write stays after the paint)", () => {
        const p = atEnd("abc");
        type(p.view, "d");
        for (const key of ["Shift", "e", "E", "Dead"])
          p.view.contentDOM.dispatchEvent(
            new KeyboardEvent("keydown", { key, shiftKey: key === "E", bubbles: true }),
          );
        expect(p.handle.doc().text).toBe("abc");
        p.view.destroy();
      });

      it("flushTyped writes what waits; whenWritten runs once it is in the document", async () => {
        const p = atEnd("abc");
        let seen: string | undefined;
        whenWritten(p.handle, () => {
          seen = p.handle.doc().text;
        });
        expect(seen).toBe("abc"); // nothing waits: at once
        type(p.view, "d");
        whenWritten(p.handle, () => {
          seen = p.handle.doc().text;
        });
        expect(seen).toBe("abc");
        await afterPaint();
        expect(seen).toBe("abcd");
        type(p.view, "e");
        flushTyped(p.handle);
        expect(p.handle.doc().text).toBe("abcde");
        p.view.destroy();
      });

      it("whenWritten waits for the keys of every editor open on the document", async () => {
        const a = atEnd("abc");
        const b = attach(a.handle);
        type(b, "#");
        type(a.view, "d");
        let seen: string | undefined;
        whenWritten(a.handle, () => {
          seen = a.handle.doc().text;
        });
        a.view.contentDOM.dispatchEvent(new FocusEvent("blur"));
        expect(seen).toBe("#abcd");
        a.view.destroy();
        b.destroy();
      });

      it("save, export and compile (the projection) write the keys waiting in every editor", async () => {
        const a = atEnd("abc");
        const b = attach(a.handle);
        const writes: string[] = [];
        const projection = startProjection(
          a.handle,
          { writeProjection: async (_name, text) => void writes.push(text) },
          { schedule: () => undefined, known: "abc" },
        );
        type(a.view, "d");
        type(b, "#");
        expect(projection.dirty()).toBe(true);
        await projection.flush();
        expect(writes).toEqual(["#abcd"]);
        projection.stop();
        a.view.destroy();
        b.destroy();
      });

      it("a change arriving while a key waits is not written over from the projection's listener", async () => {
        const a = atEnd("abc");
        const projection = startProjection(
          a.handle,
          { writeProjection: async () => {} },
          { schedule: () => undefined, known: "abc" },
        );
        type(a.view, "d");
        a.handle.change((d) => A.splice(d, ["text"], 0, 0, "#"));
        await afterPaint();
        expect(textOf(a)).toEqual({ doc: "#abcd", editor: "#abcd" });
        expect(changes(a.handle)).toBe(3);
        projection.stop();
        a.view.destroy();
      });

      it("the cursor shown to others is taken once the key is in the document", async () => {
        const sent: Array<[string, unknown]> = [];
        const spy = vi
          .spyOn(Presence.prototype, "broadcast")
          .mockImplementation((channel, value) => void sent.push([String(channel), value]));
        const handle = repo.create<Doc>({ text: "abc" });
        let view: EditorView | undefined;
        const presence = presenceSync(handle, { id: "me", name: "Me" }, () => view);
        view = attach(handle, () => {}, presence.extension);
        view.dispatch({ selection: { anchor: 1 } });
        type(view, "x");
        await afterPaint();
        const head = sent.filter(([c]) => c === "head").at(-1)?.[1] as A.Cursor;
        expect(handle.doc().text).toBe("axbc");
        expect(A.getCursorPosition(handle.doc(), ["text"], head)).toBe(2);
        presence.stop();
        view.destroy();
        spy.mockRestore();
      });
    });
  });
});
