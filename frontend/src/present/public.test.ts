import { describe, expect, it } from "vitest";
import { FakeSocket } from "../preview/testkit";
import { PublicModel } from "./public.svelte";

const SRC = "data:image/svg+xml;base64,AAAA";

function opened() {
  const sockets: FakeSocket[] = [];
  const m = new PublicModel();
  m.start(() => {
    const s = new FakeSocket();
    sockets.push(s);
    return s;
  });
  const s = sockets[0] as FakeSocket;
  s.open();
  return { m, s };
}

describe("PublicModel", () => {
  it("PRS-32 says hello when the stream opens, so that the presenter sends everything", () => {
    const { m, s } = opened();
    expect(s.sent).toEqual([{ type: "hello" }]);
    m.stop();
  });

  it("PRS-31 shows the page the presenter names, once it has the page", () => {
    const { m, s } = opened();
    s.receive({ type: "page", n: 2, rev: 1, src: SRC, width: 960, height: 540 });
    s.receive({ type: "view", page: 2, screen: "show", laser: null, away: false });
    expect(m.page).toBe(2);
    expect(m.current?.src).toBe(SRC);
    expect(s.sent).toEqual([{ type: "hello" }]); // nothing missing: nothing asked
    m.stop();
  });

  it("asks once for a page it lacks", () => {
    const { m, s } = opened();
    s.receive({ type: "view", page: 3, screen: "show", laser: null, away: false });
    s.receive({ type: "view", page: 3, screen: "show", laser: null, away: false });
    expect(s.sent.filter((f) => f.type === "need")).toEqual([{ type: "need", n: 3 }]);
    expect(m.current).toBeUndefined();
    s.receive({ type: "page", n: 3, rev: 1, src: SRC, width: 960, height: 540 });
    expect(m.current?.width).toBe(960);
    m.stop();
  });

  it("PRS-37 follows the screen, the laser and the disconnection mention", () => {
    const { m, s } = opened();
    s.receive({ type: "view", page: 1, screen: "black", laser: { x: 0.5, y: 0.5 }, away: true });
    expect([m.screen, m.laser, m.away]).toEqual(["black", { x: 0.5, y: 0.5 }, true]);
    s.receive({ type: "roster", windows: 2 });
    expect(m.windows).toBe(2);
    m.stop();
  });

  it("PRS-31 a key in the public window is a command to the presenter, and moves nothing here", () => {
    const { m, s } = opened();
    s.receive({ type: "view", page: 2, screen: "show", laser: null, away: false });
    s.sent.length = 0;
    expect(m.key({ key: "ArrowRight" })).toBe(true);
    expect(m.key({ key: "b" })).toBe(true);
    expect(m.key({ key: "Escape" })).toBe(true);
    expect(m.key({ key: "x" })).toBe(false);
    expect(s.sent).toEqual([
      { type: "cmd", op: "next" },
      { type: "cmd", op: "black" },
      { type: "cmd", op: "quit" },
    ]);
    expect(m.page).toBe(2);
    m.stop();
  });

  it("keeps only the pages near the one shown", () => {
    const { m, s } = opened();
    for (const n of [1, 2, 3, 20])
      s.receive({ type: "page", n, rev: 1, src: SRC, width: 10, height: 10 });
    s.receive({ type: "view", page: 20, screen: "show", laser: null, away: false });
    expect([...m.pages.keys()]).toEqual([20]);
    m.stop();
  });

  it("PRS-40 a frame that is not for it is dropped without effect", () => {
    const { m, s } = opened();
    s.receive({ type: "cmd", op: "next" });
    s.receive({ type: "notes", text: "secret" });
    expect(m.page).toBe(0);
    m.stop();
  });
});
