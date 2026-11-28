import type { EditorState, Text } from "@codemirror/state";
import type { MessageKey } from "../../i18n";
import type { LangSyntax } from "./syntax";
import type { InsertArgs, InsertItem } from "./types";

/** What an Insert item writes, before it is placed in the document. */
export interface Rendered {
  text: string;
  /** Block items get a line of their own. */
  block: boolean;
  /** Where the caret lands, in the rendered text (default: the end). */
  cursor?: number;
  /** Text appended at the end of the document (Markdown footnote definitions). */
  appendix?: string;
  /** The caret goes after the appended text rather than after the inserted one. */
  caretInAppendix?: boolean;
}

/** What a renderer knows besides the item's own inputs. */
export interface RenderContext {
  /** The selected text, which an inline item wraps or replaces. */
  readonly selected: string;
  /** The picked file, relative to the file being edited. */
  readonly rel: string;
  readonly doc: Text;
  /** The language's reader of the picked file (from its manifest's `readers`), else `read`. */
  readonly reader: string;
}

export type Edit = { from: number; to: number; insert: string };

/** A language whose quotes are one call around a run of lines (Typst's `#quote[…]`). */
export interface QuoteCall {
  /** Wraps each run of consecutive `lines` in one call. */
  wrap(state: EditorState, lines: readonly number[], edits: Edit[]): void;
  /** Where the call opened just before `from` on line `lineNo` closes; -1 when it does not. */
  closing(doc: Text, lineNo: number, from: number): number;
}

/**
 * What a grammar's format entry exports (ADR 0027, EX0c3): everything the editor once keyed by
 * the language's name. A language added later gets the bar, the Insert menu, the cleanup and the
 * word count by contributing one.
 */
export interface FormatModule {
  readonly syntax: LangSyntax;
  /** Empty delimiter pairs the convergence cleanup removes (only unambiguous ones). */
  readonly emptyPairs: readonly RegExp[];
  readonly quote?: QuoteCall | undefined;
  /** The prose of a text, its markup removed (the word count). */
  strip(text: string): string;
  /** The text an Insert item writes; null when an input is missing or the item is not the language's. */
  render(item: InsertItem, args: InsertArgs, ctx: RenderContext): Rendered | null;
  /** What must exist before an item makes sense, and the item that provides it (cite → bibliography). */
  precondition?: ((item: InsertItem, args: InsertArgs) => { offer: InsertItem } | null) | undefined;
  /** The items written from the picked CSV's rows (Markdown's frozen copy): the menu reads them first. */
  readonly needsRows?: ReadonlySet<InsertItem> | undefined;
  /** What each Insert item writes, shown beside it in the menu. */
  readonly insertSyntax: Readonly<Partial<Record<InsertItem, string>>>;
  /** An item's description when it differs in this language (Markdown's frozen CSV copy). */
  readonly describe?: Readonly<Partial<Record<InsertItem, MessageKey>>> | undefined;
  /** A note under the Insert menu (what the language cannot do). */
  readonly note?: MessageKey | undefined;
}

/**
 * The formatting language of a file: its grammar's format entry, with its manifest's data.
 * `undefined` is plain text (no bar, no Insert menu).
 */
export interface Lang extends FormatModule {
  /** The grammar's id (opaque: never compared to a name). */
  readonly id: string;
  /** The language's name, the grammar's `label` (an i18n key). */
  readonly label: string;
  /** The file types an Insert item lists; an item absent takes every file. */
  readonly accepts: Readonly<Partial<Record<InsertItem, readonly string[]>>>;
  /** The data reader of a file type (Typst's `csv`, `json`, …). */
  readonly readers: Readonly<Record<string, string>>;
}
