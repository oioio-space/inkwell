import type { EditorView } from "@codemirror/view";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { formatHandlers, formatsOf } from "../../editor/format";
import { type AnyLang, MARKDOWN, PLAIN, type TestLang, TYPST } from "../../editor/format/testkit";
import { registry } from "../../extensions";
import { locationShows, menuOf, registerSurface, useRegistry } from "../../extensions/commands";
import { setLive } from "../../extensions/live.svelte";
import type { ProjectFile } from "../../store/files";
import { workspace } from "../../workspace/workspace.svelte";
import { session } from "../editorSession.svelte";
import { cleanup, click, fakeEditor, fakeWidth, press, render } from "../testkit.svelte";
import { visibleCount } from "../toolbar/layout";
import FormatBar from "./FormatBar.svelte";

/** The bar's items for a language: the `format` location for its formats (EX0c1). */
const itemsFor = (lang: AnyLang) => {
  useRegistry(registry, { system: "linux" });
  return menuOf("format", { formats: formatsOf(lang) }).flatMap((g, group) =>
    g.items.map((c) => ({ id: c.id.slice(c.id.indexOf(".") + 1), group })),
  );
};

const noFiles: ProjectFile[] = [];
const mountBar = (
  marked: string,
  lang: TestLang,
  files: ProjectFile[] = noFiles,
  currentFile = lang === TYPST ? "main.typ" : "notes.md",
) => {
  const editor = fakeEditor(marked);
  // The editor's surface, as Editor.svelte declares it: the bar's commands act there.
  const target = document.createElement("div");
  document.body.append(target);
  registerSurface(target, "editor", formatHandlers(lang, editor.run), {
    viewer: registry.essentialViewer("text"),
  });
  setLive({ formats: formatsOf(lang), readOnly: false });
  const host = render(FormatBar, {
    lang,
    get editorState() {
      return editor.state;
    },
    target,
    run: editor.run,
    files,
    currentFile,
    readFile: async () => undefined,
  });
  return { editor, host };
};

const names = (host: HTMLElement) =>
  [...host.querySelectorAll<HTMLElement>("[data-bar]")].map((b) => b.dataset.bar);
const button = (host: HTMLElement, id: string) =>
  host.querySelector<HTMLButtonElement>(`button[data-bar="${id}"]`);

let restoreWidth = () => {};
// happy-dom lays nothing out: a window wide enough for the whole bar, unless a test narrows it.
beforeEach(() => {
  restoreWidth = fakeWidth(1200);
  // Rights known (standing member, a role reported): the bar is live unless a test says otherwise.
  vi.spyOn(Object.getPrototypeOf(workspace), "rightsKnown", "get").mockReturnValue(true);
});
afterEach(() => {
  vi.restoreAllMocks();
  cleanup();
  session.view = undefined;
  restoreWidth();
});

describe("formatting bar", () => {
  it("W58a the format bar is inert until the rights are known", () => {
    const known = mountBar("a‸", TYPST).host;
    expect(known.querySelector(".tb")?.hasAttribute("inert")).toBe(false);
    vi.spyOn(Object.getPrototypeOf(workspace), "rightsKnown", "get").mockReturnValue(false);
    const unknown = mountBar("a‸", TYPST).host;
    expect(
      unknown.querySelector(".tb")?.hasAttribute("inert"),
      "N-3 no write before the rights",
    ).toBe(true);
  });

  it("UI-01 follows the file: Typst bar, then Markdown bar, then none for a csv", () => {
    // A file has a bar when its formats show an item of the `format` location (EX0c1, L-6).
    useRegistry(registry, { system: "linux" });
    const shows = (name: string) =>
      locationShows("format", {
        formats: formatsOf(registry.format(registry.typeOf(name)?.grammar)),
      });
    expect(shows("x.typ")).toBe(true);
    expect(shows("x.md")).toBe(true);
    expect(shows("x.csv")).toBe(false);
    expect(shows("refs.bib")).toBe(false);
    expect(shows("notes.txt")).toBe(false);

    const typst = mountBar("a‸", TYPST).host;
    expect(names(typst)).toContain("underline");
    expect(names(typst)).toContain("formula");
    expect(names(typst)).not.toContain("task");
    expect(typst.querySelector('[role="toolbar"]')?.getAttribute("aria-label")).toContain("Typst");
    cleanup();

    const markdown = mountBar("a‸", MARKDOWN).host;
    expect(names(markdown)).not.toContain("underline");
    expect(names(markdown)).toContain("task");
    expect(markdown.querySelector('[role="toolbar"]')?.getAttribute("aria-label")).toContain(
      "Markdown",
    );
    cleanup();

    // Nothing to render for plain text: the shell does not mount a bar at all.
    expect(itemsFor(PLAIN)).toEqual([]);
  });

  it("UI-02 shows bold pressed and the heading level at the caret", () => {
    const { editor, host } = mountBar("un *gras‸* mot", TYPST);
    expect(button(host, "bold")?.getAttribute("aria-pressed")).toBe("true");
    expect(button(host, "italic")?.getAttribute("aria-pressed")).toBe("false");
    expect(host.querySelector('[data-testid="heading-current"]')?.textContent).toBe("Paragraphe");

    // Clicking Bold off (a one-transaction toggle) follows through to the button.
    click(button(host, "bold") as HTMLElement);
    expect(editor.text()).toBe("un gras mot");
    expect(button(host, "bold")?.getAttribute("aria-pressed")).toBe("false");
    cleanup();

    const heading = mountBar("== Titre‸", TYPST).host;
    expect(heading.querySelector('[data-testid="heading-current"]')?.textContent).toBe("Titre 2");
    click(heading.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
    const items = [...document.querySelectorAll('[role="menuitemradio"]')];
    expect(items.map((i) => i.getAttribute("aria-checked"))).toEqual([
      "false",
      "false",
      "true",
      "false",
      "false",
    ]);
    expect(items[1]?.textContent).toContain("=");
    expect(items[1]?.textContent).toContain("Ctrl+1");
  });

  it("UI-02 inside inline code the Code button shows pressed", () => {
    const { host } = mountBar("`co‸de`", MARKDOWN);
    expect(button(host, "code")?.getAttribute("aria-pressed")).toBe("true");
    expect(button(host, "bold")?.getAttribute("aria-pressed")).toBe("false");
  });

  it("UI-02 a heading level from the menu is one command", () => {
    const { editor, host } = mountBar("texte‸", MARKDOWN);
    click(host.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
    const h3 = [...document.querySelectorAll<HTMLElement>('[role="menuitemradio"]')].find((i) =>
      i.textContent?.includes("Titre 3"),
    );
    click(h3 as HTMLElement);
    expect(editor.text()).toBe("### texte");
    expect(document.querySelector('[role="menu"]')).toBeNull();
  });

  it("UI-03 keyboard: Tab reaches every button, arrows move in the menu, Escape closes and refocuses the editor", () => {
    const focus = vi.fn();
    session.view = { focus } as unknown as EditorView;
    const { host } = mountBar("== t‸", TYPST);

    for (const b of host.querySelectorAll("button")) expect(b.tabIndex).toBeGreaterThanOrEqual(0);

    const trigger = host.querySelector<HTMLButtonElement>('button[aria-haspopup="menu"]');
    expect(trigger?.getAttribute("aria-expanded")).toBe("false");
    trigger?.focus();
    press(trigger as HTMLElement, "ArrowDown");
    expect(trigger?.getAttribute("aria-expanded")).toBe("true");
    flushSync();

    const menu = document.querySelector('[role="menu"]') as HTMLElement;
    expect(menu).not.toBeNull();
    return Promise.resolve().then(async () => {
      await new Promise((r) => setTimeout(r, 0));
      const radios = [...menu.querySelectorAll<HTMLElement>('[role="menuitemradio"]')];
      // The open menu puts focus on the current level (Titre 2).
      expect(document.activeElement).toBe(radios[2]);
      press(menu, "ArrowDown");
      expect(document.activeElement).toBe(radios[3]);
      press(menu, "ArrowUp");
      press(menu, "ArrowUp");
      expect(document.activeElement).toBe(radios[1]);
      press(menu, "Home");
      expect(document.activeElement).toBe(radios[0]);
      press(menu, "End");
      expect(document.activeElement).toBe(radios[4]);

      press(menu, "Escape");
      expect(document.querySelector('[role="menu"]')).toBeNull();
      expect(trigger?.getAttribute("aria-expanded")).toBe("false");
      expect(focus).toHaveBeenCalled();
    });
  });

  it("UI-05 at 800 px the buttons that do not fit move to the ⋯ menu, nothing is cut", () => {
    // A bar 536 px wide (an 800 px window less its sidebar) holds only the first buttons.
    const typstGroups = itemsFor(TYPST).map((i) => i.group);
    const fit = visibleCount(typstGroups, 536);
    expect(fit).toBeGreaterThan(0);
    expect(fit).toBeLessThan(typstGroups.length);
    expect(visibleCount(typstGroups, 1200)).toBe(typstGroups.length);
    // Never claims more room than the bar has.
    expect(visibleCount(typstGroups, 0)).toBe(0);

    const restore = fakeWidth(536);
    try {
      const { host } = mountBar("a‸", TYPST);
      const shown = names(host);
      const more = host.querySelector<HTMLButtonElement>('button[aria-label="Plus d\'actions"]');
      expect(more).not.toBeNull();
      click(more as HTMLElement);
      const overflow = [
        ...document.querySelectorAll<HTMLElement>('[role^="menuitem"][data-bar]'),
      ].map((i) => i.dataset.bar);
      // Every button is either on the bar or in the menu, once.
      expect([...shown, ...overflow].sort()).toEqual(
        itemsFor(TYPST)
          .map((i) => i.id)
          .sort(),
      );
      expect(overflow.length).toBeGreaterThan(0);
    } finally {
      restore();
    }
  });

  it("UI-05 a wide bar has no ⋯ menu", () => {
    const restore = fakeWidth(1200);
    try {
      const { host } = mountBar("a‸", TYPST);
      expect(host.querySelector('button[aria-label="Plus d\'actions"]')).toBeNull();
    } finally {
      restore();
    }
  });

  it("UI-09 tooltips say what it does, the syntax it writes and the shortcut", () => {
    const typst = mountBar("a‸", TYPST).host;
    const tip = (host: HTMLElement, id: string) =>
      host.querySelector(`#tip-${id}`)?.textContent?.replace(/\s+/g, " ").trim();
    expect(tip(typst, "bold")).toBe("Gras *texte* Ctrl+B");
    expect(tip(typst, "strike")).toBe("Barré #strike[texte] Ctrl+Maj+X");
    expect(button(typst, "bold")?.getAttribute("aria-describedby")).toBe("tip-bold");
    expect(typst.querySelector("#tip-bold")?.getAttribute("role")).toBe("tooltip");
    cleanup();

    const markdown = mountBar("a‸", MARKDOWN).host;
    expect(tip(markdown, "bold")).toBe("Gras **texte** Ctrl+B");
    expect(tip(markdown, "italic")).toBe("Italique *texte* Ctrl+I");
    expect(tip(markdown, "task")).toBe("Liste de tâches - [ ] texte");
  });
});

describe("Insert menu", () => {
  const files: ProjectFile[] = [
    { path: "main.typ", size: 0, main: true },
    { path: "chapitres/ch1.typ", size: 1, main: false },
    { path: "data.xlsx", size: 1, main: false },
    { path: "fig.png", size: 1, main: false },
  ];
  const insertButton = (host: HTMLElement) =>
    [...host.querySelectorAll<HTMLButtonElement>('button[aria-haspopup="menu"]')].find((b) =>
      b.textContent?.includes("Insérer"),
    ) as HTMLElement;
  const items = () =>
    [...document.querySelectorAll<HTMLElement>("[data-insert]")].map((i) => i.dataset.insert);

  it("UI-01 Typst offers every item with its plain-language label, grouped", () => {
    const { host } = mountBar("‸", TYPST, files);
    click(insertButton(host));
    expect(items()).toEqual([
      "image",
      "figure",
      "include",
      "attach",
      "csvTable",
      "data",
      "bibliography",
      "cite",
      "ref",
      "footnote",
      "outline",
      "pagebreak",
    ]);
    const menu = document.querySelector('[role="menu"][aria-label^="Insérer"]') as HTMLElement;
    const text = menu.textContent ?? "";
    for (const label of [
      "Fichiers du projet",
      "Image ou page de PDF",
      "Joindre un fichier au PDF",
      "Voyage dans le PDF sans s'afficher (tableur, source…)",
      "Tableau depuis un CSV",
      "Données JSON, YAML ou TOML",
      "Références",
      "Citer une source",
      "Renvoi à un titre ou une figure",
      "Note de bas de page",
      "Mise en page",
      "Table des matières",
      "Saut de page",
    ]) {
      expect(text).toContain(label);
    }
  });

  it("UI-01 Markdown shows only what it can do, and says what it cannot", () => {
    const { host } = mountBar("‸", MARKDOWN, files, "notes.md");
    click(insertButton(host));
    expect(items()).toEqual(["image", "csvTable", "link", "footnote"]);
    const menu = document.querySelector('[role="menu"][aria-label^="Insérer"]') as HTMLElement;
    expect(menu.textContent).toContain("Copie figée");
    expect(menu.textContent).toContain(
      "Markdown ne sait ni joindre un fichier au PDF ni en inclure un autre",
    );
  });

  it("UI-01 attaching a file writes #pdf.attach on its own line, in the same document", () => {
    const { editor, host } = mountBar("texte‸", TYPST, files);
    click(insertButton(host));
    click(document.querySelector('[data-insert="attach"]') as HTMLElement);
    // The file chooser lists the project's files, folders included.
    const choices = [...document.querySelectorAll('[role="menuitem"]')].map((i) =>
      i.textContent?.trim(),
    );
    expect(choices.some((c) => c?.includes("data.xlsx"))).toBe(true);
    const pick = [...document.querySelectorAll<HTMLElement>('[role="menuitem"]')].find((i) =>
      i.textContent?.includes("data.xlsx"),
    );
    click(pick as HTMLElement);
    return Promise.resolve().then(() => {
      expect(editor.text()).toBe('texte\n#pdf.attach("data.xlsx", description: "")');
    });
  });

  it("UI-01 citing with no bibliography first offers to add one", () => {
    const { host } = mountBar("‸", TYPST, files);
    click(insertButton(host));
    click(document.querySelector('[data-insert="cite"]') as HTMLElement);
    expect(document.body.textContent).toContain("pas encore de bibliographie");
    expect(document.body.textContent).toContain("Ajouter une bibliographie");
  });
});
