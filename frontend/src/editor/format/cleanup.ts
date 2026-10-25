import type { StateCommand } from "@codemirror/state";
import { fencesUpTo, isInFence } from "./fence";
import type { Lang } from "./lang";
import { isFence, syntaxOf } from "./support";

// After two people edit concurrently, a delimiter pair can be left around nothing: A bolds a
// word, B deletes it, and `**` + `**` remain side by side. The rule is "empty pairs are
// removed". Only pairs that are unambiguous are touched; Markdown's empty italic `**` cannot
// be told from a stray literal and is left alone.

const EMPTY_PAIRS: Record<Exclude<Lang, "plain">, RegExp[]> = {
  typst: [/#(?:underline|strike|strong|emph)\[\]/g, /(?<!\\)\*\*/g, /(?<!\\)__/g, /(?<!`)``(?!`)/g],
  markdown: [/(?<!\\)\*\*\*\*/g, /(?<!\\)~~~~/g, /(?<!`)``(?!`)/g],
};

function cleanLine(line: string, lang: Exclude<Lang, "plain">): string {
  const scan = syntaxOf(lang)?.scan;
  let text = line;
  for (let round = 0; round < 8; round++) {
    const verbatim = scan?.(text).verbatim ?? [];
    let changed = false;
    for (const re of EMPTY_PAIRS[lang]) {
      text = text.replace(re, (m, offset: number) => {
        if (verbatim.some(([a, b]) => a <= offset && offset < b)) return m;
        changed = true;
        return "";
      });
    }
    if (!changed) break;
  }
  return text;
}

/** The text with every empty delimiter pair removed (lines of fenced code are left alone). */
export function removeEmptyDelimiters(text: string, lang: Lang): string {
  if (lang === "plain") return text;
  const lines = text.split("\n");
  const inFence = new Set<number>();
  let open = false;
  lines.forEach((l, i) => {
    const fence = isFence(l);
    if (fence || open) inFence.add(i);
    if (fence) open = !open;
  });
  return lines.map((l, i) => (inFence.has(i) ? l : cleanLine(l, lang))).join("\n");
}

/** The convergence cleanup as a command: one transaction per run, nothing when already clean. */
export function cleanupDelimiters(lang: Lang): StateCommand {
  return ({ state, dispatch }) => {
    if (lang === "plain" || state.readOnly) return false;
    const fences = fencesUpTo(state.doc, state.doc.lines);
    const changes: { from: number; to: number; insert: string }[] = [];
    for (let n = 1; n <= state.doc.lines; n++) {
      if (isInFence(fences, state.doc, n)) continue;
      const line = state.doc.line(n);
      const cleaned = cleanLine(line.text, lang);
      if (cleaned !== line.text) changes.push({ from: line.from, to: line.to, insert: cleaned });
    }
    if (changes.length === 0) return false;
    dispatch(state.update({ changes, userEvent: "format.cleanup" }));
    return true;
  };
}
