import type { ViewState, WireMessage } from "./wire";

/** How long a pilot may stay silent before the others say it is gone. The pilot repeats its state more often. */
export const PILOT_TTL_MS = 6000;
/** How often the pilot repeats its state, so that a late arrival catches up and the others know it lives. */
export const BEAT_MS = 2000;
/** How long a request for the controls waits for an answer. */
export const ASK_MS = 20_000;
const MAX_REQUESTS = 8;

export interface PilotEnv {
  /** This device's peer id. */
  me: string;
  /** Sends to everyone in the project. */
  send(m: WireMessage): void;
  /** Whether a peer may pilot: a writer or an admin, not suspended. Readers never do. */
  canPilot(peer: string): boolean;
  now(): number;
}

const beats = (a: { epoch: number; peer: string }, b: { epoch: number; peer: string }): boolean =>
  a.epoch > b.epoch || (a.epoch === b.epoch && a.peer < b.peer);

/**
 * Who pilots a presentation and what they show. One person pilots at a time, whoever the others
 * are: the pilot has an epoch that only grows, a message from an older epoch is ignored, and two
 * who claim the same epoch at once are told apart by peer id, the same on every device, so that
 * the navigation can never contradict itself. Only a device that may write can hold the controls
 * (the check is made on the receiving side, on what the signed member list says).
 *
 * The class holds no timer: the owner calls {@link beat} and {@link tick}.
 */
export class Pilot {
  /** The peer who pilots, if anyone does. */
  pilot: string | undefined;
  epoch = 0;
  /** What the pilot shows; stays when the pilot goes, so that the public view keeps the last slide. */
  view: ViewState | undefined;
  /** The laser pointer as fractions of the slide, null when off. */
  laser: { x: number; y: number } | null = null;
  /** People who asked me for the controls, oldest first (when I am the pilot). */
  requests: string[] = [];
  /** I asked for the controls and have no answer yet. */
  asking = false;
  /** The pilot said no to my request (until the next try). */
  denied = false;
  /** False when the pilot has been silent too long or left. */
  connected = true;
  /** The pilot ended the presentation. */
  ended = false;

  readonly #env: PilotEnv;
  readonly #changed: () => void;
  #seq = 0;
  #lastSeq = -1;
  #lastSeen = 0;
  #askedAt = 0;

  constructor(env: PilotEnv, onChange: () => void = () => {}) {
    this.#env = env;
    this.#changed = onChange;
  }

  get isPilot(): boolean {
    return this.pilot === this.#env.me;
  }

  /** True when someone other than me holds the controls and is there. */
  get otherPilots(): boolean {
    return this.pilot !== undefined && !this.isPilot && this.connected;
  }

  /** I may take the controls now: I am allowed, and nobody else holds them. */
  get canTake(): boolean {
    return this.#env.canPilot(this.#env.me) && !this.isPilot && !this.otherPilots;
  }

  /** Takes the controls if allowed and free. Returns whether I hold them. */
  take(): boolean {
    if (this.isPilot) return true;
    if (!this.canTake) return false;
    this.#become(this.epoch + 1);
    this.#env.send({ t: "take", epoch: this.epoch });
    if (this.view) this.publish(this.view);
    return true;
  }

  /** Says that this device has just arrived: the pilot answers with what it shows. */
  hello(): void {
    this.#env.send({ t: "hello" });
  }

  /** Asks the pilot for the controls. Returns false when there is no one to ask. */
  ask(): boolean {
    if (!this.otherPilots || !this.#env.canPilot(this.#env.me)) return false;
    this.asking = true;
    this.denied = false;
    this.#askedAt = this.#env.now();
    this.#env.send({ t: "ask" });
    this.#changed();
    return true;
  }

  /** Hands the controls to someone who may hold them. */
  grant(peer: string): boolean {
    if (!this.isPilot || !this.#env.canPilot(peer)) return false;
    this.epoch++;
    this.#env.send({ t: "give", to: peer, epoch: this.epoch });
    this.pilot = peer;
    this.#lastSeen = this.#env.now();
    this.connected = true;
    this.requests = [];
    this.laser = null;
    this.#changed();
    return true;
  }

  deny(peer: string): void {
    if (!this.isPilot) return;
    this.requests = this.requests.filter((p) => p !== peer);
    this.#env.send({ t: "deny", to: peer });
    this.#changed();
  }

  /** The pilot stops presenting. */
  release(): void {
    if (!this.isPilot) return;
    this.#env.send({ t: "end", epoch: this.epoch });
    this.pilot = undefined;
    this.requests = [];
    this.laser = null;
    this.#changed();
  }

  /** The pilot shows `view`: everyone follows. Ignored by anyone but the pilot. */
  publish(view: ViewState): void {
    if (!this.isPilot) return;
    this.view = view;
    this.#seq++;
    this.#send(view);
  }

  /** The pilot moves the laser (fractions of the slide) or switches it off with null. */
  publishLaser(point: { x: number; y: number } | null): void {
    if (!this.isPilot) return;
    this.laser = point;
    this.#env.send({ t: "laser", epoch: this.epoch, x: point?.x ?? null, y: point?.y ?? null });
  }

  /** Repeats the state: call every {@link BEAT_MS} while piloting. */
  beat(): void {
    if (this.isPilot && this.view) this.#send(this.view);
  }

  /** Checks that the pilot is still there and that a request has not waited for ever. */
  tick(): void {
    const now = this.#env.now();
    if (this.pilot && !this.isPilot && this.connected && now - this.#lastSeen > PILOT_TTL_MS) {
      this.connected = false;
      this.#changed();
    }
    if (this.asking && now - this.#askedAt > ASK_MS) {
      this.asking = false;
      this.#changed();
    }
  }

  /** The link to `peer` went down. */
  peerGone(peer: string): void {
    if (peer === this.pilot && !this.isPilot && this.connected) {
      this.connected = false;
      this.#changed();
    }
    if (this.requests.includes(peer)) {
      this.requests = this.requests.filter((p) => p !== peer);
      this.#changed();
    }
  }

  /** A message from `from`, whose identity the transport vouches for. */
  receive(from: string, m: WireMessage): void {
    if (from === this.#env.me) return;
    switch (m.t) {
      case "state":
        if (!this.#env.canPilot(from)) return;
        if (!this.#accept(from, m.epoch)) return;
        if (m.seq < this.#lastSeq) return;
        this.#lastSeq = m.seq;
        this.view = { slide: m.slide, step: m.step, screen: m.screen };
        this.#alive();
        return;
      case "laser":
        if (from !== this.pilot || m.epoch !== this.epoch) return;
        this.laser = m.x === null || m.y === null ? null : { x: m.x, y: m.y };
        this.#alive();
        return;
      case "take":
        if (this.#env.canPilot(from) && this.#accept(from, m.epoch)) this.#alive();
        return;
      case "hello":
        this.beat();
        return;
      case "ask":
        if (!this.isPilot || !this.#env.canPilot(from) || this.requests.includes(from)) return;
        this.requests = [...this.requests, from].slice(-MAX_REQUESTS);
        this.#changed();
        return;
      case "give":
        if (from !== this.pilot || m.epoch <= this.epoch || !this.#env.canPilot(m.to)) return;
        this.epoch = m.epoch;
        this.pilot = m.to;
        this.asking = false;
        this.laser = null;
        this.#lastSeq = -1;
        this.#alive();
        return;
      case "deny":
        if (m.to !== this.#env.me || !this.asking) return;
        this.asking = false;
        this.denied = true;
        this.#changed();
        return;
      case "end":
        if (from !== this.pilot || m.epoch !== this.epoch) return;
        this.pilot = undefined;
        this.laser = null;
        this.ended = true;
        this.#changed();
        return;
    }
  }

  /** Whether the claim of `peer` at `epoch` is the one to follow; adopts it if so. */
  #accept(peer: string, epoch: number): boolean {
    const wins =
      this.pilot === undefined
        ? epoch >= this.epoch // nobody pilots: only an older claim than the last one is stale
        : beats({ epoch, peer }, { epoch: this.epoch, peer: this.pilot });
    if (wins) {
      this.pilot = peer;
      this.epoch = epoch;
      this.#lastSeq = -1;
      this.requests = [];
      this.asking = false;
      this.laser = null;
      this.ended = false;
      return true;
    }
    return peer === this.pilot && epoch === this.epoch;
  }

  #become(epoch: number): void {
    this.pilot = this.#env.me;
    this.epoch = epoch;
    this.connected = true;
    this.ended = false;
    this.asking = false;
    this.#seq = 0;
    this.laser = null;
    this.#changed();
  }

  #alive(): void {
    this.#lastSeen = this.#env.now();
    this.connected = true;
    this.#changed();
  }

  #send(view: ViewState): void {
    this.#env.send({ t: "state", epoch: this.epoch, seq: this.#seq, ...view });
  }
}
