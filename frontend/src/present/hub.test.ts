import fc from "fast-check";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeSocket } from "../preview/testkit";
import { encodeHub, HubClient, type HubFrame, parseHub } from "./hub";

const img = "data:image/svg+xml;base64,PHN2Zy8+";

describe("hub frames", () => {
  it("PRS-64 round-trips every frame", () => {
    const frames: HubFrame[] = [
      { type: "view", page: 3, screen: "black", laser: { x: 0.5, y: 0.5 }, away: true },
      { type: "view", page: 1, screen: "show", laser: null, away: false },
      { type: "page", n: 2, rev: 5, src: img, width: 960, height: 540 },
      { type: "page", n: 2, rev: 5, src: "", width: 960, height: 540 },
      { type: "hello" },
      { type: "need", n: 4 },
      { type: "cmd", op: "next" },
      { type: "roster", windows: 2 },
    ];
    for (const f of frames) expect(parseHub(encodeHub(f))).toEqual(f);
  });

  it("PRS-64 refuses an image that is not a plain SVG data URL, and sizes that are not sizes", () => {
    const page = (src: string, width = 10, height = 10) =>
      JSON.stringify({ type: "page", n: 1, rev: 1, src, width, height });
    expect(parseHub(page("https://example.org/x.svg"))).toBeUndefined();
    expect(parseHub(page("javascript:alert(1)"))).toBeUndefined();
    expect(parseHub(page("data:text/html;base64,PGI+"))).toBeUndefined();
    expect(parseHub(page(`${img}"onerror="x`))).toBeUndefined();
    expect(parseHub(page(img, 0, 10))).toBeUndefined();
    expect(parseHub(page(img, Number.NaN, 10))).toBeUndefined();
    expect(parseHub(page(`data:image/svg+xml;base64,${"A".repeat(9 << 20)}`))).toBeUndefined();
  });

  it("PRS-64 refuses other commands, screens and positions", () => {
    for (const bad of [
      { type: "cmd", op: "rm" },
      { type: "view", page: 0, screen: "show", laser: null },
      { type: "view", page: 1, screen: "red", laser: null },
      { type: "view", page: 1, screen: "show", laser: { x: 3, y: 0 } },
      { type: "need", n: 0 },
      { type: "roster", windows: -1 },
      { type: "other" },
      [],
      "x",
    ])
      expect(parseHub(JSON.stringify(bad)), JSON.stringify(bad)).toBeUndefined();
    expect(parseHub("not json")).toBeUndefined();
  });

  it("PRS-64 never throws, whatever arrives", () => {
    fc.assert(
      fc.property(fc.oneof(fc.string(), fc.uint8Array(), fc.json()), (v) => {
        parseHub(typeof v === "string" ? v : v);
        return true;
      }),
      { numRuns: 500 },
    );
  });
});

describe("HubClient", () => {
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

  it("PRS-65 tells its owner at each opening, and sends only while open", () => {
    const got: HubFrame[] = [];
    let opened = 0;
    const c = new HubClient(
      open,
      (f) => got.push(f),
      () => opened++,
      100,
    );
    c.start();
    c.send({ type: "hello" }); // not open yet
    expect(last().sent).toEqual([]);
    last().open();
    expect(opened).toBe(1);
    c.send({ type: "hello" });
    expect(last().sent).toEqual([{ type: "hello" }]);
    last().receive({ type: "roster", windows: 2 });
    expect(got).toEqual([{ type: "roster", windows: 2 }]);
    last().receive({ type: "bogus" });
    expect(got).toHaveLength(1);

    last().drop();
    vi.advanceTimersByTime(100);
    expect(sockets).toHaveLength(2);
    last().open();
    expect(opened).toBe(2);
    c.stop();
    expect(last().closed).toBe(true);
    vi.advanceTimersByTime(1000);
    expect(sockets).toHaveLength(2);
  });
});
