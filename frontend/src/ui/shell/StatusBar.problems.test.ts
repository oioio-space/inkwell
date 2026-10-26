import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { preview } from "../../preview/preview.svelte";
import { problem, result } from "../../preview/testkit";
import { cleanup, click, fakeEditor, render } from "../testkit.svelte";
import StatusBar from "./StatusBar.svelte";

afterEach(cleanup);
beforeEach(() => {
  preview.hasResult = false;
  preview.diagnostics = [];
  preview.pageCount = 0;
  preview.currentPage = 0;
  preview.truncated = false;
});

const mountBar = (props: Record<string, unknown> = {}) => {
  const editor = fakeEditor("a‸");
  return render(StatusBar, {
    lang: "typst",
    status: "alone",
    peerCount: 0,
    get editorState() {
      return editor.state;
    },
    info: undefined,
    ...props,
  });
};

const trigger = (host: HTMLElement) =>
  host.querySelector('[data-testid="problems"]')?.closest("button") as HTMLButtonElement;

describe("status bar: problems and pages", () => {
  it("PRV-90 shows nothing about problems before the first compilation", () => {
    expect(mountBar().querySelector('[data-testid="problems"]')).toBeNull();
  });

  it("PRV-90 a document without problems says so", () => {
    const host = mountBar();
    preview.receive(result());
    flushSync();
    expect(host.querySelector('[data-testid="problems"]')?.textContent).toContain("Aucun problème");
  });

  it("PRV-91 counts errors and warnings, and the button names them", () => {
    const host = mountBar();
    preview.receive(
      result({
        diagnostics: [problem(), problem({ line: 9 }), problem({ severity: "warning", line: 5 })],
      }),
    );
    flushSync();
    const t = host.querySelector('[data-testid="problems"]') as HTMLElement;
    expect([t.dataset.errors, t.dataset.warnings]).toEqual(["2", "1"]);
    expect(t.textContent?.replace(/\s+/g, "")).toBe("21");
    expect(trigger(host).getAttribute("aria-label")).toBe(
      "Problèmes du document : 2 erreurs, 1 avertissement",
    );
  });

  it("PRV-92 clicking lists the problems with their place; choosing one goes there", () => {
    const onProblem = vi.fn();
    const host = mountBar({ onProblem });
    const second = problem({
      file: "chap/deux.typ",
      line: 12,
      message: "expected a comma\nhint: ...",
    });
    preview.receive(
      result({
        diagnostics: [problem(), second, { severity: "warning", message: "sans place" }],
      }),
    );
    flushSync();
    click(trigger(host));
    const items = [...document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')];
    expect(items).toHaveLength(3);
    expect(items[0]?.textContent).toContain("unknown variable: foo");
    expect(items[0]?.textContent).toContain("main.typ, ligne 3");
    expect(items[1]?.textContent).toContain("expected a comma");
    expect(items[1]?.textContent).not.toContain("hint"); // the first line says it; the rest is detail
    expect(items[1]?.textContent).toContain("chap/deux.typ, ligne 12");
    expect(items[2]?.disabled).toBe(true); // a problem with no place cannot be jumped to

    click(items[1] as HTMLElement);
    expect(onProblem).toHaveBeenCalledWith(second);
    expect(document.querySelector('[role="menuitem"]')).toBeNull(); // the list closed
  });

  it("PRV-92 more problems than are kept: the list says so", () => {
    const host = mountBar();
    preview.receive(result({ diagnostics: [problem()], truncated: true }));
    flushSync();
    click(trigger(host));
    expect(document.body.textContent).toContain("D'autres problèmes ne sont pas affichés.");
  });

  it("PRV-93 'Page X sur Y' follows the preview, and only while it is shown", () => {
    const host = mountBar({ showPage: true });
    preview.receive(result({ pageCount: 12, pages: [] }));
    preview.currentPage = 5;
    flushSync();
    expect(host.querySelector('[data-testid="page-indicator"]')?.textContent?.trim()).toBe(
      "Page 5 sur 12",
    );
    cleanup();
    const hidden = mountBar({ showPage: false });
    expect(hidden.querySelector('[data-testid="page-indicator"]')).toBeNull();
  });
});
