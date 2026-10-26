import { type IndexDoc, idFrom, imm, plain } from "./model";

/** The name of the main file of every project. */
export const MAIN_NAME = "main.typ";

/**
 * Turns the root document of a project that has only `text` into the index of ADR 0023: its
 * `main.typ` node points to the root document itself, so nothing about the text moves. The id of
 * that node comes from the document's URL, so two people who migrate the same project at the
 * same time write the same thing and the later merge loses nothing.
 *
 * Returns undefined when the document is already an index.
 */
export function migration(
  doc: IndexDoc,
  rootUrl: string,
  by: string,
  now: number,
): ((d: IndexDoc) => void) | undefined {
  if (doc.nodes !== undefined) return undefined;
  const id = idFrom(rootUrl);
  return (d) => {
    d.schema = 1;
    d.main = imm(id);
    d.nodes = {
      [id]: {
        type: imm("text") as "text",
        name: imm(MAIN_NAME),
        parent: null,
        url: imm(rootUrl),
        by: imm(by),
        at: now,
      },
    };
    d.packages = {};
    d.settings = { deletion: imm("approval") as "approval" };
    d.decisions = {};
  };
}

/**
 * What a person who cannot write sees of a project not yet migrated: the index it would become.
 * Nothing is written.
 */
export function virtualIndex(rootUrl: string, by = "-"): IndexDoc {
  const doc: IndexDoc = {};
  migration(doc, rootUrl, by, 0)?.(doc);
  return plain(doc);
}
