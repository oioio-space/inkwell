import { type Dialect, detectDialect } from "./dialect";
import { CsvModel } from "./model";
import { papaIndex } from "./papaIndex";
import { RowIndex } from "./rowIndex";

/** Below this size the first parse is done in place: a worker costs more than it saves. */
const WORKER_FROM = 1_000_000;
/** A parse that takes longer than this is reported as failed instead of freezing anything. */
const TIMEOUT_MS = 60_000;

export interface OpenOptions {
  dialect?: Dialect;
  /** 0..1 while the file is read. */
  onProgress?: (done: number) => void;
  /** Called once with a usable model made of the first records, before the parse finishes. */
  onFirst?: (model: CsvModel) => void;
  timeoutMs?: number;
}

/**
 * Opens a CSV text as a model. Big texts are indexed by Papa Parse in a Worker, with
 * progress and a first usable screen; the text itself never leaves the main thread's memory
 * twice (the worker gets a copy only for the duration of the parse).
 */
export async function openCsv(text: string, opts: OpenOptions = {}): Promise<CsvModel> {
  const dialect = opts.dialect ?? detectDialect(text);
  if (dialect.quote === false)
    return new CsvModel(text, dialect, RowIndex.scan(text, dialect.delimiter, false));
  if (text.length < WORKER_FROM || typeof Worker === "undefined") {
    const p = papaIndex(text, dialect.delimiter, opts.onProgress);
    return new CsvModel(
      text,
      dialect,
      new RowIndex(p.starts, p.cols, text.length, dialect.delimiter),
    );
  }
  const worker = new Worker(new URL("./parse.worker.ts", import.meta.url), { type: "module" });
  return new Promise<CsvModel>((resolve, reject) => {
    const timer = setTimeout(() => {
      worker.terminate();
      reject(new Error("timeout"));
    }, opts.timeoutMs ?? TIMEOUT_MS);
    worker.onmessage = (e: MessageEvent) => {
      const m = e.data as
        | { type: "progress"; done: number }
        | { type: "first"; starts: Int32Array; cols: Uint16Array }
        | { type: "done"; starts: Int32Array; cols: Uint16Array }
        | { type: "error"; message: string };
      if (m.type === "progress") opts.onProgress?.(m.done);
      else if (m.type === "first") {
        // The first records are enough to show a grid while the rest is read.
        const n = m.starts.length - 1;
        const head = new CsvModel(
          text,
          dialect,
          new RowIndex(
            m.starts.subarray(0, n),
            m.cols.subarray(0, n),
            m.starts[n] as number,
            dialect.delimiter,
          ),
        );
        opts.onFirst?.(head);
      } else if (m.type === "done") {
        clearTimeout(timer);
        worker.terminate();
        resolve(
          new CsvModel(
            text,
            dialect,
            new RowIndex(m.starts, m.cols, text.length, dialect.delimiter),
          ),
        );
      } else {
        clearTimeout(timer);
        worker.terminate();
        reject(new Error(m.message));
      }
    };
    worker.onerror = (e) => {
      clearTimeout(timer);
      worker.terminate();
      reject(new Error(e.message || "worker failed"));
    };
    worker.postMessage({ text, delimiter: dialect.delimiter });
  });
}
