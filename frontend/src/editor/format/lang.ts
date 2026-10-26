import { byFileName } from "../../filetypes";

/** The markup language of a file, which decides what the formatting bar can do. */
export type Lang = "typst" | "markdown" | "plain";

/**
 * The formatting profile of a file, from its file type (ADR 0027): the type says "typst" or
 * "markdown", every other file, text or not, is plain.
 */
export function langOf(path: string): Lang {
  return byFileName(path)?.formatProfile ?? "plain";
}
