import { getCursor, getCursorPosition } from "@automerge/automerge";
import { type DocHandle, Presence } from "@automerge/automerge-repo";
import type { Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { whenWritten } from "./amSync";
import { type RemoteCursor, setRemoteCursors } from "./remoteCursors";

type Doc = { text: string };

/** What each person broadcasts: who they are and where their selection is. */
export type CursorState = { name: string; file?: string; anchor?: string; head?: string };

/** A person currently present in the document. */
export type Person = { peerId: string; name: string; file?: string };

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
  onPeople: (people: Person[]) => void = () => {},
): { extension: Extension; rename: (name: string) => void; stop: () => void } {
  const presence = new Presence<CursorState, Doc>({ handle });
  presence.start({ initialState: { name: me.name, file: "main.typ" } });

  const refresh = () => {
    const states = Object.values(presence.getPeerStates().value);
    onPeople(
      states.map((p) => ({
        peerId: p.peerId,
        name: p.value.name,
        ...(p.value.file ? { file: p.value.file } : {}),
      })),
    );
    const view = getView();
    const doc = handle.doc();
    if (!view || !doc) return;
    const cursors: RemoteCursor[] = [];
    for (const peer of states) {
      const { name, anchor, head } = peer.value;
      if (peer.value.file && peer.value.file !== "main.typ") continue;
      if (!anchor || !head) continue;
      try {
        cursors.push({
          peerId: peer.peerId,
          name,
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

  // A cursor may arrive before the text it refers to. Retry once that text arrives.
  let stopped = false;
  const onDocumentChange = () =>
    queueMicrotask(() => {
      if (!stopped) refresh();
    });
  handle.on("change", onDocumentChange);

  // A cursor is a place in the document: it is taken once the keys typed before it are written
  // (amSync writes them after the paint), else it would point one character off.
  let cursorQueued = false;
  const sendCursor = () => {
    cursorQueued = false;
    const view = getView();
    const doc = handle.doc();
    if (stopped || !view || !doc) return;
    const sel = view.state.selection.main;
    presence.broadcast("anchor", getCursor(doc, PATH, sel.anchor));
    presence.broadcast("head", getCursor(doc, PATH, sel.head));
  };
  const extension = EditorView.updateListener.of((u) => {
    if ((!u.selectionSet && !u.docChanged) || cursorQueued) return;
    cursorQueued = true;
    whenWritten(handle, sendCursor);
  });

  return {
    extension,
    rename: (name) => presence.broadcast("name", name),
    stop: () => {
      stopped = true;
      handle.off("change", onDocumentChange);
      presence.stop();
    },
  };
}
