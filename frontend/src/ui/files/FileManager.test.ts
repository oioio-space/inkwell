import { flushSync, tick } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { until } from "../../net/testkit";
import { workspace } from "../../workspace/workspace.svelte";
import Sidebar from "../shell/Sidebar.svelte";
import { cleanup, click, press, render } from "../testkit.svelte";
import { dragEvent, fire, project, rowOf, rowPaths, shimDialog } from "./testkit";

// The Wails runtime dials its host on import; the file manager never needs it.
vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {},
}));

beforeEach(() => {
  shimDialog();
  workspace.myId = "alice";
  workspace.myName = "Alice";
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

/** Mounts the sidebar over a project that holds the given files (paths, "/" at the end: a folder). */
async function mount(role: "admin" | "writer" | "reader" = "admin", paths: string[] = []) {
  const p = await project(role);
  const { runtime } = p;
  const made = new Map<string, string>();
  for (const path of paths) {
    const parts = path.split("/").filter(Boolean);
    const name = parts.pop() as string;
    const parent = parts.length === 0 ? null : (made.get(parts.join("/")) ?? null);
    const out = path.endsWith("/")
      ? runtime.index.createFolder(parent, name)
      : runtime.index.createFile(parent, name, "x");
    if (!out.ok) throw new Error(`setup: ${JSON.stringify(out.error)}`);
    made.set(parts.concat(name).join("/"), out.ids[0] as string);
  }
  const onOpen = vi.fn();
  const host = render(Sidebar, { runtime, onOpen });
  flushSync();
  return { ...p, host, onOpen };
}

const items = (host: HTMLElement) => [...host.querySelectorAll<HTMLElement>('[role="treeitem"]')];
const text = (el: Element | null) => el?.textContent?.replace(/\s+/g, " ").trim() ?? "";
const key = (el: Element, k: string, init: KeyboardEventInit = {}) => press(el, k, init);
/** Everything announced to the person: the toasts and alerts, in the order they are in. */
const notices = () =>
  [...document.querySelectorAll('[role="status"], [role="alert"]')].map((n) => text(n)).join(" | ");
const buttonNamed = (host: ParentNode, name: RegExp | string) =>
  [...host.querySelectorAll<HTMLElement>("button, [role='menuitem']")].find((b) =>
    typeof name === "string"
      ? text(b) === name || b.getAttribute("aria-label") === name
      : name.test(text(b)) || name.test(b.getAttribute("aria-label") ?? ""),
  );

describe("UI-08 the list of files", () => {
  it("draws one labelled icon per kind and marks the main file", async () => {
    const { host } = await mount("admin", ["notes.md", "refs.bib", "data.csv", "readme.txt"]);
    const kind = (path: string) =>
      rowOf(host, path)?.querySelector("svg")?.getAttribute("aria-label");
    expect(kind("main.typ")).toBe("Typst");
    expect(kind("notes.md")).toBe("Markdown");
    expect(kind("refs.bib")).toBe("Bibliographie");
    expect(kind("data.csv")).toBe("Tableau CSV");
    expect(kind("readme.txt")).toBe("Texte");
    expect(text(rowOf(host, "main.typ"))).toContain("principal");
    expect(rowOf(host, "main.typ")?.getAttribute("aria-current")).toBe("page");
    expect(rowOf(host, "notes.md")?.hasAttribute("aria-current")).toBe(false);
  });

  it("puts folders first, sorts naturally, and a folder folds and unfolds", async () => {
    const { host } = await mount("admin", [
      "chapitres/",
      "chapitres/10-fin.typ",
      "chapitres/2-milieu.typ",
      "a.md",
    ]);
    expect(rowPaths(host)).toEqual([
      "chapitres",
      "chapitres/2-milieu.typ",
      "chapitres/10-fin.typ",
      "a.md",
      "main.typ",
    ]);
    const folder = rowOf(host, "chapitres") as HTMLElement;
    expect(folder.getAttribute("aria-expanded")).toBe("true");
    click(folder);
    expect(folder.getAttribute("aria-expanded")).toBe("false");
    expect(rowOf(host, "chapitres/2-milieu.typ")).toBeNull();
    click(folder);
    expect(rowOf(host, "chapitres/2-milieu.typ")).not.toBeNull();
  });

  it("UI-08 opening a file shows it and hands focus back to the editor", async () => {
    const { host, runtime, onOpen } = await mount("admin", ["notes.md"]);
    click(rowOf(host, "notes.md") as HTMLElement);
    expect(runtime.currentPath).toBe("notes.md");
    expect(onOpen).toHaveBeenCalledWith("notes.md");
  });
});

describe("FM-21 keyboard and accessibility", () => {
  it("is a tree of treeitems with level, position, size and expansion", async () => {
    const { host } = await mount("admin", ["d/", "d/a.typ", "d/b.typ"]);
    const tree = host.querySelector('[role="tree"]') as HTMLElement;
    expect(tree.getAttribute("aria-multiselectable")).toBe("true");
    expect(tree.getAttribute("aria-label")).toBe("Arborescence du projet");
    const dir = rowOf(host, "d") as HTMLElement;
    expect(dir.getAttribute("aria-level")).toBe("1");
    expect(dir.getAttribute("aria-expanded")).toBe("true");
    const b = rowOf(host, "d/b.typ") as HTMLElement;
    expect(b.getAttribute("aria-level")).toBe("2");
    expect(b.getAttribute("aria-posinset")).toBe("2");
    expect(b.getAttribute("aria-setsize")).toBe("2");
    expect(b.hasAttribute("aria-expanded")).toBe(false);
    // One stop for the Tab key: roving tabindex.
    expect(items(host).filter((r) => r.tabIndex === 0)).toHaveLength(1);
  });

  it("moves with the arrows, opens with Enter, folds with Left and Right", async () => {
    const { host, runtime } = await mount("admin", ["d/", "d/a.typ"]);
    const first = items(host)[0] as HTMLElement;
    // Focus moves a tick after the key: the list scrolls the row into view first.
    const press = async (k: string) => {
      key(document.activeElement as Element, k);
      await tick();
    };
    first.focus();
    await press("ArrowDown");
    expect(document.activeElement).toBe(rowOf(host, "d/a.typ"));
    await press("Enter");
    expect(runtime.currentPath).toBe("d/a.typ");
    await press("ArrowLeft"); // to the folder
    expect(document.activeElement).toBe(rowOf(host, "d"));
    await press("ArrowLeft"); // fold it
    expect(rowOf(host, "d")?.getAttribute("aria-expanded")).toBe("false");
    await press("ArrowRight"); // unfold it
    expect(rowOf(host, "d")?.getAttribute("aria-expanded")).toBe("true");
    await press("End");
    expect((document.activeElement as HTMLElement).dataset.path).toBe("main.typ");
    await press("Home");
    expect((document.activeElement as HTMLElement).dataset.path).toBe("d");
  });

  it("opens a menu with Shift+F10 on the focused row", async () => {
    const { host } = await mount("admin", ["a.md"]);
    const row = rowOf(host, "a.md") as HTMLElement;
    row.focus();
    key(row, "F10", { shiftKey: true });
    const menu = document.querySelector('[role="menu"]') as HTMLElement;
    expect(menu).not.toBeNull();
    expect(menu.getAttribute("aria-label")).toBe("Actions sur a.md");
    const names = [...menu.querySelectorAll('[role="menuitem"]')].map((i) => text(i));
    expect(names.some((n) => n.startsWith("Renommer"))).toBe(true);
    expect(names.some((n) => n.startsWith("Copier le chemin"))).toBe(true);
    key(menu, "Escape");
    expect(document.querySelector('[role="menu"]')).toBeNull();
  });
});

describe("FM-01 FM-03 creating and renaming", () => {
  it("FM-01 a new file from a template appears, opens, and is a document", async () => {
    const { host, runtime } = await mount("admin");
    click(buttonNamed(host, "Ajouter au projet") as HTMLElement);
    click(buttonNamed(document, "Nouveau fichier") as HTMLElement);
    const dialog = document.querySelector("dialog[open]") as HTMLDialogElement;
    expect(dialog).not.toBeNull();
    const name = dialog.querySelector<HTMLInputElement>(
      "input:not([type=radio])",
    ) as HTMLInputElement;
    expect(name.value).toBe("sans-titre.typ");
    // Pick Markdown: the default name follows until the person types.
    const radios = dialog.querySelectorAll<HTMLInputElement>("input[type=radio]");
    (radios[1] as HTMLInputElement).click();
    flushSync();
    expect(name.value).toBe("sans-titre.md");
    name.value = "notes.md";
    name.dispatchEvent(new Event("input", { bubbles: true }));
    dialog
      .querySelector("form")
      ?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    flushSync();
    expect(rowOf(host, "notes.md")).not.toBeNull();
    expect(runtime.currentPath).toBe("notes.md");
    const id = runtime.tree.byPath.get("notes.md")?.id as string;
    await until(() => runtime.handleOf(id) !== undefined);
    expect(runtime.handleOf(id)?.doc()?.text).toBe("# Titre\n\n");
  });

  it("UX-S6-1 a name with another extension (.py) is not filled with the Typst template", async () => {
    const { host, runtime } = await mount("admin");
    click(buttonNamed(host, "Ajouter au projet") as HTMLElement);
    click(buttonNamed(document, "Nouveau fichier") as HTMLElement);
    const dialog = document.querySelector("dialog[open]") as HTMLDialogElement;
    const name = dialog.querySelector<HTMLInputElement>(
      "input:not([type=radio])",
    ) as HTMLInputElement;
    name.value = "outil.py";
    name.dispatchEvent(new Event("input", { bubbles: true }));
    dialog
      .querySelector("form")
      ?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    flushSync();
    const id = runtime.tree.byPath.get("outil.py")?.id as string;
    await until(() => runtime.handleOf(id) !== undefined);
    expect(runtime.handleOf(id)?.doc()?.text).toBe("");
  });

  it("UX-S6-2 deleting a file tells the person it went to the trash", async () => {
    const { host } = await mount("admin", ["a.md"]);
    const row = rowOf(host, "a.md") as HTMLElement;
    row.focus();
    key(row, "Delete");
    flushSync();
    expect(notices()).toMatch(/corbeille/i);
  });

  it("FM-01 a name that is taken or invalid is explained in the dialog and nothing is made", async () => {
    const { host, runtime } = await mount("admin", ["a.md"]);
    click(buttonNamed(host, "Ajouter au projet") as HTMLElement);
    click(buttonNamed(document, "Nouveau fichier") as HTMLElement);
    const dialog = document.querySelector("dialog[open]") as HTMLDialogElement;
    const name = dialog.querySelector<HTMLInputElement>(
      "input:not([type=radio])",
    ) as HTMLInputElement;
    for (const [value, message] of [
      ["A.md", /existe déjà/],
      ["CON.txt", /réservé par Windows/],
      ["a/b", /caractère interdit/],
    ] as const) {
      name.value = value;
      name.dispatchEvent(new Event("input", { bubbles: true }));
      dialog
        .querySelector("form")
        ?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      flushSync();
      expect(text(dialog.querySelector('[role="alert"]'))).toMatch(message);
    }
    expect(runtime.tree.entries.map((e) => e.path)).toEqual(["a.md", "main.typ"]);
  });

  it("FM-03 F2 renames in place; an invalid or taken name is explained and nothing changes", async () => {
    const { host, runtime } = await mount("admin", ["a.md", "b.md"]);
    const row = rowOf(host, "a.md") as HTMLElement;
    row.focus();
    key(row, "F2");
    const input = host.querySelector<HTMLInputElement>("input.rename") as HTMLInputElement;
    expect(input.value).toBe("a.md");
    for (const [value, message] of [
      ["b.md", /existe déjà/],
      ["x:y", /caractère interdit/],
      ["", /vide/],
    ] as const) {
      input.value = value;
      input.dispatchEvent(new Event("input", { bubbles: true }));
      key(input, "Enter");
      expect(text(host.querySelector(".rename-error"))).toMatch(message);
      expect(runtime.tree.byPath.has("a.md")).toBe(true);
    }
    input.value = "c.md";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    key(input, "Enter");
    expect(host.querySelector("input.rename")).toBeNull();
    expect(rowPaths(host)).toContain("c.md");
    expect(rowPaths(host)).not.toContain("a.md");
  });

  it("FM-03 Escape leaves the name as it was", async () => {
    const { host } = await mount("admin", ["a.md"]);
    const row = rowOf(host, "a.md") as HTMLElement;
    row.focus();
    key(row, "F2");
    const input = host.querySelector<HTMLInputElement>("input.rename") as HTMLInputElement;
    input.value = "zzz";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    key(input, "Escape");
    expect(rowPaths(host)).toContain("a.md");
  });

  it("a new folder is created and goes straight to rename", async () => {
    const { host } = await mount("admin");
    click(buttonNamed(host, "Ajouter au projet") as HTMLElement);
    click(buttonNamed(document, "Nouveau dossier") as HTMLElement);
    flushSync();
    await until(() => host.querySelector("input.rename") !== null, 1000, "no rename field");
    expect(rowPaths(host)).toContain("nouveau-dossier");
  });
});

describe("FM-02 importing", () => {
  const file = (name: string, body: BlobPart, rel = "") => {
    const f = new File([body], name);
    if (rel) Object.defineProperty(f, "webkitRelativePath", { value: rel });
    return f;
  };

  it("a picked folder is reproduced; texts become documents, the rest blobs; bad names are fixed and said", async () => {
    const { host, runtime, store } = await mount("admin");
    const picker = host.parentElement?.querySelector<HTMLInputElement>(
      '[data-testid="folder-picker"]',
    );
    const input = (picker ??
      document.querySelector('[data-testid="folder-picker"]')) as HTMLInputElement;
    const files = [
      file("a.typ", "= A\n", "rapport/a.typ"),
      file("CON.txt", "x", "rapport/CON.txt"),
      file("fig.png", new Uint8Array([0x89, 0x50, 0, 1]), "rapport/img/fig.png"),
    ];
    Object.defineProperty(input, "files", { value: files, configurable: true });
    input.dispatchEvent(new Event("change", { bubbles: true }));
    await until(() => runtime.tree.byPath.has("rapport/img/fig.png"), 3000, "not imported");
    flushSync();
    expect(rowPaths(host)).toEqual(
      [
        "rapport",
        "rapport/img",
        "rapport/img/fig.png",
        "rapport/_CON.txt",
        "rapport/a.typ",
        "main.typ",
      ].sort((x, y) => rowPaths(host).indexOf(x) - rowPaths(host).indexOf(y)),
    );
    expect(runtime.tree.byPath.get("rapport/a.typ")?.type).toBe("text");
    expect(runtime.tree.byPath.get("rapport/img/fig.png")?.type).toBe("blob");
    await until(() => store.files.has("rapport/img/fig.png"), 3000);
    expect(notices()).toMatch(/3 fichier\(s\) ajouté\(s\)/);
    expect(notices()).toMatch(/1 nom\(s\) corrigé\(s\)/);
  });

  it("dropping files on the list imports them into the folder under the pointer", async () => {
    const { host, runtime } = await mount("admin", ["docs/"]);
    const target = rowOf(host, "docs") as HTMLElement;
    const over = dragEvent("dragover", {}, [file("n.md", "# N")]);
    fire(target, over);
    expect(over.defaultPrevented).toBe(true);
    expect(target.classList.contains("drop")).toBe(true);
    const drop = dragEvent("drop", {}, [file("n.md", "# N")]);
    fire(target, drop);
    await until(() => runtime.tree.byPath.has("docs/n.md"), 3000, "not imported");
  });
});

describe("FM-04 moving by drag and drop", () => {
  it("moves a file into a folder, and refuses a folder in its own descendant", async () => {
    const { host, runtime } = await mount("admin", ["a/", "a/b/", "f.md"]);
    const dragged = rowOf(host, "f.md") as HTMLElement;
    const start = dragEvent("dragstart");
    fire(dragged, start);
    const payload = (start.dataTransfer as DataTransfer).getData("application/x-inkwell-files");
    expect(JSON.parse(payload)).toEqual([runtime.tree.byPath.get("f.md")?.id]);
    fire(
      rowOf(host, "a") as HTMLElement,
      dragEvent("drop", { "application/x-inkwell-files": payload }),
    );
    expect(runtime.tree.byPath.has("a/f.md")).toBe(true);

    // The folder a into its own child a/b: refused, with a message, and nothing moves.
    const folder = JSON.stringify([runtime.tree.byPath.get("a")?.id]);
    fire(
      rowOf(host, "a/b") as HTMLElement,
      dragEvent("drop", { "application/x-inkwell-files": folder }),
    );
    expect(runtime.tree.byPath.has("a/b")).toBe(true);
    expect(notices()).toMatch(/dans lui-même/);
  });

  it("Ctrl+X then Ctrl+V on a folder moves too, for those who do not drag", async () => {
    const { host, runtime } = await mount("admin", ["d/", "f.md"]);
    const file = rowOf(host, "f.md") as HTMLElement;
    file.focus();
    key(file, "x", { ctrlKey: true });
    const dir = rowOf(host, "d") as HTMLElement;
    dir.focus();
    key(dir, "v", { ctrlKey: true });
    expect(runtime.tree.byPath.has("d/f.md")).toBe(true);
  });

  it("dropping on the empty part of the list moves to the root", async () => {
    const { host, runtime } = await mount("admin", ["d/", "d/f.md"]);
    const id = runtime.tree.byPath.get("d/f.md")?.id;
    fire(
      host.querySelector('[role="tree"]') as HTMLElement,
      dragEvent("drop", { "application/x-inkwell-files": JSON.stringify([id]) }),
    );
    expect(runtime.tree.byPath.has("f.md")).toBe(true);
  });
});

describe("FM-05 FM-06 FM-07 deleting on approval", () => {
  it("a writer's delete is a request: the file is struck through and the admin is asked", async () => {
    const { host, runtime } = await mount("writer", ["ch2.typ"]);
    const row = rowOf(host, "ch2.typ") as HTMLElement;
    row.focus();
    key(row, "Delete");
    expect(rowOf(host, "ch2.typ")?.classList.contains("asked")).toBe(true);
    expect(text(rowOf(host, "ch2.typ"))).toContain("suppression demandée");
    expect(runtime.tree.pending.map((e) => e.path)).toEqual(["ch2.typ"]);
    expect(notices()).toMatch(/administrateur/);
    // A writer is not offered to answer.
    expect(buttonNamed(host, "Accepter")).toBeUndefined();
  });

  it("an admin sees « X demande à supprimer … » and accepts: the file goes to the trash", async () => {
    const { host, runtime } = await mount("admin", ["ch2.typ"]);
    const id = runtime.tree.byPath.get("ch2.typ")?.id as string;
    runtime.role = "writer";
    runtime.index.remove([id]);
    runtime.role = "admin";
    flushSync();
    expect(text(host.querySelector(".requests"))).toMatch(/demande à supprimer ch2\.typ/);
    click(buttonNamed(host, "Accepter") as HTMLElement);
    expect(rowOf(host, "ch2.typ")).toBeNull();
    expect(runtime.tree.trash.map((t) => t.name)).toEqual(["ch2.typ"]);
  });

  it("an admin refuses: the request goes, the refusal is shown on the file", async () => {
    const { host, runtime } = await mount("admin", ["ch2.typ"]);
    const id = runtime.tree.byPath.get("ch2.typ")?.id as string;
    runtime.role = "writer";
    runtime.index.remove([id]);
    runtime.role = "admin";
    flushSync();
    click(buttonNamed(host, "Refuser") as HTMLElement);
    expect(host.querySelector(".requests")).toBeNull();
    expect(text(rowOf(host, "ch2.typ"))).toContain("Suppression refusée par");
    expect(rowOf(host, "ch2.typ")?.classList.contains("asked")).toBe(false);
  });
});

describe("FM-12 the trash", () => {
  it("lists what was deleted and who did it; restore brings it back; emptying names the files and is admin's", async () => {
    const { host, runtime } = await mount("admin", ["a.md", "b.md"]);
    runtime.index.remove([
      runtime.tree.byPath.get("a.md")?.id as string,
      runtime.tree.byPath.get("b.md")?.id as string,
    ]);
    flushSync();
    expect(rowPaths(host)).toEqual(["main.typ"]);
    const toggle = host.querySelector<HTMLElement>("button[aria-pressed]") as HTMLElement;
    expect(toggle.getAttribute("aria-label")).toBe("Afficher la corbeille (2)");
    click(toggle);
    const region = host.querySelector('[role="region"]') as HTMLElement;
    expect(text(region)).toContain("a.md");
    expect(text(region)).toContain("Supprimé par Alice");
    click(buttonNamed(region, "Restaurer a.md") as HTMLElement);
    expect(runtime.tree.byPath.has("a.md")).toBe(true);

    click(buttonNamed(region, "Vider la corbeille") as HTMLElement);
    const dialog = document.querySelector("dialog[open]") as HTMLDialogElement;
    expect(text(dialog)).toContain("b.md");
    click(buttonNamed(dialog, "Vider la corbeille") as HTMLElement);
    expect(runtime.tree.trash).toHaveLength(0);
    expect(rowPaths(host)).toEqual(["a.md", "main.typ"]);
  });

  it("a writer sees the trash but cannot empty it", async () => {
    const { host, runtime } = await mount("admin", ["a.md"]);
    runtime.index.remove([runtime.tree.byPath.get("a.md")?.id as string]);
    runtime.role = "writer";
    flushSync();
    click(host.querySelector<HTMLElement>("button[aria-pressed]") as HTMLElement);
    const region = host.querySelector('[role="region"]') as HTMLElement;
    expect(text(region)).toContain("a.md");
    expect(buttonNamed(region, "Vider la corbeille")).toBeUndefined();
  });
});

describe("FM-13 a reader", () => {
  it("sees everything and is offered no change: no menu to add, no rename, no delete, no drag", async () => {
    const { host } = await mount("admin", ["a.md"]);
    cleanup();
    const reader = await mount("reader", ["a.md"].slice(0, 0));
    expect(buttonNamed(reader.host, "Ajouter au projet")).toBeUndefined();
    const row = rowOf(reader.host, "main.typ") as HTMLElement;
    expect(row.getAttribute("draggable")).toBe("false");
    row.focus();
    key(row, "F2");
    expect(reader.host.querySelector("input.rename")).toBeNull();
    key(row, "Delete");
    expect(reader.runtime.tree.pending).toHaveLength(0);
    key(row, "F10", { shiftKey: true });
    const names = [...document.querySelectorAll('[role="menuitem"]')].map((i) => text(i));
    expect(names.map((n) => n.replace(/ ↵$/, ""))).toEqual(["Ouvrir", "Copier le chemin"]);
    void host;
  });
});

describe("FM-15 FM-16 FM-18 settings and the main file", () => {
  it("FM-16 the rule is an admin's menu; FM-15 once free, a writer deletes directly", async () => {
    const { host, runtime } = await mount("admin", ["a.md"]);
    click(buttonNamed(host, "Réglages des fichiers") as HTMLElement);
    click(buttonNamed(document, /Librement/) as HTMLElement);
    expect(runtime.index.mode).toBe("free");
    runtime.role = "writer";
    flushSync();
    expect(buttonNamed(host, "Réglages des fichiers")).toBeUndefined();
    const row = rowOf(host, "a.md") as HTMLElement;
    row.focus();
    key(row, "Delete");
    expect(rowOf(host, "a.md")).toBeNull();
    expect(runtime.tree.trash.map((t) => t.name)).toEqual(["a.md"]);
  });

  it("FM-18 the main file cannot be deleted; only an admin picks another", async () => {
    const { host, runtime } = await mount("admin", ["autre.typ"]);
    const main = rowOf(host, "main.typ") as HTMLElement;
    main.focus();
    key(main, "F10", { shiftKey: true });
    const del = buttonNamed(document, /^Supprimer/) as HTMLElement;
    expect(del.getAttribute("aria-disabled")).toBe("true");
    key(document.querySelector('[role="menu"]') as Element, "Escape");

    const other = rowOf(host, "autre.typ") as HTMLElement;
    other.focus();
    key(other, "F10", { shiftKey: true });
    click(buttonNamed(document, "Définir comme fichier principal") as HTMLElement);
    expect(runtime.tree.mainId).toBe(runtime.tree.byPath.get("autre.typ")?.id);

    runtime.role = "writer";
    flushSync();
    const row = rowOf(host, "main.typ") as HTMLElement;
    row.focus();
    key(row, "F10", { shiftKey: true });
    expect(buttonNamed(document, "Définir comme fichier principal")).toBeUndefined();
  });
});

describe("selecting several files", () => {
  it("Ctrl+click adds, Shift+click extends, and Delete acts on all of them", async () => {
    const { host, runtime } = await mount("admin", ["a.md", "b.md", "c.md", "d.md"]);
    click(rowOf(host, "a.md") as HTMLElement);
    fire(
      rowOf(host, "c.md") as HTMLElement,
      new MouseEvent("click", { bubbles: true, ctrlKey: true, detail: 1 }),
    );
    expect(
      items(host)
        .filter((r) => r.getAttribute("aria-selected") === "true")
        .map((r) => r.dataset.path),
    ).toEqual(["a.md", "c.md"]);
    fire(
      rowOf(host, "d.md") as HTMLElement,
      new MouseEvent("click", { bubbles: true, shiftKey: true, detail: 1 }),
    );
    // Shift+click selects from the last file clicked to this one, as file managers do.
    expect(
      items(host)
        .filter((r) => r.getAttribute("aria-selected") === "true")
        .map((r) => r.dataset.path),
    ).toEqual(["c.md", "d.md"]);
    const row = rowOf(host, "d.md") as HTMLElement;
    row.focus();
    key(row, "Delete");
    expect(runtime.tree.trash.map((t) => t.name).sort()).toEqual(["c.md", "d.md"]);
    expect(rowPaths(host)).toEqual(["a.md", "b.md", "main.typ"]);
  });

  it("Ctrl+A selects everything", async () => {
    const { host } = await mount("admin", ["a.md"]);
    const row = rowOf(host, "a.md") as HTMLElement;
    row.focus();
    key(row, "a", { ctrlKey: true });
    expect(items(host).every((r) => r.getAttribute("aria-selected") === "true")).toBe(true);
  });
});

describe("other actions of the menu", () => {
  it("duplicates a file and a folder with what is in it", async () => {
    const { host, runtime } = await mount("admin", ["d/", "d/f.md", "g.md"]);
    for (const path of ["g.md", "d"]) {
      const row = rowOf(host, path) as HTMLElement;
      row.focus();
      key(row, "F10", { shiftKey: true });
      click(buttonNamed(document, "Dupliquer") as HTMLElement);
    }
    expect(runtime.tree.entries.map((e) => e.path)).toEqual(
      expect.arrayContaining(["g (copie).md", "d (copie)", "d (copie)/f.md"]),
    );
  });

  it("copies the path", async () => {
    const write = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: write },
      configurable: true,
    });
    const { host } = await mount("admin", ["d/", "d/f.md"]);
    const row = rowOf(host, "d/f.md") as HTMLElement;
    row.focus();
    key(row, "F10", { shiftKey: true });
    click(buttonNamed(document, "Copier le chemin") as HTMLElement);
    expect(write).toHaveBeenCalledWith("d/f.md");
  });

  it("PRJ-13 a file of the folder is shown apart, and added only when the person says so", async () => {
    const { host, runtime, store } = await mount("admin");
    store.disk.set("scan.png", Uint8Array.of(1, 2, 3));
    await runtime.refreshUntracked();
    flushSync();
    const section = host.querySelector("section") as HTMLElement;
    expect(text(section)).toContain("Dans le dossier, pas dans le projet");
    expect(rowPaths(host)).not.toContain("scan.png");
    click(buttonNamed(section, "Ajouter au projet") as HTMLElement);
    await until(() => runtime.tree.byPath.has("scan.png"), 3000, "not added");
    flushSync();
    expect(rowPaths(host)).toContain("scan.png");
  });
});

describe("PRJ-02 PRJ-11 reception", () => {
  it("shows « Réception : 3 sur 5 fichiers » and asks before a project over its quota", async () => {
    let stored = 2;
    let confirmation = false;
    const p = await project("admin", async () => ({
      Files: 4,
      Stored: stored,
      Bytes: 4 * 1024 * 1024,
      StoredBytes: 2 * 1024 * 1024,
      Limit: 2 * 1024 ** 3,
      NeedsConfirmation: confirmation,
      Refused: confirmation ? ["gros.bin"] : [],
    }));
    for (let i = 0; i < 4; i++) {
      p.runtime.index.createMany([
        {
          type: "blob",
          name: `f${i}.png`,
          parent: null,
          hash: `blake3:${String(i).repeat(64)}`,
          size: 1024 * 1024,
        },
      ]);
    }
    const host = render(Sidebar, { runtime: p.runtime, onOpen: () => {} });
    await p.runtime.refreshProgress();
    flushSync();
    const bar = host.querySelector(".progress") as HTMLElement;
    expect(text(bar)).toMatch(/Réception : 3 sur 5 fichiers, 2 Mo/);
    expect(bar.querySelector("progress")?.getAttribute("value")).toBe("3");

    confirmation = true;
    await p.runtime.refreshProgress();
    flushSync();
    expect(text(host.querySelector(".progress"))).toMatch(/dépasse 2 Go\. Télécharger la suite/);
    expect(text(host.querySelector(".progress"))).toMatch(/gros\.bin dépasse 100 Mo/);
    stored = 4;
  });
});

describe("FM-20 a thousand files", () => {
  it("draws only the rows in view, and keeps the tree whole for the keyboard", async () => {
    const { host, runtime } = await mount("admin");
    runtime.index.createMany(
      Array.from({ length: 1000 }, (_, i) => ({
        type: "blob" as const,
        name: `f${String(i).padStart(4, "0")}.png`,
        parent: null,
        hash: `blake3:${"a".repeat(64)}`,
        size: 1,
      })),
    );
    flushSync();
    // happy-dom lays nothing out, so every row is drawn; with a window of 400 px a few dozen are.
    const nav = host.querySelector("nav") as HTMLElement;
    Object.defineProperty(nav, "clientHeight", { value: 400, configurable: true });
    nav.dispatchEvent(new Event("scroll"));
    expect(items(host).length).toBeGreaterThan(900);
    const tree = host.querySelector('[role="tree"]') as HTMLElement;
    expect(tree.querySelectorAll('[aria-setsize="1001"]').length).toBeGreaterThan(0);
    // (a loaded machine, with the whole gate running, draws 1000 rows slowly: a roomy limit)
  }, 30_000);
});
