/**
 * The left side's panels of the built-ins (EX0c2) are on the first screen, as the status items
 * are (statusEntries.ts): the files show at start, so their body is part of the start, never a
 * chunk that would pop in after it. The drawer's bodies (`native/drawer.ts`) stay lazy: the drawer
 * starts closed, and its code is fetched at idle after the first paint (ui/command/panels.svelte.ts).
 */
const PANEL_ENTRIES: Record<string, unknown> = import.meta.glob("../ext/*/native/panels.ts", {
  eager: true,
});

/** A built-in's start-side panel entry, already there; undefined for any other entry. */
export const builtinPanelEntry = (folder: string, entry: string): unknown =>
  PANEL_ENTRIES[`../ext/${folder}/${entry}`];
