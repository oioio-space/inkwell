import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { encodeSyncPoint } from "./syncPoint";

/** Where the projection goes: the project's `.typ` file, written by Go. */
export interface ProjectionSink {
  /**
   * Writes the file. `point` says where the document and the file now agree (see ./syncPoint.ts);
   * Go keeps it with the hash of what it wrote, so the two always describe the same bytes.
   */
  writeProjection(name: string, text: string, point?: string): Promise<void>;
  /**
   * Records `point` for a file that was not written: at open, in the Ctrl+S mode, when the
   * document equals the file. Go refuses it if the file changed since.
   */
  setSyncPoint?(name: string, point: string): Promise<void>;
}

/** What a successful write tells: the file now holds `text`, which the document had at `heads`. */
export interface Written {
  text: string;
  heads: A.Heads;
}

export interface ProjectionOptions {
  /**
   * The project file to write; a function when the file can be renamed, asked again at each
   * write.
   */
  name?: string | (() => string);
  /** Quiet time after the last change before writing. */
  delayMs?: number;
  /**
   * How long to wait after a change before writing, or undefined to write only when asked
   * ({@link Projection.flush}): the "save with Ctrl+S" mode. Read at each change, so the setting
   * can change while the project is open. Default: `delayMs`.
   */
  schedule?: () => number | undefined;
  /** What the file is known to hold at start, when it is not written then (save with Ctrl+S). */
  known?: string | undefined;
  /**
   * Called when a write fails, for instance because the file was edited outside Inkwell. The
   * projection keeps going and writes again at the next change or {@link Projection.flush}.
   */
  onError?: (err: unknown) => void;
  /**
   * Called after each successful write, so a failure shown to the person can be taken back. It
   * says which text the file now holds and the heads the document had then.
   */
  onWritten?: (written: Written) => void;
  /** Called when the document starts or stops differing from the file. */
  onDirty?: (dirty: boolean) => void;
  /**
   * Whether a failed write is worth trying again by itself (a disk that was full, a file that was
   * locked) rather than waiting for the next change. Default: yes. Not for a file edited
   * outside, which stays refused until the person decides.
   */
  retryable?: (err: unknown) => boolean;
  /** Wait before trying a failed write again. */
  retryMs?: number;
}

export interface Projection {
  /**
   * Writes the current text now, unless it is what was last written. With `force`, writes it
   * even then: the file may have changed on disk since.
   */
  flush(force?: boolean): Promise<void>;
  /** True while the document differs from what the file holds. */
  dirty(): boolean;
  /**
   * The save settings changed: a document that differs from the file is written at once if
   * writing is now automatic; otherwise a write that was waiting is dropped or follows the new delay.
   */
  reschedule(): void;
  /** The file now holds `text` (an outside edit that was merged): it is no longer to be written. */
  adopt(text: string): void;
  /**
   * While held, nothing is written unless forced: the file was deleted outside and the person has
   * not decided whether it comes back.
   */
  hold(on: boolean): void;
  /** Stops following the document. */
  stop(): void;
}

/** How long a failed write waits before it is tried again. */
export const PROJECTION_RETRY_MS = 5000;

/** What the debounce waits after the last keystroke (UX: the file follows the text, never leads). */
export const PROJECTION_DELAY_MS = 300;

/**
 * Keeps a `.typ` file in the project folder equal to the document's text: written once at
 * start, then {@link PROJECTION_DELAY_MS} after the last change, one write at a time. With
 * `schedule` returning undefined, it writes only when asked. Automerge stays the single source of
 * truth; the file is a projection.
 */
export function startProjection(
  handle: DocHandle<{ text: string }>,
  sink: ProjectionSink,
  opts: ProjectionOptions = {},
): Projection {
  const given = opts.name;
  const nameOf = typeof given === "function" ? given : () => given ?? "main.typ";
  const delay = opts.delayMs ?? PROJECTION_DELAY_MS;
  const schedule = opts.schedule ?? (() => delay);
  const retryMs = opts.retryMs ?? PROJECTION_RETRY_MS;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let running: Promise<void> | undefined;
  let again = false;
  let last: string | undefined = opts.known;
  let held = false;
  let wasDirty = false;

  const isDirty = () => (handle.doc()?.text ?? "") !== (last ?? "");
  const emitDirty = () => {
    const now = isDirty();
    if (now === wasDirty) return;
    wasDirty = now;
    opts.onDirty?.(now);
  };

  // Runs until no flush is pending. It yields first, so `running` is set before it can end.
  const drain = async (): Promise<void> => {
    await null;
    try {
      while (again) {
        again = false;
        const doc = handle.doc();
        const text = doc?.text ?? "";
        if (text === last) continue;
        const heads = doc ? A.getHeads(doc) : [];
        try {
          await sink.writeProjection(nameOf(), text, encodeSyncPoint(heads, text));
          last = text;
          opts.onWritten?.({ text, heads });
          emitDirty();
        } catch (err) {
          opts.onError?.(err);
          if (opts.retryable?.(err) ?? true) {
            clearTimeout(timer);
            timer = setTimeout(() => void flush(), retryMs);
          }
          return;
        }
      }
    } finally {
      running = undefined;
    }
  };

  const flush = (force = false): Promise<void> => {
    clearTimeout(timer);
    if (held && !force) return Promise.resolve();
    if (force) last = undefined;
    again = true;
    running ??= drain();
    return running;
  };

  const arm = () => {
    clearTimeout(timer);
    const wait = schedule();
    if (wait !== undefined) timer = setTimeout(() => void flush(), wait);
  };
  const onChange = () => {
    arm();
    emitDirty();
  };
  handle.on("change", onChange);
  if (schedule() === undefined) {
    // Nothing is written at open: the file keeps what it holds until the person saves. If the
    // document is the file, that is where they agree; an outside edit is later merged from there.
    const doc = handle.doc();
    if (doc && last !== undefined && doc.text === last) {
      void sink.setSyncPoint?.(nameOf(), encodeSyncPoint(A.getHeads(doc), last))?.catch(() => {
        // The file changed since it was read: the outside edit is dealt with when it is notified.
      });
    }
    emitDirty();
  } else {
    void flush();
  }

  return {
    flush,
    dirty: isDirty,
    reschedule() {
      if (schedule() !== undefined && isDirty()) void flush();
      else arm();
    },
    adopt(text) {
      last = text;
      emitDirty();
    },
    hold(on) {
      held = on;
    },
    stop() {
      clearTimeout(timer);
      handle.off("change", onChange);
    },
  };
}
