import type { Patch } from "@automerge/automerge";

/**
 * The single window of the text that a batch of Automerge patches touched, in the new text's
 * coordinates (`from`, `insLen`) and the old text's (`to`). Undefined when a patch is not a
 * plain splice of the text (then the caller rescans everything).
 */
export function patchWindow(
  patches: readonly Patch[],
  field = "text",
): { from: number; to: number; insLen: number } | undefined {
  let lo = Number.POSITIVE_INFINITY;
  let hi = 0;
  let delta = 0;
  for (const p of patches) {
    if (p.path[0] !== field) continue;
    let pos: number;
    let del: number;
    let ins: number;
    if (p.action === "splice" && p.path.length === 2) {
      pos = p.path[1] as number;
      del = 0;
      ins = p.value.length;
    } else if (p.action === "del" && p.path.length === 2) {
      pos = p.path[1] as number;
      del = p.length ?? 1;
      ins = 0;
    } else {
      return undefined;
    }
    if (lo === Number.POSITIVE_INFINITY) {
      lo = pos;
      hi = pos + ins;
    } else {
      const curHi = Math.max(hi, pos + del);
      lo = Math.min(lo, pos);
      hi = curHi + ins - del;
    }
    delta += ins - del;
  }
  if (lo === Number.POSITIVE_INFINITY) return undefined;
  return { from: lo, to: hi - delta, insLen: hi - lo };
}
