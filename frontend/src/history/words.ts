/** Differences at the granularity of the word, with moves detected (DIF-01 to DIF-03). */

export type OpKind = "eq" | "add" | "del";

export interface Op {
  kind: OpKind;
  text: string;
  /** Offset of this stretch in the old text (eq, del) and in the new text (eq, add). */
  a: number;
  b: number;
  /** A single changed word: the letters that really differ, inside `text`. */
  inner?: [number, number];
  /** The same block was removed here and added elsewhere: both ends carry the same id. */
  moved?: number;
}

type Edit = { kind: OpKind; n: number };

/** A longest-common-subsequence diff (Myers), on arrays of comparable items. */
function myers<T>(a: readonly T[], b: readonly T[], maxD: number): Edit[] | undefined {
  const n = a.length;
  const m = b.length;
  const max = Math.min(n + m, maxD);
  const offset = max + 1;
  const v = new Int32Array(2 * max + 3);
  const trace: Int32Array[] = [];
  for (let d = 0; d <= max; d++) {
    trace.push(v.slice());
    for (let k = -d; k <= d; k += 2) {
      let x: number;
      if (k === -d || (k !== d && (v[offset + k - 1] as number) < (v[offset + k + 1] as number))) {
        x = v[offset + k + 1] as number;
      } else {
        x = (v[offset + k - 1] as number) + 1;
      }
      let y = x - k;
      while (x < n && y < m && a[x] === b[y]) {
        x++;
        y++;
      }
      v[offset + k] = x;
      if (x >= n && y >= m) return backtrack(trace, d, n, m, offset);
    }
  }
  return undefined;
}

function backtrack(trace: Int32Array[], d: number, n: number, m: number, offset: number): Edit[] {
  const rev: Edit[] = [];
  const push = (kind: OpKind) => {
    const last = rev.at(-1);
    if (last && last.kind === kind) last.n++;
    else rev.push({ kind, n: 1 });
  };
  let x = n;
  let y = m;
  for (let step = d; step >= 0; step--) {
    const v = trace[step] as Int32Array;
    const k = x - y;
    const down =
      k === -step || (k !== step && (v[offset + k - 1] as number) < (v[offset + k + 1] as number));
    const prevK = down ? k + 1 : k - 1;
    const prevX = step === 0 ? 0 : (v[offset + prevK] as number);
    const prevY = step === 0 ? 0 : prevX - prevK;
    while (x > prevX && y > prevY) {
      push("eq");
      x--;
      y--;
    }
    if (step > 0) {
      push(down ? "add" : "del");
      if (down) y--;
      else x--;
    }
  }
  return rev.reverse();
}

/** Diffs two arrays: trims what they share at both ends, then runs Myers on the middle. */
function diffSeq<T>(a: readonly T[], b: readonly T[], maxD = 1000): Edit[] {
  let head = 0;
  while (head < a.length && head < b.length && a[head] === b[head]) head++;
  let tail = 0;
  while (
    tail < a.length - head &&
    tail < b.length - head &&
    a[a.length - 1 - tail] === b[b.length - 1 - tail]
  ) {
    tail++;
  }
  const mid = myers(a.slice(head, a.length - tail), b.slice(head, b.length - tail), maxD) ?? [
    { kind: "del" as const, n: a.length - head - tail },
    { kind: "add" as const, n: b.length - head - tail },
  ];
  const out: Edit[] = [];
  const add = (e: Edit) => {
    if (e.n === 0) return;
    const last = out.at(-1);
    if (last && last.kind === e.kind) last.n += e.n;
    else out.push({ ...e });
  };
  add({ kind: "eq", n: head });
  for (const e of mid) add(e);
  add({ kind: "eq", n: tail });
  return out;
}

const WORD = /[\p{L}\p{M}\p{N}_'’-]+|\s+|[^\p{L}\p{M}\p{N}\s]/gu;
const tokens = (s: string): string[] => s.match(WORD) ?? [];
const lines = (s: string): string[] => s.match(/[^\n]*\n|[^\n]+/g) ?? [];

const isWord = (t: string) => /[\p{L}\p{N}]/u.test(t);

/** The ops of a diff between two lists, over the strings they came from. */
function ops(a: readonly string[], b: readonly string[], edits: Edit[]): Op[] {
  const out: Op[] = [];
  let i = 0;
  let j = 0;
  let pa = 0;
  let pb = 0;
  for (const e of edits) {
    if (e.kind === "eq") {
      const text = a.slice(i, i + e.n).join("");
      out.push({ kind: "eq", text, a: pa, b: pb });
      i += e.n;
      j += e.n;
      pa += text.length;
      pb += text.length;
    } else if (e.kind === "del") {
      const text = a.slice(i, i + e.n).join("");
      out.push({ kind: "del", text, a: pa, b: pb });
      i += e.n;
      pa += text.length;
    } else {
      const text = b.slice(j, j + e.n).join("");
      out.push({ kind: "add", text, a: pa, b: pb });
      j += e.n;
      pb += text.length;
    }
  }
  return out;
}

/** Word diff of two texts that are small enough to be diffed directly. */
function wordOps(a: string, b: string): Op[] {
  const ta = tokens(a);
  const tb = tokens(b);
  return ops(ta, tb, diffSeq(ta, tb));
}

/**
 * The difference between two texts: first by lines (cheap, whatever the size), then by words
 * inside the lines that changed. Pure additions and removals are kept as they are.
 */
export function wordDiff(a: string, b: string): Op[] {
  if (a === b) return a ? [{ kind: "eq", text: a, a: 0, b: 0 }] : [];
  const la = lines(a);
  const lb = lines(b);
  const edits = diffSeq(la, lb);
  const lineOps = ops(la, lb, edits);
  const out: Op[] = [];
  for (let i = 0; i < lineOps.length; i++) {
    const op = lineOps[i] as Op;
    const next = lineOps[i + 1];
    if (op.kind === "del" && next?.kind === "add") {
      // Lines replaced by lines: the words inside tell what really changed.
      for (const w of wordOps(op.text, next.text)) {
        out.push({ ...w, a: w.a + op.a, b: w.b + op.b });
      }
      i++;
    } else if (op.kind === "add" && next?.kind === "del") {
      for (const w of wordOps(next.text, op.text)) {
        out.push({ ...w, a: w.a + next.a, b: w.b + op.b });
      }
      i++;
    } else out.push(op);
  }
  return underline(mergeRuns(out));
}

/** Consecutive ops of one kind joined, deletions before additions at the same place. */
function mergeRuns(list: Op[]): Op[] {
  const out: Op[] = [];
  let dels: Op[] = [];
  let adds: Op[] = [];
  const flush = () => {
    for (const group of [dels, adds]) {
      if (group.length === 0) continue;
      const first = group[0] as Op;
      out.push({ ...first, text: group.map((o) => o.text).join("") });
    }
    dels = [];
    adds = [];
  };
  for (const op of list) {
    if (op.kind === "eq") {
      flush();
      const last = out.at(-1);
      if (last?.kind === "eq") last.text += op.text;
      else out.push({ ...op });
    } else if (op.kind === "del") dels.push(op);
    else adds.push(op);
  }
  flush();
  return out;
}

const trimmed = (s: string) => s.replace(/^\s+|\s+$/g, "");

/** A word replaced by one word: the letters that differ are the ones to underline (DIF-02). */
function underline(list: Op[]): Op[] {
  for (let i = 0; i + 1 < list.length; i++) {
    const del = list[i] as Op;
    const add = list[i + 1] as Op;
    if (del.kind !== "del" || add.kind !== "add") continue;
    const x = trimmed(del.text);
    const y = trimmed(add.text);
    if (x.length === 0 || y.length === 0 || /\s/.test(x) || /\s/.test(y)) continue;
    let head = 0;
    while (head < x.length && head < y.length && x[head] === y[head]) head++;
    let tail = 0;
    while (
      tail < x.length - head &&
      tail < y.length - head &&
      x[x.length - 1 - tail] === y[y.length - 1 - tail]
    ) {
      tail++;
    }
    const lead = (s: string, t: string) => s.indexOf(t);
    del.inner = [lead(del.text, x) + head, lead(del.text, x) + x.length - tail];
    add.inner = [lead(add.text, y) + head, lead(add.text, y) + y.length - tail];
  }
  return list;
}

const normal = (s: string) => s.replace(/\s+/g, " ").trim();
const wordsOf = (s: string) => tokens(s).filter(isWord);

/** How alike two blocks are, 0 to 1: the share of words they have in common. */
function likeness(x: string, y: string): number {
  const a = wordsOf(x);
  const b = wordsOf(y);
  const counts = new Map<string, number>();
  for (const w of a) counts.set(w, (counts.get(w) ?? 0) + 1);
  let shared = 0;
  for (const w of b) {
    const n = counts.get(w) ?? 0;
    if (n > 0) {
      shared++;
      counts.set(w, n - 1);
    }
  }
  return (2 * shared) / Math.max(1, a.length + b.length);
}

/** A block this long (in words) can be said to have moved rather than been removed and added. */
const MOVE_MIN_WORDS = 4;

/**
 * Marks removed blocks that reappear as added blocks elsewhere (the same text, or almost): they
 * moved (DIF-03). Returns the number of moves found.
 */
export function detectMoves(list: Op[]): number {
  const dels = list.filter((o) => o.kind === "del" && wordsOf(o.text).length >= MOVE_MIN_WORDS);
  const adds = list.filter((o) => o.kind === "add" && wordsOf(o.text).length >= MOVE_MIN_WORDS);
  const taken = new Set<Op>();
  let id = 0;
  for (const d of dels) {
    let best: Op | undefined;
    let score = 0.85;
    for (const a of adds) {
      if (taken.has(a)) continue;
      const s = normal(a.text) === normal(d.text) ? 1 : likeness(a.text, d.text);
      if (s >= score) {
        best = a;
        score = s;
        if (s === 1) break;
      }
    }
    if (best) {
      taken.add(best);
      id++;
      d.moved = id;
      best.moved = id;
    }
  }
  return id;
}

/** A diff with its moves marked. */
export function diffWithMoves(a: string, b: string): Op[] {
  const list = wordDiff(a, b);
  detectMoves(list);
  return list;
}
