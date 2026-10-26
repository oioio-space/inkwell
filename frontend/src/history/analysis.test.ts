import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import {
  analyze,
  HUGE_TEXT,
  inlineAnalyzer,
  Superseded,
  tooCostly,
  type WorkerLike,
  workerAnalyzer,
} from "./analysis";
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

  it("a document too big to follow lists its sessions and skips who-wrote-what", async () => {
    // A 5 MB text once typed (its deleted characters still count) with 100 sessions after it.
    expect(tooCostly(5_000_000, 100)).toBe(true);
    // A page of notes with a long history is followed in full.
    expect(tooCostly(20_000, 2_000)).toBe(false);
    const a = new Device("aa01").type("texte", 0).type("!", 5, 3600);
    const result = await inlineAnalyzer().analyze(a.doc, undefined);
    expect(result.limited).toBe(false);
    expect(result.blame.spans.length).toBeGreaterThan(0);
  });

  it("a huge text is not copied into a worker at all", async () => {
    const huge = { text: "x".repeat(HUGE_TEXT + 1) } as A.Doc<HDoc>;
    let spawned = false;
    const client = workerAnalyzer(() => {
      spawned = true;
      return fakeWorker();
    });
    const result = await client.analyze(huge, undefined);
    expect(result.limited).toBe(true);
    expect(result.sessions).toEqual([]);
    expect(spawned).toBe(false);
  });

  it("keeps one job in flight and one pending: the jobs replaced meanwhile are dropped", async () => {
    const sent: number[] = [];
    const spawn = (): WorkerLike => {
      const w = fakeWorker();
      const post = w.postMessage.bind(w);
      w.postMessage = (m) => {
        sent.push((m as { id: number }).id);
        post(m);
      };
      return w;
    };
    const a = new Device("aa01").type("un", 0);
    const client = workerAnalyzer(spawn);
    const first = client.analyze(a.doc, undefined);
    a.type(" deux", 2, 1);
    const second = client.analyze(a.doc, undefined);
    a.type(" trois", 7, 1);
    const third = client.analyze(a.doc, undefined);
    await expect(second).rejects.toBeInstanceOf(Superseded);
    const r1 = await first;
    const r3 = await third;
    expect(sent).toHaveLength(2);
    expect(r3.blame.length).toBe(a.doc.text.length);
    expect(r1.sessions.length).toBeGreaterThan(0);
    client.dispose();
  });

  it("a job that takes too long ends the worker; the next one starts a new worker with everything", async () => {
    let spawned = 0;
    const hang = (): WorkerLike => {
      spawned++;
      const w: WorkerLike = { onmessage: null, onerror: null, terminate() {}, postMessage() {} };
      queueMicrotask(() => w.onmessage?.({ data: { ready: true } }));
      return w;
    };
    const a = new Device("aa01").type("texte", 0);
    const client = workerAnalyzer(() => (spawned === 0 ? hang() : fakeWorker()), { timeoutMs: 30 });
    await expect(client.analyze(a.doc, undefined)).rejects.toThrow(/too long|timeout/i);
    const again = await client.analyze(a.doc, undefined);
    expect(again.blame.length).toBe("texte".length);
    expect(spawned).toBe(1);
    client.dispose();
  });
});
