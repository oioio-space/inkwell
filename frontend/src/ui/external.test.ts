import { describe, expect, it, vi } from "vitest";
import { openExternal } from "./external";

/** A click as the browser dispatches it to a handler: trusted and not yet finished. */
const userClick = (overrides: Partial<Event> = {}): Event =>
  ({ isTrusted: true, type: "click", eventPhase: Event.AT_TARGET, ...overrides }) as Event;

describe("openExternal", () => {
  it.each(["https://typst.app/docs", "http://example.org/", "mailto:someone@example.org"])(
    "opens %s on a click",
    (url) => {
      const open = vi.fn();
      expect(openExternal(userClick(), url, open)).toBe(true);
      expect(open).toHaveBeenCalledExactlyOnceWith(new URL(url).href);
    },
  );

  it.each([
    "file:///etc/passwd",
    "javascript:alert(1)",
    "data:text/html,x",
    "smb://host/share",
    "ssh://host",
    "vscode://file/x",
    "/relative/path",
    "not a url",
  ])("refuses %s even on a click", (url) => {
    const open = vi.fn();
    expect(openExternal(userClick(), url, open)).toBe(false);
    expect(open).not.toHaveBeenCalled();
  });

  it.each([
    ["a script-made click", { isTrusted: false }],
    ["a gesture other than a click", { type: "mouseover" }],
    ["a click that has finished (replayed later)", { eventPhase: Event.NONE }],
  ])("refuses %s", (_, overrides) => {
    const open = vi.fn();
    expect(openExternal(userClick(overrides), "https://typst.app", open)).toBe(false);
    expect(open).not.toHaveBeenCalled();
  });

  it("refuses an event dispatched by a script", () => {
    const open = vi.fn();
    const target = new EventTarget();
    let opened: boolean | undefined;
    target.addEventListener("click", (e) => {
      opened = openExternal(e, "https://typst.app", open);
    });
    target.dispatchEvent(new Event("click"));
    expect(opened).toBe(false);
    expect(open).not.toHaveBeenCalled();
  });
});

const sources = import.meta.glob(["../**/*.svelte", "../**/*.ts", "!../**/*.test.ts"], {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

describe("platform actions", () => {
  // Opening a link, a file or the file manager goes through openExternal only.
  const opening = [/\bBrowser\b/, /\bOpenURL\b/, /\bOpenFile(Manager)?\b/, /\bwindow\.open\s*\(/];
  // Document or peer content is never injected as markup, where a link would become live. The
  // icons render constant markup (ui/icons/paths.ts), never built from input.
  const markup = [/\{@html\b/, /\.(inner|outer)HTML\b/, /\binsertAdjacentHTML\b/];
  const constantMarkup = new Set([
    "./icons/FileIcon.svelte",
    "./icons/Icon.svelte",
    "./icons/paths.ts",
  ]);

  it("are reached only through openExternal", () => {
    const found = Object.entries(sources).flatMap(([path, source]) => {
      const rules = [
        ...(path === "./external.ts" ? [] : opening),
        ...(constantMarkup.has(path) ? [] : markup),
      ];
      return rules.filter((re) => re.test(source)).map((re) => `${path}: ${re.source}`);
    });
    expect(found).toEqual([]);
  });

  it("scan the whole frontend", () => {
    for (const path of ["./external.ts", "../App.svelte", ...constantMarkup]) {
      expect(Object.keys(sources)).toContain(path);
    }
  });
});
