import { flushSync } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { workspace } from "../workspace/workspace.svelte";
import ExternalBanner from "./ExternalBanner.svelte";
import ExternalChangeToast from "./ExternalChangeToast.svelte";
import HeaderBar from "./shell/HeaderBar.svelte";
import StatusBar from "./shell/StatusBar.svelte";
import { cleanup, fakeEditor, render } from "./testkit.svelte";

// The Wails runtime dials its host on import; the notices never need it.
vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {},
}));

afterEach(() => {
  workspace.notice = null;
  cleanup();
});

const show = (notice: typeof workspace.notice) => {
  workspace.notice = notice;
  flushSync();
};

const buttons = (host: HTMLElement) =>
  [...host.querySelectorAll("button")].map((b) => b.textContent?.trim());

describe("notices about outside changes", () => {
  it("EXT-01 the toast says how many lines were taken in and offers View and Undo", () => {
    const host = render(ExternalChangeToast, {});
    show({
      kind: "merged",
      id: 1,
      name: "main.typ",
      lines: 3,
      ranges: [{ from: 0, to: 4 }],
      skipped: [],
    });
    expect(host.textContent).toContain("Modifié hors d'Inkwell : 3 lignes intégrées");
    expect(buttons(host)).toEqual(expect.arrayContaining(["Voir", "Annuler"]));
    show({
      kind: "merged",
      id: 2,
      name: "main.typ",
      lines: 1,
      ranges: [{ from: 0, to: 4 }],
      skipped: [],
    });
    expect(host.textContent).toContain("1 ligne intégrée");
  });

  it("EXT-01 the toast goes away by itself, the lines stay marked", () => {
    vi.useFakeTimers();
    try {
      const host = render(ExternalChangeToast, {});
      show({
        kind: "merged",
        id: 1,
        name: "main.typ",
        lines: 1,
        ranges: [{ from: 0, to: 1 }],
        skipped: [],
      });
      expect(host.querySelector(".toast")).not.toBeNull();
      vi.advanceTimersByTime(13_000);
      flushSync();
      expect(host.querySelector(".toast")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("X-15 an edit that only touched a locked zone says so and offers nothing to undo", () => {
    const host = render(ExternalChangeToast, {});
    show({
      kind: "merged",
      id: 1,
      name: "main.typ",
      lines: 0,
      ranges: [],
      skipped: [{ from: 0, to: 3, text: "x" }],
    });
    expect(host.textContent).toContain("zone verrouillée");
    expect(buttons(host)).not.toContain("Annuler");
  });

  it("EXT-13 'ask' offers to integrate or keep Inkwell's version", () => {
    const host = render(ExternalChangeToast, {});
    show({ kind: "ask", name: "main.typ" });
    expect(host.textContent).toContain("main.typ a été modifié en dehors d'Inkwell");
    expect(buttons(host)).toEqual(["Intégrer", "Garder la version d'Inkwell"]);
  });

  it("EXT-08 a deleted file: the text stays, Recreate or Close", () => {
    const host = render(ExternalBanner, {});
    show({ kind: "deleted", name: "main.typ" });
    expect(host.textContent).toContain("supprimé du disque");
    expect(host.textContent).toContain("Son texte reste dans Inkwell");
    expect(buttons(host)).toEqual(["Recréer", "Fermer"]);
    host.querySelectorAll("button")[1]?.click();
    flushSync();
    expect(workspace.notice).toBeNull();
  });

  it("EXT-11 a file that is no longer text: Inkwell keeps its version", () => {
    const host = render(ExternalBanner, {});
    show({ kind: "binary", name: "main.typ" });
    expect(host.textContent).toContain("n'est plus un texte");
    expect(buttons(host)).toEqual(["Garder la version d'Inkwell"]);
  });

  it("EXT-10 unreadable bytes name the line", () => {
    const host = render(ExternalBanner, {});
    show({ kind: "invalid", name: "main.typ", line: 12 });
    expect(host.textContent).toContain("(ligne 12)");
  });
});

describe("unsaved indicators (SAV-02)", () => {
  it("shows « Non enregistré » in the status bar and a dot before the file name", () => {
    const editor = fakeEditor("a‸");
    const bar = render(StatusBar, {
      lang: "typst",
      status: "alone",
      peerCount: 0,
      editorState: editor.state,
      info: undefined,
      unsaved: true,
    });
    expect(bar.querySelector('[data-testid="unsaved"]')?.textContent).toBe("Non enregistré");

    const header = render(HeaderBar, {
      title: "Inkwell",
      subtitle: "main.typ",
      unsaved: true,
      status: "alone",
      me: { id: "me", name: "Moi" },
      peers: [],
      sidebarOpen: true,
      toolbarVisible: true,
      canToggleToolbar: true,
      onShare: () => {},
      onToggleSidebar: () => {},
      onToggleToolbar: () => {},
      onPreferences: () => {},
    });
    expect(header.querySelector('[data-testid="unsaved-dot"]')?.getAttribute("aria-label")).toBe(
      "Non enregistré",
    );
  });

  it("shows neither when everything is written", () => {
    const editor = fakeEditor("a‸");
    const bar = render(StatusBar, {
      lang: "typst",
      status: "alone",
      peerCount: 0,
      editorState: editor.state,
      info: undefined,
    });
    expect(bar.querySelector('[data-testid="unsaved"]')).toBeNull();
  });
});
