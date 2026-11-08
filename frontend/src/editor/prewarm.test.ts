import { Repo } from "@automerge/automerge-repo";
import { EditorView } from "@codemirror/view";
import { expect, it } from "vitest";
import { prewarmTyping } from "./prewarm";

it("the typing warm-up leaves focus, the document and the page as they were", async () => {
  const handle = new Repo().create<{ text: string }>({ text: "abc" });
  const view = new EditorView({ parent: document.body, doc: "abc" });
  view.focus();
  const children = document.body.childElementCount;
  const editors = document.querySelectorAll(".cm-content, [role=textbox]").length;
  const focused: EventTarget[] = [];
  const onFocus = (e: FocusEvent) => focused.push(e.target as EventTarget);
  document.addEventListener("focusin", onFocus, true);
  const typed: string[] = [];
  prewarmTyping(
    EditorView.updateListener.of((u) => {
      for (const tr of u.transactions)
        if (tr.isUserEvent("input.type"))
          tr.changes.iterChanges((_a, _b, _c, _d, text) => typed.push(text.toString()));
    }),
  );
  document.removeEventListener("focusin", onFocus, true);
  // Nothing outlives the call: no second editor for a lookup to find, no box left in the page.
  expect(document.querySelectorAll(".cm-content, [role=textbox]").length).toBe(editors);
  expect(document.body.childElementCount).toBe(children);
  // Focus never moved: assistive technology has nothing hidden to announce.
  expect(focused).toEqual([]);
  expect(document.activeElement).toBe(view.contentDOM);
  // CodeMirror's DOM-change path ran (each "w","a","r","m" read back from the DOM), then typing.
  expect(typed.join("")).toBe("warmxxxxxxxx");
  expect(handle.doc().text).toBe("abc");
  expect(view.state.doc.toString()).toBe("abc");
  await new Promise<void>((done) =>
    requestAnimationFrame(() => setTimeout(() => setTimeout(done, 0), 0)),
  );
  expect(document.body.childElementCount).toBe(children);
  view.destroy();
});
