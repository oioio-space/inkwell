import { afterEach, describe, expect, it, vi } from "vitest";
import { findAll, SearchTooCostly } from "./find";
import { createWorkerRunner, inlineRunner, type SearchJob, type WorkerLike } from "./runner";
import { scanSync } from "./scan";
import { compile } from "./testkit";

/** A worker that answers like the real one, or never answers. */
class FakeWorker implements WorkerLike {
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  terminated = false;
  constructor(private readonly answers: boolean) {
    queueMicrotask(() => this.onmessage?.({ data: { ready: true } }));
  }
  postMessage(message: unknown) {
    if (!this.answers) return;
    const { id, text, matcher, range, limit } = message as { id: number } & SearchJob;
    const matches = scanSync(text, matcher, range, limit);
    queueMicrotask(() => this.onmessage?.({ data: { id, matches } }));
  }
  terminate() {
    this.terminated = true;
  }
}

describe("search worker runner", () => {
  afterEach(() => vi.useRealTimers());

  it("SRC-07 a catastrophic regex is cut after 200 ms and says the search is too costly", async () => {
    vi.useFakeTimers();
    const workers: FakeWorker[] = [];
    const runner = createWorkerRunner(() => {
      // The first worker hangs on `(a+)+$`; the one started afterwards is healthy.
      const w = new FakeWorker(workers.length > 0);
      workers.push(w);
      return w;
    });
    const hostile = compile("(a+)+$", { mode: "regex" });
    expect(hostile.guarded).toBe(true);
    const text = `${"a".repeat(100_000)}!`;

    const settled = vi.fn();
    const first = findAll(text, hostile, { runner }).catch((e: unknown) => {
      settled(e);
      return e;
    });
    await vi.advanceTimersByTimeAsync(199);
    expect(settled).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(2);
    const err = await first;
    expect(err).toBeInstanceOf(SearchTooCostly);
    expect((err as Error).message).toMatch(/trop coûteuse/);
    expect(workers[0]?.terminated).toBe(true);

    // The next search gets a fresh worker and works.
    const ok = await findAll("aa b aa", compile("a+", { mode: "regex" }), { runner });
    expect(ok.map((m) => m.matched)).toEqual(["aa", "aa"]);
    expect(workers).toHaveLength(2);
  });

  it("does not count the worker's start-up against the 200 ms", async () => {
    vi.useFakeTimers();
    let release: () => void = () => {};
    const runner = createWorkerRunner(() => {
      const w = new FakeWorker(true);
      w.onmessage = null;
      const gate = new Promise<void>((r) => {
        release = r;
      });
      void gate.then(() => w.onmessage?.({ data: { ready: true } }));
      return w;
    });
    const p = findAll("a", compile("a", { mode: "regex" }), { runner });
    await vi.advanceTimersByTimeAsync(1000); // slow start-up: no timeout yet
    release();
    await expect(p).resolves.toHaveLength(1);
  });

  it("jobs run one after the other", async () => {
    const runner = createWorkerRunner(() => new FakeWorker(true));
    const m = compile("b", { mode: "regex" });
    const [x, y] = await Promise.all([
      findAll("abc", m, { runner }),
      findAll("bbb", m, { runner }),
    ]);
    expect([x.length, y.length]).toEqual([1, 3]);
  });

  it("plain queries never need the worker", async () => {
    const never = { run: () => Promise.reject(new Error("must not be called")) };
    const r = await findAll("a b a", compile("a"), { runner: never });
    expect(r).toHaveLength(2);
    expect(
      await inlineRunner.run({ text: "x", matcher: compile("x"), range: {}, limit: 5 }),
    ).toHaveLength(1);
  });
});
