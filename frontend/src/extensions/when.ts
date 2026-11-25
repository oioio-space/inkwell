/**
 * `when` clauses (ADR 0027, « Commands, menus and keybindings »): a bounded expression over a
 * closed context, parsed once when the manifest is read (an unknown key or a value outside its
 * enum is refused there) and evaluated as a tree, never as code (the CSP forbids `eval`). No
 * regular expression: `=~` is refused.
 *
 *   or := and ('||' and)* ; and := not ('&&' not)* ; not := '!' not | '(' or ')' | atom
 *   atom := key | key ('==' | '!=') value | value 'in' key
 */

/** The formatting features of the open file (`editor/format`), the values of `formats`. */
export const FORMATS = [
  "bold",
  "italic",
  "underline",
  "strike",
  "link",
  "code",
  "codeblock",
  "formula",
  "ul",
  "ol",
  "task",
  "quote",
  "table",
  "h1",
  "h2",
  "h3",
  "h4",
  "paragraph",
] as const;

type KeyType =
  | { readonly kind: "bool" }
  | { readonly kind: "enum"; readonly values: readonly string[] }
  /** Any identifier: a file type no loaded extension contributes is valid and never equal. */
  | { readonly kind: "id" }
  | { readonly kind: "set"; readonly values: readonly string[] };

const BOOL = { kind: "bool" } as const;

/** The closed context: exactly the keys a built-in uses (a test refuses an unused one). */
export const CONTEXT_KEYS = {
  focus: { kind: "enum", values: ["editor", "grid", "diff", "other"] },
  field: BOOL,
  fileType: { kind: "id" },
  formats: { kind: "set", values: FORMATS },
  readOnly: BOOL,
  hasToolbar: BOOL,
  toolbarVisible: BOOL,
  previewOpen: BOOL,
  sidebarOpen: BOOL,
  reviewOpen: BOOL,
  canPresent: BOOL,
  canReturn: BOOL,
  /** The open file shows text: a caret, words, a language (EX0c1, the status bar). */
  text: BOOL,
  /** The open file is the project's main document. */
  main: BOOL,
} as const satisfies Record<string, KeyType>;

export type ContextKey = keyof typeof CONTEXT_KEYS;
export type Focus = "editor" | "grid" | "diff" | "other";

/** The live context a clause is evaluated against. */
export interface Context {
  focus: Focus;
  field: boolean;
  fileType: string;
  formats: ReadonlySet<string>;
  readOnly: boolean;
  hasToolbar: boolean;
  toolbarVisible: boolean;
  previewOpen: boolean;
  sidebarOpen: boolean;
  reviewOpen: boolean;
  canPresent: boolean;
  canReturn: boolean;
  text: boolean;
  main: boolean;
}

/** The context of a window where nothing is open. */
export const emptyContext = (): Context => ({
  focus: "other",
  field: false,
  fileType: "",
  formats: new Set(),
  readOnly: false,
  hasToolbar: false,
  toolbarVisible: false,
  previewOpen: false,
  sidebarOpen: false,
  reviewOpen: false,
  canPresent: false,
  canReturn: false,
  text: false,
  main: false,
});

export type When =
  | { readonly op: "key"; readonly key: ContextKey }
  | { readonly op: "==" | "!="; readonly key: ContextKey; readonly value: string }
  | { readonly op: "in"; readonly key: ContextKey; readonly value: string }
  | { readonly op: "!"; readonly arg: When }
  | { readonly op: "&&" | "||"; readonly args: readonly When[] };

export type WhenReason =
  | "bad-syntax"
  | "unknown-key"
  | "bad-value"
  | "bad-type"
  | "too-long"
  | "too-deep";

export class WhenError extends Error {
  constructor(readonly reason: WhenReason) {
    super(`when: ${reason}`);
    this.name = "WhenError";
  }
}

export const MAX_WHEN = 256;
export const MAX_WHEN_DEPTH = 8;

/** How many clauses were parsed and evaluated (C-12: once each, and never while typing). */
export const whenStats = { parsed: 0, evaluated: 0 };

const TOKEN = /\s*(\(|\)|!=|==|&&|\|\||!|[A-Za-z0-9][A-Za-z0-9.-]*|\S)/y;

function tokenize(text: string): string[] {
  const out: string[] = [];
  TOKEN.lastIndex = 0;
  while (TOKEN.lastIndex < text.length) {
    const at = TOKEN.lastIndex;
    const m = TOKEN.exec(text);
    if (!m) {
      if (text.slice(at).trim() === "") break;
      throw new WhenError("bad-syntax");
    }
    out.push(m[1] as string);
  }
  return out;
}

const isWord = (t: string | undefined): t is string => t !== undefined && /^[A-Za-z0-9]/.test(t);

function contextKey(word: string): ContextKey {
  if (!Object.hasOwn(CONTEXT_KEYS, word)) throw new WhenError("unknown-key");
  return word as ContextKey;
}

/** Parses a clause; throws a {@link WhenError} naming the reason. */
export function parseWhen(text: string): When {
  if (text.length > MAX_WHEN) throw new WhenError("too-long");
  const tokens = tokenize(text);
  let at = 0;
  const peek = () => tokens[at];
  const take = () => tokens[at++];

  const atom = (): When => {
    const first = take();
    if (!isWord(first)) throw new WhenError("bad-syntax");
    if (peek() === "in") {
      take();
      const name = take();
      if (!isWord(name)) throw new WhenError("bad-syntax");
      const key = contextKey(name);
      const type: KeyType = CONTEXT_KEYS[key];
      if (type.kind !== "set") throw new WhenError("bad-type");
      if (!type.values.includes(first)) throw new WhenError("bad-value");
      return { op: "in", key, value: first };
    }
    const key = contextKey(first);
    const type: KeyType = CONTEXT_KEYS[key];
    const op = peek();
    if (op === "==" || op === "!=") {
      take();
      const value = take();
      if (!isWord(value)) throw new WhenError("bad-syntax");
      if (type.kind === "enum" && !type.values.includes(value)) throw new WhenError("bad-value");
      if (type.kind === "bool" || type.kind === "set") throw new WhenError("bad-type");
      return { op, key, value };
    }
    if (type.kind !== "bool") throw new WhenError("bad-type");
    return { op: "key", key };
  };

  const not = (depth: number): When => {
    if (depth > MAX_WHEN_DEPTH) throw new WhenError("too-deep");
    if (peek() === "!") {
      take();
      return { op: "!", arg: not(depth + 1) };
    }
    if (peek() === "(") {
      take();
      const inner = or(depth + 1);
      if (take() !== ")") throw new WhenError("bad-syntax");
      return inner;
    }
    return atom();
  };
  const and = (depth: number): When => {
    const args = [not(depth)];
    while (peek() === "&&") {
      take();
      args.push(not(depth));
    }
    return args.length === 1 ? (args[0] as When) : { op: "&&", args };
  };
  const or = (depth: number): When => {
    const args = [and(depth)];
    while (peek() === "||") {
      take();
      args.push(and(depth));
    }
    return args.length === 1 ? (args[0] as When) : { op: "||", args };
  };

  const tree = or(0);
  if (at !== tokens.length) throw new WhenError("bad-syntax");
  whenStats.parsed++;
  return deepFreeze(tree);
}

/** A parsed clause is frozen down to its leaves, like the rest of the registry. */
function deepFreeze(w: When): When {
  if (w.op === "!") deepFreeze(w.arg);
  if (w.op === "&&" || w.op === "||") {
    for (const a of w.args) deepFreeze(a);
    Object.freeze(w.args);
  }
  return Object.freeze(w);
}

function holds(w: When, ctx: Context): boolean {
  switch (w.op) {
    case "key":
      return ctx[w.key] === true;
    case "==":
      return ctx[w.key] === w.value;
    case "!=":
      return ctx[w.key] !== w.value;
    case "in":
      return (ctx[w.key] as ReadonlySet<string>).has(w.value);
    case "!":
      return !holds(w.arg, ctx);
    case "&&":
      return w.args.every((a) => holds(a, ctx));
    case "||":
      return w.args.some((a) => holds(a, ctx));
  }
}

/** Evaluates a parsed clause; no clause holds always. */
export function evalWhen(w: When | undefined, ctx: Context): boolean {
  if (w === undefined) return true;
  whenStats.evaluated++;
  return holds(w, ctx);
}

/** The context keys a clause reads (C-2: every key is used by a built-in). */
export function keysOf(w: When): ContextKey[] {
  switch (w.op) {
    case "!":
      return keysOf(w.arg);
    case "&&":
    case "||":
      return w.args.flatMap(keysOf);
    default:
      return [w.key];
  }
}
