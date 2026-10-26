import type { DocHandle } from "@automerge/automerge-repo";
import type { SheetController } from "./sheet.svelte";

/**
 * The sheet of each open file, kept while the person looks at its text: coming back to the
 * grid finds the same model, selection, sort, filter, zoom and undo stack, so Ctrl+Z walks
 * through the actions of both views. Dropped with the handle.
 */
export const sessions = new WeakMap<DocHandle<{ text: string }>, SheetController>();
