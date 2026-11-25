/**
 * The live part of the `when` context (extensions/when.ts): what the shell knows about the open
 * file and its panels. `focus` and `field` are read from the event or the target at the moment a
 * command runs, never stored. Reactive, so a menu or a button greys and checks itself.
 */
import { type Context, emptyContext } from "./when";

export type LiveContext = Omit<Context, "focus" | "field">;

const { focus: _focus, field: _field, ...initial } = emptyContext();

export const live: LiveContext = $state(initial);

/** Sets what changed; the shell calls it from its effects. */
export function setLive(next: Partial<LiveContext>): void {
  Object.assign(live, next);
}

/**
 * Bumped when the bindings in force are rebuilt (the system became known late, or the person's
 * bindings changed): what shows a chord reads it, so it redraws.
 */
export const keymap = $state({ version: 0 });
