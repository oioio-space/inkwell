import { afterEach, describe, expect, it, vi } from "vitest";
import { get } from "../../filetypes";
import { cleanup, click, render } from "../testkit.svelte";
import BlobView from "./BlobView.svelte";

afterEach(cleanup);

describe("a file that is not text", () => {
  it("is shown as « Fichier non textuel » with its size and type, never as characters", () => {
    const host = render(BlobView, { name: "plan.pdf", size: 3 * 1024 * 1024, type: get("pdf") });
    expect(host.textContent).toContain("Fichier non textuel");
    expect(host.textContent).toContain("PDF · 3 Mo");
    expect(host.querySelector("svg")?.getAttribute("aria-label")).toBe("PDF");
    expect(host.querySelector(".cm-content")).toBeNull();
  });

  it("offers to open it with the system's application", () => {
    const open = vi.fn();
    const host = render(BlobView, {
      name: "font.ttf",
      size: 100,
      type: get("font"),
      openWithSystem: open,
    });
    const button = host.querySelector("button") as HTMLButtonElement;
    expect(button.textContent?.trim()).toBe("Ouvrir avec l'application du système");
    click(button);
    expect(open).toHaveBeenCalledOnce();
  });

  it.each([
    "setup.exe",
    "run.bat",
    "x.lnk",
    "a.hta",
    "b.scr",
    "c.msi",
    "d.jar",
    "e.docm",
    "f.sh",
    "noext",
  ])("never offers to run %s: it is shown in the folder instead", (name) => {
    const open = vi.fn();
    const host = render(BlobView, { name, size: 1, type: get("binary"), openWithSystem: open });
    const button = host.querySelector("button") as HTMLButtonElement;
    expect(button.textContent).not.toContain("Ouvrir");
    expect(button.textContent?.trim()).toBe("Afficher dans le dossier");
    click(button);
    expect(open).toHaveBeenCalledOnce();
  });
});
