import type { SocketLike } from "../net/WailsNetworkAdapter";
import type { ScreenMode } from "./wire";

/**
 * The `present` stream joins the windows of a presentation on this machine: the presenter view,
 * which owns the project streams, and the public view, which opens none (they serve one window
 * at a time). The presenter sends what the public view must show; the public view sends back the
 * keys pressed in it. Notes and comments never go this way: the public window cannot show what
 * it never receives.
 */

/** A command from the keyboard of the public window. */
export type Command = "next" | "prev" | "first" | "last" | "black" | "white" | "quit";

const COMMANDS: readonly string[] = ["next", "prev", "first", "last", "black", "white", "quit"];

export type HubFrame =
  /** What the public view shows now; `page` is the page of the document, 1-based. */
  | {
      type: "view";
      page: number;
      screen: ScreenMode;
      laser: { x: number; y: number } | null;
      /** The pilot is gone: the last slide stays, with a discreet mention. */
      away: boolean;
    }
  /** One page, as an image to show. */
  | { type: "page"; n: number; rev: number; src: string; width: number; height: number }
  /** The public view has just opened (or lost a page): say it all again. */
  | { type: "hello" }
  | { type: "need"; n: number }
  | { type: "cmd"; op: Command }
  /** From the hub: how many windows are joined. */
  | { type: "roster"; windows: number };

const MAX_PAGE = 100_000;
const MAX_SRC = 8 << 20;
const DATA_SVG = "data:image/svg+xml;base64,";
const BASE64 = /^[A-Za-z0-9+/]*={0,2}$/;

const isObject = (v: unknown): v is Record<string, unknown> =>
  typeof v === "object" && v !== null && !Array.isArray(v);

const int = (v: unknown, max: number): number | undefined =>
  typeof v === "number" && Number.isInteger(v) && v >= 0 && v <= max ? v : undefined;

const fraction = (v: unknown): number | undefined =>
  typeof v === "number" && Number.isFinite(v) && v >= 0 && v <= 1 ? v : undefined;

const size = (v: unknown): number | undefined =>
  typeof v === "number" && Number.isFinite(v) && v > 0 && v < MAX_PAGE ? v : undefined;

/** Reads a frame of the stream; undefined (never a throw) for anything else. */
export function parseHub(frame: Uint8Array | string): HubFrame | undefined {
  let v: unknown;
  try {
    v = JSON.parse(typeof frame === "string" ? frame : new TextDecoder().decode(frame));
  } catch {
    return undefined;
  }
  if (!isObject(v)) return undefined;
  switch (v.type) {
    case "view": {
      const page = int(v.page, MAX_PAGE);
      if (!page) return undefined;
      if (v.screen !== "show" && v.screen !== "black" && v.screen !== "white") return undefined;
      let laser: { x: number; y: number } | null = null;
      if (v.laser !== null && v.laser !== undefined) {
        if (!isObject(v.laser)) return undefined;
        const x = fraction(v.laser.x);
        const y = fraction(v.laser.y);
        if (x === undefined || y === undefined) return undefined;
        laser = { x, y };
      }
      return { type: "view", page, screen: v.screen, laser, away: v.away === true };
    }
    case "page": {
      const n = int(v.n, MAX_PAGE);
      const width = size(v.width);
      const height = size(v.height);
      if (!n || !width || !height || typeof v.src !== "string") return undefined;
      // An empty source is a page that could not be shown safely; anything else is a data image.
      const ok =
        v.src === "" ||
        (v.src.length <= MAX_SRC &&
          v.src.startsWith(DATA_SVG) &&
          BASE64.test(v.src.slice(DATA_SVG.length)));
      if (!ok) return undefined;
      return {
        type: "page",
        n,
        rev: int(v.rev, Number.MAX_SAFE_INTEGER) ?? 0,
        src: v.src,
        width,
        height,
      };
    }
    case "hello":
      return { type: "hello" };
    case "need": {
      const n = int(v.n, MAX_PAGE);
      return n ? { type: "need", n } : undefined;
    }
    case "cmd":
      return typeof v.op === "string" && COMMANDS.includes(v.op)
        ? { type: "cmd", op: v.op as Command }
        : undefined;
    case "roster": {
      const windows = int(v.windows, 64);
      return windows === undefined ? undefined : { type: "roster", windows };
    }
  }
  return undefined;
}

const encoder = new TextEncoder();

export function encodeHub(f: HubFrame): Uint8Array {
  return encoder.encode(JSON.stringify(f));
}

/**
 * The webview's end of the `present` stream. It reopens the stream when it closes and tells its
 * owner each time it opens, so that the owner can say what the other window needs again.
 */
export class HubClient {
  readonly #open: () => SocketLike;
  readonly #onFrame: (f: HubFrame) => void;
  readonly #onOpen: () => void;
  readonly #reconnectMs: number;
  #socket: SocketLike | undefined;
  #ready = false;
  #stopped = true;
  #timer: ReturnType<typeof setTimeout> | undefined;

  constructor(
    open: () => SocketLike,
    onFrame: (f: HubFrame) => void,
    onOpen: () => void = () => {},
    reconnectMs = 1000,
  ) {
    this.#open = open;
    this.#onFrame = onFrame;
    this.#onOpen = onOpen;
    this.#reconnectMs = reconnectMs;
  }

  start(): void {
    if (!this.#stopped) return;
    this.#stopped = false;
    this.#connect();
  }

  stop(): void {
    this.#stopped = true;
    clearTimeout(this.#timer);
    const socket = this.#socket;
    this.#socket = undefined;
    this.#ready = false;
    socket?.close();
  }

  /** Sends a frame; dropped while the stream is not open (the owner says it all again on opening). */
  send(f: HubFrame): void {
    if (!this.#ready || !this.#socket) return;
    try {
      this.#socket.send(encodeHub(f));
    } catch {
      // Closing: the reconnection starts over.
    }
  }

  #connect(): void {
    if (this.#stopped) return;
    const socket = this.#open();
    socket.binaryType = "arraybuffer";
    this.#socket = socket;
    socket.onopen = () => {
      if (this.#socket !== socket) return;
      this.#ready = true;
      this.#onOpen();
    };
    socket.onmessage = (ev) => {
      const data = ev.data as ArrayBuffer | string;
      const f = parseHub(typeof data === "string" ? data : new Uint8Array(data));
      if (f) this.#onFrame(f);
    };
    socket.onclose = () => {
      if (this.#socket !== socket) return;
      this.#socket = undefined;
      this.#ready = false;
      if (!this.#stopped) this.#timer = setTimeout(() => this.#connect(), this.#reconnectMs);
    };
  }
}
