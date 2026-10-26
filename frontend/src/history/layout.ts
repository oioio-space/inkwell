import type { Frag, Hunk } from "./diffview";

/** What the inline view shows, in order: unchanged text, a fold, a run of changes. */
export type Item =
  | { t: "eq"; text: string }
  | { t: "gap"; lines: number }
  | { t: "hunk"; index: number; frags: Frag[] };

/** Unchanged lines kept around each change. */
const CONTEXT = 3;

/** Keeps the lines of `text` that are near a change: at its start, its end, or both. */
function fold(text: string, keep: "start" | "end" | "both", ctx = CONTEXT): Item[] {
  const lines = text.split("\n");
  const limit = keep === "both" ? 2 * ctx + 2 : ctx + 1;
  if (lines.length <= limit) return [{ t: "eq", text }];
  const head = keep === "end" ? [] : lines.slice(0, ctx + (keep === "both" ? 0 : 1));
  const tail = keep === "start" ? [] : lines.slice(-(ctx + 1));
  const hidden = lines.length - head.length - tail.length;
  const out: Item[] = [];
  if (head.length) out.push({ t: "eq", text: `${head.join("\n")}\n` });
  out.push({ t: "gap", lines: hidden });
  if (tail.length) out.push({ t: "eq", text: tail.join("\n") });
  return out;
}

/**
 * Lays a diff out for reading: each run of changes with a few unchanged lines around it, the
 * rest folded away (« 120 lignes inchangées »), so a change in a long text is found at once.
 */
export function layoutDiff(frags: readonly Frag[], hunks: readonly Hunk[]): Item[] {
  const out: Item[] = [];
  let at = 0;
  hunks.forEach((h, index) => {
    if (h.from > at) {
      const text = frags
        .slice(at, h.from)
        .map((f) => f.text)
        .join("");
      out.push(...fold(text, at === 0 ? "end" : "both"));
    }
    out.push({ t: "hunk", index, frags: frags.slice(h.from, h.to + 1) as Frag[] });
    at = h.to + 1;
  });
  if (at < frags.length) {
    const text = frags
      .slice(at)
      .map((f) => f.text)
      .join("");
    out.push(...fold(text, hunks.length === 0 ? "both" : "start"));
  }
  return out;
}
