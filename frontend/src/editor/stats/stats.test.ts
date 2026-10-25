import { EditorSelection, EditorState } from "@codemirror/state";
import { describe, expect, it } from "vitest";
import { cursorPosition } from "./position";
import { formatCount, stats } from "./stats";
import { countWords } from "./words";

const typst = (s: string) => countWords(s, "typst");
const md = (s: string) => countWords(s, "markdown");
const plain = (s: string) => countWords(s, "plain");

describe("stats", () => {
  it("STA-01 l'astreinte compte pour un mot", () => {
    expect(plain("l'astreinte")).toBe(1);
    expect(plain("l’astreinte")).toBe(1);
  });

  it("STA-02 peut-être compte pour un mot", () => {
    expect(plain("peut-être")).toBe(1);
    expect(plain("peut - être")).toBe(2);
  });

  it("STA-03 aujourd'hui et c'est-à-dire", () => {
    expect(plain("aujourd'hui")).toBe(1);
    expect(plain("c'est-à-dire")).toBe(1);
  });

  it("STA-04 nombres 2026-114 (1) et 8 h (2)", () => {
    expect(plain("2026-114")).toBe(1);
    expect(plain("8 h")).toBe(2);
    expect(plain("3,14")).toBe(1);
  });

  it("STA-05 Typst: balisage non compté, texte compté", () => {
    expect(typst("*gras*")).toBe(1);
    expect(typst("#strong[x]")).toBe(1);
    expect(typst("#text(fill: red)[deux mots] fin")).toBe(3);
    expect(typst("= Titre")).toBe(1);
    expect(typst("_a_ et #emph[b]")).toBe(3);
  });

  it("STA-06 Typst: code, bloc, formule, commentaire, étiquette, renvoi valent 0 mot", () => {
    expect(typst("`x y`")).toBe(0);
    expect(typst("```py\nx = 1\nprint(x)\n```")).toBe(0);
    expect(typst("$a + b$")).toBe(0);
    expect(typst("// x y z")).toBe(0);
    expect(typst("/* a b */")).toBe(0);
    expect(typst("<intro> @fig-1")).toBe(0);
    expect(typst('#set text(lang: "fr")\n#let x = 1\nbonjour')).toBe(1);
    expect(typst("avant // reste\naprès")).toBe(2);
    expect(typst("voir https://exemple.fr ici")).toBeGreaterThan(2);
  });

  it("STA-07 Markdown: seul le texte d'un lien est compté", () => {
    expect(md("[texte](https://exemple.fr/a/b)")).toBe(1);
    expect(md("![image alt](a.png)")).toBe(0);
    expect(md("**gras** et _italique_")).toBe(3);
    expect(md("`code span` mot")).toBe(1);
    expect(md("```\nfn main() {}\n```\nmot")).toBe(1);
    expect(md("<!-- caché --> visible")).toBe(1);
    expect(md("1. premier\n2. second")).toBe(2);
  });

  it("STA-08 CJK compté par le segmenteur, emoji à 0", () => {
    expect(plain("🙂 👩‍👩‍👧")).toBe(0);
    expect(plain("今日は天気")).toBeGreaterThanOrEqual(2);
    expect(plain("今日は天気")).toBe(
      [...new Intl.Segmenter("fr", { granularity: "word" }).segment("今日は天気")].filter(
        (s) => s.isWordLike,
      ).length,
    );
  });

  it("STA-09 la colonne avance de 1 après é composé et après 👩‍👩‍👧", () => {
    const col = (doc: string) =>
      cursorPosition(EditorState.create({ doc, selection: EditorSelection.single(doc.length) }))
        .column;
    expect(col("é")).toBe(2);
    expect(col("👩‍👩‍👧")).toBe(2);
    expect(col("")).toBe(1);
    const state = EditorState.create({ doc: "ab\ncd", selection: EditorSelection.single(4) });
    expect(cursorPosition(state)).toEqual({ line: 2, column: 2 });
  });

  it("STA-10 texte vide: tout à 0 et « 0 mot »", () => {
    expect(stats("", "typst")).toEqual({
      words: 0,
      chars: 0,
      charsNoSpaces: 0,
      sentences: 0,
      paragraphs: 0,
      readingMinutes: 0,
    });
    expect(formatCount(0, "mot")).toBe("0 mot");
  });

  it("STA-11 sélection: statistiques en plus de celles du document", () => {
    const text = "Un deux. Trois quatre cinq.\n\nSix.";
    const s = stats(text, "plain", { from: 0, to: 8 });
    expect(s.words).toBe(6);
    expect(s.sentences).toBe(3);
    expect(s.paragraphs).toBe(2);
    expect(s.chars).toBe(text.length);
    expect(s.charsNoSpaces).toBe(text.replace(/\s/g, "").length);
    expect(s.selection?.words).toBe(2);
    expect(s.selection?.chars).toBe(8);
    expect(stats(text, "plain", { from: 3, to: 3 }).selection).toBeUndefined();
    expect(stats(text, "plain", { from: 8, to: 0 }).selection?.words).toBe(2);
    expect(stats("mot ".repeat(460), "plain").readingMinutes).toBe(2);
  });

  it("STA-12 5 Mo comptés en moins de 150 ms", () => {
    const unit = "Le *petit* chat, c'est-à-dire aujourd'hui, boit du lait 2026-114. ";
    const text = unit.repeat(Math.ceil((5 * 1024 * 1024) / unit.length));
    const t0 = performance.now();
    const n = countWords(text, "plain");
    const ms = performance.now() - t0;
    expect(n).toBe(Math.ceil((5 * 1024 * 1024) / unit.length) * 9);
    console.info(`STA-12 countWords(5 MB) = ${ms.toFixed(0)} ms`);
    // The budget is asserted only on request (INKWELL_PERF=1): a wall-clock limit fails
    // on a loaded machine, e.g. when the pre-push hook runs every suite in parallel.
    const env = (globalThis as { process?: { env?: Record<string, string | undefined> } }).process
      ?.env;
    if (env?.INKWELL_PERF === "1") expect(ms).toBeLessThan(150);
  });

  it("STA-13 pluriels de l'affichage", () => {
    expect(formatCount(0, "mot")).toBe("0 mot");
    expect(formatCount(1, "mot")).toBe("1 mot");
    expect(formatCount(2, "mot")).toBe("2 mots");
    expect(formatCount(1284, "mot")).toBe("1 284 mots");
    expect(formatCount(2, "caractère")).toBe("2 caractères");
  });
});
