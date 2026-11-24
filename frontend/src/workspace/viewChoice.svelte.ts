import { readsAsTable } from "../editor/csv/dialect";
import { type FileType, type Opening, registry, TEXT_TYPE } from "../extensions";
import { safeGet, safeSet } from "../store/safeStorage";

/** Enough for every file a person opens another way; the oldest choices go first. */
const MAX_CHOICES = 500;

/**
 * A choice per file, remembered on this device only, per project, never shared with the others:
 * the type a file opens as (« open with »), or the view it shows in (its text rather than its
 * table). Both are kept the same way (code review and UX-EX0-5).
 */
class PerFile {
  #key: string;
  #map = $state.raw<ReadonlyMap<string, string>>(new Map());

  constructor(key: string) {
    this.#key = key;
    try {
      const saved: unknown = JSON.parse(safeGet(key) ?? "[]");
      if (!Array.isArray(saved)) return;
      this.#map = new Map(
        saved.filter(
          (e): e is [string, string] =>
            Array.isArray(e) && typeof e[0] === "string" && typeof e[1] === "string",
        ),
      );
    } catch {
      // nothing remembered
    }
  }

  get(project: string, path: string): string | undefined {
    return this.#map.get(`${project}\n${path}`);
  }

  /** Remembers `value` for the file (undefined forgets it). */
  set(project: string, path: string, value: string | undefined): void {
    const key = `${project}\n${path}`;
    if (this.#map.get(key) === value) return;
    const next = new Map(this.#map);
    next.delete(key);
    if (value !== undefined) next.set(key, value);
    while (next.size > MAX_CHOICES) next.delete(next.keys().next().value as string);
    this.#map = next;
    safeSet(this.#key, JSON.stringify([...next]));
  }
}

/**
 * How a file is shown. The type chosen for it and the view the person switched it to (undefined:
 * its default) are remembered per file (`PerFile`); the caret `offset` travels with a switch and
 * is dropped when another file opens.
 */
class ViewChoice {
  offset = $state<number | undefined>();
  #types = new PerFile("inkwell.openWith");
  #views = new PerFile("inkwell.viewWith");

  /** The type the person chose for `path` in `project`, if any. */
  typeFor(project: string, path: string): string | undefined {
    return this.#types.get(project, path);
  }

  /** The view the person chose for `path` in `project`; undefined for its default. */
  viewerFor(project: string, path: string): string | undefined {
    return this.#views.get(project, path);
  }

  /** Opens `path` as `type` from now on, on this device; its default view shows. */
  chooseType(project: string, path: string, type: string): void {
    this.#types.set(project, path, type);
    this.show(project, path, undefined, undefined);
  }

  /** Opens `path` as a table from now on: the « Ouvrir comme tableau ? » offer, and the tests. */
  openAsTable(project: string, path: string): void {
    const table = tableType();
    if (table) this.chooseType(project, path, table.id);
  }

  /** Shows `path` in `viewer` (undefined: its default) from now on, its caret at `offset`. */
  show(
    project: string,
    path: string,
    viewer: string | undefined,
    offset: number | undefined,
  ): void {
    this.#views.set(project, path, viewer);
    this.offset = offset;
  }

  /** Another file opens: no caret travels to it. */
  arrive(): void {
    this.offset = undefined;
  }
}

/** The type a file opens as when it is opened as a table: the first one the grid shows by default. */
function tableType(): FileType | undefined {
  return registry.fileTypes.find((t) => registry.viewersFor(t).default.id === "tables.grid");
}

/** The grid can be offered: it has not failed to load this session. */
function tableWorks(): boolean {
  const table = tableType();
  return table !== undefined && !registry.hasFailed(registry.viewersFor(table).default);
}

export const viewChoice = new ViewChoice();

/**
 * Whether a file is offered as a table in the view bar: a text of the plain `text` type, as its
 * name made it and Go read it, whose content reads as a table, while the grid works (a grid that
 * failed to load this session is not offered again: its button would do nothing, UX-EX0-2). Never
 * the main document, which the caller leaves out, so the check never runs at start.
 */
export function offersTable(opening: Opening, text: string | undefined): boolean {
  return (
    opening.type.id === TEXT_TYPE &&
    opening.content === "text" &&
    !opening.chosen &&
    text !== undefined &&
    tableWorks() &&
    readsAsTable(text)
  );
}
