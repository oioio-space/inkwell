import { ensureSyntaxTree, syntaxTree } from "@codemirror/language";
import { Compartment, EditorState } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { highlightTree, type Tag, tags } from "@lezer/highlight";
import { typstTags } from "codemirror-lang-typst/lezer";
import { describe, expect, it } from "vitest";
import { highlightStyle, languageExtension } from "./highlight";
import { forLang } from "./setup";

/** The text spans that carry the class of `tag`, in the document. */
function styled(doc: string, lang: "typst" | "markdown", ...wanted: Tag[]): string[] {
  const state = EditorState.create({ doc, extensions: languageExtension(lang) });
  const tree = ensureSyntaxTree(state, doc.length, 1000);
  if (!tree) throw new Error("no syntax tree");
  const classes = wanted.map((tag) => highlightStyle.style([tag]));
  const out: string[] = [];
  highlightTree(tree, highlightStyle, (from, to, cls) => {
    if (classes.some((c) => c && cls.split(" ").includes(c))) out.push(doc.slice(from, to));
  });
  return out;
}

describe("Typst highlighting (WASM-free Lezer parser)", () => {
  const doc = [
    "= Plan de continuité <plan>",
    '#set text(lang: "fr")',
    "Voir *gras*, _italique_ et @plan. // note",
    "+ Geler",
    '#raw("pg_ctl")',
  ].join("\n");

  it("colours headings, strings, labels, functions, comments and list markers", () => {
    expect(styled(doc, "typst", tags.heading).join("")).toContain("Plan de continuité");
    expect(styled(doc, "typst", tags.string)).toContain('"fr"');
    expect(styled(doc, "typst", tags.escape)).toEqual(expect.arrayContaining(["<plan>", "@plan"]));
    expect(styled(doc, "typst", tags.comment)).toEqual(["// note"]);
    expect(styled(doc, "typst", typstTags.listMarker)).toEqual(["+"]);
    // `#` is the interpolation mark; the called functions are coloured like it.
    expect(styled(doc, "typst", typstTags.interpolated)).toContain("#");
    expect(styled(doc, "typst", tags.function(tags.variableName))).toEqual(
      expect.arrayContaining(["text", "raw"]),
    );
  });

  it("makes strong bold and emphasis italic", () => {
    expect(styled(doc, "typst", tags.strong).join("")).toContain("gras");
    expect(styled(doc, "typst", tags.emphasis).join("")).toContain("italique");
    expect(highlightStyle.style([tags.strong])).not.toBe(highlightStyle.style([tags.emphasis]));
  });

  it("draws every colour from a CSS variable of the theme, never a hard-coded value", () => {
    const rules = highlightStyle.specs
      .map((s) => s.color)
      .filter((c): c is string => typeof c === "string");
    expect(rules.length).toBeGreaterThan(8);
    for (const c of rules) expect(c).toMatch(/^var\(--hl-[a-z]+\)$/);
  });
});

describe("Markdown highlighting (GFM)", () => {
  const doc =
    "# Titre\n\nUn **gras**, un *it*, ~~barré~~ et `code`.\n\n- item\n\n[lien](https://x.fr)";

  it("colours headings, emphasis, strikethrough, code, marks and links", () => {
    expect(styled(doc, "markdown", tags.heading).join("")).toContain("Titre");
    expect(styled(doc, "markdown", tags.strong).join("")).toContain("gras");
    expect(styled(doc, "markdown", tags.emphasis).join("")).toContain("it");
    expect(styled(doc, "markdown", tags.strikethrough).join("")).toContain("barré");
    expect(styled(doc, "markdown", tags.monospace).join("")).toContain("code");
    // Marks (#, -, **, `) are dim; a whole list is never coloured as if it were a marker.
    expect(styled(doc, "markdown", tags.processingInstruction)).toEqual(
      expect.arrayContaining(["#", "-", "`"]),
    );
    expect(styled(doc, "markdown", tags.list)).toEqual([]);
    expect(styled(doc, "markdown", tags.link, tags.url).join("")).toContain("lien");
  });
});

describe("language swap", () => {
  it("plain text has no parser; reconfiguring the compartment gives Typst one, then Markdown", () => {
    const language = new Compartment();
    let state = EditorState.create({ doc: "= Titre\n", extensions: language.of(forLang("plain")) });
    expect(syntaxTree(state).length).toBe(0);

    state = state.update({ effects: language.reconfigure(forLang("typst")) }).state;
    const typst = ensureSyntaxTree(state, state.doc.length, 1000);
    expect(typst?.topNode.name).toBe("Typst");

    state = state.update({ effects: language.reconfigure(forLang("markdown")) }).state;
    const md = ensureSyntaxTree(state, state.doc.length, 1000);
    expect(md?.topNode.name).toBe("Document");
    expect(state.doc.toString()).toBe("= Titre\n");
  });

  it("swaps the formatting keys with the language (Ctrl+U underlines in Typst only)", () => {
    const bound = (s: EditorState) =>
      s
        .facet(keymap)
        .flat()
        .some((b) => b.key === "Mod-u");
    const language = new Compartment();
    let state = EditorState.create({ doc: "a", extensions: language.of(forLang("typst")) });
    expect(bound(state)).toBe(true);
    state = state.update({ effects: language.reconfigure(forLang("markdown")) }).state;
    expect(bound(state)).toBe(false);
    state = state.update({ effects: language.reconfigure(forLang("typst")) }).state;
    expect(bound(state)).toBe(true);
  });
});
