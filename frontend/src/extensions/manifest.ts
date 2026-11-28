/**
 * The manifest of an extension (`inkwell-plugin.json`, ADR 0027 rule 2): JSON text in, a strictly
 * checked, frozen, null-prototype value out. The one parser every manifest goes through, built-in
 * or not, so a built-in is read exactly as a plugin will be. Hand-written: a schema compiler such
 * as ajv builds code with `new Function`, which the CSP forbids.
 *
 * Nothing here is a regular expression taken from a manifest: matching is by words (extensions,
 * whole names, interpreter words), and a value that looks like a pattern is refused.
 */

import {
  chordProblem,
  type Delegated,
  isFunctionKey,
  parseChord,
  SCOPES,
  type Scope,
  SYSTEMS,
} from "./keys";
import { CONTEXT_KEYS, type ContextKey, INSERTS, parseWhen, type When, WhenError } from "./when";

/** Who loaded a manifest. The loader stamps it; a manifest cannot claim one. */
export type Origin = "builtin" | "plugin" | "project";

/** How a type's files are read: decoded as text by Go, or kept as bytes. */
export type Storage = "text" | "blob";

/** The capability that lets a built-in reach the editor's internals (X-3, the typing path). */
export const NATIVE = "ui.native";

/**
 * The core commands a viewer may handle on its own surface besides its extension's own (review
 * major 1): what acts on the focused view (undo, zoom, find, the next difference) and the
 * built-in formatting commands, which act on whichever text view is the target. Save, the
 * palette or a presentation are never a viewer's.
 */
export const DELEGABLE: ReadonlySet<string> = new Set([
  "app.undo",
  "app.redo",
  "app.zoom-in",
  "app.zoom-out",
  "app.zoom-reset",
  "app.find",
  "app.replace",
  "app.find-next",
  "app.find-previous",
  "app.next-change",
  "app.previous-change",
  ...[
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
    // The Insert menu's items (EX0c3): the text inserts, or opens the menu at the item's step.
    "insert-image",
    "insert-figure",
    "insert-include",
    "insert-attach",
    "insert-csv-table",
    "insert-data",
    "insert-bibliography",
    "insert-file-link",
    "insert-cite",
    "insert-ref",
    "insert-footnote",
    "insert-outline",
    "insert-pagebreak",
  ].map((f) => `format.${f}`),
]);

export const CATEGORIES = [
  "document",
  "edit",
  "format",
  "view",
  "navigate",
  "present",
  "app",
] as const;
export type Category = (typeof CATEGORIES)[number];
/**
 * A group of a location (EX0c1): its place is fixed by the table, never by a contribution. A
 * protected group takes items of an essential built-in only (§15.13 « intouchable »); `look` is
 * how its items are drawn: the command's icon, its label, or its label as the suggested action.
 */
export interface GroupSpec {
  readonly id: string;
  readonly protected?: true;
  readonly look?: "icon" | "label" | "suggested";
  /** The group's heading in a menu (an i18n key; GNOME's and APG's group headings). */
  readonly caption?: string;
}

/**
 * A place where commands show (plan EX0c §1): its groups in their fixed order, what happens to
 * what does not fit, and, for a location whose host is not built yet, the block that builds it
 * (`reserved`: a built-in item there is refused, another origin's is kept out and listed).
 */
export interface LocationSpec {
  readonly groups: readonly GroupSpec[];
  readonly overflow?: "width" | "submenu";
  readonly reserved?: string;
}

const icon = { look: "icon" } as const;
const locked = { protected: true } as const;
/** The context menus of §15.13, reserved for UX1-D: the core's items, then the extensions'. */
const contextMenu = {
  groups: [{ id: "core", ...locked }, { id: "extensions" }],
  overflow: "submenu",
  reserved: "UX1-D",
} as const;

/** The closed table of every location (spec §4.31, architecture §15.13), hosted or reserved. */
export const LOCATIONS = {
  main: {
    groups: [
      { id: "view" },
      { id: "insert" },
      { id: "export" },
      { id: "project", ...locked },
      { id: "document" },
      { id: "extensions" },
      { id: "app", ...locked },
    ],
  },
  "header/start": { groups: [{ id: "panels", ...locked, ...icon }] },
  "header/end": {
    groups: [
      { id: "view", ...locked, ...icon },
      { id: "document", ...locked, look: "suggested" },
      { id: "review", ...locked, ...icon },
    ],
  },
  format: {
    groups: [
      { id: "marks", ...icon },
      { id: "inline", ...icon },
      { id: "blocks", ...icon },
      { id: "objects", ...icon },
      { id: "extensions", ...icon },
    ],
    overflow: "width",
  },
  "format/heading": { groups: [{ id: "paragraph" }, { id: "headings" }] },
  "status/zoom": { groups: [{ id: "zoom" }] },
  "preview/toolbar": {
    groups: [
      { id: "zoom-out", ...icon },
      { id: "zoom-in", ...icon },
      { id: "navigate", ...icon },
      { id: "extensions", ...icon },
    ],
  },
  // EX0c2: the grid's bar ends with ‹show text›; sort holds the filter, as the bar did (one group).
  "grid/toolbar": {
    groups: [
      { id: "history", ...locked, ...icon },
      { id: "structure", ...icon },
      { id: "sort", ...icon },
      { id: "freeze", ...icon },
      { id: "extensions", ...icon },
    ],
    overflow: "width",
  },
  "grid/context": {
    groups: [
      { id: "clipboard" },
      { id: "structure" },
      { id: "sort" },
      { id: "freeze" },
      { id: "extensions" },
    ],
    overflow: "submenu",
  },
  // A pick is no command (EX0c1's encoding): ‹default›, ‹offer›, ‹colour›, ‹mode› and ‹close›
  // are fixed widgets of their hosts, so only what runs a command is a group.
  view: { groups: [{ id: "extensions", look: "label" }] },
  "history/toolbar": {
    groups: [
      { id: "seen", look: "label" },
      { id: "extensions", look: "label" },
    ],
  },
  "diff/toolbar": {
    groups: [
      { id: "navigate", look: "label" },
      { id: "extensions", look: "label" },
    ],
  },
  // EX0c3: the Insert menu, its groups captioned as they always were.
  insert: {
    groups: [
      { id: "files", caption: "insert.group.files" },
      { id: "refs", caption: "insert.group.refs" },
      { id: "layout", caption: "insert.group.layout" },
      { id: "extensions" },
    ],
  },
  "editor/context": { ...contextMenu },
  "files/context": { ...contextMenu },
  "tabs/context": { ...contextMenu },
  "preview/context": { ...contextMenu },
  "people/context": { ...contextMenu },
  "link/context": { ...contextMenu },
} as const satisfies Record<string, LocationSpec>;

export type Location = keyof typeof LOCATIONS;
export const locationOf = (id: string): LocationSpec | undefined =>
  Object.hasOwn(LOCATIONS, id) ? (LOCATIONS as Record<string, LocationSpec>)[id] : undefined;

/**
 * What a status group does when room runs out (today's rules of the bar, now per group): `text`
 * drops its words in a window of 900 px or less (`@media`), `hide` leaves a bar of 640 px or less
 * (`@container`), `more` shows 3 items, then ⋯, and all of them in ⋯ at 640 px.
 */
export interface StatusGroupSpec extends GroupSpec {
  readonly narrow?: "text" | "hide" | "more";
}

/** The status bar's two sides and their groups, in their fixed order (plan EX0c §1). */
export const STATUS = {
  start: { groups: [{ id: "presence", ...locked }] },
  end: {
    groups: [
      { id: "problems", narrow: "text" },
      { id: "preview", narrow: "hide" },
      { id: "save", ...locked },
      { id: "caret" },
      { id: "file" },
      { id: "type", narrow: "hide" },
      { id: "extensions", narrow: "more" },
    ],
  },
} as const satisfies Record<string, { groups: readonly StatusGroupSpec[] }>;

export type StatusSide = keyof typeof STATUS;

/**
 * The window's two sides that hold panels (EX0c2): each named by its label (the side's and its
 * tab list's accessible name) and opened by its toggle command. The left one affects the main view
 * (the files), the right one is subordinate to it (Relire), as the GNOME HIG puts utility panes.
 */
export const SIDES = {
  start: { label: "sidebar.label", toggle: "app.sidebar" },
  end: { label: "review.title", toggle: "app.review" },
} as const;

export type Side = keyof typeof SIDES;

export type OptionValue = string | number | boolean;

export interface FileTypeContribution {
  readonly id: string;
  /** An i18n key for a built-in. */
  readonly label: string;
  /** A name in the icon set. */
  readonly icon: string;
  /** With the dot, lower case, several dots allowed (`.tar.gz`). */
  readonly extensions: readonly string[];
  /** Whole names, compared without case (`Makefile`, `.gitignore`). */
  readonly filenames: readonly string[];
  /** Interpreter words of a `#!` first line (`bash`, `python3`). */
  readonly shebangs: readonly string[];
  readonly mime?: string | undefined;
  readonly storage: Storage;
  readonly grammar?: string | undefined;
  /** Handed to the viewer, for example a table's delimiter. */
  readonly options: Readonly<Record<string, OptionValue>>;
}

export interface ViewerContribution {
  readonly id: string;
  readonly fileTypes: readonly string[];
  readonly label: string;
  /** A relative path inside the extension's folder. */
  readonly entry: string;
  readonly capabilities: readonly string[];
  /** The commands its surface handles: its extension's own, or {@link DELEGABLE} ones. */
  readonly commands: readonly string[];
  /** It draws its own status bar: the shell's and the preview stand aside (the grid, EX0c2). */
  readonly ownStatus: boolean;
}

export interface GrammarContribution {
  readonly id: string;
  /** The language's name (an i18n key for a built-in): the status bar's language, the bar's name. */
  readonly label: string;
  /** Fenced-block info strings and aliases. */
  readonly names: readonly string[];
  readonly entry: string;
  /** A grammar in this slice is Lezer code in the webview: it declares `ui.native`. */
  readonly capabilities: readonly string[];
  /** Its formatting (EX0c3): a grammar without it has no bar and no Insert menu. */
  readonly format?: GrammarFormat | undefined;
}

/** A grammar's formatting language (EX0c3): its entry, and the file types its Insert items take. */
export interface GrammarFormat {
  /** A `native/` module of a built-in, whose default export is the language's format module. */
  readonly entry: string;
  /** The file types an Insert item lists (type ids); an item absent takes every file. */
  readonly accepts: Readonly<Record<string, readonly string[]>>;
  /** The data reader of a file type, for an item that reads one (Typst's `csv`, `json`…). */
  readonly readers: Readonly<Record<string, string>>;
}

export interface CommandContribution {
  /** `<extension>.<name>`, the extension being the manifest's own. */
  readonly id: string;
  /** i18n keys for a built-in. */
  readonly label: string;
  readonly tooltip?: string | undefined;
  readonly keywords: readonly string[];
  readonly icon?: string | undefined;
  readonly category: Category;
  /** When it can run; the person is told `unavailable` otherwise. */
  readonly when?: When | undefined;
  readonly unavailable?: string | undefined;
  /** A boolean context key: the command is a toggle, shown pressed or checked by it. */
  readonly toggled?: ContextKey | undefined;
  /** A held key runs it again (zoom, next change, next problem). */
  readonly repeat: boolean;
  /** The module that handles it when no surface does; built-ins only until EXT1. */
  readonly entry?: string | undefined;
  readonly capabilities: readonly string[];
}

export interface MenuContribution {
  readonly command: string;
  readonly location: Location;
  /** One of its location's groups. */
  readonly group: string;
  readonly order: number;
  /** When the item is shown at all. */
  readonly when?: When | undefined;
}

/**
 * An item of the status bar (EX0c1): code, since the core's items show live values; built-ins
 * only until EXT1. Its entry exports `status[id]`, the item's component.
 */
export interface StatusContribution {
  readonly id: string;
  /** i18n key: its name in the ⋯ menu, and the start of its tooltip « X : … ». */
  readonly label: string;
  readonly side: StatusSide;
  readonly group: string;
  readonly order: number;
  readonly when?: When | undefined;
  /** What a click runs. */
  readonly command?: string | undefined;
  readonly entry: string;
  readonly capabilities: readonly string[];
}

/**
 * A panel of a side (EX0c2, VS Code's `views` in a `viewsContainers`): a tab of the drawer, or the
 * left side's body. Code, like a status item; built-ins only until EXT1. Its entry exports
 * `panels[id]`, the body's component.
 */
export interface PanelContribution {
  readonly id: string;
  /** i18n key: the tab's name and the panel's accessible name. */
  readonly label: string;
  readonly side: Side;
  readonly order: number;
  /** When its tab shows at all. */
  readonly when?: When | undefined;
  /** A name in the icon set, for where its name does not fit. */
  readonly icon: string;
  readonly entry: string;
  readonly capabilities: readonly string[];
}

export interface KeybindingContribution {
  /** One of this manifest's own commands. */
  readonly command: string;
  /** Up to three chords per system, the first one shown; none is no binding there. */
  readonly linux: readonly string[];
  readonly mac: readonly string[];
  readonly win: readonly string[];
  readonly scope: Scope;
  readonly when?: When | undefined;
  /** Built-ins only: the command or owned key (keytables.ts) this binding hides in its scope. */
  readonly masks: readonly string[];
  readonly delegated?: Delegated | undefined;
}

export const METRIC_KINDS = ["counter", "duration", "volume"] as const;
export type MetricKind = (typeof METRIC_KINDS)[number];
/** The units each kind may be measured in (plan OBS1 §2): events, milliseconds, an amount. */
export const METRIC_UNITS: Readonly<Record<MetricKind, readonly string[]>> = {
  counter: ["count"],
  duration: ["ms"],
  volume: ["bytes", "words"],
};
export const METRIC_SCOPES = ["app", "document"] as const;
/** The largest bound: the largest integer a number holds exactly. */
export const MAX_METRIC = Number.MAX_SAFE_INTEGER;

/**
 * A metric (plan OBS1 §2): data only. Its values are numbers that pass rule V; it may be split
 * by a closed set of `values` (a failure's class), never by free text.
 */
export interface MetricContribution {
  /** `<extension>.<name>`, the extension being the manifest's own. */
  readonly id: string;
  readonly kind: MetricKind;
  readonly unit: string;
  readonly scope: (typeof METRIC_SCOPES)[number];
  /** An i18n key for a built-in. */
  readonly label: string;
  /** The most one flush may add, an integer from 1 to {@link MAX_METRIC}. */
  readonly max: number;
  readonly values: readonly string[];
}

/** What a field of a debug read may hold: a number, a boolean, a diagnostics code or an enum. */
export type FieldType = "number" | "boolean" | "code" | readonly string[];

/**
 * A debug provider (plan OBS1 §2): its module exports `read()`, which returns a flat record of
 * values. It runs only when its state is read, and every read passes rule V against `fields`.
 */
export interface DebugContribution {
  readonly id: string;
  readonly label: string;
  readonly entry: string;
  /**
   * The value of each field, by name. A key of a read names its field whole or after a
   * pseudonym or an index (`<link>.kind` is the field `kind`, extensions/values.ts).
   */
  readonly fields: Readonly<Record<string, FieldType>>;
}

export interface Contributes {
  readonly fileTypes: readonly FileTypeContribution[];
  readonly viewers: readonly ViewerContribution[];
  readonly grammars: readonly GrammarContribution[];
  readonly commands: readonly CommandContribution[];
  readonly menus: readonly MenuContribution[];
  readonly status: readonly StatusContribution[];
  readonly panels: readonly PanelContribution[];
  readonly keybindings: readonly KeybindingContribution[];
  readonly metrics: readonly MetricContribution[];
  readonly debug: readonly DebugContribution[];
}

export interface Manifest {
  readonly id: string;
  readonly version: string;
  /** Can be replaced, never disabled (X-7). */
  readonly essential: boolean;
  readonly activation: readonly string[];
  readonly contributes: Contributes;
}

/** A manifest as the loader hands it over, before it is parsed. */
export interface ManifestSource {
  readonly origin: Origin;
  /** The extension's folder name. */
  readonly folder: string;
  readonly text: string;
  /**
   * The code files of a built-in's folder, relative to it (`native/grid.ts`): the keys of the
   * build-time glob of that folder. An entry must be one of them.
   */
  readonly files: ReadonlySet<string>;
}

export type Reason =
  | "not-json"
  | "proto-key"
  | "too-deep"
  | "unknown-key"
  | "missing-key"
  | "bad-type"
  | "bad-value"
  | "bad-id"
  | "too-long"
  | "too-many"
  | "pattern"
  | "bad-entry"
  | "native-refused"
  | "native-unused"
  /** A command, menu item, binding or viewer naming a command that is not its manifest's. */
  | "not-own"
  /** A chord that is not in the written form, or that its system forbids (spec §8.2). */
  | "bad-chord"
  | "forbidden-chord"
  | "bad-when"
  /** A metric or debug field that would hold text (rule V). */
  | "free-text"
  /** A metric without a finite bound. */
  | "unbounded"
  /** A metric whose unit does not go with its kind. */
  | "inconsistent"
  /** A built-in that is not essential placing an item in a protected group (§15.13). */
  | "protected"
  /** A built-in placing an item in a location whose host is not built yet. */
  | "reserved";

/** A manifest that is refused, with the reason and where in it. */
export class ManifestError extends Error {
  constructor(
    readonly reason: Reason,
    readonly where: string,
  ) {
    super(`${reason} at ${where}`);
    this.name = "ManifestError";
  }
}

const MAX_TEXT = 64 * 1024;
/** Deeper than any manifest needs; bounds the walk over keys (no stack overflow). */
const MAX_DEPTH = 32;
/** A segment of an entry path: no escape (`%`), query, fragment, space or other separator. */
const SEGMENT = /^[A-Za-z0-9._-]+$/;
const MAX_ID = 64;
const MAX_LABEL = 128;
const MAX_WORD = 64;
const MAX_OPTION = 256;
const PROTO_KEYS = new Set(["__proto__", "constructor", "prototype"]);
const ID = /^[a-z0-9][a-z0-9.-]*$/;
const VERSION = /^\d{1,9}\.\d{1,9}\.\d{1,9}$/;
const EXTENSION = /^(\.[a-z0-9_+-]+)+$/;
const FILENAME = /^[A-Za-z0-9._+-]+$/;
const WORD = /^[a-z0-9._+-]+$/;
const MIME = /^[a-z0-9.+-]+\/[a-z0-9.+-]+$/;
/** Characters that make a word a pattern: globs and regular expressions are never accepted. */
const PATTERN = /[*?[\](){}|^$\\]/;
const CAPABILITIES = new Set([NATIVE]);
const STORAGES = new Set<string>(["text", "blob"]);

type Raw = Record<string, unknown>;

const fail = (reason: Reason, where: string): never => {
  throw new ManifestError(reason, where);
};

/**
 * Refuses `__proto__`, `constructor` and `prototype` as keys at any depth, arrays included, and
 * a nest deeper than {@link MAX_DEPTH}.
 */
function refuseProtoKeys(v: unknown, where: string, depth = 0): void {
  if (v === null || typeof v !== "object") return;
  if (depth >= MAX_DEPTH) fail("too-deep", where);
  for (const key of Object.keys(v)) {
    if (PROTO_KEYS.has(key)) fail("proto-key", `${where}.${key}`);
    refuseProtoKeys((v as Raw)[key], `${where}.${key}`, depth + 1);
  }
}

function object(
  v: unknown,
  where: string,
  allowed: readonly string[],
  required: readonly string[],
): Raw {
  if (v === null || typeof v !== "object" || Array.isArray(v)) return fail("bad-type", where);
  const o = v as Raw;
  for (const key of Object.keys(o))
    if (!allowed.includes(key)) fail("unknown-key", `${where}.${key}`);
  for (const key of required) if (!Object.hasOwn(o, key)) fail("missing-key", `${where}.${key}`);
  return o;
}

function string(v: unknown, where: string, max: number): string {
  if (typeof v !== "string") return fail("bad-type", where);
  if (v.length > max) fail("too-long", where);
  return v;
}

function id(v: unknown, where: string): string {
  const s = string(v, where, MAX_ID);
  if (!ID.test(s)) fail("bad-id", where);
  return s;
}

/** A word matched literally: a pattern is refused before its charset is checked. */
function word(v: unknown, where: string, shape: RegExp): string {
  const s = string(v, where, MAX_WORD);
  if (PATTERN.test(s)) fail("pattern", where);
  if (!shape.test(s)) fail("bad-value", where);
  return s;
}

function list<T>(
  v: unknown,
  where: string,
  max: number,
  item: (x: unknown, at: string) => T,
): readonly T[] {
  if (v === undefined) return Object.freeze([]);
  if (!Array.isArray(v)) return fail("bad-type", where);
  if (v.length > max) fail("too-many", where);
  return Object.freeze(v.map((x, i) => item(x, `${where}[${i}]`)));
}

/** A new null-prototype object, frozen: what every parsed value is made of. */
function frozen<T extends object>(fields: T): Readonly<T> {
  const o = Object.create(null) as T;
  for (const [key, value] of Object.entries(fields))
    if (value !== undefined) (o as Raw)[key] = value;
  return Object.freeze(o);
}

/**
 * An entry: a relative POSIX path inside the extension's folder, normalised (no `.`, `..` or empty
 * segment, no leading `/`, no backslash, drive letter or URL scheme). A built-in's entry must also
 * be a file of its own folder's glob, so it names nothing else.
 */
function entry(v: unknown, where: string, source: ManifestSource): string {
  const s = string(v, where, MAX_LABEL);
  const segments = s.split("/");
  const shapeOk =
    s !== "" &&
    !s.includes("\\") &&
    !/^[A-Za-z][A-Za-z0-9+.-]*:/.test(s) &&
    segments.every((seg) => SEGMENT.test(seg) && seg !== "." && seg !== "..");
  if (!shapeOk) fail("bad-entry", where);
  if (source.origin === "builtin" && !source.files.has(s)) fail("bad-entry", where);
  return s;
}

function capabilities(v: unknown, where: string): readonly string[] {
  return list(v, where, 8, (x, at) => {
    const s = string(x, at, MAX_WORD);
    if (!CAPABILITIES.has(s)) fail("bad-value", at);
    return s;
  });
}

function options(v: unknown, where: string): Readonly<Record<string, OptionValue>> {
  if (v === undefined) return frozen({});
  const o = object(v, where, Object.keys(v ?? {}), []);
  const keys = Object.keys(o);
  if (keys.length > 16) fail("too-many", where);
  const out: Record<string, OptionValue> = {};
  for (const key of keys) {
    if (!ID.test(key) || key.length > MAX_ID) fail("bad-id", `${where}.${key}`);
    const value = o[key];
    if (typeof value === "string") out[key] = string(value, `${where}.${key}`, MAX_OPTION);
    else if (typeof value === "boolean" || (typeof value === "number" && Number.isFinite(value)))
      out[key] = value;
    else fail("bad-type", `${where}.${key}`);
  }
  return frozen(out);
}

function fileType(v: unknown, where: string): FileTypeContribution {
  const o = object(
    v,
    where,
    [
      "id",
      "label",
      "icon",
      "extensions",
      "filenames",
      "shebangs",
      "mime",
      "storage",
      "grammar",
      "options",
    ],
    ["id", "label", "icon", "storage"],
  );
  const storage = string(o.storage, `${where}.storage`, MAX_WORD);
  if (!STORAGES.has(storage)) fail("bad-value", `${where}.storage`);
  const mime = o.mime === undefined ? undefined : string(o.mime, `${where}.mime`, MAX_LABEL);
  if (mime !== undefined && !MIME.test(mime)) fail("bad-value", `${where}.mime`);
  return frozen({
    id: id(o.id, `${where}.id`),
    label: string(o.label, `${where}.label`, MAX_LABEL),
    icon: id(o.icon, `${where}.icon`),
    extensions: list(o.extensions, `${where}.extensions`, 32, (x, at) => word(x, at, EXTENSION)),
    filenames: list(o.filenames, `${where}.filenames`, 32, (x, at) => word(x, at, FILENAME)),
    shebangs: list(o.shebangs, `${where}.shebangs`, 16, (x, at) => word(x, at, WORD)),
    mime,
    storage: storage as Storage,
    grammar: o.grammar === undefined ? undefined : id(o.grammar, `${where}.grammar`),
    options: options(o.options, `${where}.options`),
  });
}

function viewer(
  v: unknown,
  where: string,
  source: ManifestSource,
  owner: string,
): ViewerContribution {
  const o = object(
    v,
    where,
    ["id", "fileTypes", "label", "entry", "capabilities", "commands", "ownStatus"],
    ["id", "fileTypes", "label", "entry"],
  );
  const types = list(o.fileTypes, `${where}.fileTypes`, 64, (x, at) => id(x, at));
  if (types.length === 0) fail("missing-key", `${where}.fileTypes`);
  if (o.ownStatus !== undefined && typeof o.ownStatus !== "boolean")
    fail("bad-type", `${where}.ownStatus`);
  return frozen({
    id: id(o.id, `${where}.id`),
    fileTypes: types,
    label: string(o.label, `${where}.label`, MAX_LABEL),
    // Before the entry: a viewer taking another extension's command is refused for that first.
    commands: list(o.commands, `${where}.commands`, 64, (x, at) => {
      const c = commandId(x, at);
      if (!isOwn(c, owner) && !DELEGABLE.has(c)) fail("not-own", at);
      return c;
    }),
    entry: entry(o.entry, `${where}.entry`, source),
    capabilities: capabilities(o.capabilities, `${where}.capabilities`),
    ownStatus: o.ownStatus === true,
  });
}

const COMMAND_ID = /^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$/;
const isOwn = (command: string, owner: string) => command.startsWith(`${owner}.`);

function commandId(v: unknown, where: string): string {
  const s = string(v, where, MAX_ID);
  if (!COMMAND_ID.test(s)) fail("bad-id", where);
  return s;
}

function when(v: unknown, where: string): When | undefined {
  if (v === undefined) return undefined;
  try {
    return parseWhen(string(v, where, 1024));
  } catch (error) {
    if (error instanceof WhenError) return fail("bad-when", where);
    throw error;
  }
}

function command(
  v: unknown,
  where: string,
  source: ManifestSource,
  owner: string,
): CommandContribution {
  const o = object(
    v,
    where,
    [
      "id",
      "label",
      "tooltip",
      "keywords",
      "icon",
      "category",
      "when",
      "unavailable",
      "toggled",
      "repeat",
      "entry",
      "capabilities",
    ],
    ["id", "label", "category"],
  );
  const cid = commandId(o.id, `${where}.id`);
  if (!isOwn(cid, owner)) fail("not-own", `${where}.id`);
  const category = string(o.category, `${where}.category`, MAX_WORD);
  if (!(CATEGORIES as readonly string[]).includes(category)) fail("bad-value", `${where}.category`);
  const condition = when(o.when, `${where}.when`);
  if (condition !== undefined && o.unavailable === undefined)
    fail("missing-key", `${where}.unavailable`);
  let toggled: ContextKey | undefined;
  if (o.toggled !== undefined) {
    const key = string(o.toggled, `${where}.toggled`, MAX_WORD);
    if (!Object.hasOwn(CONTEXT_KEYS, key) || CONTEXT_KEYS[key as ContextKey].kind !== "bool")
      fail("bad-value", `${where}.toggled`);
    toggled = key as ContextKey;
  }
  if (o.repeat !== undefined && typeof o.repeat !== "boolean") fail("bad-type", `${where}.repeat`);
  // Code outside the built-ins runs only through the plugin host (EXT1): none is taken yet.
  if (o.entry !== undefined && source.origin !== "builtin") fail("bad-entry", `${where}.entry`);
  return frozen({
    id: cid,
    label: string(o.label, `${where}.label`, MAX_LABEL),
    tooltip: o.tooltip === undefined ? undefined : string(o.tooltip, `${where}.tooltip`, MAX_LABEL),
    keywords: list(o.keywords, `${where}.keywords`, 8, (x, at) => string(x, at, MAX_LABEL)),
    icon: o.icon === undefined ? undefined : id(o.icon, `${where}.icon`),
    category: category as Category,
    when: condition,
    unavailable:
      o.unavailable === undefined
        ? undefined
        : string(o.unavailable, `${where}.unavailable`, MAX_LABEL),
    toggled,
    repeat: o.repeat === true,
    entry: o.entry === undefined ? undefined : entry(o.entry, `${where}.entry`, source),
    capabilities: capabilities(o.capabilities, `${where}.capabilities`),
  });
}

/** What a built-in may not do with a group: a protected one is an essential built-in's alone. */
function placeGroup(
  groups: readonly GroupSpec[],
  value: unknown,
  where: string,
  source: ManifestSource,
  essential: boolean,
): string {
  const group = string(value, where, MAX_WORD);
  const spec = groups.find((g) => g.id === group);
  if (!spec) return fail("bad-value", where);
  // Another origin's item is kept out by the registry and listed (ND-3); a built-in is a bug.
  if (spec.protected && source.origin === "builtin" && !essential) fail("protected", where);
  return group;
}

function order(v: unknown, where: string): number {
  if (!Number.isInteger(v) || (v as number) < 0 || (v as number) > 999) fail("bad-value", where);
  return v as number;
}

function menu(
  v: unknown,
  where: string,
  own: ReadonlySet<string>,
  source: ManifestSource,
  essential: boolean,
): MenuContribution {
  const o = object(
    v,
    where,
    ["command", "location", "group", "order", "when"],
    ["command", "location", "group", "order"],
  );
  const cmd = commandId(o.command, `${where}.command`);
  if (!own.has(cmd)) fail("not-own", `${where}.command`);
  const location = string(o.location, `${where}.location`, MAX_WORD);
  const spec = locationOf(location);
  if (!spec) return fail("bad-value", `${where}.location`);
  if (spec.reserved !== undefined && source.origin === "builtin")
    fail("reserved", `${where}.location`);
  return frozen({
    command: cmd,
    location: location as Location,
    group: placeGroup(spec.groups, o.group, `${where}.group`, source, essential),
    order: order(o.order, `${where}.order`),
    when: when(o.when, `${where}.when`),
  });
}

function statusItem(
  v: unknown,
  where: string,
  source: ManifestSource,
  owner: string,
  essential: boolean,
): StatusContribution {
  const o = object(
    v,
    where,
    ["id", "label", "side", "group", "order", "when", "command", "entry", "capabilities"],
    ["id", "label", "side", "group", "order", "entry", "capabilities"],
  );
  const sid = commandId(o.id, `${where}.id`);
  if (!isOwn(sid, owner)) fail("not-own", `${where}.id`);
  // Code outside the built-ins runs only through the plugin host (EXT1), as for commands.
  if (source.origin !== "builtin") fail("bad-entry", `${where}.entry`);
  const side = string(o.side, `${where}.side`, MAX_WORD);
  if (!Object.hasOwn(STATUS, side)) fail("bad-value", `${where}.side`);
  const caps = capabilities(o.capabilities, `${where}.capabilities`);
  if (!caps.includes(NATIVE)) fail("missing-key", `${where}.capabilities`);
  return frozen({
    id: sid,
    label: string(o.label, `${where}.label`, MAX_LABEL),
    side: side as StatusSide,
    group: placeGroup(
      STATUS[side as StatusSide].groups,
      o.group,
      `${where}.group`,
      source,
      essential,
    ),
    order: order(o.order, `${where}.order`),
    when: when(o.when, `${where}.when`),
    command: o.command === undefined ? undefined : commandId(o.command, `${where}.command`),
    entry: entry(o.entry, `${where}.entry`, source),
    capabilities: caps,
  });
}

function panel(
  v: unknown,
  where: string,
  source: ManifestSource,
  owner: string,
): PanelContribution {
  const o = object(
    v,
    where,
    ["id", "label", "side", "order", "when", "icon", "entry", "capabilities"],
    ["id", "label", "side", "order", "icon", "entry", "capabilities"],
  );
  const pid = commandId(o.id, `${where}.id`);
  if (!isOwn(pid, owner)) fail("not-own", `${where}.id`);
  // Code outside the built-ins runs only through the plugin host (EXT1), as for status items.
  if (source.origin !== "builtin") fail("bad-entry", `${where}.entry`);
  const side = string(o.side, `${where}.side`, MAX_WORD);
  if (!Object.hasOwn(SIDES, side)) fail("bad-value", `${where}.side`);
  const caps = capabilities(o.capabilities, `${where}.capabilities`);
  if (!caps.includes(NATIVE)) fail("missing-key", `${where}.capabilities`);
  return frozen({
    id: pid,
    label: string(o.label, `${where}.label`, MAX_LABEL),
    side: side as Side,
    order: order(o.order, `${where}.order`),
    when: when(o.when, `${where}.when`),
    icon: id(o.icon, `${where}.icon`),
    entry: entry(o.entry, `${where}.entry`, source),
    capabilities: caps,
  });
}

function keybinding(
  v: unknown,
  where: string,
  source: ManifestSource,
  own: ReadonlySet<string>,
): KeybindingContribution {
  const o = object(
    v,
    where,
    ["command", "linux", "mac", "win", "scope", "when", "masks", "delegated"],
    ["command", "scope"],
  );
  const cmd = commandId(o.command, `${where}.command`);
  if (!own.has(cmd)) fail("not-own", `${where}.command`);
  const scope = string(o.scope, `${where}.scope`, MAX_WORD) as Scope;
  if (!SCOPES.includes(scope)) fail("bad-value", `${where}.scope`);
  let delegated: Delegated | undefined;
  if (o.delegated !== undefined) {
    const d = string(o.delegated, `${where}.delegated`, MAX_WORD);
    if (d !== "system" && d !== "widget") fail("bad-value", `${where}.delegated`);
    // Like `masks`: only the core says a chord is the system's or a widget's, never captured.
    if (source.origin !== "builtin") fail("bad-value", `${where}.delegated`);
    delegated = d as Delegated;
  }
  const chords = Object.fromEntries(
    SYSTEMS.map((system) => [
      system,
      list(o[system], `${where}.${system}`, 3, (x, at) => {
        const chord = string(x, at, MAX_WORD);
        const problem = chordProblem(chord, system, scope, delegated);
        if (problem === "bad-chord") fail("bad-chord", at);
        if (problem !== undefined) fail("forbidden-chord", at);
        return chord;
      }),
    ]),
  ) as Record<(typeof SYSTEMS)[number], readonly string[]>;
  // K-7: a function key wants Fn on a Mac laptop, so a command on one has a macOS chord without.
  const fkey = (c: string) => isFunctionKey(parseChord(c).key);
  if ([...chords.linux, ...chords.win].some(fkey) && !chords.mac.some((c) => !fkey(c)))
    fail("forbidden-chord", `${where}.mac`);
  const masks = list(o.masks, `${where}.masks`, 8, (x, at) => string(x, at, MAX_ID));
  if (masks.length > 0 && source.origin !== "builtin") fail("bad-value", `${where}.masks`);
  return frozen({
    command: cmd,
    ...chords,
    scope,
    when: when(o.when, `${where}.when`),
    masks,
    delegated,
  });
}

function grammar(v: unknown, where: string, source: ManifestSource): GrammarContribution {
  const o = object(
    v,
    where,
    ["id", "label", "names", "entry", "capabilities", "format"],
    ["id", "label", "names", "entry", "capabilities"],
  );
  const caps = capabilities(o.capabilities, `${where}.capabilities`);
  if (!caps.includes(NATIVE)) fail("missing-key", `${where}.capabilities`);
  return frozen({
    id: id(o.id, `${where}.id`),
    label: string(o.label, `${where}.label`, MAX_LABEL),
    names: list(o.names, `${where}.names`, 32, (x, at) => word(x, at, WORD)),
    entry: entry(o.entry, `${where}.entry`, source),
    capabilities: caps,
    format: o.format === undefined ? undefined : format(o.format, `${where}.format`, source),
  });
}

/** A data reader's name: a Typst function, never a path or an expression. */
const READER = /^[a-z][a-z0-9]*$/;

/** A grammar's `format` (EX0c3): code, so a built-in's only until the plugin host (EXT1). */
function format(v: unknown, where: string, source: ManifestSource): GrammarFormat {
  const o = object(v, where, ["entry", "accepts", "readers"], ["entry"]);
  const table = <T>(raw: unknown, at: string, value: (x: unknown, at: string) => T) => {
    if (raw === undefined) return frozen({}) as Readonly<Record<string, T>>;
    const t = object(raw, at, Object.keys(raw ?? {}), []);
    if (Object.keys(t).length > 32) fail("too-many", at);
    const out: Record<string, T> = {};
    for (const key of Object.keys(t)) out[key] = value(t[key], `${at}.${key}`);
    return frozen(out);
  };
  const items: readonly string[] = INSERTS;
  const accepts = table(o.accepts, `${where}.accepts`, (x, at) => {
    if (!items.includes(at.slice(at.lastIndexOf(".") + 1))) fail("bad-value", at);
    return list(x, at, 32, (t, i) => id(t, i));
  });
  const readers = table(o.readers, `${where}.readers`, (x, at) => {
    id(at.slice(at.lastIndexOf(".") + 1), at);
    return word(x, at, READER);
  });
  if (source.origin !== "builtin") fail("bad-entry", `${where}.entry`);
  return frozen({ entry: entry(o.entry, `${where}.entry`, source), accepts, readers });
}

/** `startup`, `onFileType:<id>`, `onLanguage:<grammar>` or `onCommand:<command>`. */
function activation(v: unknown, where: string): string {
  const s = string(v, where, MAX_ID + 16);
  const [event, arg, extra] = s.split(":");
  const ok =
    extra === undefined &&
    (event === "startup"
      ? arg === undefined
      : event === "onCommand"
        ? !!arg && COMMAND_ID.test(arg)
        : (event === "onFileType" || event === "onLanguage") && !!arg && ID.test(arg));
  if (!ok) fail("bad-value", where);
  return s;
}

/** A debug field's name, as Go's JSON writes it (`rttMs`). */
const FIELD = /^[a-z][A-Za-z0-9]*$/;
/** An enum word: what a metric's or a field's declared values are made of. */
const ENUM_WORD = /^[a-z][a-z0-9_-]*$/;
/** Field types that would let text through: refused by name, with their reason. */
const TEXT_TYPES = new Set(["text", "string", "any"]);

function enumValues(v: unknown, where: string): readonly string[] {
  if (typeof v === "string" && TEXT_TYPES.has(v)) fail("free-text", where);
  return list(v, where, 64, (x, at) => {
    const s = string(x, at, 32);
    if (!ENUM_WORD.test(s)) fail("free-text", at);
    return s;
  });
}

/**
 * A metric. Its id is its manifest's own; `max` is required, finite and an integer: a metric
 * that could add without bound is refused as `unbounded`.
 */
export function parseMetric(v: unknown, where: string, owner: string): MetricContribution {
  const o = object(
    v,
    where,
    ["id", "kind", "unit", "scope", "label", "max", "values"],
    ["id", "kind", "unit", "scope", "label"],
  );
  const mid = commandId(o.id, `${where}.id`);
  if (!isOwn(mid, owner)) fail("not-own", `${where}.id`);
  const kind = string(o.kind, `${where}.kind`, MAX_WORD);
  if (!(METRIC_KINDS as readonly string[]).includes(kind)) fail("bad-value", `${where}.kind`);
  const unit = string(o.unit, `${where}.unit`, MAX_WORD);
  if (!Object.values(METRIC_UNITS).some((units) => units.includes(unit)))
    fail("bad-value", `${where}.unit`);
  if (!METRIC_UNITS[kind as MetricKind].includes(unit)) fail("inconsistent", `${where}.unit`);
  const scope = string(o.scope, `${where}.scope`, MAX_WORD);
  if (!(METRIC_SCOPES as readonly string[]).includes(scope)) fail("bad-value", `${where}.scope`);
  if (typeof o.max !== "number" || !Number.isFinite(o.max) || o.max > MAX_METRIC)
    fail("unbounded", `${where}.max`);
  const max = o.max as number;
  if (!Number.isInteger(max) || max < 1) fail("bad-value", `${where}.max`);
  return frozen({
    id: mid,
    kind: kind as MetricKind,
    unit,
    scope: scope as MetricContribution["scope"],
    label: string(o.label, `${where}.label`, MAX_LABEL),
    max,
    values: enumValues(o.values, `${where}.values`),
  });
}

function fieldType(v: unknown, where: string): FieldType {
  if (v === "number" || v === "boolean" || v === "code") return v;
  if (typeof v === "string")
    return TEXT_TYPES.has(v) ? fail("free-text", where) : fail("bad-value", where);
  const values = enumValues(v, where);
  if (values.length === 0) fail("bad-value", where);
  return values;
}

function debugProvider(
  v: unknown,
  where: string,
  source: ManifestSource,
  owner: string,
): DebugContribution {
  const o = object(
    v,
    where,
    ["id", "label", "entry", "fields"],
    ["id", "label", "entry", "fields"],
  );
  const did = commandId(o.id, `${where}.id`);
  if (!isOwn(did, owner)) fail("not-own", `${where}.id`);
  const f = object(o.fields, `${where}.fields`, Object.keys(o.fields ?? {}), []);
  const names = Object.keys(f);
  if (names.length === 0) fail("missing-key", `${where}.fields`);
  if (names.length > 64) fail("too-many", `${where}.fields`);
  const fields: Record<string, FieldType> = {};
  for (const name of names) {
    if (!FIELD.test(name) || name.length > MAX_ID) fail("bad-id", `${where}.fields.${name}`);
    fields[name] = fieldType(f[name], `${where}.fields.${name}`);
  }
  return frozen({
    id: did,
    label: string(o.label, `${where}.label`, MAX_LABEL),
    entry: entry(o.entry, `${where}.entry`, source),
    fields: frozen(fields),
  });
}

/**
 * Parses and checks a manifest. Throws a {@link ManifestError} naming the reason. `ui.native` is
 * accepted only from a built-in that is also essential, and a built-in folder that holds code
 * must declare it (a `native/` folder is the carve-out of FE-3, never a default).
 */
export function parseManifest(source: ManifestSource): Manifest {
  if (source.text.length > MAX_TEXT) fail("too-long", "manifest");
  let raw: unknown;
  try {
    raw = JSON.parse(source.text);
  } catch {
    return fail("not-json", "manifest");
  }
  refuseProtoKeys(raw, "manifest");
  const o = object(
    raw,
    "manifest",
    ["id", "version", "essential", "activation", "contributes"],
    ["id", "version", "contributes"],
  );
  const version = string(o.version, "manifest.version", 32);
  if (!VERSION.test(version)) fail("bad-value", "manifest.version");
  if (o.essential !== undefined && typeof o.essential !== "boolean")
    fail("bad-type", "manifest.essential");
  const c = object(
    o.contributes,
    "manifest.contributes",
    [
      "fileTypes",
      "viewers",
      "grammars",
      "commands",
      "menus",
      "status",
      "panels",
      "keybindings",
      "metrics",
      "debug",
    ],
    [],
  );
  const owner = id(o.id, "manifest.id");
  const commands = list(c.commands, "manifest.contributes.commands", 128, (x, at) =>
    command(x, at, source, owner),
  );
  const own = new Set(commands.map((x) => x.id));
  const essential = o.essential === true;
  const manifest = frozen({
    id: owner,
    version,
    essential,
    activation: list(o.activation, "manifest.activation", 64, activation),
    contributes: frozen({
      fileTypes: list(c.fileTypes, "manifest.contributes.fileTypes", 64, fileType),
      viewers: list(c.viewers, "manifest.contributes.viewers", 16, (x, at) =>
        viewer(x, at, source, owner),
      ),
      grammars: list(c.grammars, "manifest.contributes.grammars", 16, (x, at) =>
        grammar(x, at, source),
      ),
      commands,
      menus: list(c.menus, "manifest.contributes.menus", 64, (x, at) =>
        menu(x, at, own, source, essential),
      ),
      status: list(c.status, "manifest.contributes.status", 32, (x, at) =>
        statusItem(x, at, source, owner, essential),
      ),
      panels: list(c.panels, "manifest.contributes.panels", 16, (x, at) =>
        panel(x, at, source, owner),
      ),
      keybindings: list(c.keybindings, "manifest.contributes.keybindings", 128, (x, at) =>
        keybinding(x, at, source, own),
      ),
      metrics: list(c.metrics, "manifest.contributes.metrics", 64, (x, at) =>
        parseMetric(x, at, owner),
      ),
      debug: list(c.debug, "manifest.contributes.debug", 16, (x, at) =>
        debugProvider(x, at, source, owner),
      ),
    }),
  });
  const native = [
    ...manifest.contributes.viewers,
    ...manifest.contributes.grammars,
    ...manifest.contributes.commands,
    ...manifest.contributes.status,
    ...manifest.contributes.panels,
  ].some((x) => x.capabilities.includes(NATIVE));
  if (native && (source.origin !== "builtin" || !manifest.essential))
    fail("native-refused", "manifest");
  // Only the `native/` folder is the carve-out: a debug provider's `debug/` module reads only.
  if (
    source.origin === "builtin" &&
    [...source.files].some((f) => f.startsWith("native/")) &&
    !native
  )
    fail("native-unused", "manifest");
  return manifest;
}
