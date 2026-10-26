import { Stream } from "@wailsio/runtime";
import { PresentService } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import { preview } from "../preview/preview.svelte";
import { safeGet, safeSet } from "../store/safeStorage";
import { workspace } from "../workspace/workspace.svelte";
import { handleChannel } from "./channel";
import type { Pos } from "./deck";
import { PublicFeed } from "./feed";
import { HubClient, type HubFrame } from "./hub";
import { type Mode, Presentation } from "./present.svelte";
import { PublicWindows } from "./windows";

const NOTES_KEY = "inkwell.present.notes";
const NOTES_MIN = 12;
const NOTES_MAX = 40;
const NOTES_DEFAULT = 18;
/** How long a presenter's last message keeps "X présente" on the screen. */
const ANNOUNCE_MS = 7000;

/**
 * The presentation of this window and what surrounds it: the stream to the public window, the
 * window layer, and the watch that tells the people of a project that someone is presenting.
 */
class PresentSession {
  /** The running presentation, if this window presents, follows or shows. */
  current = $state.raw<Presentation | undefined>();
  /** Windows joined to the presentation on this machine (the public window makes two). */
  roster = $state(0);
  /** The public window is open on this machine. */
  publicOpen = $state(false);
  /** The public view has a screen of its own. */
  dual = $state(false);
  swapped = $state(false);
  /** Someone else is presenting and this window is not in the presentation. */
  announced = $state.raw<{ peer: string; name: string } | undefined>();
  notesSize = $state(clampNotes(Number(safeGet(NOTES_KEY))));

  #hub: HubClient | undefined;
  #feed: PublicFeed | undefined;
  #windows = new PublicWindows(PresentService, window);
  #watchStop: (() => void) | undefined;
  #expire: ReturnType<typeof setTimeout> | undefined;

  /** Keeps an eye on the project's presenters while this window is not in a presentation. */
  watch(): () => void {
    this.#watchStop?.();
    const handle = workspace.handle;
    if (!handle) return () => {};
    const stop = handleChannel(handle).subscribe((from, m) => {
      if (this.current) return;
      if (m.t === "end" && this.announced?.peer === from) {
        this.#clearAnnounce();
        return;
      }
      if ((m.t !== "state" && m.t !== "take") || !workspace.canPilot(from)) return;
      this.announced = { peer: from, name: workspace.nameOf(from) };
      clearTimeout(this.#expire);
      this.#expire = setTimeout(() => this.#clearAnnounce(), ANNOUNCE_MS);
    });
    this.#watchStop = stop;
    return () => {
      stop();
      this.#clearAnnounce();
    };
  }

  #clearAnnounce(): void {
    clearTimeout(this.#expire);
    this.announced = undefined;
  }

  /** Begins a presentation in this window, from the first slide or from `from`. */
  start(mode: Exclude<Mode, "off">, from: { pos?: Pos; page?: number } = {}): void {
    const handle = workspace.handle;
    if (this.current || !handle) return;
    this.#clearAnnounce();
    const p = new Presentation({
      me: workspace.myId,
      canPilot: (peer) => workspace.canPilot(peer),
      nameOf: (peer) => workspace.nameOf(peer),
      channel: handleChannel(handle),
      preview,
    });
    this.current = p;
    p.start(mode, from);
    if (mode !== "public") this.#openHub(p);
  }

  /** Ends the presentation here, closes the public window, and returns the page it was on. */
  stop(): number {
    const p = this.current;
    if (!p) return 0;
    const page = p.stop();
    this.current = undefined;
    this.#hub?.stop();
    this.#hub = undefined;
    this.#feed = undefined;
    this.roster = 0;
    if (this.publicOpen) void this.closePublic();
    return page;
  }

  #openHub(p: Presentation): void {
    this.#feed = new PublicFeed((f) => this.#hub?.send(f));
    this.#hub = new HubClient(
      () => Stream("present") as unknown as SocketLike,
      (f) => this.#frame(p, f),
    );
    this.#hub.start();
  }

  #frame(p: Presentation, f: HubFrame): void {
    switch (f.type) {
      case "roster":
        this.roster = f.windows;
        this.publicOpen = f.windows > 1;
        return;
      case "hello":
        this.#feed?.hello();
        return;
      case "need":
        this.#feed?.need(f.n);
        return;
      case "cmd":
        p.command(f.op);
        if (f.op === "quit") this.stop();
        return;
    }
  }

  /** Brings the public window up to date with what the presentation shows now. */
  feed(p: Presentation): void {
    this.#feed?.update(p.publicView);
  }

  async openPublic(): Promise<void> {
    const opened = await this.#windows.open(this.swapped);
    this.dual = opened.dual;
    this.publicOpen = true;
  }

  async closePublic(): Promise<void> {
    await this.#windows.close();
    this.publicOpen = false;
  }

  /** "Échanger les écrans". */
  async swap(): Promise<void> {
    this.swapped = !this.swapped;
    await this.openPublic();
  }

  /** A− and A+ of the notes: remembered. */
  setNotesSize(size: number): void {
    this.notesSize = clampNotes(size);
    safeSet(NOTES_KEY, String(this.notesSize));
  }
}

function clampNotes(n: number): number {
  return Number.isFinite(n) && n > 0
    ? Math.min(NOTES_MAX, Math.max(NOTES_MIN, Math.round(n)))
    : NOTES_DEFAULT;
}

export const presentSession = new PresentSession();
export { NOTES_MAX, NOTES_MIN };
