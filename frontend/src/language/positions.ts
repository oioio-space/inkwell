import { next as A } from "@automerge/automerge";
import type { Text } from "@codemirror/state";

export interface Position {
  line: number;
  character: number;
}
export interface AnchoredRange {
  start: A.Cursor;
  end: A.Cursor;
}
/** LSP uses UTF-16 code units, as do Automerge's JS text APIs and CodeMirror. */
export function toPosition(text: string, offset: number): Position {
  const before = text.slice(0, Math.max(0, Math.min(offset, text.length)));
  const line = before.split("\n").length - 1;
  return { line, character: before.length - before.lastIndexOf("\n") - 1 };
}
/** toPosition on a CodeMirror document, from its line index: no copy of the text. */
export function docPosition(doc: Text, offset: number): Position {
  const at = Math.max(0, Math.min(offset, doc.length));
  const line = doc.lineAt(at);
  return { line: line.number - 1, character: at - line.from };
}
export function toOffset(text: string, pos: Position): number {
  let offset = 0;
  for (let line = 0; line < pos.line; line++) {
    const end = text.indexOf("\n", offset);
    if (end < 0) return text.length;
    offset = end + 1;
  }
  const end = text.indexOf("\n", offset);
  return Math.min(offset + Math.max(0, pos.character), end < 0 ? text.length : end);
}
export function anchorRange(doc: A.Doc<{ text: string }>, from: number, to: number): AnchoredRange {
  return { start: A.getCursor(doc, ["text"], from), end: A.getCursor(doc, ["text"], to) };
}
export function resolveRange(
  doc: A.Doc<{ text: string }>,
  range: AnchoredRange,
): { from: number; to: number } {
  return {
    from: A.getCursorPosition(doc, ["text"], range.start),
    to: A.getCursorPosition(doc, ["text"], range.end),
  };
}

/** One minimal replacement, with both boundaries on complete Unicode scalars. */
export function incrementalChange(
  old: string,
  next: string,
): { range: { start: Position; end: Position }; text: string } {
  let start = 0;
  while (
    start < old.length &&
    start < next.length &&
    old.charCodeAt(start) === next.charCodeAt(start)
  )
    start++;
  if (start > 0 && isHigh(old.charCodeAt(start - 1)) && isLow(old.charCodeAt(start))) start--;
  let oldEnd = old.length,
    newEnd = next.length;
  while (
    oldEnd > start &&
    newEnd > start &&
    old.charCodeAt(oldEnd - 1) === next.charCodeAt(newEnd - 1)
  ) {
    oldEnd--;
    newEnd--;
  }
  if (oldEnd > start && isHigh(old.charCodeAt(oldEnd - 1)) && isLow(old.charCodeAt(oldEnd))) {
    oldEnd++;
    newEnd++;
  }
  return {
    range: { start: toPosition(old, start), end: toPosition(old, oldEnd) },
    text: next.slice(start, newEnd),
  };
}
const isHigh = (value: number) => value >= 0xd800 && value <= 0xdbff;
const isLow = (value: number) => value >= 0xdc00 && value <= 0xdfff;
