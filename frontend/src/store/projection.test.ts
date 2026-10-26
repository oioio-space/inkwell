// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PROJECTION_DELAY_MS, type ProjectionSink, startProjection } from "./projection";
import { encodeSyncPoint } from "./syncPoint";

type Doc = { text: string };

class Sink implements ProjectionSink {
  writes: string[] = [];
  failWith: Error | undefined;
  inFlight = 0;
  maxInFlight = 0;
  async writeProjection(_name: string, text: string): Promise<void> {
    this.inFlight++;
    this.maxInFlight = Math.max(this.maxInFlight, this.inFlight);
    await Promise.resolve();
    this.inFlight--;
    if (this.failWith) throw this.failWith;
    this.writes.push(text);
  }
}

const repo = new Repo();
const type = (h: ReturnType<typeof repo.create<Doc>>, s: string) =>
  h.change((d) => A.splice(d, ["text"], d.text.length, 0, s));

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe("startProjection", () => {
  it("writes the text at start", async () => {
    const sink = new Sink();
    const h = repo.create<Doc>({ text: "= Titre" });
    const p = startProjection(h, sink);
    await vi.advanceTimersByTimeAsync(0);
    expect(sink.writes).toEqual(["= Titre"]);
    p.stop();
  });

  it("writes 300 ms after the last keystroke, not before", async () => {
    expect(PROJECTION_DELAY_MS).toBe(300);
    const sink = new Sink();
    const h = repo.create<Doc>({ text: "" });
    const p = startProjection(h, sink);
    await vi.advanceTimersByTimeAsync(0);
    sink.writes.length = 0;

    type(h, "a");
    await vi.advanceTimersByTimeAsync(200);
    type(h, "b");
    await vi.advanceTimersByTimeAsync(299);
    expect(sink.writes).toEqual([]);
    await vi.advanceTimersByTimeAsync(1);
    expect(sink.writes).toEqual(["ab"]);
    p.stop();
  });

  it("follows remote changes too, and skips a write when the text did not change", async () => {
    const sink = new Sink();
    const h = repo.create<Doc>({ text: "x" });
    const p = startProjection(h, sink);
    await vi.advanceTimersByTimeAsync(0);
    h.change((d) => A.updateText(d, ["text"], "x")); // no-op for the text
    await vi.advanceTimersByTimeAsync(400);
    expect(sink.writes).toEqual(["x"]);
    p.stop();
  });

  it("never has two writes in flight and ends on the latest text", async () => {
    const sink = new Sink();
    const h = repo.create<Doc>({ text: "" });
    const p = startProjection(h, sink, { delayMs: 10 });
    for (let i = 0; i < 20; i++) {
      type(h, String(i % 10));
      void p.flush();
      await vi.advanceTimersByTimeAsync(1);
    }
    await vi.advanceTimersByTimeAsync(50);
    expect(sink.maxInFlight).toBe(1);
    expect(sink.writes.at(-1)).toBe(h.doc()?.text);
    p.stop();
  });

  it("reports a failed write, keeps going, and retries at the next change", async () => {
    const sink = new Sink();
    sink.failWith = new Error("edited outside Inkwell");
    const errors: unknown[] = [];
    const h = repo.create<Doc>({ text: "a" });
    const p = startProjection(h, sink, { onError: (e) => errors.push(e) });
    await vi.advanceTimersByTimeAsync(0);
    expect(errors).toHaveLength(1);

    sink.failWith = undefined;
    type(h, "b");
    await vi.advanceTimersByTimeAsync(300);
    expect(sink.writes).toEqual(["ab"]);
    p.stop();
  });

  it("tries a failed write again by itself, and says when it went through", async () => {
    const sink = new Sink();
    sink.failWith = new Error("disk full");
    const errors: unknown[] = [];
    let written = 0;
    const h = repo.create<Doc>({ text: "a" });
    const p = startProjection(h, sink, {
      retryMs: 1000,
      onError: (e) => errors.push(e),
      onWritten: () => written++,
    });
    await vi.advanceTimersByTimeAsync(0);
    expect(errors).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1000);
    expect(errors).toHaveLength(2); // still failing: tried again, not given up

    sink.failWith = undefined;
    await vi.advanceTimersByTimeAsync(1000);
    expect(sink.writes).toEqual(["a"]);
    expect(written).toBe(1);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(errors).toHaveLength(2); // nothing left to retry
    p.stop();
  });

  it("does not retry what is not retryable (a file edited outside waits for the person)", async () => {
    const sink = new Sink();
    sink.failWith = new Error("edited outside Inkwell");
    const errors: unknown[] = [];
    const h = repo.create<Doc>({ text: "a" });
    const p = startProjection(h, sink, {
      retryMs: 1000,
      retryable: () => false,
      onError: (e) => errors.push(e),
    });
    await vi.advanceTimersByTimeAsync(10_000);
    expect(errors).toHaveLength(1);
    p.stop();
  });

  it("stop() also cancels a retry", async () => {
    const sink = new Sink();
    sink.failWith = new Error("disk full");
    const h = repo.create<Doc>({ text: "a" });
    const p = startProjection(h, sink, { retryMs: 1000 });
    await vi.advanceTimersByTimeAsync(0);
    p.stop();
    sink.failWith = undefined;
    await vi.advanceTimersByTimeAsync(5000);
    expect(sink.writes).toEqual([]);
  });

  it("flush(true) rewrites an unchanged text, as after ignoring an outside edit", async () => {
    const sink = new Sink();
    const h = repo.create<Doc>({ text: "same" });
    const p = startProjection(h, sink);
    await vi.advanceTimersByTimeAsync(0);
    await p.flush();
    expect(sink.writes).toEqual(["same"]);
    await p.flush(true);
    expect(sink.writes).toEqual(["same", "same"]);
    p.stop();
  });

  it("stop() cancels the pending write and detaches", async () => {
    const sink = new Sink();
    const h = repo.create<Doc>({ text: "" });
    const p = startProjection(h, sink);
    await vi.advanceTimersByTimeAsync(0);
    sink.writes.length = 0;
    type(h, "late");
    p.stop();
    await vi.advanceTimersByTimeAsync(1000);
    type(h, "later");
    await vi.advanceTimersByTimeAsync(1000);
    expect(sink.writes).toEqual([]);
  });
});

describe("the sync point", () => {
  it("goes to the sink with each write, at the heads the text was read at", async () => {
    const points: (string | undefined)[] = [];
    const sink = new (class extends Sink {
      override async writeProjection(name: string, text: string, point?: string) {
        points.push(point);
        await super.writeProjection(name, text);
      }
    })();
    const h = repo.create<Doc>({ text: "a" });
    const p = startProjection(h, sink);
    await vi.advanceTimersByTimeAsync(0);
    expect(points).toEqual([encodeSyncPoint(A.getHeads(h.doc()), "a")]);
    p.stop();
  });

  it("is recorded at open, without a write, when the document equals the file (Ctrl+S mode)", async () => {
    const recorded: string[] = [];
    const sink = Object.assign(new Sink(), {
      setSyncPoint: async (_name: string, point: string) => void recorded.push(point),
    });
    const h = repo.create<Doc>({ text: "a" });
    const p = startProjection(h, sink, { schedule: () => undefined, known: "a" });
    await vi.advanceTimersByTimeAsync(0);
    expect(sink.writes).toEqual([]);
    expect(recorded).toEqual([encodeSyncPoint(A.getHeads(h.doc()), "a")]);
    p.stop();
  });

  it("is not recorded at open when the document differs from the file", async () => {
    const recorded: string[] = [];
    const sink = Object.assign(new Sink(), {
      setSyncPoint: async (_name: string, point: string) => void recorded.push(point),
    });
    const h = repo.create<Doc>({ text: "b" });
    const p = startProjection(h, sink, { schedule: () => undefined, known: "a" });
    await vi.advanceTimersByTimeAsync(0);
    expect(recorded).toEqual([]);
    p.stop();
  });
});
