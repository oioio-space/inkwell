import { next as A } from "@automerge/automerge";

/**
 * The project index (ADR 0023): the root Automerge document of a project. The tree is made of
 * nodes with a stable id, a name and a parent, so that renaming or moving never changes what a
 * file is; the visible tree is a pure function of this document ({@link ./materialize}).
 *
 * The document also still holds `text`, the content of the main file: the first projects had
 * only that, and the migration ({@link ./migrate}) keeps it where it is.
 */

export type NodeKind = "dir" | "text" | "blob";

/** Who did something and when (a declaration of the author's device; the clock is theirs). */
export interface Stamp {
  by: string;
  at: number;
}

/** A trash mark. `req` ties it to the deletion request it accepted, if any. */
export interface Trashed extends Stamp {
  req?: string;
}

/** A request of a writer to delete a file, waiting for an admin. */
export interface DeletionRequest extends Stamp {
  req: string;
}

export interface IndexNode extends Stamp {
  type: NodeKind;
  name: string;
  parent: string | null;
  /** The automerge URL of a text file. Never changes. */
  url?: string;
  /** "blake3:" and 64 hex digits, for a blob. */
  hash?: string;
  size?: number;
  trashed?: Trashed | null;
  deletion?: DeletionRequest | null;
  /** An explicit file type (ADR 0027), kept in the index: wins over the extension. */
  ftype?: string;
  /** The name of the person who made it, as they gave it: a homonym is suffixed with it. */
  who?: string;
}

/** The refusal of an admin to a deletion request. It wins over a concurrent acceptance. */
export interface Decision extends Stamp {
  req: string;
  verdict: "refused";
}

export interface PackageRef extends Stamp {
  hash: string;
  size: number;
}

export type DeletionMode = "approval" | "free";

export type IndexDoc = {
  /** The text of the main file, edited in place. */
  text?: string;
  schema?: 1;
  /** The id of the main node. */
  main?: string;
  nodes?: Record<string, IndexNode>;
  packages?: Record<string, PackageRef>;
  settings?: { deletion: DeletionMode };
  decisions?: Record<string, Decision>;
};

/**
 * A string written as one value, not as collaborative text. Automerge makes a text object, with
 * one operation per character, of every plain string it is given; ids, names, urls and hashes
 * are registers (the last write wins, ADR 0023), and as a value each is a single operation.
 * Typed as `string`: {@link plain} gives the plain strings back when reading.
 */
export const imm = (s: string): string => new A.ImmutableString(s) as unknown as string;

/** A string out of a document, whichever way it was written. */
function plainValue(v: unknown): unknown {
  if (v instanceof A.ImmutableString) return v.toString();
  if (Array.isArray(v)) return v.map(plainValue);
  if (v === null || typeof v !== "object") return v;
  if (v instanceof Date || v instanceof Uint8Array || v instanceof A.Counter) return v;
  return plainMap(v as Record<string, unknown>);
}

function plainMap(m: Record<string, unknown>, skip?: string): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const k of Object.keys(m)) {
    if (k !== skip) out[k] = plainValue(m[k]);
  }
  return out;
}

/**
 * The index of a document as plain data: strings are strings again. The text of the main file is
 * left out; it is large and not the index's business.
 */
export function plain(doc: unknown): IndexDoc {
  if (doc === null || typeof doc !== "object") return {};
  return plainMap(doc as Record<string, unknown>, "text") as IndexDoc;
}

/** Roles as the bridge reports them, from the lowest. */
export type Role = "reader" | "writer" | "admin";

/** The most nodes a tree may have; the ones past it, in id order, are ignored (ADR 0023). */
export const MAX_NODES = 10000;

const ID = /^[0-9a-f]{32}$/;
const HASH = /^blake3:[0-9a-f]{64}$/;
const PACKAGE_KEY = /^@preview\/[a-z0-9][a-z0-9_-]{0,63}:[0-9]+\.[0-9]+\.[0-9]+$/;

/** A node id: 128 bits in hex. */
export const isId = (s: unknown): s is string => typeof s === "string" && ID.test(s);
export const isHash = (s: unknown): s is string => typeof s === "string" && HASH.test(s);
export const isPackageKey = (s: unknown): s is string =>
  typeof s === "string" && PACKAGE_KEY.test(s);

/** A fresh random node id. */
export function newId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

/** A deterministic id derived from a text (FNV-1a over four seeds): for the migration. */
export function idFrom(text: string): string {
  let out = "";
  for (let seed = 0; seed < 4; seed++) {
    let h = (0x811c9dc5 ^ (seed * 0x9e3779b1)) >>> 0;
    for (let i = 0; i < text.length; i++) {
      h = Math.imul(h ^ text.charCodeAt(i), 0x01000193) >>> 0;
    }
    out += h.toString(16).padStart(8, "0");
  }
  return out;
}

/** The 32 bytes of a "blake3:" hash. */
export function hashBytes(hash: string): Uint8Array {
  if (!isHash(hash)) throw new RangeError("bad blob hash");
  const hex = hash.slice("blake3:".length);
  return Uint8Array.from({ length: 32 }, (_, i) =>
    Number.parseInt(hex.slice(i * 2, i * 2 + 2), 16),
  );
}

/** "blake3:" and the hex of 32 bytes. */
export function hashString(bytes: Uint8Array): string {
  return `blake3:${Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("")}`;
}
