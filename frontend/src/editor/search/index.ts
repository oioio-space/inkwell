/**
 * Search and replace (plan B6, cases SRC-01…22). Pure TypeScript, no UI.
 *
 * Wiring it into the editor (task A4):
 *
 *   import { Prec } from "@codemirror/state";
 *   import { keymap } from "@codemirror/view";
 *   import { compileQuery, searchExtension, setSearchMatcher } from "../editor/search";
 *
 *   // in createEditor's `extensions`:
 *   searchExtension({
 *     find: () => panel.open("file"),            // Ctrl+F
 *     replace: () => panel.open("replace"),      // Ctrl+H
 *     findInProject: () => panel.open("project"),// Ctrl+Shift+F
 *     next: (back) => panel.step(back),          // F3 / Shift+F3
 *     nextResult: (back) => panel.stepResult(back), // F4 / Shift+F4
 *     close: () => panel.close(),                // Escape
 *   })
 *
 *   // when the query changes (SearchPanel's `onquery`), highlight it in the editor:
 *   const r = compileQuery(spec);
 *   view.dispatch({ effects: setSearchMatcher.of(r.ok ? r.matcher : null) });
 *
 *   // SearchPanel (ui/search/SearchPanel.svelte) takes the project's files, a resolver for
 *   // live positions and the locked zones; it emits `navigate({ path, from, to })`: open the
 *   // file, then view.dispatch({ selection: { anchor: from, head: to }, scrollIntoView: true }).
 *
 * Regex runs in a Web Worker (editor/search/worker.ts, bundled by Vite) with a 200 ms limit;
 * `setDefaultRunner(inlineRunner)` turns that off where there is no Worker.
 */
import { type Extension, Prec } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { searchHighlight } from "./highlight";
import { type SearchActions, searchKeymap } from "./keymap";

export { type FindOptions, findAll, findAllSync, type Match, SearchTooCostly } from "./find";
export { searchHighlight, setSearchMatcher } from "./highlight";
export { type SearchActions, searchKeymap } from "./keymap";
export {
  type FileFilter,
  type FileHits,
  findInProject,
  type Hit,
  isSearchable,
  type ProjectFileInput,
  type ProjectHits,
  type ProjectOptions,
  parseFilters,
  type ResolvedHit,
  resolveHit,
} from "./project";
export {
  type CompileResult,
  compileQuery,
  escapeRegex,
  type Matcher,
  type QuerySpec,
  type SearchMode,
} from "./query";
export {
  expandReplacement,
  type PreviewLine,
  previewReplace,
  type ReplaceOptions,
  type ReplaceReport,
  type ReplaceTarget,
  replaceAll,
  replaceInProject,
} from "./replace";
export {
  createWorkerRunner,
  inlineRunner,
  type SearchRunner,
  setDefaultRunner,
  type WorkerLike,
} from "./runner";

/** Everything the editor needs: the keys (highest precedence) and the in-text highlight. */
export function searchExtension(actions: SearchActions): Extension {
  return [Prec.high(keymap.of(searchKeymap(actions))), searchHighlight()];
}
