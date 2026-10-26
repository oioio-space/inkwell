import type { DocHandle } from "@automerge/automerge-repo";
import type { EditorState, Extension } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import type { MessageKey } from "../i18n";
import type { FileMark } from "../ui/icons/paths";

/**
 * File types are data (ADR 0027): a descriptor says what a kind of file is called, how it is
 * recognised, what it looks like in the list, and how it opens. The editor, the icons, the
 * formatting bar and the word count read descriptors; none of them knows a type by name.
 * Adding a type is one module that calls `register()`.
 */

/** The formatting profile of a type that has a bar (A1): the syntax tables it uses. */
export type FormatProfile = "typst" | "markdown";

/** A line the editor adds to the status bar. */
export interface StatusItem {
  id: string;
  text: string;
}

/** What a module is given to open a file. */
export interface EditorContext {
  /** The document of a text file; absent for a file that is not text. */
  handle?: DocHandle<{ text: string }>;
  me: { id: string; name: string };
  type: FileType;
  /** The file's name, size in bytes and path in the project. */
  name: string;
  path: string;
  size: number;
  focus: boolean;
  /** Where to put the caret when the editor opens. */
  at?: number;
  /** Asked before Ctrl+Z: true if the key was used (an outside edit taken back). */
  onUndoKey?: () => boolean;
  onPeople?: (people: { peerId: string; name: string }[]) => void;
  onView?: (view: EditorView) => void;
  onState?: (state: EditorState) => void;
  /** Hands the file to the system's own program for it. */
  openWithSystem?: () => void;
}

/** A mounted editor: what the shell can ask of it. */
export interface EditorInstance {
  destroy(): void;
  focus(): void;
  /** The selection in the document, if the module has one. */
  selection(): { from: number; to: number } | undefined;
  rename?(name: string): void;
  /** The file was renamed to another type: reconfigure in place, keeping caret and history. */
  setType?(type: FileType): void;
  statusItems?(): StatusItem[];
}

/** One small interface every editor module implements. */
export interface EditorModule {
  mount(host: HTMLElement, ctx: EditorContext): EditorInstance;
}

export interface FileType {
  /** Stable, lower case: it is what an index node's `ftype` names. */
  id: string;
  /** The name shown, as an i18n key. */
  label: MessageKey;
  /** The icon in the list, a key of FILE_MARKS. */
  icon: FileMark;
  /** Extensions with their dot, lower case. */
  extensions: readonly string[];
  /** Whole names for files that have no extension, lower case (readme, makefile, .gitignore). */
  filenames?: readonly string[];
  mime: readonly string[];
  /** How sure the bytes make us that they are this type, from 0 to 1. Optional. */
  sniff?(bytes: Uint8Array): number;
  /** The editor module, loaded when the first file of the type is opened. */
  open: () => Promise<{ default: EditorModule }>;
  /** A CodeMirror language, loaded with the editor. */
  language?: () => Promise<Extension>;
  /** The formatting bar and keys of A1. */
  formatProfile?: FormatProfile;
  /** How words are counted (A2); "none" hides the count. Default: "prose". */
  wordCount?: FormatProfile | "prose" | "none";
  /** A live preview (C2 for Typst, B8 for Markdown): the id of the previewer, wired by its task. */
  preview?: string;
  /** Items the Insert menu offers (ids of A1's insert items). */
  insertItems?: readonly string[];
  /** For a table: how the cells are cut (B9). */
  dialect?: { delimiter: string };
  /** A file that is not text: the viewer, never the text editor. */
  binary?: boolean;
}
