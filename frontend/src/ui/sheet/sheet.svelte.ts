import { colName, parseRef, rangeRef } from "../../editor/csv/address";
import {
  type CellHit,
  type CellScope,
  findInCells,
  type ReplaceCellOptions,
  replacedValue,
} from "../../editor/csv/cellSearch";
import {
  fromClipboard,
  neutralise,
  type Range,
  sizeOf,
  toHtml,
  toTsv,
} from "../../editor/csv/clipboard";
import { columnType, type Dialect } from "../../editor/csv/dialect";
import type { CsvModel } from "../../editor/csv/model";
import { findProblems, type Problem } from "../../editor/csv/problems";
import type { Splice } from "../../editor/csv/records";
import { continueSeries, today } from "../../editor/csv/series";
import type { SheetDoc } from "../../editor/csv/sheetDoc";
import { displayNumber, type SelectionStats, selectionStats } from "../../editor/csv/stats";
import {
  buildView,
  type ColumnFilter,
  isIdentity,
  NO_VIEW,
  type ViewSpec,
} from "../../editor/csv/view";
import { compileQuery, type QuerySpec } from "../../editor/search/query";
import { megabytesOf, t } from "../../i18n";
import { MAX_GROWN_CHARS } from "../../store/limits";

/** A cell of the grid: `r` counts data rows as displayed (after sort and filter), `c` columns. */
export interface Pos {
  r: number;
  c: number;
}

/**
 * Where a fill drag from the selection `from` to the cell `to` goes: in the direction of the
 * larger move, `count` cells further. `range` is the selection once filled.
 */
export function fillExtent(
  from: Range,
  to: Pos,
): { vertical: boolean; dir: 1 | -1; count: number; range: Range } | undefined {
  const dRows = to.r > from.r1 ? to.r - from.r1 : to.r < from.r0 ? to.r - from.r0 : 0;
  const dCols = to.c > from.c1 ? to.c - from.c1 : to.c < from.c0 ? to.c - from.c0 : 0;
  if (dRows === 0 && dCols === 0) return undefined;
  const vertical = Math.abs(dRows) >= Math.abs(dCols);
  const move = vertical ? dRows : dCols;
  const dir = move > 0 ? 1 : -1;
  const count = Math.abs(move);
  const range = vertical
    ? {
        ...from,
        r0: dir === -1 ? from.r0 - count : from.r0,
        r1: dir === 1 ? from.r1 + count : from.r1,
      }
    : {
        ...from,
        c0: dir === -1 ? from.c0 - count : from.c0,
        c1: dir === 1 ? from.c1 + count : from.c1,
      };
  return { vertical, dir, count, range };
}

/** Another person's selection, already in grid coordinates. */
export interface Remote {
  peerId: string;
  name: string;
  range: Range;
}

export interface SheetOptions {
  readOnly?: boolean;
  /** Record/column coordinates of a cell in a locked zone (ADR 0022). */
  isLocked?: (record: number, col: number) => boolean;
  /**
   * Asked before undoing: true says the key was used for something else (an outside edit that
   * was just taken in is taken back instead of the person's own last action).
   */
  onUndoKey?: () => boolean;
  /** The person set the dialect (separator, header, decimal) by hand. */
  onDialect?: (dialect: Dialect) => void;
  now?: () => Date;
}

const BASE_ROW = 28;
const BASE_COL = 140;
const BASE_HEADER_W = 52;
const MIN_ZOOM = 0.5;
const MAX_ZOOM = 2;

const MAX_WIDTH = 5000;

/** What an action tells the person (a toast with an optional way back). */
export interface Notice {
  text: string;
  action?: { label: string; run: () => void };
}

/**
 * The state of one open sheet: selection, edit, view (sort, filter), geometry, and every
 * action, each of which becomes ONE change of the text. The grid only draws this and forwards
 * keys and pointers.
 */
export class SheetController {
  /** Bumped by every change of the text: reading it subscribes a derived value to the data. */
  tick = $state(0);
  zoom = $state(1);
  widths = $state.raw<Record<number, number>>({});
  pinnedRows = $state(0);
  pinnedCols = $state(0);
  active = $state<Pos>({ r: 0, c: 0 });
  /** The selected ranges; the last one is the one the active cell belongs to. */
  ranges = $state.raw<Range[]>([{ r0: 0, c0: 0, r1: 0, c1: 0 }]);
  editing = $state<
    | { r: number; c: number; value: string; from: "cell" | "bar"; anchor?: string | undefined }
    | undefined
  >();
  view = $state.raw<ViewSpec>(NO_VIEW);
  rows = $state.raw<Int32Array | undefined>();
  remotes = $state.raw<Remote[]>([]);
  /** Cells that another person (or the disk) changed lately, as "record:col". */
  changed = $state.raw<ReadonlySet<string>>(new Set());
  notice = $state.raw<Notice | undefined>();
  /** A paste waiting for the person's yes (more than 1 000 cells). */
  pending = $state.raw<{ rows: string[][]; at: Pos } | undefined>();
  /** Quoting oddities in the file, found when idle (never repaired). */
  problems = $state.raw<Problem[]>([]);
  /** Statistics shown in the status bar, computed when idle. */
  stats = $state.raw<SelectionStats | undefined>();
  /** Which statistics the status bar shows. */
  shown = $state({ count: true, sum: true, mean: true, min: false, max: false });
  /** The last cut: pasting the same text moves the cells instead of copying them. */
  #cut: { text: string; range: Range } | undefined;
  #types = new Map<number, ReturnType<typeof columnType>>();
  #shown = new Map<string, string>();
  #inv: Int32Array | undefined;
  #recordsSeen = 0;

  readonly model: CsvModel;

  constructor(
    readonly doc: SheetDoc,
    readonly opts: SheetOptions = {},
  ) {
    this.model = doc.model;
    this.#recordsSeen = this.model.records;
    this.#scheduleProblems();
    doc.onChange = (w, remote) => this.#changed(w, remote);
    doc.onRefused = (chars) => {
      this.notice = {
        text: t("limit.pasteRefused", {
          size: megabytesOf(chars),
          max: megabytesOf(MAX_GROWN_CHARS),
        }),
      };
    };
    doc.onMoved = () => {
      this.notice = {
        text: t("sheet.lostEdit"),
        action: { label: t("sheet.undo"), run: () => this.undo() },
      };
    };
  }

  // ---- geometry and reading ----

  get rowH(): number {
    return Math.round(BASE_ROW * this.zoom);
  }
  get headerW(): number {
    return Math.round(BASE_HEADER_W * this.zoom);
  }
  get dialect(): Dialect {
    this.tick;
    return this.model.dialect;
  }
  get readOnly(): boolean {
    this.tick;
    return this.opts.readOnly === true || this.doc.tooBig;
  }
  /** Why the sheet cannot be edited: the file is too big, or the person may only read. */
  get readOnlyText(): string {
    this.tick;
    if (!this.doc.tooBig) return t("sheet.readOnly");
    return t("limit.tooBig", { size: megabytesOf(this.model.text.length) });
  }
  /** Displayed data rows. */
  get rowCount(): number {
    this.tick;
    return this.rows ? this.rows.length : Math.max(0, this.model.records - this.model.dataStart);
  }
  get colCount(): number {
    this.tick;
    return Math.max(1, this.model.width);
  }
  colW(c: number): number {
    return Math.round((this.widths[c] ?? BASE_COL) * this.zoom);
  }
  get filtered(): boolean {
    return this.view.filters.size > 0;
  }
  get sorted(): boolean {
    return this.view.sort !== undefined;
  }
  /** Edits that change rows or their order are refused while a view is active. */
  get structureLocked(): boolean {
    return !isIdentity(this.view);
  }

  recordOf(d: number): number {
    return this.rows ? (this.rows[d] ?? this.model.records) : this.model.dataStart + d;
  }

  displayOf(record: number): number | undefined {
    if (!this.rows) {
      const d = record - this.model.dataStart;
      return d >= 0 && d < this.rowCount ? d : undefined;
    }
    if (!this.#inv) {
      const inv = new Int32Array(this.model.records + 1).fill(-1);
      for (let i = 0; i < this.rows.length; i++) inv[this.rows[i] as number] = i;
      this.#inv = inv;
    }
    const d = this.#inv[record] ?? -1;
    return d >= 0 ? d : undefined;
  }

  /** Text of the cell at a displayed position. */
  value(d: number, c: number): string {
    this.tick;
    // The whole record, which the model keeps: a screen reads each record once, not once per cell.
    return this.model.record(this.recordOf(d))[c]?.value ?? "";
  }

  /** The column's name: the header, or "Colonne C" when empty, or its letter without a header. */
  headerName(c: number): string {
    this.tick;
    if (!this.model.dialect.header) return colName(c);
    const v = this.model.value(this.model.preamble, c);
    return v === "" ? t("sheet.column", { name: colName(c) }) : v;
  }

  typeOf(c: number) {
    this.tick;
    let ty = this.#types.get(c);
    if (!ty) {
      const sample: string[] = [];
      const n = Math.min(this.rowCount, 200);
      for (let d = 0; d < n; d++) sample.push(this.model.value(this.recordOf(d), c));
      ty = columnType(sample, this.model.dialect.decimal);
      this.#types.set(c, ty);
    }
    return ty;
  }

  /** How a cell is shown: numbers in French form, the text as stored otherwise. */
  display(d: number, c: number): string {
    const v = this.value(d, c);
    if (v === "" || this.typeOf(c) !== "number") return v;
    const key = `${this.model.dialect.decimal}${v}`;
    let shown = this.#shown.get(key);
    if (shown === undefined) {
      if (this.#shown.size > 4000) this.#shown.clear();
      shown = displayNumber(v, this.model.dialect.decimal);
      this.#shown.set(key, shown);
    }
    return shown;
  }

  /** The number shown for a row: its place in the file, so a filtered view keeps the real ones. */
  rowNumber(d: number): number {
    return this.recordOf(d) - this.model.preamble + 1;
  }

  /** "B12" or "B12:D20" for the selection, with the file's own row numbers. */
  get reference(): string {
    const p = this.primary;
    const n = (d: number) => this.rowNumber(Math.min(d, Math.max(0, this.rowCount - 1))) - 1;
    return rangeRef(n(p.r0), p.c0, n(p.r1), p.c1);
  }

  get primary(): Range {
    return this.ranges[this.ranges.length - 1] as Range;
  }

  isLocked(d: number, c: number): boolean {
    return this.opts.isLocked?.(this.recordOf(d), c) === true;
  }

  canEdit(d: number, c: number): boolean {
    return !this.readOnly && !this.isLocked(d, c);
  }

  // ---- reactions to changes of the text ----

  #changed(w: { from: number; to: number; insLen: number }, remote: boolean) {
    this.#types.clear();
    const structural = this.model.records !== this.#recordsSeen;
    this.#recordsSeen = this.model.records;
    if (this.rows && (structural || remote)) this.#rebuildView();
    this.tick++;
    if (remote) this.#highlight(w);
    this.#clamp();
    this.#scheduleStats();
    this.#scheduleProblems();
  }

  #highlight(w: { from: number; to: number; insLen: number }) {
    const m = this.model;
    // A whole new content (a file that just arrived) is not "a change": nothing to point at.
    if (m.records === 0 || w.insLen === 0 || w.insLen > 4000) return;
    const a = m.locate(w.from);
    const b = m.locate(Math.min(w.from + w.insLen, m.text.length));
    const out = new Set<string>();
    for (let r = a.r; r <= Math.min(b.r, a.r + 200); r++) {
      const cells = m.record(r);
      for (let c = 0; c < cells.length; c++) {
        const cell = cells[c];
        if (cell && cell.end >= w.from && cell.start <= w.from + w.insLen) out.add(`${r}:${c}`);
      }
    }
    this.changed = out;
    setTimeout(() => {
      if (this.changed === out) this.changed = new Set();
    }, 4000);
  }

  #rebuildView() {
    this.#inv = undefined;
    this.rows = isIdentity(this.view) ? undefined : buildView(this.model, this.view);
  }

  #clamp() {
    const rows = Math.max(1, this.rowCount);
    const cols = this.colCount;
    const fix = (r: Range): Range => ({
      r0: Math.min(r.r0, rows - 1),
      r1: Math.min(r.r1, rows - 1),
      c0: Math.min(r.c0, cols - 1),
      c1: Math.min(r.c1, cols - 1),
    });
    this.ranges = this.ranges.map(fix);
    this.active = {
      r: Math.min(this.active.r, rows - 1),
      c: Math.min(this.active.c, cols - 1),
    };
  }

  #problemsTimer: ReturnType<typeof setTimeout> | undefined;
  #scheduleProblems() {
    clearTimeout(this.#problemsTimer);
    this.#problemsTimer = setTimeout(() => {
      this.problems = findProblems(this.model);
    }, 400);
  }

  /** How sure the detection of the separator is: a regular table of two columns or more. */
  get confidence(): "high" | "low" {
    this.tick;
    const idx = this.model.index;
    return idx.modalCols >= 2 && idx.count >= 3 && idx.uneven / idx.count <= 0.1 ? "high" : "low";
  }

  #statsTimer: ReturnType<typeof setTimeout> | undefined;
  #scheduleStats() {
    clearTimeout(this.#statsTimer);
    this.#statsTimer = setTimeout(() => this.computeStats(), 60);
  }

  computeStats() {
    const cells = this.ranges.reduce((n, r) => n + (r.r1 - r.r0 + 1) * (r.c1 - r.c0 + 1), 0);
    if (cells <= 1 || this.rowCount === 0) {
      this.stats = undefined;
      return;
    }
    // Statistics go by record: a view maps displayed rows to records.
    const out: Range[] = [];
    for (const r of this.ranges) {
      if (!this.rows) out.push({ ...r, r0: this.recordOf(r.r0), r1: this.recordOf(r.r1) });
      else
        for (let d = r.r0; d <= Math.min(r.r1, r.r0 + 200_000); d++)
          out.push({ r0: this.recordOf(d), r1: this.recordOf(d), c0: r.c0, c1: r.c1 });
    }
    this.stats = selectionStats(this.model, out);
  }

  // ---- selection ----

  select(r: number, c: number, extend = false, add = false) {
    const rows = this.rowCount;
    r = Math.max(0, Math.min(rows - 1, r));
    c = Math.max(0, Math.min(this.colCount - 1, c));
    if (extend) {
      const a = this.active;
      const range = {
        r0: Math.min(a.r, r),
        r1: Math.max(a.r, r),
        c0: Math.min(a.c, c),
        c1: Math.max(a.c, c),
      };
      this.ranges = [...this.ranges.slice(0, -1), range];
    } else {
      this.active = { r, c };
      const range = { r0: r, r1: r, c0: c, c1: c };
      this.ranges = add ? [...this.ranges, range] : [range];
    }
    this.#scheduleStats();
  }

  /** Moves the active cell; with `extend` the selection grows from where it started. */
  move(dr: number, dc: number, extend = false) {
    if (extend) {
      const p = this.primary;
      const a = this.active;
      // The moving corner is the one opposite the active cell.
      const fr = p.r0 === a.r ? p.r1 : p.r0;
      const fc = p.c0 === a.c ? p.c1 : p.c0;
      const nr = Math.max(0, Math.min(this.rowCount - 1, fr + dr));
      const nc = Math.max(0, Math.min(this.colCount - 1, fc + dc));
      this.ranges = [
        ...this.ranges.slice(0, -1),
        {
          r0: Math.min(a.r, nr),
          r1: Math.max(a.r, nr),
          c0: Math.min(a.c, nc),
          c1: Math.max(a.c, nc),
        },
      ];
      this.#scheduleStats();
    } else {
      this.select(this.active.r + dr, this.active.c + dc);
    }
  }

  /** Ctrl+arrow: to the edge of the block of filled cells, or the next filled cell. */
  edge(dr: number, dc: number, extend = false) {
    const p = this.primary;
    const a = this.active;
    let r = extend ? (p.r0 === a.r ? p.r1 : p.r0) : a.r;
    let c = extend ? (p.c0 === a.c ? p.c1 : p.c0) : a.c;
    const filled = (rr: number, cc: number) => this.value(rr, cc) !== "";
    const inside = (rr: number, cc: number) =>
      rr >= 0 && rr < this.rowCount && cc >= 0 && cc < this.colCount;
    const next = (rr: number, cc: number) => ({ r: rr + dr, c: cc + dc });
    let n = next(r, c);
    if (!inside(n.r, n.c)) return;
    if (filled(r, c) && filled(n.r, n.c)) {
      while (inside(n.r, n.c) && filled(n.r, n.c)) {
        r = n.r;
        c = n.c;
        n = next(r, c);
      }
    } else {
      r = n.r;
      c = n.c;
      let guard = 0;
      while (inside(r + dr, c + dc) && !filled(r, c) && guard++ < 2_000_000) {
        r += dr;
        c += dc;
      }
    }
    if (extend) this.move(r - (p.r0 === a.r ? p.r1 : p.r0), c - (p.c0 === a.c ? p.c1 : p.c0), true);
    else this.select(r, c);
  }

  selectColumns(c0: number, c1: number, add = false) {
    const range = {
      r0: 0,
      r1: Math.max(0, this.rowCount - 1),
      c0: Math.min(c0, c1),
      c1: Math.max(c0, c1),
    };
    this.active = { r: 0, c: c0 };
    this.ranges = add ? [...this.ranges, range] : [range];
    this.#scheduleStats();
  }

  selectRows(r0: number, r1: number, add = false) {
    const range = { r0: Math.min(r0, r1), r1: Math.max(r0, r1), c0: 0, c1: this.colCount - 1 };
    this.active = { r: r0, c: 0 };
    this.ranges = add ? [...this.ranges, range] : [range];
    this.#scheduleStats();
  }

  selectAll() {
    this.ranges = [{ r0: 0, r1: Math.max(0, this.rowCount - 1), c0: 0, c1: this.colCount - 1 }];
    this.#scheduleStats();
  }

  get wholeColumns(): boolean {
    const p = this.primary;
    return p.r0 === 0 && p.r1 >= this.rowCount - 1 && this.rowCount > 0;
  }
  get wholeRows(): boolean {
    const p = this.primary;
    return p.c0 === 0 && p.c1 >= this.colCount - 1;
  }

  jumpTo(ref: string): boolean {
    const p = parseRef(ref);
    if (!p) return false;
    const d0 = this.displayOf(p.r0 + this.model.preamble);
    const d1 = this.displayOf(p.r1 + this.model.preamble);
    if (d0 === undefined || d1 === undefined) return false;
    this.active = { r: d0, c: Math.min(p.c0, this.colCount - 1) };
    this.ranges = [{ r0: d0, r1: d1, c0: p.c0, c1: Math.min(p.c1, this.colCount - 1) }];
    this.#scheduleStats();
    return true;
  }

  /** The text offset of the active cell, to keep the place when switching to the text view. */
  get textOffset(): number {
    return this.model.offsetOf(this.recordOf(this.active.r), this.active.c);
  }

  /** Puts the active cell where a text offset is (switching back from the text view). */
  placeAtOffset(pos: number) {
    const { r, c } = this.model.locate(pos);
    const d = this.displayOf(r);
    if (d === undefined) return;
    this.select(d, c);
  }

  // ---- editing ----

  startEdit(replace?: string, from: "cell" | "bar" = "cell") {
    const { r, c } = this.active;
    if (!this.canEdit(r, c)) {
      this.#refuse();
      return;
    }
    const rec = this.recordOf(r);
    this.editing = {
      r,
      c,
      value: replace ?? this.value(r, c),
      from,
      anchor: this.#anchorOf(rec),
    };
  }

  /** Renames a column header in place. */
  startHeaderEdit(c: number) {
    if (this.readOnly || !this.model.dialect.header) return;
    this.editing = { r: -1, c, value: this.model.value(this.model.preamble, c), from: "cell" };
  }

  #anchorOf(record: number): string | undefined {
    try {
      const pos = this.model.start(record);
      return pos >= this.model.text.length ? undefined : this.doc.port.cursor(pos);
    } catch {
      return undefined;
    }
  }

  /** The record an edit began on, found again through its anchor after others' changes. */
  #recordOfEdit(e: NonNullable<SheetController["editing"]>): number | undefined {
    if (e.r < 0) return this.model.preamble;
    const fallback = this.recordOf(e.r);
    if (!e.anchor) return fallback;
    try {
      const pos = this.doc.port.resolve(e.anchor);
      const r = this.model.index.recordAt(pos);
      return this.model.start(r) === pos ? r : fallback;
    } catch {
      return fallback;
    }
  }

  setEditValue(value: string) {
    if (this.editing) this.editing = { ...this.editing, value };
  }

  /** Writes the edit. `move` is where the active cell goes next: Enter down, Tab right. */
  commitEdit(move?: { dr: number; dc: number }) {
    const e = this.editing;
    if (!e) return;
    this.editing = undefined;
    const rec = this.#recordOfEdit(e);
    if (rec !== undefined && e.value !== this.model.value(rec, e.c)) {
      this.doc.perform(this.model.editCell(rec, e.c, e.value), e.c);
    }
    if (move) this.#moveInSelection(move.dr, move.dc);
  }

  cancelEdit() {
    this.editing = undefined;
  }

  /** Enter and Tab: inside a selection of several cells they cycle within it. */
  #moveInSelection(dr: number, dc: number) {
    const p = this.primary;
    const multi = p.r0 !== p.r1 || p.c0 !== p.c1;
    if (!multi) {
      this.select(this.active.r + dr, this.active.c + dc);
      return;
    }
    let { r, c } = this.active;
    if (dr !== 0) {
      r += dr;
      if (r > p.r1 || r < p.r0) {
        r = dr > 0 ? p.r0 : p.r1;
        c += dr;
        if (c > p.c1) c = p.c0;
        if (c < p.c0) c = p.c1;
      }
    } else {
      c += dc;
      if (c > p.c1 || c < p.c0) {
        c = dc > 0 ? p.c0 : p.c1;
        r += dc;
        if (r > p.r1) r = p.r0;
        if (r < p.r0) r = p.r1;
      }
    }
    this.active = { r, c };
  }

  #refuse() {
    this.notice = { text: this.readOnly ? this.readOnlyText : t("sheet.locked") };
  }

  /** Enter and Tab outside an edit: the next cell, cycling inside a multi-cell selection. */
  walk(dr: number, dc: number) {
    const p = this.primary;
    if (p.r0 === p.r1 && p.c0 === p.c1) this.select(this.active.r + dr, this.active.c + dc);
    else this.#moveInSelection(dr, dc);
  }

  /** Ctrl++: new columns when whole columns are selected, otherwise new rows above. */
  insertAtSelection() {
    if (this.wholeColumns && !this.wholeRows) this.insertCols("left");
    else this.insertRows("above");
  }

  /** Ctrl+-: delete the selected columns when whole columns are selected, otherwise rows. */
  deleteAtSelection() {
    if (this.wholeColumns && !this.wholeRows) this.deleteCols();
    else this.deleteRows();
  }

  // ---- writing ranges ----

  /** Every cell of the selected ranges (displayed coordinates). */
  #cellsIn(ranges: readonly Range[]): Pos[] {
    const out: Pos[] = [];
    for (const g of ranges)
      for (let r = g.r0; r <= g.r1; r++) for (let c = g.c0; c <= g.c1; c++) out.push({ r, c });
    return out;
  }

  /** Writes values at displayed positions as one action. Refuses locked and read-only cells. */
  #write(
    items: { r: number; c: number; value: string }[],
    extra: Splice[] = [],
    col?: number,
  ): boolean {
    if (this.readOnly) {
      this.#refuse();
      return false;
    }
    let ok = items.filter((i) => !this.isLocked(i.r, i.c));
    ok = this.#plain(ok);
    if (ok.length < items.length)
      this.notice = { text: t("sheet.lockedSkipped", { count: items.length - ok.length }) };
    if (ok.length === 0 && extra.length === 0) return false;
    const edits = ok.map((i) => ({ r: this.#recFor(i.r), c: i.c, value: i.value }));
    const splices = [...this.model.editCells(edits), ...extra];
    return this.doc.perform(splices, col);
  }

  /**
   * In a file without quotes a separator or a line break cannot be written inside a cell:
   * those become a space, and the person is told, never silently.
   */
  #plain<T extends { value: string }>(items: T[]): T[] {
    if (this.model.quoting) return items;
    const d = this.model.dialect.delimiter;
    let changed = 0;
    const out = items.map((i) => {
      if (!i.value.includes(d) && !/[\r\n]/.test(i.value)) return i;
      changed++;
      return {
        ...i,
        value: i.value
          .split(d)
          .join(" ")
          .replace(/\r\n|\r|\n/g, " "),
      };
    });
    if (changed > 0) this.notice = { text: t("sheet.noQuoteLossy", { count: changed }) };
    return out;
  }

  /** Record of a displayed row; rows past the end continue the file. */
  #recFor(d: number): number {
    return d < this.rowCount ? this.recordOf(d) : this.model.dataStart + d;
  }

  clear() {
    const items = this.#cellsIn(this.ranges).map((p) => ({ ...p, value: "" }));
    this.#cut = undefined;
    this.#write(items);
  }

  fillDown() {
    this.#fillBy(1, 0);
  }
  fillRight() {
    this.#fillBy(0, 1);
  }

  #fillBy(dr: number, dc: number) {
    const p = this.primary;
    const items: { r: number; c: number; value: string }[] = [];
    for (let r = p.r0; r <= p.r1; r++)
      for (let c = p.c0; c <= p.c1; c++) {
        if (dr && r > p.r0) items.push({ r, c, value: this.value(p.r0, c) });
        if (dc && c > p.c0) items.push({ r, c, value: this.value(r, p.c0) });
      }
    this.#write(items);
  }

  /** Ctrl+; */
  insertToday() {
    const { r, c } = this.active;
    if (!this.canEdit(r, c)) return this.#refuse();
    this.#write([{ r, c, value: today(this.opts.now?.() ?? new Date()) }]);
  }

  /**
   * The fill handle: the selection `from` is continued (series, or a copy) to `to`, in the
   * direction of the larger move. Returns the range now selected.
   */
  fillHandle(from: Range, to: Pos): Range | undefined {
    const extent = fillExtent(from, to);
    if (!extent) return undefined;
    const { vertical, dir, count, range } = extent;
    const decimal = this.model.dialect.decimal;
    const items: { r: number; c: number; value: string }[] = [];
    if (vertical) {
      for (let c = from.c0; c <= from.c1; c++) {
        const seed: string[] = [];
        for (let r = from.r0; r <= from.r1; r++) seed.push(this.value(r, c));
        const out = continueSeries(seed, count, dir, decimal);
        out.forEach((value, i) => {
          const r = dir === 1 ? from.r1 + 1 + i : from.r0 - 1 - i;
          if (r >= 0) items.push({ r, c, value });
        });
      }
    } else {
      for (let r = from.r0; r <= from.r1; r++) {
        const seed: string[] = [];
        for (let c = from.c0; c <= from.c1; c++) seed.push(this.value(r, c));
        const out = continueSeries(seed, count, dir, decimal);
        out.forEach((value, i) => {
          const c = dir === 1 ? from.c1 + 1 + i : from.c0 - 1 - i;
          if (c >= 0 && c < this.colCount) items.push({ r, c, value });
        });
      }
    }
    if (!this.#write(items)) return undefined;
    this.active = { r: range.r0, c: range.c0 };
    this.ranges = [range];
    return range;
  }

  /** Double-click on the fill handle: fills down as far as the neighbouring column has data. */
  fillHandleToEnd(): Range | undefined {
    const p = this.primary;
    const side = p.c0 > 0 ? p.c0 - 1 : p.c1 + 1 < this.colCount ? p.c1 + 1 : -1;
    if (side < 0) return undefined;
    let end = p.r1;
    while (end + 1 < this.rowCount && this.value(end + 1, side) !== "") end++;
    if (end === p.r1) return undefined;
    return this.fillHandle(p, { r: end, c: p.c1 });
  }

  // ---- clipboard ----

  /** The selected range as text, for Ctrl+C. Several ranges are refused (as in spreadsheets). */
  copyData(
    cut = false,
  ): { tsv: string; html: string; neutralised: number; raw: string } | undefined {
    if (this.rowCount === 0) return undefined;
    const p = this.primary;
    const rows = this.#displayRows(p);
    const raw = toTsv(rows);
    const safe = neutralise(rows);
    if (cut && !this.readOnly) this.#cut = { text: raw, range: p };
    else this.#cut = undefined;
    return { tsv: toTsv(safe.rows), html: toHtml(safe.rows), neutralised: safe.count, raw };
  }

  #displayRows(p: Range): string[][] {
    const rows: string[][] = [];
    for (let r = p.r0; r <= p.r1; r++) {
      const row: string[] = [];
      for (let c = p.c0; c <= p.c1; c++) row.push(this.value(r, c));
      rows.push(row);
    }
    return rows;
  }

  /** Reads clipboard data and pastes it, or asks first when it is big. Returns rows pasted. */
  paste(data: { text?: string; html?: string }): number {
    const rows = fromClipboard(data);
    if (!rows) return 0;
    const [h, w] = sizeOf(rows);
    if (h * w > 1000 && !this.pending) {
      this.pending = { rows, at: { ...this.active } };
      return 0;
    }
    return this.#pasteRows(rows);
  }

  confirmPaste(): number {
    const p = this.pending;
    this.pending = undefined;
    return p ? this.#pasteRows(p.rows) : 0;
  }

  cancelPaste() {
    this.pending = undefined;
  }

  #pasteRows(rows: string[][]): number {
    if (this.readOnly) {
      this.#refuse();
      return 0;
    }
    const p = this.primary;
    const [h, w] = sizeOf(rows);
    const items: { r: number; c: number; value: string }[] = [];
    const single = h === 1 && w === 1;
    if (single && (p.r1 > p.r0 || p.c1 > p.c0)) {
      // One value over a range fills the range.
      for (const pos of this.#cellsIn([p]))
        items.push({ ...pos, value: (rows[0] as string[])[0] ?? "" });
    } else {
      for (let i = 0; i < h; i++)
        for (let j = 0; j < w; j++)
          items.push({ r: p.r0 + i, c: p.c0 + j, value: (rows[i] as string[])[j] ?? "" });
    }
    // A cut pasted back moves the cells: the source is emptied in the same action.
    const cut = this.#cut;
    const text = toTsv(rows);
    if (cut && cut.text === text && !single) {
      const dest = new Set(items.map((i) => `${i.r}:${i.c}`));
      for (const pos of this.#cellsIn([cut.range]))
        if (!dest.has(`${pos.r}:${pos.c}`)) items.push({ ...pos, value: "" });
    }
    this.#cut = undefined;
    let maxR = 0;
    let maxC = 0;
    for (const i of items) {
      maxR = Math.max(maxR, i.r);
      maxC = Math.max(maxC, i.c);
    }
    const extra: Splice[] = [];
    if (maxC >= this.model.width) {
      const touched = new Set(items.map((i) => this.#recFor(i.r)));
      extra.push(...this.model.widen(maxC + 1, touched));
    }
    const ok = this.#write(items, extra);
    if (!ok) return 0;
    const range = {
      r0: p.r0,
      c0: p.c0,
      r1: single ? p.r1 : maxR,
      c1: single ? p.c1 : maxC,
    };
    this.#clamp();
    this.ranges = [range];
    this.active = { r: range.r0, c: range.c0 };
    return h * w;
  }

  // ---- rows and columns ----

  #selectedRows(): { first: number; n: number } {
    const p = this.primary;
    return { first: p.r0, n: p.r1 - p.r0 + 1 };
  }

  insertRows(where: "above" | "below" = "above") {
    if (this.readOnly) return this.#refuse();
    if (this.structureLocked) {
      this.notice = { text: t("sheet.viewLocked") };
      return;
    }
    const { first, n } = this.#selectedRows();
    const at = where === "above" ? first : first + n;
    const rec = this.model.dataStart + at;
    if (this.doc.perform(this.model.insertRows(rec, n))) {
      this.ranges = [{ r0: at, r1: at + n - 1, c0: 0, c1: this.colCount - 1 }];
      this.active = { r: at, c: 0 };
    }
  }

  insertCols(where: "left" | "right" = "left") {
    if (this.readOnly) return this.#refuse();
    const p = this.primary;
    const n = p.c1 - p.c0 + 1;
    const at = where === "left" ? p.c0 : p.c1 + 1;
    if (this.doc.perform(this.model.insertCols(at, n))) {
      this.ranges = [{ r0: 0, r1: Math.max(0, this.rowCount - 1), c0: at, c1: at + n - 1 }];
      this.active = { r: 0, c: at };
    }
  }

  deleteRows() {
    if (this.readOnly) return this.#refuse();
    const { first, n } = this.#selectedRows();
    const recs: number[] = [];
    for (let d = first; d < first + n && d < this.rowCount; d++) recs.push(this.recordOf(d));
    recs.sort((a, b) => b - a);
    // Contiguous records go in one splice each.
    const splices: Splice[] = [];
    for (let i = 0; i < recs.length; ) {
      let j = i;
      while (j + 1 < recs.length && (recs[j + 1] as number) === (recs[j] as number) - 1) j++;
      splices.push(...this.model.deleteRows(recs[j] as number, j - i + 1));
      i = j + 1;
    }
    if (this.doc.perform(splices)) {
      this.#recordsSeen = this.model.records;
      const d = Math.min(first, Math.max(0, this.rowCount - 1));
      this.select(d, this.active.c);
    }
  }

  deleteCols() {
    if (this.readOnly) return this.#refuse();
    const p = this.primary;
    if (this.doc.perform(this.model.deleteCols(p.c0, p.c1 - p.c0 + 1))) {
      this.select(this.active.r, Math.min(p.c0, this.colCount - 1));
    }
  }

  duplicateRow() {
    if (this.readOnly || this.structureLocked) return;
    const rec = this.recordOf(this.primary.r1);
    this.doc.perform(this.model.duplicateRow(rec));
  }

  /** Moves the selected rows before displayed row `to` (a position between rows). */
  moveRows(to: number) {
    if (this.readOnly || this.structureLocked) return;
    const { first, n } = this.#selectedRows();
    if (to >= first && to <= first + n) return;
    const dest = this.model.dataStart + to;
    if (this.doc.perform(this.model.moveRows(this.model.dataStart + first, n, dest))) {
      const at = to > first ? to - n : to;
      this.ranges = [{ r0: at, r1: at + n - 1, c0: 0, c1: this.colCount - 1 }];
      this.active = { r: at, c: 0 };
    }
  }

  moveCols(to: number) {
    if (this.readOnly) return;
    const p = this.primary;
    const n = p.c1 - p.c0 + 1;
    if (to >= p.c0 && to <= p.c0 + n) return;
    if (this.doc.perform(this.model.moveCols(p.c0, n, to))) {
      const at = to > p.c0 ? to - n : to;
      this.ranges = [{ r0: 0, r1: Math.max(0, this.rowCount - 1), c0: at, c1: at + n - 1 }];
      this.active = { r: 0, c: at };
    }
  }

  // ---- views ----

  setView(next: ViewSpec) {
    this.view = next;
    this.#rebuildView();
    this.tick++;
    this.#clamp();
  }

  sortBy(col: number, dir: 1 | -1) {
    this.setView({ ...this.view, sort: { col, dir } });
  }

  clearSort() {
    this.setView({ filters: this.view.filters });
  }

  setFilter(col: number, filter: ColumnFilter | undefined) {
    const filters = new Map(this.view.filters);
    if (filter) filters.set(col, filter);
    else filters.delete(col);
    this.setView({ ...this.view, filters });
  }

  clearView() {
    this.setView(NO_VIEW);
  }

  /** "Appliquer au fichier": writes the sorted order into the file, once. Filters stay views. */
  applyToFile() {
    if (!this.view.sort || this.readOnly) return;
    const order = buildView(this.model, { sort: this.view.sort, filters: new Map() });
    if (this.doc.perform(this.model.applySort(order))) {
      this.setView({ filters: this.view.filters });
    }
  }

  // ---- geometry actions ----

  setWidth(c: number, w: number) {
    this.widths = {
      ...this.widths,
      [c]: Math.max(32, Math.min(MAX_WIDTH, Math.round(w / this.zoom))),
    };
  }

  /** Width that fits the longest of the first values of a column. */
  autofit(c: number, measure: (text: string) => number) {
    let w = measure(this.headerName(c)) + 28;
    const n = Math.min(this.rowCount, 500);
    for (let d = 0; d < n; d++) w = Math.max(w, measure(this.display(d, c)) + 20);
    this.setWidth(c, Math.min(600, w) * this.zoom);
  }

  freezeToSelection() {
    this.pinnedRows = this.active.r;
    this.pinnedCols = this.active.c;
  }
  unfreeze() {
    this.pinnedRows = 0;
    this.pinnedCols = 0;
  }

  setZoom(z: number) {
    this.zoom = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, Math.round(z * 100) / 100));
  }

  /** Finds a query in the cells. Returns the hits, or the reason the query is wrong. */
  search(
    spec: QuerySpec,
    scope: "sheet" | "column" | "selection" | "headers",
    entireCell: boolean,
  ): { hits: CellHit[]; error?: string } {
    const compiled = compileQuery(spec);
    if (!compiled.ok) return { hits: [], error: compiled.error };
    return {
      hits: findInCells(this.model, compiled.matcher, this.#scopeOf(scope), { entireCell }),
    };
  }

  #scopeOf(scope: "sheet" | "column" | "selection" | "headers"): CellScope {
    switch (scope) {
      case "column":
        return { kind: "column", col: this.active.c };
      case "selection":
        return {
          kind: "selection",
          ranges: this.ranges.map((g) => ({
            r0: this.recordOf(g.r0),
            r1: this.recordOf(g.r1),
            c0: g.c0,
            c1: g.c1,
          })),
        };
      default:
        return { kind: scope };
    }
  }

  /** Puts the selection on the cell of a hit; false when a filter hides its row. */
  goTo(hit: { r: number; c: number }): boolean {
    if (hit.r < this.model.dataStart) {
      this.select(0, hit.c);
      return true;
    }
    const d = this.displayOf(hit.r);
    if (d === undefined) {
      this.notice = { text: t("sheet.hitHidden") };
      return false;
    }
    this.select(d, hit.c);
    return true;
  }

  /** Replaces in the given hits' cells, as one change. Locked cells are left alone and counted. */
  replaceHits(hits: readonly CellHit[], template: string, opts: ReplaceCellOptions = {}): number {
    if (this.readOnly) {
      this.#refuse();
      return 0;
    }
    const items = hits
      .filter((h) => this.opts.isLocked?.(h.r, h.c) !== true)
      .map((h) => ({ r: h.r, c: h.c, value: replacedValue(h, template, opts) }));
    if (items.length < hits.length)
      this.notice = { text: t("sheet.lockedSkipped", { count: hits.length - items.length }) };
    const safe = this.#plain(items);
    if (safe.length === 0) return 0;
    return this.doc.perform(this.model.editCells(safe)) ? safe.length : 0;
  }

  setDialect(next: Dialect) {
    this.opts.onDialect?.(next);
    this.doc.pinned = true;
    this.model.setDialect(next);
    this.#types.clear();
    this.#rebuildView();
    this.tick++;
    this.#clamp();
  }

  /** "Convertir en ;": rewrites the file once, as a single step. */
  convertDelimiter(to: Dialect["delimiter"]) {
    if (this.readOnly) return;
    const splices = this.model.convertDelimiter(to);
    if (this.doc.perform(splices)) {
      this.doc.pinned = true;
      this.model.setDialect({ ...this.model.dialect, delimiter: to });
      this.model.reset(this.model.text);
      this.tick++;
    }
  }

  undo(): boolean {
    this.editing = undefined;
    if (this.opts.onUndoKey?.()) return true;
    const ok = this.doc.undo();
    if (!ok && this.doc.canUndo === false) this.notice = { text: t("sheet.nothingToUndo") };
    return ok;
  }

  redo(): boolean {
    return this.doc.redo();
  }
}
