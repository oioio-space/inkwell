import { next as A } from "@automerge/automerge";

/**
 * When the document last changed, in Unix seconds: the latest time recorded by the changes at
 * its heads. Every person who holds the same heads gets the same answer, so the date inside the
 * document (`datetime.today()`, the PDF's creation date) is the same for everyone; the clock of
 * this machine is never used. 0 when the document has no changes.
 */
export function lastChangeSeconds(doc: A.Doc<unknown>): number {
  let latest = 0;
  for (const head of A.getHeads(doc)) {
    const time = A.inspectChange(doc, head)?.time ?? 0;
    if (Number.isFinite(time) && time > latest) latest = time;
  }
  return latest;
}
