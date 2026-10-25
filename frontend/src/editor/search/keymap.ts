import type { KeyBinding } from "@codemirror/view";

/** What the host does when a search key is pressed. Return false to let the key fall through. */
export interface SearchActions {
  /** Ctrl+F: search in this file. */
  find(): boolean | undefined;
  /** Ctrl+H: search and replace. */
  replace(): boolean | undefined;
  /** Ctrl+Shift+F: search the whole project. */
  findInProject(): boolean | undefined;
  /** F3 / Shift+F3: next / previous occurrence of the current query. */
  next(backwards: boolean): boolean | undefined;
  /** F4 / Shift+F4: next / previous entry of the results list. */
  nextResult(backwards: boolean): boolean | undefined;
  /** Escape: close the panel if it is open. */
  close?(): boolean | undefined;
}

const run = (fn: () => boolean | undefined) => () => fn() !== false;

/** The search key bindings. `Mod` is Ctrl (Cmd on a Mac). Wrap in `Prec.high(keymap.of(…))`. */
export function searchKeymap(a: SearchActions): KeyBinding[] {
  return [
    { key: "Mod-f", run: run(() => a.find()), preventDefault: true },
    { key: "Mod-h", run: run(() => a.replace()), preventDefault: true },
    { key: "Mod-Shift-f", run: run(() => a.findInProject()), preventDefault: true },
    {
      key: "F3",
      run: run(() => a.next(false)),
      shift: run(() => a.next(true)),
      preventDefault: true,
    },
    {
      key: "F4",
      run: run(() => a.nextResult(false)),
      shift: run(() => a.nextResult(true)),
      preventDefault: true,
    },
    ...(a.close ? [{ key: "Escape", run: () => a.close?.() === true }] : []),
  ];
}
