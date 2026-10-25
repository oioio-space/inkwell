import { afterEach, describe, expect, it, vi } from "vitest";
import { FileKind, type ProjectFile } from "../../store/files";
import { cleanup, click, render } from "../testkit.svelte";
import Sidebar from "./Sidebar.svelte";
import { buildTree, visibleNodes } from "./tree";

const file = (path: string, kind: FileKind, main = false): ProjectFile => ({
  path,
  kind,
  size: 1,
  main,
});

const project: ProjectFile[] = [
  file("main.typ", FileKind.typst, true),
  file("chapitres/10-fin.typ", FileKind.typst),
  file("chapitres/2-milieu.typ", FileKind.typst),
  file("figures/architecture.svg", FileKind.image),
  file("notes.md", FileKind.markdown),
  file("references.bib", FileKind.bib),
  file("inventaire.csv", FileKind.csv),
  file("lisez-moi.txt", FileKind.text),
  file("plan.pdf", FileKind.pdf),
  file("config.json", FileKind.data),
  file("blob.bin", FileKind.other),
];

afterEach(cleanup);

describe("file tree", () => {
  it("UI-08 puts folders first and sorts naturally", () => {
    const tree = buildTree(project);
    expect(tree.map((n) => `${n.type}:${n.name}`)).toEqual([
      "folder:chapitres",
      "folder:figures",
      "file:blob.bin",
      "file:config.json",
      "file:inventaire.csv",
      "file:lisez-moi.txt",
      "file:main.typ",
      "file:notes.md",
      "file:plan.pdf",
      "file:references.bib",
    ]);
    const chapters = tree[0];
    expect(chapters?.type === "folder" && chapters.children.map((c) => c.name)).toEqual([
      "2-milieu.typ",
      "10-fin.typ",
    ]);
    expect(visibleNodes(tree, new Set(["chapitres"])).some((n) => n.name === "2-milieu.typ")).toBe(
      false,
    );
  });
});

describe("sidebar", () => {
  it("UI-08 draws one labelled icon per kind and marks the main file", () => {
    const host = render(Sidebar, { files: project, current: "main.typ", onOpen: () => {} });
    const kinds = (path: string) =>
      host.querySelector(`[data-path="${path}"] svg`)?.getAttribute("aria-label");
    expect(kinds("main.typ")).toBe("Typst");
    expect(kinds("notes.md")).toBe("Markdown");
    expect(kinds("lisez-moi.txt")).toBe("Texte");
    expect(kinds("inventaire.csv")).toBe("Tableau CSV");
    expect(kinds("references.bib")).toBe("Bibliographie");
    expect(kinds("figures/architecture.svg")).toBe("Image");
    expect(kinds("plan.pdf")).toBe("PDF");
    expect(kinds("config.json")).toBe("Données");
    expect(kinds("blob.bin")).toBe("Fichier");
    for (const icon of host.querySelectorAll("svg[role='img']")) {
      expect(icon.getAttribute("aria-label")).toBeTruthy();
    }
    expect(host.querySelector('[data-path="main.typ"]')?.textContent).toContain("principal");
    expect(host.querySelector('[data-path="main.typ"]')?.getAttribute("aria-current")).toBe("page");
    expect(host.querySelector('[data-path="notes.md"]')?.hasAttribute("aria-current")).toBe(false);
  });

  it("UI-08 a folder folds and unfolds", () => {
    const host = render(Sidebar, { files: project, current: "main.typ", onOpen: () => {} });
    const folder = [...host.querySelectorAll<HTMLButtonElement>("button[aria-expanded]")].find(
      (b) => b.textContent?.includes("chapitres"),
    ) as HTMLButtonElement;
    expect(folder.getAttribute("aria-expanded")).toBe("true");
    expect(host.querySelector('[data-path="chapitres/2-milieu.typ"]')).not.toBeNull();
    expect(folder.querySelector("svg[role='img']")?.getAttribute("aria-label")).toBe("Dossier");

    click(folder);
    expect(folder.getAttribute("aria-expanded")).toBe("false");
    expect(host.querySelector('[data-path="chapitres/2-milieu.typ"]')).toBeNull();

    click(folder);
    expect(host.querySelector('[data-path="chapitres/2-milieu.typ"]')).not.toBeNull();
  });

  it("UI-08 opening a file reports its path", () => {
    const onOpen = vi.fn();
    const host = render(Sidebar, { files: project, current: "main.typ", onOpen });
    click(host.querySelector('[data-path="notes.md"]') as HTMLElement);
    expect(onOpen).toHaveBeenCalledWith("notes.md");
  });
});
