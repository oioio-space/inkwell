import { type DocHandle, Repo } from "@automerge/automerge-repo";
import { autocompletion, completionStatus } from "@codemirror/autocomplete";
import { history, undo } from "@codemirror/commands";
import { EditorSelection, EditorState, type Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { describe, expect, it } from "vitest";
import { amSync } from "./amSync";
import { ownTyping } from "./ownTyping";

// What the prototype changes and what it leaves to the browser (PERF-TYPE-native, part 3).
const mount = (text: string, extra: Extension = []) =>
  new EditorView({
    parent: document.body,
    state: EditorState.create({
      doc: text,
      selection: { anchor: text.length },
      extensions: [extra, ownTyping()],
    }),
  });
const beforeinput = (view: EditorView, init: InputEventInit) => {
  const event = new InputEvent("beforeinput", { bubbles: true, cancelable: true, ...init });
  view.contentDOM.dispatchEvent(event);
  return event;
};
const typeKey = (view: EditorView, data: string) =>
  beforeinput(view, { inputType: "insertText", data });

describe("ownTyping (prototype, perf builds only)", () => {
  it("inserts a typed character itself: the browser's default action is prevented", () => {
    const seen: string[] = [];
    const view = mount(
      "ab",
      EditorView.updateListener.of((u) => {
        for (const tr of u.transactions) if (tr.isUserEvent("input.type")) seen.push("input.type");
      }),
    );
    expect(typeKey(view, "x").defaultPrevented).toBe(true);
    expect(view.state.doc.toString()).toBe("abx");
    expect(view.state.selection.main.head).toBe(3);
    expect(seen).toEqual(["input.type"]);
    view.destroy();
  });

  it("asks the input handlers first, like CodeMirror's own default insert", () => {
    const calls: Array<[number, number, string]> = [];
    const view = mount(
      "ab",
      EditorView.inputHandler.of((_v, from, to, text) => {
        calls.push([from, to, text]);
        return text === "(";
      }),
    );
    typeKey(view, "(");
    typeKey(view, "x");
    expect(calls).toEqual([
      [2, 2, "("],
      [2, 2, "x"],
    ]);
    expect(view.state.doc.toString()).toBe("abx");
    view.destroy();
  });

  it("replaces a selection, and types at every cursor", () => {
    const view = mount("abcd", EditorState.allowMultipleSelections.of(true));
    view.dispatch({ selection: EditorSelection.create([EditorSelection.range(1, 3)]) });
    typeKey(view, "x");
    expect(view.state.doc.toString()).toBe("axd");
    view.dispatch({
      selection: EditorSelection.create([EditorSelection.cursor(0), EditorSelection.cursor(2)]),
    });
    typeKey(view, "y");
    expect(view.state.doc.toString()).toBe("yaxyd");
    view.destroy();
  });

  it("leaves composition (IME, dead keys), deletion and spell-check replacement to the browser", () => {
    const view = mount("ab");
    for (const init of [
      { inputType: "insertText", data: "é", isComposing: true },
      { inputType: "insertCompositionText", data: "´" },
      { inputType: "deleteContentBackward" },
      { inputType: "insertReplacementText", data: "abc" },
      { inputType: "insertFromPaste", data: "zz" },
    ]) {
      expect(beforeinput(view, init).defaultPrevented, init.inputType).toBe(false);
    }
    expect(view.state.doc.toString()).toBe("ab");
    view.destroy();
  });

  it("does not type into a read-only editor", () => {
    const view = mount("ab", EditorState.readOnly.of(true));
    expect(typeKey(view, "x").defaultPrevented).toBe(false);
    expect(view.state.doc.toString()).toBe("ab");
    view.destroy();
  });

  it("keeps undo grouping: keys typed in a row are undone together", () => {
    const view = mount("ab", history());
    typeKey(view, "x");
    typeKey(view, "y");
    typeKey(view, "z");
    expect(view.state.doc.toString()).toBe("abxyz");
    undo(view);
    expect(view.state.doc.toString()).toBe("ab");
    view.destroy();
  });

  it("still opens the typing help (autocompletion reacts to input.type)", async () => {
    const view = mount(
      "",
      autocompletion({
        override: [(ctx) => ({ from: ctx.pos - 1, options: [{ label: "xyz" }] })],
        activateOnTypingDelay: 0,
      }),
    );
    typeKey(view, "x");
    await new Promise<void>((done) => setTimeout(done, 50));
    expect(completionStatus(view.state)).toBe("active");
    view.destroy();
  });

  it("keeps the document write after the paint (amSync sees a typed key)", async () => {
    const handle: DocHandle<{ text: string }> = new Repo().create({ text: "ab" });
    const view = mount("ab", amSync({ handle, path: ["text"] }));
    typeKey(view, "x");
    expect(view.state.doc.toString()).toBe("abx");
    expect(handle.doc().text).toBe("ab");
    await new Promise<void>((done) => requestAnimationFrame(() => setTimeout(done, 0)));
    await new Promise<void>((done) => setTimeout(done, 0));
    expect(handle.doc().text).toBe("abx");
    view.destroy();
  });
});
