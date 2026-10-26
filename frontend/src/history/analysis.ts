import { next as A } from "@automerge/automerge";
import { SLOW_CHARS } from "../store/limits";
import {
  claimsOf,
  type DeviceClaim,
  pastOf,
  personOfMine,
  type Session,
  sessionsBefore,
  sessionsOf,
  stepsOf,
} from "./log";
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
  /** What each device says about itself, read from its own changes (see {@link claimsOf}). */
  claims: Record<string, DeviceClaim>;
  /** The document is too big to follow who wrote what (see {@link BLAME_BUDGET}): no blame, no marks. */
  limited: boolean;
}

/** What is sent back by a worker: the sessions are in `Analysis`, so the blame leaves them out. */
export type Wire = Omit<Analysis, "blame"> & { blame: Omit<Blame, "sessions"> };

const CAP = 20_000;

/**
 * Following who wrote what costs one diff per session, and a diff walks every operation of the
 * document (about 150 ns each, deleted text included). Past this many operations times sessions
 * (about 15 s of work), only the sessions are listed: the page stays light, and says so.
 */
export const BLAME_BUDGET = 100_000_000;

/**
 * A text past the "slow" tier of the size gate (ADR 0028 D1: 512 Ki characters, where the notice
 * of slowness starts) is not analysed at all: copying such a document into a worker and following
 * it costs hundreds of megabytes (Automerge keeps about 128 bytes per character), and the
 * history of a text that big is read with the disk file, not with this panel.
 */
export const HUGE_TEXT = SLOW_CHARS;

/** What the screens get for a document too big to follow: no entries, no marks, and a note. */
const tooBig = (doc: A.Doc<HDoc>): Analysis => ({
  sessions: [],
  order: [],
  blame: { spans: [], deletions: [], sessions: [], length: doc.text.length },
  sinceIdx: 0,
  claims: {},
  limited: true,
});

/** Whether following who wrote what would cost more than the budget. */
export const tooCostly = (ops: number, sessions: number): boolean => ops * sessions > BLAME_BUDGET;

/** Runs the analysis on `doc`. `since`: the heads of the last visit, or undefined for none. */
export async function analyze(
  doc: A.Doc<HDoc>,
  tracker: Tracker,
  since: Heads | undefined,
  mine?: ReadonlySet<string>,
  opts?: TrackOptions,
): Promise<Analysis> {
  const steps = stepsOf(doc);
  // A session never spans the last visit: what came after it is told apart from what came before.
  const past = since ? pastOf(steps, since) : undefined;
  const claims = claimsOf(steps);
  const sessions = sessionsOf(steps, Date.now(), personOfMine(mine), past);
  const ops = steps.at(-1)?.maxOp ?? 0;
  const limited = tooCostly(ops, sessions.length);
  const blame: Blame = limited
    ? { spans: [], deletions: [], sessions, length: doc.text.length }
    : await tracker.update(doc, sessions, opts);
  // A removed text this long is not worth carrying: only a part of it is kept for matching.
  for (const d of blame.deletions) if (d.text.length > CAP) d.text = d.text.slice(0, CAP);
  return {
    sessions,
    order: steps.map((s) => s.hash),
    blame,
    sinceIdx: since ? sessionsBefore(sessions, steps, since) : sessions.length,
    claims,
    limited,
  };
}

/** A request replaced by a newer one before it started: nobody waits for it any more. */
export class Superseded extends Error {
  constructor() {
    super("superseded");
    this.name = "Superseded";
  }
}

/** The time a job may take before its worker is ended. */
const JOB_MS = 30_000;

/** Computes analyses, on this thread or in a worker; the host never has to know which. */
export interface Analyzer {
  analyze(
    doc: A.Doc<HDoc>,
    since: Heads | undefined,
    mine?: ReadonlySet<string>,
  ): Promise<Analysis>;
  dispose(): void;
}

/** On the calling thread: for tests and small documents. */
export function inlineAnalyzer(): Analyzer {
  const tracker = new Tracker();
  return {
    analyze: (doc, since, mine) =>
      doc.text.length > HUGE_TEXT
        ? Promise.resolve(tooBig(doc))
        : analyze(doc, tracker, since, mine),
    dispose() {},
  };
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
export function workerAnalyzer(
  spawn: () => WorkerLike,
  { timeoutMs = JOB_MS }: { timeoutMs?: number } = {},
): Analyzer {
  let worker: WorkerLike | undefined;
  /** Resolves when the worker says it listens: a module worker only starts after its wasm loads. */
  let ready: Promise<void> = Promise.resolve();
  let sent: Heads | undefined;
  let seq = 0;
  interface Job {
    doc: A.Doc<HDoc>;
    since: Heads | undefined;
    mine: string[];
    resolve: (a: Analysis) => void;
    reject: (e: unknown) => void;
  }
  // One job runs, one waits: a newer request takes the place of the one waiting (it is stale).
  let running: (Job & { id: number; timer: ReturnType<typeof setTimeout> }) | undefined;
  let pending: Job | undefined;

  const stop = (reason: unknown) => {
    worker?.terminate();
    worker = undefined;
    sent = undefined;
    if (running) {
      clearTimeout(running.timer);
      running.reject(reason);
      running = undefined;
    }
  };

  function next(): void {
    if (running || !pending) return;
    const job = pending;
    pending = undefined;
    const w = boot();
    const id = ++seq;
    const timer = setTimeout(() => {
      // Too long: the worker is ended (and made again for the next job), not waited for.
      stop(new Error("analysis took too long"));
      next();
    }, timeoutMs);
    running = { ...job, id, timer };
    void ready.then(() => {
      if (worker !== w || running?.id !== id) return;
      const changes = sent ? A.getChangesSince(job.doc, sent) : A.getAllChanges(job.doc);
      sent = A.getHeads(job.doc);
      w.postMessage({ id, changes, since: job.since, mine: job.mine });
    });
  }

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
      const job = running;
      if (!job || job.id !== m.id) return;
      clearTimeout(job.timer);
      running = undefined;
      if (m.result) {
        const { blame, ...rest } = m.result;
        job.resolve({ ...rest, blame: { ...blame, sessions: rest.sessions } });
      } else job.reject(new Error(m.error ?? "analysis failed"));
      next();
    };
    w.onerror = () => {
      stop(new Error("analysis worker failed"));
      next();
    };
    worker = w;
    return w;
  }

  return {
    analyze(doc, since, mine) {
      if (doc.text.length > HUGE_TEXT) return Promise.resolve(tooBig(doc));
      return new Promise<Analysis>((resolve, reject) => {
        pending?.reject(new Superseded());
        pending = { doc, since, mine: [...(mine ?? [])], resolve, reject };
        next();
      });
    },
    dispose() {
      pending?.reject(new Error("disposed"));
      pending = undefined;
      stop(new Error("disposed"));
    },
  };
}
