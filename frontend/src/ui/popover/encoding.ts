import { Eol, type FileInfo } from "../../store/files";

const NAMES: Record<string, string> = {
  "utf-8": "UTF-8",
  "utf-16le": "UTF-16 LE",
  "utf-16be": "UTF-16 BE",
  "windows-1252": "Windows-1252",
};

/** The encoding as people write it ("Windows-1252"). */
export const encodingLabel = (encoding: string): string =>
  NAMES[encoding] ?? encoding.toUpperCase();

const EOL_NAMES: Record<Eol, string> = { [Eol.LF]: "LF", [Eol.CRLF]: "CRLF", [Eol.CR]: "CR" };

export const eolLabel = (eol: Eol): string => EOL_NAMES[eol];

/** The status bar only mentions the encoding when it is not UTF-8 with LF line ends. */
export const needsIndicator = (info: FileInfo): boolean =>
  info.encoding !== "utf-8" || info.eol !== Eol.LF;
