import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { applyEdits } from "./chunked";
import { CHUNK_CHARS, chunksOf } from "./limits";
import { contentOf, type Range } from "./spans";

type Doc = { text: string };

export type { Range };

/** Text an integration removed, and where it stood in the document right after. */
export interface Deletion {
  at: number;
  text: string;
}

/** A change from the disk that was left out because `skip` refused its span. */
export interface Skipped {
  /** The span of the file as Inkwell last knew it. */
  from: number;
  to: number;
  /** What the disk has there now. */
  text: string;
}

/** What integrating an outside edit did: enough to show it and to take it back. */
export interface Integration {
  /** Heads of the integration alone: the text there is the file as it was read. */
  branch: A.Heads;
  /** Heads of the document right after, with whatever others typed meanwhile. */
  merged: A.Heads;
  /** The text the integration added, as spans of the document at `merged`. */
  inserted: Range[];
  /** The text it removed. */
  deleted: Deletion[];
  /** How many lines of the file the outside edit touched. */
  lines: number;
  /** The changes held back by `skip` (locked zones), for a suggestion. */
  skipped: Skipped[];
}

export interface IntegrateOptions {
  /**
   * Whether the change to the span `range` (in the text as Inkwell last knew the file) must not be
   * applied: a locked zone (ADR 0022). Its change is returned in `skipped` instead. Default: none.
   */
  skip?: ((range: Range) => boolean) | undefined;
}

// The text of a document seen through patches is followed as a list of segments: stretches of the
// original text that are still there, and stretches inserted since. That gives, at the end, where
// the inserted text stands and what was removed, without ever building the intermediate strings.
type Seg = { a: number; b: number } | { ins: number };

const sizeOf = (s: Seg): number => ("ins" in s ? s.ins : s.b - s.a);

/** The segments of an untouched text of `length` units. */
function original(length: number): Seg[] {
  return length > 0 ? [{ a: 0, b: length }] : [];
}

/** Splits the segment holding position `idx` there; returns the index of the segment starting at `idx`. */
function splitAt(segs: Seg[], idx: number): number {
  let pos = 0;
  for (let i = 0; i < segs.length; i++) {
    const seg = segs[i] as Seg;
    const n = sizeOf(seg);
    if (idx === pos) return i;
    if (idx < pos + n) {
      const k = idx - pos;
      segs.splice(
        i,
        1,
        ...("ins" in seg
          ? [{ ins: k }, { ins: n - k }]
          : [
              { a: seg.a, b: seg.a + k },
              { a: seg.a + k, b: seg.b },
            ]),
      );
      return i + 1;
    }
    pos += n;
  }
  return segs.length;
}

/** Follows the edits of `patches` (in order, each in the text the previous ones left) over `segs`. */
function follow(segs: Seg[], patches: A.Patch[]): Seg[] {
  for (const patch of patches) {
    if (patch.path[0] !== "text" || patch.path.length !== 2) continue;
    const idx = patch.path[1] as number;
    if (patch.action === "splice") {
      if (patch.value.length > 0) segs.splice(splitAt(segs, idx), 0, { ins: patch.value.length });
    } else if (patch.action === "del") {
      const from = splitAt(segs, idx);
      const to = splitAt(segs, idx + (patch.length ?? 1));
      segs.splice(from, to - from);
    }
  }
  return segs;
}

/** The inserted stretches, as spans of the final text, adjacent ones joined. */
function insertedOf(segs: Seg[]): Range[] {
  const out: Range[] = [];
  let pos = 0;
  for (const seg of segs) {
    const n = sizeOf(seg);
    if ("ins" in seg) {
      const last = out.at(-1);
      if (last && last.to === pos) last.to = pos + n;
      else out.push({ from: pos, to: pos + n });
    }
    pos += n;
  }
  return out;
}

/** What was removed from `source`, and where it stood in the final text. */
function deletedOf(segs: Seg[], source: string): Deletion[] {
  const out: Deletion[] = [];
  let pos = 0;
  let kept = 0; // how much of `source` is accounted for
  for (const seg of segs) {
    if ("ins" in seg) {
      pos += seg.ins;
      continue;
    }
    if (seg.a > kept) out.push({ at: pos, text: source.slice(kept, seg.a) });
    kept = seg.b;
    pos += seg.b - seg.a;
  }
  if (kept < source.length) out.push({ at: pos, text: source.slice(kept) });
  return out;
}

/**
 * The number of lines of `text` that `spans` touch (a point counts for the line it is on). The
 * newlines at the ends of a span are not counted: see {@link contentOf}.
 */
function linesTouched(text: string, spans: Range[], points: number[]): number {
  const starts: number[] = [0];
  for (let i = text.indexOf("\n"); i >= 0; i = text.indexOf("\n", i + 1)) starts.push(i + 1);
  const lineAt = (offset: number): number => {
    let lo = 0;
    let hi = starts.length - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if ((starts[mid] as number) <= offset) lo = mid;
      else hi = mid - 1;
    }
    return lo;
  };
  // The empty line after a final newline is not a line of the file.
  const real = (line: number) => (starts[line] as number) < text.length || text.length === 0;
  const lines = new Set<number>();
  for (const span of spans) {
    const content = contentOf(span, text.slice(span.from, span.to));
    // Only newlines: a blank line was added, the one the newline opens.
    const [from, to] = content ? [content.from, content.to - 1] : [span.to, span.to];
    for (let l = lineAt(from); l <= lineAt(to); l++) if (real(l)) lines.add(l);
  }
  for (const p of points) lines.add(lineAt(Math.min(p, text.length)));
  return lines.size;
}

/** A stretch of the old text and the text that replaces it. */
type Region = { from: number; to: number; text: string };

/** The lines of `text`, each with its newline (the last one has none if the text does not end with one). */
const linesOf = (text: string): string[] => text.match(/[^\n]*\n|[^\n]+/g) ?? [];

/**
 * Pairs of equal lines (indexes into `la` and `lb`, in order) that stay as they are between
 * `la[a0..a1)` and `lb[b0..b1)`. The shared start and end are taken first; what is left is
 * matched by a longest common subsequence when small (a million line pairs at most), else by
 * anchoring on the lines that occur once on each side, in the longest order they keep, and
 * matching what lies between anchors the same way. With no such anchor nothing is paired there:
 * the stretch stays one coarse region, the last resort.
 */
function sameLines(
  la: string[],
  lb: string[],
  a0: number,
  a1: number,
  b0: number,
  b1: number,
  out: [number, number][],
): void {
  while (a0 < a1 && b0 < b1 && la[a0] === lb[b0]) out.push([a0++, b0++]);
  let ta = a1;
  let tb = b1;
  while (a0 < ta && b0 < tb && la[ta - 1] === lb[tb - 1]) {
    ta--;
    tb--;
  }
  const n = ta - a0;
  const m = tb - b0;
  if (n > 0 && m > 0) {
    if (n * m <= 1_000_000) {
      const table = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
      const row = (i: number) => table[i] as Uint32Array;
      for (let i = n - 1; i >= 0; i--) {
        for (let j = m - 1; j >= 0; j--) {
          row(i)[j] =
            la[a0 + i] === lb[b0 + j]
              ? (row(i + 1)[j + 1] as number) + 1
              : Math.max(row(i + 1)[j] as number, row(i)[j + 1] as number);
        }
      }
      for (let i = 0, j = 0; i < n && j < m; ) {
        if (la[a0 + i] === lb[b0 + j]) out.push([a0 + i++, b0 + j++]);
        else if ((row(i + 1)[j] as number) >= (row(i)[j + 1] as number)) i++;
        else j++;
      }
    } else {
      anchored(la, lb, a0, ta, b0, tb, out);
    }
  }
  for (let k = ta; k < a1; k++) out.push([k, tb + (k - ta)]);
}

/** The anchoring of {@link sameLines}: lines unique on both sides, in their longest common order. */
function anchored(
  la: string[],
  lb: string[],
  a0: number,
  a1: number,
  b0: number,
  b1: number,
  out: [number, number][],
): void {
  const seenA = new Map<string, number>(); // line -> index, or -1 if it occurs again
  for (let i = a0; i < a1; i++) {
    const line = la[i] as string;
    seenA.set(line, seenA.has(line) ? -1 : i);
  }
  const countB = new Map<string, number>();
  for (let j = b0; j < b1; j++) countB.set(lb[j] as string, (countB.get(lb[j] as string) ?? 0) + 1);
  const pairs: [number, number][] = []; // by position in b
  for (let j = b0; j < b1; j++) {
    const line = lb[j] as string;
    const i = seenA.get(line);
    if (i !== undefined && i >= 0 && countB.get(line) === 1) pairs.push([i, j]);
  }
  // Longest increasing run of `i` over the pairs (patience sorting).
  const tails: number[] = [];
  const prev: number[] = [];
  for (const [k, [i]] of pairs.entries()) {
    let lo = 0;
    let hi = tails.length;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if ((pairs[tails[mid] as number] as [number, number])[0] < i) lo = mid + 1;
      else hi = mid;
    }
    prev[k] = lo > 0 ? (tails[lo - 1] as number) : -1;
    tails[lo] = k;
  }
  const chain: [number, number][] = [];
  for (let k = tails.at(-1) ?? -1; k >= 0; k = prev[k] as number)
    chain.push(pairs[k] as [number, number]);
  chain.reverse();
  let i0 = a0;
  let j0 = b0;
  for (const [i, j] of chain) {
    sameLines(la, lb, i0, i, j0, j, out);
    out.push([i, j]);
    i0 = i + 1;
    j0 = j + 1;
  }
  if (chain.length > 0) sameLines(la, lb, i0, a1, j0, b1, out);
}

/**
 * Line-level changes between two texts: the spans of `a` that `b` replaces, in order, found by
 * matching their lines ({@link sameLines}).
 */
function lineRegions(a: string, b: string): Region[] {
  const la = linesOf(a);
  const lb = linesOf(b);
  const same: [number, number][] = [];
  sameLines(la, lb, 0, la.length, 0, lb.length, same);
  same.push([la.length, lb.length]);

  const offsets = [0];
  for (const line of la) offsets.push((offsets.at(-1) as number) + line.length);
  const regions: Region[] = [];
  let i = 0;
  let j = 0;
  for (const [si, sj] of same) {
    if (si > i || sj > j) {
      regions.push({
        from: offsets[i] as number,
        to: offsets[si] as number,
        text: lb.slice(j, sj).join(""),
      });
    }
    i = si + 1;
    j = sj + 1;
  }
  // Lines replaced by as many lines changed one for one: each can be refused on its own.
  return regions.flatMap((r) => {
    const from = linesOf(a.slice(r.from, r.to));
    const to = linesOf(r.text);
    if (from.length < 2 || from.length !== to.length) return [r];
    let at = r.from;
    return from.map((line, i) => {
      const part = { from: at, to: at + line.length, text: to[i] as string };
      at = part.to;
      return part;
    });
  });
}

/**
 * Integrates an outside edit of the file. `text` is what the file holds now; `base` the heads of
 * the document when the file last agreed with it. The edit is made as of `base`, then merged:
 * what others typed meanwhile stays. Returns undefined if the file already says what the document
 * said at `base`.
 */
export function integrate(
  handle: DocHandle<Doc>,
  base: A.Heads,
  text: string,
  opts: IntegrateOptions = {},
): Integration | undefined {
  const doc = handle.doc();
  if (!doc) return undefined;
  const baseText = A.view(doc, base).text;
  if (baseText === text) return undefined;
  const before = A.getHeads(doc);

  const skipped: Skipped[] = [];
  let apply: A.ChangeFn<Doc>;
  if (opts.skip) {
    const keep = lineRegions(baseText, text).filter((r) => {
      if (!opts.skip?.({ from: r.from, to: r.to })) return true;
      skipped.push(r);
      return false;
    });
    if (keep.length === 0) return skippedOnly(skipped);
    apply = (d) => {
      // From the end, so the spans (as of `base`) stay valid while applying them.
      for (const r of keep.toReversed()) A.splice(d, ["text"], r.from, r.to - r.from, r.text);
    };
  } else {
    apply = (d) => A.updateText(d, ["text"], text);
  }

  const pieces = opts.skip ? undefined : growthPieces(baseText, text);
  let branch: A.Heads | undefined;
  handle.update((current) => {
    let doc = current;
    let heads = base;
    for (const fn of pieces ?? [apply]) {
      const result = A.changeAt(doc, heads, fn);
      doc = result.newDoc;
      if (result.newHeads) heads = branch = result.newHeads;
    }
    return doc;
  });
  if (!branch) return undefined;
  const after = handle.doc();
  if (!after) return undefined;
  const merged = A.getHeads(after);

  // How many lines of the file the edit touched: from the edit alone (base to branch).
  const own = follow(original(baseText.length), A.diff(after, base, branch));
  const newText = A.view(after, branch).text;
  const lines = linesTouched(
    newText,
    insertedOf(own),
    deletedOf(own, baseText).map((d) => d.at),
  );

  // Where it stands in the document with the others' typing: from the document as it was.
  const there = follow(original(A.view(after, before).text.length), A.diff(after, before, merged));
  return {
    branch,
    merged,
    inserted: insertedOf(there),
    deleted: deletedOf(there, A.view(after, before).text),
    lines,
    skipped,
  };
}

function skippedOnly(skipped: Skipped[]): Integration {
  return { branch: [], merged: [], inserted: [], deleted: [], lines: 0, skipped };
}

/**
 * Takes an integration back: removes the text it added and puts back what it removed, as one
 * change. Text others typed since stays, even inside what was added.
 */
export function undoIntegration(handle: DocHandle<Doc>, done: Integration): void {
  const doc = handle.doc();
  if (!doc || done.merged.length === 0) return;
  const mergedText = A.view(doc, done.merged).text;
  const segs = follow(original(mergedText.length), A.diff(doc, done.merged, A.getHeads(doc)));

  // The surviving characters of each added span, and where each removed text goes back.
  const edits: { at: number; remove: number; put: string }[] = [];
  let pos = 0;
  for (const seg of segs) {
    const n = sizeOf(seg);
    if (!("ins" in seg)) {
      for (const r of done.inserted) {
        const from = Math.max(seg.a, r.from);
        const to = Math.min(seg.b, r.to);
        if (from < to) edits.push({ at: pos + (from - seg.a), remove: to - from, put: "" });
      }
    }
    pos += n;
  }
  for (const d of done.deleted) edits.push({ at: where(segs, d.at), remove: 0, put: d.text });

  if (edits.length === 0) return;
  // From the end; at one place, the removal first, then what is put back.
  edits.sort((x, y) => y.at - x.at || y.remove - x.remove);
  applyEdits(handle, edits);
}

/**
 * The file changes a lot of text at once (a file that was empty, a big paste or a rewrite in
 * another program), so one change would hold it all (ADR 0028 D3). The edit is built from the
 * line-level regions of {@link lineRegions}, as the locked-zone path does, so that two far-apart
 * edits stay two edits and a peer's typing between them merges by line. Only a region that is
 * itself over 64 Ki goes in changes of at most 64 Ki each; the others, in one. They come as
 * functions to apply in turn from the end of the text, each on the branch the previous one made.
 * Undefined when no region is that big: the finer edit of `updateText` does it. Past a million
 * line pairs `lineRegions` itself falls back to one coarse region, the last resort.
 */
function growthPieces(baseText: string, text: string): A.ChangeFn<Doc>[] | undefined {
  if (Math.max(text.length, baseText.length) <= CHUNK_CHARS) return undefined;
  const regions = lineRegions(baseText, text);
  if (!regions.some((r) => Math.max(r.to - r.from, r.text.length) > CHUNK_CHARS)) return undefined;
  const pieces: A.ChangeFn<Doc>[] = [];
  let small: Region[] = [];
  const flush = () => {
    if (small.length === 0) return;
    const batch = small;
    small = [];
    pieces.push((d) => {
      for (const r of batch) A.splice(d, ["text"], r.from, r.to - r.from, r.text);
    });
  };
  for (const r of regions.toReversed()) {
    if (Math.max(r.to - r.from, r.text.length) <= CHUNK_CHARS) {
      small.push(r);
      continue;
    }
    flush();
    let at = r.from;
    const chunks = chunksOf(r.text);
    // A region that only removes has no text to cut: one change.
    for (const [i, piece] of (chunks.length > 0 ? chunks : [""]).entries()) {
      const from = at;
      at += piece.length;
      pieces.push((d) => A.splice(d, ["text"], from, i === 0 ? r.to - r.from : 0, piece));
    }
  }
  flush();
  return pieces;
}

/** Where the original position `at` stands in the text the segments describe. */
function where(segs: Seg[], at: number): number {
  let pos = 0;
  for (const seg of segs) {
    if ("ins" in seg) {
      pos += seg.ins;
      continue;
    }
    if (at < seg.a) return pos; // inside something removed since: where it was
    if (at <= seg.b) return pos + (at - seg.a);
    pos += seg.b - seg.a;
  }
  return pos;
}
