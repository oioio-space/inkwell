import { next as A } from "@automerge/automerge";
import {
  type AutomergeUrl,
  type Message,
  NetworkAdapter,
  type PeerId,
  type PeerMetadata,
} from "@automerge/automerge-repo";
import { encode } from "cbor-x";
import { Admission, type Handshake, parsePeerMessage } from "./admission";
import { Coverage } from "./coverage";
import {
  decodeEnvelope,
  decodeHeads,
  decodeRoles,
  encodeEnvelope,
  encodeHeads,
  Kind,
  MalformedEnvelope,
  type MemberRole,
} from "./envelope";

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

export interface AdapterOptions {
  admitted?: (id: string) => boolean;
  ready?: (id: string) => boolean;
  load?: (id: string) => void;
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
 *
 * It also enforces who may write (ADR 0018). Before a sync message whose heads
 * changed it asks Go to sign them (`Attest`, on the same stream, so the order is
 * kept all the way to the other peers); and it lets a received sync message through
 * only for the changes that the heads attested by current writers cover (see
 * {@link Coverage}). What a reader forges therefore never reaches the Repo.
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
  #dropCounts = new Map<string, number>();
  get admissionDrops(): ReadonlyMap<string, number> {
    return this.#dropCounts;
  }
  #drop(reason: string): void {
    this.#dropCounts.set(reason, (this.#dropCounts.get(reason) ?? 0) + 1);
    this.#opts.onDrop?.(reason);
  }
  #coverage = new Coverage();
  #admission = new Admission(Date.now, Math.random().toString(36).slice(2, 15));
  #pendingTimers = new Map<PeerId, ReturnType<typeof setTimeout>>();

  scopeChanged(): void {
    this.#admission.clearQueues();
  }
  drain(): void {
    for (const peer of this.#announced) {
      for (const msg of this.#admission.drain(peer, this.#opts.ready ?? (() => true)))
        this.#deliver(msg);
    }
  }
  /** Heads of the last sync message sent for each document. */
  #lastHeads = new Map<string, string[]>();
  /** Heads of the last attestation asked for each document. */
  #attested = new Map<string, string[]>();
  #sealed = false;
  #sealedListeners: (() => void)[] = [];
  #roles = new Map<string, MemberRole>();
  #roleListeners: (() => void)[] = [];

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

  /** What write enforcement did so far: changes removed, messages dropped. */
  get coverageStats(): Readonly<Coverage["stats"]> {
    return this.#coverage.stats;
  }

  /**
   * True once Go reported that the member list is full ("liste de membres saturée —
   * créez un nouveau projet"): this device signs no more, so edits stop reaching
   * the others.
   */
  get sealed(): boolean {
    return this.#sealed;
  }

  /** Calls cb when the member list is reported full (at once if it already was). */
  onSealed(cb: () => void): void {
    this.#sealedListeners.push(cb);
    if (this.#sealed) cb();
  }

  /**
   * The role of a device in the member list that Go signed, undefined for a device it does not
   * list (or before Go has said anything). Anything that depends on rights asks here.
   */
  roleOf(peer: string): MemberRole | undefined {
    return this.#roles.get(peer);
  }

  /** Calls cb whenever Go reports the roles of the members (when they change). */
  onRoles(cb: () => void): void {
    this.#roleListeners.push(cb);
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
    if (message.type === "ephemeral" && message.senderId !== this.peerId) return;
    this.#attest(message);
    this.#sendTo(message.targetId, message);
  }

  disconnect(): void {
    this.#closed = true;
    this.#socket?.close();
    this.#socket = undefined;
    this.#disconnectAll();
    this.emit("close");
  }

  /** Every announced peer is gone with the link: the repo forgets its sync state with them. */
  #disconnectAll(): void {
    for (const peerId of this.#announced) this.emit("peer-disconnected", { peerId });
    for (const peer of this.#announced) this.#admission.forget(peer);
    for (const timer of this.#pendingTimers.values()) clearTimeout(timer);
    this.#pendingTimers.clear();
    this.#announced.clear();
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
      this.#disconnectAll();
      if (!this.#closed) setTimeout(() => this.#dial(), this.#opts.reconnectDelayMs ?? 500);
    };
    this.#socket = socket;
  }

  #receive(frame: Uint8Array): void {
    let env: ReturnType<typeof decodeEnvelope>;
    try {
      env = decodeEnvelope(frame);
    } catch (err) {
      this.#drop(err instanceof MalformedEnvelope ? err.message : "undecodable frame");
      return;
    }
    const peer = env.peer as PeerId;
    switch (env.kind) {
      case Kind.PeerUp:
        if (this.#admission.blocked(peer)) this.#scheduleReset(peer);
        else this.#sendHandshake(peer, "arrive");
        return;
      case Kind.PeerDown:
        this.#admission.forget(peer);
        clearTimeout(this.#pendingTimers.get(peer));
        this.#pendingTimers.delete(peer);
        this.#coverage.forgetPeer(peer);
        this.#projects.delete(peer);
        if (this.#announced.delete(peer)) this.emit("peer-disconnected", { peerId: peer });
        return;
      case Kind.Message:
        this.#receiveMessage(peer, env.payload);
        return;
      case Kind.Attested:
        this.#receiveAttested(peer, env.payload);
        return;
      case Kind.Revoked:
        this.#coverage.revoked(peer);
        this.#reattest();
        return;
      case Kind.Roles:
        this.#receiveRoles(env.payload);
        return;
      case Kind.Sealed:
        this.#sealed = true;
        for (const cb of this.#sealedListeners) cb();
        return;
    }
  }

  #receiveRoles(payload: Uint8Array): void {
    try {
      this.#roles = new Map(decodeRoles(payload).map((m) => [m.peer, m]));
    } catch {
      this.#drop("undecodable roles");
      return;
    }
    for (const cb of this.#roleListeners) cb();
  }

  #receiveAttested(signer: string, payload: Uint8Array): void {
    try {
      const { doc, heads } = decodeHeads(payload);
      this.#coverage.attested(signer, doc, heads);
    } catch {
      this.#drop("undecodable attestation");
    }
  }

  /**
   * Asks Go to sign the heads of the sync message about to go out, unless they are
   * the ones last attested. Go gossips the attestation before relaying the message.
   */
  #attest(message: Message): void {
    const { type, documentId, data } = message;
    if ((type !== "sync" && type !== "request") || !documentId || !data) return;
    let heads: string[];
    try {
      heads = [...A.decodeSyncMessage(data).heads].sort();
    } catch {
      return; // the other side will not make sense of it either
    }
    if (heads.length === 0) return;
    this.#lastHeads.set(documentId, heads);
    if (sameHeads(this.#attested.get(documentId), heads)) return;
    this.#sendAttest(documentId, heads);
  }

  /** After a revocation every writer vouches again for what it holds. */
  #reattest(): void {
    for (const [doc, heads] of this.#lastHeads) this.#sendAttest(doc, heads);
  }

  #sendAttest(doc: string, heads: string[]): void {
    const socket = this.#socket;
    if (!socket || !this.peerId) return;
    let frame: Uint8Array;
    try {
      frame = encodeEnvelope({
        kind: Kind.Attest,
        peer: this.peerId,
        payload: encodeHeads({ doc, heads }),
      });
    } catch {
      this.#drop("heads cannot be attested"); // e.g. more than 64 heads
      return;
    }
    socket.send(frame);
    this.#attested.set(doc, heads);
  }

  #receiveMessage(from: PeerId, payload: Uint8Array): void {
    const msg = parsePeerMessage(payload, {
      from,
      me: this.peerId ?? "",
      announced: this.#announced.has(from),
      has: this.#opts.admitted ?? (() => false),
    });
    if (!msg) {
      this.#drop("invalid peer message");
      return;
    }
    if (this.#admission.blocked(from)) {
      this.#drop("reset cooldown");
      return;
    }
    if (msg.type === "arrive" || msg.type === "welcome") {
      const hs = msg as Handshake;
      if (hs.type === "arrive" && !this.#admission.arrive(from, hs)) {
        this.#drop("re-arrive rate");
        this.#schedulePending(from);
        return;
      }
      this.#acceptHandshake(from, hs);
      return;
    }
    let message = msg as Message;
    if (message.type === "ephemeral") {
      const rewritten = this.#admission.ephemeral(from, message);
      if (!rewritten) {
        this.#drop("ephemeral rate or count");
        return;
      }
      message = rewritten;
    }
    if (message.documentId && !(this.#opts.ready?.(message.documentId) ?? true)) {
      if (message.type === "ephemeral") return;
      if (!this.#admission.hold(from, message)) {
        this.#drop("hold queue overflow");
        clearTimeout(this.#pendingTimers.get(from));
        this.#pendingTimers.delete(from);
        this.#scheduleReset(from);
        if (this.#announced.delete(from)) this.emit("peer-disconnected", { peerId: from });
        return;
      }
      this.#opts.load?.(message.documentId);
      return;
    }
    this.#deliver(message);
  }

  #scheduleReset(from: PeerId): void {
    if (this.#pendingTimers.has(from)) return;
    this.#pendingTimers.set(
      from,
      setTimeout(() => {
        this.#pendingTimers.delete(from);
        this.#sendHandshake(from, "arrive");
      }, this.#admission.retryAfter(from)),
    );
  }

  #schedulePending(from: PeerId): void {
    if (this.#pendingTimers.has(from)) return;
    this.#pendingTimers.set(
      from,
      setTimeout(() => {
        this.#pendingTimers.delete(from);
        const hs = this.#admission.pending(from) as Handshake | undefined;
        if (hs) this.#acceptHandshake(from, hs);
      }, 20000),
    );
  }

  #acceptHandshake(from: PeerId, hs: Handshake): void {
    if (hs.type === "welcome" && this.#announced.has(from)) return;
    this.#admission.reset(from);
    if (hs.type === "arrive") {
      if (this.#announced.delete(from)) {
        this.emit("peer-disconnected", { peerId: from });
        this.#coverage.forgetPeer(from);
        this.#reattest();
      }
      this.#sendHandshake(from, "welcome");
    }
    this.#announce(from, hs);
  }

  #deliver(msg: Message): void {
    const checked = this.#enforce(msg as Message);
    if (checked) this.emit("message", checked);
  }

  /**
   * Keeps from a sync message only the changes that current writers' attestations
   * cover. Other messages (cursors, presence, ...) are not changes: readers may
   * send them.
   */
  #enforce(msg: Message): Message | undefined {
    if (msg.type !== "sync" && msg.type !== "request") return msg;
    const data =
      msg.documentId && msg.data && this.#coverage.filter(msg.documentId, msg.senderId, msg.data);
    if (!data) {
      this.#drop("sync message dropped");
      return undefined;
    }
    return data === msg.data ? msg : { ...msg, data };
  }

  #sendHandshake(to: PeerId, type: Handshake["type"]): void {
    if (!this.peerId) return;
    const hs: Handshake = {
      type,
      senderId: this.peerId,
      peerMetadata: { isEphemeral: true },
      ...(this.#projectUrl ? { projectUrl: this.#projectUrl } : {}),
    };
    this.#sendTo(to, hs);
  }

  #announce(peer: PeerId, hs: Handshake): void {
    if (hs.projectUrl) {
      this.#projects.set(peer, hs.projectUrl);
      for (const cb of this.#projectListeners) cb(hs.projectUrl, peer);
    }
    if (this.#announced.has(peer)) return;
    this.#announced.add(peer);
    this.emit("peer-candidate", { peerId: peer, peerMetadata: { isEphemeral: true } });
  }

  #sendTo(to: PeerId, value: Handshake | Message): void {
    const socket = this.#socket;
    if (!socket) return;
    socket.send(encodeEnvelope({ kind: Kind.Message, peer: to, payload: encode(value) }));
  }
}

function sameHeads(a: string[] | undefined, b: string[]): boolean {
  return a !== undefined && a.length === b.length && a.every((h, i) => h === b[i]);
}
