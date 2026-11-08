import { installNativeTaskHooks, internalCallback, type NativeTaskSpan } from "./nativeTaskHooks";
import { FrameTasks } from "./nativeTiming";

export interface EarlyFrame {
  start: number;
  end: number;
  timestamp: number;
}
export interface EarlySink {
  capture(event: Event): void;
  bubble(event: Event): void;
  frame(frame: EarlyFrame): void;
  frameEnd(frame: EarlyFrame): void;
  task(span: NativeTaskSpan): void;
}
let installed = false;
let beforeApp = false;
let sink: EarlySink | undefined;
let earlierWork: (() => void) | undefined;
let stopLoop: (() => void) | undefined;

/** Called by main BEFORE importing App. Hooks are inert until Go authorizes a sink. */
export function installNativeEarly(preApp = false): void {
  if (installed) return;
  installed = true;
  beforeApp = preApp;
  const tasks = new FrameTasks();
  const rawRaf = requestAnimationFrame;
  const undo = installNativeTaskHooks((span) => sink?.task(span));
  const capture = internalCallback((event: Event) => sink?.capture(event));
  const bubble = internalCallback((event: Event) => sink?.bubble(event));
  for (const name of ["keydown", "beforeinput", "input"]) {
    window.addEventListener(name, capture, true);
    window.addEventListener(name, bubble);
  }
  let first = 0;
  let frame = 0;
  // This slot precedes even the early measuring rAF. Its control must be
  // detected independently by the task probe and cadence oracle.
  const beforeFrame = () => {
    first = rawRaf(beforeFrame);
    const work = earlierWork;
    earlierWork = undefined;
    work?.();
  };
  const tick = (timestamp: number) => {
    frame = rawRaf(tick); // Stay ahead of App's next callbacks.
    const record = { start: performance.now(), end: 0, timestamp };
    sink?.frame(record);
    tasks.post(() => {
      record.end = performance.now();
      sink?.frameEnd(record);
    });
  };
  first = rawRaf(beforeFrame);
  frame = rawRaf(tick);
  stopLoop = () => {
    cancelAnimationFrame(first);
    cancelAnimationFrame(frame);
    tasks.close();
    for (const name of ["keydown", "beforeinput", "input"]) {
      window.removeEventListener(name, capture, true);
      window.removeEventListener(name, bubble);
    }
    undo();
    installed = false;
    beforeApp = false;
    sink = undefined;
    earlierWork = undefined;
  };
}
export function subscribeNativeEarly(value: EarlySink): { beforeApp: boolean; close(): void } {
  installNativeEarly();
  if (sink) throw new Error("native early recorder already attached");
  sink = value;
  return {
    beforeApp,
    close() {
      if (sink === value) {
        sink = undefined;
        earlierWork = undefined;
      }
    },
  };
}
export function scheduleEarlierFrame(work: () => void): void {
  earlierWork = work;
}
export function closeNativeEarly(): void {
  stopLoop?.();
  stopLoop = undefined;
}

/** Continuous task-queue upper bounds, including timer scheduling gaps.
 * One pending message, with an 8ms timer yield; no tight spin loop or blind gap.
 * All yielded/scheduling time remains in the recorded upper bound.
 */
export class BusyProbe {
  #tasks = new FrameTasks();
  #timer: ReturnType<typeof setTimeout> | undefined;
  #closed = false;
  #last = performance.now();
  constructor(private readonly record: (span: { start: number; end: number }) => void) {
    this.#post();
  }
  #post(): void {
    this.#tasks.post(() => {
      if (this.#closed) return;
      const end = performance.now();
      this.record({ start: this.#last, end });
      this.#last = end;
      this.#timer = setTimeout(
        internalCallback(() => this.#post()),
        8,
      );
    });
  }
  boundary(): { start: number; end: number } {
    return { start: this.#last, end: performance.now() };
  }
  close(): void {
    this.#closed = true;
    clearTimeout(this.#timer);
    this.#tasks.close();
  }
}
