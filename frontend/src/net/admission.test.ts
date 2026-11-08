// @vitest-environment node

import * as repoModule from "@automerge/automerge-repo";
import { generateAutomergeUrl, Repo } from "@automerge/automerge-repo";
import { type CBORValue, decodeCBOR, encodeCBOR } from "@inkwell/protocol";
import { encode as arbitraryEncode } from "cbor-x";
import fc from "fast-check";
import { expect, it, vi } from "vitest";
import { Admission, parsePeerMessage, validFilePath, validPresenceName } from "./admission";
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
/** Encodes a value that conforms to the CBOR validity profile (§9.2.1). */
const encodeValid = (v: unknown) => encodeCBOR(v as CBORValue);
/** Encodes an arbitrary JS value as CBOR (may produce invalid CBOR per §9.2.1). */
const encodeArbitrary = (v: unknown) => arbitraryEncode(v);
const parseValid = (v: unknown) => parsePeerMessage(encodeValid(v), context);
const parseArbitrary = (v: unknown) => parsePeerMessage(encodeArbitrary(v), context);
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
])("2 rejects %j", (v) => expect(parseArbitrary(v)).toBeUndefined());
it("2 rebuilds own fields and never throws", () => {
  expect(parseValid({ ...request, extra: "discard" })).toEqual(request);
  expect(parseArbitrary({ ...request, ["__proto__"]: {} })).toBeUndefined();
  fc.assert(
    fc.property(fc.anything(), (v) => {
      expect(() => parseArbitrary(v)).not.toThrow();
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
    data: encodeValid({ __presence: { type: "heartbeat" } }),
  };
}
it("2 validates handshakes and presence", () => {
  for (const v of [
    { type: "arrive", senderId: "peer", targetId: "me", peerMetadata: [] },
    { type: "arrive", senderId: "peer", targetId: "me", projectUrl: "bad" },
    { type: "arrive", senderId: "peer", targetId: "me", extra: "x".repeat(1024) },
  ])
    expect(parseValid(v)).toBeUndefined();
  for (const patch of [
    { sessionId: "x".repeat(1 << 20) },
    { count: 0 },
    { count: 1.2 },
    { sessionId: "BAD" },
    { data: encodeValid({ __presence: { type: "update", channel: "evil", value: "x" } }) },
    { data: new Uint8Array(1025) },
  ])
    expect(parseArbitrary({ ...ephemeral(), ...patch })).toBeUndefined();
});
const presenceOf = (p: unknown) => ({
  ...ephemeral(),
  data: encodeValid({ __presence: p }),
});
const fileUpdate = (value: unknown) => presenceOf({ type: "update", channel: "file", value });
const fileSnapshot = (file: unknown) =>
  presenceOf({ type: "snapshot", state: { name: "Bob", file } });
/** Variant for property tests with arbitrary (possibly invalid) inner payloads. */
const arbitraryFileSnapshot = (file: unknown) => ({
  ...ephemeral(),
  data: arbitraryEncode({ __presence: { type: "snapshot", state: { name: "Bob", file } } }),
});
const arbitraryPresenceOf = (p: unknown) => ({
  ...ephemeral(),
  data: arbitraryEncode({ __presence: p }),
});
it("2 admits a presence file that is a safe project path", () => {
  for (const file of [
    "main.typ",
    "figures.csv",
    "docs/a b/é.md",
    "x".repeat(800),
    "日本語/ノート.md",
  ]) {
    expect(parseValid(fileUpdate(file)), file).toBeDefined();
    expect(parseValid(fileSnapshot(file)), file).toBeDefined();
  }
});
it("2 drops a presence file that is not a safe project path", () => {
  for (const file of [
    "",
    "/etc/passwd",
    "a/../b",
    "..",
    ".",
    "a/./b",
    "a//b",
    "a/",
    "a\\b",
    "a\u0000b",
    "a\nb",
    "a\u007fb",
    "a\u0085b",
    "x".repeat(1025),
    "é".repeat(600),
    "e\u0301.md",
    42,
    null,
    ["main.typ"],
    { a: 1 },
    ...SPOOFING.map((c) => `a${c}b.md`),
  ]) {
    expect(parseValid(fileUpdate(file))).toBeUndefined();
    // A snapshot loses the bad file only: the person's name still arrives, unless the whole
    // message is over the 1024-byte ephemeral bound.
    const snapshot = fileSnapshot(file);
    const want = snapshot.data.length > 1024 ? undefined : { name: "Bob" };
    expect(stateOf(parseValid(snapshot)), JSON.stringify(file)).toEqual(want);
  }
});
/** Bidirectional and format controls that make one name or path look like another. */
const SPOOFING = [
  "\u202a",
  "\u202b",
  "\u202c",
  "\u202d",
  "\u202e",
  "\u2066",
  "\u2067",
  "\u2068",
  "\u2069",
  "\u200e",
  "\u200f",
  "\u200b",
  "\u2060",
  "\ufeff",
  "\u00ad",
  "\u061c",
];
/** The presence state an admitted snapshot carries, as the repository will decode it. */
function stateOf(msg: unknown): unknown {
  const data = (msg as { data?: Uint8Array } | undefined)?.data;
  if (!data) return undefined;
  return (decodeCBOR(data) as { __presence: { state: unknown } }).__presence.state;
}
it("2 drops a presence name with hidden or reordering characters", () => {
  for (const c of SPOOFING) {
    const name = `Ali${c}ce`;
    expect(validPresenceName(name), JSON.stringify(name)).toBe(false);
    expect(
      parseValid(presenceOf({ type: "update", channel: "name", value: name })),
      JSON.stringify(name),
    ).toBeUndefined();
    expect(
      stateOf(parseValid(presenceOf({ type: "snapshot", state: { name, file: "main.typ" } }))),
    ).toEqual({ file: "main.typ" });
  }
  for (const name of ["Alice", "Zoë", "李雷", "אבי", "مریم", "", "x".repeat(128)])
    expect(validPresenceName(name), name).toBe(true);
  expect(validPresenceName("x".repeat(129))).toBe(false);
  expect(validPresenceName("a\nb")).toBe(false);
  expect(validPresenceName("\ud800")).toBe(false);
});
it("2 keeps the valid keys of a snapshot and drops the others", () => {
  const msg = parseValid(
    presenceOf({
      type: "snapshot",
      state: { name: "Bob", file: "e\u0301.md", anchor: "a".repeat(65), head: "h", extra: 1 },
    }),
  );
  expect(stateOf(msg)).toEqual({ name: "Bob", head: "h" });
  const untouched = presenceOf({ type: "snapshot", state: { name: "Bob", file: "main.typ" } });
  expect([...(parseValid(untouched) as { data: Uint8Array }).data]).toEqual([...untouched.data]);
  expect(stateOf(parseValid(presenceOf({ type: "snapshot", state: "x" })))).toBeUndefined();
});
it("2 an admitted snapshot only ever carries safe values", () => {
  const value = fc.oneof(
    fc.string({ unit: "binary" }),
    fc.constantFrom(...SPOOFING, "main.typ", "a/../b", "Bob", "e\u0301"),
    fc.anything(),
  );
  fc.assert(
    fc.property(
      fc.dictionary(fc.constantFrom("name", "file", "anchor", "head", "extra"), value),
      (state) => {
        const s = stateOf(parseArbitrary(arbitraryPresenceOf({ type: "snapshot", state })));
        if (s === undefined) return;
        for (const [k, v] of Object.entries(s as Record<string, unknown>)) {
          expect(["name", "file", "anchor", "head"]).toContain(k);
          // Admitted values must be safe, not necessarily equal to input (filtering may occur).
          if (k === "name") expect(validPresenceName(v)).toBe(true);
          if (k === "file") expect(validFilePath(v)).toBe(true);
        }
      },
    ),
    { examples: [[{ name: "Ali\u202ece", file: "x\u2066.md" }], [{ name: "Bob", file: "a\\b" }]] },
  );
});
it("2 validFilePath refuses what CBOR cannot carry, such as a lone surrogate", () => {
  expect(validFilePath("\ud800.md")).toBe(false);
  expect(validFilePath("a".repeat(1024))).toBe(true);
  expect(validFilePath("a".repeat(1025))).toBe(false);
  expect(validFilePath("é".repeat(513))).toBe(false);
});
it("2 never admits a presence file that a property finds unsafe", () => {
  fc.assert(
    fc.property(fc.string({ unit: "binary" }), (file) => {
      const admitted = parseValid(fileUpdate(file)) !== undefined;
      if (admitted) {
        expect(new TextEncoder().encode(file).length).toBeLessThanOrEqual(1024);
        expect(file.normalize("NFC")).toBe(file);
        expect(file.split("/")).not.toContain("..");
        expect(file.startsWith("/")).toBe(false);
        expect([...file].some((c) => c.charCodeAt(0) < 32 || c === "\u007f")).toBe(false);
        expect(/[\p{Cf}\p{Cs}\\]/u.test(file)).toBe(false);
      }
    }),
  );
  fc.assert(
    fc.property(fc.anything(), (file) => {
      expect(() => parseArbitrary(arbitraryFileSnapshot(file))).not.toThrow();
    }),
  );
});
it("3 P2 drops 50,000 invented ids", () => {
  for (let i = 0; i < 50000; i++)
    expect(parseValid({ ...request, documentId: `invented${i}` })).toBeUndefined();
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
  a.setMembers(["peer", "bad", "honest", "p", "other"]);
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
  a.setMembers(["peer", "bad", "honest", "p", "other"]);
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
  a.setMembers(["peer", "bad", "honest", "p", "other"]);
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
  a.setMembers(["peer", "bad", "honest", "p", "other"]);
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
  a.setMembers(["peer", "bad", "honest", "p", "other"]);
  let n = 0;
  for (let i = 0; i < 1000; i++) {
    now = i;
    if (a.ephemeral("p", ephemeral("a", i + 1) as never)) n++;
  }
  expect(n).toBeLessThanOrEqual(192);
  const first = a.ephemeral("other", ephemeral() as never);
  const other = new Admission(Date.now, "newadapter");
  other.setMembers(["p"]);
  const second = other.ephemeral("p", ephemeral() as never);
  expect((first as unknown as { sessionId: string }).sessionId).not.toBe(
    (second as unknown as { sessionId: string }).sessionId,
  );
});

it.each([
  { t: "hello" },
  { t: "ask" },
  { t: "take", epoch: 1 },
  { t: "state", epoch: 1, seq: 2, slide: 2, step: 0, screen: "show" },
  { t: "laser", epoch: 1, x: 5000, y: 2500 },
  { t: "laser", epoch: 1, x: 2, y: 0 },
  { t: "give", epoch: 2, to: "other" },
  { t: "deny", to: "other" },
  { t: "end", epoch: 2 },
])("admits bounded presentation messages: %j", (message) => {
  const data = encodeValid({ __present: message });
  const parsed = parseValid({ ...ephemeral(), data });
  expect(parsed).toBeDefined();
  expect(parsed && "data" in parsed ? [...(parsed.data ?? [])] : undefined).toEqual([...data]);
});
it("keeps presentation schema, size, scope and session bounds", () => {
  for (const message of [
    { t: "state", epoch: 1, seq: 1, slide: 0, step: 0, screen: "show" },
    { t: "laser", epoch: 1, x: 10001, y: 0 },
    { t: "invented" },
    { t: "give", epoch: 1, to: "x".repeat(65) },
    { t: "hello", padding: "x".repeat(1024) },
  ])
    expect(
      parseValid({ ...ephemeral(), data: encodeValid({ __present: message }) }),
    ).toBeUndefined();
  expect(
    parseArbitrary({
      ...ephemeral(),
      data: arbitraryEncode({ __present: { t: "laser", epoch: 1, x: 0.5, y: 0.5 } }),
    }),
  ).toBeUndefined();
  const m = { ...ephemeral(), data: encodeValid({ __present: { t: "hello" } }) };
  expect(parseValid({ ...m, sessionId: "BAD" })).toBeUndefined();
  expect(parseValid({ ...m, documentId: "unlisted" })).toBeUndefined();
});

it("a presentation cannot smuggle an unchecked presence payload", () => {
  expect(
    parseArbitrary({
      ...ephemeral(),
      data: encodeValid({
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
  a.setMembers(["peer", "p"]);
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
    parseArbitrary({
      type: "arrive",
      senderId: "peer",
      targetId: "me",
      peerMetadata: { ["__proto__"]: {} },
    }),
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
it("A14 strangers cannot consume member entries or cooldowns", () => {
  const a = new Admission(() => 0);
  a.setMembers(["member"]);
  const maps = stateMaps(() => {
    a.arrive("member", {});
    for (let i = 0; i < 2000; i++) {
      expect(a.arrive(`s${i}`, {})).toBe(false);
      expect(a.hold(`s${i}`, request as never)).toBe(false);
      expect(a.ephemeral(`s${i}`, ephemeral() as never)).toBeUndefined();
    }
  });
  expect(maps.peers.size).toBe(1);
  expect(maps.cooldowns.size).toBe(0);
});
