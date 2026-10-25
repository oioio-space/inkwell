export type Mark = "bold" | "italic" | "underline" | "strike" | "code" | "link";

export type Block =
  | "h1"
  | "h2"
  | "h3"
  | "h4"
  | "paragraph"
  | "ul"
  | "ol"
  | "task"
  | "quote"
  | "codeblock";

/** What the Insert menu offers. `formula` is the Ctrl+M action. */
export type InsertItem =
  | "image"
  | "figure"
  | "include"
  | "attach"
  | "csvTable"
  | "data"
  | "bibliography"
  | "cite"
  | "ref"
  | "footnote"
  | "link"
  | "outline"
  | "pagebreak"
  | "table"
  | "formula";

/** Anything the bar can ask whether a format handles. */
export type Feature = Mark | Block | InsertItem;

/** The inputs of an insertion; each item reads the fields it needs. */
export interface InsertArgs {
  /** Project path of the file to reference (image, include, attach, csvTable, data, bibliography). */
  path?: string;
  /** Project path of the file being edited: `path` is written relative to it. */
  currentFile?: string;
  /** Alt text, caption, footnote text or link text. */
  text?: string;
  /** Link target. */
  url?: string;
  /** Description of an attached file. */
  description?: string;
  /** Citation key or cross-reference label. */
  key?: string;
  /** Whether the project already has a bibliography (cite needs one). */
  hasBibliography?: boolean;
  /** Cells of the CSV file, for the frozen Markdown copy. */
  rows?: readonly (readonly string[])[];
  /** Name of the variable that holds loaded data. */
  name?: string;
  /** Size of an empty table. */
  cols?: number;
  rowCount?: number;
}
