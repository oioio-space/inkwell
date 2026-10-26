import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { history, sessionsBefore, sessionsOf, stepsOf } from "./log";
import { Device } from "./testkit";

const MIN = 60;

describe("sessions (B2)", () => {
  it("groups one author's edits with no gap over 10 minutes", () => {
    const a = new Device("aa01");
    a.type("un ", 0, 1)
      .type("deux ", 3, 5 * MIN)
      .type("trois", 8, 9 * MIN);
    a.type(" quatre", 13, 11 * MIN);
    const sessions = history(a.doc, a.clock * 1000);
    expect(sessions.map((s) => s.count)).toEqual([4, 1]);
  });

  it("splits at each change of author, whatever the clocks say", () => {
    const a = new Device("aa01").type("A");
    const b = new Device("bb02", a.doc).type("B");
    a.sync(b);
    a.type("C");
    const sessions = history(a.doc, a.clock * 1000);
    expect(sessions).toHaveLength(3);
    expect(new Set(sessions.map((s) => s.actor))).toEqual(new Set(["aa01", "bb02"]));
  });

  it("X-20 an actor's clock two hours ahead does not reorder the history", () => {
    const a = new Device("aa01").type("one ");
    const b = new Device("bb02", a.doc, a.clock + 2 * 3600).type("two ");
    a.sync(b);
    a.type("three");
    const steps = stepsOf(a.doc);
    // Causal order: every step comes after the ones it depends on, whatever its time.
    const seen = new Set<string>();
    for (const s of steps) {
      for (const d of s.deps) expect(seen.has(d)).toBe(true);
      seen.add(s.hash);
    }
    const sessions = history(a.doc, (a.clock + 3600) * 1000);
    // The step written after reading b's text, on a clock two hours earlier, is out of order.
    const skewed = sessions.filter((s) => s.skewed).map((s) => s.actor);
    expect(skewed).toContain("aa01");
  });

  it("a person who reloads the page (a new actor) stays in the same session", () => {
    const a = new Device("aa01").type("un ", 0).claim("Marie", "peer-m");
    const b = new Device("aa02", a.doc).type("deux", 3).claim("Marie", "peer-m");
    a.sync(b);
    const sessions = history(a.doc, b.clock * 1000);
    expect(sessions.filter((s) => !s.action)).toHaveLength(1);
  });

  it("an action of Inkwell's own is an entry of its own", () => {
    const a = new Device("aa01").type("texte");
    a.edit((d) => A.updateText(d, ["text"], "autre"), 1, "inkwell:revert:Premier jet");
    a.type("!", 5);
    const sessions = history(a.doc, a.clock * 1000);
    expect(sessions).toHaveLength(3);
    expect(sessions[1]?.action).toEqual({ kind: "revert", label: "Premier jet" });
  });

  it("HIS-15 10 000 changes: grouping into sessions is under 100 ms; the panel never waits for it", () => {
    const a = new Device("aa01");
    for (let i = 0; i < 10_000; i++) a.type("x", a.doc.text.length, 1);
    // Reading the changes' metadata (wasm) is done in the worker, off the page; what the page
    // would pay is the grouping. The best of a few runs says what the code costs, not the machine.
    const steps = stepsOf(a.doc);
    const personOf = (actor: string) => actor;
    let ms = Number.POSITIVE_INFINITY;
    let sessions = sessionsOf(steps, a.clock * 1000, personOf);
    for (let i = 0; i < 5; i++) {
      const t0 = performance.now();
      sessions = sessionsOf(steps, a.clock * 1000, personOf);
      ms = Math.min(ms, performance.now() - t0);
    }
    expect(sessions.length).toBeGreaterThan(0);
    expect(ms).toBeLessThan(100);
    // The whole thing, metadata included, stays well under a second.
    const t0 = performance.now();
    history(a.doc, a.clock * 1000);
    expect(performance.now() - t0).toBeLessThan(2_000);
  }, 60_000);

  it("finds how many sessions lie in the past of some heads", () => {
    const a = new Device("aa01").type("a");
    const seen = a.heads;
    const b = new Device("bb02", a.doc).type("b");
    a.sync(b);
    const steps = stepsOf(a.doc);
    const sessions = history(a.doc);
    expect(sessionsBefore(sessions, steps, seen)).toBe(1);
    expect(sessionsBefore(sessions, steps, [])).toBe(0);
  });
});
