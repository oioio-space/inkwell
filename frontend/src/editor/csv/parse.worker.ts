/// <reference lib="webworker" />
import { papaIndex } from "./papaIndex";

/** Messages: in `{ text, delimiter }`; out `progress`, `first` (usable grid), `done`, `error`. */
self.onmessage = (e: MessageEvent<{ text: string; delimiter: string }>) => {
  try {
    const { text, delimiter } = e.data;
    const result = papaIndex(
      text,
      delimiter,
      (done) => self.postMessage({ type: "progress", done }),
      (partial) => self.postMessage({ type: "first", ...partial }),
    );
    self.postMessage({ type: "done", starts: result.starts, cols: result.cols }, [
      result.starts.buffer,
      result.cols.buffer,
    ]);
  } catch (err) {
    self.postMessage({
      type: "error",
      message: err instanceof Error ? err.message : "parse failed",
    });
  }
};
