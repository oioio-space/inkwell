import type { DocHandle } from "@automerge/automerge-repo";

/** Where the projection goes: the project's `.typ` file, written by Go. */
export interface ProjectionSink {
  writeProjection(name: string, text: string): Promise<void>;
}

export interface ProjectionOptions {
  /** The project file to write. */
  name?: string;
  /** Quiet time after the last change before writing. */
  delayMs?: number;
  /**
   * Called when a write fails, for instance because the file was edited outside Inkwell. The
   * projection keeps going and writes again at the next change or {@link Projection.flush}.
   */
  onError?: (err: unknown) => void;
}

export interface Projection {
  /**
   * Writes the current text now, unless it is what was last written. With `force`, writes it
   * even then: the file may have changed on disk since.
   */
  flush(force?: boolean): Promise<void>;
  /** Stops following the document. */
  stop(): void;
}

/** What the debounce waits after the last keystroke (UX: the file follows the text, never leads). */
export const PROJECTION_DELAY_MS = 300;

/**
 * Keeps a `.typ` file in the project folder equal to the document's text: written once at
 * start, then {@link PROJECTION_DELAY_MS} after the last change, one write at a time.
 * Automerge stays the single source of truth; the file is a projection.
 */
export function startProjection(
  handle: DocHandle<{ text: string }>,
  sink: ProjectionSink,
  opts: ProjectionOptions = {},
): Projection {
  const name = opts.name ?? "main.typ";
  const delay = opts.delayMs ?? PROJECTION_DELAY_MS;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let running: Promise<void> | undefined;
  let again = false;
  let last: string | undefined;

  // Runs until no flush is pending. It yields first, so `running` is set before it can end.
  const drain = async (): Promise<void> => {
    await null;
    try {
      while (again) {
        again = false;
        const text = handle.doc()?.text ?? "";
        if (text === last) continue;
        try {
          await sink.writeProjection(name, text);
          last = text;
        } catch (err) {
          opts.onError?.(err);
          return;
        }
      }
    } finally {
      running = undefined;
    }
  };

  const flush = (force = false): Promise<void> => {
    clearTimeout(timer);
    if (force) last = undefined;
    again = true;
    running ??= drain();
    return running;
  };

  const onChange = () => {
    clearTimeout(timer);
    timer = setTimeout(() => void flush(), delay);
  };
  handle.on("change", onChange);
  void flush();

  return {
    flush,
    stop() {
      clearTimeout(timer);
      handle.off("change", onChange);
    },
  };
}
