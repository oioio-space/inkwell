import type { StateCommand } from "@codemirror/state";
import { flushSync, mount, unmount } from "svelte";
import { parse, show } from "../editor/format/testkit";

/**
 * Test helpers for the shell's components: an editor stand-in whose state the components
 * follow (`⟨…⟩` is a selection, `‸` a caret), and mounting with cleanup.
 */
export function fakeEditor(marked: string) {
  let state = $state.raw(parse(marked));
  return {
    get state() {
      return state;
    },
    /** Same contract as the shell's `run`: true when the command did something. */
    run: (command: StateCommand): boolean =>
      command({
        state,
        dispatch: (tr) => {
          state = tr.state;
        },
      }),
    /** The document with the selection written in it. */
    show: () => show(state),
    text: () => state.doc.toString(),
  };
}

const mounted: Record<string, unknown>[] = [];

/** Mounts a component into the document; `cleanup()` removes everything mounted. */
export function render<P extends Record<string, unknown>>(
  // biome-ignore lint/suspicious/noExplicitAny: Svelte's component type is generic per component
  component: any,
  props: P,
): HTMLElement {
  const target = document.createElement("div");
  document.body.append(target);
  mounted.push(mount(component, { target, props }) as Record<string, unknown>);
  flushSync();
  return target;
}

export function cleanup(): void {
  for (const instance of mounted.splice(0)) unmount(instance);
  document.body.replaceChildren();
}

/** Clicks, then lets Svelte update the DOM. */
export function click(el: Element): void {
  el.dispatchEvent(new MouseEvent("click", { bubbles: true, detail: 1 }));
  flushSync();
}

export function press(el: Element, key: string, init: KeyboardEventInit = {}): void {
  el.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init }));
  flushSync();
}

/** happy-dom lays nothing out: give the bar the width a real window would. */
export function fakeWidth(width: number): () => void {
  const original = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "clientWidth");
  Object.defineProperty(HTMLElement.prototype, "clientWidth", {
    configurable: true,
    get: () => width,
  });
  return () => {
    if (original) Object.defineProperty(HTMLElement.prototype, "clientWidth", original);
  };
}
