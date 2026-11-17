import { type Message, type PeerId, Repo } from "@automerge/automerge-repo";
import { type CBORValue, encodeCBOR } from "@inkwell/protocol";
import type { Handshake } from "./admission";
import type { SimNet } from "./simnet";
import { WailsNetworkAdapter } from "./WailsNetworkAdapter";

/** A simulated device: its repo, its adapter, and the peers its adapter announced. */
export interface TestPeer {
  id: string;
  adapter: WailsNetworkAdapter;
  repo: Repo;
  connected: Set<string>;
}

/** Builds peers on a {@link SimNet} and shuts their repos down at the end of a test. */
export class Kit {
  #repos: Repo[] = [];

  peer(net: SimNet, id: string): TestPeer {
    const adapter = new WailsNetworkAdapter(() => net.socket(id), {
      reconnectDelayMs: 5,
      admitted: () => true,
    });
    const repo = new Repo({
      peerId: id as PeerId,
      network: [adapter],
      sharePolicy: async () => true,
    });
    this.#repos.push(repo);
    const connected = new Set<string>();
    adapter.on("peer-candidate", ({ peerId }) => connected.add(peerId));
    adapter.on("peer-disconnected", ({ peerId }) => connected.delete(peerId));
    return { id, adapter, repo, connected };
  }

  async shutdown(): Promise<void> {
    await Promise.all(this.#repos.splice(0).map((r) => r.shutdown()));
  }
}

/** Creates a handshake message (arrive/welcome) with required fields filled to the spec. */
export function peerMessage(
  type: Handshake["type"],
  from: string,
  to: string,
  over?: Omit<Partial<Handshake>, "peerMetadata"> & { peerMetadata?: Record<string, unknown> },
): Handshake {
  const overPeerMetadata = over?.peerMetadata ?? {};
  const msg = {
    type,
    senderId: from as PeerId,
    targetId: to as PeerId,
    peerMetadata: { isEphemeral: true, ...overPeerMetadata },
    ...over,
  };
  return msg as Handshake;
}

/** Creates a generic repo message with required fields filled to the spec. */
export function repoMessage(
  type: Message["type"],
  from: string,
  to: string,
  documentId: string,
  over?: Partial<Message>,
): Message {
  return {
    type,
    senderId: from as PeerId,
    targetId: to as PeerId,
    documentId: documentId as import("@automerge/automerge-repo").DocumentId,
    data: new Uint8Array(),
    ...over,
  };
}

/**
 * The CBOR of `count` messages that differ only by an invented `documentId` (`invented00000`,
 * `invented00001`, …, all of one length), as {@link encodeCBOR} gives them. The first is encoded,
 * then each other is a copy with the digits rewritten: the profile encoder costs about 21 µs a
 * message, and 50 000 of them were 90 % of a test that measures the boundary (FE1). The caller
 * checks a few against `encodeCBOR` itself (the shortcut must stay exact).
 */
export function inventedIds(count: number, message: Record<string, unknown>) {
  const width = String(count - 1).length;
  const id = (i: number) => `invented${String(i).padStart(width, "0")}`;
  const first = encodeCBOR({ ...message, documentId: id(0) } as CBORValue);
  const want = new TextEncoder().encode(id(0));
  const start = first.findIndex((_, i) => want.every((b, k) => first[i + k] === b));
  if (start < 0) throw new Error("inventedIds: the id is not in its encoding");
  const at = start + "invented".length;
  return {
    id,
    bytes(i: number): Uint8Array {
      const out = first.slice();
      for (const [k, c] of [...String(i).padStart(width, "0")].entries()) {
        out[at + k] = c.charCodeAt(0);
      }
      return out;
    },
  };
}

/** Polls `cond` every few milliseconds; throws `what` if it is still false after `ms`. */
export async function until(cond: () => boolean, ms = 5000, what = "timed out"): Promise<void> {
  const deadline = Date.now() + ms;
  while (!cond()) {
    if (Date.now() > deadline) throw new Error(what);
    await new Promise((r) => setTimeout(r, 5));
  }
}
