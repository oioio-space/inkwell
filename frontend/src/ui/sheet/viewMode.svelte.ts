/** Files that open as a spreadsheet: delimited text tables. */
export const isTabular = (path: string): boolean => /\.(csv|tsv|psv|tab)$/i.test(path);

/**
 * Which view of a table file is open: the grid, or the raw text in the editor. The caret's
 * text offset travels with the switch, so the same cell is under it on the other side.
 */
class SheetView {
  text = $state(false);
  offset = $state<number | undefined>();

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
