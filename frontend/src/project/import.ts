import type { AutomergeUrl } from "@automerge/automerge-repo";
import type { BlobInfo } from "../store/tree";
import type { Materialized } from "./materialize";
import { hashString, isHash } from "./model";
import { collisionKey, sanitizeName, splitExt } from "./names";
import type { NodeSpec } from "./ops";

/** The most a blob may weigh (ADR 0023: 100 MB; amends ADR 0006). */
export const MAX_BLOB_BYTES = 100 * 1024 * 1024;
/** Beyond this a text file travels as a blob: it would be a very large document. */
export const MAX_TEXT_BYTES = 8 * 1024 * 1024;
/** What one frame of the store stream can carry of a file being added from the webview. */
export const MAX_UPLOAD_BYTES = 30 * 1024 * 1024;

/** A file the person brought: dropped, picked, or found in a dropped folder. */
export interface IncomingFile {
  /** Its path inside what was dropped, with "/" between folders (just the name for one file). */
  path: string;
  size: number;
  /** Reads the bytes. */
  read(): Promise<Uint8Array>;
}

export interface ImportDeps {
  /** Makes a text document with `text`. */
  createText(text: string): AutomergeUrl;
  /**
   * Stores bytes as the project file `path` and gives their hash (the `PutBlob` of the store
   * stream). Only called for a file the person chose to add.
   */
  putBlob(path: string, bytes: Uint8Array): Promise<BlobInfo>;
}

export type SkipReason = "too_large" | "unreadable" | "empty_name";

export interface ImportPlan {
  specs: NodeSpec[];
  /** Files whose name had to be changed to work everywhere or to avoid a clash, [before, after]. */
  renamed: [string, string][];
  skipped: { path: string; reason: SkipReason; limit?: number }[];
  /** How many files the plan adds (folders not counted). */
  files: number;
}

/** File extensions that are text whatever their bytes look like. */
const TEXT_EXT = new Set([
  "typ",
  "typst",
  "md",
  "markdown",
  "txt",
  "csv",
  "tsv",
  "bib",
  "json",
  "yaml",
  "yml",
  "toml",
  "xml",
  "svg",
  "html",
  "css",
  "tex",
]);

// biome-ignore lint/suspicious/noControlCharactersInRegex: that is the point
const BINARY_CONTROL = /[\u0001-\u0008\u000b\u000e-\u001f]/;

/** The bytes of a text file as the editor wants them (UTF-8, LF), or undefined if it is not text. */
export function asText(name: string, bytes: Uint8Array): string | undefined {
  if (bytes.length > MAX_TEXT_BYTES || bytes.includes(0)) return undefined;
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes);
  } catch {
    return undefined;
  }
  if (text.charCodeAt(0) === 0xfeff) text = text.slice(1);
  const known = TEXT_EXT.has(splitExt(name).ext.slice(1).toLowerCase());
  if (!known && BINARY_CONTROL.test(text)) return undefined;
  return text.replace(/\r\n?/g, "\n");
}

/** A folder of the plan: what its children are attached to, and the names taken inside. */
interface Place {
  ref: NodeSpec["parent"];
  used: Set<string>;
}

/**
 * Plans an import (FM-02): the folders and files of what was dropped, reproduced under `parent`
 * in one change. Names that would not work everywhere are corrected and reported (PRJ-10), a name
 * that is taken gets "(2)", never an overwrite. Text files become documents; the other files are
 * stored as blobs, and only because the person chose to add them. Files that cannot be added are
 * reported, never silently dropped.
 */
export async function planImport(
  files: IncomingFile[],
  parent: string | null,
  tree: Materialized,
  deps: ImportDeps,
): Promise<ImportPlan> {
  const plan: ImportPlan = { specs: [], renamed: [], skipped: [], files: 0 };
  const taken = (id: string | null) =>
    new Set(tree.entries.filter((e) => e.parentId === id).map((e) => collisionKey(e.name)));
  const root: Place = { ref: parent, used: taken(parent) };
  const places = new Map<string, Place>([["", root]]);

  /** A name free in `place`: `name`, else `stem (2).ext`... */
  const free = (place: Place, name: string): string => {
    const { stem, ext } = splitExt(name);
    let out = name;
    for (let n = 2; place.used.has(collisionKey(out)); n++) out = `${stem} (${n})${ext}`;
    place.used.add(collisionKey(out));
    return out;
  };

  const placeOf = (segments: string[]): Place => {
    let key = "";
    let here = root;
    for (const raw of segments) {
      const name = sanitizeName(raw);
      key = key === "" ? name : `${key}/${name}`;
      const known = places.get(key);
      if (known) {
        here = known;
        continue;
      }
      const same = (e: { type: string; parentId: string | null; name: string }) =>
        e.type === "dir" && e.parentId === here.ref && collisionKey(e.name) === collisionKey(name);
      const existing = typeof here.ref === "object" ? undefined : tree.entries.find(same);
      let next: Place;
      if (existing) {
        next = { ref: existing.id, used: taken(existing.id) };
      } else {
        const unique = free(here, name);
        if (unique !== raw) plan.renamed.push([raw, unique]);
        plan.specs.push({ type: "dir", name: unique, parent: here.ref });
        next = { ref: { spec: plan.specs.length - 1 }, used: new Set() };
      }
      places.set(key, next);
      here = next;
    }
    return here;
  };

  /** The path the project will give a file in `place`, for the blob store. */
  const refPath = (ref: NodeSpec["parent"]): string => {
    if (typeof ref === "object" && ref !== null) {
      const spec = plan.specs[ref.spec] as NodeSpec;
      const base = refPath(spec.parent);
      return base === "" ? spec.name : `${base}/${spec.name}`;
    }
    return ref === null ? "" : (tree.byId.get(ref)?.path ?? "");
  };
  const pathIn = (place: Place, name: string): string => {
    const base = refPath(place.ref);
    return base === "" ? name : `${base}/${name}`;
  };

  for (const file of files) {
    const parts = file.path.split("/").filter((p) => p !== "");
    const rawName = parts.pop();
    if (rawName === undefined) {
      plan.skipped.push({ path: file.path, reason: "empty_name" });
      continue;
    }
    if (file.size > MAX_BLOB_BYTES) {
      plan.skipped.push({ path: file.path, reason: "too_large", limit: MAX_BLOB_BYTES });
      continue;
    }
    let bytes: Uint8Array;
    try {
      bytes = await file.read();
    } catch {
      plan.skipped.push({ path: file.path, reason: "unreadable" });
      continue;
    }
    const text = asText(rawName, bytes);
    if (text === undefined && bytes.length > MAX_UPLOAD_BYTES) {
      plan.skipped.push({ path: file.path, reason: "too_large", limit: MAX_UPLOAD_BYTES });
      continue;
    }
    const place = placeOf(parts);
    const name = free(place, sanitizeName(rawName));
    if (name !== rawName) plan.renamed.push([rawName, name]);
    if (text !== undefined) {
      plan.specs.push({ type: "text", name, parent: place.ref, url: deps.createText(text) });
    } else {
      try {
        const info = await deps.putBlob(pathIn(place, name), bytes);
        const hash = hashString(info.hash);
        if (!isHash(hash)) throw new Error("bad hash");
        plan.specs.push({ type: "blob", name, parent: place.ref, hash, size: info.size });
      } catch {
        plan.skipped.push({ path: file.path, reason: "unreadable" });
        continue;
      }
    }
    plan.files++;
  }
  return plan;
}
