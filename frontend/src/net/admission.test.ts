// @vitest-environment node

import * as repoModule from "@automerge/automerge-repo";
import { generateAutomergeUrl, Repo } from "@automerge/automerge-repo";
import { encode } from "cbor-x";
import fc from "fast-check";
import { expect, it, vi } from "vitest";
import { Admission, parsePeerMessage } from "./admission";
import { AdmittedSet } from "./scope";

vi.mock("@automerge/automerge-repo", { spy: true });

const context = { from: "peer", me: "me", announced: true, has: (id: string) => id === "doc" };
const request = {
  type: "request",
  senderId: "peer",
  targetId: "me",
  documentId: "doc",
  data: new Uint8Array(),
};
const parse = (v: unknown) => parsePeerMessage(encode(v), context);
it.each([
  null,
  undefined,
  4,
  [],
  {},
  { type: "unknown" },
  { type: "remote-heads-changed" },
  { ...request, data: [] },
  { ...request, senderId: "other" },
  { ...request, targetId: "other" },
  { ...request, documentId: "x".repeat(65) },
])("2 rejects %j", (v) => expect(parse(v)).toBeUndefined());
it("2 rebuilds own fields and never throws", () => {
  expect(parse({ ...request, extra: "discard" })).toEqual(request);
  expect(parsePeerMessage(encode({ ...request, ["__proto__"]: {} }), context)).toBeUndefined();
  fc.assert(
    fc.property(fc.anything(), (v) => {
      expect(() => parse(v)).not.toThrow();
    }),
  );
  fc.assert(
    fc.property(fc.uint8Array(), (v) => {
      expect(() => parsePeerMessage(v, context)).not.toThrow();
    }),
  );
});
function ephemeral(sessionId = "a", count = 1) {
  return {
    ...request,
    type: "ephemeral",
    sessionId,
    count,
    data: encode({ __presence: { type: "heartbeat" } }),
  };
}
it("2 validates handshakes and presence", () => {
  for (const v of [
    { type: "arrive", senderId: "peer", peerMetadata: [] },
    { type: "arrive", senderId: "peer", projectUrl: "bad" },
    { type: "arrive", senderId: "peer", extra: "x".repeat(1024) },
  ])
    expect(parse(v)).toBeUndefined();
  for (const patch of [
    { sessionId: "x".repeat(1 << 20) },
    { count: 0 },
    { count: 1.2 },
    { sessionId: "BAD" },
    { data: encode({ __presence: { type: "update", channel: "evil", value: "x" } }) },
    { data: new Uint8Array(1025) },
  ])
    expect(parse({ ...ephemeral(), ...patch })).toBeUndefined();
});
it("3 P2 drops 50,000 invented ids", () => {
  for (let i = 0; i < 50000; i++)
    expect(parse({ ...request, documentId: `invented${i}` })).toBeUndefined();
});
it("4 sticky scope caps distinct variants and changes epoch", () => {
  const repo = new Repo();
  const root = repo.create({}).url;
  const set = new AdmittedSet();
  set.setRoot(root);
  const urls = Array.from({ length: 10000 }, () => generateAutomergeUrl());
  for (let i = 0; i < 10000; i++) set.refresh({ meta: { variants: [{ url: urls[i] }] } });
  expect(set.ids.size).toBe(257);
  const id = [...set.ids][1];
  if (!id) throw Error("missing variant");
  set.refresh({ meta: { variants: [] } });
  expect(set.has(id)).toBe(true);
  const epoch = set.epoch;
  const next = urls[0];
  if (!next) throw Error("missing root");
  set.setRoot(next);
  expect(set.epoch).toBe(epoch + 1);
  expect(set.ids.size).toBe(1);
});
it("5 P1 rewrites sessions, enforces counts and rate, resets on down", () => {
  let now = 0;
  const a = new Admission(() => now);
  const sessions = new Set();
  let delivered = 0;
  for (let i = 0; i < 10000; i++) {
    const m = a.ephemeral("peer", ephemeral(i.toString(36)) as never);
    if (m) {
      sessions.add((m as unknown as { sessionId: string }).sessionId);
      delivered++;
    }
  }
  expect(sessions.size).toBe(1);
  expect(delivered).toBe(128);
  now = 1000;
  expect(a.ephemeral("peer", ephemeral("restart") as never)).toBeDefined();
  expect(a.ephemeral("peer", ephemeral("restart") as never)).toBeUndefined();
  a.forget("peer");
  expect(a.ephemeral("peer", ephemeral("restart") as never)).toBeDefined();
});
it("6 re-arrive bucket keeps latest pending", () => {
  let now = 0;
  const a = new Admission(() => now);
  let accepted = 0;
  for (let i = 0; i < 40; i++) if (a.arrive("peer", { i })) accepted++;
  expect(accepted).toBe(3);
  now = 20000;
  expect(a.pending("peer")).toEqual({ i: 39 });
  expect(a.pending("peer")).toBeUndefined();
});
it("7 per peer hold bounds and persistent cooldown", () => {
  let now = 0;
  const a = new Admission(() => now);
  for (let i = 0; i < 4; i++) expect(a.hold("bad", request as never)).toBe(true);
  expect(a.hold("bad", request as never)).toBe(false);
  expect(a.held("bad")).toBe(0);
  a.forget("bad");
  expect(a.blocked("bad")).toBe(true);
  expect(a.hold("honest", request as never)).toBe(true);
  now = 30000;
  expect(a.blocked("bad")).toBe(false);
});

it("7 byte limit, MaxFrame exception, isolation and exponential reset cooldown", () => {
  let now = 0;
  const a = new Admission(() => now);
  const bytes = new Uint8Array(5 * 1024 * 1024);
  expect(a.hold("bad", { ...request, data: bytes } as never)).toBe(true);
  expect(a.hold("bad", { ...request, documentId: "other", data: bytes } as never)).toBe(false);
  expect(a.hold("honest", { ...request, data: new Uint8Array(64 * 1024 * 1024) } as never)).toBe(
    true,
  );
  expect(a.drain("honest", () => true)).toHaveLength(1);
  for (let attempt = 0; attempt < 7; attempt++) {
    now += 600000;
    a.forget("bad");
    for (let i = 0; i < 4; i++) a.hold("bad", request as never);
    a.hold("bad", request as never);
    const expected = Math.min(600000, 60000 * 2 ** attempt);
    now += expected - 1;
    expect(a.blocked("bad")).toBe(true);
    now++;
    expect(a.blocked("bad")).toBe(false);
  }
});
it("5 rate never exceeds 192 in one second and new adapter uses a new session", () => {
  let now = 0;
  const a = new Admission(() => now);
  let n = 0;
  for (let i = 0; i < 1000; i++) {
    now = i;
    if (a.ephemeral("p", ephemeral("a", i + 1) as never)) n++;
  }
  expect(n).toBeLessThanOrEqual(192);
  const first = a.ephemeral("other", ephemeral() as never);
  const second = new Admission(Date.now, "newadapter").ephemeral("p", ephemeral() as never);
  expect((first as unknown as { sessionId: string }).sessionId).not.toBe(
    (second as unknown as { sessionId: string }).sessionId,
  );
});

it.each([
  { t: "hello" },
  { t: "ask" },
  { t: "take", epoch: 1 },
  { t: "state", epoch: 1, seq: 2, slide: 2, step: 0, screen: "show" },
  { t: "laser", epoch: 1, x: 0.5, y: 0.5 },
  { t: "give", epoch: 2, to: "other" },
  { t: "deny", to: "other" },
  { t: "end", epoch: 2 },
])("admits bounded presentation messages: %j", (message) => {
  const data = encode({ __present: message });
  const parsed = parse({ ...ephemeral(), data });
  expect(parsed).toBeDefined();
  expect(parsed && "data" in parsed ? [...(parsed.data ?? [])] : undefined).toEqual([...data]);
});
it("keeps presentation schema, size, scope and session bounds", () => {
  for (const message of [
    { t: "state", epoch: 1, seq: 1, slide: 0, step: 0, screen: "show" },
    { t: "laser", epoch: 1, x: 2, y: 0 },
    { t: "invented" },
    { t: "give", epoch: 1, to: "x".repeat(65) },
    { t: "hello", padding: "x".repeat(1024) },
  ])
    expect(parse({ ...ephemeral(), data: encode({ __present: message }) })).toBeUndefined();
  const m = { ...ephemeral(), data: encode({ __present: { t: "hello" } }) };
  expect(parse({ ...m, sessionId: "BAD" })).toBeUndefined();
  expect(parse({ ...m, documentId: "unlisted" })).toBeUndefined();
});

it("a presentation cannot smuggle an unchecked presence payload", () => {
  expect(
    parse({
      ...ephemeral(),
      data: encode({
        __present: { t: "hello" },
        __presence: { type: "update", channel: "name", value: "x".repeat(129) },
      }),
    }),
  ).toBeUndefined();
});

it("rejects a 1 MiB variant URL before validation", () => {
  const set = new AdmittedSet();
  const validate = vi.mocked(repoModule.isValidAutomergeUrl);
  validate.mockClear();
  set.refresh({ meta: { variants: [{ url: `automerge:${"z".repeat(1024 * 1024)}` }] } });
  expect(validate).not.toHaveBeenCalled();
  validate.mockClear();
  expect(set.ids.size).toBe(0);
});
it("disconnect and accepted arrives cannot refresh peer budgets", () => {
  const a = new Admission(() => 0);
  for (let i = 1; i <= 128; i++)
    expect(a.ephemeral("peer", ephemeral("a", i) as never)).toBeDefined();
  for (let i = 0; i < 3; i++) {
    a.forget("peer");
    expect(a.arrive("peer", {})).toBe(true);
    expect(a.ephemeral("peer", ephemeral("new", i + 1) as never)).toBeUndefined();
  }
  for (let i = 0; i < 10000; i++) {
    a.forget("peer");
    expect(a.arrive("peer", {})).toBe(false);
    expect(a.ephemeral("peer", ephemeral("new", i + 4) as never)).toBeUndefined();
  }
});
it("rejects an own __proto__ field in nested metadata", () => {
  expect(
    parse({ type: "arrive", senderId: "peer", peerMetadata: { ["__proto__"]: {} } }),
  ).toBeUndefined();
});

// Inspect actual map sizes without exposing mutable admission state in production.
function stateMaps(run: () => void) {
  const sets = vi.spyOn(Map.prototype, "set");
  try {
    run();
    const peers = sets.mock.calls.findIndex(([, value]) => Array.isArray(value?.queue));
    const cooldowns = sets.mock.calls.findIndex(([, value]) => typeof value?.until === "number");
    return {
      peers: sets.mock.contexts[peers] instanceof Map ? sets.mock.contexts[peers] : new Map(),
      cooldowns:
        sets.mock.contexts[cooldowns] instanceof Map ? sets.mock.contexts[cooldowns] : new Map(),
    };
  } finally {
    sets.mockRestore();
  }
}
it("bounds both maps across 10,000 distinct ids and unknown PeerDowns", () => {
  const a = new Admission(() => 0);
  const down = stateMaps(() => {
    for (let i = 0; i < 10000; i++) {
      a.forget(`down${i}`);
      a.reset(`down${i}`);
    }
  });
  expect(down.peers.size).toBe(0);
  const maps = stateMaps(() => {
    for (let i = 0; i < 10000; i++) {
      a.ephemeral(`e${i}`, ephemeral() as never);
      a.arrive(`a${i}`, {});
      for (let j = 0; j < 5; j++) if (!a.hold(`h${i}`, request as never)) break;
    }
  });
  expect(maps.peers.size).toBe(1024);
  expect(maps.cooldowns.size).toBeLessThanOrEqual(1024);
}, 30000);
it("evicts oldest fully refilled budgets with no extra budget on recreation", () => {
  let now = 0;
  const a = new Admission(() => now);
  const kept = new Admission(() => now);
  for (const admission of [a, kept]) {
    for (let i = 1; i <= 128; i++) admission.ephemeral("old", ephemeral("a", i) as never);
    admission.arrive("old", {});
  }
  now = 2000; // Ephemeral full, handshake still depleted: must retain it.
  const maps = stateMaps(() => {
    for (let i = 0; i < 1023; i++) a.ephemeral(`p${i}`, ephemeral() as never);
  });
  a.ephemeral("pressure", ephemeral() as never);
  expect(maps.peers.has("old")).toBe(true);
  now = 20000; // Both budgets full: recreation must give exactly the kept budget.
  a.ephemeral("pressure", ephemeral() as never);
  expect(maps.peers.has("old")).toBe(false);
  for (let i = 1; i <= 129; i++)
    expect(!!a.ephemeral("old", ephemeral("new", i) as never)).toBe(
      !!kept.ephemeral("old", ephemeral("new", i) as never),
    );
  for (let i = 0; i < 4; i++) expect(a.arrive("old", {})).toBe(kept.arrive("old", {}));
});
it("cap pressure preserves queues, pending handshakes and active cooldowns", () => {
  let now = 0;
  const a = new Admission(() => now);
  a.hold("queued", request as never);
  for (let i = 0; i < 4; i++) a.arrive("pending", { i });
  const maps = stateMaps(() => {
    for (let i = 0; i < 10000; i++) {
      for (let j = 0; j < 5; j++) if (!a.hold(`bad${i}`, request as never)) break;
    }
  });
  expect(maps.cooldowns.size).toBe(1022);
  a.forget("bad0");
  expect(a.retryAfter("bad0")).toBe(30000);
  expect(a.arrive("bad0", {})).toBe(false);
  expect(a.held("queued")).toBe(1);
  now = 20000;
  expect(a.pending("pending")).toEqual({ i: 3 });
  expect(a.retryAfter("bad0")).toBe(10000);
  now = 30000;
  for (let i = 0; i < 10000; i++) {
    a.forget(`bad${i}`);
    for (let j = 0; j < 5; j++) if (!a.hold(`next${i}`, request as never)) break;
  }
  expect(maps.peers.size).toBe(1024);
  expect(maps.cooldowns.size).toBeLessThanOrEqual(1024);
}, 30000);
