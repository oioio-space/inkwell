// Block EX0c3 (docs/plans/2026-11-25-ex0c3-insert-format.md §4): the formatting language is the
// `format` entry of a grammar, the Insert menu is the `insert` location. One test per rule F-n; each
// has a mutant (scripts/netsim/mutants.toml, MI1..MI7 and MI9) that only that test catches.
import { EditorState } from "@codemirror/state";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  formatHandlers,
  formatsOf,
  insert,
  insertsOf,
  type Lang,
  noteInsertHost,
  openInsertStep,
  removeEmptyDelimiters,
  setBlock,
  textHandlers,
} from "../../editor/format";
import { apply, MARKDOWN, PLAIN, parse, TYPST } from "../../editor/format/testkit";
import { languageOf } from "../../editor/setup";
import { countWords } from "../../editor/stats/words";
import { registry } from "../../extensions";
import {
  configure,
  locationShows,
  menuOf,
  registerSurface,
  type Shell,
  useRegistry,
} from "../../extensions/commands";
import { builtinFormatEntry, FORMAT_FILES } from "../../extensions/formatEntries";
import { setLive } from "../../extensions/live.svelte";
import { builtinSources, globLoader } from "../../extensions/loader";
import { ManifestError, type ManifestSource, parseManifest } from "../../extensions/manifest";
import { buildRegistry } from "../../extensions/registry";
import { LANGUAGE_TYPE } from "../../language/client";
import { writingHelp } from "../../language/extension";
import { problems } from "../../language/problems.svelte";
import type { ProjectFile } from "../../store/files";
import { viewChoice } from "../../workspace/viewChoice.svelte";
import { workspace } from "../../workspace/workspace.svelte";
import { cleanup, click, fakeEditor, fakeWidth, render } from "../testkit.svelte";
import FormatBar from "./FormatBar.svelte";

const VECTORS = import.meta.glob("../../extensions/testdata/manifests/*/*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const verdict = (s: ManifestSource): string => {
  try {
    parseManifest(s);
    return "accepted";
  } catch (error) {
    if (error instanceof ManifestError) return error.reason;
    throw error;
  }
};

const files: ProjectFile[] = [
  { path: "main.typ", size: 0, main: true },
  { path: "fig.png", size: 1, main: false },
  { path: "a/schema.svg", size: 1, main: false },
  { path: "notes.md", size: 1, main: false },
  { path: "data.csv", size: 1, main: false },
  { path: "refs.bib", size: 1, main: false },
];

let restoreWidth = () => {};
beforeEach(() => {
  restoreWidth = fakeWidth(1200);
  vi.spyOn(Object.getPrototypeOf(workspace), "rightsKnown", "get").mockReturnValue(true);
});
afterEach(() => {
  vi.restoreAllMocks();
  cleanup();
  restoreWidth();
});

/** The bar of a file in a language, on an editor surface, as Editor.svelte declares it. */
function mountBar(
  marked: string,
  lang: Lang,
  readFile = async (_: string) => undefined as string | undefined,
) {
  useRegistry(registry, { system: "linux" });
  const editor = fakeEditor(marked);
  const target = document.createElement("div");
  document.body.append(target);
  registerSurface(target, "editor", formatHandlers(lang, editor.run), {
    viewer: registry.essentialViewer("text"),
  });
  setLive({ formats: formatsOf(lang), inserts: insertsOf(lang), readOnly: false });
  const host = render(FormatBar, {
    lang,
    get editorState() {
      return editor.state;
    },
    target,
    run: editor.run,
    files,
    currentFile: lang === TYPST ? "main.typ" : "notes.md",
    readFile,
  });
  return { editor, host, target };
}

const openInsert = (host: HTMLElement) =>
  click(
    [...host.querySelectorAll<HTMLElement>('button[aria-haspopup="menu"]')].find((b) =>
      b.textContent?.includes("Insérer"),
    ) as HTMLElement,
  );
const menuItems = () => [...document.querySelectorAll<HTMLElement>("[data-insert]")];
const picks = () =>
  [...document.querySelectorAll<HTMLElement>('[data-fixed="insert-file"]')].map((b) =>
    b.textContent?.trim(),
  );
const settle = () => new Promise((r) => setTimeout(r, 0));

describe("EX0c3 the Insert menu and the formatting language", () => {
  it("EX0c3 F-1 a malformed format entry is refused with its reason", () => {
    const vectors = Object.entries(VECTORS).filter(([path]) =>
      /\/(invalid|valid)\/[^/]*(format|grammar-no-label)[^/]*\.json$/.test(path),
    );
    const invalid = vectors.filter(([path]) => path.includes("/invalid/"));
    expect(invalid.length).toBe(4);
    for (const [path, text] of invalid) {
      const name = path.slice(path.lastIndexOf("/") + 1);
      const origin = name.includes("plugin-") ? "plugin" : "builtin";
      const source = { origin, folder: "lab", text, files: new Set<string>() } as const;
      expect({ name, reason: verdict(source) }).toEqual({
        name,
        reason: name.slice(0, name.indexOf(".")),
      });
    }
    // A built-in's format entry is accepted, its data as written.
    const [, valid] = vectors.find(([path]) => path.includes("/valid/")) ?? ["", ""];
    const files = new Set(["native/grammar.ts", "native/format.ts"]);
    const grammar = parseManifest({ origin: "builtin", folder: "lab", text: valid, files })
      .contributes.grammars[0];
    expect(grammar?.label).toBe("Lab");
    expect(JSON.parse(JSON.stringify(grammar?.format))).toEqual({
      entry: "native/format.ts",
      accepts: { image: ["png", "svg"] },
      readers: { csv: "csv" },
    });
    expect(Object.getPrototypeOf(grammar?.format?.accepts)).toBeNull();
  });

  it("EX0c3 F-2 the formatting language is its grammar's format entry, plain text without one", () => {
    // The built-ins' entries are there with the first screen: no load to wait for.
    expect(FORMAT_FILES).toEqual(
      expect.arrayContaining(["../ext/typst/native/format.ts", "../ext/markdown/native/format.ts"]),
    );
    expect(builtinFormatEntry("typst", "native/format.ts")).toHaveProperty("default.syntax");
    expect(registry.format("typst")).toBe(TYPST);
    expect([TYPST.id, TYPST.label, MARKDOWN.label]).toEqual([
      "typst",
      "lang.typst",
      "lang.markdown",
    ]);
    expect(registry.format(registry.typeOf("data.csv")?.grammar)).toBe(PLAIN);
    // A grammar without `format` is plain text: no bar, no Insert item.
    const lab: ManifestSource = {
      origin: "builtin",
      folder: "lab",
      files: new Set(["native/grammar.ts"]),
      text: JSON.stringify({
        id: "lab",
        version: "1.0.0",
        essential: true,
        contributes: {
          grammars: [
            {
              id: "lab",
              label: "kind.text",
              names: ["labnotes"],
              entry: "native/grammar.ts",
              capabilities: ["ui.native"],
            },
          ],
        },
      }),
    };
    const r = buildRegistry([...builtinSources(), lab], globLoader(), {
      formatEntry: builtinFormatEntry,
    });
    expect(r.refused).toEqual([]);
    expect(r.format("lab")).toBeUndefined();
    expect(r.format("labnotes")).toBeUndefined();
    expect(r.format("markdown")?.syntax).toBe(MARKDOWN.syntax);
    useRegistry(registry, { system: "linux" });
    expect(locationShows("format", { formats: formatsOf(r.format("lab")) })).toBe(false);
    // `formats` and `inserts` are the entry's.
    expect(formatsOf(MARKDOWN).has("task")).toBe(true);
    expect(formatsOf(TYPST).has("task")).toBe(false);
    expect([...insertsOf(MARKDOWN)].sort()).toEqual(["csvTable", "footnote", "image", "link"]);
    expect(insertsOf(PLAIN).size).toBe(0);
  });

  it("EX0c3 F-3 a language's quote wrapping, cleanup pairs and word stripping come from its entry", () => {
    expect(apply("⟨a\nb⟩", setBlock(TYPST, "quote"))).toBe("#quote[⟨a\nb⟩]");
    expect(apply("⟨a\nb⟩", setBlock(MARKDOWN, "quote"))).toBe("> ⟨a\n> b⟩");
    // A language without a quote call writes its quote prefix, whatever its other tables.
    const prefixed: Lang = { ...TYPST, syntax: MARKDOWN.syntax, quote: undefined };
    expect(apply("⟨a⟩", setBlock(prefixed, "quote"))).toBe("> ⟨a⟩");
    // The cleanup removes the entry's own empty pairs, and only those.
    expect(removeEmptyDelimiters("a ** b", TYPST)).toBe("a  b");
    expect(removeEmptyDelimiters("a ** b", MARKDOWN)).toBe("a ** b");
    const own: Lang = { ...MARKDOWN, emptyPairs: [/@@/g] };
    expect(removeEmptyDelimiters("a @@ ** b", own)).toBe("a  ** b");
    expect(removeEmptyDelimiters("a ** b", PLAIN)).toBe("a ** b");
    // Words are counted on what the entry's `strip` leaves.
    expect(countWords('= Titre\n#set text(lang: "fr")\nun mot', TYPST)).toBe(3);
    expect(countWords("deux mots", { ...MARKDOWN, strip: () => "un deux trois" })).toBe(3);
  });

  it("EX0c3 F-4 the Insert menu is the insert location, its groups captioned, its items hidden by their when", () => {
    const { host, target } = mountBar("‸", TYPST);
    openInsert(host);
    const menu = document.querySelector('[role="menu"][aria-label^="Insérer"]') as HTMLElement;
    const want = menuOf("insert", { inserts: insertsOf(TYPST) }, target).flatMap((g) =>
      g.items.map((c) => c.id),
    );
    expect(menuItems().map((i) => i.dataset.cmd)).toEqual(want);
    // The file link stands in for attach only: Typst attaches.
    expect(want).not.toContain("format.insert-file-link");
    expect([...menu.querySelectorAll(".cap")].map((c) => c.textContent)).toEqual([
      "Fichiers du projet",
      "Références",
      "Mise en page",
    ]);
    // Named by its label, described by its description and the syntax it writes.
    const image = menuItems()[0] as HTMLElement;
    expect(image.getAttribute("aria-label")).toBe("Image ou page de PDF");
    const described = (image.getAttribute("aria-describedby") ?? "")
      .split(" ")
      .map((id) => document.getElementById(id)?.textContent);
    expect(described).toEqual(["Affiche le fichier dans le document", "#image()"]);
    cleanup();

    const md = mountBar("‸", MARKDOWN);
    openInsert(md.host);
    expect(menuItems().map((i) => i.dataset.insert)).toEqual([
      "image",
      "csvTable",
      "link",
      "footnote",
    ]);
    const csv = menuItems()[1] as HTMLElement;
    const ids = (csv.getAttribute("aria-describedby") ?? "").split(" ");
    expect(document.getElementById(ids[0] ?? "")?.textContent).toContain("Copie figée");
    expect(document.getElementById(ids[1] ?? "")?.textContent).toBe("| a | b |");
    expect(document.querySelector('[data-fixed="insert-note"]')?.textContent).toContain(
      "Markdown ne sait ni joindre",
    );
  });

  it("EX0c3 F-5 an Insert item inserts at once or opens its step, the picker listing the types its language accepts", async () => {
    const { editor, host } = mountBar("texte‸", TYPST, async (path) =>
      path === "refs.bib" ? "@book{knuth, title={TAOCP}}" : undefined,
    );
    // Nothing to ask: one insertion, the menu closed.
    openInsert(host);
    click(document.querySelector('[data-insert="footnote"]') as HTMLElement);
    expect(editor.text()).toBe("texte#footnote[]");
    expect(menuItems()).toEqual([]);
    // A file: the picker lists the types the language accepts for the item, by path.
    openInsert(host);
    click(document.querySelector('[data-insert="image"]') as HTMLElement);
    expect(picks()).toEqual(["a/schema.svg", "fig.png"]);
    click(document.querySelector('[data-fixed="insert-back"]') as HTMLElement);
    click(document.querySelector('[data-insert="bibliography"]') as HTMLElement);
    expect(picks()).toEqual(["refs.bib"]);
    click(document.querySelector('[data-fixed="insert-back"]') as HTMLElement);
    // A key without a bibliography: its step first.
    click(document.querySelector('[data-insert="cite"]') as HTMLElement);
    expect(document.querySelector('[data-fixed="insert-nobib"]')).not.toBeNull();
    click(document.querySelector('[data-fixed="insert-add-bib"]') as HTMLElement);
    expect(picks()).toEqual(["refs.bib"]);
    cleanup();

    // With a bibliography: the key form and the keys of the files `accepts` lists.
    const cited = mountBar('#bibliography("refs.bib")\n‸', TYPST, async (path) =>
      path === "refs.bib" ? "@book{knuth, title={TAOCP}}" : undefined,
    );
    openInsert(cited.host);
    click(document.querySelector('[data-insert="cite"]') as HTMLElement);
    await settle();
    expect(document.querySelector('[data-fixed="insert-keyform"]')).not.toBeNull();
    expect(
      [...document.querySelectorAll('[data-fixed="insert-key"]')].map((k) => k.textContent?.trim()),
    ).toEqual(["knuth refs.bib"]);
    cleanup();

    // From the palette: the command asks the shell for the step; the menu opens there.
    const steps: string[] = [];
    configure({ shell: { step: (item: string) => steps.push(item) } as unknown as Shell });
    const handlers = textHandlers(TYPST, () => true);
    expect(handlers["format.insert-image"]?.()).toBe(true);
    expect(steps).toEqual(["image"]);
    mountBar("‸", TYPST);
    openInsertStep("image");
    await settle();
    expect(picks()).toEqual(["a/schema.svg", "fig.png"]);
    configure({ shell: undefined as unknown as Shell });
  });

  it("EX0c3 review a step asked for one file or one bar is dropped when the file changes or the bar hides", async () => {
    // Asked on main.typ, then the person went to another file before any menu was there.
    noteInsertHost({ file: "main.typ", bar: false });
    openInsertStep("image");
    noteInsertHost({ file: "notes.md", bar: false });
    noteInsertHost({ file: "main.typ", bar: true });
    mountBar("‸", TYPST);
    await settle();
    expect(picks()).toEqual([]);
    cleanup();
    // Asked while the bar showed, then the bar was hidden: dropped too.
    noteInsertHost({ file: "main.typ", bar: true });
    openInsertStep("image");
    noteInsertHost({ file: "main.typ", bar: false });
    noteInsertHost({ file: "main.typ", bar: true });
    mountBar("‸", TYPST);
    await settle();
    expect(picks()).toEqual([]);
    cleanup();
    // The shell showing the bar for the step keeps it: the menu opens there.
    noteInsertHost({ file: "main.typ", bar: false });
    openInsertStep("image");
    noteInsertHost({ file: "main.typ", bar: true });
    mountBar("‸", TYPST);
    await settle();
    expect(picks()).toEqual(["a/schema.svg", "fig.png"]);
  });

  it("EX0c3 F-6 type ids are read only at K-7: the writing help, a viewer's problems and Typst's data readers", async () => {
    // The writing help is the language service's type's, never another's.
    const help = (type: string | undefined) =>
      (languageOf(undefined, type) as unknown[]).includes(writingHelp);
    expect(help(LANGUAGE_TYPE)).toBe(true);
    expect(help("markdown")).toBe(false);
    expect(help(undefined)).toBe(false);
    // A viewer's problem opens in the text view, at its line.
    const show = vi.spyOn(viewChoice, "show").mockImplementation(() => {});
    vi.spyOn(workspace, "refreshFiles").mockResolvedValue(undefined as never);
    vi.spyOn(workspace, "openFile").mockResolvedValue(undefined as never);
    problems.viewer("data.csv", "a,b\n1\n", [{ line: 2, message: "short row" }]);
    const p = problems.entries.find((x) => x.file === "data.csv");
    expect(p?.source).toBe("viewer");
    await problems.jump(p as NonNullable<typeof p>);
    expect(show).toHaveBeenCalledWith(expect.anything(), "data.csv", "text.editor", 4);
    problems.reset();
    // Typst's data reader comes from its manifest's `readers`; a type without one is read.
    const data = (lang: Lang, path: string) =>
      apply("‸", insert(lang, "data", { path, currentFile: "main.typ" }));
    expect(data(TYPST, "conf.yaml")).toBe('#let conf = yaml("conf.yaml")‸');
    expect(data(TYPST, "x.cbor")).toBe('#let x = read("x.cbor")‸');
    expect(data({ ...TYPST, readers: {} }, "conf.yaml")).toBe('#let conf = read("conf.yaml")‸');
  });

  it("EX0c3 F-7 format entries and the insert location cost nothing more at start: indexed once at build", () => {
    expect(registry.format("typst")).toBe(registry.format("typ"));
    expect(registry.format("markdown")).toBe(registry.format("markdown"));
    expect(Object.isFrozen(TYPST)).toBe(true);
    expect(registry.menu("insert")).toBe(registry.menu("insert"));
    expect(registry.menu("insert").length).toBe(13);
    // An editor's state does not change the language object it is given.
    const state = parse("a‸");
    expect(state).toBeInstanceOf(EditorState);
    expect(registry.format("typst")).toBe(TYPST);
  });
});
