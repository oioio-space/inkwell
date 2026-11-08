import { completionStatus, currentCompletions } from "@codemirror/autocomplete";
import { redoDepth, undoDepth } from "@codemirror/commands";
import { Compartment, EditorState, StateEffect, Transaction } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import type { SocketLike } from "../net/WailsNetworkAdapter";

/**
 * Typing checks, perf builds only (`?typing-check=1`, Go phase `typing-check`): the driver
 * (scripts/perf/typing_checks.py) types with real keys through Mutter and reads back here what
 * CodeMirror did. No measurement runs in this mode. Commands come from the driver's stdin; every
 * reply is one JSON message that Go prints on stdout.
 */
export interface TypingCheckOptions {
  open: () => SocketLike;
  view: () => EditorView | undefined;
  /** The read-only facets the editor applies for a reader (setup.ts). */
  readOnly: (on: boolean) => import("@codemirror/state").Extension;
}

interface Edit {
  userEvent: string | null;
  inserted: string[];
}
interface InputRecord {
  type: string;
  inputType?: string;
  data?: string | null;
  prevented?: boolean;
  composing?: boolean;
  ctrl?: boolean;
}

export function startTypingCheck(options: TypingCheckOptions): () => void {
  const socket = options.open();
  socket.binaryType = "arraybuffer";
  const access = new Compartment();
  let attached: EditorView | undefined;
  let edits: Edit[] = [];
  let inputs: InputRecord[] = [];
  let pointer: { x: number; y: number } | undefined;
  const send = (value: unknown) => socket.send(new TextEncoder().encode(JSON.stringify(value)));

  const attach = (): EditorView | undefined => {
    const view = options.view();
    if (!view || view === attached) return view;
    attached = view;
    view.dispatch({
      effects: StateEffect.appendConfig.of([
        access.of([]),
        EditorView.updateListener.of((u) => {
          for (const tr of u.transactions) {
            if (!tr.docChanged) continue;
            const inserted: string[] = [];
            tr.changes.iterChanges((_a, _b, _c, _d, text) => inserted.push(text.toString()));
            edits.push({ userEvent: tr.annotation(Transaction.userEvent) ?? null, inserted });
          }
        }),
      ]),
    });
    return view;
  };
  // Bubble phase on window: after CodeMirror's own handlers, so `defaultPrevented` is final.
  const record = (event: Event) => {
    if (event instanceof InputEvent)
      inputs.push({
        type: event.type,
        inputType: event.inputType,
        data: event.data,
        prevented: event.defaultPrevented,
        composing: event.isComposing,
      });
    else if (event instanceof CompositionEvent) inputs.push({ type: event.type, data: event.data });
    else if (event instanceof MouseEvent) inputs.push({ type: event.type, ctrl: event.ctrlKey });
  };
  const move = (event: MouseEvent) => (pointer = { x: event.clientX, y: event.clientY });
  const names = [
    "beforeinput",
    "input",
    "compositionstart",
    "compositionupdate",
    "compositionend",
    "mousedown",
  ];
  for (const name of names) window.addEventListener(name, record);
  window.addEventListener("mousemove", move, true);

  const state = () => {
    const view = attach();
    if (!view) return { type: "state", ready: false };
    const s = view.state;
    return {
      type: "state",
      ready: true,
      ownsTyping: import.meta.env.VITE_PERF_CM_TYPING === "1",
      doc: s.doc.toString(),
      ranges: s.selection.ranges.map((r) => ({ anchor: r.anchor, head: r.head })),
      focused: view.hasFocus,
      composing: view.composing,
      readOnly: s.readOnly,
      multipleSelections: s.facet(EditorState.allowMultipleSelections),
      completion: completionStatus(s),
      completions: currentCompletions(s).map((c) => c.label),
      undo: undoDepth(s),
      redo: redoDepth(s),
      edits,
      inputs,
      pointer,
    };
  };
  socket.onmessage = (event: MessageEvent) => {
    const command = JSON.parse(new TextDecoder().decode(event.data as ArrayBuffer)) as {
      cmd: string;
      text?: string;
      anchor?: number;
      on?: boolean;
      pos?: number;
    };
    const view = attach();
    switch (command.cmd) {
      case "reset": {
        if (!view) break;
        const text = command.text ?? "";
        view.dispatch({
          changes: { from: 0, to: view.state.doc.length, insert: text },
          selection: { anchor: Math.min(command.anchor ?? text.length, text.length) },
          annotations: Transaction.addToHistory.of(false),
        });
        view.focus();
        edits = [];
        inputs = [];
        break;
      }
      case "readonly":
        view?.dispatch({ effects: access.reconfigure(options.readOnly(command.on === true)) });
        break;
      case "coords": {
        const rect = view?.coordsAtPos(command.pos ?? 0);
        send({ type: "coords", pos: command.pos, rect, dpr: window.devicePixelRatio });
        return;
      }
    }
    send(state());
  };
  socket.onopen = () => send({ ...state(), type: "ready" });

  return () => {
    for (const name of names) window.removeEventListener(name, record);
    window.removeEventListener("mousemove", move, true);
    socket.close();
  };
}
