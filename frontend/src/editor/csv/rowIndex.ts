import { scanAll, scanRecord } from "./records";

/**
 * Where each record of the text starts, and how many cells it has. A text change updates the
 * index locally: it rescans from the first touched record until the scan lands on a record
 * boundary that existed before (shifted by the change), and shifts everything after it. Nothing
 * after the change is parsed again, so one edit costs the touched records plus a pass over the
 * offsets (a typed array, so 500 000 rows shift in well under a millisecond).
 */
export class RowIndex {
  starts: Int32Array;
  cols: Uint16Array;
  count: number;
  length: number;
  /** How many records have each cell count; the widest one gives the grid's width. */
  readonly hist = new Map<number, number>();

  constructor(
    starts: ArrayLike<number>,
    cols: ArrayLike<number>,
    length: number,
    readonly delim: string,
    readonly quote = true,
  ) {
    this.count = starts.length;
    this.starts = Int32Array.from(starts);
    this.cols = Uint16Array.from(cols, (c) => Math.min(c, 0xffff));
    this.length = length;
    for (let i = 0; i < this.count; i++) this.#bump(this.cols[i] as number, 1);
  }

  static scan(text: string, delim: string, quote = true): RowIndex {
    const { starts, cols } = scanAll(text, delim, quote);
    return new RowIndex(starts, cols, text.length, delim, quote);
  }

  #bump(cols: number, by: number) {
    const n = (this.hist.get(cols) ?? 0) + by;
    if (n <= 0) this.hist.delete(cols);
    else this.hist.set(cols, n);
  }

  /** The widest record. */
  get maxCols(): number {
    let m = 0;
    for (const c of this.hist.keys()) if (c > m) m = c;
    return m;
  }

  /** The most common cell count: the table's real width; other records are "uneven". */
  get modalCols(): number {
    let best = 0;
    let bestN = -1;
    for (const [c, n] of this.hist) {
      if (n > bestN || (n === bestN && c > best)) {
        best = c;
        bestN = n;
      }
    }
    return best;
  }

  /** How many records do not have the usual number of cells. */
  get uneven(): number {
    return this.count - (this.hist.get(this.modalCols) ?? 0);
  }

  /** The record containing text offset `pos` (the last one for any offset past the end). */
  recordAt(pos: number): number {
    let lo = 0;
    let hi = this.count - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if ((this.starts[mid] as number) <= pos) lo = mid;
      else hi = mid - 1;
    }
    return lo;
  }

  /** Offset where record `r` ends, terminator excluded. */
  endOf(text: string, r: number): number {
    const next = r + 1 < this.count ? (this.starts[r + 1] as number) : this.length;
    let e = next;
    if (e > 0 && text.charCodeAt(e - 1) === 10) e--;
    if (e > (this.starts[r] as number) && text.charCodeAt(e - 1) === 13 && next !== e) e--;
    return e;
  }

  /** First old boundary index whose offset is >= pos (count if none). */
  #lowerBound(pos: number): number {
    let lo = 0;
    let hi = this.count;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if ((this.starts[mid] as number) < pos) lo = mid + 1;
      else hi = mid;
    }
    return lo;
  }

  /**
   * Takes one change of the text: `[from, to)` of the old text was replaced by `insLen`
   * characters, and `text` is the new text.
   */
  update(text: string, from: number, to: number, insLen: number): void {
    const delta = insLen - (to - from);
    const newLen = text.length;
    if (this.count === 0) {
      const all = RowIndex.scan(text, this.delim, this.quote);
      this.#adopt(all);
      return;
    }
    const first = this.recordAt(Math.min(from, Math.max(0, this.length - 1)));
    const fresh: number[] = [];
    const freshCols: number[] = [];
    let pos = this.starts[first] as number;
    const changeEndNew = from + insLen;
    let resync = this.count; // old record index from which the old index is kept
    while (pos < newLen) {
      const r = scanRecord(text, pos, this.delim, this.quote);
      fresh.push(pos);
      freshCols.push(r.cols);
      pos = r.next;
      if (pos >= changeEndNew) {
        const oldPos = pos - delta;
        if (oldPos >= to) {
          if (pos === newLen && oldPos === this.length) {
            resync = this.count;
            break;
          }
          const j = this.#lowerBound(oldPos);
          if (j < this.count && this.starts[j] === oldPos) {
            resync = j;
            break;
          }
        }
      }
    }
    this.#splice(first, resync, fresh, freshCols, delta);
    this.length = newLen;
  }

  #adopt(other: RowIndex) {
    this.starts = other.starts;
    this.cols = other.cols;
    this.count = other.count;
    this.length = other.length;
    this.hist.clear();
    for (const [k, v] of other.hist) this.hist.set(k, v);
  }

  /** Replaces old records `[a, b)` by the fresh ones and shifts the records after `b`. */
  #splice(a: number, b: number, fresh: number[], freshCols: number[], delta: number) {
    for (let i = a; i < b; i++) this.#bump(this.cols[i] as number, -1);
    const removed = b - a;
    const newCount = this.count - removed + fresh.length;
    if (newCount > this.starts.length || newCount < this.starts.length / 4) {
      const cap = Math.max(16, Math.ceil(newCount * 1.25));
      const starts = new Int32Array(cap);
      const cols = new Uint16Array(cap);
      starts.set(this.starts.subarray(0, a), 0);
      cols.set(this.cols.subarray(0, a), 0);
      starts.set(this.starts.subarray(b, this.count), a + fresh.length);
      cols.set(this.cols.subarray(b, this.count), a + fresh.length);
      this.starts = starts;
      this.cols = cols;
    } else if (fresh.length !== removed) {
      this.starts.copyWithin(a + fresh.length, b, this.count);
      this.cols.copyWithin(a + fresh.length, b, this.count);
    }
    for (let i = 0; i < fresh.length; i++) {
      this.starts[a + i] = fresh[i] as number;
      this.cols[a + i] = Math.min(freshCols[i] as number, 0xffff);
      this.#bump(freshCols[i] as number, 1);
    }
    if (delta !== 0) {
      const st = this.starts;
      for (let i = a + fresh.length; i < newCount; i++) st[i] = (st[i] as number) + delta;
    }
    this.count = newCount;
  }
}
