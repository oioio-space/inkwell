import { undo } from "@codemirror/commands";
import { EditorState } from "@codemirror/state";
import { describe, expect, it } from "vitest";
import { setBlock, toggleMark } from "./commands";
import { formatHandlers } from "./handlers";
import { insert } from "./insert";
import type { Lang } from "./lang";
import { supports } from "./support";
import { apply, MARKDOWN, PLAIN, parse, run, show, TYPST } from "./testkit";
import type { Block, InsertItem, Mark } from "./types";

const bold = (lang: Lang) => toggleMark(lang, "bold");

describe("marks", () => {
  it("FMT-01 bolds the selected word in Typst and keeps the selection on it", () => {
    expect(apply("un ⟨mot⟩ ici", bold(TYPST))).toBe("un *⟨mot⟩* ici");
  });

  it("FMT-02 bolds the selected word in Markdown", () => {
    expect(apply("un ⟨mot⟩ ici", bold(MARKDOWN))).toBe("un **⟨mot⟩** ici");
  });

  it("FMT-03 bold again toggles it off, from an inner or an outer selection", () => {
    expect(apply("un *⟨mot⟩* ici", bold(TYPST))).toBe("un ⟨mot⟩ ici");
    expect(apply("un ⟨*mot*⟩ ici", bold(TYPST))).toBe("un ⟨mot⟩ ici");
    expect(apply("un **⟨mot⟩** ici", bold(MARKDOWN))).toBe("un ⟨mot⟩ ici");
    expect(apply("un ⟨**mot**⟩ ici", bold(MARKDOWN))).toBe("un ⟨mot⟩ ici");
  });

  it("FMT-03 italic does not mistake a bold run for italic, and stacks with it", () => {
    const italic = toggleMark(MARKDOWN, "italic");
    expect(apply("**⟨mot⟩**", italic)).toBe("***⟨mot⟩***");
    expect(apply("***⟨mot⟩***", italic)).toBe("**⟨mot⟩**");
    expect(apply("***⟨mot⟩***", bold(MARKDOWN))).toBe("*⟨mot⟩*");
    expect(apply("*⟨mot⟩*", bold(MARKDOWN))).toBe("***⟨mot⟩***");
  });

  it("FMT-04 a caret inside a word formats the whole word and stays where it was", () => {
    expect(apply("un mo‸t ici", bold(MARKDOWN))).toBe("un **mo‸t** ici");
    expect(apply("un mo‸t ici", bold(TYPST))).toBe("un *mo‸t* ici");
    expect(apply("un mot‸ ici", bold(TYPST))).toBe("un *mot‸* ici");
    expect(apply("un **mo‸t** ici", bold(MARKDOWN))).toBe("un mo‸t ici");
  });

  it("FMT-05 a caret outside any word inserts an empty pair with the caret between", () => {
    expect(apply("‸", bold(MARKDOWN))).toBe("**‸**");
    expect(apply("a ‸ b", bold(MARKDOWN))).toBe("a **‸** b");
    expect(apply("a ‸ b", bold(TYPST))).toBe("a *‸* b");
    expect(apply("a **‸** b", bold(MARKDOWN))).toBe("a ‸ b");
  });

  it("FMT-06 a partly bold selection becomes entirely bold, without nested delimiters", () => {
    expect(apply("⟨a **b** c⟩", bold(MARKDOWN))).toBe("**⟨a b c⟩**");
    expect(apply("⟨a *b* c⟩", bold(TYPST))).toBe("*⟨a b c⟩*");
    expect(apply("⟨a #underline[b] c⟩", toggleMark(TYPST, "underline"))).toBe(
      "#underline[⟨a b c⟩]",
    );
  });

  it("FMT-07 several selections are all formatted in one transaction, one undo step", () => {
    const start = parse("⟨un⟩ ⟨deux⟩ ⟨trois⟩");
    const done = run(start, bold(MARKDOWN));
    expect(done.dispatched).toBe(1);
    expect(show(done.state)).toBe("**⟨un⟩** **⟨deux⟩** **⟨trois⟩**");
    let undone = done.state;
    expect(undo({ state: done.state, dispatch: (tr) => (undone = tr.state) })).toBe(true);
    expect(undone.doc.toString()).toBe("un deux trois");
  });

  it("FMT-07 several carets format their own words", () => {
    expect(apply("u‸n de‸ux", bold(TYPST))).toBe("*u‸n* *de‸ux*");
  });

  it("FMT-08 a multi-line selection is formatted line by line, never across a blank line", () => {
    expect(apply("⟨ligne un\nligne deux\n\nligne trois⟩", bold(MARKDOWN))).toBe(
      "**⟨ligne un⟩**\n**⟨ligne deux⟩**\n\n**⟨ligne trois⟩**",
    );
    // Toggling again removes it everywhere.
    const first = run(parse("⟨a\nb⟩"), bold(TYPST));
    expect(show(run(first.state, bold(TYPST)).state)).toBe("⟨a⟩\n⟨b⟩");
  });

  it("FMT-08 leading and trailing spaces stay outside the delimiters", () => {
    expect(apply("a⟨  mot  ⟩b", bold(MARKDOWN))).toBe("a  **⟨mot⟩**  b");
  });

  it("FMT-09 Markdown has no underline: the command declines and the shortcut stays free", () => {
    expect(supports(MARKDOWN, "underline")).toBe(false);
    const result = run(parse("un ⟨mot⟩"), toggleMark(MARKDOWN, "underline"));
    expect(result.ran).toBe(false);
    expect(result.dispatched).toBe(0);
    // No handler in Markdown: Ctrl+U goes on to CodeMirror (extensions/commands.ts).
    expect(Object.keys(formatHandlers(MARKDOWN, () => true))).not.toContain("format.underline");
    expect(Object.keys(formatHandlers(TYPST, () => true))).toContain("format.underline");
  });

  it("FMT-09 Typst underline and strike are function calls", () => {
    expect(apply("un ⟨mot⟩", toggleMark(TYPST, "underline"))).toBe("un #underline[⟨mot⟩]");
    expect(apply("un #underline[⟨mot⟩]", toggleMark(TYPST, "underline"))).toBe("un ⟨mot⟩");
    expect(apply("un ⟨mot⟩", toggleMark(TYPST, "strike"))).toBe("un #strike[⟨mot⟩]");
    expect(apply("un ⟨mot⟩", toggleMark(MARKDOWN, "strike"))).toBe("un ~~⟨mot⟩~~");
  });

  it("FMT-16 inline code doubles its delimiter around a backtick", () => {
    const code = toggleMark(MARKDOWN, "code");
    expect(apply("⟨x⟩", code)).toBe("`⟨x⟩`");
    expect(apply("⟨x`y⟩", code)).toBe("`` ⟨x`y⟩ ``");
    expect(apply("`` ⟨x`y⟩ ``", code)).toBe("⟨x`y⟩");
    expect(apply("⟨a``b⟩", code)).toBe("``` ⟨a``b⟩ ```");
    expect(apply("`⟨x⟩`", code)).toBe("⟨x⟩");
    expect(apply("⟨`x`⟩", code)).toBe("⟨x⟩");
    expect(apply("`a` ⟨x⟩ `b`", code)).toBe("`a` `⟨x⟩` `b`");
  });

  it("FMT-17 a selection that looks like a URL becomes a link to itself", () => {
    expect(apply("⟨https://ex.org/a⟩", toggleMark(MARKDOWN, "link"))).toBe(
      "[⟨https://ex.org/a⟩](https://ex.org/a)",
    );
    expect(apply("⟨https://ex.org/a⟩", toggleMark(TYPST, "link"))).toBe(
      '#link("⟨https://ex.org/a⟩")',
    );
    expect(apply("[⟨https://ex.org/a⟩](https://ex.org/a)", toggleMark(MARKDOWN, "link"))).toBe(
      "⟨https://ex.org/a⟩",
    );
    expect(apply('#link("⟨https://ex.org/a⟩")', toggleMark(TYPST, "link"))).toBe(
      "⟨https://ex.org/a⟩",
    );
  });

  it("FMT-17 other text becomes a link with the caret where the address goes", () => {
    expect(apply("⟨texte⟩", toggleMark(MARKDOWN, "link"))).toBe("[texte](‸)");
    expect(apply("⟨texte⟩", toggleMark(TYPST, "link"))).toBe('#link("‸")[texte]');
    expect(apply("[deux m‸ots](u)", toggleMark(MARKDOWN, "link"))).toBe("deux m‸ots");
    expect(apply('#link("u")[deux m‸ots]', toggleMark(TYPST, "link"))).toBe("deux m‸ots");
  });

  it("FMT-18 never cuts a grapheme: composed accents, ZWJ emoji, Arabic", () => {
    // "e" + combining acute: a selection ending between them takes the accent along.
    expect(apply("a⟨e⟩́b", bold(TYPST))).toBe("a*⟨é⟩*b");
    // A family emoji (ZWJ sequence) is selected from its middle.
    const family = "👨‍👩‍👧";
    expect(apply(`x⟨${family.slice(0, 3)}⟩${family.slice(3)}y`, bold(TYPST))).toBe(
      `x*⟨${family}⟩*y`,
    );
    // A caret inside the emoji puts the empty pair before it, never inside.
    const state = parse(`‸${family}`);
    const inside = state.update({ selection: { anchor: 2 } }).state;
    expect(show(run(inside, bold(TYPST)).state)).toBe(`*‸*${family}`);
    // Arabic word.
    expect(apply("⟨مرحبا⟩ بالعالم", bold(MARKDOWN))).toBe("**⟨مرحبا⟩** بالعالم");
    // CJK word.
    expect(apply("今日‸は", bold(MARKDOWN))).toBe("**今日‸**は");
  });
});

describe("blocks", () => {
  it("FMT-10 sets a heading level in one undo step", () => {
    const start = parse("= A‸");
    const done = run(start, setBlock(TYPST, "h2"));
    expect(show(done.state)).toBe("== A‸");
    expect(done.dispatched).toBe(1);
    let undone = done.state;
    undo({ state: done.state, dispatch: (tr) => (undone = tr.state) });
    expect(undone.doc.toString()).toBe("= A");
  });

  it("FMT-10 changes heading levels in both directions, and Markdown too", () => {
    expect(apply("== A‸", setBlock(TYPST, "h4"))).toBe("==== A‸");
    expect(apply("==== A‸", setBlock(TYPST, "h1"))).toBe("= A‸");
    expect(apply("A‸", setBlock(MARKDOWN, "h3"))).toBe("### A‸");
    expect(apply("# A‸", setBlock(MARKDOWN, "h2"))).toBe("## A‸");
    expect(apply("#heading(level: 2)[A]‸", setBlock(TYPST, "h3"))).toBe("=== A‸");
  });

  it("FMT-10 the same level again changes nothing", () => {
    const result = run(parse("== A‸"), setBlock(TYPST, "h2"));
    expect(result.dispatched).toBe(0);
    expect(result.ran).toBe(true);
  });

  it("FMT-11 paragraph removes the heading prefix", () => {
    expect(apply("### A‸", setBlock(MARKDOWN, "paragraph"))).toBe("A‸");
    expect(apply("== A‸", setBlock(TYPST, "paragraph"))).toBe("A‸");
    expect(apply("- a‸", setBlock(MARKDOWN, "paragraph"))).toBe("a‸");
  });

  it("FMT-12 a list over three lines leaves the blank one alone, and toggles back", () => {
    const first = run(parse("⟨un\n\ndeux⟩"), setBlock(MARKDOWN, "ul"));
    expect(first.state.doc.toString()).toBe("- un\n\n- deux");
    const second = run(first.state, setBlock(MARKDOWN, "ul"));
    expect(second.state.doc.toString()).toBe("un\n\ndeux");
    const typst = run(parse("⟨un\n\ndeux⟩"), setBlock(TYPST, "ul"));
    expect(typst.state.doc.toString()).toBe("- un\n\n- deux");
    expect(run(typst.state, setBlock(TYPST, "ul")).state.doc.toString()).toBe("un\n\ndeux");
  });

  it("FMT-12 a caret on an empty line starts the list", () => {
    expect(apply("‸", setBlock(MARKDOWN, "ul"))).toBe("- ‸");
  });

  it("FMT-12 changing the kind of list replaces the marker", () => {
    const text = (marked: string, block: Block) =>
      apply(marked, setBlock(MARKDOWN, block)).replace(/[⟨⟩‸]/g, "");
    expect(text("⟨- a\n- b⟩", "ol")).toBe("1. a\n2. b");
    expect(text("⟨1. a\n2. b⟩", "task")).toBe("- [ ] a\n- [ ] b");
    expect(text("⟨- [ ] a\n- [x] b⟩", "task")).toBe("a\nb");
    expect(text("⟨> a\n> b⟩", "quote")).toBe("a\nb");
    expect(text("⟨a\nb⟩", "quote")).toBe("> a\n> b");
    expect(text("⟨- [x] a\n- b⟩", "task")).toBe("- [x] a\n- [ ] b");
  });

  it("FMT-13 a numbered list over twelve lines counts 1 to 12", () => {
    const lines = Array.from({ length: 12 }, (_, i) => `ligne ${i + 1}`);
    const result = run(parse(`⟨${lines.join("\n")}⟩`), setBlock(MARKDOWN, "ol"));
    expect(result.state.doc.toString()).toBe(lines.map((l, i) => `${i + 1}. ${l}`).join("\n"));
    expect(result.dispatched).toBe(1);
    const typst = run(parse(`⟨${lines.join("\n")}⟩`), setBlock(TYPST, "ol"));
    expect(typst.state.doc.toString()).toBe(lines.map((l) => `+ ${l}`).join("\n"));
  });

  it("FMT-13 a numbered list skips blank lines when counting", () => {
    expect(apply("⟨a\n\nb⟩", setBlock(MARKDOWN, "ol")).replace(/[⟨⟩]/g, "")).toBe("1. a\n\n2. b");
  });

  it("FMT-14 a code block fences the selected lines, caret on the language", () => {
    expect(apply("avant\n⟨a\nb⟩\naprès", setBlock(MARKDOWN, "codeblock"))).toBe(
      "avant\n```‸\na\nb\n```\naprès",
    );
    expect(apply("⟨a⟩", setBlock(TYPST, "codeblock"))).toBe("```‸\na\n```");
    // Partial lines are fenced whole.
    expect(apply("x ⟨a\nb⟩ y", setBlock(MARKDOWN, "codeblock"))).toBe("```‸\nx a\nb y\n```");
  });

  it("FMT-14 a code block toggles off, fences gone, content kept", () => {
    expect(apply("```js\na‸\n```", setBlock(MARKDOWN, "codeblock"))).toBe("a‸");
    expect(apply("```\n‸\n```", setBlock(TYPST, "codeblock"))).toBe("‸");
    expect(apply("x\n```\na‸\n```\ny", setBlock(TYPST, "paragraph"))).toBe("x\na‸\ny");
  });

  it("FMT-15 a code block without a selection is an empty block with the caret inside", () => {
    expect(apply("‸", setBlock(MARKDOWN, "codeblock"))).toBe("```\n‸\n```");
    expect(apply("texte‸", setBlock(MARKDOWN, "codeblock"))).toBe("texte\n```\n‸\n```");
  });

  it("FMT-27 a whitespace-only line survives a code block toggled twice", () => {
    for (const lang of [TYPST, MARKDOWN] as Lang[]) {
      const once = run(parse("aaa\n ‸\na"), setBlock(lang, "codeblock"));
      // The blank line becomes the block's body, its spaces kept and the caret after them.
      expect(show(once.state)).toBe("aaa\n```\n ‸\n```\na");
      expect(show(run(once.state, setBlock(lang, "codeblock")).state)).toBe("aaa\n ‸\na");
    }
  });

  it("FMT-14 other blocks decline inside a code block", () => {
    const result = run(parse("```\n- a‸\n```"), setBlock(MARKDOWN, "ul"));
    expect(result.ran).toBe(false);
    expect(result.dispatched).toBe(0);
  });

  it("wraps and unwraps a Typst quote, also around several lines", () => {
    expect(apply("‸mot", setBlock(TYPST, "quote"))).toBe("#quote[‸mot]");
    expect(apply("⟨a\nb⟩", setBlock(TYPST, "quote"))).toBe("#quote[⟨a\nb⟩]");
    expect(apply("#quote[mot]‸", setBlock(TYPST, "quote"))).toBe("mot‸");
    expect(apply("#quote[a‸\nb]", setBlock(TYPST, "paragraph"))).toBe("a‸\nb");
    expect(apply("#quote[mot‸]", setBlock(TYPST, "h2"))).toBe("== mot‸");
  });

  it("multi-cursor block edits are one transaction", () => {
    const result = run(parse("a‸\nb\nc‸"), setBlock(MARKDOWN, "h1"));
    expect(result.state.doc.toString()).toBe("# a\nb\n# c");
    expect(result.dispatched).toBe(1);
  });
});

describe("FMT-21 read-only", () => {
  const marks: Mark[] = ["bold", "italic", "underline", "strike", "code", "link"];
  const blocks: Block[] = [
    "h1",
    "h2",
    "h3",
    "h4",
    "paragraph",
    "ul",
    "ol",
    "task",
    "quote",
    "codeblock",
  ];
  const items: InsertItem[] = [
    "image",
    "figure",
    "include",
    "attach",
    "csvTable",
    "data",
    "bibliography",
    "cite",
    "ref",
    "footnote",
    "link",
    "outline",
    "pagebreak",
    "table",
    "formula",
  ];

  it("every command declines and dispatches nothing for a reader", () => {
    const readOnly = EditorState.readOnly.of(true);
    for (const lang of [TYPST, MARKDOWN] as const) {
      const state = parse("⟨un mot⟩\n= titre", readOnly);
      const commands = [
        ...marks.map((m) => toggleMark(lang, m)),
        ...blocks.map((b) => setBlock(lang, b)),
        ...items.map((i) =>
          insert(lang, i, { path: "a.typ", key: "k", hasBibliography: true, rows: [["a"]] }),
        ),
      ];
      for (const command of commands) {
        const result = run(state, command);
        expect(result.ran).toBe(false);
        expect(result.dispatched).toBe(0);
      }
    }
  });

  it("plain text has nothing to format", () => {
    const state = parse("⟨un mot⟩");
    expect(run(state, toggleMark(PLAIN, "bold")).ran).toBe(false);
    expect(run(state, setBlock(PLAIN, "h1")).ran).toBe(false);
    expect(run(state, insert(PLAIN, "link", { url: "u" })).ran).toBe(false);
    expect(formatHandlers(PLAIN, () => true)).toEqual({});
  });
});

describe("format handlers", () => {
  it("the handlers are the language's format commands, bold edits the text, and a heading's chord again makes a paragraph", () => {
    let state = parse("un ⟨mot⟩");
    const view = { state, dispatch: (tr: { state: EditorState }) => (state = tr.state) };
    const handlers = formatHandlers(TYPST, (command) => {
      view.state = state;
      return command(view as never);
    });
    expect(Object.keys(handlers)).toEqual([
      "format.bold",
      "format.italic",
      "format.underline",
      "format.strike",
      "format.link",
      "format.code",
      "format.codeblock",
      "format.formula",
      "format.ul",
      "format.ol",
      "format.quote",
      "format.table",
      "format.h1",
      "format.h2",
      "format.h3",
      "format.h4",
      "format.paragraph",
    ]);
    handlers["format.bold"]?.();
    expect(show(state)).toBe("un *⟨mot⟩*");
    state = parse("Titre‸");
    handlers["format.h2"]?.();
    expect(show(state)).toBe("== Titre‸");
    handlers["format.h2"]?.();
    expect(show(state)).toBe("Titre‸");
  });
});
