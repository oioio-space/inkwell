/**
 * Portable names (ADR 0023, PRJ-10): the TypeScript twin of internal/project/names.go. Both are
 * checked against the golden vectors in internal/project/testdata/names.json, so a name the
 * webview accepts is a name Go accepts, and the other way round.
 */

/** Limits of portable names. */
export const MAX_NAME_BYTES = 255;
export const MAX_PATH_BYTES = 1024;
/** The deepest folder nesting of a project tree (Go: project.MaxDepth). */
export const MAX_DEPTH = 32;

/** Why a name or a path was refused: the stable codes shared with Go. */
export type Reason =
  | "empty"
  | "too_long"
  | "bad_utf8"
  | "bad_char"
  | "dot"
  | "trailing"
  | "device"
  | "reserved"
  | "short_name"
  | "ignorable"
  | "root_reserved"
  | "empty_component"
  | "path_too_long";

const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true });

/** The names no user file may have at the root of the project, folded. */
const RESERVED_AT_ROOT: readonly string[] = [".inkwell", ".typstdeps"];

/** Windows resolves the 8.3 short names of the reserved names (and of the repository folder). */
const SHORT_NAME = /^(inkwel|typstd|git)~[0-9]+$/i;
const IGNORABLE = /\p{Default_Ignorable_Code_Point}/u;
const LONE_SURROGATE = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/;
const DEVICES: ReadonlySet<string> = new Set([
  "CON",
  "PRN",
  "AUX",
  "NUL",
  "CLOCK$",
  "CONIN$",
  "CONOUT$",
]);
const DEVICE_PORTS: ReadonlySet<string> = new Set([
  "0",
  "1",
  "2",
  "3",
  "4",
  "5",
  "6",
  "7",
  "8",
  "9",
  "¹",
  "²",
  "³",
]);

/** True when `s` has no lone surrogate, i.e. it is a sequence of Unicode scalar values. */
function wellFormed(s: string): boolean {
  return !LONE_SURROGATE.test(s);
}

/**
 * The key under which two sibling names collide: the name in NFC, case-folded, in NFC again.
 * "Notes.md" and "notes.md" share a key, and so do the composed and the decomposed "é" (macOS
 * writes the latter), and "Straße" and "STRASSE". Meant for valid names.
 */
export function collisionKey(name: string): string {
  const folded = name.normalize("NFC").toUpperCase().toLowerCase().replaceAll("ß", "ss");
  return folded.normalize("NFC");
}

function isForbidden(code: number, ch: string): boolean {
  return code < 0x20 || code === 0x7f || '/\\<>:"|?*'.includes(ch);
}

function hasForbidden(name: string): boolean {
  for (const ch of name) {
    if (isForbidden(ch.codePointAt(0) as number, ch)) return true;
  }
  return false;
}

function isDevice(name: string): boolean {
  const dot = name.indexOf(".");
  const stem = (dot < 0 ? name : name.slice(0, dot))
    .replace(/ +$/, "")
    .replace(/[a-z]/g, (c) => c.toUpperCase());
  if (DEVICES.has(stem)) return true;
  const port = stem.startsWith("COM") || stem.startsWith("LPT") ? stem.slice(3) : undefined;
  return port !== undefined && DEVICE_PORTS.has(port);
}

/**
 * Checks one path component against the portable-name rules of ADR 0023, which hold on every
 * platform Inkwell runs on. Returns the reason it is refused, or undefined when it is valid.
 * Whether a name is reserved at the root of the project is a matter of {@link validatePath}.
 */
export function validateName(name: string): Reason | undefined {
  if (name === "") return "empty";
  if (!wellFormed(name))
    return encoder.encode(name).length > MAX_NAME_BYTES ? "too_long" : "bad_utf8";
  if (encoder.encode(name).length > MAX_NAME_BYTES) return "too_long";
  if (hasForbidden(name)) return "bad_char";
  if (name === "." || name === "..") return "dot";
  if (name.endsWith(".") || name.endsWith(" ")) return "trailing";
  if (isDevice(name)) return "device";
  if (SHORT_NAME.test(name)) return "short_name";
  if (IGNORABLE.test(name)) return "ignorable";
  if (collisionKey(name) === ".git") return "reserved";
  return undefined;
}

/** Like {@link validateName}, for the raw UTF-8 bytes of a name (which may not be valid UTF-8). */
export function validateNameBytes(bytes: Uint8Array): Reason | undefined {
  if (bytes.length === 0) return "empty";
  if (bytes.length > MAX_NAME_BYTES) return "too_long";
  try {
    return validateName(decoder.decode(bytes));
  } catch {
    return "bad_utf8";
  }
}

/**
 * Checks a project-relative path: components separated by "/", each accepted by
 * {@link validateName}, at most 1024 bytes in all, and not under a name that is reserved at the
 * root, .inkwell or .typstdeps in any case.
 */
export function validatePath(path: string): Reason | undefined {
  if (path === "") return "empty";
  if (!wellFormed(path)) return "bad_utf8";
  if (encoder.encode(path).length > MAX_PATH_BYTES) return "path_too_long";
  const parts = path.split("/");
  for (const [i, part] of parts.entries()) {
    if (part === "") return "empty_component";
    const reason = validateName(part);
    if (reason) return reason;
    if (i === 0 && RESERVED_AT_ROOT.includes(collisionKey(part))) return "root_reserved";
  }
  return undefined;
}

/** Like {@link validatePath}, for raw UTF-8 bytes. */
export function validatePathBytes(bytes: Uint8Array): Reason | undefined {
  if (bytes.length === 0) return "empty";
  if (bytes.length > MAX_PATH_BYTES) return "path_too_long";
  try {
    return validatePath(decoder.decode(bytes));
  } catch {
    return "bad_utf8";
  }
}

/** Cuts `s` to at most `max` UTF-8 bytes, on a code point boundary. */
function clipBytes(s: string, max: number): string {
  let out = "";
  let used = 0;
  for (const ch of s) {
    const n = encoder.encode(ch).length;
    if (used + n > max) break;
    out += ch;
    used += n;
  }
  return out;
}

/** The name given to something whose own name had nothing left after cleaning. */
export const FALLBACK_NAME = "_";

/**
 * Turns any text into a valid name, changing as little as possible: forbidden characters become
 * "_", invisible ones are dropped, a trailing dot or space is removed, a device or short name
 * gets a "_" in front of it or after it, and the whole is cut to 255 bytes. A valid name is
 * returned unchanged. The result is always valid (a property test says so).
 */
export function sanitizeName(raw: string): string {
  let s = wellFormed(raw) ? raw : raw.replace(new RegExp(LONE_SURROGATE, "g"), "�");
  s = [...s]
    .filter((ch) => !IGNORABLE.test(ch))
    .map((ch) => (isForbidden(ch.codePointAt(0) as number, ch) ? "_" : ch))
    .join("");
  s = clipBytes(s, MAX_NAME_BYTES).replace(/[. ]+$/, "");
  if (s === "") return FALLBACK_NAME;
  if (isDevice(s)) s = clipBytes(`_${s}`, MAX_NAME_BYTES).replace(/[. ]+$/, "");
  if (SHORT_NAME.test(s)) s = `${s}_`;
  if (collisionKey(s) === ".git") s = `_${s.slice(1)}`;
  // Cutting or prefixing can in theory break an earlier rule: fall back rather than loop.
  return validateName(s) === undefined ? s : FALLBACK_NAME;
}

/** Splits a name into stem and extension ("a.tar.gz" gives "a.tar" and ".gz"; ".env" has none). */
export function splitExt(name: string): { stem: string; ext: string } {
  const dot = name.lastIndexOf(".");
  if (dot <= 0) return { stem: name, ext: "" };
  return { stem: name.slice(0, dot), ext: name.slice(dot) };
}

/** The last component of a path. */
export function baseName(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1);
}
