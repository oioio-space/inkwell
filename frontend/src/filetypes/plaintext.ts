import { register } from "./registry";

const open = () => import("./editors/text");

register({
  id: "plaintext",
  label: "kind.text",
  icon: "text",
  extensions: [".txt", ".text"],
  filenames: [
    "readme",
    "license",
    "licence",
    "copying",
    "notice",
    "changelog",
    "authors",
    "contributors",
    "todo",
  ],
  mime: ["text/plain"],
  open,
});

register({
  id: "log",
  label: "kind.log",
  icon: "log",
  extensions: [".log", ".out"],
  mime: ["text/x-log"],
  wordCount: "none",
  open,
});

register({
  id: "config",
  label: "kind.config",
  icon: "config",
  extensions: [".ini", ".conf", ".cfg", ".env", ".properties"],
  filenames: [
    ".gitignore",
    ".gitattributes",
    ".editorconfig",
    ".dockerignore",
    ".env.example",
    ".npmrc",
    ".prettierrc",
  ],
  mime: ["text/x-ini"],
  wordCount: "none",
  open,
});

register({
  id: "bibtex",
  label: "kind.bib",
  icon: "bib",
  extensions: [".bib", ".bibtex"],
  mime: ["text/x-bibtex"],
  wordCount: "none",
  open,
});
