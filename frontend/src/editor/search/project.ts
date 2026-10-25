import { next as A } from "@automerge/automerge";
import { FileKind } from "../../store/files";
import { type FindOptions, findAll, type Match, type Range } from "./find";
import type { Matcher } from "./query";
import { DEFAULT_LIMIT } from "./scan";

/** A project file as the search sees it. `text()` is only ever called for text kinds. */
export interface ProjectFileInput {
  path: string;
  kind: FileKind;
  text(): string | Promise<string>;
}

/** Position of a hit in the live document, as Automerge cursors (they follow edits). */
export interface HitAnchor {
  start: A.Cursor;
  /** On the last matched character; absent for an empty match. */
  last?: A.Cursor;
}

/** An occurrence that can follow concurrent edits and tell when it no longer holds. */
export interface Hit extends Match {
  anchor?: HitAnchor;
}

export interface FileHits {
  path: string;
  hits: Hit[];
  /** The per-file limit was reached: more occurrences exist. */
  truncated: boolean;
}

export interface ProjectHits {
  /** Only files with at least one hit, in the order given. */
  files: FileHits[];
  total: number;
  /** Files whose text was searched. */
  scanned: number;
}

export interface ProjectOptions {
  runner?: FindOptions["runner"] | undefined;
  limitPerFile?: number | undefined;
  /** Only this part of every file (the selection, when one file is searched). */
  range?: Range | undefined;
  /** Where to anchor a file's hits (its Automerge document), if it is open. */
  anchor?: ((path: string) => A.Doc<{ text: string }> | undefined) | undefined;
  /** "Search in these results": only these lines (1-based) of these files. */
  restrictTo?: ReadonlyMap<string, ReadonlySet<number>> | undefined;
}

const TEXT_KINDS: ReadonlySet<FileKind> = new Set([
  FileKind.typst,
  FileKind.markdown,
  FileKind.text,
  FileKind.csv,
  FileKind.bib,
]);

/** Kinds whose text is searched; images, PDFs and data blobs are never read. */
export function isSearchable(kind: FileKind): boolean {
  return TEXT_KINDS.has(kind);
}

// --- File filters: "*.typ *.md !drafts/" -----------------------------------------------

export type FileFilter = (path: string) => boolean;

function globSource(glob: string): string {
  let re = "";
  for (let i = 0; i < glob.length; i++) {
    const c = glob.charAt(i);
    if (c === "*") {
      if (glob.charAt(i + 1) === "*") {
        re += ".*";
        i++;
      } else re += "[^/]*";
    } else if (c === "?") re += "[^/]";
    else re += c.replace(/[\\^$.+()[\]{}|/]/g, "\\$&");
  }
  return re;
}

/** One filter token as a regular expression on the project-relative path. */
function tokenRegex(pattern: string): RegExp {
  if (pattern.endsWith("/")) {
    // A folder, at any depth unless the pattern has its own path.
    return new RegExp(`(?:^|/)${globSource(pattern.replace(/^\/+/, ""))}`, "iu");
  }
  const anywhere = pattern.includes("/") ? "^" : "(?:^|/)";
  return new RegExp(`${anywhere}${globSource(pattern.replace(/^\/+/, ""))}$`, "iu");
}

/**
 * Parses space-separated filters. `*.typ` keeps matching files (at any depth unless the pattern
 * has a `/`); `src/` keeps a folder; `!drafts/` leaves a folder out, `!*.bak` a kind of file.
 * With no keeping pattern every file is kept; leaving out always wins.
 */
export function parseFilters(spec: string): FileFilter {
  const keep: RegExp[] = [];
  const drop: RegExp[] = [];
  for (const token of spec.split(/\s+/).filter(Boolean)) {
    const negated = token.startsWith("!");
    const pattern = negated ? token.slice(1) : token;
    if (pattern) (negated ? drop : keep).push(tokenRegex(pattern));
  }
  return (path) =>
    !drop.some((re) => re.test(path)) && (keep.length === 0 || keep.some((re) => re.test(path)));
}

// --- Searching ---------------------------------------------------------------------------

function anchorHit(doc: A.Doc<{ text: string }>, m: Match): Hit {
  try {
    const start = A.getCursor(doc, ["text"], m.from);
    if (m.to === m.from) return { ...m, anchor: { start } };
    return { ...m, anchor: { start, last: A.getCursor(doc, ["text"], m.to - 1) } };
  } catch {
    return m;
  }
}

/**
 * Searches every text file the filters keep, grouped by file. Binary files are skipped without
 * being read. When `anchor` yields the file's document, each hit gets Automerge cursors so it
 * keeps pointing at the same words while other people type.
 */
export async function findInProject(
  files: ReadonlyArray<ProjectFileInput>,
  matcher: Matcher,
  filters = "",
  opts: ProjectOptions = {},
): Promise<ProjectHits> {
  const keep = parseFilters(filters);
  const limit = opts.limitPerFile ?? DEFAULT_LIMIT;
  const out: FileHits[] = [];
  let total = 0;
  let scanned = 0;
  if (matcher.empty) return { files: out, total, scanned };
  for (const file of files) {
    if (!isSearchable(file.kind) || !keep(file.path)) continue;
    const lines = opts.restrictTo?.get(file.path);
    if (opts.restrictTo && !lines) continue;
    const text = await file.text();
    scanned++;
    let found = await findAll(text, matcher, { ...opts.range, limit, runner: opts.runner });
    const truncated = found.length >= limit;
    if (lines) found = found.filter((m) => lines.has(m.line));
    if (found.length === 0) continue;
    const doc = opts.anchor?.(file.path);
    const hits: Hit[] = doc ? found.map((m) => anchorHit(doc, m)) : found;
    out.push({ path: file.path, hits, truncated });
    total += hits.length;
  }
  return { files: out, total, scanned };
}

/** Where a hit is now, in a document that may have changed since the search. */
export interface ResolvedHit {
  from: number;
  to: number;
  /** The words at that place are no longer the ones that were found. */
  stale: boolean;
}

/**
 * Follows a hit through concurrent edits (SRC-12). A hit with cursors moves with its text; one
 * without is checked in place. Either way it is stale unless the very same words are still there.
 */
export function resolveHit(doc: A.Doc<{ text: string }>, hit: Hit): ResolvedHit {
  const text = doc.text;
  let from = hit.from;
  let to = hit.to;
  if (hit.anchor) {
    try {
      from = A.getCursorPosition(doc, ["text"], hit.anchor.start);
      to = hit.anchor.last ? A.getCursorPosition(doc, ["text"], hit.anchor.last) + 1 : from;
    } catch {
      return { from: hit.from, to: hit.to, stale: true };
    }
  }
  const inBounds = from >= 0 && to >= from && to <= text.length;
  return { from, to, stale: !inBounds || text.slice(from, to) !== hit.matched };
}
