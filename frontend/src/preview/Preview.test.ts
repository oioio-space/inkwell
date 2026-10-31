import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, click, render } from "../ui/testkit.svelte";
import Preview from "./Preview.svelte";
import { preview } from "./preview.svelte";
import { result as makeResult, problem, svgPage } from "./testkit";

// The singleton survives tests; frames must keep increasing across fixtures too.
let sequence = 0;
const result = (over: Parameters<typeof makeResult>[0] = {}) =>
  makeResult({ ...over, seq: ++sequence });

// happy-dom lays nothing out: give the scroller the size of a real pane.
function fakeSize(width: number, height: number): () => void {
  const w = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "clientWidth");
  const h = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "clientHeight");
  Object.defineProperty(HTMLElement.prototype, "clientWidth", {
    configurable: true,
    get: () => width,
  });
  Object.defineProperty(HTMLElement.prototype, "clientHeight", {
    configurable: true,
    get: () => height,
  });
  return () => {
    if (w) Object.defineProperty(HTMLElement.prototype, "clientWidth", w);
    if (h) Object.defineProperty(HTMLElement.prototype, "clientHeight", h);
  };
}

let restore: () => void;
beforeEach(() => {
  restore = fakeSize(900, 700);
  // The model is a singleton: start every test from an empty one.
  preview.receive({ type: "compiling", seq: 0 });
  preview.pages = new Map();
  preview.pageCount = 0;
  preview.hasResult = false;
  preview.failure = undefined;
  preview.diagnostics = [];
  preview.marker = undefined;
});
afterEach(() => {
  cleanup();
  restore();
  vi.restoreAllMocks();
});

const mount = (over: Record<string, unknown> = {}) => {
  const onSourceJump = vi.fn();
  const onCursorJump = vi.fn();
  const host = render(Preview, { onSourceJump, onCursorJump, ...over });
  return { host, onSourceJump, onCursorJump };
};

const pageEls = (host: HTMLElement) => [...host.querySelectorAll<HTMLElement>(".page")];

describe("Preview pane", () => {
  it("PRV-80 says it is getting ready until Go has answered", () => {
    const { host } = mount();
    expect(host.querySelector('[data-testid="preview-empty"]')?.textContent).toContain(
      "Préparation de l'aperçu",
    );
    expect(host.querySelector('[data-testid="pages"]')).toBeNull();
  });

  it("PRV-81 draws the pages as images with a name, and nothing runs in them", () => {
    const { host } = mount();
    preview.receive(result());
    flushSync();
    const imgs = [...host.querySelectorAll<HTMLImageElement>(".page img")];
    expect(imgs.length).toBeGreaterThan(0);
    expect(imgs[0]?.src.startsWith("data:image/svg+xml;base64,")).toBe(true);
    expect(imgs[0]?.alt).toBe("Page 1 sur 3");
    expect(host.querySelector(".page svg, .page script, .page object")).toBeNull();
  });

  it("PRV-82 only the pages near the view are in the page, long documents included", () => {
    const { host } = mount();
    preview.receive(result({ pageCount: 400, pages: [svgPage(), svgPage()] }));
    flushSync();
    const numbers = pageEls(host).map((p) => Number(p.dataset.page));
    expect(numbers[0]).toBe(1);
    expect(numbers.length).toBeLessThanOrEqual(3); // 700 px of view, pages of about 450 px at "fit"
    expect(host.querySelector<HTMLElement>(".stack")?.style.height).not.toBe("");
  });

  it("PRV-82 a page not here yet shows a placeholder of the right size, not a hole", () => {
    const { host } = mount();
    preview.receive(result({ pageCount: 3, pages: [svgPage()] }));
    flushSync();
    const second = pageEls(host).find((p) => p.dataset.page === "2");
    expect(second?.querySelector("img")).toBeNull();
    expect(second?.querySelector(".skeleton")).not.toBeNull();
    expect(second?.style.height).toBe(pageEls(host)[0]?.style.height);
  });

  it("PRV-83 a click on a page says which page and where, in points", () => {
    const { host, onSourceJump } = mount();
    preview.receive(result());
    flushSync();
    const button = host.querySelector<HTMLElement>(".page .pagebtn") as HTMLElement;
    const k = 4 / 3; // 100 % : 96 px per 72 pt
    vi.spyOn(button, "getBoundingClientRect").mockReturnValue({
      left: 10,
      top: 20,
      width: 595.28 * k,
      height: 841.89 * k,
      right: 0,
      bottom: 0,
      x: 10,
      y: 20,
      toJSON: () => ({}),
    });
    button.dispatchEvent(
      new MouseEvent("click", {
        bubbles: true,
        detail: 1,
        clientX: 10 + 100 * k,
        clientY: 20 + 150 * k,
      }),
    );
    expect(onSourceJump).toHaveBeenCalledOnce();
    const [page, x, y] = onSourceJump.mock.calls[0] as number[];
    expect(page).toBe(1);
    expect(x).toBeCloseTo(100);
    expect(y).toBeCloseTo(150);
  });

  it("PRV-83 Enter on a page, with no pointer, goes to its top", () => {
    const { host, onSourceJump } = mount();
    preview.receive(result());
    flushSync();
    (host.querySelector(".page .pagebtn") as HTMLElement).dispatchEvent(
      new MouseEvent("click", { bubbles: true, detail: 0 }),
    );
    expect(onSourceJump).toHaveBeenCalledWith(1, 0, 0);
  });

  it("PRV-84 zoom: fits the width at first, then steps, then fits again", () => {
    const { host } = mount();
    preview.receive(result());
    flushSync();
    const percent = () => host.querySelector('[data-testid="zoom"]')?.textContent;
    const fit = percent();
    expect(fit).toMatch(/^\d+ %$/);
    click(host.querySelector('button[aria-label="Agrandir"]') as HTMLElement);
    const bigger = Number.parseInt(percent() ?? "0", 10);
    expect(bigger).toBeGreaterThan(Number.parseInt(fit ?? "0", 10));
    click(host.querySelector('button[aria-label="Réduire"]') as HTMLElement);
    click(host.querySelector('button[aria-label="Réduire"]') as HTMLElement);
    expect(Number.parseInt(percent() ?? "0", 10)).toBeLessThan(bigger);
    click(host.querySelector('button[aria-label="Ajuster à la largeur"]') as HTMLElement);
    expect(percent()).toBe(fit);
  });

  it("PRV-85 a missing package is explained: which one, and that nothing is downloaded", () => {
    const { host } = mount();
    preview.receive({
      ...result({
        pageCount: 0,
        pages: [],
        diagnostics: [problem({ package: "@preview/zz:1.0.0" })],
      }),
      failure: { kind: "package", packages: ["@preview/zz:1.0.0"] },
    });
    flushSync();
    const empty = host.querySelector('[data-testid="preview-empty"]');
    expect(empty?.getAttribute("aria-live")).toBe("polite");
    expect(empty?.textContent).toContain("@preview/zz:1.0.0");
    expect(empty?.textContent).toContain("ne télécharge jamais");
  });

  it("PRV-85 every failure has its own sentence", () => {
    const { host } = mount();
    const seen = new Set<string>();
    for (const kind of [
      "errors",
      "noMain",
      "timeout",
      "memory",
      "tooLarge",
      "unsafe",
      "unavailable",
      "internal",
    ] as const) {
      preview.receive({ ...result({ pageCount: 0, pages: [] }), failure: { kind, packages: [] } });
      flushSync();
      const text = host.querySelector('[data-testid="preview-empty"]')?.textContent?.trim() ?? "";
      expect(text, kind).not.toBe("");
      expect(text, kind).not.toContain("preview.fail"); // a key shown raw would mean a missing message
      seen.add(text);
    }
    expect(seen.size).toBe(8);
  });

  it("PRV-86 with errors but an earlier version, the old pages stay and a notice says so", () => {
    const { host } = mount();
    preview.receive(result());
    preview.receive({
      ...result({ seq: 2, rev: 2, pageCount: 0, pages: [] }),
      failure: { kind: "errors", packages: [] },
    });
    flushSync();
    expect(host.querySelector(".page img")).not.toBeNull();
    expect(host.querySelector(".notice")?.textContent).toContain(
      "dernière version qui fonctionnait",
    );
  });

  it("PRV-86 with an old version shown and a package missing, the notice names the package", () => {
    const { host } = mount();
    preview.receive(result());
    preview.receive({
      ...result({ seq: 2, rev: 2, pageCount: 0, pages: [] }),
      failure: { kind: "package", packages: ["@preview/zz:1.0.0"] },
    });
    flushSync();
    expect(host.querySelector(".page img")).not.toBeNull();
    expect(host.querySelector(".notice")?.textContent).toContain("@preview/zz:1.0.0");
  });

  it("PRV-87 'go to the cursor' needs a document, then asks the shell", () => {
    const { host, onCursorJump } = mount();
    const button = host.querySelector(
      'button[aria-label="Aller à l\'endroit du curseur"]',
    ) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    preview.receive(result());
    flushSync();
    click(button);
    expect(onCursorJump).toHaveBeenCalledOnce();
  });

  it("PRV-88 a jump from the source marks the place on its page", () => {
    const { host } = mount();
    preview.receive(result());
    flushSync();
    const scrollTo = vi.fn();
    (host.querySelector('[data-testid="pages"]') as HTMLElement).scrollTo = scrollTo;
    preview.mark(1, 100);
    flushSync();
    expect(host.querySelector(".page[data-page='1'] .mark")).not.toBeNull();
    expect(scrollTo).toHaveBeenCalled();
  });

  it("PRV-89 the current page follows the scroll", () => {
    const { host } = mount();
    preview.receive(result({ pageCount: 20, pages: [svgPage(), svgPage(), svgPage()] }));
    flushSync();
    expect(preview.currentPage).toBe(1);
    const scroller = host.querySelector('[data-testid="pages"]') as HTMLElement;
    scroller.scrollTop = 2400;
    scroller.dispatchEvent(new Event("scroll"));
    flushSync();
    expect(preview.currentPage).toBe(3); // pages are about 1230 px high at "fit": 2400 px is deep in page 3
  });

  it("PRV-89 shows that it is updating", () => {
    const { host } = mount();
    preview.receive(result());
    preview.receive({ type: "compiling", seq: 2 });
    flushSync();
    expect(host.querySelector(".busy")?.textContent).toContain("Mise à jour");
  });
});
