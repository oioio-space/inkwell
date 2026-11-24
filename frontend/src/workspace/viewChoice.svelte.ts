import { readsAsTable } from "../editor/csv/dialect";
import { type Opening, registry, TEXT_TYPE } from "../extensions";
import { safeGet, safeSet } from "../store/safeStorage";

/** Where this device keeps the types the person chose for files (« open with »). */
const KEY = "inkwell.openWith";
/** Enough for every file a person opens another way; the oldest choices go first. */
const MAX_CHOICES = 500;

function load(): Map<string, string> {
  try {
    const saved: unknown = JSON.parse(safeGet(KEY) ?? "[]");
    if (!Array.isArray(saved)) return new Map();
    return new Map(
      saved.filter(
        (e): e is [string, string] =>
          Array.isArray(e) && typeof e[0] === "string" && typeof e[1] === "string",
      ),
    );
  } catch {
    return new Map();
  }
}

/**
 * How the open file is shown. `viewer` is the view the person switched to (the text of a table),
 * undefined for the file's default; it lasts while the file stays open, as the caret `offset`
 * that travels with the switch. The type chosen for a file (a text opened as a table) is
 * remembered on this device only, per project, never shared with the others.
 */
class ViewChoice {
  viewer = $state<string | undefined>();
  offset = $state<number | undefined>();
  #types = $state.raw<ReadonlyMap<string, string>>(load());

  /** The type the person chose for `path` in `project`, if any. */
  typeFor(project: string, path: string): string | undefined {
    return this.#types.get(`${project}\n${path}`);
  }

  /** Opens `path` as `type` from now on, on this device; its default view shows. */
  chooseType(project: string, path: string, type: string): void {
    const next = new Map(this.#types);
    const key = `${project}\n${path}`;
    next.delete(key);
    next.set(key, type);
    while (next.size > MAX_CHOICES) next.delete(next.keys().next().value as string);
    this.#types = next;
    safeSet(KEY, JSON.stringify([...next]));
    this.show(undefined, undefined);
  }

  /**
   * Opens `path` as a table: the first type the grid shows by default. The person cannot ask for
   * it in this slice (the « looks tabular » suggestion is a later point); the tests and the
   * native walk do, to show a document as a grid.
   */
  openAsTable(project: string, path: string): void {
    const table = registry.fileTypes.find(
      (t) => registry.viewersFor(t).default.id === "tables.grid",
    );
    if (table) this.chooseType(project, path, table.id);
  }

  /** Shows `viewer` (undefined: the default one), its caret at `offset`. */
  show(viewer: string | undefined, offset: number | undefined): void {
    this.viewer = viewer;
    this.offset = offset;
  }
}

export const viewChoice = new ViewChoice();

/**
 * Whether a file is offered as a table in the view bar: a text of the plain `text` type, as its
 * name made it and Go read it, whose content reads as a table. Never the main document, which the
 * caller leaves out, so the check never runs at start.
 */
export function offersTable(opening: Opening, text: string | undefined): boolean {
  return (
    opening.type.id === TEXT_TYPE &&
    opening.content === "text" &&
    !opening.chosen &&
    text !== undefined &&
    readsAsTable(text)
  );
}
