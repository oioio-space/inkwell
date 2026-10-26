import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {},
}));

beforeEach(() => {
  localStorage.clear();
  vi.resetModules();
});
afterEach(() => {
  document.documentElement.style.removeProperty("--text-zoom");
});

describe("text zoom (ZOOM-01 to ZOOM-03)", () => {
  it("ZOOM-01 steps of 10 between 50 and 300, back to 100", async () => {
    const { zoom, ZOOM_MAX, ZOOM_MIN } = await import("./zoom.svelte");
    expect(zoom.level("p")).toBe(100);
    zoom.in("p");
    expect(zoom.level("p")).toBe(110);
    for (let i = 0; i < 40; i++) zoom.in("p");
    expect(zoom.level("p")).toBe(ZOOM_MAX);
    for (let i = 0; i < 60; i++) zoom.out("p");
    expect(zoom.level("p")).toBe(ZOOM_MIN);
    zoom.reset("p");
    expect(zoom.level("p")).toBe(100);
    zoom.set("p", 9999);
    expect(zoom.level("p")).toBe(300);
    zoom.set("p", Number.NaN);
    expect(zoom.level("p")).toBe(100);
  });

  it("ZOOM-02 reopening: the zoom is found again, per project, and stays on this device", async () => {
    const first = await import("./zoom.svelte");
    first.zoom.set("a", 150);
    first.zoom.set("b", 80);
    vi.resetModules();
    const again = await import("./zoom.svelte");
    expect(again.zoom.level("a")).toBe(150);
    expect(again.zoom.level("b")).toBe(80);
    expect(again.zoom.level("c")).toBe(100);
    // Only localStorage holds it: nothing in the document, nothing sent.
    expect(Object.keys(localStorage)).toEqual(["inkwell.zoom"]);
  });

  it("ZOOM-02 a broken saved value is ignored", async () => {
    localStorage.setItem("inkwell.zoom", "{not json");
    const { zoom } = await import("./zoom.svelte");
    expect(zoom.level("a")).toBe(100);
  });
});
