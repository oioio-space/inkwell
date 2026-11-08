import { Repo } from "@automerge/automerge-repo";
import { EditorState } from "@codemirror/state";
import { EditorView, showTooltip, type Tooltip } from "@codemirror/view";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./service", () => ({
  language: { request: vi.fn() },
  currentFile: vi.fn(() => "main.typ"),
}));

import { amSync } from "../editor/amSync";
import { requestHover, requestSignature, writingHelp } from "./extension";
import { diagnosticMarks, problemMarks } from "./marks";
import { currentFile, language } from "./service";

let view: EditorView;
let answer: (value: unknown) => void;
beforeEach(() => {
  vi.useFakeTimers();
  vi.mocked(currentFile).mockReturnValue("main.typ");
  vi.mocked(language.request).mockImplementation(
    () =>
      new Promise((resolve) => {
        answer = resolve;
      }),
  );
  const parent = document.createElement("div");
  document.body.append(parent);
  view = new EditorView({
    parent,
    state: EditorState.create({
      doc: "#foo(1)",
      selection: { anchor: 6 },
      extensions: [writingHelp, diagnosticMarks],
    }),
  });
});
afterEach(() => {
  view.destroy();
  document.body.replaceChildren();
  vi.restoreAllMocks();
  vi.useRealTimers();
});
const tooltipText = (tooltip: Tooltip | null | undefined) => tooltip?.create(view).dom.textContent;
const signatureTooltip = () => view.state.facet(showTooltip).find((t) => t !== null);

describe("signature help while typing (N29)", () => {
  // What CodeMirror's DOM input path does for a typed character: ask the input handlers, then
  // dispatch the default insert. amSync's handler marks it typed, so its write waits for the paint.
  function type(text: string) {
    const at = view.state.selection.main.head;
    const insert = () =>
      view.state.update({
        changes: { from: at, insert: text },
        selection: { anchor: at + text.length },
        userEvent: "input.type",
      });
    if (!view.state.facet(EditorView.inputHandler).some((h) => h(view, at, at, text, insert)))
      view.dispatch(insert());
  }
  it.each(["(", ","])(
    "appears by itself when %s is typed, despite the deferred write",
    async (key) => {
      view.destroy();
      const doc = key === "(" ? "#foo" : "#foo(1";
      const handle = new Repo().create<{ text: string }>({ text: doc });
      view = new EditorView({
        parent: document.body.appendChild(document.createElement("div")),
        state: EditorState.create({
          doc,
          selection: { anchor: doc.length },
          extensions: [writingHelp, amSync({ handle, path: ["text"] })],
        }),
      });
      // The language service reads the document, not the editor: a request made before the typed
      // key is written is about the old text, and the client drops its answer.
      vi.mocked(language.request).mockImplementation(async (method) =>
        method === "textDocument/signatureHelp" && handle.doc().text === view.state.doc.toString()
          ? { signatures: [{ label: "foo(value)", documentation: "Useful description." }] }
          : undefined,
      );
      type(key);
      expect(handle.doc().text).toBe(doc);
      await vi.advanceTimersByTimeAsync(50);
      expect(handle.doc().text).toBe(doc + key);
      expect(tooltipText(signatureTooltip())).toContain("foo(value)");
    },
  );
});

describe("current help survives purely decorative transactions", () => {
  it("renders a >100ms current signature after diagnostic decorations change", async () => {
    const pending = requestSignature(view);
    await vi.advanceTimersByTimeAsync(150);
    view.dispatch({ effects: problemMarks.of([{ from: 1, to: 4, severity: "error" }]) });
    answer({ signatures: [{ label: "foo(value)", documentation: "Useful description." }] });
    await pending;
    expect(tooltipText(signatureTooltip())).toContain("Useful description.");
  });
  it("renders a >100ms current hover after diagnostic decorations change", async () => {
    const pending = requestHover(view, 2);
    await vi.advanceTimersByTimeAsync(150);
    view.dispatch({ effects: problemMarks.of([{ from: 1, to: 4, severity: "error" }]) });
    answer({ contents: "Useful hover." });
    expect(tooltipText(await pending)).toBe("Useful hover.");
  });
});
for (const mutation of ["text", "caret", "file"] as const) {
  describe(`${mutation} changes invalidate help`, () => {
    function change() {
      if (mutation === "text") view.dispatch({ changes: { from: 5, to: 6, insert: "2" } });
      else if (mutation === "caret") view.dispatch({ selection: { anchor: 1 } });
      else vi.mocked(currentFile).mockReturnValue("other.typ");
    }
    it("does not render the old signature", async () => {
      const pending = requestSignature(view);
      change();
      answer({ signatures: [{ label: "stale signature" }] });
      await pending;
      expect(signatureTooltip()).toBeUndefined();
    });
    it("does not return the old hover", async () => {
      const pending = requestHover(view, 2);
      change();
      answer({ contents: "stale hover" });
      expect(await pending).toBeNull();
    });
  });
}
