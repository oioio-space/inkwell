import { getCursor, getCursorPosition } from "@automerge/automerge";
import { type DocHandle, Presence } from "@automerge/automerge-repo";
import type { CursorState, Person } from "../../editor/presence";
import type { Remote, SheetController } from "./sheet.svelte";

type Doc = { text: string };
const PATH = ["text"];

/**
 * Shares this person's selection on the document's presence channel, the same one the text
 * editor uses, so someone in the text view sees the range as a text selection and the other
 * way round. The range goes out as two stable anchors (first cell's start, last cell's end);
 * incoming ones are turned back into cells, row and column included, whatever was inserted
 * since. Presence is never stored.
 */
export function sheetPresence(
  handle: DocHandle<Doc>,
  me: { name: string },
  sheet: SheetController,
  onPeople: (people: Person[]) => void = () => {},
): { rename: (name: string) => void; broadcast: () => void; stop: () => void } {
  const presence = new Presence<CursorState, Doc>({ handle });
  presence.start({ initialState: { name: me.name } });

  const refresh = () => {
    const states = Object.values(presence.getPeerStates().value);
    onPeople(states.map((p) => ({ peerId: p.peerId, name: p.value.name })));
    const doc = handle.doc();
    if (!doc) return;
    const m = sheet.model;
    const remotes: Remote[] = [];
    for (const peer of states) {
      const { name, anchor, head } = peer.value;
      if (!anchor || !head) continue;
      try {
        const a = getCursorPosition(doc, PATH, anchor);
        const h = getCursorPosition(doc, PATH, head);
        const from = Math.min(a, h);
        const to = Math.max(a, h);
        const p = m.locate(from);
        const q = m.locate(Math.max(from, to > from ? to - 1 : to));
        const d0 = sheet.displayOf(p.r);
        const d1 = sheet.displayOf(q.r);
        if (d0 === undefined || d1 === undefined) continue; // the header, or a hidden row
        remotes.push({
          peerId: peer.peerId,
          name,
          range: {
            r0: Math.min(d0, d1),
            r1: Math.max(d0, d1),
            c0: Math.min(p.c, q.c),
            c1: Math.max(p.c, q.c),
          },
        });
      } catch {
        // An anchor on text this replica has not received yet: skip until the next update.
      }
    }
    sheet.remotes = remotes;
  };
  for (const event of ["update", "snapshot", "goodbye", "pruning"] as const)
    presence.on(event, refresh);

  const broadcast = () => {
    const doc = handle.doc();
    if (!doc || sheet.rowCount === 0) return;
    const m = sheet.model;
    const p = sheet.primary;
    const at = (pos: number) =>
      getCursor(doc, PATH, Math.max(0, Math.min(pos, doc.text.length - 1)));
    try {
      const first = m.offsetOf(sheet.recordOf(p.r0), p.c0);
      const lastRec = sheet.recordOf(p.r1);
      const lastCell = m.record(lastRec)[p.c1];
      const end = lastCell ? lastCell.end : first;
      presence.broadcast("anchor", at(first));
      presence.broadcast("head", at(Math.max(first, end)));
    } catch {
      // Nothing to point at yet.
    }
  };

  // Closing the window says goodbye, so the others' view of this person goes at once.
  const bye = () => presence.stop();
  window.addEventListener("pagehide", bye);

  return {
    rename: (name) => presence.broadcast("name", name),
    broadcast,
    stop: () => {
      window.removeEventListener("pagehide", bye);
      presence.stop();
    },
  };
}
