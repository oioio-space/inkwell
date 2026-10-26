import { describe, expect, it } from "vitest";
import type { DeckPage } from "../preview/protocol";
import { Deck } from "./deck";

const page = (slide: number, step: number, over: Partial<DeckPage> = {}): DeckPage => ({
  slide,
  step,
  hidden: false,
  note: "",
  ...over,
});

/** Slide 1 alone, slide 2 in three steps, slide 3 alone. */
const pages = [page(1, 0), page(2, 0), page(2, 1, { note: "sourire" }), page(2, 2), page(3, 0)];
const deck = new Deck({ pages, minutes: 20 }, 5);

describe("Deck", () => {
  it("PRS-05 moves by steps, then by slides, and stops at both ends", () => {
    let p = deck.first();
    const seen = [p];
    for (let i = 0; i < 8; i++) {
      p = deck.next(p);
      seen.push(p);
    }
    expect(seen.slice(0, 6)).toEqual([
      { slide: 1, step: 0 },
      { slide: 2, step: 0 },
      { slide: 2, step: 1 },
      { slide: 2, step: 2 },
      { slide: 3, step: 0 },
      { slide: 3, step: 0 }, // the end: next stays
    ]);
    expect(deck.prev(deck.first())).toEqual(deck.first());
    expect(deck.prev({ slide: 3, step: 0 })).toEqual({ slide: 2, step: 2 }); // back into the last step
    expect(deck.prev({ slide: 2, step: 1 })).toEqual({ slide: 2, step: 0 });
  });

  it("PRS-05 goes to the first, the last, and the nth slide", () => {
    expect(deck.last()).toEqual({ slide: 3, step: 0 });
    expect(deck.nth(2)).toEqual({ slide: 2, step: 0 });
    expect(deck.nth(0)).toEqual({ slide: 1, step: 0 });
    expect(deck.nth(99)).toEqual({ slide: 3, step: 0 });
    expect(deck.indexOf({ slide: 3, step: 0 })).toBe(3);
  });

  it("maps a position to its page", () => {
    expect(deck.pageOf({ slide: 2, step: 2 })).toBe(4);
    expect(deck.pageOf({ slide: 9, step: 9 })).toBe(5); // clamped to the end
  });

  it("PRS-13 finds the slide and step of a page, and the other way round", () => {
    expect(deck.posOfPage(3)).toEqual({ slide: 2, step: 1 });
    expect(deck.posOfPage(5)).toEqual({ slide: 3, step: 0 });
    expect(deck.posOfPage(99)).toEqual({ slide: 3, step: 0 });
    expect(deck.posOfPage(0)).toEqual({ slide: 1, step: 0 });
    for (let n = 1; n <= 5; n++) expect(deck.pageOf(deck.posOfPage(n))).toBe(n);
  });

  it("PRS-07 says what comes next: a step, a slide, or the end", () => {
    expect(deck.upcoming({ slide: 2, step: 0 })).toEqual({
      kind: "step",
      pos: { slide: 2, step: 1 },
    });
    expect(deck.upcoming({ slide: 2, step: 2 })).toEqual({
      kind: "slide",
      pos: { slide: 3, step: 0 },
    });
    expect(deck.upcoming({ slide: 3, step: 0 })).toEqual({ kind: "end" });
  });

  it("PRS-08 gives the note of a slide whichever step carries it", () => {
    expect(deck.noteOf({ slide: 2, step: 0 })).toBe("sourire");
    expect(deck.noteOf({ slide: 1, step: 0 })).toBe("");
    const two = new Deck(
      { pages: [page(1, 0, { note: "a" }), page(1, 1, { note: "b" })], minutes: 0 },
      2,
    );
    expect(two.noteOf({ slide: 1, step: 0 })).toBe("a\n\nb");
  });

  it("skips hidden slides in every move", () => {
    const d = new Deck(
      { pages: [page(1, 0), page(2, 0, { hidden: true }), page(3, 0)], minutes: 0 },
      3,
    );
    expect(d.count).toBe(2);
    expect(d.next({ slide: 1, step: 0 })).toEqual({ slide: 3, step: 0 });
    expect(d.prev({ slide: 3, step: 0 })).toEqual({ slide: 1, step: 0 });
    expect(d.clamp({ slide: 2, step: 0 })).toEqual({ slide: 3, step: 0 });
    expect(d.indexOf({ slide: 3, step: 0 })).toBe(2);
  });

  it("PRS-02 without metadata every page is a slide", () => {
    const d = new Deck(undefined, 3);
    expect(d.count).toBe(3);
    expect(d.next({ slide: 1, step: 0 })).toEqual({ slide: 2, step: 0 });
    expect(d.pageOf({ slide: 3, step: 0 })).toBe(3);
  });

  it("falls back to pages when the metadata is not about this many pages", () => {
    // A deck from an older compilation: the pages on screen win.
    const d = new Deck({ pages: [page(1, 0)], minutes: 0 }, 4);
    expect(d.count).toBe(4);
  });

  it("PRS-11 keeps the position through an edit that adds or removes slides", () => {
    const shorter = new Deck({ pages: [page(1, 0), page(2, 0)], minutes: 0 }, 2);
    expect(shorter.clamp({ slide: 3, step: 0 })).toEqual({ slide: 2, step: 0 });
    expect(shorter.clamp({ slide: 2, step: 5 })).toEqual({ slide: 2, step: 0 });
    const longer = new Deck({ pages: [...pages, page(4, 0)], minutes: 0 }, 6);
    expect(longer.clamp({ slide: 2, step: 1 })).toEqual({ slide: 2, step: 1 });
  });

  it("an empty deck has a position and no moves", () => {
    const d = new Deck(undefined, 0);
    expect(d.empty).toBe(true);
    expect(d.first()).toEqual({ slide: 1, step: 0 });
    expect(d.next(d.first())).toEqual({ slide: 1, step: 0 });
    expect(d.pageOf(d.first())).toBe(1);
    expect(d.upcoming(d.first())).toEqual({ kind: "end" });
  });
});
