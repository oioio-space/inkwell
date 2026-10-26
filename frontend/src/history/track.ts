import { next as A } from "@automerge/automerge";
import type { Session } from "./log";

/** A stretch of the current text and the session that wrote it. */
export interface Span {
  from: number;
  to: number;
  /** Index in the sessions list. */
  s: number;
  /** It replaced text in the same session (a modification, not a pure addition). */
  replace: boolean;
}

/** Text that was removed, where it stood in the current text, and who removed it. */
export interface Deletion {
  at: number;
  text: string;
  /** The session that removed it. */
  by: number;
  /** The session that had written it. */
  owner: number;
}

export interface Blame {
  spans: Span[];
  deletions: Deletion[];
  sessions: readonly Session[];
  /** The length of the text this describes. */
  length: number;
}

// The text is followed as a list of pieces: the stretches each session wrote and still there, and
// zero-length markers where text was removed. Patches are applied to it session by session; no
// intermediate text is ever built, so a document of several megabytes costs what its edits cost.
interface Piece {
  len: number;
  s: number;
  text: string;
  replace: boolean;
  /** A marker: text of session `owner` removed by session `by` at this point. */
  del?: { by: number; owner: number };
}

/** Index of the first piece that starts at or after `pos`; `afterMarkers`: past the markers there. */
function indexAt(pieces: Piece[], pos: number, afterMarkers: boolean): number {
  let acc = 0;
  for (let i = 0; i < pieces.length; i++) {
    const p = pieces[i] as Piece;
    if (acc === pos && (p.len > 0 || !afterMarkers)) return i;
    if (acc + p.len > pos) {
      const k = pos - acc;
      pieces.splice(
        i,
        1,
        { ...p, len: k, text: p.text.slice(0, k) },
        { ...p, len: p.len - k, text: p.text.slice(k) },
      );
      return i + 1;
    }
    acc += p.len;
  }
  return pieces.length;
}

function applyPatches(pieces: Piece[], patches: A.Patch[], s: number): void {
  let lastDelAt = -1;
  // The text just inserted: removing right after it, or inserting right where text was removed,
  // is a replacement (a modification), whichever patch comes first.
  let lastIns: { piece: Piece; end: number } | undefined;
  for (const patch of patches) {
    if (patch.path[0] !== "text") continue;
    // The whole text put in one go (not typed): everything before is gone, this is all new.
    if (patch.path.length === 1 && patch.action === "put" && typeof patch.value === "string") {
      const gone = pieces.filter((p) => p.len > 0);
      pieces.length = 0;
      for (const p of gone) {
        pieces.push({ len: 0, s, text: p.text, replace: false, del: { by: s, owner: p.s } });
      }
      if (patch.value.length > 0) {
        pieces.push({ len: patch.value.length, s, text: patch.value, replace: gone.length > 0 });
      }
      continue;
    }
    if (patch.path.length !== 2) continue;
    const at = patch.path[1] as number;
    if (patch.action === "splice") {
      const text = patch.value;
      if (text.length === 0) continue;
      const piece: Piece = { len: text.length, s, text, replace: at === lastDelAt };
      pieces.splice(indexAt(pieces, at, true), 0, piece);
      lastIns = { piece, end: at + text.length };
    } else if (patch.action === "del") {
      const length = patch.length ?? 1;
      const from = indexAt(pieces, at, true);
      const to = indexAt(pieces, at + length, false);
      const removed = pieces.slice(from, to).filter((p) => p.len > 0);
      const markers: Piece[] = [];
      for (const p of removed) {
        const last = markers.at(-1);
        if (last?.del && last.del.owner === p.s) last.text += p.text;
        else markers.push({ len: 0, s, text: p.text, replace: false, del: { by: s, owner: p.s } });
      }
      pieces.splice(from, to - from, ...markers);
      if (lastIns && at === lastIns.end) lastIns.piece.replace = true;
      lastDelAt = at;
    }
  }
}

function summarize(pieces: readonly Piece[], sessions: readonly Session[]): Blame {
  const spans: Span[] = [];
  const deletions: Deletion[] = [];
  let pos = 0;
  for (const p of pieces) {
    if (p.del) {
      deletions.push({ at: pos, text: p.text, by: p.del.by, owner: p.del.owner });
      continue;
    }
    const last = spans.at(-1);
    if (last && last.s === p.s && last.replace === p.replace && last.to === pos) last.to += p.len;
    else spans.push({ from: pos, to: pos + p.len, s: p.s, replace: p.replace });
    pos += p.len;
  }
  return { spans, deletions, sessions, length: pos };
}

const yieldToUi = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 0));

export interface TrackOptions {
  signal?: AbortSignal | undefined;
  /** How long to work before giving the thread back, in ms. */
  budget?: number;
  /** Called after each batch with the fraction done (0 to 1). */
  onProgress?: (done: number) => void;
}

/**
 * Follows who wrote each stretch of the text, session by session, from the empty document. The
 * work is done in slices (the thread is given back every few milliseconds) so a large document
 * never freezes the page, and it is incremental: sessions that are over are kept between calls,
 * and only the new ones and the one still open are followed again.
 */
export class Tracker {
  #pieces: Piece[] = [];
  /** The sessions already followed, as (id, after) pairs. */
  #done: { id: string; after: string }[] = [];

  async update(
    doc: A.Doc<{ text: string }>,
    sessions: readonly Session[],
    { signal, budget = 8, onProgress }: TrackOptions = {},
  ): Promise<Blame> {
    // Sessions over: all but the last, which may still grow.
    const closed = Math.max(0, sessions.length - 1);
    let k = 0;
    while (k < this.#done.length && k < closed) {
      const d = this.#done[k] as { id: string; after: string };
      const s = sessions[k] as Session;
      if (d.id !== s.id || d.after !== s.after.join(",")) break;
      k++;
    }
    // Something earlier changed (rare): the sure way is to follow everything again.
    if (k < this.#done.length) {
      this.#done = [];
      this.#pieces = [];
    }
    let t0 = performance.now();
    for (let i = this.#done.length; i < closed; i++) {
      if (signal?.aborted) throw new DOMException("aborted", "AbortError");
      const s = sessions[i] as Session;
      applyPatches(this.#pieces, A.diff(doc, s.before, s.after), i);
      this.#done.push({ id: s.id, after: s.after.join(",") });
      if (performance.now() - t0 > budget) {
        onProgress?.(i / Math.max(1, sessions.length));
        await yieldToUi();
        t0 = performance.now();
      }
    }
    const open = [...this.#pieces];
    const last = sessions[closed];
    if (last) applyPatches(open, A.diff(doc, last.before, last.after), closed);
    return summarize(open, sessions);
  }
}

/** One-shot: who wrote what, now. */
export function blameOf(
  doc: A.Doc<{ text: string }>,
  sessions: readonly Session[],
  opts?: TrackOptions,
): Promise<Blame> {
  return new Tracker().update(doc, sessions, opts);
}

/** The session that wrote the character at `pos`, or -1. */
export function sessionAt(blame: Blame, pos: number): number {
  let lo = 0;
  let hi = blame.spans.length - 1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    const sp = blame.spans[mid] as Span;
    if (pos < sp.from) hi = mid - 1;
    else if (pos >= sp.to) lo = mid + 1;
    else return sp.s;
  }
  return -1;
}
