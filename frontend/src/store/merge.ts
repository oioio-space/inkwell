import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
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

/** The lines of `text`, each with its newline (the last one has none if the text does not end with one). */
const linesOf = (text: string): string[] => text.match(/[^\n]*\n|[^\n]+/g) ?? [];

/**
 * Line-level changes between two texts: the spans of `a` that `b` replaces, in order, found by a
 * longest common subsequence of lines (one single region past a million line pairs).
 */
function lineRegions(a: string, b: string): { from: number; to: number; text: string }[] {
  const la = linesOf(a);
  const lb = linesOf(b);
  let head = 0;
  while (head < la.length && head < lb.length && la[head] === lb[head]) head++;
  let tail = 0;
  while (
    tail < la.length - head &&
    tail < lb.length - head &&
    la[la.length - 1 - tail] === lb[lb.length - 1 - tail]
  ) {
    tail++;
  }
  const n = la.length - head - tail;
  const m = lb.length - head - tail;
  // Pairs of equal lines (indexes from `head`) that stay as they are, ending with a sentinel.
  const same: [number, number][] = [];
  if (n * m <= 1_000_000) {
    const row = (i: number) => table[i] as Uint32Array;
    const table = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
    for (let i = n - 1; i >= 0; i--) {
      for (let j = m - 1; j >= 0; j--) {
        row(i)[j] =
          la[head + i] === lb[head + j]
            ? (row(i + 1)[j + 1] as number) + 1
            : Math.max(row(i + 1)[j] as number, row(i)[j + 1] as number);
      }
    }
    for (let i = 0, j = 0; i < n && j < m; ) {
      if (la[head + i] === lb[head + j]) same.push([i++, j++]);
      else if ((row(i + 1)[j] as number) >= (row(i)[j + 1] as number)) i++;
      else j++;
    }
  }
  same.push([n, m]);

  const offsets = [0];
  for (const line of la) offsets.push((offsets.at(-1) as number) + line.length);
  const regions: { from: number; to: number; text: string }[] = [];
  let i = 0;
  let j = 0;
  for (const [si, sj] of same) {
    if (si > i || sj > j) {
      regions.push({
        from: offsets[head + i] as number,
        to: offsets[head + si] as number,
        text: lb.slice(head + j, head + sj).join(""),
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

  let branch: A.Heads | undefined;
  handle.update((current) => {
    const result = A.changeAt(current, base, apply);
    branch = result.newHeads ?? undefined;
    return result.newDoc;
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
  handle.change((d) => {
    for (const e of edits) A.splice(d, ["text"], e.at, e.remove, e.put);
  });
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
