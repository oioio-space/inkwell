import { t } from "../i18n";
import { headingsOf, sectionAt } from "./summary";
import { when } from "./when";
import type { Op } from "./words";

/**
 * A moment as an entry shows it. A time is what the author's device said, so it is believed
 * only so far: when it disagrees with the order of the changes, it says whose clock it comes
 * from (X-20, DIF-10).
 */
export function stamp(ms: number, skewed: boolean, who: string, now = Date.now()): string {
  const text = when(ms, now);
  return skewed ? `${text} ${t("history.device", { name: who })}` : text;
}

/** Where a moved block left and where it went, by the titles of the sections: « de … vers … ». */
export function moveSections(
  oldText: string,
  newText: string,
  ops: readonly Op[],
): Map<number, { from: string; to: string }> {
  const out = new Map<number, { from: string; to: string }>();
  const oldHeads = headingsOf(oldText);
  const newHeads = headingsOf(newText);
  for (const op of ops) {
    if (op.moved === undefined) continue;
    const entry = out.get(op.moved) ?? { from: "", to: "" };
    if (op.kind === "del") entry.from = sectionAt(oldHeads, op.a)?.title ?? "";
    if (op.kind === "add") entry.to = sectionAt(newHeads, op.b)?.title ?? "";
    out.set(op.moved, entry);
  }
  return out;
}
