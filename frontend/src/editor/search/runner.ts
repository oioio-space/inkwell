import type { Matcher } from "./query";
import { type Match, type Range, scanSync } from "./scan";

export interface SearchJob {
  text: string;
  matcher: Matcher;
  range: Range;
  limit: number;
}

/** Runs searches. The default one uses a Web Worker; tests inject their own. */
export interface SearchRunner {
  run(job: SearchJob): Promise<Match[]>;
  dispose?(): void;
}

/** The search ran past its time limit and was stopped. */
export class SearchTooCostly extends Error {
  constructor() {
    super("Recherche trop coûteuse : simplifiez l'expression.");
    this.name = "SearchTooCostly";
  }
}

/** Searches on the calling thread, with no time limit. For tests and plain queries. */
export const inlineRunner: SearchRunner = {
  run: (job) => Promise.resolve(scanSync(job.text, job.matcher, job.range, job.limit)),
};

/** The slice of the Worker interface the runner needs. */
export interface WorkerLike {
  postMessage(message: unknown): void;
  terminate(): void;
  onmessage: ((e: { data: unknown }) => void) | null;
  onerror: ((e: unknown) => void) | null;
}

export interface WorkerRunnerOptions {
  /** Time a job may take once the worker is up (SRC-07). */
  timeoutMs?: number;
  /** Time the worker may take to start. */
  startMs?: number;
}

/**
 * Runs jobs one at a time in a worker made by `spawn`. A job that outlives `timeoutMs` kills
 * the worker (the only way to stop a runaway regex) and fails with SearchTooCostly; the next
 * job starts a fresh worker. The clock starts when the worker is up, so loading it never counts.
 */
export function createWorkerRunner(
  spawn: () => WorkerLike,
  { timeoutMs = 200, startMs = 5000 }: WorkerRunnerOptions = {},
): SearchRunner {
  let worker: WorkerLike | undefined;
  let ready: Promise<void> = Promise.resolve();
  let seq = 0;
  let queue: Promise<unknown> = Promise.resolve();

  const kill = () => {
    worker?.terminate();
    worker = undefined;
  };

  function boot(): WorkerLike {
    if (worker) return worker;
    const w = spawn();
    worker = w;
    ready = new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        if (worker === w) kill();
        reject(new Error("Le moteur de recherche ne démarre pas."));
      }, startMs);
      w.onmessage = (e) => {
        if ((e.data as { ready?: boolean } | null)?.ready) {
          clearTimeout(timer);
          resolve();
        }
      };
      w.onerror = () => {
        clearTimeout(timer);
        if (worker === w) kill();
        reject(new Error("Le moteur de recherche a échoué."));
      };
    });
    ready.catch(() => {});
    return w;
  }

  async function exec(job: SearchJob): Promise<Match[]> {
    const w = boot();
    await ready;
    return new Promise<Match[]>((resolve, reject) => {
      const id = ++seq;
      const timer = setTimeout(() => {
        if (worker === w) kill();
        reject(new SearchTooCostly());
      }, timeoutMs);
      w.onmessage = (e) => {
        const d = e.data as { id?: number; matches?: Match[]; error?: string };
        if (d.id !== id) return;
        clearTimeout(timer);
        if (d.error !== undefined) reject(new Error(d.error));
        else resolve(d.matches ?? []);
      };
      w.onerror = () => {
        clearTimeout(timer);
        if (worker === w) kill();
        reject(new Error("Le moteur de recherche a échoué."));
      };
      w.postMessage({ id, ...job });
    });
  }

  return {
    run(job) {
      const next = queue.then(
        () => exec(job),
        () => exec(job),
      );
      queue = next;
      return next;
    },
    dispose: kill,
  };
}

let shared: SearchRunner | undefined;

/** The runner everything uses unless told otherwise: a worker when the platform has one. */
export function defaultRunner(): SearchRunner {
  shared ??=
    typeof Worker === "undefined"
      ? inlineRunner
      : createWorkerRunner(
          () =>
            new Worker(new URL("./worker.ts", import.meta.url), {
              type: "module",
            }) as unknown as WorkerLike,
        );
  return shared;
}

/** Replaces the default runner (tests, or a host with its own worker pool). */
export function setDefaultRunner(runner: SearchRunner | undefined): void {
  shared = runner;
}
