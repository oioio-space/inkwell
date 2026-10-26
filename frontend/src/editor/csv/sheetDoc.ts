import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import type { Dialect } from "./dialect";
import { detectDialect } from "./dialect";
import { CsvModel, windowOf } from "./model";
import type { Splice } from "./records";

/** What the sheet needs from a document holding the CSV text: reads, one-change writes, anchors. */
export interface DocPort {
  text(): string;
  /** Applies all splices (non-overlapping, original coordinates) as ONE change. */
  change(splices: readonly Splice[]): void;
  /** A stable anchor on the character at `pos`; follows concurrent edits. */
  cursor(pos: number): string;
  resolve(cursor: string): number;
}

/** The port over an automerge-repo handle (text at `["text"]`). */
export function handlePort(handle: DocHandle<{ text: string }>): DocPort {
  const path = ["text"];
  return {
    text: () => handle.doc()?.text ?? "",
    change(splices) {
      handle.change((d) => {
        for (const s of [...splices].sort((a, b) => b.pos - a.pos))
          A.splice(d, path, s.pos, s.del, s.ins);
      });
    },
    cursor: (pos) => A.getCursor(handle.doc(), path, pos),
    resolve: (c) => A.getCursorPosition(handle.doc(), path, c),
  };
}

/**
 * Automerge 3.5 panics on a text of about 20 MB (ADR 0026), so a file past this many characters
 * is only read: the sheet shows, sorts, filters, searches and copies it, and never writes.
 */
export const MAX_EDITABLE_CHARS = 10 * 1024 * 1024;

const END = "end";
const START = "start";

interface Undo {
  /** Where the action's inserted text now sits, as anchors, to find it again after other edits. */
  parts: { before: string; after: string; ins: string; restore: string }[];
  /** The column an edit of a single cell touched, to notice when it ends up elsewhere. */
  col?: number;
  /** The whole cell as written, as anchors on its first character and the one after it. */
  span?: { start: string; end: string };
}

/**
 * The CSV of one file: the model plus the way edits reach the document. Each user action is a
 * single `change`, hence one history step; undo is per person (only my own actions are on the
 * stack) and re-finds the text it wrote through anchors, so it stays right after other
 * people's edits, and refuses (rather than guess) when that text was changed since.
 */
export class SheetDoc {
  readonly model: CsvModel;
  #undo: Undo[] = [];
  #redo: Undo[] = [];
  /** Called with the window of text a change touched, mine or a remote one. */
  onChange: (window: { from: number; to: number; insLen: number }, remote: boolean) => void =
    () => {};
  /** Called when a remote change moved text I wrote into another column. */
  onMoved: (col: number) => void = () => {};

  /**
   * Until this person edits or chooses, the dialect follows what arrives: an empty file has none
   * to detect yet, and a file that is still being written reads better once it has more rows.
   */
  autoDialect: boolean;
  /** The person chose the separator and the like: detection no longer overrides it. */
  pinned = false;

  constructor(
    readonly port: DocPort,
    model?: CsvModel,
    private readonly hint: { forced?: Dialect["delimiter"]; prefer?: Dialect["delimiter"] } = {},
  ) {
    this.model = model ?? new CsvModel(port.text());
    this.autoDialect = this.model.text === "" || this.model.text.length < 200_000;
  }

  get canUndo(): boolean {
    return this.#undo.length > 0;
  }
  get canRedo(): boolean {
    return this.#redo.length > 0;
  }

  /** True when the text is too big to be edited (see {@link MAX_EDITABLE_CHARS}). */
  get tooBig(): boolean {
    return this.model.text.length > MAX_EDITABLE_CHARS;
  }

  /** Applies one user action (splices in the model's coordinates). */
  perform(splices: readonly Splice[], col?: number): boolean {
    if (splices.length === 0 || this.tooBig) return false;
    this.autoDialect = false;
    const entry = this.#apply(splices, col);
    this.#undo.push(entry);
    if (this.#undo.length > 200) this.#undo.shift();
    this.#redo = [];
    return true;
  }

  #apply(splices: readonly Splice[], col?: number): Undo {
    const old = this.model.text;
    const asc = [...splices].sort((a, b) => a.pos - b.pos);
    this.port.change(splices);
    const next = this.port.text();
    const win = windowOf(splices, next.length, old.length);
    this.model.update(next, win);
    // Anchors on the new text, where each inserted piece now starts.
    const parts: Undo["parts"] = [];
    let shift = 0;
    for (const s of asc) {
      const at = s.pos + shift;
      // Anchored on the characters around the text, not on the text itself: an undo that
      // deletes and writes it again leaves them where they were.
      parts.push({
        before: at <= 0 ? START : this.port.cursor(at - 1),
        after: at + s.ins.length >= next.length ? END : this.port.cursor(at + s.ins.length),
        ins: s.ins,
        restore: old.slice(s.pos, s.pos + s.del),
      });
      shift += s.ins.length - s.del;
    }
    this.onChange(win, false);
    if (col === undefined) return { parts };
    const entry: Undo = { parts, col };
    if (asc.length === 1) {
      const at = (asc[0] as Splice).pos;
      const { r, c } = this.model.locate(at);
      const cell = this.model.record(r)[c];
      if (cell) {
        entry.span = {
          start: this.#anchor(cell.start, next),
          end: this.#anchor(cell.end, next),
        };
      }
    }
    return entry;
  }

  #anchor(pos: number, text: string): string {
    return pos >= text.length ? END : this.port.cursor(pos);
  }

  /** The text was rewritten wholesale (in the text view): read it again, detecting its dialect. */
  rescan(): void {
    const next = this.port.text();
    if (next === this.model.text) return;
    this.model.reset(next);
    if (!this.pinned)
      this.model.setDialect(detectDialect(next, this.hint.forced, this.hint.prefer));
    this.autoDialect = false;
    this.onChange({ from: 0, to: 0, insLen: next.length }, false);
  }

  /** Takes a change made elsewhere (another person, the disk) into the model. */
  receive(window: { from: number; to: number; insLen: number } | undefined): void {
    const next = this.port.text();
    if (next === this.model.text) return;
    let w = window;
    if (this.autoDialect && !this.pinned && next.length < 200_000) {
      this.model.reset(next);
      this.model.setDialect({
        ...detectDialect(next, this.hint.forced, this.hint.prefer),
        quote: this.model.dialect.quote,
      });
      w = window ?? { from: 0, to: 0, insLen: next.length };
    } else if (w) this.model.update(next, w);
    else {
      this.model.reset(next);
      w = { from: 0, to: 0, insLen: next.length };
    }
    this.onChange(w, true);
    this.#notice();
  }

  #notice() {
    for (const e of this.#undo.slice(-10)) {
      if (e.col === undefined || !e.span) continue;
      const start = this.#where({ at: e.span.start });
      const end = this.#where({ at: e.span.end });
      if (start === undefined || end === undefined) continue;
      const { r, c } = this.model.locate(start);
      const cell = this.model.record(r)[c];
      if (!cell || cell.start !== start || cell.end !== end || c !== e.col) this.onMoved(e.col);
    }
  }

  #where(p: { at: string }): number | undefined {
    try {
      return p.at === END ? this.model.text.length : this.port.resolve(p.at);
    } catch {
      return undefined;
    }
  }

  /** The splices that put an entry back, or undefined when its text is no longer there. */
  #inverse(e: Undo): Splice[] | undefined {
    const text = this.model.text;
    const out: Splice[] = [];
    for (const p of e.parts) {
      const from = p.before === START ? 0 : this.#where({ at: p.before });
      const to = this.#where({ at: p.after });
      if (from === undefined || to === undefined) return undefined;
      const pos = p.before === START ? 0 : from + 1;
      if (text.slice(pos, to) !== p.ins) return undefined;
      out.push({ pos, del: p.ins.length, ins: p.restore });
    }
    return out;
  }

  /** Undoes my last action. False when there is nothing to undo or its text changed since. */
  undo(): boolean {
    return this.#step(this.#undo, this.#redo);
  }

  redo(): boolean {
    return this.#step(this.#redo, this.#undo);
  }

  #step(from: Undo[], to: Undo[]): boolean {
    const entry = from[from.length - 1];
    if (!entry || this.tooBig) return false;
    const inv = this.#inverse(entry);
    if (!inv) {
      from.pop();
      return false;
    }
    from.pop();
    to.push(this.#apply(inv));
    return true;
  }
}
