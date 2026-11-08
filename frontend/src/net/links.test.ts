// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  decodeLinks,
  encodeEnvelope,
  Kind,
  LINK_FIXED,
  LOSS_UNKNOWN,
  MalformedEnvelope,
  type PeerLink,
} from "./envelope";
import { links, toLinkView } from "./links.svelte";
import { type SocketLike, WailsNetworkAdapter } from "./WailsNetworkAdapter";

const unhex = (s: string) => Uint8Array.from(s.match(/../g) ?? [], (h) => Number.parseInt(h, 16));

/** One entry as Go encodes it: the golden bytes are the ones of the protocol vector `bridge-links`. */
function entry(peer: string, path = 2, quality = 2, scope = 1): Uint8Array {
  const b = new Uint8Array(1 + peer.length + LINK_FIXED);
  b[0] = peer.length;
  for (let i = 0; i < peer.length; i++) b[1 + i] = peer.charCodeAt(i);
  b[1 + peer.length] = path;
  b[2 + peer.length] = quality;
  b[1 + peer.length + 2 + 4 + 2 + 2 + 12 + 2] = scope;
  return b;
}
const list = (...entries: Uint8Array[]) =>
  Uint8Array.from([entries.length >> 8, entries.length & 255, ...entries.flatMap((e) => [...e])]);

describe("decodeLinks", () => {
  it("reads the figures in the wire order", () => {
    const e = entry("peer-a");
    const at = 1 + 6 + 2; // after peer, path, quality
    new DataView(e.buffer).setUint32(at, 61000); // pathAgeMs
    new DataView(e.buffer).setUint16(at + 4, 2); // upgrades
    new DataView(e.buffer).setUint32(at + 8, 180); // rttMs
    new DataView(e.buffer).setUint16(at + 20, 43); // lossPermille
    new DataView(e.buffer).setUint32(at + 23, 120000); // upMs
    new DataView(e.buffer).setUint32(e.length - 4, 9); // resets
    const [l] = decodeLinks(list(e)) as [PeerLink];
    expect(l).toMatchObject({
      peer: "peer-a",
      path: "direct",
      quality: "fair",
      pathAgeMs: 61000,
      upgrades: 2,
      rttMs: 180,
      lossPermille: 43,
      lossScope: "path",
      upMs: 120000,
      resets: 9,
    });
  });

  it("accepts the empty list", () => {
    expect(decodeLinks(unhex("0000"))).toEqual([]);
  });

  it.each([
    ["empty", () => new Uint8Array()],
    ["unsorted", () => list(entry("b"), entry("a"))],
    ["duplicate", () => list(entry("a"), entry("a"))],
    ["path 4", () => list(entry("a", 4))],
    ["quality 4", () => list(entry("a", 2, 4))],
    ["scope 3", () => list(entry("a", 2, 2, 3))],
    ["n 1025", () => unhex("0401")],
    ["truncated", () => list(entry("a")).subarray(0, 20)],
    ["trailing byte", () => Uint8Array.from([...list(entry("a")), 0])],
  ])("rejects %s", (_name, make) => {
    expect(() => decodeLinks(make())).toThrow(MalformedEnvelope);
  });
});

describe("toLinkView", () => {
  const base = decodeLinks(list(entry("a")))[0] as PeerLink;
  it("reads permille as percent and no measure as null", () => {
    const v = toLinkView({ ...base, rttMs: 0, lossPermille: LOSS_UNKNOWN });
    expect(v).toMatchObject({ rttMs: null, lossPct: null, lossScope: null });
    expect(toLinkView({ ...base, quality: "unknown" }).quality).toBeNull();
    const w = toLinkView({ ...base, rttMs: 12, lossPermille: 43 });
    expect(w).toMatchObject({ rttMs: 12, lossPct: 4.3, lossScope: "path" });
  });
});

describe("view", () => {
  it("ages what Go sent once, and leaves a disconnected link and a missing measure alone", () => {
    const [l] = decodeLinks(list(entry("a"))) as [PeerLink];
    links.set([{ ...l, pathAgeMs: 1000, upMs: 5000, rttMs: 10, rttAgeMs: 100 }], 10_000);
    expect(links.view(12_000)[0]).toMatchObject({ pathAgeMs: 3000, upMs: 7000, rttAgeMs: 2100 });
    links.set([{ ...l, upMs: 0, rttMs: 0, rttAgeMs: 0 }], 10_000);
    expect(links.view(12_000)[0]).toMatchObject({ upMs: 0, rttAgeMs: 0 });
  });
});

describe("view sync counts", () => {
  it("NI-4 C4, C5: merges the adapter's per-peer sync counts into each link", () => {
    const [l] = decodeLinks(list(entry("a"))) as [PeerLink];
    const counts = new Map([["a", { sent: 7, recv: 3 }]]);
    links.syncCounts = (peer) => counts.get(peer) ?? { sent: 0, recv: 0 };
    links.set([l], 10_000);
    expect(links.view(10_000)[0]).toMatchObject({ syncSent: 7, syncRecv: 3 });
    counts.set("a", { sent: 9, recv: 3 }); // counts move between two reports from Go
    expect(links.view(10_000)[0]).toMatchObject({ syncSent: 9, syncRecv: 3 });
    links.syncCounts = () => ({ sent: 0, recv: 0 });
  });
});

describe("adapter", () => {
  it("hands the links of a Links frame to onLinks and drops a malformed one", () => {
    const socket: SocketLike = {
      binaryType: "",
      send: () => {},
      close: () => {},
      onopen: null,
      onclose: null,
      onmessage: null,
    };
    const dropped: string[] = [];
    const adapter = new WailsNetworkAdapter(() => socket, { onDrop: (r) => dropped.push(r) });
    adapter.onLinks((l) => links.set(l));
    adapter.connect("me" as never);
    const deliver = (payload: Uint8Array) => {
      const frame = Uint8Array.from([1, Kind.Links, 2, 109, 101, ...payload]);
      socket.onmessage?.({ data: frame.buffer } as MessageEvent);
    };
    deliver(list(entry("a"), entry("b")));
    expect(links.list.map((l) => l.peer)).toEqual(["a", "b"]);
    deliver(list(entry("b"), entry("a")));
    expect(links.list.map((l) => l.peer)).toEqual(["a", "b"]);
    expect(dropped).toEqual([expect.stringContaining("links")]);
    expect(() =>
      encodeEnvelope({ kind: Kind.Links, peer: "me", payload: list(entry("a")) }),
    ).not.toThrow();
  });
});
