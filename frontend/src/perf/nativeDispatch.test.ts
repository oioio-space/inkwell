import { EditorView } from "@codemirror/view";
import { expect, it } from "vitest";
import {
  dispatchNativeTransactions,
  profileNativeStage,
  registerNativeRecorder,
  registerNativeStages,
} from "./nativeDispatch";

it("wraps actual CM input dispatch including view update, and restores the default path", () => {
  const parent = document.createElement("div");
  document.body.append(parent);
  const view = new EditorView({
    parent,
    doc: "fixture",
    dispatchTransactions: dispatchNativeTransactions,
  });
  const stages: string[] = [];
  const off = registerNativeRecorder(view, (transactions, forward) => {
    expect(transactions[0]?.isUserEvent("input")).toBe(true);
    stages.push(view.state.doc.toString());
    forward();
    stages.push(view.state.doc.toString());
  });
  view.dispatch({ changes: { from: 7, insert: "x" }, userEvent: "input.type" });
  expect(stages).toEqual(["fixture", "fixturex"]);
  off();
  view.dispatch({ changes: { from: 8, insert: "y" }, userEvent: "input.type" });
  expect(stages).toHaveLength(2);
  expect(view.state.doc.toString()).toBe("fixturexy");
  view.destroy();
  parent.remove();
});

it("profiles nested input stages only while the native capability owns a recorder", () => {
  const stages: { event: string; start: number; end: number }[] = [];
  expect(profileNativeStage("automerge", () => 42)).toBe(42);
  const off = registerNativeStages((stage) => stages.push(stage));
  expect(
    profileNativeStage("automerge", () => profileNativeStage("language-didChange", () => 42)),
  ).toBe(42);
  expect(stages.map((stage) => stage.event)).toEqual(["language-didChange", "automerge"]);
  expect(stages.every((stage) => stage.end >= stage.start)).toBe(true);
  off();
  profileNativeStage("automerge", () => 42);
  expect(stages).toHaveLength(2);
});
