/** Page-side participants in the desktop close barrier. The Go coordinator owns the attempt. */

const buffers = new Set<() => void | Promise<void>>();
const publishers = new Set<{ pause(): void; resume(): void }>();
let paused = false;

export function closeBuffer(flush: () => void | Promise<void>): () => void {
  buffers.add(flush);
  return () => buffers.delete(flush);
}

export async function flushCloseBuffers(): Promise<void> {
  for (const flush of buffers) await flush();
}

export function closePublisher(pause: () => void, resume: () => void): () => void {
  const publisher = { pause, resume };
  publishers.add(publisher);
  if (paused) pause();
  return () => publishers.delete(publisher);
}

export function pauseClosePresence(): void {
  paused = true;
  for (const publisher of publishers) publisher.pause();
}

export function resumeClosePresence(): void {
  if (!paused) return;
  paused = false;
  for (const publisher of publishers) publisher.resume();
}

export interface CloseControl {
  close: "ready" | "prepare" | "abort" | "project-approved" | "project-denied";
  attempt: number;
}

export interface CloseReply {
  close: "empty" | "saved" | "complete" | "failed" | "project";
  attempt: number;
  reason?: string;
  files?: string[];
}

export function readCloseControl(data: Uint8Array | string): CloseControl | undefined {
  try {
    const frame = JSON.parse(typeof data === "string" ? data : new TextDecoder().decode(data));
    if (frame?.close === "ready") return { close: "ready", attempt: 0 };
    if (
      ["prepare", "abort", "project-approved", "project-denied"].includes(frame?.close) &&
      Number.isInteger(frame.attempt) &&
      frame.attempt > 0 &&
      frame.attempt <= 0xffffffff
    )
      return frame as CloseControl;
  } catch {
    // Malformed frames are ignored as for any other open-stream message.
  }
  return undefined;
}

export interface ClosePageTarget {
  save(signal: AbortSignal): Promise<boolean>;
  goodbye(attempt: number, signal: AbortSignal): Promise<void>;
  restore(): void;
  describe(error: unknown): Pick<CloseReply, "reason" | "files">;
}

export class ClosePage {
  #hold: { attempt: number; abort: AbortController } | undefined;
  #send: ((reply: CloseReply) => void) | undefined;
  #project: ((approved: boolean) => void) | undefined;
  #leaving = false;
  #receivedReady: () => void = () => {};
  readonly ready = new Promise<void>((resolve) => {
    this.#receivedReady = resolve;
  });

  constructor(private readonly target: ClosePageTarget) {}

  restore(): void {
    if (!this.#hold) return;
    this.#hold.abort.abort();
    this.#hold = undefined;
    this.#leaving = false;
    this.target.restore();
  }

  disconnect(): void {
    this.#send = undefined;
    this.#project?.(false);
    this.#project = undefined;
    if (!this.#leaving) this.restore();
  }

  requestProject(): Promise<boolean> | undefined {
    const send = this.#send;
    if (!send) return;
    if (this.#project) return Promise.resolve(false);
    return new Promise<boolean>((resolve) => {
      this.#project = resolve;
      try {
        send({ close: "project", attempt: 0 });
      } catch {
        this.#project = undefined;
        resolve(false);
      }
    });
  }

  receive(frame: CloseControl, send: (reply: CloseReply) => void): void {
    const { attempt } = frame;
    if (frame.close === "ready") {
      this.#send = send;
      this.#receivedReady();
      return;
    }
    if (frame.close === "project-approved" || frame.close === "project-denied") {
      if (!this.#project) return;
      this.#leaving = frame.close === "project-approved" && this.#hold?.attempt === attempt;
      this.#project(this.#leaving);
      this.#project = undefined;
      return;
    }
    if (frame.close === "abort") {
      if (this.#hold?.attempt === attempt) this.restore();
      return;
    }
    if (this.#hold?.attempt === attempt) return;
    this.restore();
    const abort = new AbortController();
    this.#hold = { attempt, abort };
    const signal = abort.signal;
    void (async () => {
      try {
        const saved = await this.target.save(signal);
        signal.throwIfAborted();
        if (!saved) {
          send({ close: "empty", attempt });
          return;
        }
        send({ close: "saved", attempt });
        await this.target.goodbye(attempt, signal);
        signal.throwIfAborted();
        send({ close: "complete", attempt });
      } catch (error) {
        if (!signal.aborted) send({ close: "failed", attempt, ...this.target.describe(error) });
      }
    })();
  }
}
