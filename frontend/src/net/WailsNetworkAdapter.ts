import {
  type AutomergeUrl,
  isValidAutomergeUrl,
  type Message,
  NetworkAdapter,
  type PeerId,
  type PeerMetadata,
} from "@automerge/automerge-repo";
import { decode, encode } from "cbor-x";
import { decodeEnvelope, encodeEnvelope, Kind, MalformedEnvelope } from "./envelope";

/**
 * The part of the WebSocket API the adapter uses. The Wails `Stream("net")` object
 * implements it, and so do the test doubles.
 */
export interface SocketLike {
  binaryType: string;
  send(data: Uint8Array): void;
  close(): void;
  onopen: ((ev: Event) => unknown) | null;
  onclose: ((ev: Event) => unknown) | null;
  onmessage: ((ev: MessageEvent) => unknown) | null;
}

/** Repo-level handshake carried inside a Message payload; Go never sees it. */
type Handshake = {
  type: "arrive" | "welcome";
  senderId: PeerId;
  peerMetadata: PeerMetadata;
  projectUrl?: AutomergeUrl;
};

export interface AdapterOptions {
  /** Project document announced to peers (the host sets it; a joiner learns it). */
  projectUrl?: AutomergeUrl;
  /** Delay before reopening a socket that closed while connected. */
  reconnectDelayMs?: number;
  /** Diagnostics for dropped frames; never receives payload bytes. */
  onDrop?: (reason: string) => void;
}

/**
 * An automerge-repo NetworkAdapter over the Wails bridge (ADR 0009). Go routes
 * opaque frames between machines; the repo handshake (arrive/welcome) travels
 * inside the payloads, so a peer becomes a candidate only once its repo answers.
 */
export class WailsNetworkAdapter extends NetworkAdapter {
  #open: () => SocketLike;
  #socket: SocketLike | undefined;
  #opts: AdapterOptions;
  #projectUrl: AutomergeUrl | undefined;
  #announced = new Set<PeerId>();
  #projects = new Map<PeerId, AutomergeUrl>();
  #projectListeners: ((url: AutomergeUrl, peer: PeerId) => void)[] = [];
  #ready = false;
  #resolveReady: () => void = () => {};
  #readyPromise = new Promise<void>((resolve) => {
    this.#resolveReady = resolve;
  });
  #closed = false;

  constructor(open: () => SocketLike, opts: AdapterOptions = {}) {
    super();
    this.#open = open;
    this.#opts = opts;
    this.#projectUrl = opts.projectUrl;
  }

  isReady(): boolean {
    return this.#ready;
  }

  whenReady(): Promise<void> {
    return this.#readyPromise;
  }

  /** Calls cb whenever a peer announces its project. */
  onProject(cb: (url: AutomergeUrl, peer: PeerId) => void): void {
    this.#projectListeners.push(cb);
  }

  /** The project a connected peer announced, if any. */
  projectOf(peer: PeerId): AutomergeUrl | undefined {
    return this.#projects.get(peer);
  }

  /** Sets the project announced to peers met from now on. */
  setProject(url: AutomergeUrl): void {
    this.#projectUrl = url;
  }

  connect(peerId: PeerId, peerMetadata?: PeerMetadata): void {
    this.peerId = peerId;
    this.peerMetadata = peerMetadata ?? {};
    this.#closed = false;
    this.#dial();
  }

  send(message: Message): void {
    this.#sendTo(message.targetId, message);
  }

  disconnect(): void {
    this.#closed = true;
    this.#socket?.close();
    this.#socket = undefined;
    for (const peerId of this.#announced) this.emit("peer-disconnected", { peerId });
    this.#announced.clear();
    this.emit("close");
  }

  #dial(): void {
    const socket = this.#open();
    socket.binaryType = "arraybuffer";
    socket.onopen = () => {
      this.#ready = true;
      this.#resolveReady();
    };
    socket.onmessage = (ev) => this.#receive(new Uint8Array(ev.data as ArrayBuffer));
    socket.onclose = () => {
      if (this.#socket !== socket) return;
      for (const peerId of this.#announced) this.emit("peer-disconnected", { peerId });
      this.#announced.clear();
      if (!this.#closed) setTimeout(() => this.#dial(), this.#opts.reconnectDelayMs ?? 500);
    };
    this.#socket = socket;
  }

  #receive(frame: Uint8Array): void {
    let env: ReturnType<typeof decodeEnvelope>;
    try {
      env = decodeEnvelope(frame);
    } catch (err) {
      this.#opts.onDrop?.(err instanceof MalformedEnvelope ? err.message : "undecodable frame");
      return;
    }
    const peer = env.peer as PeerId;
    switch (env.kind) {
      case Kind.PeerUp:
        this.#sendHandshake(peer, "arrive");
        return;
      case Kind.PeerDown:
        this.#projects.delete(peer);
        if (this.#announced.delete(peer)) this.emit("peer-disconnected", { peerId: peer });
        return;
      case Kind.Message:
        this.#receiveMessage(peer, env.payload);
    }
  }

  #receiveMessage(from: PeerId, payload: Uint8Array): void {
    let msg: Handshake | Message;
    try {
      msg = decode(payload) as Handshake | Message;
    } catch {
      this.#opts.onDrop?.("undecodable payload");
      return;
    }
    // A peer may only speak for itself: the sender id is the one Go authenticated
    // at the transport layer, never what the payload claims.
    if (msg.senderId !== from) {
      this.#opts.onDrop?.("sender mismatch");
      return;
    }
    if (msg.type === "arrive" || msg.type === "welcome") {
      const hs = msg as Handshake;
      if (hs.type === "arrive") this.#sendHandshake(from, "welcome");
      this.#announce(from, hs);
      return;
    }
    if (!this.#announced.has(from) || (msg as Message).targetId !== this.peerId) {
      this.#opts.onDrop?.("message before handshake or for another peer");
      return;
    }
    this.emit("message", msg as Message);
  }

  #sendHandshake(to: PeerId, type: Handshake["type"]): void {
    if (!this.peerId) return;
    const hs: Handshake = {
      type,
      senderId: this.peerId,
      peerMetadata: this.peerMetadata ?? {},
      ...(this.#projectUrl ? { projectUrl: this.#projectUrl } : {}),
    };
    this.#sendTo(to, hs);
  }

  #announce(peer: PeerId, hs: Handshake): void {
    if (hs.projectUrl && isValidAutomergeUrl(hs.projectUrl)) {
      this.#projects.set(peer, hs.projectUrl);
      for (const cb of this.#projectListeners) cb(hs.projectUrl, peer);
    }
    if (this.#announced.has(peer)) return;
    this.#announced.add(peer);
    this.emit("peer-candidate", { peerId: peer, peerMetadata: hs.peerMetadata ?? {} });
  }

  #sendTo(to: PeerId, value: Handshake | Message): void {
    const socket = this.#socket;
    if (!socket) return;
    socket.send(encodeEnvelope({ kind: Kind.Message, peer: to, payload: encode(value) }));
  }
}
