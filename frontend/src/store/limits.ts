/**
 * The one size gate (ADR 0028 D1), counted in characters of the text. Everything that puts text
 * into a document, whether it is a paste, a table's paste, a file read from the disk or a change
 * from a peer, asks here first: the size is known before Automerge sees a byte, never after.
 *
 * Measured (ADR 0028): a document costs about 128 bytes of WASM per character to open, and a text
 * pasted in one change about 300 bytes per character (7 when cut in changes of 64 Ki). The 4 GiB
 * of a 32-bit WASM memory is shared by every open document.
 */

const Ki = 1024;
const Mi = 1024 * Ki;

/** Up to here a text is smooth and silent. */
export const SLOW_CHARS = 512 * Ki;

/**
 * A text of more than this many characters does not enter a document: it is only read, from the
 * disk, as one plain string (ADR 0026, ADR 0028 D1). Between {@link SLOW_CHARS} and this, the file
 * stays editable, with a notice that other people's changes may arrive with a delay (the Automerge
 * Web Worker of ADR 0028 D2 lifts that).
 */
export const MAX_EDITABLE_CHARS = 2 * Mi;

/** A document that already exists may grow to this size; a local addition beyond is refused. */
export const MAX_GROWN_CHARS = 4 * Mi;

/** The most characters one Automerge change carries when a large text is added (D3). */
export const CHUNK_CHARS = 64 * Ki;

export type SizeTier = "fluid" | "large" | "readonly";

/** Which tier a text of this many characters is in. */
export function sizeTier(chars: number): SizeTier {
  if (chars > MAX_EDITABLE_CHARS) return "readonly";
  return chars > SLOW_CHARS ? "large" : "fluid";
}

/** Whether a text of this many characters is too big to put through the document. */
export function tooBigToEdit(chars: number): boolean {
  return chars > MAX_EDITABLE_CHARS;
}

/** Whether a document of this many characters is past what is still edited by several people. */
export function tooBigToGrow(chars: number): boolean {
  return chars > MAX_GROWN_CHARS;
}

/** Whether an edit that takes a text from `before` to `after` characters must be refused. */
export function growthRefused(before: number, after: number): boolean {
  return after > before && tooBigToGrow(after);
}

/** The text in pieces of at most {@link CHUNK_CHARS}, never cut inside a surrogate pair. */
export function chunksOf(text: string): string[] {
  const out: string[] = [];
  for (let at = 0; at < text.length; ) {
    let end = Math.min(at + CHUNK_CHARS, text.length);
    if (end < text.length) {
      const last = text.charCodeAt(end - 1);
      if (last >= 0xd800 && last <= 0xdbff) end--; // a high surrogate: keep the pair together
    }
    out.push(text.slice(at, end));
    at = end;
  }
  return out;
}
