/**
 * The application's registry of contributions, built once at start from the built-in manifests
 * (ADR 0027). Consumers read this frozen snapshot; nothing else knows that contributions exist.
 */
import { builtinSources, globLoader } from "./loader";
import { buildRegistry, type Registry } from "./registry";

export type {
  ContentClass,
  FileType,
  Grammar,
  Opening,
  Registry,
  Viewer,
} from "./registry";
export { BINARY_TYPE, TEXT_TYPE } from "./registry";

export const registry: Registry = buildRegistry(builtinSources(), globLoader());
for (const r of registry.refused) console.error(`extension ${r.folder} refused: ${r.reason}`);

let started = false;

/**
 * Makes the grammars of the extensions activated at `startup` ready (Typst's), so the main
 * document opens with its colours. Their entries, and the text editor's, already load from the
 * page's start (`preloadStartup`, main.ts); this only builds the languages once they are there.
 */
export function activateStartup(): void {
  if (started) return;
  started = true;
  const at = new Set(
    registry.extensions.filter((e) => e.activation.includes("startup")).map((e) => e.id),
  );
  for (const g of registry.grammars) if (at.has(g.extension)) void registry.languageSupport(g.id);
}
