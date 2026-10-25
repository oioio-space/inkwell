import { next as A } from "@automerge/automerge";
import { type FileHits, type Hit, resolveHit } from "./project";
import { unescapeExtended } from "./query";
import type { Match } from "./scan";

/** The part of a DocHandle that replacing needs. */
export interface ReplaceTarget {
  doc(): A.Doc<{ text: string }> | undefined;
  change(fn: (doc: { text: string }) => void): unknown;
}

export interface ReplaceOptions {
  /** Expand `$1`, `$<name>`, `$&` and `$$` in the replacement. */
  regex?: boolean;
  /** Read `\n`, `\t`… in the replacement. */
  extended?: boolean;
  /** True for a range that must not be touched (a locked zone, ADR 0022). */
  skip?: (range: { from: number; to: number }) => boolean;
}

export interface ReplaceReport {
  replaced: number;
  /** Occurrences left alone because they fall in a locked zone. */
  skippedLocked: number;
  /** Occurrences left alone because the words changed since the search. */
  skippedStale: number;
}

/** What `$1`, `$<name>`, `$&`, `$$` in `template` become for one match. */
export function expandReplacement(template: string, m: Match): string {
  return template.replace(
    /\$(\$|&|<([^>]*)>|\d{1,2})/g,
    (all: string, token: string, name: string | undefined) => {
      if (token === "$") return "$";
      if (token === "&") return m.matched;
      if (name !== undefined) return name in m.named ? (m.named[name] ?? "") : all;
      const n = Number(token);
      if (n >= 1 && n <= m.groups.length) return m.groups[n - 1] ?? "";
      if (token.length === 2) {
        const first = Number(token.charAt(0));
        if (first >= 1 && first <= m.groups.length)
          return (m.groups[first - 1] ?? "") + token.charAt(1);
      }
      return all;
    },
  );
}

function replacementFor(template: string, m: Match, opts: ReplaceOptions): string {
  return opts.regex ? expandReplacement(template, m) : template;
}

function prepare(template: string, opts: ReplaceOptions): string {
  return opts.extended ? unescapeExtended(template) : template;
}

/**
 * Replaces the given occurrences of one document in a single `handle.change`, so it is one
 * undo step and one message to the other people. Occurrences whose words changed since the
 * search, or that fall in a locked zone, are left alone and counted.
 */
export function replaceAll(
  handle: ReplaceTarget,
  matches: ReadonlyArray<Hit>,
  replacement: string,
  opts: ReplaceOptions = {},
): ReplaceReport {
  const report: ReplaceReport = { replaced: 0, skippedLocked: 0, skippedStale: 0 };
  const doc = handle.doc();
  if (!doc) return report;
  const template = prepare(replacement, opts);
  const edits: Array<{ from: number; to: number; insert: string }> = [];
  for (const hit of matches) {
    const at = resolveHit(doc, hit);
    if (at.stale) {
      report.skippedStale++;
    } else if (opts.skip?.({ from: at.from, to: at.to })) {
      report.skippedLocked++;
    } else {
      edits.push({ from: at.from, to: at.to, insert: replacementFor(template, hit, opts) });
    }
  }
  edits.sort((a, b) => b.from - a.from); // last first: earlier offsets stay valid
  const real = edits.filter((e) => doc.text.slice(e.from, e.to) !== e.insert);
  if (real.length > 0) {
    handle.change((d) => {
      for (const e of real) A.splice(d, ["text"], e.from, e.to - e.from, e.insert);
    });
  }
  report.replaced = edits.length;
  return report;
}

export interface ProjectReplaceReport extends ReplaceReport {
  /** Files that changed. */
  files: number;
  /** Files with hits but no open document to change. */
  unavailable: string[];
}

/** Replaces across files: one change (one undo step) per file. */
export function replaceInProject(
  results: ReadonlyArray<FileHits>,
  handleOf: (path: string) => ReplaceTarget | undefined,
  replacement: string,
  opts: ReplaceOptions & {
    skipIn?: (path: string, range: { from: number; to: number }) => boolean;
  } = {},
): ProjectReplaceReport {
  const total: ProjectReplaceReport = {
    replaced: 0,
    skippedLocked: 0,
    skippedStale: 0,
    files: 0,
    unavailable: [],
  };
  for (const file of results) {
    const handle = handleOf(file.path);
    if (!handle) {
      total.unavailable.push(file.path);
      continue;
    }
    const { skipIn, ...rest } = opts;
    const perFile: ReplaceOptions = skipIn
      ? { ...rest, skip: (range) => skipIn(file.path, range) }
      : rest;
    const r = replaceAll(handle, file.hits, replacement, perFile);
    total.replaced += r.replaced;
    total.skippedLocked += r.skippedLocked;
    total.skippedStale += r.skippedStale;
    if (r.replaced > 0) total.files++;
  }
  return total;
}

/** One changed spot of the preview: the lines around it, before and after. */
export interface PreviewLine {
  /** 1-based line where the spot starts. */
  line: number;
  before: string;
  after: string;
  /** Occurrences replaced here. */
  replaced: number;
  /** Occurrences here left alone (locked zone). */
  locked: number;
}

const PREVIEW_MAX = 300;

function clip(s: string): string {
  return s.length > PREVIEW_MAX ? `${s.slice(0, PREVIEW_MAX)}…` : s;
}

/**
 * The before/after lines `replaceAll` would produce for `text`, without changing anything.
 * Occurrences on the same line share one entry.
 */
export function previewReplace(
  text: string,
  matches: ReadonlyArray<Match>,
  replacement: string,
  opts: ReplaceOptions = {},
): PreviewLine[] {
  const template = prepare(replacement, opts);
  const lineStartOf = (pos: number) => (pos === 0 ? 0 : text.lastIndexOf("\n", pos - 1) + 1);
  const lineEndOf = (pos: number) => {
    const i = text.indexOf("\n", pos);
    return i === -1 ? text.length : i;
  };
  const out: PreviewLine[] = [];
  let i = 0;
  while (i < matches.length) {
    const first = matches[i] as Match;
    const start = lineStartOf(first.from);
    let end = lineEndOf(first.to);
    let j = i;
    while (j + 1 < matches.length && lineStartOf((matches[j + 1] as Match).from) <= end) {
      j++;
      end = Math.max(end, lineEndOf((matches[j] as Match).to));
    }
    let after = "";
    let at = start;
    let replaced = 0;
    let locked = 0;
    for (let k = i; k <= j; k++) {
      const m = matches[k] as Match;
      after += text.slice(at, m.from);
      if (opts.skip?.({ from: m.from, to: m.to })) {
        locked++;
        after += m.matched;
      } else {
        replaced++;
        after += replacementFor(template, m, opts);
      }
      at = m.to;
    }
    after += text.slice(at, end);
    out.push({
      line: first.line,
      before: clip(text.slice(start, end)),
      after: clip(after),
      replaced,
      locked,
    });
    i = j + 1;
  }
  return out;
}
