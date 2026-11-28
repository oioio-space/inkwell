import { describe, expect, it } from "vitest";
import { insert, insertPrecondition, relativePath } from "./insert";
import { apply, MARKDOWN, parse, run, TYPST } from "./testkit";

describe("insert", () => {
  it("FMT-22 attaches a file to the PDF on its own line", () => {
    const attach = insert(TYPST, "attach", { path: "data.xlsx", description: "Données brutes" });
    expect(apply("‸", attach)).toBe('#pdf.attach("data.xlsx", description: "Données brutes")‸');
    expect(apply("avant ‸ après", attach)).toBe(
      'avant \n#pdf.attach("data.xlsx", description: "Données brutes")‸\n après',
    );
    expect(apply("‸", attach)).not.toContain("pdf.embed");
  });

  it("FMT-22 puts the caret in the description when none is given", () => {
    expect(apply("‸", insert(TYPST, "attach", { path: "data.xlsx" }))).toBe(
      '#pdf.attach("data.xlsx", description: "‸")',
    );
  });

  it("FMT-22 is not offered in Markdown", () => {
    const result = run(parse("‸"), insert(MARKDOWN, "attach", { path: "data.xlsx" }));
    expect(result.ran).toBe(false);
    expect(result.dispatched).toBe(0);
  });

  it("FMT-23 includes a file, its path relative to the current file", () => {
    const at = (path: string, currentFile?: string) =>
      apply("‸", insert(TYPST, "include", { path, ...(currentFile ? { currentFile } : {}) }));
    expect(at("chapitres/ch1.typ", "chapitres/main.typ")).toBe('#include "ch1.typ"‸');
    expect(at("intro/a.typ", "chapitres/main.typ")).toBe('#include "../intro/a.typ"‸');
    expect(at("ch1.typ", "main.typ")).toBe('#include "ch1.typ"‸');
    expect(at("a/b/c.typ", "main.typ")).toBe('#include "a/b/c.typ"‸');
    expect(at("ch1.typ")).toBe('#include "ch1.typ"‸');
    expect(relativePath("a/b/c.typ", "a/x/y.typ")).toBe("../x/y.typ");
    expect(relativePath("a/b/c.typ", "a/b/c2.typ")).toBe("c2.typ");
  });

  it("FMT-24 a CSV table: linked in Typst, frozen copy in Markdown", () => {
    expect(apply("‸", insert(TYPST, "csvTable", { path: "x.csv" }))).toBe(
      '#table(..csv("x.csv").flatten())‸',
    );
    expect(apply("‸", insert(TYPST, "csvTable", { path: "x.csv", cols: 3 }))).toBe(
      '#table(columns: 3, ..csv("x.csv").flatten())‸',
    );
    const rows = [["a", "b"], ["1", "2|3"], ["x"]];
    const md = apply("‸", insert(MARKDOWN, "csvTable", { path: "x.csv", rows }));
    expect(md.split("\n")).toEqual([
      "<!-- copie figée de x.csv : à réinsérer si le fichier change -->",
      "| a | b |",
      "| --- | --- |",
      "| 1 | 2\\|3 |",
      "| x |  |‸",
    ]);
    expect(run(parse("‸"), insert(MARKDOWN, "csvTable", { path: "x.csv" })).ran).toBe(false);
  });

  it("FMT-25 cite first offers a bibliography when there is none", () => {
    const noBib = { key: "knuth84" };
    expect(insertPrecondition(TYPST, "cite", noBib)).toEqual({ offer: "bibliography" });
    const declined = run(parse("‸"), insert(TYPST, "cite", noBib));
    expect(declined.ran).toBe(false);
    expect(declined.dispatched).toBe(0);
    expect(insertPrecondition(TYPST, "cite", { ...noBib, hasBibliography: true })).toBeNull();
    expect(apply("voir ‸", insert(TYPST, "cite", { ...noBib, hasBibliography: true }))).toBe(
      "voir @knuth84‸",
    );
    expect(apply("‸", insert(TYPST, "bibliography", { path: "refs.bib" }))).toBe(
      '#bibliography("refs.bib")‸',
    );
    expect(insertPrecondition(MARKDOWN, "cite", noBib)).toBeNull();
  });

  it("FMT-26 escapes quotes, backslashes, spaces and accents in paths", () => {
    expect(apply("‸", insert(TYPST, "image", { path: 'a "b".png' }))).toBe(
      '#image("a \\"b\\".png")‸',
    );
    expect(apply("‸", insert(TYPST, "include", { path: "dossier é/mon\\fichier.typ" }))).toBe(
      '#include "dossier é/mon\\\\fichier.typ"‸',
    );
    expect(apply("‸", insert(TYPST, "attach", { path: "a.pdf", description: 'dit "oui"' }))).toBe(
      '#pdf.attach("a.pdf", description: "dit \\"oui\\"")‸',
    );
    expect(apply("‸", insert(MARKDOWN, "image", { path: "mon image (1).png" }))).toBe(
      "![](<mon image (1).png>)‸",
    );
    expect(apply("‸", insert(MARKDOWN, "image", { path: "é.png", text: "a [b]" }))).toBe(
      "![a \\[b\\]](é.png)‸",
    );
  });

  it("images, figures and data", () => {
    expect(apply("‸", insert(TYPST, "image", { path: "img/a.png", text: "Un chat" }))).toBe(
      '#image("img/a.png", alt: "Un chat")‸',
    );
    expect(apply("‸", insert(TYPST, "figure", { path: "a.png", text: "Un chat" }))).toBe(
      '#figure(image("a.png"), caption: [Un chat])‸',
    );
    expect(apply("‸", insert(TYPST, "figure", { path: "a.png" }))).toBe(
      '#figure(image("a.png"), caption: [‸])',
    );
    expect(apply("‸", insert(TYPST, "data", { path: "donnees/mes stats.json" }))).toBe(
      '#let mes-stats = json("donnees/mes stats.json")‸',
    );
    expect(apply("‸", insert(TYPST, "data", { path: "t.csv", name: "tab" }))).toBe(
      '#let tab = csv("t.csv")‸',
    );
    expect(apply("‸", insert(TYPST, "data", { path: "1.bin" }))).toBe(
      '#let data-1 = read("1.bin")‸',
    );
  });

  it("references, footnotes, outline, page break, tables, formulas", () => {
    expect(apply("‸", insert(TYPST, "ref", { key: "fig-a" }))).toBe("@fig-a‸");
    expect(apply("a‸", insert(TYPST, "footnote", { text: "note [1]" }))).toBe(
      "a#footnote[note \\[1\\]]‸",
    );
    expect(apply("a‸", insert(TYPST, "footnote"))).toBe("a#footnote[‸]");
    expect(apply("‸", insert(TYPST, "outline"))).toBe("#outline()‸");
    expect(apply("‸", insert(TYPST, "pagebreak"))).toBe("#pagebreak()‸");
    expect(apply("‸", insert(TYPST, "table", { cols: 2, rowCount: 1 }))).toBe(
      "#table(\n  columns: 2,\n  [‸], [],\n)",
    );
    expect(apply("⟨x^2⟩", insert(TYPST, "formula"))).toBe("$x^2$‸");
    expect(apply("‸", insert(MARKDOWN, "formula"))).toBe("$‸$");
    expect(apply("‸", insert(MARKDOWN, "table", { cols: 2, rowCount: 2 }))).toBe(
      "| ‸  |   |\n| --- | --- |\n|   |   |",
    );
  });

  it("links, with their text from the selection", () => {
    expect(apply("⟨texte⟩", insert(TYPST, "link", { url: "https://a.b" }))).toBe(
      '#link("https://a.b")[texte]‸',
    );
    expect(apply("‸", insert(TYPST, "link"))).toBe('#link("‸")[]');
    expect(apply("⟨texte⟩", insert(MARKDOWN, "link", { url: "https://a.b/c d" }))).toBe(
      "[texte](<https://a.b/c d>)‸",
    );
    expect(apply("⟨texte⟩", insert(MARKDOWN, "link"))).toBe("[texte](‸)");
  });

  it("Markdown footnotes get the next number and a note at the end", () => {
    expect(
      apply("un mot‸\n\n[^1]: ancienne", insert(MARKDOWN, "footnote", { text: "nouvelle" })),
    ).toBe("un mot[^2]‸\n\n[^1]: ancienne\n\n[^2]: nouvelle");
    expect(apply("un mot‸", insert(MARKDOWN, "footnote"))).toBe("un mot[^1]\n\n[^1]: ‸");
  });

  it("one insertion is one transaction", () => {
    const result = run(parse("‸"), insert(MARKDOWN, "footnote", { text: "n" }));
    expect(result.dispatched).toBe(1);
    expect(result.transactions[0]?.isUserEvent("format.insert")).toBe(true);
  });
});
