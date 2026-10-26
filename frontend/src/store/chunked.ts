import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { CHUNK_CHARS, chunksOf } from "./limits";

/** One splice of the text: remove `remove` characters at `at`, put `put` there. */
export interface Edit {
  at: number;
  remove: number;
  put: string;
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
): void {
  let small: Edit[] = [];
  const flush = () => {
    if (small.length === 0) return;
    const batch = small;
    small = [];
    handle.change((d) => {
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
      handle.change((d) => A.splice(d, path.slice(), from, del, piece));
      at += piece.length;
      remove = 0;
    }
  }
  flush();
}
