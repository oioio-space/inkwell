import { type FileType, resolve } from "../filetypes";
import { FileKind } from "../store/files";
import type { Entry } from "./materialize";
import { splitExt } from "./names";

/** The kinds Go gives files by extension (internal/project/files.go), for the icons. */
const BY_EXT: Record<string, FileKind> = {
  ".typ": FileKind.typst,
  ".md": FileKind.markdown,
  ".markdown": FileKind.markdown,
  ".txt": FileKind.text,
  ".csv": FileKind.csv,
  ".tsv": FileKind.csv,
  ".bib": FileKind.bib,
  ".pdf": FileKind.pdf,
  ".png": FileKind.image,
  ".jpg": FileKind.image,
  ".jpeg": FileKind.image,
  ".gif": FileKind.image,
  ".webp": FileKind.image,
  ".svg": FileKind.image,
  ".bmp": FileKind.image,
  ".json": FileKind.data,
  ".yaml": FileKind.data,
  ".yml": FileKind.data,
  ".toml": FileKind.data,
  ".xml": FileKind.data,
  ".cbor": FileKind.data,
};

/** The name of a kind, the word the interface catalogs use ("kind.pdf"). */
export function kindName(kind: FileKind) {
  switch (kind) {
    case FileKind.typst:
      return "typst";
    case FileKind.markdown:
      return "markdown";
    case FileKind.text:
      return "text";
    case FileKind.csv:
      return "csv";
    case FileKind.bib:
      return "bib";
    case FileKind.image:
      return "image";
    case FileKind.pdf:
      return "pdf";
    case FileKind.data:
      return "data";
    default:
      return "other";
  }
}

/** What a file is, from its name; a text file of an unknown name is plain text. */
export function kindOf(name: string, isText: boolean): FileKind {
  const known = BY_EXT[splitExt(name).ext.toLowerCase()];
  if (known !== undefined) return known;
  return isText ? FileKind.text : FileKind.other;
}

/** The file type of an entry (ADR 0027): the choice kept in the index, else name, else content kind. */
export function typeOfEntry(e: Entry): FileType {
  return resolve({ name: e.name, override: e.ftype, isText: e.type === "text" });
}

/** The kind of an entry of the tree (not meaningful for a folder). */
export const kindOfEntry = (e: Entry): FileKind => kindOf(e.name, e.type === "text");
