import { getCursor, getCursorPosition } from "@automerge/automerge";
import { type DocHandle, Presence } from "@automerge/automerge-repo";
import type { Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { personColor } from "../ui/people";
import { type RemoteCursor, setRemoteCursors } from "./remoteCursors";

type Doc = { text: string };

/** What each person broadcasts: who they are and where their selection is. */
export type CursorState = { name: string; color: string; anchor?: string; head?: string };

const PATH = ["text"];

/**
 * Shares this person's selection as Automerge cursors (stable under concurrent
 * edits) and shows everyone else's. Presence is never persisted (design doc:
 * "La présence n'est jamais enregistrée").
 */
export function presenceSync(
  handle: DocHandle<Doc>,
  me: { id: string; name: string },
  getView: () => EditorView | undefined,
): { extension: Extension; rename: (name: string) => void; stop: () => void } {
  const presence = new Presence<CursorState, Doc>({ handle });
  presence.start({ initialState: { name: me.name, color: personColor(me.id) } });

  const refresh = () => {
    const view = getView();
    const doc = handle.doc();
    if (!view || !doc) return;
    const cursors: RemoteCursor[] = [];
    for (const peer of Object.values(presence.getPeerStates().value)) {
      const { name, color, anchor, head } = peer.value;
      if (!anchor || !head) continue;
      try {
        cursors.push({
          peerId: peer.peerId,
          name,
          color,
          anchor: getCursorPosition(doc, PATH, anchor),
          head: getCursorPosition(doc, PATH, head),
        });
      } catch {
        // Cursor refers to text this replica has not received yet: skip until next update.
      }
    }
    view.dispatch({ effects: setRemoteCursors.of(cursors) });
  };
  for (const event of ["update", "snapshot", "goodbye", "pruning"] as const)
    presence.on(event, refresh);

  const extension = EditorView.updateListener.of((u) => {
    if (!u.selectionSet && !u.docChanged) return;
    const doc = handle.doc();
    if (!doc) return;
    const sel = u.state.selection.main;
    presence.broadcast("anchor", getCursor(doc, PATH, sel.anchor));
    presence.broadcast("head", getCursor(doc, PATH, sel.head));
  });

  return {
    extension,
    rename: (name) => presence.broadcast("name", name),
    stop: () => presence.stop(),
  };
}
