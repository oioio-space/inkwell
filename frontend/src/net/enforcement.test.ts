// @vitest-environment node
import { next as A } from "@automerge/automerge";
import type { DocHandle, DocumentId, Message, PeerId } from "@automerge/automerge-repo";
import { encode } from "cbor-x";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  decodeEnvelope,
  decodeHeads,
  encodeEnvelope,
  encodeHeads,
  encodeRoles,
  Kind,
} from "./envelope";
import { rng, SimNet } from "./simnet";
import { Kit, type TestPeer, until } from "./testkit";
import { type SocketLike, WailsNetworkAdapter } from "./WailsNetworkAdapter";

type Doc = { text: string };

const kit = new Kit();
afterEach(() => kit.shutdown());

vi.setConfig({ testTimeout: 90_000 });
const SEEDS = Array.from({ length: 10 }, (_, i) => i + 1);

function type(h: DocHandle<Doc>, s: string): void {
  h.change((d) => A.splice(d, ["text"], d.text.length, 0, s));
}

const text = (h: DocHandle<Doc>) => h.doc()?.text ?? "";
const headsOf = (h: DocHandle<Doc>) => JSON.stringify(h.heads());

function converged(...hs: DocHandle<Doc>[]): boolean {
  return new Set(hs.map(text)).size === 1 && new Set(hs.map(headsOf)).size === 1;
}

function mesh(net: SimNet, ids: string[]): void {
  for (const [i, x] of ids.entries()) for (const y of ids.slice(i + 1)) net.link(x, y);
}

async function ready(peers: TestPeer[]): Promise<void> {
  await Promise.all(peers.map((p) => p.adapter.whenReady()));
}

describe("un lecteur forge une modification", () => {
  for (const seed of SEEDS) {
    it(`est retirée, et les honnêtes convergent — seed ${seed}`, async () => {
      const net = new SimNet(seed);
      const random = rng(seed * 7919);
      const [a, b, r] = ["a", "b", "r"].map((id) => kit.peer(net, id)) as [
        TestPeer,
        TestPeer,
        TestPeer,
      ];
      net.setRole("r", "reader");
      const ids = ["a", "b", "r"];
      mesh(net, ids);
      await until(() => [a, b, r].every((p) => p.connected.size === 2));

      const ha = a.repo.create<Doc>({ text: "" });
      const hb = await b.repo.find<Doc>(ha.url);
      const hr = await r.repo.find<Doc>(ha.url);

      const cursors: unknown[] = [];
      hb.on("ephemeral-message", ({ message }) => cursors.push(message));

      for (let step = 0; step < 24; step++) {
        const who = random();
        if (who < 0.35)
          type(hr, "X"); // the reader forges
        else type(who < 0.7 ? ha : hb, String.fromCharCode(97 + Math.floor(random() * 26)));
        if (random() < 0.15) {
          const [x, y] = [ids[Math.floor(random() * 3)], ids[Math.floor(random() * 3)]];
          if (x && y) {
            net.partition(x, y);
            if (random() < 0.7) net.link(x, y);
          }
        }
        if (random() < 0.3) await new Promise((res) => setTimeout(res, 0));
      }
      mesh(net, ids);
      type(hr, "X"); // one more, on a healed network

      await until(() => converged(ha, hb), 10_000, "honest peers did not converge");
      await net.settle(8000); // a forger does not keep the network busy

      expect(text(ha)).not.toContain("X");
      expect(text(hb)).toBe(text(ha));
      expect(net.refused.length).toBeGreaterThan(0); // Go signs nothing for a reader
      expect(new Set(net.refused)).toEqual(new Set(["r"]));
      expect(a.adapter.coverageStats.removed + b.adapter.coverageStats.removed).toBeGreaterThan(0);
      // The reader does get the honest work, and its own edits stay its own.
      // Reconnect bursts now wait for the persistent 20-second arrival budget.
      await until(
        () => text(hr).replaceAll("X", "") === text(ha),
        60_000,
        "reader did not catch up",
      );
      // Ephemeral messages of a reader are not changes: they pass.
      await until(
        () => {
          hr.broadcast({ __presence: { type: "update", channel: "anchor", value: String(seed) } }); // sent again until the peers are back
          return cursors.length > 0;
        },
        60_000,
        "ephemeral message from the reader lost",
      );
    });
  }
});

describe("fragment de document complet", () => {
  for (const seed of SEEDS) {
    it(`couvert : il est accepté — seed ${seed}`, async () => {
      const net = new SimNet(seed);
      const random = rng(seed * 104729);
      const a = kit.peer(net, "a");
      const b = kit.peer(net, "b");
      await ready([a, b]);
      const ha = a.repo.create<Doc>({ text: "" });
      const edits = 5 + Math.floor(random() * 10);
      for (let i = 0; i < edits; i++) type(ha, String.fromCharCode(97 + Math.floor(random() * 26)));

      net.link("a", "b"); // b holds nothing: a answers with the whole document
      await until(() => b.connected.has("a"));
      const hb = await b.repo.find<Doc>(ha.url);
      await until(() => converged(ha, hb));
      expect(text(hb).length).toBe(edits);
      expect(b.adapter.coverageStats.fragments).toBeGreaterThan(0);
      expect(b.adapter.coverageStats.dropped).toBe(0);
    });

    it(`avec une modification non couverte : le message entier est jeté — seed ${seed}`, async () => {
      const net = new SimNet(seed);
      const random = rng(seed * 15485863);
      const [a, r, c] = ["a", "r", "c"].map((id) => kit.peer(net, id)) as [
        TestPeer,
        TestPeer,
        TestPeer,
      ];
      net.setRole("r", "reader");
      await ready([a, r, c]);
      net.link("a", "r");
      await until(() => a.connected.has("r") && r.connected.has("a"));
      const ha = a.repo.create<Doc>({ text: "" });
      const hr = await r.repo.find<Doc>(ha.url);
      for (let i = 0; i < 3 + Math.floor(random() * 5); i++) type(ha, "h");
      await until(() => text(hr) === text(ha));
      type(hr, "X"); // the reader's copy now holds a forged change
      net.partition("a", "r");

      // c meets the reader first: it sends the whole document, forged change included.
      net.link("c", "r");
      await until(() => c.connected.has("r") && r.connected.has("c"));
      const found = c.repo.find<Doc>(ha.url);
      await until(() => c.adapter.coverageStats.dropped > 0, 5000, "the fragment was not dropped");
      // The honest path then gives c the document, without the forgery.
      net.link("c", "a");
      const hc = await found;
      await until(() => text(hc) === text(ha));
      expect(text(hc)).not.toContain("X");
    });
  }
});

describe("écrivain révoqué hors ligne", () => {
  /** a (admin), b and x write; x then goes offline and types "Z"s. */
  async function offlineWriter(seed: number) {
    const net = new SimNet(seed);
    const random = rng(seed * 6700417);
    const [a, b, x] = ["a", "b", "x"].map((id) => kit.peer(net, id)) as [
      TestPeer,
      TestPeer,
      TestPeer,
    ];
    mesh(net, ["a", "b", "x"]);
    await until(() => [a, b, x].every((p) => p.connected.size === 2));
    const ha = a.repo.create<Doc>({ text: "" });
    const hb = await b.repo.find<Doc>(ha.url);
    const hx = await x.repo.find<Doc>(ha.url);
    type(ha, "a");
    type(hx, "x"); // accepted: x was a writer
    type(hb, "b");
    await until(() => converged(ha, hb, hx));

    net.partition("x", "a");
    net.partition("x", "b");
    const offline = 1 + Math.floor(random() * 4);
    for (let i = 0; i < offline; i++) type(hx, "Z");
    return { net, random, a, b, x, ha, hb, hx };
  }

  for (const seed of SEEDS) {
    it(`rétrogradé : ses modifications hors ligne sont refusées — seed ${seed}`, async () => {
      const { net, random, a, b, ha, hb, hx } = await offlineWriter(seed);
      net.demote("x", "a"); // the list says reader; x does not know yet
      for (let i = 0; i < 3; i++) type(random() < 0.5 ? ha : hb, "h");

      net.link("x", "a"); // x comes back, still a member
      net.link("x", "b");
      await until(() => converged(ha, hb), 10_000, "honest peers did not converge");
      await net.settle(8000);

      expect(text(ha)).not.toContain("Z");
      expect(text(ha)).toContain("x"); // what was accepted before stays
      expect(text(hb)).toBe(text(ha));
      expect(a.adapter.coverageStats.removed + b.adapter.coverageStats.removed).toBeGreaterThan(0);
      // x learned it is a reader: its later attestations are refused.
      type(hx, "Z");
      await net.settle(8000);
      expect(net.refused).toContain("x");
      expect(text(ha)).not.toContain("Z");
    });

    it(`retiré : il n'atteint plus personne — seed ${seed}`, async () => {
      const { net, a, b, x, ha, hb, hx } = await offlineWriter(seed);
      net.remove("x", "a");
      type(ha, "h");
      net.link("x", "a");
      net.link("x", "b");
      await net.settle();
      expect(x.connected.size).toBe(0);
      await until(() => converged(ha, hb));
      expect(text(ha)).not.toContain("Z");
      expect(text(hx)).toContain("Z");
      expect(a.connected.has("x") || b.connected.has("x")).toBe(false);
    });
  }
});

describe("nouveau pair après révocation", () => {
  for (const seed of SEEDS) {
    it(`se synchronise — seed ${seed}`, async () => {
      const net = new SimNet(seed);
      const random = rng(seed * 2147483629);
      const [a, b, x, n] = ["a", "b", "x", "n"].map((id) => kit.peer(net, id)) as [
        TestPeer,
        TestPeer,
        TestPeer,
        TestPeer,
      ];
      mesh(net, ["a", "b", "x"]);
      await until(() => [a, b, x].every((p) => p.connected.size === 2));
      const ha = a.repo.create<Doc>({ text: "" });
      const hb = await b.repo.find<Doc>(ha.url);
      const hx = await x.repo.find<Doc>(ha.url);
      const handles = [ha, hb, hx];
      for (let i = 0; i < 12; i++) {
        const h = handles[Math.floor(random() * 3)] as DocHandle<Doc>;
        type(h, h === hx ? "x" : "a");
        if (random() < 0.4) await new Promise((res) => setTimeout(res, 0));
      }
      await until(() => converged(ha, hb, hx));
      const history = text(ha);
      expect(history).toContain("x");

      net.remove("x", "a");
      type(ha, "b");
      type(hb, "b");
      await until(() => converged(ha, hb));

      // n joins after the revocation and asks the writers who are left. The history
      // x wrote before is covered by what they attest, not by x.
      mesh(net, ["n", "a", "b"]);
      await until(() => n.connected.size === 2);
      const hn = await n.repo.find<Doc>(ha.url);
      await until(() => converged(ha, hb, hn), 10_000, "the newcomer did not converge");
      expect(text(hn)).toContain(history);
      expect(n.adapter.coverageStats.removed).toBe(0);
    });
  }
});

/** A adapter on a socket the test drives by hand. */
function wire() {
  const sent: Uint8Array[] = [];
  let socket: SocketLike | undefined;
  const drops: string[] = [];
  const messages: Message[] = [];
  const fake: SocketLike = {
    binaryType: "",
    send: (d) => sent.push(d),
    close: () => {},
    onopen: null,
    onclose: null,
    onmessage: null,
  };
  const adapter = new WailsNetworkAdapter(
    () => {
      socket = fake;
      return fake;
    },
    { onDrop: (r) => drops.push(r), admitted: () => true },
  );
  adapter.on("message", (m) => messages.push(m));
  adapter.connect("me" as PeerId);
  const deliver = (b: Uint8Array) =>
    socket?.onmessage?.(new MessageEvent("message", { data: b.buffer }));
  const rolesFrame = encodeEnvelope({
    kind: Kind.Roles,
    peer: "me",
    payload: encodeRoles([{ peer: "peer", role: "writer", suspended: false }]),
  });
  socket?.onmessage?.(new MessageEvent("message", { data: rolesFrame.buffer }));
  const kinds = () => sent.map((f) => decodeEnvelope(f).kind);
  const handshake = (peer: string) => {
    deliver(encodeEnvelope({ kind: Kind.PeerUp, peer, payload: new Uint8Array() }));
    deliver(
      encodeEnvelope({
        kind: Kind.Message,
        peer,
        payload: encode({ type: "arrive", senderId: peer, peerMetadata: {} }),
      }),
    );
  };
  return { adapter, sent, deliver, kinds, drops, messages, handshake };
}

function syncMessage(doc: A.Doc<Doc>): Uint8Array {
  const [, m] = A.generateSyncMessage(doc, A.initSyncState());
  if (!m) throw new Error("no sync message");
  return m;
}

function outgoing(doc: A.Doc<Doc>, data = syncMessage(doc)): Message {
  return {
    type: "sync",
    senderId: "me" as PeerId,
    targetId: "peer" as PeerId,
    documentId: "doc1" as DocumentId,
    data,
  };
}

describe("l'adaptateur demande l'attestation", () => {
  it("avant un message de synchro dont les têtes ont changé, sur le même flux", () => {
    const w = wire();
    w.handshake("peer");
    w.sent.length = 0;
    let doc = A.from<Doc>({ text: "" });

    w.adapter.send(outgoing(doc));
    expect(w.kinds()).toEqual([Kind.Attest, Kind.Message]);
    const attest = decodeEnvelope(w.sent[0] as Uint8Array);
    expect(attest.peer).toBe("me");
    expect(decodeHeads(attest.payload)).toEqual({ doc: "doc1", heads: A.getHeads(doc) });

    w.adapter.send(outgoing(doc)); // same heads: nothing new to vouch for
    expect(w.kinds()).toEqual([Kind.Attest, Kind.Message, Kind.Message]);

    doc = A.change(doc, (d) => A.splice(d, ["text"], 0, 0, "a"));
    w.adapter.send(outgoing(doc));
    expect(w.kinds().slice(3)).toEqual([Kind.Attest, Kind.Message]);
  });

  it("pas pour les messages éphémères ni pour un document vide", () => {
    const w = wire();
    w.handshake("peer");
    w.sent.length = 0;
    w.adapter.send({
      type: "ephemeral",
      senderId: "me" as PeerId,
      targetId: "peer" as PeerId,
      documentId: "doc1" as DocumentId,
      data: new Uint8Array([1]),
    });
    w.adapter.send(outgoing(A.init<Doc>(), new Uint8Array([0x42, 0, 0, 0, 0])));
    expect(w.kinds()).toEqual([Kind.Message, Kind.Message]);
  });

  it("de nouveau après une révocation, avec les dernières têtes", () => {
    const w = wire();
    w.handshake("peer");
    const doc = A.from<Doc>({ text: "a" });
    w.adapter.send(outgoing(doc));
    w.sent.length = 0;
    w.deliver(encodeEnvelope({ kind: Kind.Revoked, peer: "x", payload: new Uint8Array() }));
    expect(w.kinds()).toEqual([Kind.Attest]);
    expect(decodeHeads(decodeEnvelope(w.sent[0] as Uint8Array).payload).heads).toEqual(
      A.getHeads(doc),
    );
  });

  it("n'attend pas de réponse : un Attest refusé par Go ne bloque pas le message", () => {
    const w = wire();
    w.handshake("peer");
    expect(() => w.adapter.send(outgoing(A.from<Doc>({ text: "" })))).not.toThrow();
  });
});

describe("l'adaptateur filtre ce qu'il reçoit", () => {
  function setup() {
    const w = wire();
    w.handshake("peer");
    const honest = A.from<Doc>({ text: "ok" });
    const forged = A.change(A.clone(honest), (d) => A.splice(d, ["text"], 0, 0, "X"));
    const incoming = (changes: Uint8Array[], heads: string[]) => {
      const data = A.encodeSyncMessage({ heads, need: [], have: [], changes });
      return encodeEnvelope({
        kind: Kind.Message,
        peer: "peer",
        payload: encode({
          type: "sync",
          senderId: "peer",
          targetId: "me",
          documentId: "doc1",
          data,
        }),
      });
    };
    const attested = (heads: string[]) =>
      encodeEnvelope({
        kind: Kind.Attested,
        peer: "peer",
        payload: encodeHeads({ doc: "doc1", heads }),
      });
    return { w, honest, forged, incoming, attested };
  }

  it("retire la modification non couverte avant le Repo", () => {
    const { w, honest, forged, incoming, attested } = setup();
    w.deliver(attested(A.getHeads(honest)));
    w.deliver(
      incoming([...A.getAllChanges(honest), ...A.getChanges(honest, forged)], A.getHeads(forged)),
    );
    expect(w.messages).toHaveLength(1);
    const data = (w.messages[0] as Message & { data: Uint8Array }).data;
    expect(A.decodeSyncMessage(data).changes).toHaveLength(A.getAllChanges(honest).length);
    expect(w.adapter.coverageStats.removed).toBe(1);
  });

  it("sans attestation, rien ne passe", () => {
    const { w, honest, incoming } = setup();
    w.deliver(incoming(A.getAllChanges(honest), A.getHeads(honest)));
    const data = (w.messages[0] as (Message & { data: Uint8Array }) | undefined)?.data;
    expect(data ? A.decodeSyncMessage(data).changes : []).toHaveLength(0);
  });

  it("jette un message mal formé", () => {
    const { w, incoming } = setup();
    w.deliver(incoming([new Uint8Array(40)], []));
    expect(w.messages).toHaveLength(0);
    expect(w.drops).toContain("sync message dropped");
  });
});

describe("liste de membres saturée", () => {
  it("prévient l'interface, avant comme après l'abonnement", () => {
    const w = wire();
    const seen: string[] = [];
    w.adapter.onSealed(() => seen.push("early"));
    expect(w.adapter.sealed).toBe(false);
    w.deliver(encodeEnvelope({ kind: Kind.Sealed, peer: "me", payload: new Uint8Array() }));
    expect(w.adapter.sealed).toBe(true);
    w.adapter.onSealed(() => seen.push("late"));
    expect(seen).toEqual(["early", "late"]);
  });

  it("Go ne signe plus : un pair saturé n'est pas couvert", async () => {
    const net = new SimNet(1);
    const a = kit.peer(net, "a");
    const b = kit.peer(net, "b");
    mesh(net, ["a", "b"]);
    await until(() => a.connected.size === 1 && b.connected.size === 1);
    const ha = a.repo.create<Doc>({ text: "" });
    const hb = await b.repo.find<Doc>(ha.url);
    net.seal("a");
    await until(() => a.adapter.sealed);
    type(ha, "s");
    await net.settle();
    expect(net.refused).toContain("a");
    expect(text(hb)).not.toContain("s");
    expect(b.adapter.coverageStats.removed).toBeGreaterThan(0);
  });
});
