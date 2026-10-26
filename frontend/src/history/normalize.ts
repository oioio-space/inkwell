import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { t } from "../i18n";
import { headingsOf } from "./summary";
import { actionMessage, type HDoc } from "./types";

/** Level of each heading, by its title (first one when a title repeats). */
function levels(text: string): Map<string, number> {
  const out = new Map<string, number>();
  for (const h of headingsOf(text)) if (!out.has(h.title)) out.set(h.title, h.level);
  return out;
}

/**
 * X-04: two people change the level of the same heading at the same time (one makes it level 2,
 * the other level 3): both edits add marker characters, and the text ends with `====`. After a
 * merge, a heading whose level is neither side's takes the higher of the two. Exactly one of the
 * devices makes the correction (the one whose actor is smaller), or both would delete characters.
 * `before` is the document as this device had it just before the merge.
 * Returns true when it wrote a correction.
 */
export function normalizeHeadings(handle: DocHandle<HDoc>, before: A.Doc<HDoc>): boolean {
  const doc = handle.doc();
  if (!doc || doc.text === before.text) return false;
  const mine = A.getActorId(doc);
  const added = A.getChangesMetaSince(doc, A.getHeads(before));
  if (added.length === 0) return false;
  // The other side: the document as the new changes (and what they were built on) left it.
  const leaves = added.filter((c) => !added.some((o) => o.deps.includes(c.hash)));
  const theirs = A.view(
    doc,
    leaves.map((c) => c.hash),
  );
  // Only the one with the smaller actor writes; the other receives its correction.
  const others = added.filter((c) => c.actor !== mine);
  if (others.length === 0) return false;
  if (others.some((c) => c.actor < mine)) return false;

  const a = levels(before.text);
  const b = levels(theirs.text);
  const now = headingsOf(doc.text);
  const fixes: { pos: number; have: number; want: number }[] = [];
  for (const h of now) {
    const la = a.get(h.title);
    const lb = b.get(h.title);
    if (la === undefined || lb === undefined || h.level === la || h.level === lb) continue;
    fixes.push({ pos: h.pos, have: h.level, want: Math.max(la, lb) });
  }
  if (fixes.length === 0) return false;
  handle.change(
    (d) => {
      // From the end, so the positions stay right.
      for (const f of fixes.toReversed()) {
        if (f.have > f.want) A.splice(d, ["text"], f.pos, f.have - f.want, "");
        else A.splice(d, ["text"], f.pos, 0, (doc.text[f.pos] ?? "=").repeat(f.want - f.have));
      }
    },
    { message: actionMessage("fix", t("history.heading.fixed")) },
  );
  return true;
}

function touchesMarker(text: string, patch: A.Patch): boolean {
  if (patch.path[0] !== "text" || patch.path.length !== 2) return false;
  const at = patch.path[1] as number;
  const from = text.lastIndexOf("\n", at - 1) + 1;
  return at - from <= 6 && /^(=+|#+)[ \t]/.test(text.slice(from, from + 10));
}

/** Runs {@link normalizeHeadings} after every change that comes from elsewhere. Returns a stop function. */
export function watchHeadings(handle: DocHandle<HDoc>): () => void {
  const on = ({ patches, patchInfo }: { patches: A.Patch[]; patchInfo: A.PatchInfo<HDoc> }) => {
    if (patchInfo.source === "change") return;
    // Cheap test first: only an edit inside the `=` or `#` run that opens a line can matter.
    const text = handle.doc()?.text ?? "";
    if (patches.some((p) => touchesMarker(text, p))) normalizeHeadings(handle, patchInfo.before);
  };
  handle.on("change", on);
  return () => handle.off("change", on);
}
