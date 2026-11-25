/**
 * The status bar's items of the built-ins (EX0c1) are on the first screen: their entries are part
 * of the start, not a chunk of their own that would pop in after it. Kept apart from the loader,
 * which the registry needs before any of them can be imported.
 */
const STATUS_ENTRIES: Record<string, unknown> = import.meta.glob("../ext/*/native/status.ts", {
  eager: true,
});

/** A built-in's status entry, already there; undefined for any other entry. */
export const builtinStatusEntry = (folder: string, entry: string): unknown =>
  STATUS_ENTRIES[`../ext/${folder}/${entry}`];
