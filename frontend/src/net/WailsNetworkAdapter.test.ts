// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { type DocHandle, type DocumentId, type PeerId, Repo } from "@automerge/automerge-repo";
import { encode } from "cbor-x";
import { afterEach, describe, expect, it } from "vitest";
import { decodeEnvelope, encodeEnvelope, encodeRoles, Kind } from "./envelope";
import { rng, SimNet } from "./simnet";
import { type SocketLike, WailsNetworkAdapter } from "./WailsNetworkAdapter";

type Doc = { text: string };

const repos: Repo[] = [];
afterEach(async () => {
  await Promise.all(repos.splice(0).map((r) => r.shutdown()));
});

function peer(net: SimNet, id: string) {
  const adapter = new WailsNetworkAdapter(() => net.socket(id), { reconnectDelayMs: 5 });
  const repo = new Repo({
    peerId: id as PeerId,
    network: [adapter],
    sharePolicy: async () => true,
  });
  repos.push(repo);
  const connected = new Set<string>();
  adapter.on("peer-candidate", ({ peerId }) => connected.add(peerId));
  adapter.on("peer-disconnected", ({ peerId }) => connected.delete(peerId));
  return { id, adapter, repo, connected };
}

async function until(cond: () => boolean, ms = 5000): Promise<void> {
  const deadline = Date.now() + ms;
  while (!cond()) {
    if (Date.now() > deadline) throw new Error("timed out");
    await new Promise((r) => setTimeout(r, 5));
  }
}

describe("WailsNetworkAdapter", () => {
  it("announces a peer only after the repo handshake", async () => {
    const net = new SimNet(1);
    const a = peer(net, "a");
    const b = peer(net, "b");
    const seen: string[] = [];
    a.adapter.on("peer-candidate", ({ peerId }) => seen.push(peerId));
    await a.adapter.whenReady();
    await b.adapter.whenReady();
    expect(seen).toEqual([]);
    net.link("a", "b");
    await until(() => seen.length === 1);
    expect(seen).toEqual(["b"]);
  });

  it("emits peer-disconnected on PeerDown and re-handshakes on reconnect", async () => {
    const net = new SimNet(1);
    const a = peer(net, "a");
    peer(net, "b");
    const events: string[] = [];
    a.adapter.on("peer-candidate", ({ peerId }) => events.push(`up:${peerId}`));
    a.adapter.on("peer-disconnected", ({ peerId }) => events.push(`down:${peerId}`));
    net.link("a", "b");
    await until(() => events.length === 1);
    net.partition("a", "b");
    net.link("a", "b");
    await until(() => events.length === 3);
    expect(events).toEqual(["up:b", "down:b", "up:b"]);
  });

  it("drops malformed frames and spoofed senders without closing", async () => {
    let socket: SocketLike | undefined;
    const fake: SocketLike = {
      binaryType: "",
      send: () => {},
      close: () => {},
      onopen: null,
      onclose: null,
      onmessage: null,
    };
    const drops: string[] = [];
    const adapter = new WailsNetworkAdapter(
      () => {
        socket = fake;
        return fake;
      },
      { onDrop: (r) => drops.push(r) },
    );
    adapter.connect("me" as PeerId);
    const deliver = (b: Uint8Array) =>
      socket?.onmessage?.(new MessageEvent("message", { data: b.buffer }));
    deliver(new Uint8Array([9, 9, 9]));
    deliver(
      encodeEnvelope({
        kind: Kind.Message,
        peer: "mallory",
        payload: encode({ type: "arrive", senderId: "alice", peerMetadata: {} }),
      }),
    );
    expect(drops).toEqual(["unsupported version 9", "sender mismatch"]);
  });

  it("PRS-63 knows the role of each member from what Go reports, and tells when it changes", () => {
    let socket: SocketLike | undefined;
    const fake: SocketLike = {
      binaryType: "",
      send: () => {},
      close: () => {},
      onopen: null,
      onclose: null,
      onmessage: null,
    };
    const drops: string[] = [];
    const adapter = new WailsNetworkAdapter(
      () => {
        socket = fake;
        return fake;
      },
      { onDrop: (r) => drops.push(r) },
    );
    let told = 0;
    adapter.onRoles(() => told++);
    adapter.connect("me" as PeerId);
    const deliver = (b: Uint8Array) =>
      socket?.onmessage?.(new MessageEvent("message", { data: b.buffer }));
    expect(adapter.roleOf("bob")).toBeUndefined();
    const roles = (members: Parameters<typeof encodeRoles>[0]) =>
      encodeEnvelope({ kind: Kind.Roles, peer: "me", payload: encodeRoles(members) });
    deliver(
      roles([
        { peer: "bob", role: "writer", suspended: false },
        { peer: "eve", role: "reader", suspended: false },
      ]),
    );
    expect(adapter.roleOf("bob")).toEqual({ peer: "bob", role: "writer", suspended: false });
    expect(adapter.roleOf("eve")?.role).toBe("reader");
    expect(adapter.roleOf("mallory")).toBeUndefined();
    expect(told).toBe(1);
    deliver(roles([{ peer: "bob", role: "reader", suspended: true }])); // a newer list replaces it
    expect(adapter.roleOf("bob")).toEqual({ peer: "bob", role: "reader", suspended: true });
    expect(adapter.roleOf("eve")).toBeUndefined();
    expect(drops).toEqual([]);
  });

  it("treats an arrive from an announced peer as that peer coming back", async () => {
    // The other side lost the link and began again while this one never saw it
    // go down (a stale connection): the repo must start a new sync with it.
    let socket: SocketLike | undefined;
    const fake: SocketLike = {
      binaryType: "",
      send: () => {},
      close: () => {},
      onopen: null,
      onclose: null,
      onmessage: null,
    };
    const adapter = new WailsNetworkAdapter(() => {
      socket = fake;
      return fake;
    });
    const events: string[] = [];
    adapter.on("peer-candidate", ({ peerId }) => events.push(`up:${peerId}`));
    adapter.on("peer-disconnected", ({ peerId }) => events.push(`down:${peerId}`));
    adapter.connect("me" as PeerId);
    const arrive = encodeEnvelope({
      kind: Kind.Message,
      peer: "alice",
      payload: encode({ type: "arrive", senderId: "alice", peerMetadata: {} }),
    });
    const welcome = encodeEnvelope({
      kind: Kind.Message,
      peer: "alice",
      payload: encode({ type: "welcome", senderId: "alice", peerMetadata: {} }),
    });
    const deliver = (b: Uint8Array) =>
      socket?.onmessage?.(new MessageEvent("message", { data: b.buffer }));
    deliver(arrive);
    deliver(welcome); // the answer to our own arrive: not a restart
    deliver(arrive);
    expect(events).toEqual(["up:alice", "down:alice", "up:alice"]);
  });

  it("signs the held heads again when a peer starts over on a link Go never saw go down", async () => {
    const sent: Uint8Array[] = [];
    let socket: SocketLike | undefined;
    const fake: SocketLike = {
      binaryType: "",
      send: (data) => sent.push(data as Uint8Array),
      close: () => {},
      onopen: null,
      onclose: null,
      onmessage: null,
    };
    const adapter = new WailsNetworkAdapter(() => {
      socket = fake;
      return fake;
    });
    adapter.connect("me" as PeerId);
    const arrive = encodeEnvelope({
      kind: Kind.Message,
      peer: "alice",
      payload: encode({ type: "arrive", senderId: "alice", peerMetadata: {} }),
    });
    const deliver = (b: Uint8Array) =>
      socket?.onmessage?.(new MessageEvent("message", { data: b.buffer }));
    const attests = () => sent.filter((f) => decodeEnvelope(f).kind === Kind.Attest).length;

    deliver(arrive);
    const doc = A.change(A.init<Doc>(), (d) => {
      d.text = "x";
    });
    const [, data] = A.generateSyncMessage(doc, A.initSyncState());
    adapter.send({
      type: "sync",
      senderId: "me" as PeerId,
      targetId: "alice" as PeerId,
      documentId: "doc" as DocumentId,
      data: data as Uint8Array,
    });
    expect(attests()).toBe(1);

    deliver(arrive); // alice started over: the attestation Go gossiped went nowhere
    expect(attests()).toBe(2);
  });

  it("hands the host's project URL to a joining peer", async () => {
    const net = new SimNet(1);
    const host = peer(net, "host");
    const guest = peer(net, "guest");
    const handle = host.repo.create<Doc>({ text: "" });
    host.adapter.setProject(handle.url);
    const learned: string[] = [];
    guest.adapter.onProject((url) => learned.push(url));
    net.link("host", "guest");
    await until(() => learned.length > 0);
    expect(learned[0]).toBe(handle.url);
  });
});

describe("convergence under link flaps (seeded)", () => {
  const SEEDS = 20;
  for (let seed = 1; seed <= SEEDS; seed++) {
    it(`3 peers converge — seed ${seed}`, async () => {
      const net = new SimNet(seed);
      const random = rng(seed * 7919);
      const ids = ["a", "b", "c"];
      const peers = ids.map((id) => peer(net, id));
      net.link("a", "b");
      net.link("b", "c");
      net.link("a", "c");
      const [first] = peers;
      if (!first) throw new Error("no peers");
      await until(() => peers.every((p) => p.connected.size === ids.length - 1));
      const root = first.repo.create<Doc>({ text: "" });
      const handles: DocHandle<Doc>[] = await Promise.all(
        peers.map((p) => (p === first ? root : p.repo.find<Doc>(root.url))),
      );

      for (let step = 0; step < 30; step++) {
        const i = Math.floor(random() * handles.length);
        const h = handles[i];
        if (!h) continue;
        h.change((d) => {
          const pos = Math.floor(random() * (d.text.length + 1));
          A.splice(d, ["text"], pos, 0, String.fromCharCode(97 + Math.floor(random() * 26)));
        });
        if (random() < 0.2) {
          const [x, y] = [ids[Math.floor(random() * 3)], ids[Math.floor(random() * 3)]];
          if (x && y) {
            net.partition(x, y);
            if (random() < 0.7) net.link(x, y);
          }
        }
        if (random() < 0.3) await new Promise((r) => setTimeout(r, 0));
      }
      net.link("a", "b");
      net.link("b", "c");
      net.link("a", "c");
      await until(() => {
        const texts = handles.map((h) => h.doc()?.text);
        const heads = handles.map((h) => JSON.stringify(h.heads()));
        return new Set(texts).size === 1 && new Set(heads).size === 1;
      }, 10_000);
      expect(handles[0]?.doc()?.text.length).toBe(30);
    });
  }
});
