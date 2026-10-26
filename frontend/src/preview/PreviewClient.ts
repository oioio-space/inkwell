import type { SocketLike } from "../net/WailsNetworkAdapter";
import { encodeRequest, type PreviewMessage, type PreviewRequest, parseMessage } from "./protocol";

export interface PreviewClientOptions {
  /** Delay before reopening a stream that closed. */
  reconnectMs?: number;
}

/**
 * The webview's end of the `preview` stream. It reopens the stream when it closes (Go serves one
 * page at a time, and a reload replaces the previous one) and, after each opening, says again
 * what Go needs to know: the pages in view and when the document last changed.
 */
export class PreviewClient {
  readonly #open: () => SocketLike;
  readonly #onMessage: (m: PreviewMessage) => void;
  readonly #reconnectMs: number;
  #socket: SocketLike | undefined;
  #ready = false;
  #stopped = true;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #view: PreviewRequest | undefined;
  #edit: PreviewRequest | undefined;

  constructor(
    open: () => SocketLike,
    onMessage: (m: PreviewMessage) => void,
    opts: PreviewClientOptions = {},
  ) {
    this.#open = open;
    this.#onMessage = onMessage;
    this.#reconnectMs = opts.reconnectMs ?? 1000;
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

  /** These pages are in view (1-based, inclusive). */
  view(first: number, last: number): void {
    this.#view = { op: "view", first, last };
    this.#send(this.#view);
  }

  /** The document last changed at `seconds` (Unix time). */
  edit(seconds: number): void {
    this.#edit = { op: "edit", time: Math.floor(seconds) };
    this.#send(this.#edit);
  }

  /** Ask where the headings and paragraphs are; the answer arrives as an "anchors" message. */
  anchors(): void {
    this.#send({ op: "anchors" });
  }

  #send(req: PreviewRequest): void {
    if (!this.#ready || !this.#socket) return; // `view` and `edit` are repeated after opening
    try {
      this.#socket.send(encodeRequest(req));
    } catch {
      // The stream is closing: the reconnection says it all again.
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
      if (this.#edit) this.#send(this.#edit);
      if (this.#view) this.#send(this.#view);
    };
    socket.onmessage = (ev) => {
      const data = ev.data as ArrayBuffer | string;
      const m = parseMessage(typeof data === "string" ? data : new Uint8Array(data));
      if (m) this.#onMessage(m);
      else console.warn("preview: dropped an undecodable frame");
    };
    socket.onclose = () => {
      if (this.#socket !== socket) return;
      this.#socket = undefined;
      this.#ready = false;
      if (!this.#stopped) this.#timer = setTimeout(() => this.#connect(), this.#reconnectMs);
    };
  }
}
