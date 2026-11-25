/**
 * What the palette and the shortcuts window share once open. Kept out of dialogs.svelte.ts, which
 * is in the start chunk: this module is loaded with the dialogs only (criterion `chunks`).
 */
import { chordsOf, commandSystem, run } from "../../extensions/commands";
import { eventChord } from "../../extensions/keys";
import { session } from "../editorSession.svelte";

/** Gives the focus back to a dialog's target, or to the editor when it left the page. */
export function giveFocusBack(target: Element | undefined): void {
  if (target instanceof HTMLElement && target.isConnected) target.focus();
  else session.focus();
}

/** A dialog's count is said this long after the last keystroke. */
const STATUS_MS = 300;

/**
 * Says `text` in a dialog's own `status` region once the typing stops (a modal dialog makes the
 * rest of the page inert): call it from an effect, which cancels it at the next keystroke.
 */
export function sayLater(say: (text: string) => void, text: string): () => void {
  const timer = setTimeout(() => say(text), STATUS_MS);
  return () => clearTimeout(timer);
}

/** A key that ends or edits an input method's composition: never a dialog's. */
export const composing = (e: KeyboardEvent): boolean => e.isComposing || e.keyCode === 229;

const OWN = ["app.palette", "app.shortcuts"];

/**
 * Runs this block's two commands when their chord is pressed in one of the dialogs (the dialog is
 * modal: no other registry chord acts in it). Returns whether the key was one of them.
 */
export function runOwnChord(e: KeyboardEvent): boolean {
  const own = (x: string) => OWN.find((id) => chordsOf(id).includes(x));
  const chord = eventChord(e, commandSystem(), (x) => own(x) !== undefined);
  const id = chord === undefined ? undefined : own(chord);
  if (id === undefined) return false;
  e.preventDefault();
  run(id);
  return true;
}
