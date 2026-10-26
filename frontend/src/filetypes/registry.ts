import { asText } from "../project/import";
import type { FileType } from "./types";

const types = new Map<string, FileType>();
const byExt = new Map<string, FileType>();
const byName = new Map<string, FileType>();

/** The id of the type that text of an unknown kind opens as, and the one for binary data. */
export const PLAIN = "plaintext";
export const BINARY = "binary";

/**
 * Adds a type. Each module of `filetypes/` calls it once for its type, and nothing else needs to
 * know: the editor, the list, the bar read the registry. A type that claims an extension or a
 * name another one already has is a mistake and throws, so two modules never fight silently.
 */
export function register(type: FileType): void {
  if (types.has(type.id)) throw new Error(`file type "${type.id}" is already registered`);
  for (const e of type.extensions) {
    const owner = byExt.get(e);
    if (owner) throw new Error(`"${e}" is already ${owner.id}`);
  }
  for (const n of type.filenames ?? []) {
    const owner = byName.get(n);
    if (owner) throw new Error(`"${n}" is already ${owner.id}`);
  }
  types.set(type.id, type);
  for (const e of type.extensions) byExt.set(e, type);
  for (const n of type.filenames ?? []) byName.set(n, type);
}

/** Removes a type (for tests that add a fake one). */
export function unregister(id: string): void {
  const type = types.get(id);
  if (!type) return;
  types.delete(id);
  for (const e of type.extensions) if (byExt.get(e) === type) byExt.delete(e);
  for (const n of type.filenames ?? []) if (byName.get(n) === type) byName.delete(n);
}

export const all = (): FileType[] => [...types.values()];
export const get = (id: string): FileType | undefined => types.get(id);

function required(id: string): FileType {
  const t = types.get(id);
  if (!t) throw new Error(`file type "${id}" is not registered`);
  return t;
}

/** What is known of a file when asking what it is. */
export interface Subject {
  /** Its name, or its path. */
  name: string;
  /** An explicit choice kept in the index (ADR 0023): wins over everything. */
  override?: string | undefined;
  /** Its first bytes, when they are at hand. */
  bytes?: Uint8Array | undefined;
  /** True for a file that is a text document; false for a file that is not; undefined if unknown. */
  isText?: boolean | undefined;
}

const base = (name: string) => name.slice(name.lastIndexOf("/") + 1).toLowerCase();

/** The type a file's name says: its whole name, then its last extension. */
export function byFileName(name: string): FileType | undefined {
  const b = base(name);
  const whole = byName.get(b);
  if (whole) return whole;
  const dot = b.lastIndexOf(".");
  return dot > 0 ? byExt.get(b.slice(dot)) : undefined;
}

/** The sniffers' best guess, if one is sure enough. */
function bySniff(bytes: Uint8Array): FileType | undefined {
  let best: FileType | undefined;
  let score = 0.5;
  for (const t of types.values()) {
    const s = t.sniff?.(bytes) ?? 0;
    if (s > score) {
      best = t;
      score = s;
    }
  }
  return best;
}

/**
 * What a file is. In this order: the choice kept in the index; its extension or name; what its
 * bytes look like; plain text if it is text; else the binary viewer. A text document is never
 * given a viewer: if its name says "png", it still opens as text.
 */
export function resolve(s: Subject): FileType {
  const chosen = s.override ? types.get(s.override) : undefined;
  if (chosen && !(s.isText && chosen.binary)) return chosen;
  const named = byFileName(s.name);
  if (named && !(s.isText && named.binary)) return named;
  if (s.bytes && s.isText === undefined) {
    const sniffed = bySniff(s.bytes);
    if (sniffed) return sniffed;
    if (asText(s.name, s.bytes) !== undefined) return required(PLAIN);
    return required(BINARY);
  }
  return required(s.isText === false ? BINARY : PLAIN);
}
