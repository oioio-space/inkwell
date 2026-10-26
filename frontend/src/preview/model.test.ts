import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PreviewClient } from "./PreviewClient";
import { PreviewModel } from "./preview.svelte";
import { FakeSocket, problem, result, svgPage } from "./testkit";

let sockets: FakeSocket[] = [];
const open = () => {
  const s = new FakeSocket();
  sockets.push(s);
  return s;
};
const last = () => sockets[sockets.length - 1] as FakeSocket;

beforeEach(() => {
  vi.useFakeTimers();
  sockets = [];
});
afterEach(() => vi.useRealTimers());

describe("PreviewClient", () => {
  it("PRV-70 says again what Go needs after every opening", () => {
    const got: unknown[] = [];
    const c = new PreviewClient(open, (m) => got.push(m), { reconnectMs: 100 });
    c.start();
    c.view(1, 4); // before the stream is open: kept, not lost
    c.edit(1700000000);
    expect(last().sent).toEqual([]);
    last().open();
    expect(last().sent).toEqual([
      { op: "edit", time: 1700000000 },
      { op: "view", first: 1, last: 4 },
    ]);

    last().drop(); // Go went away, the page reloaded...
    expect(sockets).toHaveLength(1);
    vi.advanceTimersByTime(100);
    expect(sockets).toHaveLength(2);
    last().open();
    expect(last().sent).toEqual([
      { op: "edit", time: 1700000000 },
      { op: "view", first: 1, last: 4 },
    ]);
    c.stop();
    expect(last().closed).toBe(true);
    vi.advanceTimersByTime(1000);
    expect(sockets).toHaveLength(2); // stopped: no more reconnection
  });

  it("PRV-70 delivers the messages of Go and drops the ones it cannot read", () => {
    const got: unknown[] = [];
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const c = new PreviewClient(open, (m) => got.push(m));
    c.start();
    last().open();
    last().receive({ type: "compiling", seq: 3 });
    last().receive({ type: "garbage" });
    expect(got).toEqual([{ type: "compiling", seq: 3 }]);
    expect(warn).toHaveBeenCalledOnce();
    c.stop();
  });
});

describe("PreviewModel", () => {
  const started = () => {
    const m = new PreviewModel();
    m.start(open);
    last().open();
    return m;
  };

  it("PRV-71 a result gives the page count, the pages and the problems", () => {
    const m = started();
    m.receive({ type: "compiling", seq: 1 });
    expect(m.compiling).toBe(true);
    m.receive(result({ diagnostics: [problem(), problem({ severity: "warning" })] }));
    expect(m.compiling).toBe(false);
    expect(m.hasResult).toBe(true);
    expect(m.pageCount).toBe(3);
    expect([...m.pages.keys()]).toEqual([1, 2, 3]);
    expect(m.pages.get(1)?.width).toBeCloseTo(595.28);
    expect([m.errors, m.warnings]).toEqual([1, 1]);
    m.stop();
  });

  it("PRV-72 a failure keeps the last good pages and says why", () => {
    const m = started();
    m.receive(result());
    m.receive({
      ...result({ seq: 2, rev: 2, pageCount: 0, pages: [], diagnostics: [problem()] }),
      failure: { kind: "errors", packages: [] },
    });
    expect(m.failure?.kind).toBe("errors");
    expect(m.pageCount).toBe(3);
    expect(m.pages.size).toBe(3);
    expect(m.errors).toBe(1);
    m.receive(result({ seq: 3, rev: 3, pageCount: 2, pages: [svgPage(), svgPage()] }));
    expect(m.failure).toBeUndefined();
    expect(m.pageCount).toBe(2);
    expect([...m.pages.keys()]).toEqual([1, 2]); // page 3 is gone with the text
    m.stop();
  });

  it("PRV-73 a page that cannot be shown safely is marked, not rendered", () => {
    const m = started();
    m.receive(result({ pageCount: 1, pages: ['<svg width="1pt" height="1pt"><script/></svg>'] }));
    expect(m.pages.get(1)?.src).toBe("");
    m.stop();
  });

  it("PRV-74 asks Go only for the pages in view that are missing or out of date", () => {
    const m = started();
    m.receive(result({ pageCount: 30, pages: [svgPage(), svgPage(), svgPage(), svgPage()] }));
    const views = () => last().sent.filter((r) => r.op === "view");
    m.setVisible(1, 2);
    expect(views()).toEqual([]); // pages 1 to 3 are here

    m.setVisible(10, 11); // scrolled far
    expect(views()).toEqual([{ op: "view", first: 9, last: 12 }]);
    m.setVisible(10, 11); // asking twice for the same thing is not asking twice
    expect(views()).toHaveLength(1);

    m.receive(
      result({
        seq: 2,
        rev: 1,
        first: 9,
        pageCount: 30,
        pages: [svgPage(), svgPage(), svgPage(), svgPage()],
      }),
    );
    m.setVisible(10, 11);
    expect(views()).toHaveLength(1); // arrived, same version: nothing to ask

    m.receive(
      result({
        seq: 3,
        rev: 2,
        first: 9,
        pageCount: 30,
        pages: [svgPage(), svgPage(), svgPage(), svgPage()],
      }),
    ); // the text changed
    m.setVisible(1, 2); // pages 1-3 are of the old version now
    expect(views().at(-1)).toEqual({ op: "view", first: 1, last: 3 });
    m.stop();
  });

  it("PRV-74 the window never exceeds what Go renders at once, and far pages are forgotten", () => {
    const m = started();
    m.receive(result({ pageCount: 500, pages: [svgPage()] }));
    m.setVisible(100, 400);
    const view = last().sent.find((r) => r.op === "view") as { first: number; last: number };
    expect(view.last - view.first + 1).toBeLessThanOrEqual(40);
    expect(m.pages.has(1)).toBe(false);
    m.stop();
  });

  it("PRV-75 anchors are asked for once and kept for the same version", async () => {
    const m = started();
    m.receive(result());
    const a = m.anchors();
    const b = m.anchors(); // while the first is pending: still one request
    expect(last().sent.filter((r) => r.op === "anchors")).toHaveLength(1);
    m.receive({ type: "anchors", anchors: [{ heading: true, page: 1, x: 1, y: 2, text: "T" }] });
    expect(await a).toHaveLength(1);
    expect(await b).toHaveLength(1);
    expect(await m.anchors()).toHaveLength(1);
    expect(last().sent.filter((r) => r.op === "anchors")).toHaveLength(1); // cached

    m.receive(result({ seq: 2, rev: 2 }));
    const c = m.anchors();
    expect(last().sent.filter((r) => r.op === "anchors")).toHaveLength(2);
    m.receive({ type: "anchors", anchors: [], failure: { kind: "errors", packages: [] } });
    await expect(c).rejects.toThrow("errors");
    m.stop();
  });

  it("PRV-75 anchors that never come time out", async () => {
    const m = started();
    m.receive(result());
    const p = m.anchors();
    const assertion = expect(p).rejects.toThrow("timeout");
    vi.advanceTimersByTime(61_000);
    await assertion;
    m.stop();
  });

  it("PRV-76 an edit says at once that the preview is out of date, and the mention goes by itself if nothing follows", () => {
    const m = started();
    m.noteEdit(1700000000); // nothing shown yet: nothing to call out of date
    expect(m.compiling).toBe(false);
    m.receive(result());
    m.noteEdit(1700000001);
    expect(m.compiling).toBe(true);
    vi.advanceTimersByTime(3000); // the change left the file as it was: Go never compiled
    expect(m.compiling).toBe(false);

    m.noteEdit(1700000002);
    m.receive({ type: "compiling", seq: 5 });
    vi.advanceTimersByTime(3000);
    expect(m.compiling).toBe(true); // Go is on it: it stays until the result
    m.receive(result({ seq: 5, rev: 5 }));
    expect(m.compiling).toBe(false);
    m.stop();
  });

  it("PRV-76 the date of the document goes to Go once it is known", () => {
    const m = started();
    m.noteEdit(0); // no change yet: nothing to say
    m.noteEdit(1700000000);
    expect(last().sent).toEqual([{ op: "edit", time: 1700000000 }]);
    m.stop();
  });
});
