import { internalCallback } from "./nativeTaskHooks";

export const FRAME_UNAVAILABLE =
  "Aucun rappel d’image : écran éteint ou verrouillé, ou rendu indisponible. Allumez et déverrouillez l’écran, puis relancez la mesure (frame heartbeat absent).";
export const FOCUS_INTERRUPTED =
  "Mesure interrompue par une perte de focus : autre fenêtre ou dialogue GNOME d’autorisation d’interaction distante/XTest. Autorisez l’interaction si ce dialogue apparaît, puis relancez.";

export interface ClockObservation {
  refreshHz?: number;
  resolutionMs: number;
  deltasMs: number[];
  reads: number;
  roundingRule: "upper:+quantum-per-disjoint-interval";
}

/** No IPC in the input path. Coarse clocks are NOT called fine clocks: every
 * interval retains its full observed quantum as uncertainty in the Go verdict.
 * Four batches bound read cost without pretending a 0ms single read is free.
 */
export class BrowserClock {
  #previous = -1;
  read(): number {
    const value = performance.now();
    if (!Number.isFinite(value) || value < this.#previous)
      throw new Error("browser boundary clock is nonmonotonic or unavailable");
    this.#previous = value;
    return value;
  }
  observe(quantum: number) {
    if (!(quantum > 0 && quantum <= 1))
      throw new Error("browser clock uncertainty cannot certify the 4ms budget");
    const batchSize = 1000;
    const readSpansMs: number[] = [];
    for (let group = 0; group < 4; group++) {
      const start = performance.now();
      for (let i = 0; i < batchSize; i++) this.read();
      readSpansMs.push((performance.now() - start + quantum) / batchSize);
    }
    return { source: "browser-bounded", resolutionMs: quantum, batchSize, readSpansMs };
  }
}

/** Native monotonic timestamps avoid WebKit's 1 ms performance.now coarsening.
 * Synchronous requests bracket real work; transport is included, never deducted.
 * Unique paths prevent caches from returning an old timestamp.
 */
export class NativeClock {
  #sequence = 0;
  #previous = -1;
  #resolution = 0;
  constructor(private readonly nonce: string) {}
  read(): number {
    const request = new XMLHttpRequest();
    request.open("GET", `/__native_perf_clock/${this.nonce}/${++this.#sequence}.json`, false);
    request.send();
    if (request.status !== 200) throw new Error("native monotonic clock unavailable");
    const value = JSON.parse(request.responseText) as { ms: number; resolutionMs: number };
    if (
      !Number.isFinite(value.ms) ||
      value.ms <= this.#previous ||
      !Number.isFinite(value.resolutionMs) ||
      value.resolutionMs <= 0 ||
      value.resolutionMs > 0.01 ||
      (this.#resolution !== 0 && value.resolutionMs !== this.#resolution)
    )
      throw new Error("native monotonic clock lacks adequate, stable resolution");
    this.#previous = value.ms;
    this.#resolution = value.resolutionMs;
    return value.ms;
  }
  observe(quantum = 0) {
    const readSpansMs: number[] = [];
    for (let i = 0; i < 4; i++) {
      const before = performance.now();
      this.read();
      readSpansMs.push(performance.now() - before);
    }
    if (Math.max(...readSpansMs) + quantum > 4)
      throw new Error("native clock preflight transport upper bound exceeds 4ms");
    return { source: "native-monotonic", resolutionMs: this.#resolution, readSpansMs };
  }
}
/** Bounded preflight; never assume that DOMHighResTimeStamp has sub-ms precision. */
export function observeClock(now = () => performance.now()): ClockObservation {
  const first = now();
  let previous = first;
  const deltasMs: number[] = [];
  let reads = 1;
  for (; reads < 100000 && deltasMs.length < 8; reads++) {
    const current = now();
    if (current > previous) deltasMs.push(current - previous);
    previous = current;
    if (current - first >= 8) break;
  }
  return {
    resolutionMs: deltasMs.length >= 3 ? Math.min(...deltasMs) : 0,
    deltasMs,
    reads,
    roundingRule: "upper:+quantum-per-disjoint-interval",
  };
}

/** Next task after the rAF callback; a work-window boundary, not a paint claim. */
export class FrameTasks {
  #channel = new MessageChannel();
  #next = 0;
  #tasks = new Map<number, () => void>();
  constructor() {
    this.#channel.port1.onmessage = internalCallback((event: MessageEvent<number>) => {
      const task = this.#tasks.get(event.data);
      this.#tasks.delete(event.data);
      task?.();
    });
  }
  post(task: () => void): void {
    const id = ++this.#next;
    this.#tasks.set(id, task);
    this.#channel.port2.postMessage(id);
  }
  close(): void {
    this.#tasks.clear();
    this.#channel.port1.close();
    this.#channel.port2.close();
  }
}
