import type { Extension } from "@codemirror/state";
import { register } from "./registry";
import type { FileType } from "./types";

const open = () => import("./editors/text");

/** The palette of the editor with a language; all of it loads only when a file of the type opens. */
async function coloured(language: () => Promise<Extension>): Promise<Extension> {
  const [lang, { syntaxHighlighting }, { highlightStyle }] = await Promise.all([
    language(),
    import("@codemirror/language"),
    import("../editor/highlight"),
  ]);
  return [lang, syntaxHighlighting(highlightStyle)];
}

type Mode = Parameters<typeof import("@codemirror/language").StreamLanguage.define>[0];

/** A stream-parser mode of `@codemirror/legacy-modes`, as a language. */
async function legacy(load: () => Promise<Mode>): Promise<Extension> {
  const [{ StreamLanguage }, mode] = await Promise.all([import("@codemirror/language"), load()]);
  return StreamLanguage.define(mode);
}

type Code = Omit<FileType, "open" | "wordCount" | "binary" | "language"> & {
  lang?: () => Promise<Extension>;
};

function code({ lang, ...type }: Code): void {
  register({
    ...type,
    wordCount: "none",
    open,
    ...(lang ? { language: () => coloured(lang) } : {}),
  });
}

code({
  id: "json",
  label: "kind.json",
  icon: "json",
  extensions: [".json", ".jsonc", ".webmanifest", ".geojson"],
  mime: ["application/json"],
  lang: async () => (await import("@codemirror/lang-json")).json(),
});
code({
  id: "yaml",
  label: "kind.yaml",
  icon: "yaml",
  extensions: [".yaml", ".yml"],
  mime: ["application/yaml"],
  lang: async () => (await import("@codemirror/lang-yaml")).yaml(),
});
code({
  id: "toml",
  label: "kind.config",
  icon: "config",
  extensions: [".toml"],
  mime: ["application/toml"],
  lang: () => legacy(async () => (await import("@codemirror/legacy-modes/mode/toml")).toml),
});
code({
  id: "xml",
  label: "kind.xml",
  icon: "xml",
  extensions: [".xml", ".xsd", ".xsl", ".plist", ".rss"],
  mime: ["application/xml", "text/xml"],
  lang: async () => (await import("@codemirror/lang-xml")).xml(),
});
code({
  id: "html",
  label: "kind.xml",
  icon: "xml",
  extensions: [".html", ".htm", ".xhtml"],
  mime: ["text/html"],
  lang: async () => (await import("@codemirror/lang-html")).html(),
});
code({
  id: "css",
  label: "kind.code",
  icon: "code",
  extensions: [".css", ".scss"],
  mime: ["text/css"],
  lang: async () => (await import("@codemirror/lang-css")).css(),
});
code({
  id: "javascript",
  label: "kind.js",
  icon: "js",
  extensions: [".js", ".mjs", ".cjs", ".jsx"],
  mime: ["text/javascript"],
  lang: async () => (await import("@codemirror/lang-javascript")).javascript({ jsx: true }),
});
code({
  id: "typescript",
  label: "kind.ts",
  icon: "ts",
  extensions: [".ts", ".mts", ".cts", ".tsx"],
  mime: ["text/typescript"],
  lang: async () =>
    (await import("@codemirror/lang-javascript")).javascript({ typescript: true, jsx: true }),
});
code({
  id: "python",
  label: "kind.py",
  icon: "py",
  extensions: [".py", ".pyw"],
  mime: ["text/x-python"],
  lang: async () => (await import("@codemirror/lang-python")).python(),
});
code({
  id: "go",
  label: "kind.go",
  icon: "go",
  extensions: [".go"],
  mime: ["text/x-go"],
  lang: async () => (await import("@codemirror/lang-go")).go(),
});
code({
  id: "shell",
  label: "kind.shell",
  icon: "shell",
  extensions: [".sh", ".bash", ".zsh"],
  mime: ["application/x-sh"],
  lang: () => legacy(async () => (await import("@codemirror/legacy-modes/mode/shell")).shell),
});
code({
  id: "sql",
  label: "kind.code",
  icon: "code",
  extensions: [".sql"],
  mime: ["application/sql"],
  lang: async () => (await import("@codemirror/lang-sql")).sql(),
});
// Other code: the icon of code and plain colours, until someone adds a language.
code({
  id: "code",
  label: "kind.code",
  icon: "code",
  extensions: [
    ".c",
    ".h",
    ".cpp",
    ".rs",
    ".java",
    ".rb",
    ".php",
    ".lua",
    ".tex",
    ".diff",
    ".patch",
  ],
  filenames: ["makefile", "dockerfile"],
  mime: ["text/x-source"],
});
