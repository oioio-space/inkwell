import type { DeckData } from "../preview/protocol";

/** Where a presentation stands: a slide (1-based, as Typst numbers them) and a step in it (0-based). */
export interface Pos {
  slide: number;
  step: number;
}

/** One slide of the deck: the pages that show its steps, and what the speaker wrote for it. */
interface Slide {
  /** The slide's number. */
  slide: number;
  /** Page number (1-based) of each step. */
  pages: number[];
  note: string;
}

/** What the "next" thumbnail shows. */
export type Upcoming = { kind: "step" | "slide"; pos: Pos } | { kind: "end" };

/**
 * A document seen as slides and steps. With Touying's metadata the pages of one slide are its
 * steps (`#pause`); without it every page is a slide of one step. Hidden slides are numbered but
 * skipped by every move. All moves are total: they clamp rather than fail, so a document that
 * changes under the presentation never leaves the position nowhere.
 */
export class Deck {
  readonly #slides: Slide[];
  readonly #shown: Slide[];
  /** The planned duration in minutes, 0 if unset. */
  readonly minutes: number;

  constructor(data: DeckData | undefined, pageCount: number) {
    this.minutes = data?.minutes ?? 0;
    const bySlide = new Map<number, Slide>();
    const hidden = new Set<number>();
    const pages = data?.pages ?? [];
    const usable = pages.length > 0 && pages.length === pageCount;
    if (usable) {
      pages.forEach((p, i) => {
        let s = bySlide.get(p.slide);
        if (!s) {
          s = { slide: p.slide, pages: [], note: "" };
          bySlide.set(p.slide, s);
        }
        s.pages[p.step] = i + 1;
        if (p.note) s.note = s.note ? `${s.note}\n\n${p.note}` : p.note;
        if (p.hidden) hidden.add(p.slide);
      });
    } else {
      for (let n = 1; n <= pageCount; n++) bySlide.set(n, { slide: n, pages: [n], note: "" });
    }
    // A step that no page shows (a gap) is dropped, so that steps are consecutive.
    this.#slides = [...bySlide.values()].map((s) => ({ ...s, pages: s.pages.filter(Boolean) }));
    this.#shown = this.#slides.filter((s) => !hidden.has(s.slide));
  }

  /** How many slides a presentation walks through. */
  get count(): number {
    return this.#shown.length;
  }

  /** True when there is nothing to present. */
  get empty(): boolean {
    return this.#shown.length === 0;
  }

  first(): Pos {
    return { slide: this.#shown[0]?.slide ?? 1, step: 0 };
  }

  last(): Pos {
    const s = this.#shown.at(-1);
    return { slide: s?.slide ?? 1, step: Math.max(0, (s?.pages.length ?? 1) - 1) };
  }

  /** `pos` if it is a place of the deck, else the nearest one. */
  clamp(pos: Pos): Pos {
    if (this.empty) return { slide: 1, step: 0 };
    let i = this.#shown.findIndex((s) => s.slide >= pos.slide);
    if (i < 0) i = this.#shown.length - 1;
    const s = this.#shown[i] as Slide;
    return { slide: s.slide, step: Math.min(Math.max(0, pos.step), s.pages.length - 1) };
  }

  /** The 1-based page that shows `pos`; 1 for an empty deck. */
  pageOf(pos: Pos): number {
    const p = this.clamp(pos);
    return this.#slides.find((s) => s.slide === p.slide)?.pages[p.step] ?? 1;
  }

  /** The slide and step that page `n` (1-based) shows; the nearest shown slide if it is hidden. */
  posOfPage(n: number): Pos {
    for (const s of this.#slides) {
      const step = s.pages.indexOf(n);
      if (step >= 0) return this.clamp({ slide: s.slide, step });
    }
    return this.clamp(n < 1 ? this.first() : this.last());
  }

  /** The position, from 1 to `count`, of the slide among those shown. */
  indexOf(pos: Pos): number {
    const p = this.clamp(pos);
    return this.#shown.findIndex((s) => s.slide === p.slide) + 1;
  }

  /** The `n`th slide shown (1-based), at its first step; clamped. */
  nth(n: number): Pos {
    const s = this.#shown[Math.min(Math.max(1, Math.trunc(n)), this.#shown.length) - 1];
    return { slide: s?.slide ?? 1, step: 0 };
  }

  /** The speaker notes of the slide at `pos`. */
  noteOf(pos: Pos): string {
    return this.#slides.find((s) => s.slide === this.clamp(pos).slide)?.note ?? "";
  }

  /** The next step, or the first step of the next slide; `pos` itself at the end. */
  next(pos: Pos): Pos {
    const u = this.upcoming(pos);
    return u.kind === "end" ? this.clamp(pos) : u.pos;
  }

  /** The previous step, or the last step of the previous slide; `pos` itself at the start. */
  prev(pos: Pos): Pos {
    const p = this.clamp(pos);
    if (p.step > 0) return { slide: p.slide, step: p.step - 1 };
    const i = this.#shown.findIndex((s) => s.slide === p.slide);
    const before = this.#shown[i - 1];
    return before ? { slide: before.slide, step: before.pages.length - 1 } : p;
  }

  /** What `next` leads to, and whether it is a step or a slide. */
  upcoming(pos: Pos): Upcoming {
    if (this.empty) return { kind: "end" };
    const p = this.clamp(pos);
    const i = this.#shown.findIndex((s) => s.slide === p.slide);
    const s = this.#shown[i] as Slide;
    if (p.step + 1 < s.pages.length)
      return { kind: "step", pos: { slide: p.slide, step: p.step + 1 } };
    const after = this.#shown[i + 1];
    return after ? { kind: "slide", pos: { slide: after.slide, step: 0 } } : { kind: "end" };
  }
}
