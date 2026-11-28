import { describe, expect, it } from "vitest";
import { registry } from "../../extensions";
import { activeBlock, activeMarks } from "./active";
import { supports } from "./support";
import { MARKDOWN, PLAIN, parse, type TestLang, TYPST } from "./testkit";

const marks = (marked: string, lang: TestLang) => [...activeMarks(parse(marked), lang)].sort();

describe("the formatting language of a file", () => {
  it("reads the language from the file type's grammar", () => {
    const langOf = (path: string) => registry.format(registry.typeOf(path)?.grammar);
    expect(langOf("chapitres/intro.typ")).toBe(TYPST);
    expect(langOf("A.TYP")).toBe(TYPST);
    expect(langOf("notes.md")).toBe(MARKDOWN);
    expect(langOf("dir.d/README.markdown")).toBe(MARKDOWN);
    expect(langOf("data.csv")).toBe(PLAIN);
    expect(langOf("Makefile")).toBe(PLAIN);
    expect(langOf(".typ")).toBe(PLAIN);
    expect(langOf("a\\b\\c.md")).toBe(MARKDOWN);
  });
});

describe("supports", () => {
  it("only claims what the format can write", () => {
    expect(supports(TYPST, "underline")).toBe(true);
    expect(supports(MARKDOWN, "underline")).toBe(false);
    for (const item of [
      "attach",
      "include",
      "bibliography",
      "cite",
      "ref",
      "outline",
      "pagebreak",
    ] as const) {
      expect(supports(TYPST, item)).toBe(true);
      expect(supports(MARKDOWN, item)).toBe(false);
    }
    expect(supports(MARKDOWN, "task")).toBe(true);
    expect(supports(TYPST, "task")).toBe(false);
    expect(supports(PLAIN, "bold")).toBe(false);
    expect(supports(PLAIN, "paragraph")).toBe(false);
  });
});

describe("activeMarks", () => {
  it("FMT-19 reports nested marks at the caret", () => {
    expect(marks("*a _b‸_ c*", TYPST)).toEqual(["bold", "italic"]);
    expect(marks("*a _‸b_ c*", TYPST)).toEqual(["bold", "italic"]);
    expect(marks("*a _b_‸ c*", TYPST)).toEqual(["bold"]);
    expect(marks("*a _b_ c‸*", TYPST)).toEqual(["bold"]);
    expect(marks("*a _b_ c*‸", TYPST)).toEqual([]);
    expect(marks("**a *b‸* c**", MARKDOWN)).toEqual(["bold", "italic"]);
    expect(marks("***a‸***", MARKDOWN)).toEqual(["bold", "italic"]);
    expect(marks("~~a‸~~", MARKDOWN)).toEqual(["strike"]);
    expect(marks("#underline[a‸]", TYPST)).toEqual(["underline"]);
    expect(marks('#link("u")[a‸]', TYPST)).toEqual(["link"]);
    expect(marks("[a‸](u)", MARKDOWN)).toEqual(["link"]);
  });

  it("FMT-19 reports a selection only when all of it is inside the span", () => {
    expect(marks("*⟨a b⟩*", TYPST)).toEqual(["bold"]);
    expect(marks("*a* ⟨b *c*⟩", TYPST)).toEqual([]);
  });

  it("FMT-19 reports only code inside inline code, nothing in a formula or code block", () => {
    expect(marks("`*a‸*`", TYPST)).toEqual(["code"]);
    expect(marks("*a `b‸` c*", TYPST)).toEqual(["code"]);
    expect(marks("`**a‸**`", MARKDOWN)).toEqual(["code"]);
    expect(marks("`a‸`", MARKDOWN)).toEqual(["code"]);
    expect(marks("$ *a‸* $", TYPST)).toEqual([]);
    expect(marks("$*a‸*$", MARKDOWN)).toEqual([]);
    expect(marks("```\n**a‸**\n```", MARKDOWN)).toEqual([]);
    expect(marks("```\n*a‸*\n```", TYPST)).toEqual([]);
  });

  it("is quiet about punctuation that is not markup", () => {
    expect(marks("2 * 3 * 4‸", MARKDOWN)).toEqual([]);
    expect(marks("snake_case_na‸me", TYPST)).toEqual([]);
    expect(marks("a \\*b‸\\*", MARKDOWN)).toEqual([]);
    expect(marks("[a‸]", MARKDOWN)).toEqual([]);
    expect(marks("![a‸](p)", MARKDOWN)).toEqual([]);
  });

  it("is empty for plain text", () => {
    expect(activeMarks(parse("*a‸*"), PLAIN).size).toBe(0);
  });
});

describe("activeBlock", () => {
  const block = (marked: string, lang: TestLang) => activeBlock(parse(marked), lang);

  it("FMT-20 reads the heading level, whichever way Typst spells it", () => {
    expect(block("== x‸", TYPST)).toBe("h2");
    expect(block("#heading(level: 2)[x‸]", TYPST)).toBe("h2");
    expect(block("= x", TYPST)).toBe("h1");
    expect(block("===== deep", TYPST)).toBe("h4");
    expect(block("### x", MARKDOWN)).toBe("h3");
  });

  it("reads lists, quotes and code blocks", () => {
    expect(block("- a‸", TYPST)).toBe("ul");
    expect(block("+ a‸", TYPST)).toBe("ol");
    expect(block("1. a‸", TYPST)).toBe("ol");
    expect(block("#quote[a‸]", TYPST)).toBe("quote");
    expect(block("- a‸", MARKDOWN)).toBe("ul");
    expect(block("* a‸", MARKDOWN)).toBe("ul");
    expect(block("3. a‸", MARKDOWN)).toBe("ol");
    expect(block("- [ ] a‸", MARKDOWN)).toBe("task");
    expect(block("> a‸", MARKDOWN)).toBe("quote");
    expect(block("```js\nx‸\n```", MARKDOWN)).toBe("codeblock");
    expect(block("```\nx\n```\ny‸", MARKDOWN)).toBe("paragraph");
    expect(block("texte‸", MARKDOWN)).toBe("paragraph");
    expect(block("‸", TYPST)).toBe("paragraph");
  });

  it("is a paragraph in plain text", () => {
    expect(activeBlock(parse("= x‸"), PLAIN)).toBe("paragraph");
  });
});
