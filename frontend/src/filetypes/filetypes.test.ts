import { afterEach, describe, expect, it, vi } from "vitest";
import en from "../i18n/en.json";
import fr from "../i18n/fr.json";
import { FILE_MARKS } from "../ui/icons/paths";
import {
  all,
  BINARY,
  byFileName,
  type FileType,
  get,
  PLAIN,
  register,
  resolve,
  unregister,
} from "./index";

const name = (path: string, extra: Partial<Parameters<typeof resolve>[0]> = {}) =>
  resolve({ name: path, ...extra }).id;

describe("resolution: override, extension, bytes, plain text, binary", () => {
  it("an ordinary text file opens as plain text, with no bar and no format", () => {
    for (const f of ["notes.txt", "README", "LICENSE", "readme", "unknown.xyz123"]) {
      const t = resolve({ name: f, isText: true });
      expect(t.formatProfile, f).toBeUndefined();
      expect(t.binary, f).toBeFalsy();
    }
    expect(name("README", { isText: true })).toBe(PLAIN);
    expect(name("whatever.zzz", { isText: true })).toBe(PLAIN);
  });

  it("knows the usual text and code files by extension or by name", () => {
    const cases: Record<string, string> = {
      "a.log": "log",
      "a.ini": "config",
      "x.conf": "config",
      "x.cfg": "config",
      ".gitignore": "config",
      ".env.example": "config",
      "a.json": "json",
      "a.yaml": "yaml",
      "a.yml": "yaml",
      "a.toml": "toml",
      "a.xml": "xml",
      "a.html": "html",
      "a.css": "css",
      "a.js": "javascript",
      "a.ts": "typescript",
      "a.sh": "shell",
      "a.py": "python",
      "a.go": "go",
      "a.sql": "sql",
      Makefile: "code",
      "a.typ": "typst",
      "a.MD": "markdown",
      "a.csv": "csv",
      "a.tsv": "tsv",
      "a.bib": "bibtex",
      "a.png": "image",
      "a.pdf": "pdf",
      "a.ttf": "font",
      "a.zip": "archive",
      "a.exe": BINARY,
    };
    for (const [file, id] of Object.entries(cases)) expect(name(file), file).toBe(id);
  });

  it("a text file never gets a viewer, even if its name says png", () => {
    expect(name("odd.png", { isText: true })).toBe(PLAIN);
    expect(name("odd.pdf", { isText: true })).toBe(PLAIN);
  });

  it("a file that is not text and not known opens in the binary viewer", () => {
    const t = resolve({ name: "mystery", isText: false });
    expect(t.id).toBe(BINARY);
    expect(t.binary).toBe(true);
  });

  it("with only bytes: sniffing recognises a PNG, a PDF, a zip; text is plain; the rest is binary", () => {
    const bytes = (...b: number[]) => Uint8Array.from([...b, ...new Array(16).fill(0)]);
    expect(name("blob", { bytes: bytes(0x89, 0x50, 0x4e, 0x47) })).toBe("image");
    expect(name("blob", { bytes: bytes(0x25, 0x50, 0x44, 0x46) })).toBe("pdf");
    expect(name("blob", { bytes: bytes(0x50, 0x4b, 3, 4) })).toBe("archive");
    expect(name("blob", { bytes: new TextEncoder().encode("just words") })).toBe(PLAIN);
    expect(name("blob", { bytes: Uint8Array.from([1, 2, 0, 3, 4, 0, 255]) })).toBe(BINARY);
  });

  it("the extension wins over the bytes, and the explicit choice wins over both", () => {
    const png = Uint8Array.from([0x89, 0x50, 0x4e, 0x47, 0]);
    expect(name("x.txt", { bytes: png })).toBe(PLAIN);
    expect(name("x.txt", { override: "json", isText: true })).toBe("json");
    expect(name("x.png", { override: "pdf" })).toBe("pdf");
    expect(name("x.txt", { override: "no-such-type", isText: true })).toBe(PLAIN);
  });

  it("TSV is not CSV, and the sheet opens tables as text until it exists (B9)", async () => {
    expect(get("tsv")?.icon).toBe("tsv");
    expect(get("csv")?.icon).toBe("csv");
    expect(get("tsv")?.dialect?.delimiter).toBe("\t");
    const module = await get("csv")?.open();
    expect(typeof module?.default.mount).toBe("function");
  });
});

describe("a new type needs no edit outside its module", () => {
  afterEach(() => unregister("fake"));

  it("registering a fake type is all it takes: it resolves, opens lazily and has an icon", async () => {
    const mount = vi.fn(() => ({ destroy() {}, focus() {}, selection: () => undefined }));
    const open = vi.fn(async () => ({ default: { mount } }));
    const fake: FileType = {
      id: "fake",
      label: "kind.code",
      icon: "code",
      extensions: [".fake"],
      mime: ["application/x-fake"],
      open,
    };
    register(fake);
    expect(name("thing.FAKE")).toBe("fake");
    expect(byFileName("dir/thing.fake")).toBe(fake);
    expect(all()).toContain(fake);
    // Nothing is loaded until a file of the type is opened.
    expect(open).not.toHaveBeenCalled();
    const module = await resolve({ name: "x.fake" }).open();
    expect(module.default.mount).toBe(mount);
    unregister("fake");
    expect(name("thing.fake", { isText: true })).toBe(PLAIN);
  });

  it("a type that takes an extension another one has is refused, so modules never fight silently", () => {
    const clash: FileType = {
      id: "clash",
      label: "kind.code",
      icon: "code",
      extensions: [".json"],
      mime: [],
      open: async () => {
        throw new Error("never");
      },
    };
    expect(() => register(clash)).toThrow(/already json/);
    expect(get("clash")).toBeUndefined();
    expect(() => register({ ...clash, id: "json", extensions: [".zzz"] })).toThrow(
      /already registered/,
    );
  });
});

describe("lazy loading: a type's editor and language load when a file of it opens", () => {
  it("every descriptor's open and language are functions that have not run at registration", () => {
    for (const t of all()) {
      expect(typeof t.open, t.id).toBe("function");
      if (t.language) expect(typeof t.language, t.id).toBe("function");
    }
  });

  it("the languages of the code types load and give an extension", async () => {
    for (const id of [
      "json",
      "yaml",
      "toml",
      "xml",
      "html",
      "css",
      "javascript",
      "typescript",
      "python",
      "go",
      "shell",
      "sql",
    ]) {
      const ext = await get(id)?.language?.();
      expect(ext, id).toBeDefined();
    }
    expect(get("code")?.language).toBeUndefined();
  });
});

describe("legend: every type has an icon with a name, in both languages", () => {
  it("each descriptor's icon exists and its label says what it is", () => {
    for (const t of all()) {
      expect(t.icon in FILE_MARKS, `${t.id} icon ${t.icon}`).toBe(true);
      expect(t.label in fr && t.label in en, `${t.id} label ${t.label}`).toBe(true);
      expect(
        `kind.${t.icon}` in fr && `kind.${t.icon}` in en,
        `${t.id} aria-label kind.${t.icon}`,
      ).toBe(true);
      expect(t.extensions.length + (t.filenames?.length ?? 0), t.id).toBeGreaterThan(0);
    }
  });

  it("every icon of the set has a name for assistive technology", () => {
    for (const mark of Object.keys(FILE_MARKS)) {
      expect(`kind.${mark}` in fr, `kind.${mark}`).toBe(true);
    }
  });

  it("covers the families asked for", () => {
    const icons = new Set(all().map((t) => t.icon));
    for (const want of [
      "typst",
      "markdown",
      "text",
      "log",
      "csv",
      "tsv",
      "json",
      "yaml",
      "config",
      "xml",
      "code",
      "js",
      "ts",
      "py",
      "go",
      "shell",
      "bib",
      "image",
      "pdf",
      "font",
      "archive",
      "binary",
    ]) {
      expect(icons.has(want as never), want).toBe(true);
    }
  });
});
