import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { analyze, inlineAnalyzer, type WorkerLike, workerAnalyzer } from "./analysis";
import { Device } from "./testkit";
import { Tracker } from "./track";
import type { HDoc } from "./types";

/** A worker that runs the real analysis in the same thread, with the worker's own document. */
function fakeWorker(): WorkerLike {
  let doc = A.init<HDoc>();
  const tracker = new Tracker();
  const w: WorkerLike = {
    onmessage: null,
    onerror: null,
    terminate() {},
    postMessage(m) {
      const { id, changes, since } = m as { id: number; changes: Uint8Array[]; since: string[] };
      [doc] = A.applyChanges(doc, changes);
      void analyze(doc, tracker, since).then(({ blame, ...rest }) => {
        const { sessions: _s, ...b } = blame;
        w.onmessage?.({ data: { id, result: { ...rest, blame: b } } });
      });
    },
  };
  queueMicrotask(() => w.onmessage?.({ data: { ready: true } }));
  return w;
}

describe("analysis in a worker", () => {
  it("the first request sends the whole history, the next only what is new", async () => {
    const a = new Device("aa01").type("un ", 0).claim("Alice");
    const client = workerAnalyzer(fakeWorker);
    const first = await client.analyze(a.doc, a.heads);
    expect(first.sessions.length).toBeGreaterThan(0);
    expect(first.blame.length).toBe(a.doc.text.length);
    a.type("deux", 3, 3600);
    const second = await client.analyze(a.doc, a.heads);
    const inline = await inlineAnalyzer().analyze(a.doc, a.heads);
    expect(second.sessions.map((s) => s.id)).toEqual(inline.sessions.map((s) => s.id));
    expect(second.blame.spans).toEqual(inline.blame.spans);
    expect(second.blame.sessions).toBe(second.sessions);
    client.dispose();
  });

  it("a document that arrived whole (nothing sent before) is analysed completely", async () => {
    const a = new Device("aa01").type("texte reçu", 0);
    const loaded = A.load<HDoc>(A.save(a.doc));
    const result = await workerAnalyzer(fakeWorker).analyze(loaded, undefined);
    expect(result.blame.length).toBe("texte reçu".length);
    expect(result.sinceIdx).toBe(result.sessions.length);
  });
});
