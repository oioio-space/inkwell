import { LanguageSupport } from "@codemirror/language";
import { typst_lezer } from "codemirror-lang-typst/lezer";

/**
 * Typst through the package's Lezer parser, made incremental by our pinned patch
 * (frontend/patches/). The CSP would allow WASM ('wasm-unsafe-eval', for Automerge), but the
 * package's WASM entry gives no syntax tree and repaints the whole file's colours on the main
 * thread. Only the language is used: its bundled colours, Enter/indent services and linter would
 * change how typing behaves.
 */
export function grammar(): LanguageSupport {
  return new LanguageSupport(typst_lezer().language);
}
