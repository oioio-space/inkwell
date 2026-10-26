import { lang, t, tn } from "../i18n";
import type { Op } from "./words";

export interface Heading {
  /** Offset of the heading line. */
  pos: number;
  level: number;
  title: string;
}

/** The headings of a Typst or Markdown text (a trailing label like `<id>` is not part of the title). */
export function headingsOf(text: string): Heading[] {
  const out: Heading[] = [];
  let pos = 0;
  for (const line of text.split("\n")) {
    const m = line.match(/^(={1,6}|#{1,6})[ \t]+(.+?)\s*$/);
    if (m) {
      const title = (m[2] as string).replace(/\s*<[^<>\s]+>\s*$/, "").trim();
      if (title) out.push({ pos, level: (m[1] as string).length, title });
    }
    pos += line.length + 1;
  }
  return out;
}

/** The heading a position falls under: the last one at or before it. */
export function sectionAt(headings: readonly Heading[], pos: number): Heading | undefined {
  let found: Heading | undefined;
  for (const h of headings) {
    if (h.pos > pos) break;
    found = h;
  }
  return found;
}

/** The text around `at` is on a heading line. */
const headingTouched = (text: string, at: number, _part: string): boolean => {
  const from = text.lastIndexOf("\n", at - 1) + 1;
  return /^(={1,6}|#{1,6})[ \t]+\S/.test(text.slice(from, from + 12));
};

/** The stretch is a whole line (or several): it starts a line and ends at a line end. */
const wholeLine = (text: string, at: number, part: string): boolean => {
  const startsLine = at === 0 || text[at - 1] === "\n";
  const end = at + part.length;
  return startsLine && (part.endsWith("\n") || end >= text.length || text[end] === "\n");
};

const sentences = (s: string): number => {
  if (!/[\p{L}\p{N}]/u.test(s)) return 0;
  const ends = s.match(/[.!?…](?=\s|$)/g)?.length ?? 0;
  return Math.max(1, ends);
};

const wordCount = (s: string): number => s.match(/[\p{L}\p{N}][\p{L}\p{N}'’_-]*/gu)?.length ?? 0;

type Unit = "sentence" | "heading" | "word" | "block";
type Verb = "added" | "removed" | "changed" | "moved" | "fixed";

interface Counts {
  /** Insertion order is the order of the clauses. */
  [key: string]: number;
}

/** What a diff did, in plain words, by section: « ajouté 2 phrases et modifié 1 titre ». */
export interface Summary {
  text: string;
  /** Titles of the sections touched, in order of appearance. */
  sections: string[];
  /** The number of distinct changes (a replaced word, an added sentence...). */
  changes: number;
}

const listFormat = new Intl.ListFormat(lang, { style: "long", type: "conjunction" });

const clause = (verb: Verb, unit: Unit, n: number): string => tn(`history.sum.${verb}.${unit}`, n);

/** Names the sections by their title, never by a line number (DIF-01, DIF-03). */
export function summarize(
  oldText: string,
  newText: string,
  ops: readonly Op[],
  who?: string,
): Summary {
  const heads = headingsOf(newText);
  const oldHeads = headingsOf(oldText);
  const groups = new Map<string, Counts>();
  const order: string[] = [];
  let changes = 0;

  const bump = (section: string, verb: Verb, unit: Unit, n = 1) => {
    if (n <= 0) return;
    let c = groups.get(section);
    if (!c) {
      c = {};
      groups.set(section, c);
      order.push(section);
    }
    const key = `${verb}/${unit}`;
    c[key] = (c[key] ?? 0) + n;
    changes += n;
  };
  const sectionName = (pos: number, inOld = false): string =>
    (sectionAt(inOld ? oldHeads : heads, pos)?.title ?? "") as string;

  for (let i = 0; i < ops.length; i++) {
    const op = ops[i] as Op;
    if (op.kind === "eq") continue;
    if (op.moved !== undefined) {
      if (op.kind === "add") bump(sectionName(op.b), "moved", "block");
      continue;
    }
    const next = ops[i + 1];
    const pair =
      op.kind === "del" && next?.kind === "add" && next.moved === undefined ? next : undefined;
    if (pair) i++;
    const add = pair ?? (op.kind === "add" ? op : undefined);
    const del = op.kind === "del" ? op : undefined;
    const at = add ? add.b : (del?.b ?? 0);
    const section = sectionName(at);
    // A heading line touched inside is a modified title; a whole heading line is added or removed.
    const heading =
      (add
        ? headingTouched(newText, add.b, add.text)
        : headingTouched(oldText, del?.a ?? 0, del?.text ?? "")) &&
      ((add ?? del)?.text.trim() ?? "") !== "";
    if (heading) {
      const whole = add
        ? wholeLine(newText, add.b, add.text)
        : wholeLine(oldText, del?.a ?? 0, del?.text ?? "");
      bump(
        section,
        add && del ? "changed" : !whole ? "changed" : add ? "added" : "removed",
        "heading",
      );
    } else if (add && del && wordCount(add.text) === 1 && wordCount(del.text) === 1) {
      bump(section, "fixed", "word");
    } else if (add && del) {
      bump(section, "changed", "sentence", Math.max(sentences(add.text), sentences(del.text)));
    } else if (add) bump(section, "added", "sentence", sentences(add.text));
    else if (del) bump(section, "removed", "sentence", sentences(del.text));
  }

  const parts: string[] = [];
  for (const section of order) {
    const counts = groups.get(section) as Counts;
    const clauses = Object.entries(counts).map(([key, n]) => {
      const [verb, unit] = key.split("/") as [Verb, Unit];
      return clause(verb, unit, n);
    });
    if (clauses.length === 0) continue;
    const what = listFormat.format(clauses);
    parts.push(section ? t("history.sum.in", { what, section }) : what);
  }
  if (parts.length === 0) return { text: t("history.sum.none"), sections: [], changes: 0 };
  const all = listFormat.format(parts);
  return {
    text: who ? t("history.sum.who", { who, what: all }) : t("history.sum.anon", { what: all }),
    sections: order.filter(Boolean),
    changes,
  };
}
