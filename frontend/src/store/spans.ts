/** A span of the text, in UTF-16 units (the unit of CodeMirror and Automerge). */
export interface Range {
  from: number;
  to: number;
}

/**
 * The part of an inserted span that is content, without the newlines at its ends. A diff puts
 * the newline that separates a new line from the old one either side of it: "A\nB" added after
 * "A" may be "\nB" or "B\n". The line the newline closes (or the one it opens, at the start) was
 * not changed, so it is not counted or marked. `text` is the span's own text; undefined if the
 * span is only newlines (a blank line was added).
 */
export function contentOf(span: Range, text: string): Range | undefined {
  let start = 0;
  while (start < text.length && text[start] === "\n") start++;
  let end = text.length;
  while (end > start && text[end - 1] === "\n") end--;
  return start === end ? undefined : { from: span.from + start, to: span.from + end };
}
