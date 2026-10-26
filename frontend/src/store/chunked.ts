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
  let small: Edit[] = [];
  const flush = () => {
    if (small.length === 0) return;
    const batch = small;
    small = [];
    change((d) => {
      for (const e of batch) A.splice(d, path.slice(), e.at, e.remove, e.put);
    });
  };
  for (const e of edits) {
    if (e.put.length <= CHUNK_CHARS) {
      small.push(e);
      continue;
    }
    flush();
    let at = e.at;
    let remove = e.remove;
    for (const piece of chunksOf(e.put)) {
      const from = at;
      const del = remove;
      change((d) => A.splice(d, path.slice(), from, del, piece));
      at += piece.length;
      remove = 0;
    }
  }
  flush();
  // Only a first write and no edit (nothing to change but a version): still one change.
  if (first) change(() => undefined);
}
