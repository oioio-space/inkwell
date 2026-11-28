/**
 * The formatting languages of the built-ins (EX0c3) are on the first screen: the bar, the Insert
 * menu and the word count need them before any paint, as the status items' entries
 * (statusEntries.ts). Kept apart from the loader, which the registry needs first.
 */
const FORMAT_ENTRIES: Record<string, unknown> = import.meta.glob("../ext/*/native/format.ts", {
  eager: true,
});

/** A built-in's format entry, already there; undefined for any other entry. */
export const builtinFormatEntry = (folder: string, entry: string): unknown =>
  FORMAT_ENTRIES[`../ext/${folder}/${entry}`];

/** Their paths, so a manifest can name only a file of its own folder (the loader's list). */
export const FORMAT_FILES: readonly string[] = Object.keys(FORMAT_ENTRIES);
