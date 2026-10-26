import { type Dialect, detectDialect } from "./dialect";
import { type Cell, rawCell, type Splice, splitRecord } from "./records";
import { RowIndex } from "./rowIndex";

/** One cell to write: its row is a record index of the text, its column a 0-based cell index. */
export interface CellEdit {
  r: number;
  c: number;
  value: string;
}

/** Applies splices given in the coordinates of `text`, highest position first or any order. */
export function applySplices(text: string, splices: readonly Splice[]): string {
  const sorted = [...splices].sort((a, b) => b.pos - a.pos);
  let out = text;
  for (const s of sorted) out = out.slice(0, s.pos) + s.ins + out.slice(s.pos + s.del);
  return out;
}

/** The single window of the text that a set of non-overlapping splices touches. */
export function windowOf(
  splices: readonly Splice[],
  newLen: number,
  oldLen: number,
): { from: number; to: number; insLen: number } {
  let from = Number.POSITIVE_INFINITY;
  let to = 0;
  for (const s of splices) {
    from = Math.min(from, s.pos);
    to = Math.max(to, s.pos + s.del);
  }
  if (!Number.isFinite(from)) return { from: 0, to: 0, insLen: 0 };
  return { from, to, insLen: to - from + (newLen - oldLen) };
}

/** `[start, end)` of text common to the old and new raw cell, trimmed off, as one splice. */
function minimalSplice(base: number, oldRaw: string, newRaw: string): Splice | undefined {
  if (oldRaw === newRaw) return undefined;
  const max = Math.min(oldRaw.length, newRaw.length);
  let p = 0;
  while (p < max && oldRaw.charCodeAt(p) === newRaw.charCodeAt(p)) p++;
  let s = 0;
  while (
    s < max - p &&
    oldRaw.charCodeAt(oldRaw.length - 1 - s) === newRaw.charCodeAt(newRaw.length - 1 - s)
  )
    s++;
  return {
    pos: base + p,
    del: oldRaw.length - p - s,
    ins: newRaw.slice(p, newRaw.length - s),
  };
}

/**
 * The CSV document as a table, read lazily from its text, with the edits as minimal text
 * splices (RFC 4180 quoting). The model owns no cells: a cell is parsed from its record when
 * asked, so memory stays proportional to the text and an edit costs the touched records.
 */
export class CsvModel {
  text: string;
  dialect: Dialect;
  index: RowIndex;
  /** Records before the table that are not data (Excel's `sep=;` line). */
  preamble: number;
  eol: "\n" | "\r\n";
  #cache = new Map<number, Cell[]>();
  /**
   * Quote every cell that is written. Two people replacing the same cell at once merge their
   * texts end to end; a plain run next to a quoted one (`a","`) would split the cell, while
   * two quoted runs (`"a"","`) always stay one cell. On while others are in the file.
   */
  safeQuoting = false;

  constructor(text: string, dialect?: Dialect, index?: RowIndex) {
    this.text = text;
    const sep = /^sep=(.)\r?\n/.exec(text);
    this.preamble = sep ? 1 : 0;
    this.dialect = dialect ?? detectDialect(sep ? text.slice(sep[0].length) : text);
    if (sep && !dialect) {
      const d = sep[1];
      if (d === "," || d === ";" || d === "\t" || d === "|") this.dialect.delimiter = d;
    }
    this.index = index ?? RowIndex.scan(text, this.dialect.delimiter, this.quoting);
    this.eol = text.includes("\r\n") ? "\r\n" : "\n";
  }

  /** Number of records, header and preamble included. */
  get records(): number {
    return this.index.count;
  }

  /** Records that are table rows (the preamble excluded), header included. */
  get rows(): number {
    return Math.max(0, this.index.count - this.preamble);
  }

  /** Number of columns of the table. */
  get width(): number {
    return this.index.maxCols;
  }

  /** The first record that is data, after the preamble and the header. */
  get dataStart(): number {
    return this.preamble + (this.dialect.header ? 1 : 0);
  }

  /** Cells can be quoted in this file. */
  get quoting(): boolean {
    return this.dialect.quote !== false;
  }

  get endsWithNewline(): boolean {
    return this.text.endsWith("\n");
  }

  /** Changes how the text is read (delimiter, header, decimal): rescans only if needed. */
  setDialect(next: Dialect): void {
    const rescan =
      next.delimiter !== this.dialect.delimiter || (next.quote !== false) !== this.quoting;
    this.dialect = next;
    this.#cache.clear();
    if (rescan) this.index = RowIndex.scan(this.text, next.delimiter, next.quote !== false);
  }

  start(r: number): number {
    return r < this.index.count ? (this.index.starts[r] as number) : this.text.length;
  }

  /** Cells of record `r` with their raw ranges. */
  record(r: number): Cell[] {
    if (r < 0 || r >= this.index.count) return [];
    const hit = this.#cache.get(r);
    if (hit) return hit;
    const cells = splitRecord(
      this.text,
      this.start(r),
      this.index.endOf(this.text, r),
      this.dialect.delimiter,
      Number.POSITIVE_INFINITY,
      this.quoting,
    );
    if (this.#cache.size > 400) this.#cache.clear();
    this.#cache.set(r, cells);
    return cells;
  }

  /** The cells up to column `c` only, uncached: for passes over many records. */
  recordUpTo(r: number, c: number): Cell[] {
    if (r < 0 || r >= this.index.count) return [];
    const hit = this.#cache.get(r);
    if (hit) return hit;
    return splitRecord(
      this.text,
      this.start(r),
      this.index.endOf(this.text, r),
      this.dialect.delimiter,
      c,
      this.quoting,
    );
  }

  /** One cell's value, parsing only as far as that cell when the record is not cached. */
  value(r: number, c: number): string {
    if (r < 0 || r >= this.index.count) return "";
    const hit = this.#cache.get(r);
    if (hit) return hit[c]?.value ?? "";
    const cells = splitRecord(
      this.text,
      this.start(r),
      this.index.endOf(this.text, r),
      this.dialect.delimiter,
      c,
      this.quoting,
    );
    return cells[c]?.value ?? "";
  }

  /** The text offset where cell (r, c) starts (or would start, for a missing cell). */
  offsetOf(r: number, c: number): number {
    const cells = this.record(r);
    const cell = cells[c];
    if (cell) return cell.start;
    return this.index.endOf(this.text, Math.min(r, this.index.count - 1));
  }

  /** The cell holding text offset `pos`. */
  locate(pos: number): { r: number; c: number } {
    if (this.index.count === 0) return { r: 0, c: 0 };
    const r = this.index.recordAt(pos);
    const cells = this.record(r);
    let c = 0;
    for (let i = 0; i < cells.length; i++) {
      if ((cells[i] as Cell).start <= pos) c = i;
      else break;
    }
    return { r, c };
  }

  /** Takes a change of the text (after the splices were applied) into the index. */
  update(newText: string, window: { from: number; to: number; insLen: number }): void {
    this.index.update(newText, window.from, window.to, window.insLen);
    this.text = newText;
    this.#cache.clear();
  }

  /** Replaces the whole text and rescans (when no precise change is known). */
  reset(newText: string): void {
    this.text = newText;
    this.index = RowIndex.scan(newText, this.dialect.delimiter, this.quoting);
    this.#cache.clear();
  }

  // ---- edits: each returns the splices of ONE user action, in the coordinates of `text` ----

  #rawFor(value: string, quoted: boolean): string {
    return rawCell(value, this.dialect.delimiter, quoted || this.safeQuoting, this.quoting);
  }

  #emptyRow(): string {
    return this.dialect.delimiter.repeat(Math.max(0, this.width - 1));
  }

  /** Appends `content` (records, each ending with a newline) at the end of the text. */
  #append(content: string): Splice {
    const len = this.text.length;
    const lead = len > 0 && !this.text.endsWith("\n") ? this.eol : "";
    return { pos: len, del: 0, ins: lead + content };
  }

  /** Writes cells. Cells that do not exist yet extend their record, or append records. */
  editCells(edits: readonly CellEdit[]): Splice[] {
    const byRow = new Map<number, Map<number, string>>();
    for (const e of edits) {
      let row = byRow.get(e.r);
      if (!row) {
        row = new Map();
        byRow.set(e.r, row);
      }
      row.set(e.c, e.value);
    }
    const rows = [...byRow.keys()].sort((a, b) => a - b);
    const out: Splice[] = [];
    const d = this.dialect.delimiter;
    let appended = "";
    let nextNew = this.index.count;
    for (const r of rows) {
      const cols = byRow.get(r) as Map<number, string>;
      if (r >= this.index.count) {
        // Records past the end: empty ones in between, then this one.
        while (nextNew < r) {
          appended += this.#emptyRow() + this.eol;
          nextNew++;
        }
        const cells: string[] = [];
        const width = Math.max(this.width, ...[...cols.keys()].map((c) => c + 1));
        for (let c = 0; c < width; c++)
          cells.push(rawCell(cols.get(c) ?? "", d, this.safeQuoting && cols.has(c), this.quoting));
        appended += cells.join(d) + this.eol;
        nextNew = r + 1;
        continue;
      }
      const cells = this.record(r);
      let extra = "";
      const maxC = Math.max(...cols.keys());
      for (const [c, value] of [...cols].sort((a, b) => a[0] - b[0])) {
        const cell = cells[c];
        if (cell) {
          const raw = this.#rawFor(value, cell.quoted);
          const old = this.text.slice(cell.start, cell.end);
          const s = minimalSplice(cell.start, old, raw);
          if (s) out.push(s);
        }
      }
      if (maxC >= cells.length) {
        for (let c = cells.length; c <= maxC; c++)
          extra += d + rawCell(cols.get(c) ?? "", d, this.safeQuoting && cols.has(c), this.quoting);
        out.push({ pos: this.index.endOf(this.text, r), del: 0, ins: extra });
      }
    }
    if (appended) out.push(this.#append(appended));
    return out.sort((a, b) => b.pos - a.pos);
  }

  editCell(r: number, c: number, value: string): Splice[] {
    return this.editCells([{ r, c, value }]);
  }

  /** Empty rows before record `at` (or at the end). */
  insertRows(at: number, n = 1): Splice[] {
    const block = (this.#emptyRow() + this.eol).repeat(n);
    if (at >= this.index.count) return [this.#append(block)];
    return [{ pos: this.start(at), del: 0, ins: block }];
  }

  /** A copy of record `at`, right below it. */
  duplicateRow(at: number): Splice[] {
    const line = this.text.slice(this.start(at), this.index.endOf(this.text, at));
    if (at + 1 >= this.index.count) return [this.#append(line + this.eol)];
    return [{ pos: this.start(at + 1), del: 0, ins: line + this.eol }];
  }

  deleteRows(at: number, n = 1): Splice[] {
    const last = Math.min(this.index.count, at + n);
    if (at >= last) return [];
    const from = this.start(at);
    const to = last < this.index.count ? this.start(last) : this.text.length;
    // Removing the final records leaves the previous one's terminator as the text's end.
    return [{ pos: from, del: to - from, ins: "" }];
  }

  insertCols(at: number, n = 1): Splice[] {
    const d = this.dialect.delimiter.repeat(n);
    const out: Splice[] = [];
    for (let r = this.preamble; r < this.index.count; r++) {
      const cells = this.record(r);
      if (at < cells.length) out.push({ pos: (cells[at] as Cell).start, del: 0, ins: d });
      else if (at === cells.length)
        out.push({ pos: this.index.endOf(this.text, r), del: 0, ins: d });
    }
    return out.reverse();
  }

  deleteCols(at: number, n = 1): Splice[] {
    const out: Splice[] = [];
    for (let r = this.preamble; r < this.index.count; r++) {
      const cells = this.record(r);
      if (at >= cells.length) continue;
      const stop = Math.min(at + n, cells.length);
      if (stop < cells.length) {
        const from = (cells[at] as Cell).start;
        out.push({ pos: from, del: (cells[stop] as Cell).start - from, ins: "" });
      } else if (at > 0) {
        const from = (cells[at - 1] as Cell).end;
        out.push({ pos: from, del: (cells[cells.length - 1] as Cell).end - from, ins: "" });
      } else {
        const from = (cells[0] as Cell).start;
        out.push({ pos: from, del: (cells[cells.length - 1] as Cell).end - from, ins: "" });
      }
    }
    return out.reverse();
  }

  /** Moves `n` records starting at `from` so that they sit before record `to`. */
  moveRows(from: number, n: number, to: number): Splice[] {
    const end = Math.min(this.index.count, from + n);
    if (to >= from && to <= end) return [];
    const a = this.start(from);
    const b = end < this.index.count ? this.start(end) : this.text.length;
    let cut = this.text.slice(a, b);
    if (!cut.endsWith("\n")) cut += this.eol;
    const out: Splice[] = [{ pos: a, del: b - a, ins: "" }];
    if (to >= this.index.count) out.push(this.#append(cut));
    else out.push({ pos: this.start(to), del: 0, ins: cut });
    return out.sort((x, y) => y.pos - x.pos);
  }

  /** Moves `n` columns starting at `from` so that they sit before column `to`. */
  moveCols(from: number, n: number, to: number): Splice[] {
    if (to >= from && to <= from + n) return [];
    const out: Splice[] = [];
    const d = this.dialect.delimiter;
    for (let r = this.preamble; r < this.index.count; r++) {
      const cells = this.record(r);
      if (cells.length <= from) continue;
      const raws = cells.map((c) => this.text.slice(c.start, c.end));
      const moving = raws.splice(from, n);
      const target = to > from ? to - moving.length : to;
      raws.splice(Math.min(target, raws.length), 0, ...moving);
      const s = minimalSplice(
        (cells[0] as Cell).start,
        this.text.slice((cells[0] as Cell).start, (cells[cells.length - 1] as Cell).end),
        raws.join(d),
      );
      if (s) out.push(s);
    }
    return out.reverse();
  }

  /** Adds empty cells at the end of the records narrower than `width`, except those in `skip`. */
  widen(width: number, skip: ReadonlySet<number> = new Set()): Splice[] {
    const out: Splice[] = [];
    const d = this.dialect.delimiter;
    for (let r = this.preamble; r < this.index.count; r++) {
      const have = this.index.cols[r] as number;
      if (have >= width || skip.has(r) || have < this.index.modalCols) continue;
      out.push({ pos: this.index.endOf(this.text, r), del: 0, ins: d.repeat(width - have) });
    }
    return out.reverse();
  }

  /** Rewrites the whole table with another delimiter (cells quoted as they were, or as needed). */
  convertDelimiter(to: Dialect["delimiter"]): Splice[] {
    if (to === this.dialect.delimiter || this.index.count === 0) return [];
    const lines: string[] = [];
    for (let r = 0; r < this.index.count; r++) {
      if (r < this.preamble) {
        lines.push(`sep=${to}`);
        continue;
      }
      lines.push(
        this.record(r)
          .map((c) => rawCell(c.value, to, c.quoted, this.quoting))
          .join(to),
      );
    }
    const tail = this.endsWithNewline ? this.eol : "";
    return [{ pos: 0, del: this.text.length, ins: lines.join(this.eol) + tail }];
  }

  /**
   * Reorders the data records (everything after the header) as `order` says. The rows that
   * already keep their relative order stay where they are; only the others are cut and written
   * again at their place. So an edit someone makes in a row that stays is untouched, and text
   * that lands in a moved row's old place joins its neighbour instead of making a stray line.
   * A very large reorder is one splice instead (a splice per row would take longer than the
   * reordering is worth).
   */
  applySort(order: ArrayLike<number>): Splice[] {
    const first = this.dataStart;
    const count = this.index.count;
    if (first >= count) return [];
    let identity = true;
    for (let i = 0; i < order.length; i++) {
      if (order[i] !== first + i) {
        identity = false;
        break;
      }
    }
    if (identity) return [];
    const n = order.length;

    // The longest run of rows already in order stays put (patience sorting, O(n log n)).
    const tails: number[] = []; // index in `order` ending the best run of each length
    const prev = new Int32Array(n).fill(-1);
    for (let i = 0; i < n; i++) {
      let lo = 0;
      let hi = tails.length;
      while (lo < hi) {
        const mid = (lo + hi) >> 1;
        if ((order[tails[mid] as number] as number) < (order[i] as number)) lo = mid + 1;
        else hi = mid;
      }
      if (lo > 0) prev[i] = tails[lo - 1] as number;
      tails[lo] = i;
    }
    const stay = new Uint8Array(n);
    for (let k = tails[tails.length - 1] ?? -1; k >= 0; k = prev[k] as number) stay[k] = 1;
    let moved = 0;
    for (let i = 0; i < n; i++) if (!stay[i]) moved++;

    const line = (r: number) => this.text.slice(this.start(r), this.index.endOf(this.text, r));
    if (moved > 2000) {
      const lines: string[] = [];
      for (let i = 0; i < n; i++) lines.push(line(order[i] as number));
      const a = this.start(first);
      const tail = this.endsWithNewline ? this.eol : "";
      return [{ pos: a, del: this.text.length - a, ins: lines.join(this.eol) + tail }];
    }

    const out: Splice[] = [];
    const eol = this.eol;
    // Cut every moved row (its line and terminator).
    for (let i = 0; i < n; i++) {
      if (stay[i]) continue;
      const r = order[i] as number;
      const from = this.start(r);
      const to = r + 1 < count ? this.start(r + 1) : this.text.length;
      out.push({ pos: from, del: to - from, ins: "" });
    }
    // A text without a final newline needs one before what is appended, unless its last row
    // is itself moved (its cut already takes the text up to the end).
    const lastMoved =
      order.length > 0 && Array.from(order).some((r, k) => r === count - 1 && !stay[k]);
    const lead = !this.endsWithNewline && !lastMoved ? eol : "";
    // Write each run of moved rows before the next row that stays, or at the end.
    let i = 0;
    while (i < n) {
      if (stay[i]) {
        i++;
        continue;
      }
      let j = i;
      let block = "";
      while (j < n && !stay[j]) {
        block += line(order[j] as number) + eol;
        j++;
      }
      if (j < n) out.push({ pos: this.start(order[j] as number), del: 0, ins: block });
      else out.push({ pos: this.text.length, del: 0, ins: lead + block });
      i = j;
    }
    // Several splices at one position keep the order they were listed in.
    return out.sort((x, y) => y.pos - x.pos);
  }
}
