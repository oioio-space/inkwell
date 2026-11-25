import { ChangeSet, EditorState, Text, type TransactionSpec } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ActiveTime, IDLE_MS } from "./active";
import { activity, type Counts } from "./index";
import { WordTracker } from "./words";

const bridge = vi.hoisted(() => ({
  Descriptors: vi.fn(async () => []),
  Declare: vi.fn(async (_ds: object[]) => {}),
  Flush: vi.fn(async (_batch: object[]) => {}),
}));
vi.mock("../../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  StatsService: bridge,
}));

const DOC = "3dxpJ7Vv6fHQm2BRq1xUqMr8KiW5";

/** Counts recorded as they are added. */
function recorder(): Counts & { words: () => number; active: () => number; calls: string[] } {
  const calls: string[] = [];
  let words = 0;
  let active = 0;
  return {
    calls,
    words: () => words,
    active: () => active,
    add: (id, v) => {
      calls.push(id);
      if (id === "activity.words") words += v;
    },
    duration: (id, ms) => {
      calls.push(id);
      if (id === "activity.active") active += ms;
    },
  };
}

function editor(text: string, counts: Counts) {
  return new EditorView({
    parent: document.body,
    doc: text,
    extensions: [activity(DOC, { counts })],
  });
}

/** Types `text` at `at`, one character a transaction, as a person does. */
function type(view: EditorView, at: number, text: string) {
  for (const [i, ch] of [...text].entries())
    view.dispatch({ changes: { from: at + i, insert: ch }, userEvent: "input.type" });
}

/** An edit that is not the person's: someone else's, arriving through the document. */
const remote = (view: EditorView, spec: TransactionSpec) => view.dispatch(spec);

afterEach(() => {
  vi.useRealTimers();
  document.body.replaceChildren();
});

describe("OBS1 activity", () => {
  it("OBS1 T-1 words written count only the person's own edits, mapped through remote ones, at a cost per transaction independent of the document size", () => {
    // Remote words before, after and inside the person's line never count; the person's own do.
    const counts = recorder();
    const view = editor("Titre\n\nfin", counts);
    type(view, 6, "Bonjour le monde");
    remote(view, { changes: { from: 0, insert: "un deux trois quatre " } });
    remote(view, { changes: { from: view.state.doc.length, insert: " cinq six" } });
    const line = view.state.doc.line(2);
    remote(view, { changes: { from: line.from, insert: "gamma " } });
    type(view, view.state.doc.line(2).to, " encore");
    view.destroy();
    expect(view.state.doc.line(2).text).toBe("gamma Bonjour le monde encore");
    expect(counts.words()).toBe(4);

    // A remote deletion of the person's words takes them back; their own deletions too, and a
    // window that removes more than it adds counts nothing.
    const c2 = recorder();
    const v2 = editor("a b c", c2);
    type(v2, 5, " un deux trois");
    const at = v2.state.doc.toString().indexOf(" deux");
    remote(v2, { changes: { from: at, to: at + 5 } });
    v2.dispatch({ changes: { from: 0, to: 2 }, userEvent: "delete.backward" });
    v2.destroy();
    expect(v2.state.doc.toString()).toBe("b c un trois");
    expect(c2.words()).toBe(1);
    const c3 = recorder();
    const v3 = editor("un deux trois quatre", c3);
    v3.dispatch({ changes: { from: 0, to: 14 }, userEvent: "delete.cut" });
    v3.dispatch({ changes: { from: 0, insert: "cinq " }, userEvent: "input.paste" });
    v3.destroy();
    expect(c3.words()).toBe(0);
    // Pasted words count; a word typed in two pieces is one; a new line splits nothing it should not.
    const c4 = recorder();
    const v4 = editor("", c4);
    v4.dispatch({
      changes: { from: 0, insert: "un deux\ntrois quatre cinq" },
      userEvent: "input.paste",
    });
    type(v4, v4.state.doc.length, " hel");
    type(v4, v4.state.doc.length, "lo\nfin");
    v4.destroy();
    expect(c4.words()).toBe(7);

    // The cost: per transaction one change-set operation, and no text read, whatever the size.
    const work = (lines: number) => {
      let state = EditorState.create({
        doc: Array.from({ length: lines }, (_, i) => `ligne ${i}`).join("\n"),
      });
      const tracker = new WordTracker(state.doc);
      const spied = [
        vi.spyOn(ChangeSet.prototype, "compose"),
        vi.spyOn(ChangeSet.prototype, "map"),
        vi.spyOn(ChangeSet.prototype, "invert"),
      ];
      const protos = new Set([
        Text.prototype,
        Object.getPrototypeOf(state.doc),
        Object.getPrototypeOf(Text.of(["x"])),
      ]);
      const read = [...protos].flatMap((p) =>
        (["sliceString", "toString"] as const)
          .filter((name) => Object.hasOwn(p, name))
          .map((name) => vi.spyOn(p as Record<string, () => string>, name)),
      );
      let ops = 0;
      let reads = 0;
      for (let i = 0; i < 300; i++) {
        const own = i % 3 !== 0;
        const at = Math.floor(state.doc.length / 2) + (i % 7);
        const tr = state.update({
          changes: { from: at, insert: own ? "x" : "y " },
          ...(own ? { userEvent: "input.type" } : {}),
        });
        for (const s of [...spied, ...read]) s.mockClear();
        if (own) tracker.local(tr.changes, state.doc);
        else tracker.remote(tr.changes);
        ops += spied.reduce((n, s) => n + s.mock.calls.length, 0);
        reads += read.reduce((n, s) => n + s.mock.calls.length, 0);
        state = tr.state;
      }
      state.doc.sliceString(0, 1); // the spies see a read
      expect(read.some((s) => s.mock.calls.length > 0)).toBe(true);
      for (const s of [...spied, ...read]) s.mockRestore();
      return { ops, reads, words: tracker.take(state.doc) };
    };
    const small = work(10);
    const big = work(200_000);
    expect(small.reads).toBe(0);
    expect(big).toEqual(small);
    expect(small.ops).toBe(200 * 2 + 100); // an invert and a compose for an own edit, a map for a remote one
  });

  it("OBS1 T-2 active time stops after 60 s without an edit", () => {
    const t = new ActiveTime();
    for (const at of [0, 10_000, 30_000, 95_000, 100_000, 160_000, 160_001 + IDLE_MS]) t.stamp(at);
    // 0 → 30 s active, 30 → 95 s idle (65 s), 95 → 100 → 160 s active, then idle again.
    expect(t.take()).toBe(30_000 + 5_000 + 60_000);
    expect(t.take()).toBe(0);

    // In the editor: the window ends a minute after the last edit, and the time before counts.
    vi.useFakeTimers({ now: 0 });
    const counts = recorder();
    const view = editor("", counts);
    type(view, 0, "un");
    vi.advanceTimersByTime(20_000);
    type(view, 2, " deux");
    vi.advanceTimersByTime(IDLE_MS - 1);
    expect(counts.words()).toBe(0); // not idle yet: a minute after the last edit is not over
    vi.advanceTimersByTime(1);
    expect([counts.words(), counts.active()]).toEqual([2, 20_000]);
    // Two hours later, one edit: the gap is idle, and nothing more is active.
    vi.advanceTimersByTime(2 * 3600_000);
    type(view, view.state.doc.length, " trois");
    vi.advanceTimersByTime(IDLE_MS);
    expect([counts.words(), counts.active()]).toEqual([3, 20_000]);
    view.destroy();
  });

  it("OBS1 Z-2 typing makes no bridge call until the flush", async () => {
    vi.useFakeTimers({ now: 0 });
    const view = new EditorView({ parent: document.body, doc: "", extensions: [activity(DOC)] });
    await vi.advanceTimersByTimeAsync(0);
    // The session declares its metrics once, at start, before any typing.
    expect(bridge.Declare).toHaveBeenCalledTimes(1);
    expect(bridge.Declare.mock.calls[0]?.[0]).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: "activity.words", unit: "words", scope: "document" }),
      ]),
    );
    for (const f of Object.values(bridge)) f.mockClear();
    for (let i = 0; i < 40; i++) {
      type(view, view.state.doc.length, "mot ");
      await vi.advanceTimersByTimeAsync(1000);
    }
    expect(Object.values(bridge).map((f) => f.mock.calls.length)).toEqual([0, 0, 0]);
    // The minute's flush hands the sums over in one call; the window then ends a minute after
    // the last key, and the next flush carries the words.
    await vi.advanceTimersByTimeAsync(20_000);
    expect(bridge.Flush).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(2 * 60_000);
    const batch = bridge.Flush.mock.calls.flatMap(([batch]) => batch);
    expect(batch).toContainEqual({ m: "activity.words", doc: DOC, v: 40 });
    expect(batch).toContainEqual({ m: "activity.opened", doc: DOC, v: 1 });
    expect(
      Object.values(bridge)
        .slice(0, 2)
        .map((f) => f.mock.calls.length),
    ).toEqual([0, 0]);
    view.destroy();
  });
});
