/**
 * The application's registry of contributions, built once at start from the built-in manifests
 * (ADR 0027). Consumers read this frozen snapshot; nothing else knows that contributions exist.
 */
import { readUserBindings } from "./bindings";
import { useRegistry } from "./commands";
import { manySources } from "./e2eMany";
import { builtinFormatEntry } from "./formatEntries";
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

// Timed for the start's budget (PERF-START, ADR 0038): `inkwell:registry` in the page's measures.
performance.mark("inkwell:registry:start");
export const registry: Registry = buildRegistry(
  [...builtinSources(), ...(import.meta.env.VITE_E2E_OBSERVE === "1" ? manySources() : [])],
  globLoader(),
  { formatEntry: builtinFormatEntry },
);
for (const r of registry.refused) console.error(`extension ${r.folder} refused: ${r.reason}`);

/** The person's own keybindings on this device (UX1-5 edits them); unreadable means none. */
function storedBindings(): string | null {
  try {
    return localStorage.getItem("inkwell.keybindings");
  } catch {
    return null;
  }
}
useRegistry(registry, { user: readUserBindings(storedBindings()) });
performance.measure("inkwell:registry", "inkwell:registry:start");

/**
 * Loads the command entries (ext/app, ext/typst) once the page is idle: the first paint does not
 * wait for them, and a command run before they are there loads its entry then (`run()`).
 */
export function warmCommands(): void {
  const load = () => {
    for (const c of registry.commands) void registry.loadCommand(c)?.catch(() => undefined);
  };
  if (typeof requestIdleCallback === "function") requestIdleCallback(load, { timeout: 2000 });
  else setTimeout(load, 500);
}

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
