/**
 * Active time (rule T-2 of plan docs/plans/2026-11-24-obs1-observability.md): each edit of the
 * person stamps a time; the time between two edits counts unless it is over a minute, which is
 * idle.
 */

/** A gap without an edit longer than this is idle. */
export const IDLE_MS = 60_000;

export class ActiveTime {
  #last: number | undefined;
  #active = 0;

  /** An edit at `now` (milliseconds). */
  stamp(now: number): void {
    if (this.#last !== undefined) {
      const gap = now - this.#last;
      if (gap > 0 && gap <= IDLE_MS) this.#active += gap;
    }
    this.#last = now;
  }

  /** When the last edit was, if there was one. */
  get last(): number | undefined {
    return this.#last;
  }

  /** The active milliseconds since the last take. */
  take(): number {
    const active = this.#active;
    this.#active = 0;
    return active;
  }
}
