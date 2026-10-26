import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { PILOT_TTL_MS, Pilot } from "./pilot";
import type { ViewState, WireMessage } from "./wire";

const show = (slide: number, step = 0): ViewState => ({ slide, step, screen: "show" });

/** A few devices on a network the test can cut and heal; messages arrive in order, at once. */
class Net {
  now = 0;
  readonly roles = new Map<string, "reader" | "writer" | "admin">();
  readonly devices = new Map<string, Pilot>();
  readonly cut = new Set<string>();
  readonly log: { from: string; m: WireMessage }[] = [];

  add(id: string, role: "reader" | "writer" | "admin" = "writer"): Pilot {
    this.roles.set(id, role);
    const p = new Pilot({
      me: id,
      now: () => this.now,
      canPilot: (peer) => this.roles.get(peer) !== undefined && this.roles.get(peer) !== "reader",
      send: (m) => {
        this.log.push({ from: id, m });
        for (const [other, q] of this.devices) {
          if (other === id || this.cut.has(`${id}|${other}`) || this.cut.has(`${other}|${id}`))
            continue;
          q.receive(id, structuredClone(m));
        }
      },
    });
    this.devices.set(id, p);
    return p;
  }

  partition(a: string, b: string): void {
    this.cut.add(`${a}|${b}`);
  }

  heal(): void {
    this.cut.clear();
  }

  advance(ms: number): void {
    this.now += ms;
    for (const p of this.devices.values()) p.tick();
  }
}

describe("Pilot", () => {
  it("PRS-10 the pilot's slide reaches the followers, and only the pilot's", () => {
    const net = new Net();
    const [a, b, c] = [net.add("a"), net.add("b"), net.add("c")] as [Pilot, Pilot, Pilot];
    expect(a.take()).toBe(true);
    a.publish(show(3, 1));
    expect(b.view).toEqual(show(3, 1));
    expect(c.view).toEqual(show(3, 1));
    expect(b.pilot).toBe("a");
    b.publish(show(9)); // not the pilot: nothing goes out
    expect(a.view).toEqual(show(3, 1));
    expect(c.view).toEqual(show(3, 1));
  });

  it("PRS-35 two who take the controls at once end up with one pilot, the same everywhere", () => {
    const net = new Net();
    const [a, b, c] = [net.add("a"), net.add("b"), net.add("c")] as [Pilot, Pilot, Pilot];
    net.partition("a", "b"); // neither hears the other take
    a.take();
    b.take();
    a.publish(show(2));
    b.publish(show(7));
    net.heal();
    a.beat();
    b.beat();
    for (const p of [a, b, c]) expect(p.pilot).toBe("a"); // same epoch: the lower id wins
    expect(a.isPilot).toBe(true);
    expect(b.isPilot).toBe(false);
    expect(c.view).toEqual(show(2));
    expect(b.view).toEqual(show(2));
  });

  it("PRS-35 take is refused while someone else pilots; asking, then being granted, moves the controls", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    a.publish(show(4));
    expect(b.canTake).toBe(false);
    expect(b.take()).toBe(false);
    expect(b.ask()).toBe(true);
    expect(a.requests).toEqual(["b"]);
    expect(a.grant("b")).toBe(true);
    expect(a.isPilot).toBe(false);
    expect(b.isPilot).toBe(true);
    expect(b.asking).toBe(false);
    b.publish(show(5));
    expect(a.view).toEqual(show(5));
    a.publish(show(1)); // the old pilot is a follower now
    expect(b.view).toEqual(show(5));
  });

  it("PRS-35 a refused request says so, and the pilot keeps the controls", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    b.ask();
    a.deny("b");
    expect(a.requests).toEqual([]);
    expect(b.denied).toBe(true);
    expect(b.asking).toBe(false);
    expect(a.isPilot).toBe(true);
  });

  it("PRS-38 a reader follows but can neither take nor be given the controls", () => {
    const net = new Net();
    const [a, r] = [net.add("a"), net.add("r", "reader")] as [Pilot, Pilot];
    a.take();
    a.publish(show(2));
    expect(r.view).toEqual(show(2));
    expect(r.take()).toBe(false);
    expect(r.ask()).toBe(false);
    expect(a.grant("r")).toBe(false);
    // A reader that forges the messages is not believed by anyone.
    r.receive("x", { t: "ask" });
    const forged = net.add("f", "reader");
    forged.receive("a", { t: "ask" });
    a.receive("f", { t: "ask" });
    expect(a.requests).toEqual([]);
    a.receive("f", { t: "state", epoch: 99, seq: 1, slide: 8, step: 0, screen: "show" });
    a.receive("f", { t: "take", epoch: 99 });
    expect(a.isPilot).toBe(true);
    expect(a.view).toEqual(show(2));
  });

  it("PRS-37 the pilot goes silent: the last slide stays, marked as disconnected, and resumes on return", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    a.publish(show(6, 2));
    expect(b.connected).toBe(true);
    net.advance(PILOT_TTL_MS - 1);
    expect(b.connected).toBe(true);
    net.advance(2);
    expect(b.connected).toBe(false);
    expect(b.view).toEqual(show(6, 2));
    a.beat(); // back
    expect(b.connected).toBe(true);
    expect(b.pilot).toBe("a");
  });

  it("PRS-37 a link that goes down marks the pilot at once", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    a.publish(show(2));
    b.peerGone("a");
    expect(b.connected).toBe(false);
    expect(b.view).toEqual(show(2));
  });

  it("PRS-37 someone else can take the controls from a pilot who is gone, and the old one yields on return", () => {
    const net = new Net();
    const [a, b, c] = [net.add("a"), net.add("b"), net.add("c")] as [Pilot, Pilot, Pilot];
    a.take();
    a.publish(show(3));
    net.partition("a", "b");
    net.partition("a", "c");
    net.advance(PILOT_TTL_MS + 1);
    expect(b.canTake).toBe(true);
    expect(b.take()).toBe(true);
    expect(c.pilot).toBe("b");
    expect(c.view).toEqual(show(3)); // starts where the last pilot stopped
    net.heal();
    a.beat();
    b.beat();
    expect(a.isPilot).toBe(false);
    expect(a.pilot).toBe("b");
  });

  it("a pilot who ends the presentation leaves the last slide and frees the controls", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    a.publish(show(5));
    a.release();
    expect(b.pilot).toBeUndefined();
    expect(b.ended).toBe(true);
    expect(b.view).toEqual(show(5));
    expect(b.take()).toBe(true);
  });

  it("the laser follows the pilot, and only while the pilot is the one in charge", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    a.publishLaser({ x: 0.5, y: 0.25 });
    expect(b.laser).toEqual({ x: 0.5, y: 0.25 });
    a.publishLaser(null);
    expect(b.laser).toBeNull();
    b.receive("c", { t: "laser", epoch: 1, x: 0.1, y: 0.1 }); // not the pilot
    expect(b.laser).toBeNull();
  });

  it("a late arrival catches up from the next beat", () => {
    const net = new Net();
    const a = net.add("a");
    a.take();
    a.publish(show(8, 1));
    const late = net.add("late");
    expect(late.view).toBeUndefined();
    a.beat();
    expect(late.view).toEqual(show(8, 1));
    expect(late.pilot).toBe("a");
  });

  it("a device that arrives asks who pilots, and the pilot answers at once", () => {
    const net = new Net();
    const a = net.add("a");
    a.take();
    a.publish(show(4, 1));
    const late = net.add("late");
    late.hello();
    expect(late.view).toEqual(show(4, 1));
    expect(late.pilot).toBe("a");
  });

  it("an old state never moves the slide back", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    a.publish(show(1));
    a.publish(show(2));
    b.receive("a", { t: "state", epoch: 1, seq: 0, slide: 1, step: 0, screen: "show" });
    expect(b.view).toEqual(show(2));
  });

  it("a request nobody answers lapses", () => {
    const net = new Net();
    const [a, b] = [net.add("a"), net.add("b")] as [Pilot, Pilot];
    a.take();
    b.ask();
    expect(b.asking).toBe(true);
    net.advance(21_000);
    expect(b.asking).toBe(false);
  });

  it("PRS-35 whatever the order of takes and partitions, devices that hear each other agree on the pilot", () => {
    fc.assert(
      fc.property(
        fc.array(
          fc.tuple(
            fc.constantFrom("a", "b", "c"),
            fc.constantFrom("take", "cut", "heal", "beat", "wait"),
          ),
          { maxLength: 30 },
        ),
        (steps) => {
          const net = new Net();
          const all = ["a", "b", "c"].map((id) => net.add(id));
          for (const [who, op] of steps) {
            const p = net.devices.get(who) as Pilot;
            if (op === "take") {
              if (p.take()) p.publish(p.view ?? show(1)); // a presenter shows its slide at once
            } else if (op === "cut") net.partition(who, who === "a" ? "b" : "a");
            else if (op === "heal") net.heal();
            else if (op === "beat") p.beat();
            else net.advance(PILOT_TTL_MS + 1);
          }
          net.heal();
          for (const p of all) p.beat();
          for (const p of all) p.beat();
          // Everyone who is not a pilot agrees on who is; at most one thinks it is.
          expect(all.filter((p) => p.isPilot).length).toBeLessThanOrEqual(1);
          const named = new Set(all.map((p) => p.pilot).filter((x) => x !== undefined));
          expect(named.size).toBeLessThanOrEqual(1);
        },
      ),
      { numRuns: 200 },
    );
  });
});
