import type { SocketLike } from "../net/WailsNetworkAdapter";
import { type Command, HubClient, type HubFrame } from "./hub";
import { KeyReader } from "./keys";
import type { ScreenMode } from "./wire";

/** A page of the document as the public window holds it. */
export interface PublicPage {
  src: string;
  width: number;
  height: number;
  rev: number;
}

/** Pages kept around the one shown: enough for the next steps, not enough to fill memory. */
const KEEP = 6;

/**
 * The second window of a presentation, on this machine: it shows the slide and nothing else. It
 * holds no document, no notes and no comments, only the pages and the view the presenter window
 * sends it, and it opens no stream but the `present` one. Keys pressed in it go back to the
 * presenter as commands, so that either window can move the one presentation.
 */
export class PublicModel {
  /** The page shown, 1-based; 0 until the presenter has said. */
  page = $state(0);
  screen = $state<ScreenMode>("show");
  laser = $state.raw<{ x: number; y: number } | null>(null);
  /** The presenter is disconnected: the last slide stays, with a mention. */
  away = $state(false);
  /** How many windows of the presentation are open, this one included. */
  windows = $state(0);
  pages = $state.raw<ReadonlyMap<number, PublicPage>>(new Map());

  #client: HubClient | undefined;
  readonly #keys = new KeyReader();
  readonly #asked = new Set<number>();

  /** The page to draw; undefined while it has not arrived. */
  current = $derived(this.pages.get(this.page));

  start(open: () => SocketLike): void {
    this.#client ??= new HubClient(
      open,
      (f) => this.receive(f),
      () => {
        this.#asked.clear();
        this.#client?.send({ type: "hello" });
      },
    );
    this.#client.start();
  }

  stop(): void {
    this.#client?.stop();
  }

  receive(f: HubFrame): void {
    switch (f.type) {
      case "view":
        this.page = f.page;
        this.screen = f.screen;
        this.laser = f.laser;
        this.away = f.away;
        this.#trim();
        if (f.screen === "show" && !this.pages.has(f.page) && !this.#asked.has(f.page)) {
          this.#asked.add(f.page);
          this.#client?.send({ type: "need", n: f.page });
        }
        return;
      case "page": {
        const next = new Map(this.pages);
        next.set(f.n, { src: f.src, width: f.width, height: f.height, rev: f.rev });
        this.pages = next;
        this.#asked.delete(f.n);
        this.#trim();
        return;
      }
      case "roster":
        this.windows = f.windows;
        return;
    }
  }

  /** A key in this window: the presenter moves, not this window. True when the key was for the slides. */
  key(e: { key: string; ctrlKey?: boolean; altKey?: boolean; metaKey?: boolean }): boolean {
    const a = this.#keys.read(e);
    if (!a) return false;
    const op: Command | undefined =
      a.kind === "goto" || a.kind === "laser" ? undefined : (a.kind as Command);
    if (op) this.#client?.send({ type: "cmd", op });
    return op !== undefined;
  }

  #trim(): void {
    if (this.page === 0) return; // nothing shown yet: nothing is far
    const far = [...this.pages.keys()].filter((n) => Math.abs(n - this.page) > KEEP);
    if (far.length === 0) return;
    const next = new Map(this.pages);
    for (const n of far) next.delete(n);
    this.pages = next;
  }
}
