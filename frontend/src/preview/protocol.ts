/**
 * The preview stream (`preview`, no local port: ADR 0009). The webview says which pages it shows,
 * Go compiles the project and answers with the SVG of those pages, the page count and the
 * diagnostics. Frames are JSON; both directions are checked, since a frame is only a frame.
 */

export type Severity = "error" | "warning";

export interface Diagnostic {
  severity: Severity;
  /** Relative to the project, forward slashes; undefined when the message is about no file. */
  file?: string;
  message: string;
  /** `@preview/name:version` when a package is missing. */
  package?: string;
  /** 1-based; undefined when the message has no position. */
  line?: number;
  column?: number;
}

export type FailureKind =
  | "errors"
  | "package"
  | "noMain"
  | "timeout"
  | "memory"
  | "tooLarge"
  | "unsafe"
  | "unavailable"
  | "internal";

export interface Failure {
  kind: FailureKind;
  /** The missing packages, for kind "package". */
  packages: string[];
}

/** A heading or a paragraph of the laid-out document and where it landed (points, from the top left). */
export interface Anchor {
  heading: boolean;
  page: number;
  x: number;
  y: number;
  text: string;
}

export interface ResultMessage {
  type: "result";
  seq: number;
  /** The version of the sources: pages of one `rev` belong together, whichever result brought them. */
  rev: number;
  pageCount: number;
  /** The page number of `pages[0]`. */
  first: number;
  pages: string[];
  diagnostics: Diagnostic[];
  truncated: boolean;
  failure?: Failure | undefined;
}

export type PreviewMessage =
  | { type: "compiling"; seq: number }
  | ResultMessage
  | { type: "anchors"; anchors: Anchor[]; failure?: Failure | undefined };

export type PreviewRequest =
  | { op: "view"; first: number; last: number }
  | { op: "edit"; time: number }
  | { op: "refresh" }
  | { op: "anchors" };

/** What a frame may hold: bounds that keep a damaged frame from filling memory. */
export const MAX_DIAGNOSTICS = 500;
export const MAX_ANCHORS = 20_000;
export const MAX_PAGES = 64;
export const MAX_PAGE_COUNT = 100_000;

const FAILURES: readonly string[] = [
  "errors",
  "package",
  "noMain",
  "timeout",
  "memory",
  "tooLarge",
  "unsafe",
  "unavailable",
  "internal",
];

const decoder = new TextDecoder();
const encoder = new TextEncoder();

export function encodeRequest(req: PreviewRequest): Uint8Array {
  return encoder.encode(JSON.stringify(req));
}

const isObject = (v: unknown): v is Record<string, unknown> =>
  typeof v === "object" && v !== null && !Array.isArray(v);

const int = (v: unknown, max: number): number | undefined =>
  typeof v === "number" && Number.isInteger(v) && v >= 0 && v <= max ? v : undefined;

function parseFailure(v: unknown): Failure | undefined {
  if (!isObject(v) || typeof v.kind !== "string" || !FAILURES.includes(v.kind)) return undefined;
  const packages = Array.isArray(v.packages)
    ? v.packages.filter((p): p is string => typeof p === "string").slice(0, 20)
    : [];
  return { kind: v.kind as FailureKind, packages };
}

function parseDiagnostic(v: unknown): Diagnostic | undefined {
  if (!isObject(v) || (v.severity !== "error" && v.severity !== "warning")) return undefined;
  if (typeof v.message !== "string") return undefined;
  const d: Diagnostic = { severity: v.severity, message: v.message.slice(0, 4000) };
  if (typeof v.file === "string" && v.file !== "") d.file = v.file;
  if (typeof v.package === "string" && v.package !== "") d.package = v.package;
  const line = int(v.line, 1e9);
  if (line) d.line = line;
  const column = int(v.column, 1e9);
  if (column) d.column = column;
  return d;
}

function parseAnchor(v: unknown): Anchor | undefined {
  if (!isObject(v)) return undefined;
  const page = int(v.page, MAX_PAGE_COUNT);
  if (!page || typeof v.x !== "number" || typeof v.y !== "number" || typeof v.text !== "string") {
    return undefined;
  }
  if (!Number.isFinite(v.x) || !Number.isFinite(v.y)) return undefined;
  return { heading: v.heading === true, page, x: v.x, y: v.y, text: v.text };
}

function list<T>(v: unknown, max: number, one: (x: unknown) => T | undefined): T[] {
  if (!Array.isArray(v)) return [];
  const out: T[] = [];
  for (const item of v.slice(0, max)) {
    const parsed = one(item);
    if (parsed) out.push(parsed);
  }
  return out;
}

/** Parses a frame from Go; undefined (never a throw) for anything that is not a known message. */
export function parseMessage(frame: Uint8Array | string): PreviewMessage | undefined {
  let v: unknown;
  try {
    v = JSON.parse(typeof frame === "string" ? frame : decoder.decode(frame));
  } catch {
    return undefined;
  }
  if (!isObject(v)) return undefined;
  switch (v.type) {
    case "compiling": {
      const seq = int(v.seq, Number.MAX_SAFE_INTEGER);
      return seq === undefined ? undefined : { type: "compiling", seq };
    }
    case "result": {
      const seq = int(v.seq, Number.MAX_SAFE_INTEGER);
      if (seq === undefined) return undefined;
      return {
        type: "result",
        seq,
        rev: int(v.rev, Number.MAX_SAFE_INTEGER) ?? 0,
        pageCount: int(v.pageCount, MAX_PAGE_COUNT) ?? 0,
        first: int(v.first, MAX_PAGE_COUNT) ?? 1,
        pages: list(v.pages, MAX_PAGES, (p) => (typeof p === "string" ? p : undefined)),
        diagnostics: list(v.diagnostics, MAX_DIAGNOSTICS, parseDiagnostic),
        truncated: v.truncated === true,
        failure: parseFailure(v.failure),
      };
    }
    case "anchors":
      return {
        type: "anchors",
        anchors: list(v.anchors, MAX_ANCHORS, parseAnchor),
        failure: parseFailure(v.failure),
      };
  }
  return undefined;
}
