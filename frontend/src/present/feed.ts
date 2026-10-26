import type { Page } from "../preview/preview.svelte";
import type { HubFrame } from "./hub";
import type { ScreenMode } from "./wire";

/** What the public view should show now. */
export interface PublicView {
  /** The page of the document, 1-based. */
  page: number;
  pageCount: number;
  screen: ScreenMode;
  laser: { x: number; y: number } | null;
  away: boolean;
  /** The version of the sources the pages belong to. */
  rev: number;
  pages: ReadonlyMap<number, Page>;
}

/** How many pages ahead of the current one the public view is given, so that a step is instant. */
const AHEAD = 2;

/**
 * Feeds the public window over the hub: the view when it changes, and the pages it needs (the
 * current one and a few ahead), each sent once per version of the sources. The notes and comments
 * are not here: this is all the public window ever receives.
 */
export class PublicFeed {
  readonly #send: (f: HubFrame) => void;
  #view = "";
  readonly #sent = new Map<number, number>();
  #last: PublicView | undefined;

  constructor(send: (f: HubFrame) => void) {
    this.#send = send;
  }

  /** Brings the public window up to date with `v`. Cheap to call as often as the view changes. */
  update(v: PublicView): void {
    this.#last = v;
    const view: HubFrame = {
      type: "view",
      page: v.page,
      screen: v.screen,
      laser: v.laser,
      away: v.away,
    };
    const text = JSON.stringify(view);
    // Pages first, so that the view never names a page the window does not have yet.
    if (v.screen === "show") {
      for (let n = v.page; n <= Math.min(v.pageCount, v.page + AHEAD); n++) this.#page(n, v);
    }
    if (text !== this.#view) {
      this.#view = text;
      this.#send(view);
    }
  }

  /** The public window opened or lost something: say it all again. */
  hello(): void {
    this.#view = "";
    this.#sent.clear();
    if (this.#last) this.update(this.#last);
  }

  /** The public window asks for a page it lacks. */
  need(n: number): void {
    this.#sent.delete(n);
    if (this.#last) this.update(this.#last);
  }

  #page(n: number, v: PublicView): void {
    const page = v.pages.get(n);
    if (!page || this.#sent.get(n) === page.rev) return;
    this.#sent.set(n, page.rev);
    this.#send({
      type: "page",
      n,
      rev: page.rev,
      src: page.src,
      width: page.width,
      height: page.height,
    });
  }
}
