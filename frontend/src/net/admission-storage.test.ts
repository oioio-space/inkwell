// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { type PeerId, Repo } from "@automerge/automerge-repo";
import { encode } from "cbor-x";
import { expect, it, vi } from "vitest";
import { encodeEnvelope, Kind } from "./envelope";
import { type SocketLike, WailsNetworkAdapter } from "./WailsNetworkAdapter";

it("6 forty storageIds never load, save or setSyncInfo, even with delayed saves", async () => {
  const saved: string[][] = [];
  const loaded: string[][] = [];
  const storage = {
    load: async (key: string[]) => {
      loaded.push(key);
      return undefined;
    },
    save: async (key: string[]) => {
      await new Promise((r) => setTimeout(r, 1000));
      saved.push(key);
    },
    remove: async () => {},
    loadRange: async () => [],
    removeRange: async () => {},
  };
  const socket: SocketLike = {
    binaryType: "",
    send: () => {},
    close: () => {},
    onopen: null,
    onclose: null,
    onmessage: null,
  };
  const adapter = new WailsNetworkAdapter(() => socket, { admitted: () => true });
  const repo = new Repo({ peerId: "me" as PeerId, network: [adapter], storage, isEphemeral: true });
  const doc = repo.create({ text: "hello" });
  const syncInfo = vi.spyOn(doc, "setSyncInfo");
  await new Promise((r) => setTimeout(r, 0));
  const deliver = (value: unknown) => {
    const frame = encodeEnvelope({ kind: Kind.Message, peer: "peer", payload: encode(value) });
    socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  };
  const [, data] = A.generateSyncMessage(A.init(), A.initSyncState());
  for (let i = 0; i < 40; i++) {
    deliver({ type: "arrive", senderId: "peer", peerMetadata: { storageId: `store${i}` } });
    deliver({ type: "sync", senderId: "peer", targetId: "me", documentId: doc.documentId, data });
  }
  await new Promise((r) => setTimeout(r, 1200));
  expect(saved.some((key) => key.includes("sync-state"))).toBe(false);
  expect(loaded.some((key) => key.includes("sync-state"))).toBe(false);
  expect(syncInfo).not.toHaveBeenCalled();
  await repo.shutdown();
});
