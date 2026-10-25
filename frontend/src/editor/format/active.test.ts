import { describe, expect, it } from "vitest";
import { activeBlock, activeMarks } from "./active";
import { langOf } from "./lang";
import { supports } from "./support";
import { parse } from "./testkit";

const marks = (marked: string, lang: "typst" | "markdown") =>
  [...activeMarks(parse(marked), lang)].sort();

describe("langOf", () => {
  it("reads the language from the extension", () => {
    expect(langOf("chapitres/intro.typ")).toBe("typst");
    expect(langOf("A.TYP")).toBe("typst");
    expect(langOf("notes.md")).toBe("markdown");
    expect(langOf("dir.d/README.markdown")).toBe("markdown");
    expect(langOf("data.csv")).toBe("plain");
    expect(langOf("Makefile")).toBe("plain");
    expect(langOf(".typ")).toBe("plain");
    expect(langOf("a\\b\\c.md")).toBe("markdown");
  });
});

describe("supports", () => {
  it("only claims what the format can write", () => {
    expect(supports("typst", "underline")).toBe(true);
    expect(supports("markdown", "underline")).toBe(false);
    for (const item of [
      "attach",
      "include",
      "bibliography",
      "cite",
      "ref",
      "outline",
      "pagebreak",
    ] as const) {
      expect(supports("typst", item)).toBe(true);
      expect(supports("markdown", item)).toBe(false);
    }
    expect(supports("markdown", "task")).toBe(true);
    expect(supports("typst", "task")).toBe(false);
    expect(supports("plain", "bold")).toBe(false);
    expect(supports("plain", "paragraph")).toBe(false);
  });
});

describe("activeMarks", () => {
  it("FMT-19 reports nested marks at the caret", () => {
    expect(marks("*a _b‸_ c*", "typst")).toEqual(["bold", "italic"]);
    expect(marks("*a _‸b_ c*", "typst")).toEqual(["bold", "italic"]);
    expect(marks("*a _b_‸ c*", "typst")).toEqual(["bold"]);
    expect(marks("*a _b_ c‸*", "typst")).toEqual(["bold"]);
    expect(marks("*a _b_ c*‸", "typst")).toEqual([]);
    expect(marks("**a *b‸* c**", "markdown")).toEqual(["bold", "italic"]);
    expect(marks("***a‸***", "markdown")).toEqual(["bold", "italic"]);
    expect(marks("~~a‸~~", "markdown")).toEqual(["strike"]);
    expect(marks("#underline[a‸]", "typst")).toEqual(["underline"]);
    expect(marks('#link("u")[a‸]', "typst")).toEqual(["link"]);
    expect(marks("[a‸](u)", "markdown")).toEqual(["link"]);
  });

  it("FMT-19 reports a selection only when all of it is inside the span", () => {
    expect(marks("*⟨a b⟩*", "typst")).toEqual(["bold"]);
    expect(marks("*a* ⟨b *c*⟩", "typst")).toEqual([]);
  });

  it("FMT-19 reports only code inside inline code, nothing in a formula or code block", () => {
    expect(marks("`*a‸*`", "typst")).toEqual(["code"]);
    expect(marks("*a `b‸` c*", "typst")).toEqual(["code"]);
    expect(marks("`**a‸**`", "markdown")).toEqual(["code"]);
    expect(marks("`a‸`", "markdown")).toEqual(["code"]);
    expect(marks("$ *a‸* $", "typst")).toEqual([]);
    expect(marks("$*a‸*$", "markdown")).toEqual([]);
    expect(marks("```\n**a‸**\n```", "markdown")).toEqual([]);
    expect(marks("```\n*a‸*\n```", "typst")).toEqual([]);
  });

  it("is quiet about punctuation that is not markup", () => {
    expect(marks("2 * 3 * 4‸", "markdown")).toEqual([]);
    expect(marks("snake_case_na‸me", "typst")).toEqual([]);
    expect(marks("a \\*b‸\\*", "markdown")).toEqual([]);
    expect(marks("[a‸]", "markdown")).toEqual([]);
    expect(marks("![a‸](p)", "markdown")).toEqual([]);
  });

  it("is empty for plain text", () => {
    expect(activeMarks(parse("*a‸*"), "plain").size).toBe(0);
  });
});

describe("activeBlock", () => {
  const block = (marked: string, lang: "typst" | "markdown") => activeBlock(parse(marked), lang);

  it("FMT-20 reads the heading level, whichever way Typst spells it", () => {
    expect(block("== x‸", "typst")).toBe("h2");
    expect(block("#heading(level: 2)[x‸]", "typst")).toBe("h2");
    expect(block("= x", "typst")).toBe("h1");
    expect(block("===== deep", "typst")).toBe("h4");
    expect(block("### x", "markdown")).toBe("h3");
  });

  it("reads lists, quotes and code blocks", () => {
    expect(block("- a‸", "typst")).toBe("ul");
    expect(block("+ a‸", "typst")).toBe("ol");
    expect(block("1. a‸", "typst")).toBe("ol");
    expect(block("#quote[a‸]", "typst")).toBe("quote");
    expect(block("- a‸", "markdown")).toBe("ul");
    expect(block("* a‸", "markdown")).toBe("ul");
    expect(block("3. a‸", "markdown")).toBe("ol");
    expect(block("- [ ] a‸", "markdown")).toBe("task");
    expect(block("> a‸", "markdown")).toBe("quote");
    expect(block("```js\nx‸\n```", "markdown")).toBe("codeblock");
    expect(block("```\nx\n```\ny‸", "markdown")).toBe("paragraph");
    expect(block("texte‸", "markdown")).toBe("paragraph");
    expect(block("‸", "typst")).toBe("paragraph");
  });

  it("is a paragraph in plain text", () => {
    expect(activeBlock(parse("= x‸"), "plain")).toBe("paragraph");
  });
});
