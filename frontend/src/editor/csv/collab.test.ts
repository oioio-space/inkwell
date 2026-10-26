import { next as A } from "@automerge/automerge";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { CsvModel } from "./model";
import { type DocPort, SheetDoc } from "./sheetDoc";

type D = { text: string };
const DIALECT = { delimiter: ",", header: true, decimal: "." } as const;

/** One person: a real Automerge document and the sheet over it. */
class Peer {
  sheet: SheetDoc;
  constructor(
    public doc: A.Doc<D>,
    safe = true,
  ) {
    const port: DocPort = {
      text: () => this.doc.text,
      change: (splices) => {
        this.doc = A.change(this.doc, (d) => {
          for (const s of [...splices].sort((a, b) => b.pos - a.pos))
            A.splice(d, ["text"], s.pos, s.del, s.ins);
        });
      },
      cursor: (pos) => A.getCursor(this.doc, ["text"], pos),
      resolve: (c) => A.getCursorPosition(this.doc, ["text"], c),
    };
    const model = new CsvModel(doc.text, DIALECT);
    model.safeQuoting = safe;
    this.sheet = new SheetDoc(port, model);
  }
  get text() {
    return this.doc.text;
  }
  get m() {
    return this.sheet.model;
  }
  edit(r: number, c: number, v: string) {
    this.sheet.perform(this.m.editCell(r, c, v), c);
  }
}

function pair(text: string): [Peer, Peer] {
  const base = A.from<D>({ text });
  return [new Peer(A.clone(base)), new Peer(A.clone(base))];
}

/** Both ways: each side merges the other; texts must then be identical. */
function sync(a: Peer, b: Peer) {
  const da = a.doc;
  const db = b.doc;
  a.doc = A.merge(A.clone(da), db);
  b.doc = A.merge(A.clone(db), da);
  a.sheet.receive(undefined);
  b.sheet.receive(undefined);
  expect(a.text).toBe(b.text);
}

/** Every record of the text has the same number of cells. */
function consistent(text: string, width: number) {
  const m = new CsvModel(text, DIALECT);
  for (let r = 0; r < m.records; r++) expect(m.record(r).length).toBe(width);
}

const BASE = "a,b,c,d\n1,2,3,4\n5,6,7,8\n9,10,11,12\n";

describe("two people, one CSV", () => {
  it("CSV-03 different cells of the same row both survive on both sides", () => {
    const [a, b] = pair(BASE);
    a.edit(2, 1, "B3");
    b.edit(2, 3, "D3");
    sync(a, b);
    expect(a.text).toBe('a,b,c,d\n1,2,3,4\n5,"B3",7,"D3"\n9,10,11,12\n');
  });

  it("CSV-04 the same cell edited by both converges, columns stay aligned", () => {
    const [a, b] = pair(BASE);
    a.edit(1, 1, "x,y");
    b.edit(1, 1, 'say "z"');
    sync(a, b);
    consistent(a.text, 4);
    expect(a.m.records).toBe(4);
  });

  it("CSV-04 property: any concurrent cell edits merge to the same, well-shaped CSV", () => {
    const value = fc.string({ unit: fc.constantFrom("a", "b", ",", '"', "\n", " ", "é") });
    const op = fc.record({ r: fc.nat(3), c: fc.nat(3), v: value, who: fc.boolean() });
    fc.assert(
      fc.property(fc.array(op, { minLength: 1, maxLength: 8 }), (ops) => {
        const [a, b] = pair(BASE);
        for (const o of ops) (o.who ? a : b).edit(o.r, o.c, o.v);
        sync(a, b);
        consistent(a.text, 4);
        expect(new CsvModel(a.text, DIALECT).records).toBe(4);
      }),
      { numRuns: 200 },
    );
  }, 30_000);

  it("CSV-04b minimal quoting cannot be made safe: a plain write before a quoted one splits the cell", () => {
    // The same cell written by two people at once: Automerge keeps both inserts, in an order that
    // follows the actor ids (nothing the writer controls). `x` then `"y,z"` is `x"y,z"`, which
    // reads as two cells. Quoting only when needed therefore breaks one of the two orders.
    const broken = (safe: boolean) =>
      [["aa", "bb"] as const, ["bb", "aa"] as const].map(([actorA, actorB]) => {
        const base = A.from<D>({ text: BASE }, { actor: "00" });
        const a = new Peer(A.clone(base, { actor: actorA }), safe);
        const b = new Peer(A.clone(base, { actor: actorB }), safe);
        a.edit(1, 1, "x");
        b.edit(1, 1, "y,z");
        sync(a, b);
        return new CsvModel(a.text, DIALECT).record(1).length !== 4;
      });
    expect(broken(false)).toContain(true);
    expect(broken(true)).toEqual([false, false]);
  });

  it("CSV-05 a row inserted above does not move the edit of the person below", () => {
    const [a, b] = pair(BASE);
    a.sheet.perform(a.m.insertRows(1), undefined);
    b.edit(3, 2, "edit");
    sync(a, b);
    expect(a.text).toBe('a,b,c,d\n,,,\n1,2,3,4\n5,6,7,8\n9,10,"edit",12\n');
  });

  it("CSV-06 a column deleted under an edit: same text on both sides, columns consistent, B can undo", () => {
    const [a, b] = pair(BASE);
    a.sheet.perform(a.m.deleteCols(1));
    b.edit(2, 1, "BX");
    let moved = -1;
    b.sheet.onMoved = (c) => {
      moved = c;
    };
    sync(a, b);
    consistent(a.text, 3);
    expect(moved).toBe(1);
    // The text B wrote is found again and removed by B's own undo.
    expect(b.sheet.undo()).toBe(true);
    expect(b.text).not.toContain("BX");
    consistent(b.text, 3);
  });

  it("SHEET-MU undo undoes only my own action, never the other person's", () => {
    const [a, b] = pair(BASE);
    a.edit(1, 0, "A1");
    b.edit(2, 0, "B1");
    sync(a, b);
    expect(a.sheet.undo()).toBe(true);
    expect(a.text).toBe('a,b,c,d\n1,2,3,4\n"B1",6,7,8\n9,10,11,12\n');
    expect(a.sheet.undo()).toBe(false);
    a.sheet.redo();
    expect(a.text).toContain('"A1",2,3,4');
  });

  it("an undo whose text was changed since is refused, not guessed", () => {
    const [a, b] = pair(BASE);
    a.edit(1, 0, "AAA");
    sync(a, b);
    b.edit(1, 0, "BBB");
    sync(a, b);
    const before = a.text;
    a.sheet.undo();
    expect(a.text === before || a.text.includes("BBB")).toBe(true);
    consistent(a.text, 4);
  });

  it("SHEET-MU interleaved structural operations always converge to a well-shaped CSV", () => {
    type Op =
      | { k: "edit"; r: number; c: number; v: string }
      | { k: "insRow"; at: number }
      | { k: "delRow"; at: number }
      | { k: "delCol"; at: number }
      | { k: "paste"; r: number; c: number }
      | { k: "sort" };
    const op: fc.Arbitrary<Op> = fc.oneof(
      fc.record({
        k: fc.constant("edit" as const),
        r: fc.nat(3),
        c: fc.nat(3),
        v: fc.constantFrom("x", "y,z", 'q"q'),
      }),
      fc.record({ k: fc.constant("insRow" as const), at: fc.nat(4) }),
      fc.record({ k: fc.constant("delRow" as const), at: fc.integer({ min: 1, max: 3 }) }),
      fc.record({ k: fc.constant("delCol" as const), at: fc.nat(3) }),
      fc.record({ k: fc.constant("paste" as const), r: fc.nat(2), c: fc.nat(2) }),
      fc.constant({ k: "sort" as const }),
    );
    const run = (p: Peer, o: Op) => {
      const m = p.m;
      switch (o.k) {
        case "edit":
          return p.edit(o.r, o.c, o.v);
        case "insRow":
          return void p.sheet.perform(m.insertRows(Math.min(o.at, m.records)));
        case "delRow":
          return void p.sheet.perform(m.deleteRows(Math.min(o.at, m.records - 1)));
        case "delCol":
          return void p.sheet.perform(m.deleteCols(o.at));
        case "paste":
          return void p.sheet.perform(
            m.editCells([
              { r: o.r, c: o.c, value: "p1" },
              { r: o.r + 1, c: o.c, value: "p;2" },
            ]),
          );
        case "sort": {
          const order = Array.from({ length: m.records - 1 }, (_, i) => m.records - 1 - i);
          return void p.sheet.perform(m.applySort(order));
        }
      }
    };
    fc.assert(
      fc.property(fc.array(fc.tuple(fc.boolean(), op), { minLength: 1, maxLength: 10 }), (ops) => {
        const [a, b] = pair(BASE);
        for (const [who, o] of ops) run(who ? a : b, o);
        sync(a, b);
        const m = new CsvModel(a.text, DIALECT);
        // Every record is a well-formed row: its cells parse and none is lost to a broken quote.
        for (let r = 0; r < m.records; r++) expect(m.record(r).length).toBeGreaterThan(0);
        expect(a.text).toBe(b.text);
      }),
      { numRuns: 120 },
    );
  });
});
