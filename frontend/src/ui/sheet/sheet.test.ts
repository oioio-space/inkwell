import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { CsvModel } from "../../editor/csv/model";
import { type DocPort, SheetDoc } from "../../editor/csv/sheetDoc";
import { SheetController, type SheetOptions } from "./sheet.svelte";

const DIALECT = { delimiter: ";", header: true, decimal: "," } as const;
const BASE = "nom;qte;prix\nA;1;1,5\nB;2;2,5\nC;3;3,5\nD;4;4,5\n";

/** A sheet over a real Automerge document, counting the changes it makes. */
function open(text = BASE, opts: SheetOptions = {}) {
  const state = { doc: A.from({ text }), changes: 0 };
  const port: DocPort = {
    text: () => state.doc.text,
    change: (splices) => {
      state.changes++;
      state.doc = A.change(state.doc, (d) => {
        for (const s of [...splices].sort((a, b) => b.pos - a.pos))
          A.splice(d, ["text"], s.pos, s.del, s.ins);
      });
    },
    cursor: (pos) => A.getCursor(state.doc, ["text"], pos),
    resolve: (c) => A.getCursorPosition(state.doc, ["text"], c),
  };
  const sheet = new SheetController(new SheetDoc(port, new CsvModel(text, DIALECT)), opts);
  return { sheet, state, text: () => state.doc.text };
}

describe("selection and navigation", () => {
  it("SHEET-KEY arrows, Shift+arrows, Ctrl+arrows, select all, name box", () => {
    const { sheet } = open();
    sheet.move(1, 1);
    expect(sheet.active).toEqual({ r: 1, c: 1 });
    sheet.move(1, 0, true);
    expect(sheet.primary).toEqual({ r0: 1, r1: 2, c0: 1, c1: 1 });
    sheet.select(0, 0);
    sheet.edge(1, 0);
    expect(sheet.active.r).toBe(3);
    sheet.edge(-1, 0);
    expect(sheet.active.r).toBe(0);
    sheet.selectAll();
    expect(sheet.primary).toEqual({ r0: 0, r1: 3, c0: 0, c1: 2 });
    expect(sheet.reference).toBe("A2:C5");
    expect(sheet.jumpTo("B3")).toBe(true);
    expect(sheet.active).toEqual({ r: 1, c: 1 });
    expect(sheet.reference).toBe("B3");
    expect(sheet.jumpTo("nope")).toBe(false);
  });

  it("SHEET-KEY Enter and Tab cycle inside a selection", () => {
    const { sheet } = open();
    sheet.select(0, 0);
    sheet.select(1, 1, true);
    sheet.startEdit();
    sheet.commitEdit({ dr: 1, dc: 0 });
    expect(sheet.active).toEqual({ r: 1, c: 0 });
    sheet.startEdit();
    sheet.commitEdit({ dr: 1, dc: 0 });
    expect(sheet.active).toEqual({ r: 0, c: 1 });
  });

  it("SHEET-STATUS selection statistics", async () => {
    const { sheet } = open();
    sheet.select(0, 2);
    sheet.select(3, 2, true);
    sheet.computeStats();
    expect(sheet.stats).toMatchObject({ count: 4, sum: 12, min: 1.5, max: 4.5 });
  });
});

describe("editing", () => {
  it("CSV-02 / SHEET one edit is one change and one undo step", () => {
    const { sheet, state, text } = open();
    sheet.select(0, 0);
    sheet.startEdit("x;y");
    sheet.commitEdit();
    expect(text()).toBe('nom;qte;prix\n"x;y";1;1,5\nB;2;2,5\nC;3;3,5\nD;4;4,5\n');
    expect(state.changes).toBe(1);
    expect(sheet.undo()).toBe(true);
    expect(text()).toBe(BASE);
  });

  it("an unchanged cell does not touch the text", () => {
    const { sheet, state } = open();
    sheet.startEdit();
    sheet.commitEdit();
    expect(state.changes).toBe(0);
  });

  it("CSV-21 a formula is stored as text", () => {
    const { sheet, text } = open();
    sheet.select(0, 0);
    sheet.startEdit("=SUM(A1:A3)");
    sheet.commitEdit();
    expect(text()).toContain("\n=SUM(A1:A3);1;1,5\n");
  });

  it("header rename edits the header record", () => {
    const { sheet, text } = open();
    sheet.startHeaderEdit(1);
    sheet.setEditValue("quantité");
    sheet.commitEdit();
    expect(text().startsWith("nom;quantité;prix\n")).toBe(true);
  });

  it("CSV-12 editing a visible cell of a filtered view touches the right file row", () => {
    const { sheet, text } = open();
    sheet.setFilter(0, { values: new Set(["C", "D"]) });
    expect(sheet.rowCount).toBe(2);
    sheet.select(1, 1);
    sheet.startEdit("40");
    sheet.commitEdit();
    expect(text()).toBe("nom;qte;prix\nA;1;1,5\nB;2;2,5\nC;3;3,5\nD;40;4,5\n");
  });

  it("CSV-18 a reader cannot edit, but can copy, sort and filter locally", () => {
    const { sheet, text } = open(BASE, { readOnly: true });
    sheet.startEdit("x");
    expect(sheet.editing).toBeUndefined();
    sheet.clear();
    sheet.insertRows();
    sheet.deleteRows();
    expect(text()).toBe(BASE);
    expect(sheet.copyData()?.tsv).toBe("A");
    sheet.sortBy(1, -1);
    expect(Array.from(sheet.rows ?? [])).toEqual([4, 3, 2, 1]);
    expect(text()).toBe(BASE);
  });

  it("CSV-19 a locked cell refuses edit, paste and fill, and says why", () => {
    const { sheet, text } = open(BASE, { isLocked: (r) => r === 2 });
    sheet.select(1, 0);
    sheet.startEdit("x");
    expect(sheet.editing).toBeUndefined();
    expect(sheet.notice?.text).toBeTruthy();
    sheet.select(0, 0);
    sheet.select(2, 0, true);
    sheet.paste({ text: "Z" });
    expect(text()).toBe(
      "nom;qte;prix\nZ;1;1,5\nB;2;2,5\nZ;3;3,5\nD;4;4,5\n".replace("B;2;2,5\nZ", "B;2;2,5\nZ"),
    );
  });
});

describe("copy, cut and paste", () => {
  it("CSV-07 pasting 200 x 10 cells extends the grid as one change, one undo", () => {
    const { sheet, state, text } = open();
    const big = Array.from({ length: 200 }, (_, r) =>
      Array.from({ length: 10 }, (_, c) => `r${r}c${c}`).join("\t"),
    ).join("\n");
    sheet.select(0, 0);
    expect(sheet.paste({ text: big })).toBe(0); // more than 1 000 cells: waits for a yes
    expect(sheet.pending?.rows.length).toBe(200);
    expect(sheet.confirmPaste()).toBe(2000);
    expect(state.changes).toBe(1);
    expect(sheet.model.records).toBe(201);
    expect(sheet.model.width).toBe(10);
    expect(sheet.model.value(200, 9)).toBe("r199c9");
    expect(sheet.model.index.uneven).toBe(0);
    sheet.undo();
    expect(text()).toBe(BASE);
  });

  it("copy gives TSV and an HTML table; formulas are neutralised and counted", () => {
    const { sheet } = open("a;b\n=1+1;x\n-5;y\n", {});
    sheet.selectAll();
    const c = sheet.copyData();
    expect(c?.tsv).toBe("'=1+1\tx\n-5\ty");
    expect(c?.raw).toBe("=1+1\tx\n-5\ty");
    expect(c?.neutralised).toBe(1);
    expect(c?.html).toContain("<td>&#39;=1+1</td>".replace("&#39;", "'"));
  });

  it("one value pasted over a range fills it; a range pasted over one cell extends", () => {
    const { sheet, text } = open();
    sheet.select(0, 1);
    sheet.select(2, 1, true);
    sheet.paste({ text: "9" });
    expect(text()).toBe("nom;qte;prix\nA;9;1,5\nB;9;2,5\nC;9;3,5\nD;4;4,5\n");
    sheet.select(3, 1);
    sheet.paste({ text: "p\tq\nr\ts" });
    expect(text()).toBe("nom;qte;prix\nA;9;1,5\nB;9;2,5\nC;9;3,5\nD;p;q\n;r;s\n");
  });

  it("cut then paste moves the cells as one undo step", () => {
    const { sheet, state, text } = open();
    sheet.select(0, 0);
    sheet.select(1, 1, true);
    const cut = sheet.copyData(true);
    sheet.select(2, 1);
    sheet.paste({ text: cut?.raw ?? "" });
    expect(text()).toBe("nom;qte;prix\n;;1,5\n;;2,5\nC;A;1\nD;B;2\n");
    expect(state.changes).toBe(1);
    sheet.undo();
    expect(text()).toBe(BASE);
  });

  it("pasting an HTML table from a page takes its cells", () => {
    const { sheet, text } = open();
    sheet.select(0, 0);
    sheet.paste({ html: "<table><tr><td>x</td><td><b>y</b></td></tr></table>", text: "x y" });
    expect(text().split("\n")[1]).toBe("x;y;1,5");
  });
});

describe("fill handle and fill commands", () => {
  it("SHEET-FILL drags a series down, continues a name, goes down and up", () => {
    const { sheet, text } = open("n;l\n1;Lundi\n2;\n;\n;\n;\n", {});
    sheet.select(0, 0);
    sheet.select(1, 0, true);
    sheet.fillHandle(sheet.primary, { r: 3, c: 0 });
    expect(text()).toContain("\n1;Lundi\n2;\n3;\n4;\n;\n");
    sheet.select(0, 1);
    const r = sheet.fillHandle(sheet.primary, { r: 2, c: 1 });
    expect(r).toEqual({ r0: 0, c0: 1, r1: 2, c1: 1 });
    expect(text()).toContain("\n1;Lundi\n2;Mardi\n3;Mercredi\n4;\n");
  });

  it("SHEET-FILL Ctrl+D and Ctrl+R copy the first row and column", () => {
    const { sheet, text } = open();
    sheet.select(0, 0);
    sheet.select(2, 1, true);
    sheet.fillDown();
    expect(text()).toBe("nom;qte;prix\nA;1;1,5\nA;1;2,5\nA;1;3,5\nD;4;4,5\n");
    sheet.fillRight();
    expect(text()).toBe("nom;qte;prix\nA;A;1,5\nA;A;2,5\nA;A;3,5\nD;4;4,5\n");
  });

  it("SHEET-FILL double-click on the handle fills to the end of the neighbouring data", () => {
    const { sheet, text } = open("a;b\n1;x\n;y\n;z\n");
    sheet.select(0, 0);
    sheet.fillHandleToEnd();
    expect(text()).toBe("a;b\n1;x\n1;y\n1;z\n");
  });
});

describe("rows and columns", () => {
  it("SHEET-ROWS insert above, below and several rows are one undo step each", () => {
    const { sheet, state, text } = open();
    sheet.select(1, 0);
    sheet.select(2, 0, true);
    sheet.insertRows("above");
    expect(text()).toBe("nom;qte;prix\nA;1;1,5\n;;\n;;\nB;2;2,5\nC;3;3,5\nD;4;4,5\n");
    expect(sheet.primary).toEqual({ r0: 1, r1: 2, c0: 0, c1: 2 });
    expect(state.changes).toBe(1);
    sheet.undo();
    expect(text()).toBe(BASE);
    sheet.select(0, 0);
    sheet.insertRows("below");
    expect(text()).toBe("nom;qte;prix\nA;1;1,5\n;;\nB;2;2,5\nC;3;3,5\nD;4;4,5\n");
  });

  it("SHEET-ROWS delete several rows and columns; selection stays on a real cell", () => {
    const { sheet, text } = open();
    sheet.selectRows(1, 2);
    sheet.deleteRows();
    expect(text()).toBe("nom;qte;prix\nA;1;1,5\nD;4;4,5\n");
    expect(sheet.active.r).toBe(1);
    sheet.selectColumns(1, 2);
    sheet.deleteCols();
    expect(text()).toBe("nom\nA\nD\n");
    expect(sheet.active.c).toBe(0);
  });

  it("SHEET-ROWS columns inserted and moved", () => {
    const { sheet, text } = open();
    sheet.selectColumns(1, 1);
    sheet.insertCols("right");
    expect(text()).toBe("nom;qte;;prix\nA;1;;1,5\nB;2;;2,5\nC;3;;3,5\nD;4;;4,5\n");
    sheet.selectColumns(3, 3);
    sheet.moveCols(0);
    expect(text().split("\n")[0]).toBe("prix;nom;qte;");
  });

  it("SHEET-ROWS moving rows by drag is one step and keeps the moved rows selected", () => {
    const { sheet, state, text } = open();
    sheet.selectRows(0, 0);
    sheet.moveRows(3);
    expect(text()).toBe("nom;qte;prix\nB;2;2,5\nC;3;3,5\nA;1;1,5\nD;4;4,5\n");
    expect(sheet.primary).toMatchObject({ r0: 2, r1: 2 });
    expect(state.changes).toBe(1);
  });
});

describe("views", () => {
  it("CSV-11 sorting is a view; 'Appliquer au fichier' writes it in one change", () => {
    const { sheet, state, text } = open();
    sheet.sortBy(1, -1);
    expect(sheet.value(0, 0)).toBe("D");
    expect(text()).toBe(BASE);
    sheet.applyToFile();
    expect(text()).toBe("nom;qte;prix\nD;4;4,5\nC;3;3,5\nB;2;2,5\nA;1;1,5\n");
    expect(state.changes).toBe(1);
    expect(sheet.sorted).toBe(false);
  });

  it("insert and move are refused while a view is active, with the reason", () => {
    const { sheet, text } = open();
    sheet.sortBy(1, -1);
    sheet.insertRows();
    expect(text()).toBe(BASE);
    expect(sheet.notice?.text).toBeTruthy();
  });

  it("CSV-EDGE-14 'Convertir en tabulation' rewrites the file once, as one undo step", () => {
    const { sheet, state, text } = open('a;b\n"x;y";2\n');
    sheet.convertDelimiter("\t");
    expect(text()).toBe('a\tb\n"x;y"\t2\n');
    expect(state.changes).toBe(1);
    sheet.undo();
    expect(text()).toBe('a;b\n"x;y";2\n');
  });
});

describe("geometry", () => {
  it("SHEET-ZOOM stays between 50 % and 200 %, SHEET-FREEZE pins to the selection", () => {
    const { sheet } = open();
    sheet.setZoom(5);
    expect(sheet.zoom).toBe(2);
    sheet.setZoom(0.1);
    expect(sheet.zoom).toBe(0.5);
    sheet.setZoom(1);
    sheet.select(2, 1);
    sheet.freezeToSelection();
    expect([sheet.pinnedRows, sheet.pinnedCols]).toEqual([2, 1]);
    sheet.setWidth(1, 300);
    expect(sheet.colW(1)).toBe(300);
    sheet.setWidth(1, 5);
    expect(sheet.colW(1)).toBe(32);
  });

  it("CSV-13 headers without a name read « Colonne C »", () => {
    const { sheet } = open("a;;c\n1;2;3\n");
    expect(sheet.headerName(1)).toMatch(/B/);
  });
});
