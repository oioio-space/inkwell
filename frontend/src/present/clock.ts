/** The talk's timer: starts at the first advance, can be paused and reset. The clock is given, so tests control it. */
export class Stopwatch {
  readonly #now: () => number;
  #since: number | undefined;
  #before = 0;
  #everStarted = false;

  constructor(now: () => number = Date.now) {
    this.#now = now;
  }

  get running(): boolean {
    return this.#since !== undefined;
  }

  /** True once started, even if paused: the first advance starts the timer only the first time. */
  get started(): boolean {
    return this.#everStarted;
  }

  start(): void {
    if (this.#since !== undefined) return;
    this.#since = this.#now();
    this.#everStarted = true;
  }

  /** Starts the timer if it never was: what the first advance of the talk does. */
  startOnce(): void {
    if (!this.#everStarted) this.start();
  }

  pause(): void {
    if (this.#since === undefined) return;
    this.#before += this.#now() - this.#since;
    this.#since = undefined;
  }

  reset(): void {
    this.#since = undefined;
    this.#before = 0;
    this.#everStarted = false;
  }

  elapsedMs(): number {
    return this.#before + (this.#since === undefined ? 0 : this.#now() - this.#since);
  }
}

export type Pace = { kind: "onTime" } | { kind: "ahead" | "behind"; minutes: number };

/** How far a talk is from its plan. Under half a minute of difference is on time. */
const TOLERANCE_MS = 30_000;

/**
 * Compares the time spent with the time the plan gives for the slides before this one: the
 * target is spread evenly over the slides. Undefined when there is no target.
 */
export function pace(
  elapsedMs: number,
  targetMinutes: number,
  index: number,
  count: number,
): Pace | undefined {
  if (targetMinutes <= 0 || count <= 0 || index < 1) return undefined;
  const planned = (targetMinutes * 60_000 * (index - 1)) / count;
  const gap = elapsedMs - planned;
  if (Math.abs(gap) < TOLERANCE_MS) return { kind: "onTime" };
  return {
    kind: gap > 0 ? "behind" : "ahead",
    minutes: Math.max(1, Math.round(Math.abs(gap) / 60_000)),
  };
}

/** 83 seconds as "1:23"; an hour or more as "1:02:03". */
export function formatElapsed(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const s = total % 60;
  const m = Math.floor(total / 60) % 60;
  const h = Math.floor(total / 3600);
  const two = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${two(m)}:${two(s)}` : `${m}:${two(s)}`;
}
