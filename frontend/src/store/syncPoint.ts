import { next as A } from "@automerge/automerge";

/**
 * The point where a file and the document agreed: the heads of the document, and a fingerprint of
 * the text at those heads. Go keeps it (as opaque JSON) in the record of the file's hash, so the
 * two describe the same bytes; an outside edit is merged from it.
 */
export interface SyncPoint {
  heads: A.Heads;
  fingerprint: number;
}

/** cyrb53: a quick 53-bit fingerprint, to tell whether the text at saved heads is the text saved. */
function fingerprint(s: string): number {
  let h1 = 0xdeadbeef;
  let h2 = 0x41c6ce57;
  for (let i = 0; i < s.length; i++) {
    const ch = s.charCodeAt(i);
    h1 = Math.imul(h1 ^ ch, 2654435761);
    h2 = Math.imul(h2 ^ ch, 1597334677);
  }
  h1 = Math.imul(h1 ^ (h1 >>> 16), 2246822507) ^ Math.imul(h2 ^ (h2 >>> 13), 3266489909);
  h2 = Math.imul(h2 ^ (h2 >>> 16), 2246822507) ^ Math.imul(h1 ^ (h1 >>> 13), 3266489909);
  return 4294967296 * (2097151 & h2) + (h1 >>> 0);
}

/** The point to record for a file that holds `text`, which the document had at `heads`. */
export function encodeSyncPoint(heads: A.Heads, text: string): string {
  const point: SyncPoint = { heads, fingerprint: fingerprint(text) };
  return JSON.stringify(point);
}

/** Reads a recorded point; undefined if there is none or it is damaged. */
export function decodeSyncPoint(raw: string | null | undefined): SyncPoint | undefined {
  if (!raw) return undefined;
  try {
    const value = JSON.parse(raw) as Partial<SyncPoint>;
    if (
      Array.isArray(value.heads) &&
      value.heads.every((h) => typeof h === "string") &&
      typeof value.fingerprint === "number"
    ) {
      return { heads: value.heads, fingerprint: value.fingerprint };
    }
  } catch {
    // damaged: ignored, the next write records a new one
  }
  return undefined;
}

/**
 * The heads of `doc` to merge an outside edit from: the point's if the document still has them and
 * the text there is the text recorded; else the document's current heads.
 */
export function mergeHeads(doc: A.Doc<{ text: string }>, point: SyncPoint | undefined): A.Heads {
  if (point) {
    try {
      if (fingerprint(A.view(doc, point.heads).text) === point.fingerprint) return point.heads;
    } catch {
      // The heads are not in this document (another project's, or damaged): start from now.
    }
  }
  return A.getHeads(doc);
}
