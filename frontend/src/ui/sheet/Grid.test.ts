import { next as A } from "@automerge/automerge";
import { flushSync } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import { CsvModel } from "../../editor/csv/model";
import { type DocPort, SheetDoc } from "../../editor/csv/sheetDoc";
import { cleanup, fakeWidth, render } from "../testkit.svelte";
import Grid from "./Grid.svelte";
import { SheetController, type SheetOptions } from "./sheet.svelte";

let restore = () => {};
afterEach(() => {
  restore();
  cleanup();
});

const TEXT = "nom;qte;prix\nA;1;1,5\nB;2;2,5\nC;3;3,5\n";

function mount(opts: SheetOptions = {}) {
  restore = fakeWidth(900); // happy-dom lays nothing out
  let doc = A.from({ text: TEXT });
  const port: DocPort = {
    text: () => doc.text,
    change: (splices) => {
      doc = A.change(doc, (d) => {
        for (const s of [...splices].sort((a, b) => b.pos - a.pos))
          A.splice(d, ["text"], s.pos, s.del, s.ins);
      });
    },
    cursor: (pos) => A.getCursor(doc, ["text"], pos),
    resolve: (c) => A.getCursorPosition(doc, ["text"], c),
  };
  const ctrl = new SheetController(
    new SheetDoc(port, new CsvModel(TEXT, { delimiter: ";", header: true, decimal: "," })),
    opts,
  );
  const target = render(Grid, {
    ctrl,
    oncontext: () => {},
    onfilter: () => {},
    onfind: () => {},
  });
  return { ctrl, target };
}

describe("the grid's markup", () => {
  it("CSV-20 is an ARIA grid: rows, column headers, cells with their indexes, the active cell named", () => {
    const { ctrl, target } = mount();
    const grid = target.querySelector('[role="grid"]') as HTMLElement;
    expect(grid.getAttribute("aria-rowcount")).toBe("4");
    expect(grid.getAttribute("aria-colcount")).toBe("3");
    expect(grid.getAttribute("aria-activedescendant")).toBe("cell-0-0");
    expect(target.querySelectorAll('[role="columnheader"]').length).toBeGreaterThanOrEqual(3);
    const cell = target.querySelector("#cell-1-2") as HTMLElement;
    expect(cell.getAttribute("role")).toBe("gridcell");
    expect(cell.getAttribute("aria-colindex")).toBe("3");
    expect(cell.closest('[role="row"]')?.getAttribute("aria-rowindex")).toBe("3");
    ctrl.select(1, 2);
    flushSync();
    expect(grid.getAttribute("aria-activedescendant")).toBe("cell-1-2");
    expect(target.querySelector('[aria-live="polite"]')?.textContent).toBe(
      "ligne 3, colonne prix, 2,5",
    );
  });

  it("CSV-18 a reader's grid says it is read-only and cells are marked", () => {
    const { target } = mount({ readOnly: true });
    expect(target.querySelector('[role="grid"]')?.getAttribute("aria-readonly")).toBe("true");
    expect(target.querySelector("#cell-0-0")?.getAttribute("aria-readonly")).toBe("true");
  });

  it("CSV-19 locked cells are greyed and marked read-only, the others stay editable", () => {
    const { target } = mount({ isLocked: (record) => record === 2 });
    expect(target.querySelector("#cell-1-0")?.classList.contains("locked")).toBe(true);
    expect(target.querySelector("#cell-1-0")?.getAttribute("aria-readonly")).toBe("true");
    expect(target.querySelector("#cell-0-0")?.classList.contains("locked")).toBe(false);
  });

  it("numbers are right-aligned and shown in French, the stored text is untouched", () => {
    const { ctrl, target } = mount();
    expect(target.querySelector("#cell-0-1")?.classList.contains("num")).toBe(true);
    expect(target.querySelector("#cell-0-0")?.classList.contains("num")).toBe(false);
    expect(ctrl.value(0, 2)).toBe("1,5");
  });

  it("remote people show as coloured rectangles with their name on the first cell", () => {
    const { ctrl, target } = mount();
    ctrl.remotes = [{ peerId: "peer-a", name: "Alice", range: { r0: 0, c0: 0, r1: 1, c1: 1 } }];
    flushSync();
    expect(target.querySelectorAll(".cell.rem").length).toBe(4);
    expect(target.querySelector(".tag")?.textContent).toBe("Alice");
    expect(target.querySelector("#cell-0-0 .tag")).not.toBeNull();
  });
});
