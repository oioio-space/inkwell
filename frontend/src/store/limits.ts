/**
 * Automerge 3.5 panics on a text of about 20 MB (ADR 0026), and decoding the change that holds a
 * paste of that size overflows too. A text past this many characters is only read: nothing copies,
 * merges, diffs or decodes its document, and the sheet refuses writes.
 */
export const MAX_EDITABLE_CHARS = 10 * 1024 * 1024;

/** The one size gate: whether a text of this many characters is too big to put through the document. */
export function tooBigToEdit(chars: number): boolean {
  return chars > MAX_EDITABLE_CHARS;
}
