import { describe, expect, it } from "vitest";
import type { Page } from "../preview/preview.svelte";
import { PublicFeed, type PublicView } from "./feed";
import type { HubFrame } from "./hub";

const page = (rev = 1): Page => ({
  rev,
  src: "data:image/svg+xml;base64,AAAA",
  width: 960,
  height: 540,
});

const view = (over: Partial<PublicView> = {}): PublicView => ({
  page: 1,
  pageCount: 5,
  screen: "show",
  laser: null,
  away: false,
  rev: 1,
  pages: new Map([1, 2, 3, 4, 5].map((n) => [n, page()])),
  ...over,
});

const types = (frames: HubFrame[]) =>
  frames.map((f) => (f.type === "page" ? `page${f.n}` : f.type));

describe("PublicFeed", () => {
  it("PRS-31 sends the current page and the next ones before the view that names them", () => {
    const out: HubFrame[] = [];
    const feed = new PublicFeed((f) => out.push(f));
    feed.update(view());
    expect(types(out)).toEqual(["page1", "page2", "page3", "view"]);
    out.length = 0;
    feed.update(view({ page: 2 }));
    expect(types(out)).toEqual(["page4", "view"]); // 2 and 3 were sent
  });

  it("sends nothing when nothing changed, and a page again when its version did", () => {
    const out: HubFrame[] = [];
    const feed = new PublicFeed((f) => out.push(f));
    feed.update(view());
    out.length = 0;
    feed.update(view());
    expect(out).toEqual([]);
    const pages = new Map([1, 2, 3].map((n) => [n, page(2)]));
    feed.update(view({ pages, rev: 2 })); // PRS-39: the document was edited
    expect(types(out)).toEqual(["page1", "page2", "page3"]); // the view itself did not change
  });

  it("PRS-12 a black screen sends no page, only the view", () => {
    const out: HubFrame[] = [];
    const feed = new PublicFeed((f) => out.push(f));
    feed.update(view({ screen: "black" }));
    expect(types(out)).toEqual(["view"]);
  });

  it("waits for a page that is not compiled yet", () => {
    const out: HubFrame[] = [];
    const feed = new PublicFeed((f) => out.push(f));
    feed.update(view({ pages: new Map() }));
    expect(types(out)).toEqual(["view"]);
    feed.update(view());
    expect(types(out)).toEqual(["view", "page1", "page2", "page3"]);
  });

  it("does not run past the last page", () => {
    const out: HubFrame[] = [];
    const feed = new PublicFeed((f) => out.push(f));
    feed.update(view({ page: 5 }));
    expect(types(out)).toEqual(["page5", "view"]);
  });

  it("says it all again when the public window asks, or lacks a page", () => {
    const out: HubFrame[] = [];
    const feed = new PublicFeed((f) => out.push(f));
    feed.update(view());
    out.length = 0;
    feed.hello();
    expect(types(out)).toEqual(["page1", "page2", "page3", "view"]);
    out.length = 0;
    feed.need(2);
    expect(types(out)).toEqual(["page2"]);
  });

  it("PRS-40 never carries more than the view and the pages", () => {
    const out: HubFrame[] = [];
    const feed = new PublicFeed((f) => out.push(f));
    feed.update(view({ laser: { x: 0.1, y: 0.2 }, away: true }));
    for (const f of out) expect(["view", "page"]).toContain(f.type);
    expect(Object.keys(out.at(-1) as object).sort()).toEqual([
      "away",
      "laser",
      "page",
      "screen",
      "type",
    ]);
  });
});
