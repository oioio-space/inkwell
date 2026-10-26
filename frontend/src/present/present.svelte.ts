import type { Page } from "../preview/preview.svelte";
import type { DeckData } from "../preview/protocol";
import type { Channel } from "./channel";
import { pace, Stopwatch } from "./clock";
import { Deck, type Pos } from "./deck";
import type { PublicView } from "./feed";
import type { Command } from "./hub";
import { type Action, KeyReader } from "./keys";
import { BEAT_MS, Pilot } from "./pilot";
import type { ScreenMode } from "./wire";

/**
 * What a window does with a presentation. "presenter" pilots (current slide, next, notes, timer);
 * "follower" watches someone else's, and may browse on its own; "public" is the slide alone, driven
 * by the pilot, on a machine that is plugged to the projector.
 */
export type Mode = "off" | "presenter" | "follower" | "public";

/** What the presentation needs from the preview of the document. */
export interface PageSource {
  readonly pageCount: number;
  readonly rev: number;
  readonly deck: DeckData | undefined;
  readonly pages: ReadonlyMap<number, Page>;
  setVisible(first: number, last: number): void;
  wantDeck(on: boolean): void;
}

export interface PresentDeps {
  /** This device's peer id. */
  me: string;
  /** Whether a peer may pilot (see `Workspace.canPilot`). */
  canPilot(peer: string): boolean;
  nameOf(peer: string): string;
  channel: Channel;
  preview: PageSource;
  now?: () => number;
}

/** The pages kept ready around the one shown: the next steps appear without waiting. */
const PREFETCH = 2;

/**
 * One window's presentation: where it stands, who pilots, what it shows. There is one state of
 * navigation: the pilot's. A pilot's keys move it and everyone who follows; a follower's own keys
 * move nothing but its own view, and only if it browses freely. Whatever arrives from the
 * network is judged by {@link Pilot}, which only believes devices that may write.
 */
export class Presentation {
  mode = $state<Mode>("off");
  /** Whether a window that does not pilot follows the pilot or browses on its own. */
  follow = $state<"follow" | "free">("follow");
  /** The laser tool is on (the pointer follows the mouse on the slide). */
  laserOn = $state(false);
  /** The digits typed for "go to slide". */
  typed = $state("");
  /** A page to start from, until the deck says which slide it is (see {@link resolvePage}). */
  pendingPage = $state<number | undefined>();
  /** Seconds-resolution clock for the timer, refreshed once a second while presenting. */
  now = $state(0);

  /** The timer is running (the stopwatch itself is not reactive). */
  timerRunning = $state(false);

  readonly stopwatch: Stopwatch;
  #pos = $state<Pos>({ slide: 1, step: 0 });
  #screen = $state<ScreenMode>("show");
  #ver = $state(0);
  readonly #pilot: Pilot;
  readonly #deps: PresentDeps;
  readonly #keys = new KeyReader();
  #stop: (() => void) | undefined;
  #timer: ReturnType<typeof setInterval> | undefined;
  #beats = 0;
  #laserSent = 0;

  /** The document as slides and steps; follows the preview as it is compiled again. */
  readonly deck: Deck;

  constructor(deps: PresentDeps) {
    this.#deps = deps;
    this.deck = $derived(new Deck(deps.preview.deck, deps.preview.pageCount));
    const clock = deps.now ?? Date.now;
    this.stopwatch = new Stopwatch(clock);
    this.now = clock();
    this.#pilot = new Pilot(
      {
        me: deps.me,
        now: clock,
        canPilot: (peer) => deps.canPilot(peer),
        send: (m) => deps.channel.broadcast(m),
      },
      () => {
        this.#ver++;
      },
    );
  }

  startTimer(): void {
    this.stopwatch.start();
    this.timerRunning = this.stopwatch.running;
  }

  pauseTimer(): void {
    this.stopwatch.pause();
    this.timerRunning = this.stopwatch.running;
  }

  resetTimer(): void {
    this.stopwatch.reset();
    this.timerRunning = false;
  }

  // --- who pilots -------------------------------------------------------------------------

  /** I hold the controls. */
  get isPilot(): boolean {
    void this.#ver;
    return this.#pilot.isPilot;
  }

  /** The person who pilots, if any. */
  get pilotPeer(): string | undefined {
    void this.#ver;
    return this.#pilot.pilot;
  }

  get pilotName(): string {
    const peer = this.pilotPeer;
    return peer === undefined ? "" : this.#deps.nameOf(peer);
  }

  /** The pilot has gone quiet or left: the slide stays, with a mention. */
  get away(): boolean {
    void this.#ver;
    return this.#pilot.pilot !== undefined && !this.#pilot.isPilot && !this.#pilot.connected;
  }

  /** Nobody pilots: the presentation is waiting for its presenter. */
  get waiting(): boolean {
    void this.#ver;
    return this.#pilot.pilot === undefined;
  }

  /** The presentation was ended by its pilot. */
  get ended(): boolean {
    void this.#ver;
    return this.#pilot.ended;
  }

  /** I may take the controls now. */
  get canTake(): boolean {
    void this.#ver;
    return this.#pilot.canTake;
  }

  /** I may ask the pilot for the controls. */
  get canAsk(): boolean {
    void this.#ver;
    return this.#pilot.otherPilots && this.#deps.canPilot(this.#deps.me) && !this.#pilot.asking;
  }

  get asking(): boolean {
    void this.#ver;
    return this.#pilot.asking;
  }

  get denied(): boolean {
    void this.#ver;
    return this.#pilot.denied;
  }

  /** People who asked me for the controls, with their names. */
  get requests(): { peer: string; name: string }[] {
    void this.#ver;
    return this.#pilot.requests.map((peer) => ({ peer, name: this.#deps.nameOf(peer) }));
  }

  /** My own role allows piloting (readers show and follow only). */
  get mayPilot(): boolean {
    return this.#deps.canPilot(this.#deps.me);
  }

  // --- what is shown ----------------------------------------------------------------------

  /** True when this window shows the pilot's slide, not its own. */
  get following(): boolean {
    void this.#ver;
    if (this.#pilot.isPilot) return false;
    if (this.mode === "public") return true;
    return this.follow === "follow" && this.#pilot.view !== undefined;
  }

  /** Where this window is: the pilot's place when following, else its own, always a place of the deck. */
  get pos(): Pos {
    void this.#ver;
    const v = this.following ? this.#pilot.view : undefined;
    return this.deck.clamp(v ? { slide: v.slide, step: v.step } : this.#pos);
  }

  get screen(): ScreenMode {
    void this.#ver;
    return this.following ? (this.#pilot.view?.screen ?? "show") : this.#screen;
  }

  /** The laser pointer as seen here: the pilot's when following, mine when I pilot with the tool on. */
  get laser(): { x: number; y: number } | null {
    void this.#ver;
    return this.#pilot.laser;
  }

  /** The page of the document that shows the current place. */
  get page(): number {
    return this.deck.pageOf(this.pos);
  }

  /** The next place, for the "next" thumbnail. */
  get upcoming() {
    return this.deck.upcoming(this.pos);
  }

  get notes(): string {
    return this.deck.noteOf(this.pos);
  }

  /** The slide's number among those shown, from 1. */
  get index(): number {
    return this.deck.indexOf(this.pos);
  }

  /** How the talk stands against its plan; undefined without a planned duration. */
  get pace() {
    return pace(this.stopwatch.elapsedMs(), this.deck.minutes, this.index, this.deck.count);
  }

  /** What the public window shows, for {@link PublicFeed}. */
  get publicView(): PublicView {
    const p = this.#deps.preview;
    return {
      page: this.page,
      pageCount: p.pageCount,
      screen: this.screen,
      laser: this.screen === "show" ? this.laser : null,
      away: this.away,
      rev: p.rev,
      pages: p.pages,
    };
  }

  // --- life -------------------------------------------------------------------------------

  /**
   * Begins presenting in this window. A presenter takes the controls if nobody holds them (and the
   * person may write); otherwise it shows the pilot's slide until the controls are asked for and
   * granted. It starts from `from.pos`, or from the slide that shows page `from.page` once the deck is
   * known, by default the first slide.
   */
  start(mode: Exclude<Mode, "off">, from: { pos?: Pos; page?: number } = {}): void {
    if (this.mode !== "off") return;
    this.mode = mode;
    this.follow = "follow";
    this.#screen = "show";
    this.#deps.preview.wantDeck(true);
    this.#pos = from.pos ?? this.deck.first();
    this.pendingPage = from.page;
    this.#stop = this.#deps.channel.subscribe((from, m) => this.#pilot.receive(from, m));
    this.now = (this.#deps.now ?? Date.now)();
    this.#timer = setInterval(() => this.#tick(), 1000);
    this.#pilot.hello();
    if (mode === "presenter") this.take();
  }

  /** The deck is known: moves to the slide of the page this presentation was asked to start from. */
  resolvePage(): void {
    const n = this.pendingPage;
    if (n === undefined) return;
    this.pendingPage = undefined;
    this.#pos = this.deck.posOfPage(n);
    if (this.isPilot) this.#publish();
  }

  /** Leaves the presentation; returns the page it was on. The controls are released. */
  stop(): number {
    const page = this.page;
    this.#pilot.release();
    this.#stop?.();
    this.#stop = undefined;
    clearInterval(this.#timer);
    this.#timer = undefined;
    this.#deps.preview.wantDeck(false);
    this.mode = "off";
    this.laserOn = false;
    this.resetTimer();
    return page;
  }

  /** The members' rights changed: a pilot who may no longer pilot loses the controls. */
  revalidate(): void {
    this.#pilot.revalidate();
  }

  /** A link went down: if it was the pilot's, the slide stays and says so. */
  peerGone(peer: string): void {
    this.#pilot.peerGone(peer);
  }

  #tick(): void {
    this.now = (this.#deps.now ?? Date.now)();
    this.#pilot.tick();
    if (++this.#beats % Math.round(BEAT_MS / 1000) === 0) this.#pilot.beat();
  }

  // --- the controls -----------------------------------------------------------------------

  /** Takes the controls from a pilot who is gone, or from no one. */
  take(): boolean {
    if (!this.mayPilot) return false;
    this.#keepOwnPlace(); // from where the last pilot stopped, not from where this window was
    if (!this.#pilot.take()) return false;
    this.#publish();
    return true;
  }

  /** Asks the pilot for the controls ("Prendre la main"). */
  ask(): void {
    this.#pilot.ask();
  }

  grant(peer: string): void {
    this.#pilot.grant(peer);
    this.#keepOwnPlace();
  }

  deny(peer: string): void {
    this.#pilot.deny(peer);
  }

  /** "Parcourir librement" / "Revenir au présentateur". */
  setFollow(mode: "follow" | "free"): void {
    if (mode === "free") this.#pos = this.pos; // browse from where the pilot is
    this.follow = mode;
  }

  /** Makes the place and screen shown here this window's own, so that they survive a change of pilot. */
  #keepOwnPlace(): void {
    this.#pos = this.pos;
    this.#screen = this.screen;
  }

  /** What a key does in this window; false when the key is not for the presentation. */
  key(e: { key: string; ctrlKey?: boolean; altKey?: boolean; metaKey?: boolean }): boolean {
    const action = this.#keys.read(e);
    this.typed = this.#keys.buffer;
    if (!action) return this.typed !== "" || e.key === "Escape";
    this.act(action);
    return true;
  }

  /** A command from the public window's keyboard. */
  command(op: Command): void {
    this.act({ kind: op });
  }

  act(a: Action): void {
    switch (a.kind) {
      case "quit":
        this.stop();
        return;
      case "laser":
        this.toggleLaser();
        return;
      case "black":
      case "white":
        this.#toggleScreen(a.kind);
        return;
    }
    if (!this.#canMove()) return;
    const from = this.pos;
    let to: Pos;
    switch (a.kind) {
      case "next":
        to = this.deck.next(from);
        break;
      case "prev":
        to = this.deck.prev(from);
        break;
      case "first":
        to = this.deck.first();
        break;
      case "last":
        to = this.deck.last();
        break;
      case "goto":
        to = this.deck.nth(a.n);
        break;
    }
    this.#move(from, to);
    if (a.kind === "next" && this.isPilot && !this.stopwatch.started) this.startTimer();
  }

  /** Whether this window's keys move the slide: the pilot's do, a browsing follower's move its own view. */
  #canMove(): boolean {
    return this.isPilot || (this.mode !== "public" && this.follow === "free");
  }

  #move(from: Pos, to: Pos): void {
    this.#pos = to;
    if (this.isPilot) {
      // A slide change clears the pointer: it points at what is no longer there.
      if (to.slide !== from.slide && this.#pilot.laser) this.#pilot.publishLaser(null);
      this.#publish();
    }
  }

  #toggleScreen(kind: "black" | "white"): void {
    if (!this.isPilot) return;
    this.#screen = this.#screen === kind ? "show" : kind;
    this.#publish();
  }

  toggleLaser(): void {
    if (!this.isPilot) return;
    this.laserOn = !this.laserOn;
    if (!this.laserOn) this.#pilot.publishLaser(null);
  }

  /** The mouse is at (x, y), fractions of the slide, with the laser on. At most about 30 a second. */
  moveLaser(x: number, y: number): void {
    if (!this.isPilot || !this.laserOn) return;
    const t = (this.#deps.now ?? Date.now)();
    if (t - this.#laserSent < 30) return;
    this.#laserSent = t;
    this.#pilot.publishLaser({ x: Math.min(1, Math.max(0, x)), y: Math.min(1, Math.max(0, y)) });
  }

  #publish(): void {
    const at = this.deck.clamp(this.#pos);
    this.#pilot.publish({ slide: at.slide, step: at.step, screen: this.#screen });
  }

  /** The pages around the place are asked of Go: the one shown and the next ones. */
  wantedPages(): { first: number; last: number } {
    const n = this.page;
    return {
      first: Math.max(1, n - 1),
      last: Math.min(Math.max(1, this.#deps.preview.pageCount), n + PREFETCH),
    };
  }
}
