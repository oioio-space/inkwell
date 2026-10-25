import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { settings } from "../../store/settings.svelte";
import Banner from "../Banner.svelte";
import Toast from "../Toast.svelte";
import { cleanup, click, render } from "../testkit.svelte";
import Preferences from "./Preferences.svelte";

// happy-dom has no modal dialog: give it the two methods the component calls.
beforeEach(() => {
  localStorage.clear();
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
    this.dispatchEvent(new Event("close"));
  };
  settings.setSaveMode("auto");
  settings.setSaveDelay(2);
  settings.setExternalPolicy("ask");
});
afterEach(cleanup);

const stored = () => JSON.parse(localStorage.getItem("inkwell.settings") ?? "{}");

describe("preferences", () => {
  it("stores the save mode, the delay and what to do with outside changes", () => {
    const host = render(Preferences, { open: true });
    flushSync();
    expect(host.querySelector("dialog")?.hasAttribute("open")).toBe(true);

    const modes = [...host.querySelectorAll<HTMLElement>('[role="radio"]')];
    expect(modes.map((m) => m.textContent?.trim())).toEqual(["Automatiquement", "Avec Ctrl+S"]);
    expect(modes.map((m) => m.getAttribute("aria-checked"))).toEqual(["true", "false"]);

    click(modes[1] as HTMLElement);
    expect(settings.saveMode).toBe("manual");
    expect(stored().saveMode).toBe("manual");
    expect(modes[1]?.getAttribute("aria-checked")).toBe("true");

    const [delay, external] = [...host.querySelectorAll<HTMLSelectElement>("select")] as [
      HTMLSelectElement,
      HTMLSelectElement,
    ];
    // The pause only matters when saving automatically.
    expect(delay.disabled).toBe(true);
    click(modes[0] as HTMLElement);
    expect(delay.disabled).toBe(false);

    delay.value = "5";
    delay.dispatchEvent(new Event("change", { bubbles: true }));
    expect(settings.saveDelay).toBe(5);
    expect([...delay.options].map((o) => o.textContent)).toEqual([
      "1 seconde",
      "2 secondes",
      "5 secondes",
      "10 secondes",
    ]);

    external.value = "keep";
    external.dispatchEvent(new Event("change", { bubbles: true }));
    expect(settings.externalPolicy).toBe("keep");
    expect(stored()).toEqual({ saveMode: "auto", saveDelay: 5, externalPolicy: "keep" });
  });

  it("closes from its button", () => {
    const host = render(Preferences, { open: true });
    flushSync();
    click(host.querySelector('button[aria-label^="Fermer"]') as HTMLElement);
    expect(host.querySelector("dialog")?.hasAttribute("open")).toBe(false);
  });
});

describe("settings", () => {
  it("remembers the toolbar and the sidebar per project, and survives a blocked storage", () => {
    expect(settings.toolbarVisible("p1")).toBe(true);
    settings.setToolbarVisible("p1", false);
    expect(settings.toolbarVisible("p1")).toBe(false);
    expect(settings.toolbarVisible("p2")).toBe(true);
    expect(JSON.parse(localStorage.getItem("inkwell.layout") ?? "{}").p1.toolbarHidden).toBe(true);
    settings.setSidebarVisible("p1", false);
    expect(settings.sidebarVisible("p1")).toBe(false);
    expect(settings.toolbarVisible("p1")).toBe(false);

    const real = Storage.prototype.setItem;
    Storage.prototype.setItem = () => {
      throw new Error("blocked");
    };
    try {
      expect(() => settings.setToolbarVisible("p3", false)).not.toThrow();
      expect(settings.toolbarVisible("p3")).toBe(false);
    } finally {
      Storage.prototype.setItem = real;
    }
  });
});

describe("toast and banner", () => {
  it("a toast speaks with its role, offers its actions and closes", () => {
    let closed = false;
    const host = render(Toast, {
      role: "alert",
      onclose: () => {
        closed = true;
      },
      children: textSnippet("Modifié hors d'Inkwell"),
    });
    expect(host.querySelector('[role="alert"]')?.textContent).toContain("Modifié hors d'Inkwell");
    click(host.querySelector('button[aria-label^="Masquer"]') as HTMLElement);
    expect(closed).toBe(true);
  });

  it("a banner carries its tone", () => {
    const host = render(Banner, {
      tone: "warning",
      role: "alert",
      children: textSnippet("Attention"),
    });
    const banner = host.querySelector('[role="alert"]');
    expect(banner?.className).toContain("warning");
    expect(banner?.textContent).toContain("Attention");
  });
});

/** A snippet that renders a text node, for components that take their content as `children`. */
function textSnippet(text: string) {
  return ((anchor: Comment) => {
    anchor.before(document.createTextNode(text));
  }) as never;
}
