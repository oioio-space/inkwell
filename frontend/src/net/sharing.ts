import type { DocumentId, PeerId, Repo } from "@automerge/automerge-repo";
import type { AdmittedSet } from "./scope";
export function shareAdmitted(set: Pick<AdmittedSet, "has">): Repo["shareConfig"] {
  const has = async (_peer: PeerId, id?: DocumentId) => id !== undefined && set.has(id);
  return { announce: has, access: has };
}
