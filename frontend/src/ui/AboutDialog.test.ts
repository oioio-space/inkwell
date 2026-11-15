import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AboutDialog from "./AboutDialog.svelte";
import { cleanup, click, render } from "./testkit.svelte";

// The tier Go reports (ADR 0044 point 7), swapped per test.
const networkMode = vi.hoisted(() => vi.fn<() => Promise<string>>());
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  PreviewService: { NetworkMode: networkMode },
}));

// happy-dom has no modal dialog: give it the two methods the component calls.
beforeEach(() => {
  networkMode.mockResolvedValue("namespace");
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
    this.dispatchEvent(new Event("close"));
  };
});
afterEach(cleanup);

const info = { Version: "0.1.0-rc.1", Commit: "0123456789abcdef", Date: "2026-10-26T08:00:00Z" };

describe("about dialog", () => {
  it("PKG-07 shows the version Go reports, the short commit and the day", () => {
    const host = render(AboutDialog, { open: true, info });
    flushSync();
    const dialog = host.querySelector("dialog");
    expect(dialog?.hasAttribute("open")).toBe(true);
    expect(dialog?.getAttribute("aria-labelledby")).toBe("about-title");
    const text = dialog?.textContent ?? "";
    expect(text).toContain("Inkwell");
    expect(text).toContain("Version 0.1.0-rc.1");
    expect(text).toContain("Révision 0123456, 2026-10-26");
    expect(text).toContain("MPL-2.0");
  });

  it("says only what it knows while the version is not loaded or has no commit", () => {
    const host = render(AboutDialog, { open: true });
    flushSync();
    expect(host.textContent).not.toContain("Version ");
    cleanup();

    const dev = render(AboutDialog, {
      open: true,
      info: { Version: "0.0.0-dev", Commit: "", Date: "" },
    });
    flushSync();
    expect(dev.textContent).toContain("Version 0.0.0-dev");
    expect(dev.textContent).not.toContain("Révision");
  });

  it("closes with its button and tells its owner", () => {
    const host = render(AboutDialog, { open: true, info });
    flushSync();
    click(host.querySelector<HTMLElement>("button.close") as HTMLElement);
    expect(host.querySelector("dialog")?.hasAttribute("open")).toBe(false);
  });
});

describe("about dialog, isolation line", () => {
  const line = async (mode: string) => {
    networkMode.mockResolvedValue(mode);
    const host = render(AboutDialog, { open: true, info });
    await vi.waitFor(() =>
      expect(host.querySelector('[data-testid="network-mode"]')?.getAttribute("data-mode")).toBe(
        mode,
      ),
    );
    return host.querySelector('[data-testid="network-mode"]')?.textContent ?? "";
  };

  it("SB2a1 about shows the tier: namespaces", async () => {
    expect(await line("namespace")).toBe(
      "La mise en page tourne dans un espace isolé : elle ne peut joindre aucun réseau, pas même cet ordinateur.",
    );
  });

  it("SB2a1 about shows the tier: reduced", async () => {
    const text = await line("reduced");
    expect(text).toBe(
      "Vos fichiers restent protégés : le moteur de mise en page ne peut ni lire vos autres fichiers ni utiliser le réseau. Ce système ne permet pas l'isolation complète : le moteur n'est pas entièrement séparé des autres programmes ouverts.",
    );
    // UX-SB2a1-5: two short sentences, not a wall.
    expect(text.split(". ")).toHaveLength(2);
    expect(text).not.toMatch(/proxy|seccomp|Landlock|sandbox/i);
  });

  it("SB2a1 about explains a refused sandbox and what to do", async () => {
    const text = await line("refused");
    expect(text).toBe(
      "L'aperçu est désactivé sur cet ordinateur : ce système ne permet pas d'isoler le moteur de mise en page, et Inkwell ne le lance jamais sans cette protection. Le texte et la collaboration fonctionnent normalement ; pour voir l'aperçu, ouvrez le document sur un autre ordinateur, ou mettez à jour le système et Inkwell.",
    );
    expect(text).not.toMatch(/non disponible|proxy|seccomp|Landlock|sandbox/i);
  });

  it("SB2a1 about says a start that did not succeed yet can be tried again", async () => {
    expect(await line("pending")).toBe(
      "L'aperçu n'a pas pu démarrer cette fois : Inkwell n'a pas pu vérifier l'isolation du moteur de mise en page. Choisissez « Réessayer » dans l'aperçu.",
    );
  });

  it("SB2a1 about gives the focus back to the button that opened it", () => {
    const opener = document.createElement("button");
    document.body.append(opener);
    const host = render(AboutDialog, { open: true, info, opener });
    flushSync();
    click(host.querySelector<HTMLElement>("button.close") as HTMLElement);
    expect(document.activeElement).toBe(opener);
  });
});
