import {
  type ChangeSpec,
  EditorSelection,
  type EditorState,
  type SelectionRange,
  type StateCommand,
  type Text,
  type Transaction,
} from "@codemirror/state";
import { fenceAt, fencesUpTo, isInFence } from "./fence";
import type { Lang } from "./lang";
import { syntaxOf } from "./support";
import type { LineScan, MarkSpec, Removal } from "./syntax";
import { snapToGrapheme, wordAt } from "./text";
import type { Block, Mark } from "./types";

// Every command here is one transaction: one undo step, one patch for the other people.
// A command that cannot apply (read-only file, format that lacks the feature) returns false
// without dispatching, so its shortcut falls through.

interface Target {
  from: number;
  to: number;
  /** Where the caret was when the target is the word around it. */
  cursor?: number;
  main: boolean;
  forward: boolean;
}

/** The pieces of text a mark applies to: the word at each caret, each line of each selection. */
function targetsOf(state: EditorState): Target[] {
  const found: Target[] = [];
  const { doc, selection } = state;
  selection.ranges.forEach((range, i) => {
    const main = i === selection.mainIndex;
    const forward = range.anchor <= range.head;
    if (range.empty) {
      const line = doc.lineAt(range.head);
      const offset = range.head - line.from;
      const word = wordAt(line.text, offset);
      if (word) {
        found.push({
          from: line.from + word[0],
          to: line.from + word[1],
          cursor: range.head,
          main,
          forward,
        });
      } else {
        const at = line.from + snapToGrapheme(line.text, offset, -1);
        found.push({ from: at, to: at, main, forward });
      }
      return;
    }
    for (let n = doc.lineAt(range.from).number; n <= doc.lineAt(range.to).number; n++) {
      const line = doc.line(n);
      const a = Math.max(range.from, line.from) - line.from;
      const b = Math.min(range.to, line.to) - line.from;
      const piece = line.text.slice(a, b);
      const trimmed = piece.trim();
      if (!trimmed) continue;
      const start = a + piece.length - piece.trimStart().length;
      const from = snapToGrapheme(line.text, start, -1);
      const to = snapToGrapheme(line.text, start + trimmed.length, 1);
      found.push({ from: line.from + from, to: line.from + to, main, forward });
    }
  });
  found.sort((x, y) => x.from - y.from || x.to - y.to);
  const merged: Target[] = [];
  for (const t of found) {
    const last = merged.at(-1);
    if (last && t.from <= last.to) {
      // A caret keeps its place only when the carets all sit in one and the same word.
      if (last.from !== t.from || last.to !== t.to) delete last.cursor;
      last.to = Math.max(last.to, t.to);
      last.main ||= t.main;
    } else {
      merged.push({ ...t });
    }
  }
  return merged;
}

interface Plan {
  /** The marked text after peeling delimiters the selection itself wraps. */
  from: number;
  to: number;
  cursor: number | undefined;
  removal: Removal | null;
}

function analyse(
  state: EditorState,
  spec: MarkSpec,
  target: Target,
  scans: Map<number, LineScan>,
  scanLine: (text: string) => LineScan,
): Plan {
  const line = state.doc.lineAt(target.from);
  let scan = scans.get(line.number);
  if (!scan) {
    scan = scanLine(line.text);
    scans.set(line.number, scan);
  }
  const core = line.text.slice(target.from - line.from, target.to - line.from);
  const { lead, trail } = spec.peel(core);
  const from = target.from - line.from + lead;
  const to = target.to - line.from - trail;
  const removal = spec.detect({ line: line.text, from, to, scan });
  return { from: line.from + from, to: line.from + to, cursor: target.cursor, removal };
}

interface Outcome {
  edits: { from: number; to: number; insert: string }[];
  /** Selection in the document as edited by this target alone. */
  sel: [number, number];
  delta: number;
}

function removeOutcome(plan: Plan, removal: Removal): Outcome {
  let before = 0;
  let delta = 0;
  const edits = removal.del.map(([a, b]) => {
    delta -= b - a;
    if (b <= 0) before += b - a;
    return { from: plan.from + a, to: plan.from + b, insert: "" };
  });
  const from = plan.from - before;
  const to = plan.to - before;
  return {
    edits,
    sel: plan.cursor === undefined ? [from, to] : [plan.cursor - before, plan.cursor - before],
    delta,
  };
}

function applyOutcome(plan: Plan, spec: MarkSpec, state: EditorState): Outcome {
  const text = state.doc.sliceString(plan.from, plan.to);
  const app = spec.apply(text);
  const core = app.core ?? text;
  const delta = app.prefix.length + app.suffix.length + core.length - text.length;
  const edits =
    app.core === undefined
      ? [
          { from: plan.from, to: plan.from, insert: app.prefix },
          { from: plan.to, to: plan.to, insert: app.suffix },
        ]
      : [{ from: plan.from, to: plan.to, insert: app.prefix + core + app.suffix }];
  let sel: [number, number];
  if (app.sel) sel = [plan.from + app.sel[0], plan.from + app.sel[1]];
  else if (plan.cursor !== undefined) {
    const c = plan.cursor + app.prefix.length;
    sel = [c, c];
  } else sel = [plan.from + app.prefix.length, plan.from + app.prefix.length + core.length];
  return { edits, sel, delta };
}

const noop = (plan: Plan): Outcome => {
  const sel: [number, number] =
    plan.cursor === undefined ? [plan.from, plan.to] : [plan.cursor, plan.cursor];
  return { edits: [], sel, delta: 0 };
};

function dispatchOutcomes(
  state: EditorState,
  targets: Target[],
  outcomes: Outcome[],
  userEvent: string,
  dispatch: (tr: Transaction) => void,
): void {
  const changes: ChangeSpec[] = [];
  const ranges: SelectionRange[] = [];
  let shift = 0;
  let main = 0;
  outcomes.forEach((o, i) => {
    const target = targets[i];
    if (!target) return;
    changes.push(...o.edits);
    const [a, b] = o.sel;
    ranges.push(
      target.forward
        ? EditorSelection.range(a + shift, b + shift)
        : EditorSelection.range(b + shift, a + shift),
    );
    if (target.main) main = ranges.length - 1;
    shift += o.delta;
  });
  if (changes.length === 0) return;
  dispatch(
    state.update({
      changes,
      selection: EditorSelection.create(ranges, main),
      userEvent,
      scrollIntoView: true,
    }),
  );
}

/**
 * Turns an inline mark on or off. Off when every piece of text is already marked, on
 * otherwise (a partly marked selection becomes fully marked, never nested).
 */
export function toggleMark(lang: Lang, mark: Mark): StateCommand {
  return ({ state, dispatch }) => {
    const syntax = syntaxOf(lang);
    const spec = syntax?.marks[mark];
    if (!syntax || !spec || state.readOnly) return false;
    const targets = targetsOf(state);
    const scans = new Map<number, LineScan>();
    const plans = targets.map((t) => analyse(state, spec, t, scans, syntax.scan));
    const removing = plans.length > 0 && plans.every((p) => p.removal);
    let reach = -1;
    const outcomes = plans.map((plan) => {
      let outcome: Outcome;
      if (plan.removal && removing) outcome = removeOutcome(plan, plan.removal);
      else if (plan.removal) outcome = noop(plan);
      else outcome = applyOutcome(plan, spec, state);
      // Neighbouring edits that would overlap (a delimiter shared by two pieces) are skipped.
      const first = Math.min(...outcome.edits.map((e) => e.from));
      if (outcome.edits.length > 0 && first < reach) outcome = noop(plan);
      for (const e of outcome.edits) reach = Math.max(reach, e.to);
      return outcome;
    });
    dispatchOutcomes(state, targets, outcomes, `format.${mark}`, dispatch);
    return true;
  };
}

// ── Blocks ──────────────────────────────────────────────────────────────────

/**
 * The first and last line numbers a range covers. A selection that ends at the very start of a
 * line (after a triple click, say) does not take that line.
 */
function lineRange(doc: Text, range: SelectionRange): [number, number] {
  const first = doc.lineAt(range.from).number;
  const last = doc.lineAt(range.to);
  const endsBefore = !range.empty && range.to === last.from && last.number > first;
  return [first, endsBefore ? last.number - 1 : last.number];
}

/** Line numbers a block command acts on: every line the selections touch. */
function selectedLines(state: EditorState): number[] {
  const lines = new Set<number>();
  for (const range of state.selection.ranges) {
    const [first, last] = lineRange(state.doc, range);
    for (let n = first; n <= last; n++) lines.add(n);
  }
  return [...lines].sort((a, b) => a - b);
}

/** Position of the `]` that closes the bracket opened just before `from` on line `lineNo`. */
function closingBracket(doc: Text, lineNo: number, from: number): number {
  let depth = 1;
  for (let n = lineNo; n <= doc.lines; n++) {
    const line = doc.line(n);
    const start = n === lineNo ? from - line.from : 0;
    for (let i = start; i < line.text.length; i++) {
      const ch = line.text[i];
      if (ch === "\\") i++;
      else if (ch === "[") depth++;
      else if (ch === "]" && --depth === 0) return line.from + i;
    }
  }
  return -1;
}

const TOGGLED: ReadonlySet<Block> = new Set(["ul", "ol", "task", "quote"]);

function codeBlockCommand(lang: Lang): StateCommand {
  return ({ state, dispatch }) => {
    const { doc } = state;
    const head = doc.lineAt(state.selection.main.head);
    const fence = fenceAt(doc, head.number);
    if (fence) return unfence(state, dispatch, fence.open, fence.close);

    const filled = state.selection.ranges.filter((r) => !r.empty);
    const changes: ChangeSpec[] = [];
    const slots: number[] = [];
    let shift = 0;
    if (filled.length === 0) {
      // No selection: an empty block, cursor inside it.
      const blank = head.text.trim() === "";
      const at = blank ? head.from : head.to;
      const insert = `${blank ? "" : "\n"}\`\`\`\n\n\`\`\``;
      changes.push({ from: at, to: head.to, insert });
      slots.push(at + (blank ? 0 : 1) + 4);
    } else {
      for (const [first, last] of lineSpans(state, filled)) {
        const a = doc.line(first);
        const b = doc.line(last);
        changes.push({ from: a.from, insert: "```\n" }, { from: b.to, insert: "\n```" });
        slots.push(a.from + shift + 3);
        shift += 8;
      }
    }
    const selection = EditorSelection.create(slots.map((s) => EditorSelection.cursor(s)));
    dispatch(
      state.update({
        changes,
        selection,
        userEvent: `format.codeblock.${lang}`,
        scrollIntoView: true,
      }),
    );
    return true;
  };
}

/** The first and last line number of each selection, merged where they overlap. */
function lineSpans(state: EditorState, ranges: readonly SelectionRange[]): [number, number][] {
  const spans: [number, number][] = [];
  for (const r of ranges) {
    const [first, last] = lineRange(state.doc, r);
    const prev = spans.at(-1);
    if (prev && first <= prev[1]) prev[1] = Math.max(prev[1], last);
    else spans.push([first, last]);
  }
  return spans;
}

function unfence(
  state: EditorState,
  dispatch: (tr: Transaction) => void,
  open: number,
  close: number | null,
): boolean {
  const { doc } = state;
  const o = doc.line(open);
  const cuts: [number, number][] = [[o.from, Math.min(doc.length, o.to + 1)]];
  if (close !== null) {
    const c = doc.line(close);
    cuts.push(c.to < doc.length ? [c.from, c.to + 1] : [Math.max(0, c.from - 1), c.to]);
  }
  const merged: [number, number][] = [];
  for (const cut of cuts) {
    const last = merged.at(-1);
    if (last && cut[0] <= last[1]) last[1] = Math.max(last[1], cut[1]);
    else merged.push([cut[0], cut[1]]);
  }
  const changes = state.changes(merged.map(([from, to]) => ({ from, to })));
  dispatch(
    state.update({
      changes,
      selection: state.selection.map(changes, 1),
      userEvent: "format.codeblock",
      scrollIntoView: true,
    }),
  );
  return true;
}

/** Sets the block type of the lines the selections touch (heading level, list, quote…). */
export function setBlock(lang: Lang, block: Block): StateCommand {
  return (target) => {
    const { state, dispatch } = target;
    const syntax = syntaxOf(lang);
    if (!syntax?.blocks.has(block) || state.readOnly) return false;
    if (block === "codeblock") return codeBlockCommand(lang)(target);

    const { doc } = state;
    const head = doc.lineAt(state.selection.main.head);
    const inFence = fenceAt(doc, head.number);
    // Inside a code block only "paragraph" means anything: it takes the block apart.
    if (inFence)
      return block === "paragraph" && unfence(state, dispatch, inFence.open, inFence.close);

    const all = selectedLines(state);
    const fences = fencesUpTo(doc, all.at(-1) ?? 0);
    const outside = all.filter((n) => !isInFence(fences, doc, n));
    const filled = outside.filter((n) => doc.line(n).text.trim() !== "");
    // Blank lines stay blank in a multi-line selection; a lone blank line starts the block.
    const lines = filled.length > 0 ? filled : outside.slice(0, 1);
    if (lines.length === 0) return true;

    const kinds = lines.map((n) => syntax.classify(doc.line(n).text).block);
    const typstQuote = lang === "typst" && block === "quote";
    const off = TOGGLED.has(block) && kinds.every((k) => k === block);
    const wanted: Block = off ? "paragraph" : block;

    const edits: { from: number; to: number; insert: string }[] = [];
    if (typstQuote && !off) wrapQuotes(state, lines, edits);
    else {
      let n = 0;
      for (const lineNo of lines) {
        const line = doc.line(lineNo);
        const cls = syntax.classify(line.text);
        n++;
        if (cls.block === wanted && wanted !== "ol") continue;
        const prefix = syntax.prefix(wanted, n);
        const start = line.from + cls.indent;
        if (cls.block === "quote" && lang === "typst") {
          const closing = closingBracket(doc, lineNo, line.from + cls.prefixEnd);
          if (closing >= 0) edits.push({ from: closing, to: closing + 1, insert: "" });
        }
        if (doc.sliceString(start, line.from + cls.prefixEnd) !== prefix) {
          edits.push({ from: start, to: line.from + cls.prefixEnd, insert: prefix });
        }
        if (cls.suffix > 0) edits.push({ from: line.to - cls.suffix, to: line.to, insert: "" });
      }
    }
    if (edits.length === 0) return true;
    edits.sort((a, b) => a.from - b.from);
    const changes = state.changes(edits);
    dispatch(
      state.update({
        changes,
        selection: state.selection.map(changes, 1),
        userEvent: `format.${block}`,
        scrollIntoView: true,
      }),
    );
    return true;
  };
}

/** Typst quotes are one `#quote[…]` around a run of consecutive lines. */
function wrapQuotes(
  state: EditorState,
  lines: readonly number[],
  edits: { from: number; to: number; insert: string }[],
): void {
  const runs: number[][] = [];
  for (const n of lines) {
    const run = runs.at(-1);
    if (run && run.at(-1) === n - 1) run.push(n);
    else runs.push([n]);
  }
  for (const run of runs) {
    const first = state.doc.line(run[0] ?? 0);
    const last = state.doc.line(run.at(-1) ?? 0);
    const indent = /^[ \t]*/.exec(first.text)?.[0].length ?? 0;
    edits.push({ from: first.from + indent, to: first.from + indent, insert: "#quote[" });
    edits.push({ from: last.to, to: last.to, insert: "]" });
  }
}
