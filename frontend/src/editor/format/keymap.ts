import type { StateCommand } from "@codemirror/state";
import type { KeyBinding } from "@codemirror/view";
import { setBlock, toggleMark } from "./commands";
import { insert } from "./insert";
import type { Lang } from "./lang";
import { supports } from "./support";
import type { Feature } from "./types";

const BINDINGS: [key: string, feature: Feature, command: (lang: Lang) => StateCommand][] = [
  ["Mod-b", "bold", (l) => toggleMark(l, "bold")],
  ["Mod-i", "italic", (l) => toggleMark(l, "italic")],
  ["Mod-u", "underline", (l) => toggleMark(l, "underline")],
  ["Mod-Shift-x", "strike", (l) => toggleMark(l, "strike")],
  ["Mod-e", "code", (l) => toggleMark(l, "code")],
  ["Mod-Shift-e", "codeblock", (l) => setBlock(l, "codeblock")],
  ["Mod-k", "link", (l) => toggleMark(l, "link")],
  ["Mod-m", "formula", (l) => insert(l, "formula")],
  ["Mod-1", "h1", (l) => setBlock(l, "h1")],
  ["Mod-2", "h2", (l) => setBlock(l, "h2")],
  ["Mod-3", "h3", (l) => setBlock(l, "h3")],
  ["Mod-4", "h4", (l) => setBlock(l, "h4")],
  ["Mod-Alt-0", "paragraph", (l) => setBlock(l, "paragraph")],
];

/**
 * The shortcuts of the formatting bar, only for what the language supports: Ctrl+U is not
 * bound in Markdown (no underline), so the key stays free. Install it with `Prec.high`
 * ahead of the default keymap, which binds Mod-i to "select parent syntax".
 */
export function formatKeymap(lang: Lang): KeyBinding[] {
  return BINDINGS.filter(([, feature]) => supports(lang, feature)).map(([key, , command]) => ({
    key,
    run: command(lang),
    preventDefault: true,
  }));
}
