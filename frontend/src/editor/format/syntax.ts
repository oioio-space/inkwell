import { leadRun, trailRun } from "./text";
import type { Block, InsertItem, Mark } from "./types";

// What a language contributes to the format commands. Everything here is pure string work on
// one line at a time; the commands turn the answers into CodeMirror transactions.

/** A delimited inline span: where its opening and closing delimiters sit in the line. */
export interface Span {
  mark: Mark;
  open: [number, number];
  close: [number, number];
}

export interface LineScan {
  spans: Span[];
  /** Inline code and formulas: marks are not interpreted inside. */
  verbatim: [number, number][];
}

/** Characters to delete, as offsets relative to the start of the marked text. */
export interface Removal {
  del: [number, number][];
}

/** What turning a mark on does to a piece of text. */
export interface Application {
  prefix: string;
  suffix: string;
  /** The marked text, when it must change too (stripping inner delimiters, escaping). */
  core?: string;
  /** Selection after, relative to the start of the new text; default: the marked text. */
  sel?: [number, number];
}

export interface DetectContext {
  line: string;
  /** The marked text is `line.slice(from, to)`. */
  from: number;
  to: number;
  scan: LineScan;
}

export interface MarkSpec {
  /** Delimiters the selection itself wraps (`*x*` selected): how many chars to look inside. */
  peel(core: string): { lead: number; trail: number };
  /** Is the text already marked? Where are the delimiters? */
  detect(ctx: DetectContext): Removal | null;
  apply(core: string): Application;
}

export interface LineClass {
  block: Block;
  /** Leading whitespace kept when the prefix changes. */
  indent: number;
  /** End of the prefix (after the indent). */
  prefixEnd: number;
  /** Chars removed from the end of the line along with the prefix (`]` of a Typst heading call). */
  suffix: number;
}

export interface LangSyntax {
  marks: Partial<Record<Mark, MarkSpec>>;
  blocks: ReadonlySet<Block>;
  inserts: ReadonlySet<InsertItem>;
  scan(line: string): LineScan;
  classify(line: string): LineClass;
  /** The prefix of a block; `n` is the 1-based position in a numbered list. */
  prefix(block: Block, n: number): string;
}

const NONE = { lead: 0, trail: 0 };

// ── Mark specs ──────────────────────────────────────────────────────────────

/**
 * A mark written as a run of one character: `*x*`, `**x**`, `~~x~~`. Runs are counted so
 * nesting (`***x***`) stays well-formed: `odd` marks (single-char delimiters) are active when
 * the run around the text is odd, the others when it holds at least `unit` chars.
 */
export function runSpec(char: string, unit: 1 | 2, odd: boolean, strip: RegExp): MarkSpec {
  const delimiter = char.repeat(unit);
  const width = odd ? 1 : unit;
  return {
    peel(core) {
      const lead = leadRun(core, char);
      if (lead >= core.length) return NONE;
      const k = Math.min(lead, trailRun(core, char));
      return { lead: k, trail: k };
    },
    detect({ line, from, to }) {
      const m = Math.min(trailRun(line.slice(0, from), char), leadRun(line.slice(to), char));
      if (odd ? m % 2 === 0 : m < unit) return null;
      return {
        del: [
          [-width, 0],
          [to - from, to - from + width],
        ],
      };
    },
    apply(core) {
      const stripped = core.replace(strip, "$1");
      return {
        prefix: delimiter,
        suffix: delimiter,
        ...(stripped === core ? {} : { core: stripped }),
      };
    },
  };
}

/** A mark written with fixed opening and closing text: `#underline[x]`. */
export function pairSpec(open: string, close: string, strip?: RegExp): MarkSpec {
  return {
    peel(core) {
      return core.length >= open.length + close.length &&
        core.startsWith(open) &&
        core.endsWith(close)
        ? { lead: open.length, trail: close.length }
        : NONE;
    },
    detect({ line, from, to }) {
      if (!line.slice(0, from).endsWith(open) || !line.slice(to).startsWith(close)) return null;
      return {
        del: [
          [-open.length, 0],
          [to - from, to - from + close.length],
        ],
      };
    },
    apply(core) {
      const stripped = strip ? core.replace(strip, "$1") : core;
      return {
        prefix: open,
        suffix: close,
        ...(stripped === core ? {} : { core: stripped }),
      };
    },
  };
}

/** Inline code: the fence is one backtick longer than any run inside, and padded when it is doubled. */
export const codeSpec: MarkSpec = {
  peel(core) {
    const lead = /^`+/.exec(core)?.[0].length ?? 0;
    const trail = /`+$/.exec(core)?.[0].length ?? 0;
    if (lead === 0 || lead !== trail || core.length <= lead + trail) return NONE;
    const padded = core[lead] === " " && core[core.length - trail - 1] === " ";
    const inner = core.slice(lead + 1, core.length - trail - 1);
    if (padded && (lead > 1 || inner.startsWith("`") || inner.endsWith("`"))) {
      return { lead: lead + 1, trail: trail + 1 };
    }
    return { lead, trail };
  },
  detect({ line, from, to }) {
    const left = /(`+)( ?)$/.exec(line.slice(0, from));
    const right = /^( ?)(`+)/.exec(line.slice(to));
    if (!left || !right) return null;
    const [lm, lTicks = "", lPad = ""] = left;
    const [rm, rPad = "", rTicks = ""] = right;
    if (lTicks.length !== rTicks.length || lPad !== rPad) return null;
    if (lPad && lTicks.length < 2 && !/^`|`$/.test(line.slice(from, to))) return null;
    return {
      del: [
        [-lm.length, 0],
        [to - from, to - from + rm.length],
      ],
    };
  },
  apply(core) {
    if (core === "") return { prefix: "`", suffix: "`" };
    let longest = 0;
    for (const run of core.match(/`+/g) ?? []) longest = Math.max(longest, run.length);
    const ticks = "`".repeat(longest + 1);
    const pad = longest > 0 ? " " : "";
    return { prefix: ticks + pad, suffix: pad + ticks };
  },
};

/** Removal of the innermost link span around the marked text. */
export function enclosingLink({ from, to, scan }: DetectContext): Removal | null {
  let best: Span | undefined;
  for (const span of scan.spans) {
    if (span.mark !== "link" || span.open[1] > from || to > span.close[0]) continue;
    if (!best || span.open[0] > best.open[0]) best = span;
  }
  if (!best) return null;
  return {
    del: [
      [best.open[0] - from, best.open[1] - from],
      [best.close[0] - from, best.close[1] - from],
    ],
  };
}

export const looksLikeUrl = (s: string): boolean => /^[a-z][a-z0-9+.-]*:\/\/\S+$/i.test(s);

// ── Inline scanner ──────────────────────────────────────────────────────────

interface RunRule {
  m1?: Mark;
  m2?: Mark;
  /** `_` does not open or close inside a word. */
  wordSafe?: boolean;
}

interface Opener {
  ch: string;
  pos: number;
  remaining: number;
}

const isSpace = (c: string | undefined) => c === undefined || /\s/u.test(c);
const isAlnum = (c: string | undefined) => c !== undefined && /[\p{L}\p{N}]/u.test(c);

/** Delimiter-run matching in the spirit of CommonMark's emphasis algorithm. */
export class Emphasis {
  readonly spans: Span[] = [];
  private stack: Opener[] = [];

  constructor(
    private readonly text: string,
    private readonly rules: Record<string, RunRule>,
  ) {}

  run(ch: string, pos: number, n: number): void {
    const rule = this.rules[ch];
    if (!rule) return;
    const prev = this.text[pos - 1];
    const next = this.text[pos + n];
    let canOpen = !isSpace(next);
    let canClose = !isSpace(prev);
    if (rule.wordSafe) {
      canOpen &&= !isAlnum(prev);
      canClose &&= !isAlnum(next);
    }
    let left = n;
    while (canClose && left > 0) {
      const idx = this.stack.findLastIndex((o) => o.ch === ch);
      const opener = this.stack[idx];
      if (!opener) break;
      const use = rule.m2 && opener.remaining >= 2 && left >= 2 ? 2 : rule.m1 ? 1 : 0;
      const mark = use === 2 ? rule.m2 : rule.m1;
      if (!mark) break;
      const oStart = opener.pos + opener.remaining - use;
      const cStart = pos + (n - left);
      this.spans.push({ mark, open: [oStart, oStart + use], close: [cStart, cStart + use] });
      opener.remaining -= use;
      left -= use;
      this.stack.length = idx + 1;
      if (opener.remaining === 0) this.stack.pop();
    }
    if (left > 0 && canOpen) this.stack.push({ ch, pos: pos + (n - left), remaining: left });
  }
}

/** The end of the verbatim span (backticks or `$...$`) starting at `i`, or -1. */
export function verbatimEnd(text: string, i: number): number {
  const ch = text[i];
  if (ch === "`") {
    const n = leadRun(text.slice(i), "`");
    let j = i + n;
    while (j < text.length) {
      const k = text.indexOf("`", j);
      if (k < 0) return -1;
      const m = leadRun(text.slice(k), "`");
      if (m === n) return k + m;
      j = k + m;
    }
    return -1;
  }
  if (ch === "$") {
    for (let j = i + 1; j < text.length; j++) {
      if (text[j] === "\\") j++;
      else if (text[j] === "$") return j + 1;
    }
  }
  return -1;
}
