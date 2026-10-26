<script lang="ts">
import type { DocHandle } from "@automerge/automerge-repo";
import type { EditorState } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import { untrack } from "svelte";
import type { EditorInstance, FileType } from "../filetypes";
import { workspace } from "../workspace/workspace.svelte";
import { session } from "./editorSession.svelte";

/**
 * One open file. Its type says which module draws it (ADR 0027): the text editor for text,
 * the viewer for what is not. Every file that is open keeps its own (hidden when it is not the
 * one shown), so going back to a file finds the caret, the scroll and the undo history where
 * they were.
 */
let {
  handle,
  me,
  type,
  name,
  path,
  size,
  active,
  at,
}: {
  /** The document of a text file; absent for a file that is not text. */
  handle?: DocHandle<{ text: string }> | undefined;
  me: { id: string; name: string };
  type: FileType;
  name: string;
  path: string;
  size: number;
  active: boolean;
  /** Where to put the caret when it opens (switching from a table's grid to its text). */
  at?: number | undefined;
} = $props();

// Derived, so that a new object around the same values (the list of editors is rebuilt at each
// change of the index) does not make the editor start over: the caret would go back to 0.
const doc = $derived(handle);
const kind = $derived(type);
const open = $derived(type.open);
const myName = $derived(me.name);
const shown = $derived(active);
let host: HTMLDivElement;
let instance: EditorInstance | undefined;
let view: EditorView | undefined;
let state: EditorState | undefined;

// The module is loaded when the first file of its type opens; it is mounted again only when the
// document or the module changes. A rename to another type of the same module reconfigures it.
$effect(() => {
  const h = doc;
  const load = open;
  let gone = false;
  void load().then(({ default: module }) => {
    if (gone) return;
    instance = untrack(() =>
      module.mount(host, {
        ...(h ? { handle: h } : {}),
        me,
        type: kind,
        name,
        path,
        size,
        focus: active,
        ...(at !== undefined ? { at } : {}),
        onUndoKey: () => workspace.undoWithKey(),
        onPeople: (people) => {
          workspace.names = {
            ...workspace.names,
            ...Object.fromEntries(people.map((p) => [p.peerId, p.name])),
          };
        },
        onView: (v) => {
          view = v;
          if (untrack(() => active)) session.view = v;
        },
        onState: (s) => {
          state = s;
          if (untrack(() => active)) session.state = s;
        },
        openWithSystem: () => void workspace.openWithSystem(path),
      }),
    );
  });
  return () => {
    gone = true;
    instance?.destroy();
    instance = undefined;
    if (view && session.view === view) {
      session.view = undefined;
      session.state = undefined;
    }
    view = undefined;
    state = undefined;
  };
});
// The values are read before `instance` is looked at: when the editor is not there yet, the
// optional chain would stop first and the effect would never depend on them.
$effect(() => {
  const next = myName;
  instance?.rename?.(next);
});
$effect(() => {
  const next = kind;
  instance?.setType?.(next);
});

// When this file becomes the one shown, the shell talks to its editor.
$effect(() => {
  if (!shown) return;
  if (view) {
    session.view = view;
    session.state = state;
    view.requestMeasure();
  } else {
    session.view = undefined;
    session.state = undefined;
  }
});
</script>

<div class="page" class:hidden={!active} bind:this={host} aria-hidden={active ? undefined : "true"}></div>

<style>
  /* The source reads like the page it becomes: quiet chrome, the text in its own view. */
  .page {
    height: 100%;
    background: var(--view-bg);
  }
  .page.hidden {
    display: none;
  }
</style>
