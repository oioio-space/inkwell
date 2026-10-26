import { next as A } from "@automerge/automerge";
import { personOfDoc, type Session, sessionsBefore, sessionsOf, stepsOf } from "./log";
import { type Blame, Tracker, type TrackOptions } from "./track";
import type { HDoc, Heads } from "./types";

/**
 * Everything the history screens need that costs time on a large document: the sessions, who
 * wrote each stretch of the text, and where the last visit stands among the sessions. All of it
 * comes from one place (the same order of changes), so an entry id means the same thing everywhere.
 */
export interface Analysis {
  sessions: Session[];
  /** The hash of each change, in the order the sessions were cut from (for the graph). */
  order: string[];
  blame: Blame;
  /** How many sessions, from the oldest, were already there at the last visit. */
  sinceIdx: number;
}

/** What is sent back by a worker: the sessions are in `Analysis`, so the blame leaves them out. */
export type Wire = Omit<Analysis, "blame"> & { blame: Omit<Blame, "sessions"> };

const CAP = 20_000;

/** Runs the analysis on `doc`. `since`: the heads of the last visit, or undefined for none. */
export async function analyze(
  doc: A.Doc<HDoc>,
  tracker: Tracker,
  since: Heads | undefined,
  opts?: TrackOptions,
): Promise<Analysis> {
  const steps = stepsOf(doc);
  const sessions = sessionsOf(steps, Date.now(), personOfDoc(doc));
  const blame = await tracker.update(doc, sessions, opts);
  // A removed text this long is not worth carrying: only a part of it is kept for matching.
  for (const d of blame.deletions) if (d.text.length > CAP) d.text = d.text.slice(0, CAP);
  return {
    sessions,
    order: steps.map((s) => s.hash),
    blame,
    sinceIdx: since ? sessionsBefore(sessions, steps, since) : sessions.length,
  };
}

/** Computes analyses, on this thread or in a worker; the host never has to know which. */
export interface Analyzer {
  analyze(doc: A.Doc<HDoc>, since: Heads | undefined): Promise<Analysis>;
  dispose(): void;
}

/** On the calling thread: for tests and small documents. */
export function inlineAnalyzer(): Analyzer {
  const tracker = new Tracker();
  return { analyze: (doc, since) => analyze(doc, tracker, since), dispose() {} };
}

/** The slice of the Worker interface the client needs. */
export interface WorkerLike {
  postMessage(message: unknown): void;
  terminate(): void;
  onmessage: ((e: { data: unknown }) => void) | null;
  onerror: ((e: unknown) => void) | null;
}

/**
 * Analyses in a worker (DIF-05: a document of several megabytes never freezes the page). The
 * worker keeps its own copy of the document: it receives all changes once, then only new ones.
 * A newer request replaces the one waiting; the worker's answer to an old one is dropped.
 */
export function workerAnalyzer(spawn: () => WorkerLike): Analyzer {
  let worker: WorkerLike | undefined;
  /** Resolves when the worker says it listens: a module worker only starts after its wasm loads. */
  let ready: Promise<void> = Promise.resolve();
  let sent: Heads | undefined;
  let seq = 0;
  const waiting = new Map<
    number,
    { resolve: (a: Analysis) => void; reject: (e: unknown) => void }
  >();

  const kill = (reason: unknown) => {
    worker?.terminate();
    worker = undefined;
    sent = undefined;
    for (const w of waiting.values()) w.reject(reason);
    waiting.clear();
  };

  function boot(): WorkerLike {
    if (worker) return worker;
    const w = spawn();
    ready = new Promise<void>((resolve) => {
      w.onmessage = (e) => {
        if ((e.data as { ready?: boolean } | null)?.ready) {
          resolve();
          w.onmessage = receive;
        }
      };
    });
    const receive = (e: { data: unknown }) => {
      const m = e.data as { id: number; result?: Wire; error?: string };
      const job = waiting.get(m.id);
      if (!job) return;
      waiting.delete(m.id);
      if (m.result) {
        const { blame, ...rest } = m.result;
        job.resolve({ ...rest, blame: { ...blame, sessions: rest.sessions } });
      } else job.reject(new Error(m.error ?? "analysis failed"));
    };
    w.onerror = () => kill(new Error("analysis worker failed"));
    worker = w;
    return w;
  }

  return {
    analyze(doc, since) {
      const w = boot();
      const id = ++seq;
      // Only the latest request matters: the earlier ones are dropped by the caller's own order.
      return new Promise<Analysis>((resolve, reject) => {
        waiting.set(id, { resolve, reject });
        void ready.then(() => {
          if (worker !== w) return;
          const changes = sent ? A.getChangesSince(doc, sent) : A.getAllChanges(doc);
          sent = A.getHeads(doc);
          w.postMessage({ id, changes, since });
        });
      });
    },
    dispose: () => kill(new Error("disposed")),
  };
}
