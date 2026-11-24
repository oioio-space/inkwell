/**
 * The manifest of an extension (`inkwell-plugin.json`, ADR 0027 rule 2): JSON text in, a strictly
 * checked, frozen, null-prototype value out. The one parser every manifest goes through, built-in
 * or not, so a built-in is read exactly as a plugin will be. Hand-written: a schema compiler such
 * as ajv builds code with `new Function`, which the CSP forbids.
 *
 * Nothing here is a regular expression taken from a manifest: matching is by words (extensions,
 * whole names, interpreter words), and a value that looks like a pattern is refused.
 */

/** Who loaded a manifest. The loader stamps it; a manifest cannot claim one. */
export type Origin = "builtin" | "plugin" | "project";

/** How a type's files are read: decoded as text by Go, or kept as bytes. */
export type Storage = "text" | "blob";

/** The capability that lets a built-in reach the editor's internals (X-3, the typing path). */
export const NATIVE = "ui.native";

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
}

export interface GrammarContribution {
  readonly id: string;
  /** Fenced-block info strings and aliases. */
  readonly names: readonly string[];
  readonly entry: string;
  /** A grammar in this slice is Lezer code in the webview: it declares `ui.native`. */
  readonly capabilities: readonly string[];
}

export interface Contributes {
  readonly fileTypes: readonly FileTypeContribution[];
  readonly viewers: readonly ViewerContribution[];
  readonly grammars: readonly GrammarContribution[];
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
  | "native-unused";

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

function viewer(v: unknown, where: string, source: ManifestSource): ViewerContribution {
  const o = object(
    v,
    where,
    ["id", "fileTypes", "label", "entry", "capabilities"],
    ["id", "fileTypes", "label", "entry"],
  );
  const types = list(o.fileTypes, `${where}.fileTypes`, 64, (x, at) => id(x, at));
  if (types.length === 0) fail("missing-key", `${where}.fileTypes`);
  return frozen({
    id: id(o.id, `${where}.id`),
    fileTypes: types,
    label: string(o.label, `${where}.label`, MAX_LABEL),
    entry: entry(o.entry, `${where}.entry`, source),
    capabilities: capabilities(o.capabilities, `${where}.capabilities`),
  });
}

function grammar(v: unknown, where: string, source: ManifestSource): GrammarContribution {
  const o = object(
    v,
    where,
    ["id", "names", "entry", "capabilities"],
    ["id", "names", "entry", "capabilities"],
  );
  const caps = capabilities(o.capabilities, `${where}.capabilities`);
  if (!caps.includes(NATIVE)) fail("missing-key", `${where}.capabilities`);
  return frozen({
    id: id(o.id, `${where}.id`),
    names: list(o.names, `${where}.names`, 32, (x, at) => word(x, at, WORD)),
    entry: entry(o.entry, `${where}.entry`, source),
    capabilities: caps,
  });
}

/** `startup`, `onFileType:<id>` or `onLanguage:<grammar>`. */
function activation(v: unknown, where: string): string {
  const s = string(v, where, MAX_ID + 16);
  const [event, arg, extra] = s.split(":");
  const ok =
    extra === undefined &&
    (event === "startup"
      ? arg === undefined
      : (event === "onFileType" || event === "onLanguage") && !!arg && ID.test(arg));
  if (!ok) fail("bad-value", where);
  return s;
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
  const c = object(o.contributes, "manifest.contributes", ["fileTypes", "viewers", "grammars"], []);
  const manifest = frozen({
    id: id(o.id, "manifest.id"),
    version,
    essential: o.essential === true,
    activation: list(o.activation, "manifest.activation", 64, activation),
    contributes: frozen({
      fileTypes: list(c.fileTypes, "manifest.contributes.fileTypes", 64, fileType),
      viewers: list(c.viewers, "manifest.contributes.viewers", 16, (x, at) =>
        viewer(x, at, source),
      ),
      grammars: list(c.grammars, "manifest.contributes.grammars", 16, (x, at) =>
        grammar(x, at, source),
      ),
    }),
  });
  const native = [...manifest.contributes.viewers, ...manifest.contributes.grammars].some((x) =>
    x.capabilities.includes(NATIVE),
  );
  if (native && (source.origin !== "builtin" || !manifest.essential))
    fail("native-refused", "manifest");
  if (source.origin === "builtin" && source.files.size > 0 && !native)
    fail("native-unused", "manifest");
  return manifest;
}
