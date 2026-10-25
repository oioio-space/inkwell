import type { Text } from "@codemirror/state";
import { isFence } from "./support";

/** A fenced code block, by the numbers of its opening and closing fence lines. */
export interface Fence {
  open: number;
  /** null while the block is unterminated (it then runs to the end of the document). */
  close: number | null;
}

/** The fenced blocks whose opening line is at or before `upTo`. */
export function fencesUpTo(doc: Text, upTo: number): Fence[] {
  const out: Fence[] = [];
  let open = 0;
  for (let n = 1; n <= Math.min(upTo, doc.lines) || (open && n <= doc.lines); n++) {
    if (!isFence(doc.line(n).text)) continue;
    if (open) {
      out.push({ open, close: n });
      open = 0;
    } else {
      open = n;
    }
  }
  if (open) out.push({ open, close: null });
  return out;
}

/** The fenced block that holds line `lineNo` (fence lines included), if any. */
export function fenceAt(doc: Text, lineNo: number): Fence | null {
  for (const fence of fencesUpTo(doc, lineNo)) {
    if (fence.open <= lineNo && lineNo <= (fence.close ?? doc.lines)) return fence;
  }
  return null;
}

/** Whether a line belongs to some fenced block. */
export function isInFence(fences: readonly Fence[], doc: Text, lineNo: number): boolean {
  return fences.some((f) => f.open <= lineNo && lineNo <= (f.close ?? doc.lines));
}
