/**
 * The one loader of built-in extensions. Manifests are read as raw text at build time and go
 * through the same parser a plugin's will (never through Vite's JSON-to-module transform); the
 * loader, not the manifest, stamps their origin. Entries are a lazy glob: each is a chunk of its
 * own, imported only when the registry asks for it.
 */
import { type ManifestSource, parseManifest } from "./manifest";
import type { EntryLoader } from "./registry";

type Importers = Record<string, () => Promise<unknown>>;

const MANIFESTS = import.meta.glob<string>("../ext/*/inkwell-plugin.json", {
  query: "?raw",
  import: "default",
  eager: true,
});
const ENTRIES: Importers = import.meta.glob("../ext/*/native/*.ts");

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
 * grammar) as the page starts, alongside the application's own chunk rather than after it. The
 * registry's later load of the same entry gets the same module.
 */
export function preloadStartup(): void {
  for (const source of builtinSources()) {
    let manifest: ReturnType<typeof parseManifest>;
    try {
      manifest = parseManifest(source);
    } catch {
      continue; // the registry refuses it, and says so
    }
    if (!manifest.activation.includes("startup")) continue;
    const { viewers, grammars } = manifest.contributes;
    for (const c of [...viewers, ...grammars])
      void ENTRIES[`../ext/${source.folder}/${c.entry}`]?.().catch(() => undefined);
  }
}
