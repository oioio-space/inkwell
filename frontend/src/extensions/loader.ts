/**
 * The one loader of built-in extensions. Manifests are read as raw text at build time and go
 * through the same parser a plugin's will (never through Vite's JSON-to-module transform); the
 * loader, not the manifest, stamps their origin. Entries are a lazy glob: each is a chunk of its
 * own, imported only when the registry asks for it.
 */
import type { ManifestSource } from "./manifest";
import type { EntryLoader } from "./registry";

type Importers = Record<string, () => Promise<unknown>>;

const MANIFESTS = import.meta.glob<string>("../ext/*/inkwell-plugin.json", {
  query: "?raw",
  import: "default",
  eager: true,
});
// `native/` holds the code that reaches the editor (FE-3), `debug/` the debug providers' reads.
const ENTRIES: Importers = import.meta.glob(["../ext/*/native/*.ts", "../ext/*/debug/*.ts"]);

/** `<root>/<folder>/<rest>` → [folder, rest]. */
function split(key: string, root: string): [string, string] | undefined {
  if (!key.startsWith(root)) return undefined;
  const rest = key.slice(root.length);
  const slash = rest.indexOf("/");
  return slash > 0 ? [rest.slice(0, slash), rest.slice(slash + 1)] : undefined;
}

/**
 * The manifests found under `root` (`../ext/` for the built-ins), sorted by folder so a conflict
 * is refused the same way on every start, each with the code files of its own folder.
 */
export function builtinSources(
  manifests: Record<string, string> = MANIFESTS,
  entries: Importers = ENTRIES,
  root = "../ext/",
): ManifestSource[] {
  const files = new Map<string, Set<string>>();
  for (const key of Object.keys(entries)) {
    const at = split(key, root);
    if (!at) continue;
    const [folder, file] = at;
    files.set(folder, (files.get(folder) ?? new Set()).add(file));
  }
  return Object.entries(manifests)
    .flatMap(([key, text]) => {
      const at = split(key, root);
      return at
        ? [
            {
              origin: "builtin" as const,
              folder: at[0],
              text,
              files: files.get(at[0]) ?? new Set<string>(),
            },
          ]
        : [];
    })
    .sort((a, b) => (a.folder < b.folder ? -1 : a.folder > b.folder ? 1 : 0));
}

/** Imports a built-in's entry from the glob; an entry outside it is refused, never guessed. */
export function globLoader(entries: Importers = ENTRIES, root = "../ext/"): EntryLoader {
  return (folder, entry) => {
    const importer = entries[`${root}${folder}/${entry}`];
    return importer ? importer() : Promise.reject(new Error(`no entry ${folder}/${entry}`));
  };
}

/**
 * Starts loading the entries of the built-ins activated at `startup` (the text editor, Typst's
 * grammar) as the page starts, alongside the application's own chunk
 * rather than after it. Only a hint: the manifest is read loosely here and checked by the registry,
 * which gets the same module when it loads the entry; an entry outside the glob is never imported.
 */
export function preloadStartup(): void {
  for (const source of builtinSources()) {
    let raw: { activation?: unknown; contributes?: Record<string, unknown> };
    try {
      raw = JSON.parse(source.text);
    } catch {
      continue; // the registry refuses it, and says so
    }
    if (!Array.isArray(raw.activation) || !raw.activation.includes("startup")) continue;
    const entries = new Set<unknown>();
    // Command entries wait for an idle page (`warmCommands`): the first paint does not need them.
    for (const point of ["viewers", "grammars"]) {
      const list = raw.contributes?.[point];
      if (Array.isArray(list)) for (const c of list) entries.add((c as { entry?: unknown })?.entry);
    }
    for (const entry of entries)
      if (typeof entry === "string")
        void ENTRIES[`../ext/${source.folder}/${entry}`]?.().catch(() => undefined);
  }
}
