import { selectAll } from "@codemirror/commands";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, describe, expect, it, vi } from "vitest";
import { session } from "../ui/editorSession.svelte";
import type { Anchor } from "./protocol";

vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  PreviewService: {},
}));

let answer: (a: Anchor[]) => void = () => {};
vi.mock("./preview.svelte", () => ({
  preview: { anchors: () => new Promise<Anchor[]>((resolve) => (answer = resolve)) },
}));

const { jumpFromPage } = await import("./actions");

const TEXT = "= Un\nBonjour.\n\n= Deux\nFin.\n";
const ANCHORS: Anchor[] = [
  { heading: true, page: 1, x: 70, y: 70, text: "Un" },
  { heading: true, page: 2, x: 70, y: 70, text: "Deux" },
];

function editor(): EditorView {
  const view = new EditorView({
    parent: document.body,
    state: EditorState.create({ doc: TEXT }),
    dispatchTransactions: (trs, v) => {
      v.update(trs);
      session.note(v.state, trs);
    },
  });
  session.view = view;
  session.note(view.state, []);
  return view;
}

afterEach(() => {
  session.view?.destroy();
  session.view = undefined;
  session.state = undefined;
});

describe("jumpFromPage", () => {
  it("puts the caret on the line of the page", async () => {
    const view = editor();
    const done = jumpFromPage(2, 0, 70);
    answer(ANCHORS);
    await done;
    expect(view.state.doc.lineAt(view.state.selection.main.head).number).toBe(4);
  });

  // R-E2E1-4 (PRS-33): leaving a presentation jumps to the slide's line once the preview has
  // answered; a person who has already selected everything and pressed Delete lost one character.
  it("E2E2 a jump answered after the person acted on the editor leaves their selection alone", async () => {
    const view = editor();
    const done = jumpFromPage(2, 0, 70);
    selectAll(view);
    answer(ANCHORS);
    await done;
    expect(view.state.selection.main.from).toBe(0);
    expect(view.state.selection.main.to).toBe(TEXT.length);
  });

  it("E2E2 a change from someone else before the answer does not cancel the jump", async () => {
    const view = editor();
    const done = jumpFromPage(2, 0, 70);
    view.dispatch({ changes: { from: TEXT.length, insert: "Ajouté.\n" } }); // no userEvent: remote
    answer(ANCHORS);
    await done;
    expect(view.state.doc.lineAt(view.state.selection.main.head).number).toBe(4);
  });
});
