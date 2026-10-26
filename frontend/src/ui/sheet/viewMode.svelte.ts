/** Files that open as a spreadsheet: delimited text tables. */
export const isTabular = (path: string): boolean => /\.(csv|tsv|psv|tab)$/i.test(path);

/**
 * Which view of a table file is open: the grid, or the raw text in the editor. The caret's
 * text offset travels with the switch, so the same cell is under it on the other side.
 */
class SheetView {
  text = $state(false);
  /** Other files the person chose to open as a table (a `.txt` that looks like one). */
  #asTable = $state.raw<ReadonlySet<string>>(new Set());
  offset = $state<number | undefined>();

  /** Whether `path` is shown as a spreadsheet: a table file, or a text the person opened as one. */
  tabular(path: string): boolean {
    return isTabular(path) || this.#asTable.has(path);
  }

  openAsTable(path: string) {
    this.#asTable = new Set([...this.#asTable, path]);
    this.toGrid(undefined);
  }

  toText(offset: number) {
    this.offset = offset;
    this.text = true;
  }

  toGrid(offset: number | undefined) {
    this.offset = offset;
    this.text = false;
  }
}

export const sheetView = new SheetView();

/** A table shows as a spreadsheet, unless the person asked for its text. */
export const showsSheet = (path: string): boolean => sheetView.tabular(path) && !sheetView.text;
