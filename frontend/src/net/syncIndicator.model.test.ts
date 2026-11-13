// @vitest-environment node
import { next as A } from "@automerge/automerge";
import type { DocHandle, DocumentId } from "@automerge/automerge-repo";
import * as fc from "fast-check";
import { expect, it, vi } from "vitest";
import { decodeCBOR, Kind, type PeerLink } from "./envelope";
import { SimNet } from "./simnet";
import { globalSyncState, observePeers, type PeerSyncState, peerSyncState } from "./syncIndicator";
import { Kit, type TestPeer } from "./testkit";

/**
 * W33 model-based test of the sync indicator (spec: docs/plans/2026-11-08-netsim-impaired-plan.md,
 * "Honest sync indicator: executable specification"; model: docs/protocol/tla/SyncIndicator.tla).
 *
 * Three real automerge-repo Repos (me, p1, p2) behind real WailsNetworkAdapters, joined by a
 * step-by-step SimNet: a sync frame is held per directed link until a command delivers or loses it.
 * Me is the hub, as in the model. The safety oracle (PeerHonest, GlobalHonest) runs after every
 * command against the peers' real documents, which the indicator never reads; the liveness oracle
 * (EventuallySynced) runs after the quiet phase.
 */

type Name = "me" | "p1" | "p2";
const REMOTES = ["p1", "p2"] as const;
type Remote = (typeof REMOTES)[number];
const key = (from: string, to: string) => `${from}>${to}`;
const TICK_MS = 250;

/** A SimNet whose sync frames wait, per directed link, until a command moves them. */
class StepNet extends SimNet {
  readonly held = new Map<string, Uint8Array[]>();
  readonly socks = new Map<
    string,
    { deliver(kind: Kind, peer: string, payload?: Uint8Array): void }
  >();

  override socket(peer: string) {
    const s = super.socket(peer);
    this.socks.set(peer, s as never);
    return s;
  }
  override route(from: string, to: string, payload: Uint8Array): void {
    const message = decodeCBOR(payload) as { type?: string } | null;
    if (message?.type !== "sync" && message?.type !== "request") {
      super.route(from, to, payload);
      return;
    }
    if (!this.linkedTo(from).includes(to)) return;
    const queue = this.held.get(key(from, to)) ?? [];
    queue.push(payload);
    this.held.set(key(from, to), queue);
  }
  override partition(a: string, b: string): void {
    super.partition(a, b);
    this.held.delete(key(a, b));
    this.held.delete(key(b, a));
  }
  count(from: string, to: string): number {
    return this.held.get(key(from, to))?.length ?? 0;
  }
  total(): number {
    return [...this.held.values()].reduce((n, q) => n + q.length, 0);
  }
  deliver(from: string, to: string): boolean {
    const payload = this.held.get(key(from, to))?.shift();
    if (!payload) return false;
    this.inject(to, from, payload);
    return true;
  }
  lose(from: string, to: string): boolean {
    return this.held.get(key(from, to))?.shift() !== undefined;
  }
}

/** The widths of a Links entry after its peer (packages/protocol/src/envelope.ts, FIGURES). */
const LINK_WIDTHS = [1, 1, 4, 2, 2, 4, 4, 4, 2, 1, 4, 2, 4, 4, 2, 4, 4, 4, 4, 4, 4];
const PENDING_AT = LINK_WIDTHS.slice(0, 16).reduce((a, b) => a + b, 0);

function linksFrame(peer: string, pendingMsgs: number): Uint8Array {
  const body = new Uint8Array(1 + peer.length + LINK_WIDTHS.reduce((a, b) => a + b, 0));
  body[0] = peer.length;
  for (let i = 0; i < peer.length; i++) body[1 + i] = peer.charCodeAt(i);
  new DataView(body.buffer).setUint32(1 + peer.length + PENDING_AT, pendingMsgs);
  return Uint8Array.from([0, 1, ...body]);
}

type Shape = { log: Record<string, number> };

interface Real {
  net: StepNet;
  nodes: Record<Name, TestPeer>;
  docId: DocumentId;
  /** The last Links notification me received, by peer. */
  links: Map<string, PeerLink>;
}
interface Model {
  edits: Record<Name, number>;
  up: Record<Remote, boolean>;
  owed: Record<Remote, boolean>;
  downs: number;
  losses: number;
}

const advance = (ms = TICK_MS) => vi.advanceTimersByTimeAsync(ms);

const handleOf = (real: Real, name: Name) =>
  real.nodes[name].repo.handles[real.docId] as DocHandle<Shape>;

/** What the header would derive for each announced peer of me, from observables only. */
function observedBy(real: Real) {
  const me = real.nodes.me;
  return observePeers(
    me.adapter,
    (id) => {
      const h = Object.hasOwn(me.repo.handles, id) ? me.repo.handles[id as DocumentId] : undefined;
      return h?.isReady() ? h.doc() : undefined;
    },
    (peer) => real.links.get(peer),
  );
}

function truth(real: Real): { peers: Map<string, PeerSyncState>; global: PeerSyncState } {
  const peers = new Map([...observedBy(real)].map(([p, o]) => [p, peerSyncState(o)] as const));
  return { peers, global: globalSyncState([...peers.values()]) };
}

/** Safety (PeerHonest, GlobalHonest): the ground truth behind a claimed « Synchronisé ». */
function honest(real: Real, peer: string): void {
  const me = real.nodes.me;
  const myDoc = handleOf(real, "me").doc();
  expect(me.adapter.announcedPeers(), `${peer} announced`).toContain(peer);
  const peerDoc = handleOf(real, peer as Name).doc();
  expect(A.hasHeads(peerDoc, A.getHeads(myDoc)), `${peer} holds every change of mine`).toBe(true);
  const theirs = me.adapter.heardHeads(peer).get(real.docId);
  expect(theirs, `${peer} heard`).toBeTruthy();
  expect(A.hasHeads(myDoc, theirs as string[]), `I hold every change ${peer} reported`).toBe(true);
}

/** The label of the rule, restated from the heads alone (the TLA+ header: behind, sending, synced). */
function labelOf(real: Real, peer: string, state: PeerSyncState): void {
  const theirs = real.nodes.me.adapter.heardHeads(peer).get(real.docId);
  const myDoc = handleOf(real, "me").doc();
  let expected: PeerSyncState = "checking";
  if (theirs) {
    const same = [...A.getHeads(myDoc)].sort().join() === [...theirs].sort().join();
    expected = !A.hasHeads(myDoc, [...theirs]) ? "behind" : same ? "synced" : "sending";
  }
  expect(state, `state of ${peer}`).toBe(expected);
}

function oracle(real: Real): void {
  const { peers, global } = truth(real);
  for (const [peer, state] of peers) labelOf(real, peer, state);
  for (const [peer, state] of peers) if (state === "synced") honest(real, peer);
  if (global === "synced") {
    const announced = real.nodes.me.adapter.announcedPeers();
    expect(announced.length, "synced needs a connected member").toBeGreaterThan(0);
    for (const peer of announced) honest(real, peer);
  }
}

function resync(real: Real, peer: Remote): void {
  real.net.partition("me", peer);
  real.net.link("me", peer);
}

abstract class Command implements fc.AsyncCommand<Model, Real> {
  check(_m: Readonly<Model>): boolean {
    return true;
  }
  abstract exec(m: Model, r: Real): void | Promise<void>;
  async run(m: Model, r: Real): Promise<void> {
    await this.exec(m, r);
    await advance();
    oracle(r);
  }
  abstract toString(): string;
}

class Edit extends Command {
  constructor(readonly node: Name) {
    super();
  }
  override check(m: Readonly<Model>) {
    return m.edits[this.node] < 3;
  }
  exec(m: Model, r: Real) {
    m.edits[this.node]++;
    handleOf(r, this.node).change((d: Shape) => {
      d.log[this.node] = (d.log[this.node] ?? 0) + 1;
    });
  }
  toString = () => `Edit(${this.node})`;
}

class Deliver extends Command {
  constructor(
    readonly from: Name,
    readonly to: Name,
  ) {
    super();
  }
  exec(_m: Model, r: Real) {
    r.net.deliver(this.from, this.to);
  }
  toString = () => `Deliver(${this.from}>${this.to})`;
}

class Lose extends Command {
  constructor(
    readonly from: Name,
    readonly to: Name,
  ) {
    super();
  }
  override check(m: Readonly<Model>) {
    return m.losses < 2;
  }
  exec(m: Model, r: Real) {
    if (!r.net.lose(this.from, this.to)) return;
    m.losses++;
    m.owed[(this.from === "me" ? this.to : this.from) as Remote] = true;
  }
  toString = () => `Lose(${this.from}>${this.to})`;
}

class Resync extends Command {
  constructor(readonly peer: Remote) {
    super();
  }
  override check(m: Readonly<Model>) {
    return m.up[this.peer] && m.owed[this.peer];
  }
  exec(m: Model, r: Real) {
    resync(r, this.peer);
    m.owed[this.peer] = false;
  }
  toString = () => `Resync(${this.peer})`;
}

class Down extends Command {
  constructor(readonly peer: Remote) {
    super();
  }
  override check(m: Readonly<Model>) {
    return m.up[this.peer] && m.downs < 2;
  }
  exec(m: Model, r: Real) {
    r.net.partition("me", this.peer);
    m.up[this.peer] = false;
    m.owed[this.peer] = false;
    m.downs++;
  }
  toString = () => `Down(${this.peer})`;
}

class Up extends Command {
  constructor(readonly peer: Remote) {
    super();
  }
  override check(m: Readonly<Model>) {
    return !m.up[this.peer];
  }
  exec(m: Model, r: Real) {
    r.net.link("me", this.peer);
    m.up[this.peer] = true;
  }
  toString = () => `Up(${this.peer})`;
}

/** The coalesced Links notification: pendingMsgs = the frames held towards the peer. */
class LinksSample extends Command {
  constructor(readonly peer: Remote) {
    super();
  }
  exec(_m: Model, r: Real) {
    const frame = linksFrame(this.peer, r.net.count("me", this.peer));
    r.net.socks.get("me")?.deliver(Kind.Links, "me", frame.subarray(0));
  }
  toString = () => `Links(${this.peer})`;
}

const names: Name[] = ["me", "p1", "p2"];
const directed: [Name, Name][] = [
  ["me", "p1"],
  ["p1", "me"],
  ["me", "p2"],
  ["p2", "me"],
];
const remote = fc.constantFrom<Remote>(...REMOTES);
const commands = fc.commands(
  [
    fc.constantFrom(...names).map((n) => new Edit(n)),
    fc.constantFrom(...names).map((n) => new Edit(n)),
    ...[0, 1].map(() => fc.constantFrom(...directed).map(([a, b]) => new Deliver(a, b))),
    ...[0, 1].map(() => fc.constantFrom(...directed).map(([a, b]) => new Deliver(a, b))),
    fc.constantFrom(...directed).map(([a, b]) => new Lose(a, b)),
    remote.map((p) => new Resync(p)),
    remote.map((p) => new Down(p)),
    remote.map((p) => new Up(p)),
    remote.map((p) => new LinksSample(p)),
  ],
  { maxCommands: 24, size: "max" },
);

async function setup(kit: Kit): Promise<{ real: Real; model: Model }> {
  const net = new StepNet(1);
  const nodes = Object.fromEntries(names.map((n) => [n, kit.peer(net, n)])) as Record<
    Name,
    TestPeer
  >;
  const first = nodes.me.repo.create<Shape>({ log: {} });
  const binary = A.save(first.doc());
  for (const n of REMOTES) nodes[n].repo.import<Shape>(binary, { docId: first.documentId });
  const real: Real = { net, nodes, docId: first.documentId, links: new Map() };
  nodes.me.adapter.onLinks((list) => {
    for (const l of list) real.links.set(l.peer, l);
  });
  await advance(10);
  for (const n of REMOTES) net.link("me", n);
  await advance();
  return {
    real,
    model: {
      edits: { me: 0, p1: 0, p2: 0 },
      up: { p1: true, p2: true },
      owed: { p1: false, p2: false },
      downs: 0,
      losses: 0,
    },
  };
}

/** The network becomes quiet and connected: every peer up, every owed resync done, frames delivered. */
async function quietPhase(real: Real, model: Model): Promise<void> {
  for (const n of REMOTES) {
    if (!model.up[n]) {
      real.net.link("me", n);
      model.up[n] = true;
    }
    if (model.owed[n]) {
      resync(real, n);
      model.owed[n] = false;
    }
  }
  let quiet = 0;
  for (let round = 0; round < 50 && quiet < 2; round++) {
    await advance(1000);
    let moved = false;
    for (const [a, b] of directed) while (real.net.deliver(a, b)) moved = true;
    quiet = moved || real.net.total() > 0 ? 0 : quiet + 1;
  }
  await advance(1000);
  expect(real.net.total(), "frames still held after the quiet phase").toBe(0);
}

it("W33 sync indicator follows SyncIndicator.tla: PeerHonest, GlobalHonest, EventuallySynced", async () => {
  const seed = Number(process.env.SI1_SEED) || Date.now() % 2 ** 31;
  console.log(`W33 sync indicator model test: seed ${seed} (SI1_SEED=${seed} replays it)`);
  await fc.assert(
    fc.asyncProperty(commands, async (cmds) => {
      vi.useFakeTimers();
      const kit = new Kit();
      try {
        const { real, model } = await setup(kit);
        await fc.asyncModelRun(() => ({ model, real }), cmds);
        await quietPhase(real, model);
        oracle(real);
        // Liveness: nothing connected is left unknown, and the header says the truth.
        expect(truth(real).global, "EventuallySynced").toBe("synced");
      } finally {
        await kit.shutdown();
        vi.useRealTimers();
      }
    }),
    { numRuns: 200, seed, endOnFailure: false },
  );
}, 900_000);
