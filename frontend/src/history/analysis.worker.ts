/// History worker: follows the document's changes off the UI thread (sessions, who wrote what).
import { next as A } from "@automerge/automerge";
import { analyze, type Wire } from "./analysis";
import { Tracker } from "./track";
import type { HDoc, Heads } from "./types";

interface Job {
  id: number;
  /** Changes the worker has not seen yet. */
  changes: Uint8Array[];
  since: Heads | undefined;
  /** The actors of this device: they alone are one person in the grouping. */
  mine: string[];
}

const scope = self as unknown as {
  postMessage(message: unknown): void;
  onmessage: ((e: { data: Job }) => void) | null;
};

let doc = A.init<HDoc>();
const tracker = new Tracker();
// Jobs are answered one at a time, in order: a later one needs the document the earlier built.
let queue: Promise<void> = Promise.resolve();

scope.onmessage = (e) => {
  const { id, changes, since, mine } = e.data;
  queue = queue.then(async () => {
    try {
      [doc] = A.applyChanges(doc, changes);
      const result = await analyze(doc, tracker, since, new Set(mine));
      const { sessions, ...rest } = result.blame;
      void sessions;
      const wire: Wire = { ...result, blame: rest };
      scope.postMessage({ id, result: wire });
    } catch (err) {
      scope.postMessage({ id, error: err instanceof Error ? err.message : String(err) });
    }
  });
};

// Tells the page it listens: a module worker only starts once the Automerge wasm is loaded.
scope.postMessage({ ready: true });
