// CL1 (docs/plans/2026-11-15-cl1-claim-on-local-edits.md): the history claim arms on this device's
// own edits only. Each device is a real HistoryState on a real DocHandle of its own Repo; the wire
// between them is the bytes of each change, delivered after a delay through DocHandle.update, the
// call automerge-repo's synchronizer makes for a change received from a peer. Time is fake. A person's
// edit is written as the editor writes it (editor/amSync.ts, store/merge.ts): DocHandle.update around
// A.changeAt; `editByChange` is the other way in, DocHandle.change. The document is created by a device
// that is none of the tested ones, unless a test asks for its creator: the creation is a write of the
// creator's actor, which owes a claim at the opening (C-2 and C-5 as precised on 2026-11-16).
import { next as A } from "@automerge/automerge";
import { type DocHandle, Repo } from "@automerge/automerge-repo";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { type HDoc, parseAction } from "../../history/types";
import { withMutant } from "../../mutant";
import { workspace } from "../../workspace/workspace.svelte";
import { history } from "./history.svelte";

vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: { Members: async () => [] },
}));

/** The pause of history.svelte.ts (CLAIM_PAUSE_MS), restated: the rule C-3 is about this number. */
const PAUSE = 2_000;

type History = typeof history;
interface Module {
  history: History;
}

interface Device {
  handle: DocHandle<HDoc>;
  actor: string;
  /** When this device wrote each of its claims, in ms since the start of the test. */
  claims: number[];
  /** How many changes this device wrote under its own actor, of any kind. */
  written: number;
  /** When this device put a change on the wire, to anyone. */
  sent: number[];
  history?: History;
  /** A person types one character here: `update` around `changeAt`, as the editor writes. */
  edit(): void;
  /** The same through `DocHandle.change` (the replace bar, the versions). */
  editByChange(): void;
}

let t0 = 0;
const now = () => Date.now() - t0;
/** How long a change takes to reach another device. */
const net = { latency: 50 };
const attached: History[] = [];
/**
 * True while a test writes through DocHandle.change. Nothing reads it but the mutant of C-1 that stands
 * for the rejected implementation (a flag set around `change`): the plan's rule must not need it.
 */
const viaChange = { now: false };
(globalThis as { __cl1ViaChange?: () => boolean }).__cl1ViaChange = () => viaChange.now;

/**
 * `count` devices on one document, every one wired to every other; the first `withHistory` of them
 * run the history module `mod` (the others are devices whose history panel plays no part). With
 * `created`, the first device is the one that created the document; otherwise all of them received it.
 */
function devices(mod: Module, count: number, withHistory = count, created = false): Device[] {
  const first = new Repo({}).create<HDoc>({ text: "", meta: { versions: [], variants: [] } });
  const handles = created ? [first] : [];
  while (handles.length < count) handles.push(new Repo({}).import<HDoc>(A.save(first.doc())));
  const out = handles.map((handle): Device => {
    const actor = A.getActorId(handle.doc());
    const d: Device = {
      handle,
      actor,
      claims: [],
      written: 0,
      sent: [],
      edit: () =>
        handle.update(
          (doc) => A.changeAt(doc, A.getHeads(doc), (x) => A.splice(x, ["text"], 0, 0, "x")).newDoc,
        ),
      editByChange: () => {
        viaChange.now = true;
        try {
          handle.change((doc) => A.splice(doc, ["text"], 0, 0, "x"));
        } finally {
          viaChange.now = false;
        }
      },
    };
    handle.on("change", ({ patchInfo }) => {
      const since = A.getHeads(patchInfo.before);
      for (const m of A.getChangesMetaSince(patchInfo.after, since)) {
        if (m.actor !== actor) continue;
        d.written++;
        if (parseAction(m.message)?.kind === "claim") d.claims.push(now());
      }
      const bytes = A.saveSince(patchInfo.after, since);
      d.sent.push(now());
      for (const other of handles) {
        if (other === handle) continue;
        setTimeout(() => other.update((doc) => A.loadIncremental(doc, bytes)), net.latency);
      }
    });
    return d;
  });
  const HistoryState = mod.history.constructor as new () => History;
  for (const d of out.slice(0, withHistory)) {
    const h = new HistoryState();
    h.attach(d.handle);
    h.useInline();
    attached.push(h);
    d.history = h;
  }
  return out;
}

/** The claims in `d`'s document written under `actor`. */
const claimsOf = (d: Device, actor: string) =>
  A.getChangesMetaSince(d.handle.doc(), []).filter(
    (m) => m.actor === actor && parseAction(m.message)?.kind === "claim",
  ).length;

/** Ends one part of a test: the devices are left, the clock starts again. */
function again(): void {
  for (const h of attached.splice(0)) h.detach();
  t0 = Date.now();
  net.latency = 50;
  workspace.myId = "me";
}

const at = (ms: number) => vi.advanceTimersByTimeAsync(ms - now());

// The oracles: each one throws with the text of the rule it holds, on the original module and on a mutant.

/** C-1: a change from another device, arriving just before the timer fires, does not postpone the claim. */
async function c1(mod: Module, write: "edit" | "editByChange" = "edit"): Promise<void> {
  const [me, other] = devices(mod, 2, 1) as [Device, Device];
  me[write](); // t = 0
  await at(1_850);
  other.edit(); // reaches this device at 1 900, 100 ms before its timer
  await at(PAUSE);
  expect(me.claims, "C-1 a change from another device re-armed the claim timer").toEqual([PAUSE]);
}

/** C-1, the other half: every way this device writes arms the timer, `update` around `changeAt` as `change`. */
async function c1ways(mod: Module): Promise<void> {
  const [byChange] = devices(mod, 1) as [Device];
  byChange.editByChange();
  await at(PAUSE);
  expect(byChange.claims, "C-1 an edit written through change did not arm the claim").toEqual([
    PAUSE,
  ]);
  again();
  const [byUpdate] = devices(mod, 1) as [Device];
  byUpdate.edit();
  await at(PAUSE);
  expect(
    byUpdate.claims,
    "C-1 an edit written through update and changeAt did not arm the claim",
  ).toEqual([PAUSE]);
}

/**
 * C-2 and C-5 at the opening: the device that created the document is named once, a pause after it
 * opens it, without an edit (the creation is a change of its actor, and an entry of the history);
 * the device that received the document and wrote nothing claims nothing.
 */
async function creator(mod: Module): Promise<void> {
  const [made, received] = devices(mod, 2, 2, true) as [Device, Device];
  await at(PAUSE);
  expect(made.claims, "C-5 the creator of the document was not named at its opening").toEqual([
    PAUSE,
  ]);
  await at(10_000);
  expect(made.claims, "C-5 the creator is named once").toEqual([PAUSE]);
  expect(received.written, "C-2 a device that wrote nothing claimed at the opening").toBe(0);
  expect(claimsOf(received, made.actor), "C-5 the creator's claim reached the other device").toBe(
    1,
  );
}

/** C-2: no edit since the last claim (or ever), no claim, whatever arrives. The device received the document. */
async function c2(mod: Module): Promise<void> {
  const [me, other] = devices(mod, 2, 1) as [Device, Device];
  other.edit();
  await at(500);
  other.edit();
  await at(10_000);
  expect(me.written, "C-2 a device that never wrote claimed").toBe(0);
  me.edit();
  await at(10_000 + PAUSE);
  expect(me.claims, "C-2 the claim after the first edit").toEqual([10_000 + PAUSE]);
  await at(15_000);
  other.edit();
  // One second after a peer's change everything it started has run (delivery 50 ms, the list's refresh
  // 300 ms): a timer still waiting is a claim timer that the change armed.
  await at(16_000);
  expect(vi.getTimerCount(), "C-2 a change from another device armed a timer").toBe(0);
  await at(30_000);
  expect(me.written, "C-2 a second claim without an edit").toBe(2); // the edit and its one claim
}

/** C-3: the claim lands CLAIM_PAUSE_MS after the last local edit, and never while the person types. */
async function c3(mod: Module): Promise<void> {
  const [me, other] = devices(mod, 2, 1) as [Device, Device];
  me.edit(); // t = 0
  await at(400);
  other.edit();
  await at(800);
  me.edit();
  await at(1_200);
  other.edit();
  await at(1_600);
  me.edit(); // the last local edit
  await at(1_600 + PAUSE - 1);
  expect(me.claims, "C-3 no claim while the person types").toEqual([]);
  await at(1_600 + PAUSE);
  expect(me.claims, "C-3 the claim lands CLAIM_PAUSE_MS after the last local edit").toEqual([
    1_600 + PAUSE,
  ]);
}

/**
 * C-4, the shape of netsim scenario 8 (frontend/e2e/netsim.spec.ts): three devices type in turn, the
 * quiescence baseline is read at least 3.8 s after the last keystroke (the typist's last sleep, at
 * least 0.8 s, then the 3 s settle), and the window lasts 30 s. Nothing may be put on the wire in it.
 */
async function c4(mod: Module): Promise<void> {
  // As in scenario 8, the first device created the document: its claim is owed from the opening.
  const all = devices(mod, 3, 3, true);
  for (let round = 0; round < 3; round++)
    for (const [i, d] of all.entries()) {
      await at(round * 900 + i * 300);
      d.edit();
    }
  const last = 2 * 900 + 2 * 300;
  // The race of the real network: a claim reaches a device just before that device's own timer.
  net.latency = 20;
  const baseline = last + 3_800;
  await at(baseline + 30_000);
  const late = all.flatMap((d, i) => d.sent.filter((t) => t >= baseline).map((t) => `${i}@${t}`));
  expect(late, "C-4 claim traffic in the quiescence window").toEqual([]);
  expect(
    all.map((d) => d.claims),
    "C-4 every device that wrote has claimed once before the baseline",
  ).toEqual([[1_800 + PAUSE], [2_100 + PAUSE], [2_400 + PAUSE]]);
}

/** C-5: a claim that is owed is not lost when the document or the page is left. */
async function c5left(mod: Module): Promise<void> {
  const [closing] = devices(mod, 1) as [Device];
  closing.edit();
  await at(500);
  closing.history?.detach(); // another document is opened before the pause ends
  expect(closing.claims, "C-5 the claim owed was dropped when the document was left").toEqual([
    500,
  ]);
  again();
  const [hiding] = devices(mod, 1) as [Device];
  hiding.edit();
  await at(500);
  window.dispatchEvent(new Event("pagehide")); // the window closes: the next session has another actor
  expect(hiding.claims, "C-5 the claim owed was dropped when the page was hidden").toEqual([500]);
  await at(10_000);
  expect(hiding.claims, "C-5 one claim, not two, after the page was hidden").toEqual([500]);
}

/** C-5: a claim that cannot be written at the pause (no identity yet) is retried, with no change to wake it. */
async function c5retry(mod: Module): Promise<void> {
  const [me] = devices(mod, 1) as [Device];
  workspace.myId = "";
  me.edit();
  await at(PAUSE);
  expect(me.claims, "C-5 no claim without an identity").toEqual([]);
  workspace.myId = "me";
  await at(2 * PAUSE);
  expect(me.claims, "C-5 the claim owed was not retried").toEqual([2 * PAUSE]);
}

/** C-5: a claim that could not be written when the document was left is found again at its next opening. */
async function c5reopen(mod: Module): Promise<void> {
  const [me] = devices(mod, 1) as [Device];
  me.edit();
  workspace.myId = "";
  await at(500);
  me.history?.detach(); // cannot write now: nothing is written
  expect(me.claims, "C-5 no claim without an identity").toEqual([]);
  workspace.myId = "me";
  await at(1_000);
  me.history?.attach(me.handle);
  me.history?.useInline();
  await at(1_000 + PAUSE + 1_000); // the analysis of the opening, then the pause
  expect(me.claims.length, "C-5 the claim owed was not found again at the opening").toBe(1);
}

/**
 * C-6: the editor's recovery hands the handle a clone (editor/amSync.ts:242), and a clone writes under a
 * new actor (pinned by the first assertion). The device then claims for that actor too.
 */
async function c6(mod: Module): Promise<void> {
  const [me] = devices(mod, 1) as [Device];
  me.edit();
  await at(PAUSE);
  expect(claimsOf(me, me.actor), "C-6 the first actor is claimed").toBe(1);
  me.handle.update((doc) => A.clone(doc));
  const second = A.getActorId(me.handle.doc());
  expect(second, "C-6 pin: A.clone keeps the actor (the rule may then cache it)").not.toBe(
    me.actor,
  );
  await at(5_000);
  me.edit();
  await at(5_000 + PAUSE);
  expect(claimsOf(me, second), "C-6 the actor after a clone was not claimed").toBe(1);
}

/** The fix as the plan writes it; a mutant that needs it cannot be built before it exists. */
const ARMS_ON_OWN = "if (this.#wroteHere(patchInfo)) {";
/** The planned calls of C-5 (plan, « Planned shape »): the owed claim settled when leaving, and its retry. */
const SETTLE_AT_DETACH = 'this.#settle("detach");';
const SETTLE_WHEN_HIDDEN = 'this.#settle("hidden");';
const RETRY = "this.#armClaim(); // owed and not written: retried";
const OWED_AT_ATTACH = "this.#owedFrom(result);";
const ACTOR_NOW = "const actor = A.getActorId(patchInfo.after);";
/** What the opening reads: whether this device's actor wrote in the document. */
const WROTE_BEFORE = "const wrote = result.sessions.some((s) => s.actor === actor);";

beforeEach(() => {
  vi.useFakeTimers();
  t0 = Date.now();
  net.latency = 50;
  vi.spyOn(Object.getPrototypeOf(workspace), "access", "get").mockReturnValue(undefined);
  vi.spyOn(Object.getPrototypeOf(workspace), "rightsKnown", "get").mockReturnValue(true);
  workspace.myId = "me";
});
afterEach(() => {
  for (const h of attached.splice(0)) h.detach();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("CL1 the history claim arms on local edits only", () => {
  it("CL1 C-1 a change from another device does not re-arm the claim timer", () => c1({ history }));

  it("CL1 C-2 a device that made no edit since its last claim sends no claim", () =>
    c2({ history }));

  it("CL1 C-3 the claim lands CLAIM_PAUSE_MS after the last local edit", () => c3({ history }));

  it("CL1 C-4 three devices: no claim traffic in the quiescence window of scenario 8", () =>
    c4({ history }));

  it("CL1 C-1 every way this device writes arms the claim", () => c1ways({ history }));

  it("CL1 C-5 a claim owed when the document or the page is left is written", () =>
    c5left({ history }));

  it("CL1 C-5 a claim that could not be written is retried", () => c5retry({ history }));

  it("CL1 C-5 a claim owed is found again when the document is opened", () =>
    c5reopen({ history }));

  it("CL1 C-6 the actor a clone gives this device is claimed too", () => c6({ history }));

  it("CL1 C-5 the device that created a document is named at its opening", () =>
    creator({ history }));

  it("CL1 mutant C-1 a flag around change is caught", async () => {
    // The rejected implementation: « mine » is whatever is written while DocHandle.change runs.
    await withMutant<Module, void>(
      "ui/history/history.svelte.ts",
      "cl1mutant-c1flag",
      ARMS_ON_OWN,
      "if ((globalThis as unknown as { __cl1ViaChange: () => boolean }).__cl1ViaChange()) {",
      async (m) => {
        // A peer's change still does not arm: C-1's first oracle passes when the local edit is
        // written the way this mutant sees, through DocHandle.change.
        await c1(m, "editByChange").then(again);
        await expect(c1ways(m)).rejects.toThrow(
          /C-1 an edit written through update and changeAt did not arm the claim/,
        );
      },
    );
  });

  for (const [title, tag, anchor, replacement, oracle, text] of [
    [
      "dropping the owed claim at detach",
      "c5detach",
      SETTLE_AT_DETACH,
      "",
      c5left,
      /C-5 the claim owed was dropped when the document was left/,
    ],
    [
      "dropping the owed claim when hidden",
      "c5hidden",
      SETTLE_WHEN_HIDDEN,
      "",
      c5left,
      /C-5 the claim owed was dropped when the page was hidden/,
    ],
    ["no retry", "c5retry", RETRY, "", c5retry, /C-5 the claim owed was not retried/],
    [
      "nothing owed at the opening",
      "c5attach",
      OWED_AT_ATTACH,
      "",
      c5reopen,
      /C-5 the claim owed was not found again at the opening/,
    ],
    [
      "the creator not named at the opening",
      "c5creator",
      OWED_AT_ATTACH,
      "",
      creator,
      /C-5 the creator of the document was not named at its opening/,
    ],
    [
      "C-2 owing a claim at every opening",
      "c2owing",
      WROTE_BEFORE,
      "const wrote = true;",
      creator,
      /C-2 a device that wrote nothing claimed at the opening/,
    ],
    [
      "the actor cached at the opening",
      "c6cached",
      ACTOR_NOW,
      "const actor = this.myActor;",
      c6,
      /C-6 the actor after a clone was not claimed/,
    ],
  ] as const) {
    it(`CL1 mutant ${title} is caught`, async () => {
      await withMutant<Module, void>(
        "ui/history/history.svelte.ts",
        `cl1mutant-${tag}`,
        anchor,
        replacement,
        (m) => expect(oracle(m)).rejects.toThrow(text),
      );
    });
  }

  it("CL1 mutant C-1 arming on every change is caught", async () => {
    await withMutant<Module, void>(
      "ui/history/history.svelte.ts",
      "cl1mutant-c1any",
      ARMS_ON_OWN,
      "if (true) {",
      (m) =>
        expect(c1(m)).rejects.toThrow(/C-1 a change from another device re-armed the claim timer/),
    );
  });

  it("CL1 mutant C-2 arming at the opening is caught", async () => {
    await withMutant<Module, void>(
      "ui/history/history.svelte.ts",
      "cl1mutant-c2attach",
      "void this.loadMembers();\n    void this.refresh();\n  }\n\n  detach(): void {",
      "this.#claimTimer = setTimeout(() => this.#claim(true), CLAIM_PAUSE_MS);\n    void this.loadMembers();\n    void this.refresh();\n  }\n\n  detach(): void {",
      async (m) => {
        await c1(m); // the mutant leaves C-1 alone: it is C-2 that sees it
        again();
        await expect(c2(m)).rejects.toThrow(/C-2 a device that never wrote claimed/);
      },
    );
  });

  it("CL1 mutant C-3 a longer pause is caught", async () => {
    await withMutant<Module, void>(
      "ui/history/history.svelte.ts",
      "cl1mutant-c3pause",
      "const CLAIM_PAUSE_MS = 2_000;",
      "const CLAIM_PAUSE_MS = 2_500;",
      (m) =>
        expect(c3(m)).rejects.toThrow(
          /C-3 the claim lands CLAIM_PAUSE_MS after the last local edit/,
        ),
    );
  });

  it("CL1 mutant C-4 arming on every change is caught", async () => {
    await withMutant<Module, void>(
      "ui/history/history.svelte.ts",
      "cl1mutant-c4any",
      ARMS_ON_OWN,
      "if (true) {",
      (m) => expect(c4(m)).rejects.toThrow(/C-4 claim traffic in the quiescence window/),
    );
  });
});
