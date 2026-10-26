import { next as A } from "@automerge/automerge";
import { tooBigToGrow } from "../store/limits";

/**
 * When the document last changed, in Unix seconds: the latest time recorded by the changes at
 * its heads. Every person who holds the same heads gets the same answer, so the date inside the
 * document (`datetime.today()`, the PDF's creation date) is the same for everyone; the clock of
 * this machine is never used. 0 when the document has no changes, or is too big to be edited together: decoding
 * the change that holds such a text makes Automerge panic, and the preview then waits for no date.
 */
export function lastChangeSeconds(doc: A.Doc<{ text?: string }>): number {
  if (tooBigToGrow(doc.text?.length ?? 0)) return 0;
  let latest = 0;
  for (const head of A.getHeads(doc)) {
    const time = A.inspectChange(doc, head)?.time ?? 0;
    if (Number.isFinite(time) && time > latest) latest = time;
  }
  return latest;
}
