import type { Page } from "../preview/preview.svelte";
import type { DeckData, DeckPage } from "../preview/protocol";
import type { Channel } from "./channel";
import { type PageSource, Presentation } from "./present.svelte";
import type { WireMessage } from "./wire";

export const deckPage = (slide: number, step: number, note = ""): DeckPage => ({
  slide,
  step,
  hidden: false,
  note,
});

/** Slide 1; slide 2 in three steps with a note; slide 3. Five pages, a planned 20 minutes. */
export const DECK: DeckData = {
  minutes: 20,
  pages: [
    deckPage(1, 0),
    deckPage(2, 0, "Souriez"),
    deckPage(2, 1),
    deckPage(2, 2),
    deckPage(3, 0),
  ],
};

/** The preview of the document as a presentation sees it. */
export class FakePreview implements PageSource {
  pageCount = $state(5);
  rev = $state(1);
  deck = $state.raw<DeckData | undefined>(DECK);
  pages = $state.raw<ReadonlyMap<number, Page>>(new Map());
  deckWanted = false;
  visible: [number, number][] = [];

  setVisible(first: number, last: number): void {
    this.visible.push([first, last]);
  }

  wantDeck(on: boolean): void {
    this.deckWanted = on;
  }
}

/** Devices on one network: a message sent by one reaches the others at once, unless cut. */
export class Link {
  readonly roles = new Map<string, "reader" | "writer" | "admin">();
  readonly cut = new Set<string>();
  readonly #listeners = new Map<string, (from: string, m: WireMessage) => void>();
  readonly sent: { from: string; m: WireMessage }[] = [];
  now = 1_000_000;

  channel(me: string): Channel {
    return {
      broadcast: (m) => {
        this.sent.push({ from: me, m });
        for (const [id, cb] of this.#listeners) {
          if (id === me || this.cut.has(`${me}|${id}`) || this.cut.has(`${id}|${me}`)) continue;
          cb(me, structuredClone(m));
        }
      },
      subscribe: (cb) => {
        this.#listeners.set(me, cb);
        return () => this.#listeners.delete(me);
      },
    };
  }

  canPilot = (peer: string): boolean => {
    const role = this.roles.get(peer);
    return role !== undefined && role !== "reader";
  };

  /** A device with a role, running a presentation over this link. */
  device(id: string, role: "reader" | "writer" | "admin" = "writer") {
    this.roles.set(id, role);
    const preview = new FakePreview();
    const presentation = new Presentation({
      me: id,
      canPilot: this.canPilot,
      nameOf: (p) => `Nom de ${p}`,
      channel: this.channel(id),
      preview,
      now: () => this.now,
    });
    return { id, preview, presentation };
  }
}
