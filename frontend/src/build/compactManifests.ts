import { readFileSync } from "node:fs";
import type { Plugin } from "vite";

/** A built-in manifest read as text, as the loader asks for it. */
const MANIFEST = "/inkwell-plugin.json?raw";

/**
 * The built-ins' manifests are read as text (`?raw`, ADR 0027 rule 4) and ship in the start
 * chunk: this hands the bundle the same JSON without its whitespace (EX0c1 review). A manifest
 * that is not JSON is left as it is, so the registry refuses it with its reason as before.
 */
export const compactManifests = (): Plugin => ({
  name: "inkwell-compact-manifests",
  enforce: "pre",
  load(id) {
    if (!id.endsWith(MANIFEST)) return undefined;
    const text = readFileSync(id.slice(0, -"?raw".length), "utf8");
    let compact = text;
    try {
      compact = JSON.stringify(JSON.parse(text));
    } catch {
      // Not JSON: the registry's parser says why.
    }
    return `export default ${JSON.stringify(compact)};`;
  },
});
