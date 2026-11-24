import { ensureSyntaxTree, LanguageDescription, syntaxTree } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { describe, expect, it } from "vitest";
import fr from "../i18n/fr.json";
import { builtinSources, globLoader } from "./loader";
import { type ManifestSource, parseManifest } from "./manifest";
import { buildRegistry, type EntryLoader, type GrammarModule } from "./registry";

const FIXTURE_MANIFESTS = import.meta.glob<string>("./testdata/fixture/*/inkwell-plugin.json", {
  query: "?raw",
  import: "default",
  eager: true,
});
const FIXTURE_ENTRIES = import.meta.glob("./testdata/fixture/*/native/*.ts");
const FIXTURE_ROOT = "./testdata/fixture/";

/** A loader that counts its calls and never loads anything. */
function counting(): EntryLoader & { calls: number } {
  const loader = Object.assign(
    (): Promise<unknown> => {
      loader.calls++;
      return Promise.resolve({});
    },
    { calls: 0 },
  );
  return loader;
}

/** A data-only built-in manifest source. */
function extension(id: string, fileTypes: object[], extra: object = {}): ManifestSource {
  return {
    origin: "builtin",
    folder: id,
    text: JSON.stringify({ id, version: "1.0.0", contributes: { fileTypes }, ...extra }),
    files: new Set(),
  };
}

const type = (id: string, fields: object = {}) => ({
  id,
  label: "kind.text",
  icon: "text",
  storage: "text",
  ...fields,
});

/** Every object and array reachable from v, functions aside. */
function reachable(v: unknown, out: object[] = []): object[] {
  if (v === null || typeof v !== "object" || out.includes(v)) return out;
  out.push(v);
  for (const x of Object.values(v)) if (typeof x !== "function") reachable(x, out);
  return out;
}

describe("registry", () => {
  it("EX0 R-1 the registry is built from manifests without running code", () => {
    const loader = counting();
    const sources = builtinSources();
    const registry = buildRegistry(sources, loader);
    expect(loader.calls).toBe(0);
    expect(registry.refused).toEqual([]);
    expect(registry.extensions.map((e) => e.folder)).toEqual(sources.map((s) => s.folder));
    expect(sources.length).toBe(9);
    // Any failure to read a manifest refuses that manifest alone (code review, finding 5).
    const broken = {
      origin: "builtin" as const,
      folder: "broken",
      files: new Set<string>(),
      get text(): string {
        throw new RangeError("Maximum call stack size exceeded");
      },
    };
    const survived = buildRegistry([...sources, broken], counting());
    expect(survived.refused).toEqual([{ folder: "broken", reason: "invalid" }]);
    expect(survived.extensions.length).toBe(9);
    // Every built-in validates through the one parser, and its labels are messages of the UI.
    for (const s of sources) {
      const m = parseManifest(s);
      for (const c of [...m.contributes.fileTypes, ...m.contributes.viewers])
        expect(Object.hasOwn(fr, c.label), `${s.folder}: ${c.label}`).toBe(true);
    }
    // The snapshot is frozen down to its leaves.
    const objects = reachable({
      registry,
      extensions: registry.extensions,
      fileTypes: registry.fileTypes,
      viewers: registry.viewers,
      grammars: registry.grammars,
    }).slice(1);
    expect(objects.length).toBeGreaterThan(50);
    for (const o of objects) expect(Object.isFrozen(o)).toBe(true);
    // Resolving and listing still run nothing.
    registry.open("a.csv", { content: "text" });
    registry.viewersFor(registry.fileTypes[0] ?? (registry.essentialViewer("text") as never));
    registry.codeLanguages();
    expect(loader.calls).toBe(0);
  });

  it("EX0 R-3 a claim already taken is refused in either order", () => {
    const base = builtinSources().filter((s) => s.folder !== "tables");
    const claims: [string, object, object][] = [
      [
        "extension .csv",
        type("alpha-t", { extensions: [".csv"] }),
        type("beta-t", { extensions: [".csv"] }),
      ],
      [
        "name zfile",
        type("alpha-t", { filenames: ["Zfile"] }),
        type("beta-t", { filenames: ["zfile"] }),
      ],
      ["type same", type("same", { extensions: [".aa"] }), type("same", { extensions: [".bb"] })],
    ];
    for (const [claim, a, b] of claims) {
      const alpha = extension("alpha", [a]);
      const beta = extension("beta", [b]);
      const ab = buildRegistry([...base, alpha, beta], counting());
      const ba = buildRegistry([...base, beta, alpha], counting());
      expect(ab.refused).toEqual([
        { folder: "beta", reason: `conflict on ${claim}`, with: ["alpha", "beta"] },
      ]);
      expect(ba.refused).toEqual([
        { folder: "alpha", reason: `conflict on ${claim}`, with: ["beta", "alpha"] },
      ]);
      // The rest of the registry is the same whichever came first.
      const rest = (r: typeof ab, held: string) =>
        r.fileTypes.filter((t) => t.extension !== held).map((t) => t.id);
      expect(rest(ab, "alpha")).toEqual(rest(ba, "beta"));
      expect(ab.extensions.map((e) => e.id)).toContain("alpha");
      expect(ab.extensions.map((e) => e.id)).not.toContain("beta");
    }
    // A whole name or a longer suffix ending in an essential type's extension would take its
    // files (`main.typ`, `x.x.typ`): refused, whichever comes first (code review, finding 3).
    const all = builtinSources();
    for (const [claim, t] of [
      ["name main.typ", type("taker", { filenames: ["main.typ"] })],
      ["extension .x.typ", type("taker", { extensions: [".x.typ"] })],
    ] as const) {
      const taker = extension("aaa-taker", [t]);
      for (const sources of [
        [taker, ...all],
        [...all, taker],
      ]) {
        const r = buildRegistry(sources, counting());
        expect(r.refused).toEqual([
          { folder: "aaa-taker", reason: `conflict on ${claim}`, with: ["typst", "aaa-taker"] },
        ]);
        expect(r.typeOf("main.typ")?.id).toBe("typst");
        expect(r.typeOf("a.x.typ")?.id).toBe("typst");
      }
    }
  });

  it("EX0 R-4 resolution follows the order of ADR 0027", () => {
    const registry = buildRegistry(
      [
        ...builtinSources(),
        extension("archives", [
          type("archive", { extensions: [".tar.gz"], storage: "blob" }),
          type("gzip", { extensions: [".gz"], storage: "blob" }),
        ]),
        extension("builds", [
          type("make", { filenames: ["Makefile"] }),
          // A whole name with the extension of a type that is not essential (data's `.json`).
          type("npm", { filenames: ["package.json"] }),
          type("shell", { extensions: [".sh"], shebangs: ["bash", "sh"] }),
        ]),
      ],
      counting(),
    );
    const rows: [
      string,
      { firstLine?: string; content?: "text" | "binary"; choice?: string },
      string,
      string,
      string?,
    ][] = [
      ["a.tar.gz", {}, "archive", "files.card"],
      ["a.gz", {}, "gzip", "files.card"],
      ["Makefile", { content: "text" }, "make", "text.editor"],
      ["src/package.json", { content: "text" }, "npm", "text.editor"],
      ["config.json", { content: "text" }, "json", "text.editor"],
      ["notes.txt", { content: "text" }, "text", "text.editor"],
      ["run", { firstLine: "#!/usr/bin/env bash", content: "text" }, "shell", "text.editor"],
      ["run", { firstLine: "#!/bin/sh -e", content: "text" }, "shell", "text.editor"],
      ["data.csv", { content: "binary" }, "csv", "files.card", "not-text"],
      ["data.csv", { content: "text" }, "csv", "tables.grid"],
      ["notes.unknown", { content: "text" }, "text", "text.editor"],
      ["blob.unknown", { content: "binary" }, "binary", "files.card"],
      ["fig.svg", {}, "svg", "images.view"],
      ["main.typ", { content: "text", choice: "csv" }, "csv", "tables.grid"],
      ["A.TYP", { content: "text" }, "typst", "text.editor"],
      [".typ", { content: "text" }, "text", "text.editor"],
      ["dir.v2\\README", { content: "text" }, "text", "text.editor"],
    ];
    for (const [name, hints, typeId, viewer, notice] of rows) {
      const o = registry.open(name, hints);
      expect({ name, hints, type: o.type.id, viewer: o.viewer.id, notice: o.notice }).toEqual({
        name,
        hints,
        type: typeId,
        viewer,
        notice,
      });
    }
    // A type kept as bytes is opened with no content class asked; text and unknown names are read first.
    expect(registry.needsContent("fig.svg")).toBe(false);
    expect(registry.needsContent("a.csv")).toBe(true);
    expect(registry.needsContent("notes.unknown")).toBe(true);
    expect(() => registry.open("a.csv")).toThrow();
    expect(registry.open("main.typ", { content: "text", choice: "csv" }).chosen).toBe(true);
    expect(registry.open("a.csv", { content: "text" }).chosen).toBeUndefined();
  });

  it("EX0 R-5 ui.native is refused to any origin but built-in", () => {
    const native = {
      id: "grid2",
      version: "1.0.0",
      essential: true,
      contributes: {
        fileTypes: [type("grid2-t", { extensions: [".g2"] })],
        viewers: [
          {
            id: "grid2.view",
            fileTypes: ["grid2-t"],
            label: "kind.text",
            entry: "native/v.ts",
            capabilities: ["ui.native"],
          },
        ],
      },
    };
    const files = new Set(["native/v.ts"]);
    const reasons = (s: ManifestSource) =>
      buildRegistry([...builtinSources(), s], counting()).refused;
    const text = JSON.stringify(native);
    expect(reasons({ origin: "plugin", folder: "grid2", text, files })).toEqual([
      { folder: "grid2", reason: "native-refused" },
    ]);
    expect(reasons({ origin: "project", folder: "grid2", text, files })).toEqual([
      { folder: "grid2", reason: "native-refused" },
    ]);
    expect(
      reasons({
        origin: "builtin",
        folder: "grid2",
        text: JSON.stringify({ ...native, essential: false }),
        files,
      }),
    ).toEqual([{ folder: "grid2", reason: "native-refused" }]);
    expect(reasons({ origin: "builtin", folder: "grid2", text, files })).toEqual([]);
    // A manifest cannot claim its origin.
    expect(
      reasons({
        origin: "plugin",
        folder: "grid2",
        text: JSON.stringify({ ...native, origin: "builtin" }),
        files,
      }),
    ).toEqual([{ folder: "grid2", reason: "unknown-key" }]);
    // A built-in folder with code under a manifest that does not declare ui.native fails.
    expect(
      reasons({ ...extension("data2", [type("d2", { extensions: [".d2"] })]), files }),
    ).toEqual([{ folder: "data2", reason: "native-unused" }]);
    // The loader stamps every built-in as such.
    expect(new Set(builtinSources().map((s) => s.origin))).toEqual(new Set(["builtin"]));
  });

  it("EX0 R-6 a viewer loads on first open, once, and falls back on failure", async () => {
    let release: (m: unknown) => void = () => {};
    const pending = new Promise((resolve) => {
      release = resolve;
    });
    let calls = 0;
    const registry = buildRegistry(builtinSources(), () => {
      calls++;
      return pending;
    });
    const o = registry.open("a.csv", { content: "text" });
    const notes: number[] = [];
    const first = registry.loadViewer(o, () => notes.push(1));
    const second = registry.loadViewer(o, () => notes.push(1));
    expect(calls).toBe(1);
    const grid = { component: "grid" };
    release(grid);
    const [a, b] = await Promise.all([first, second]);
    expect(a.module).toBe(grid);
    expect(b.module).toBe(grid);
    await registry.loadViewer(o, () => notes.push(1));
    expect(calls).toBe(1);
    expect(notes).toEqual([]);
    // A failed load: the essential viewer of the content class, and one notice.
    const failing = buildRegistry(builtinSources(), (folder) =>
      folder === "tables" || folder === "images"
        ? Promise.reject(new Error("chunk lost"))
        : Promise.resolve({ component: folder }),
    );
    const text = await failing.loadViewer(failing.open("a.csv", { content: "text" }), () =>
      notes.push(1),
    );
    expect(text.viewer.id).toBe("text.editor");
    expect(text.module).toEqual({ component: "text" });
    expect(notes).toEqual([1]);
    const card = await failing.loadViewer(failing.open("fig.png"), () => notes.push(2));
    expect(card.viewer.id).toBe("files.card");
    expect(notes).toEqual([1, 2]);
    // The substitution is remembered: a later open neither reloads nor tells again.
    let tries = 0;
    const once = buildRegistry(builtinSources(), (folder) => {
      if (folder === "tables") tries++;
      return folder === "tables" ? Promise.reject(new Error("chunk lost")) : Promise.resolve({});
    });
    const said: number[] = [];
    for (let i = 0; i < 3; i++) {
      const got = await once.loadViewer(once.open("a.csv", { content: "text" }), () =>
        said.push(i),
      );
      expect(got.viewer.id).toBe("text.editor");
    }
    expect([tries, said]).toEqual([1, [0]]);
    // Entries are cached by origin too: a project's folder of the same name is another entry.
    let loads = 0;
    const viewer = {
      id: "same.view",
      fileTypes: ["same-t"],
      label: "kind.text",
      entry: "native/v.ts",
    };
    const twins = buildRegistry(
      [
        ...builtinSources(),
        {
          origin: "project",
          folder: "same",
          files: new Set<string>(),
          text: JSON.stringify({
            id: "same",
            version: "1.0.0",
            contributes: {
              fileTypes: [type("same-t", { extensions: [".same"] })],
              viewers: [viewer],
            },
          }),
        },
      ],
      () => {
        loads++;
        return Promise.resolve({});
      },
    );
    const own = twins.viewers.find((v) => v.id === "same.view");
    expect(own?.origin).toBe("project");
    if (own) {
      await twins.load(own);
      await twins.load({ ...own, origin: "builtin" });
    }
    expect(loads).toBe(2);
  });

  it("EX0 R-7 the essential extensions alone open every file", () => {
    const registry = buildRegistry(builtinSources(), counting(), {
      disabled: ["bibtex", "pdf", "data", "text"],
    });
    const ids = registry.fileTypes.map((t) => t.id);
    expect(ids).not.toContain("bibtex");
    expect(ids).not.toContain("pdf");
    expect(ids).not.toContain("json");
    // An essential extension cannot be disabled.
    expect(ids).toContain("text");
    const bib = registry.open("refs.bib", { content: "text" });
    expect([bib.type.id, bib.viewer.id]).toEqual(["text", "text.editor"]);
    expect(registry.needsContent("a.pdf")).toBe(true);
    const pdf = registry.open("a.pdf", { content: "binary" });
    expect([pdf.type.id, pdf.viewer.id, pdf.notice]).toEqual(["binary", "files.card", undefined]);
    const json = registry.open("config.json", { content: "text" });
    expect([json.type.id, json.viewer.id]).toEqual(["text", "text.editor"]);
  });

  it("EX0 R-8 one folder adds a type and its grammar", async () => {
    const fixtures = builtinSources(FIXTURE_MANIFESTS, FIXTURE_ENTRIES, FIXTURE_ROOT);
    expect(fixtures.map((s) => s.folder)).toEqual(["lab"]);
    const builtin = globLoader();
    const fixture = globLoader(FIXTURE_ENTRIES, FIXTURE_ROOT);
    const registry = buildRegistry([...builtinSources(), ...fixtures], (folder, entry) =>
      folder === "lab" ? fixture(folder, entry) : builtin(folder, entry),
    );
    expect(registry.refused).toEqual([]);
    const lab = registry.fileType("lab-notes");
    expect([lab?.icon, lab?.grammar, registry.typeOf("x.lab")?.id]).toEqual([
      "data",
      "lab",
      "lab-notes",
    ]);
    expect(registry.viewers.map((v) => v.id)).toContain("lab.view");
    expect(registry.grammar("labnotes")?.id).toBe("lab");
    // Markdown's fenced blocks are coloured through the same registry.
    const names = registry.codeLanguages().flatMap((d) => [d.name, ...d.alias]);
    expect(names).toEqual(expect.arrayContaining(["lab", "labnotes", "typst", "markdown"]));
    const desc = LanguageDescription.matchLanguageName(registry.codeLanguages(), "labnotes");
    const labSupport = await desc?.load();
    const md = await registry.languageSupport("markdown");
    expect(md && labSupport).toBeTruthy();
    if (!md || !labSupport) return;
    const doc = "# Notes\n\n```labnotes\nalpha beta\n```\n";
    const state = EditorState.create({ doc, extensions: md });
    ensureSyntaxTree(state, doc.length, 5000);
    const at = doc.indexOf("alpha") + 1;
    const inner = syntaxTree(state).resolveInner(at, 1);
    const labTop = (
      await (registry.load(registry.grammar("lab") as never) as Promise<GrammarModule>)
    )
      .grammar({ codeLanguages: [] })
      .language.parser.parse("alpha").type.name;
    const chain: string[] = [];
    for (let n: typeof inner | null = inner; n; n = n.parent) chain.push(n.type.name);
    expect(chain).toContain(labTop);
  });

  it("EX0 R-14 a type without a viewer opens in the essential viewer of its storage", () => {
    const registry = buildRegistry(
      [
        ...builtinSources(),
        extension("raw", [type("raw-blob", { extensions: [".raw"], storage: "blob" })]),
      ],
      counting(),
    );
    const bib = registry.open("refs.bib", { content: "text" });
    expect([bib.type.id, bib.viewer.id]).toEqual(["bibtex", "text.editor"]);
    const pdf = registry.open("a.pdf");
    expect([pdf.type.id, pdf.viewer.id]).toEqual(["pdf", "files.card"]);
    const raw = registry.open("x.raw");
    expect([raw.type.id, raw.viewer.id]).toEqual(["raw-blob", "files.card"]);
    const typst = registry.fileType("typst");
    expect(typst && registry.viewersFor(typst).default.id).toBe("text.editor");
  });

  it("EX0 R-15 a table offers its grid and its text without running viewer code", () => {
    const loader = counting();
    const registry = buildRegistry(builtinSources(), loader);
    const csv = registry.open("inventaire.csv", { content: "text" });
    expect(csv.viewer.id).toBe("tables.grid");
    expect(csv.alternatives.map((v) => v.id)).toEqual(["text.editor"]);
    const typst = registry.open("main.typ", { content: "text" });
    expect(typst.viewer.id).toBe("text.editor");
    expect(typst.alternatives).toEqual([]);
    const png = registry.open("fig.png");
    expect(png.alternatives).toEqual([]);
    expect(loader.calls).toBe(0);
  });
});
