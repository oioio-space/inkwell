import { next as A } from "@automerge/automerge";
import * as fc from "fast-check";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import PeopleDialog from "../ui/people/PeopleDialog.svelte";
import type { Member } from "../ui/people/roster";
import { cleanup, render } from "../ui/testkit.svelte";
import { workspace } from "../workspace/workspace.svelte";
import { AdmittedSet } from "./scope";
import { SimNet } from "./simnet";
import {
  type GlobalSyncState,
  globalSyncState,
  isSlow,
  MinShowMs,
  type Observed,
  observePeers,
  peerSyncState,
  SlowAfterMs,
  SyncDisplay,
  SyncedHoldMs,
  sendingCount,
} from "./syncIndicator";
import { statusLabel } from "./syncLabel";
import { Kit, until } from "./testkit";

vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Session: { Members: vi.fn(async () => null) },
  Projects: {},
}));

type Doc = { text: string };
const base = A.from<Doc>({ text: "a" });
const ahead = A.change(A.clone(base), (d) => {
  d.text = "ab";
});
const heads = (d: A.Doc<Doc>) => A.getHeads(d);

/** An announced peer for which one document is shared. */
function observed(
  mine: A.Doc<Doc>,
  theirs: readonly string[] | null,
  over: Partial<Observed> = {},
) {
  return {
    announced: true,
    docs: new Map([["d1", { mine, theirs }]]),
    pendingMsgs: 0,
    oldestPendingMs: 0,
    ...over,
  } satisfies Observed;
}

describe("peerSyncState", () => {
  it("is offline, checking, behind, sending or synced, the first that holds", () => {
    expect(peerSyncState(observed(base, heads(base), { announced: false }))).toBe("offline");
    expect(peerSyncState(observed(base, null))).toBe("checking");
    expect(peerSyncState({ ...observed(base, null), docs: new Map() })).toBe("checking");
    expect(peerSyncState(observed(base, heads(ahead)))).toBe("behind");
    expect(peerSyncState(observed(ahead, heads(base)))).toBe("sending");
    expect(peerSyncState(observed(ahead, heads(ahead)))).toBe("synced");
  });
});

it("SI1-1 behind wins over sending", () => {
  const two = {
    ...observed(base, heads(ahead)),
    docs: new Map([
      ["d1", { mine: base, theirs: heads(ahead) }], // behind
      ["d2", { mine: ahead, theirs: heads(base) }], // sending
    ]),
  };
  expect(peerSyncState(two)).toBe("behind");
  expect(globalSyncState(["synced", "sending", "behind", "checking"])).toBe("behind");
  expect(globalSyncState(["synced", "checking", "sending"])).toBe("sending");
  expect(globalSyncState(["synced", "checking"])).toBe("checking");
  expect(globalSyncState(["synced", "offline", "synced"])).toBe("synced");
});

it("SI1-1 no connected peer is never Synchronisé", () => {
  expect(globalSyncState([])).toBe("offline");
  expect(globalSyncState(["offline", "offline"])).toBe("offline");
});

describe("SyncDisplay (D-SI5)", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());
  const make = () => {
    const seen: string[] = [];
    const display = new SyncDisplay((s) => seen.push(s));
    return { display, seen };
  };

  it("SI1-2 Synchronisé only after SyncedHoldMs of true synced", () => {
    expect(SyncedHoldMs).toBe(1000);
    const { display, seen } = make();
    display.update("sending");
    display.update("synced");
    vi.advanceTimersByTime(SyncedHoldMs - 1);
    expect(display.shown).toBe("sending");
    vi.advanceTimersByTime(1);
    expect(display.shown).toBe("synced");
    expect(seen).toEqual(["sending", "synced"]);
  });

  it("SI1-2 a break restarts the hold", () => {
    const { display } = make();
    display.update("sending");
    display.update("synced");
    vi.advanceTimersByTime(600);
    display.update("sending");
    display.update("synced");
    vi.advanceTimersByTime(600);
    expect(display.shown).toBe("sending");
    vi.advanceTimersByTime(400);
    expect(display.shown).toBe("synced");
  });

  it("SI1b DS-3 a label stays at least MinShowMs before another non-synced label", () => {
    expect(MinShowMs).toBe(1000);
    const { display } = make();
    display.update("checking"); // the first label shows at once
    expect(display.shown).toBe("checking");
    vi.advanceTimersByTime(100);
    display.update("sending", 1);
    vi.advanceTimersByTime(400);
    display.update("behind"); // the latest wins
    vi.advanceTimersByTime(MinShowMs - 500 - 1);
    expect(display.shown).toBe("checking");
    vi.advanceTimersByTime(1);
    expect(display.shown).toBe("behind");
    // and the next one waits again
    display.update("sending", 1);
    vi.advanceTimersByTime(MinShowMs - 1);
    expect(display.shown).toBe("behind");
    vi.advanceTimersByTime(1);
    expect(display.shown).toBe("sending");
  });

  it("SI1b DS-4 Envoi en cours is never shown with 0", () => {
    expect(statusLabel("sending", 0, false)).toBe("Envoi en cours");
    expect(statusLabel("sending", 3, false)).toBe("Envoi en cours (3)");
    const { display } = make();
    display.update("sending", 2);
    vi.advanceTimersByTime(10);
    display.update("sending", 0); // n is frozen with its label: it does not shrink
    expect(display.shownN).toBe(2);
    display.update("sending", 3); // but it may grow
    expect(display.shownN).toBe(3);
    // truth back to sending with no one counted: the label has no count
    display.update("synced");
    vi.advanceTimersByTime(MinShowMs);
    display.update("sending", 0);
    expect(display.shown).toBe("sending");
    expect(display.shownN).toBe(0);
  });

  it("SI1-2 Synchronisé leaves at once when it stops being true", () => {
    const { display } = make();
    display.update("synced");
    vi.advanceTimersByTime(SyncedHoldMs);
    expect(display.shown).toBe("synced");
    display.update("sending");
    expect(display.shown).toBe("sending");
    vi.advanceTimersByTime(5 * SyncedHoldMs);
    expect(display.shown).toBe("sending");
  });

  it("stays synced without notifying again while it keeps being true", () => {
    const { display, seen } = make();
    display.update("synced");
    vi.advanceTimersByTime(SyncedHoldMs);
    display.update("synced");
    vi.advanceTimersByTime(SyncedHoldMs);
    expect(seen).toEqual(["synced"]);
  });
});

it("SI1-3 n counts the connected people missing my changes", () => {
  const lacking = (pendingMsgs: number) => observed(ahead, heads(base), { pendingMsgs });
  const people = [
    lacking(40),
    observed(ahead, heads(ahead)), // has everything
    observed(ahead, heads(base), { announced: false }), // offline
    lacking(0),
    observed(ahead, null), // still being checked
  ];
  // people, not the bridge's message count (40 + 0 messages here)
  expect(sendingCount(people)).toBe(2);
  expect(statusLabel("sending", 2, false)).toBe("Envoi en cours (2)");
  expect(statusLabel("behind", 1, false)).toBe("Pas encore à jour");
  expect(statusLabel("checking", 1, false)).toBe("Vérification…");
  expect(statusLabel("synced", 0, false)).toBe("Synchronisé");
});

it("SI1-3 Lien lent after 2 s of oldestPendingMs", () => {
  expect(SlowAfterMs).toBe(2000);
  const at = (ms: number) => [observed(ahead, heads(base), { oldestPendingMs: ms })];
  expect(isSlow(at(2000))).toBe(false);
  expect(isSlow(at(2001))).toBe(true);
  expect(isSlow([observed(ahead, heads(base), { announced: false, oldestPendingMs: 9000 })])).toBe(
    false,
  );
  expect(statusLabel("sending", 1, true)).toBe("Envoi en cours (1) · Lien lent");
});

describe("people popover", () => {
  beforeEach(() => {
    HTMLDialogElement.prototype.showModal = function showModal() {
      this.setAttribute("open", "");
    };
  });
  afterEach(() => {
    cleanup();
    workspace.roster = [];
    workspace.peers = [];
    workspace.peerSync = {};
  });
  const member = (id: string, over: Partial<Member> = {}): Member => ({
    id,
    name: id,
    role: "writer",
    suspended: false,
    self: false,
    devices: [{ peer: `peer-${id}`, suspended: false }],
    ...over,
  });

  it("SI1-3 an offline member is Hors ligne in the popover", () => {
    workspace.roster = [member("me", { self: true }), member("bob")];
    workspace.peerSync = { "peer-bob": "synced" }; // stale: bob is not connected
    const host = render(PeopleDialog, { open: true });
    flushSync();
    const state = host.querySelector('[data-person-row="bob"] .state')?.textContent;
    expect(state).toBe("Hors ligne");
  });

  it("gives a connected person's own state", () => {
    workspace.roster = [member("me", { self: true }), member("bob")];
    workspace.peers = ["peer-bob"];
    workspace.peerSync = { "peer-bob": "behind" };
    const host = render(PeopleDialog, { open: true });
    flushSync();
    expect(host.querySelector('[data-person-row="bob"] .sync')?.textContent).toContain(
      "Pas encore à jour",
    );
  });
});

describe("spec amendment SI1-A", () => {
  it("SI1-A a document outside the admitted scope is not counted", () => {
    const source = {
      announcedPeers: () => ["p"],
      // the second one is the document of a project the peer is not in: it never answers
      heardHeads: () =>
        new Map<string, readonly string[] | null>([
          ["in", heads(ahead)],
          ["out", null],
        ]),
    };
    const seen = observePeers(
      source,
      () => ahead,
      () => undefined,
      (id) => id === "in",
    ).get("p") as Observed;
    expect([...seen.docs.keys()]).toEqual(["in"]);
    expect(peerSyncState(seen)).toBe("synced");
    // nothing in scope yet: nothing to claim
    const none = observePeers(
      source,
      () => ahead,
      () => undefined,
      () => false,
    ).get("p") as Observed;
    expect(peerSyncState(none)).toBe("checking");
  });

  it("SI1-A a scope change clears the heard heads", async () => {
    const net = new SimNet(1);
    const kit = new Kit();
    try {
      const [a, b] = [kit.peer(net, "a"), kit.peer(net, "b")];
      const first = a.repo.create<{ n: number }>({ n: 0 });
      b.repo.import<{ n: number }>(A.save(first.doc()), { docId: first.documentId });
      await new Promise((r) => setTimeout(r, 20));
      net.link("a", "b");
      await until(() => a.adapter.heardHeads("b").get(first.documentId) != null);
      let told = 0;
      a.adapter.onSync(() => told++);
      a.adapter.scopeChanged();
      expect(a.adapter.heardHeads("b").size).toBe(0);
      expect(told).toBeGreaterThan(0);
    } finally {
      await kit.shutdown();
    }
  });

  it("SI1-A a local edit makes a synced peer sending", async () => {
    const net = new SimNet(2);
    const kit = new Kit();
    try {
      const [a, b] = [kit.peer(net, "a"), kit.peer(net, "b")];
      const first = a.repo.create<{ n: number }>({ n: 0 });
      b.repo.import<{ n: number }>(A.save(first.doc()), { docId: first.documentId });
      const admitted = new AdmittedSet();
      admitted.setRoot(first.url);
      workspace.attach(a.adapter, a.repo, admitted);
      await new Promise((r) => setTimeout(r, 20));
      net.link("a", "b");
      await until(() => workspace.peerSync.b === "synced", 5000, "never synced");
      first.change((d: { n: number }) => {
        d.n = 1;
      });
      // at once, with no network event in between: the heads of the document moved
      expect(workspace.peerSync.b).toBe("sending");
      await until(() => workspace.peerSync.b === "synced", 5000, "never synced again");
    } finally {
      workspace.peers = [];
      workspace.peerSync = {};
      await kit.shutdown();
    }
  });
});

describe("SI1b display rule", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());
  const states: GlobalSyncState[] = ["offline", "checking", "behind", "sending", "synced"];
  const step = fc.record({
    dt: fc.integer({ min: 0, max: 1800 }),
    truth: fc.constantFrom(...states),
    n: fc.integer({ min: 0, max: 3 }),
  });

  it("SI1b DS-1..DS-5 hold for every timed sequence of true states", () => {
    fc.assert(
      fc.property(fc.array(step, { minLength: 1, maxLength: 40 }), (steps) => {
        vi.setSystemTime(0);
        const log: { at: number; label: GlobalSyncState; n: number }[] = [];
        const display = new SyncDisplay((label, n) => log.push({ at: Date.now(), label, n }));
        // The true state, as a timeline: when it last stopped being synced.
        let truth: GlobalSyncState = "offline";
        const notSyncedAt: number[] = [];
        for (const e of steps) {
          vi.advanceTimersByTime(e.dt);
          const now = Date.now();
          if (e.truth !== "synced") {
            notSyncedAt.push(now);
          }
          truth = e.truth;
          display.update(e.truth, e.n);
          // DS-2: leaving synced is immediate
          if (e.truth !== "synced") expect(display.shown).not.toBe("synced");
        }
        // DS-1: every « synced » shown follows SyncedHoldMs of true synced
        for (const entry of log.filter((l) => l.label === "synced")) {
          expect(
            notSyncedAt.every((t) => t <= entry.at - SyncedHoldMs || t >= entry.at),
            "DS-1",
          ).toBe(true);
          expect(entry.at - SyncedHoldMs, "DS-1 start").toBeGreaterThanOrEqual(
            notSyncedAt.filter((t) => t < entry.at).at(-1) ?? Number.NEGATIVE_INFINITY,
          );
        }
        // DS-3 / DS-4: a non-synced label stays MinShowMs before any other label, except
        // that the label may grow its n; n only changes with the label or by growing.
        let labelSince = Number.NEGATIVE_INFINITY;
        const all = [{ at: labelSince, label: "offline" as GlobalSyncState, n: 0 }, ...log];
        for (let i = 1; i < all.length; i++) {
          const [prev, cur] = [all[i - 1] as (typeof log)[0], all[i] as (typeof log)[0]];
          if (cur.label === prev.label) {
            expect(cur.n, "DS-4 n only grows").toBeGreaterThan(prev.n);
          } else {
            if (prev.label !== "synced")
              expect(cur.at - labelSince, "DS-3").toBeGreaterThanOrEqual(MinShowMs);
            labelSince = cur.at;
          }
        }
        // DS-5: a constant truth is followed within MinShowMs + SyncedHoldMs
        vi.advanceTimersByTime(MinShowMs + SyncedHoldMs);
        expect(display.shown, "DS-5").toBe(truth);
      }),
      { numRuns: 600 },
    );
  });
});
