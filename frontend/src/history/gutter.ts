import { next as A } from "@automerge/automerge";
import type { ChangeMark } from "../editor/changeGutter";
import { t } from "../i18n";
import type { Author } from "./people";
import type { Blame } from "./track";
import { clock } from "./when";

type Doc = { text: string };

/** The cursor of the char at `pos`, or "end" for the point after the last one. */
export const END = "end";

function cursorAt(doc: A.Doc<Doc>, pos: number): string {
  return pos >= doc.text.length ? END : (A.getCursor(doc, ["text"], pos) as string);
}

/**
 * The point changes are counted from: where this device left the document, or now when it has
 * never seen it (or what it remembers is not in this document): nothing is marked then.
 */
export function sinceHeads(doc: A.Doc<Doc>, visit: { heads: string[] } | undefined): string[] {
  return visit && visit.heads.length > 0 && A.hasHeads(doc, visit.heads)
    ? visit.heads
    : A.getHeads(doc);
}

/** Turns a cursor from {@link changeMarks} into an offset of the document now. */
export function resolver(doc: A.Doc<Doc>): (cursor: string) => number | undefined {
  return (cursor) => {
    if (cursor === END) return doc.text.length;
    try {
      return A.getCursorPosition(doc, ["text"], cursor);
    } catch {
      return undefined;
    }
  };
}

/**
 * What others changed since `since` (the sessions from index `since` on), for the gutter:
 * additions, modifications and removals. Your own edits are not marked (HIS-04). Each mark holds
 * Automerge cursors, so it follows its text when the text moves (HIS-05).
 */
export function changeMarks(
  doc: A.Doc<Doc>,
  blame: Blame,
  since: number,
  author: (actor: string) => Author,
): ChangeMark[] {
  const out: ChangeMark[] = [];
  const label = (s: number) => {
    const session = blame.sessions[s];
    if (!session) return "";
    return t("history.mark.label", { name: author(session.actor).name, time: clock(session.end) });
  };
  for (const sp of blame.spans) {
    const session = blame.sessions[sp.s];
    if (!session || sp.s < since || sp.to <= sp.from) continue;
    const who = author(session.actor);
    if (who.mine) continue;
    out.push({
      kind: sp.replace ? "modified" : "added",
      from: cursorAt(doc, sp.from),
      to: cursorAt(doc, sp.to - 1),
      person: who.index,
      label: `${label(sp.s)} — ${t(sp.replace ? "history.mark.modified" : "history.mark.added")}`,
    });
  }
  for (const d of blame.deletions) {
    const session = blame.sessions[d.by];
    // Text that was already there at the last visit, taken away by someone else since.
    if (!session || d.by < since || d.owner >= since) continue;
    const who = author(session.actor);
    if (who.mine) continue;
    const near = blame.spans.find(
      (sp) => sp.s === d.by && sp.replace && (sp.from === d.at || sp.to === d.at),
    );
    if (near) continue; // shown as a modification
    const cursor = cursorAt(doc, d.at);
    out.push({
      kind: "deleted",
      from: cursor,
      to: cursor,
      person: who.index,
      label: `${label(d.by)} — ${t("history.mark.deleted")}`,
    });
  }
  return out;
}
