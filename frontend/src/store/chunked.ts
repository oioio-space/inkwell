import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { CHUNK_CHARS, chunksOf } from "./limits";

/** One splice of the text: remove `remove` characters at `at`, put `put` there. */
export interface Edit {
  at: number;
  remove: number;
  put: string;
}

/** What goes with the edits: the message of the change, and more to write in the first one. */
export interface EditOptions {
  message?: string;
  /** Written in the first change, so a version or a mark is made together with the first edit. */
  first?: (d: Record<string, unknown>) => void;
}

/**
 * Applies the edits to the document's text, in the order given (each one in the coordinates the
 * previous ones left), as ONE change; except that an addition of more than 64 Ki characters goes
 * in changes of at most 64 Ki each (ADR 0028 D3). Automerge holds a text pasted in one change at
 * about 300 bytes per character and the same text in pieces at about 7, so a paste of 12 MB no
 * longer costs 3.6 GB of WASM before anything else is copied.
 */
export function applyEdits(
  handle: DocHandle<{ text: string }>,
  edits: readonly Edit[],
  path: string[] = ["text"],
  opts: EditOptions = {},
): void {
  let first = opts.first;
  const change = (fn: (d: { text: string }) => void) => {
    const f = first;
    first = undefined;
    const run = (d: { text: string }) => {
      f?.(d as unknown as Record<string, unknown>);
      fn(d);
    };
    if (opts.message) handle.change(run, { message: opts.message });
    else handle.change(run);
  };
  for (const batch of chunkEdits(edits)) change((d) => spliceAll(d, path, batch));
  // Only a first write and no edit (nothing to change but a version): still one change.
  if (first) change(() => undefined);
}

/**
 * The edits cut into the changes they are written in (ADR 0028 D3): small edits in a row share
 * one change; an addition of more than 64 Ki characters goes in changes of at most 64 Ki each.
 * Every write of edits to a document goes through this cut, whatever heads it is made on.
 */
export function chunkEdits(edits: readonly Edit[]): Edit[][] {
  const batches: Edit[][] = [];
  let small: Edit[] = [];
  const close = () => {
    if (small.length > 0) batches.push(small);
    small = [];
  };
  for (const e of edits) {
    if (e.put.length <= CHUNK_CHARS) {
      small.push(e);
      continue;
    }
    close();
    let at = e.at;
    let remove = e.remove;
    for (const piece of chunksOf(e.put)) {
      batches.push([{ at, remove, put: piece }]);
      at += piece.length;
      remove = 0;
    }
  }
  close();
  return batches;
}

/** Applies one batch of edits, in order, inside a change. */
export function spliceAll(d: unknown, path: readonly string[], batch: readonly Edit[]): void {
  for (const e of batch) A.splice(d as { text: string }, path.slice(), e.at, e.remove, e.put);
}
