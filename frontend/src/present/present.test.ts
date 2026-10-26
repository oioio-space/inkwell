import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Link } from "./testkit.svelte";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

const press = (p: { key(e: { key: string }): boolean }, ...keys: string[]) => {
  for (const key of keys) p.key({ key });
};

describe("Presentation on one machine", () => {
  it("PRS-05 the keys move by steps and by slides, and stop at both ends", () => {
    const link = new Link();
    const { presentation: p } = link.device("a");
    p.start("presenter");
    expect(p.isPilot).toBe(true);
    expect(p.pos).toEqual({ slide: 1, step: 0 });
    press(p, "ArrowRight", "ArrowRight");
    expect(p.pos).toEqual({ slide: 2, step: 1 });
    expect(p.page).toBe(3);
    press(p, "PageDown", "PageDown", "PageDown", "PageDown");
    expect(p.pos).toEqual({ slide: 3, step: 0 });
    press(p, "Home");
    expect(p.pos).toEqual({ slide: 1, step: 0 });
    press(p, "ArrowLeft");
    expect(p.pos).toEqual({ slide: 1, step: 0 });
    press(p, "End");
    expect(p.pos).toEqual({ slide: 3, step: 0 });
    press(p, "2", "Enter");
    expect(p.pos).toEqual({ slide: 2, step: 0 });
    p.stop();
  });

  it("PRS-07 says what comes next: the next step, then the next slide, then the end", () => {
    const { presentation: p } = new Link().device("a");
    p.start("presenter");
    press(p, "ArrowRight");
    expect(p.upcoming).toMatchObject({ kind: "step" });
    press(p, "ArrowRight", "ArrowRight");
    expect(p.upcoming).toMatchObject({ kind: "slide" });
    press(p, "End");
    expect(p.upcoming).toEqual({ kind: "end" });
    p.stop();
  });

  it("PRS-08 shows the notes of the slide, whichever step is on", () => {
    const { presentation: p } = new Link().device("a");
    p.start("presenter");
    expect(p.notes).toBe("");
    press(p, "ArrowRight", "ArrowRight");
    expect(p.notes).toBe("Souriez");
    p.stop();
  });

  it("PRS-06 the timer starts at the first advance, and compares with the plan", () => {
    const link = new Link();
    const { presentation: p } = link.device("a");
    p.start("presenter");
    link.now += 60_000;
    expect(p.stopwatch.running).toBe(false); // looking at the first slide does not start it
    press(p, "ArrowRight");
    expect(p.stopwatch.running).toBe(true);
    link.now += 10 * 60_000; // 20 minutes over 3 slides: slide 2 should begin at 6 min 40
    expect(p.pace).toMatchObject({ kind: "behind" });
    p.stopwatch.pause();
    expect(p.stopwatch.elapsedMs()).toBe(10 * 60_000);
    p.stop();
  });

  it("PRS-12 B and W switch the screen to black or white and back; L toggles the laser", () => {
    const { presentation: p } = new Link().device("a");
    p.start("presenter");
    press(p, "b");
    expect(p.screen).toBe("black");
    press(p, "b");
    expect(p.screen).toBe("show");
    press(p, "w");
    expect(p.screen).toBe("white");
    press(p, "b"); // black replaces white
    expect(p.screen).toBe("black");
    press(p, "l");
    expect(p.laserOn).toBe(true);
    p.stop();
  });

  it("PRS-13 Esc leaves and gives the page to put the caret on", () => {
    const { presentation: p } = new Link().device("a");
    p.start("presenter");
    press(p, "ArrowRight", "ArrowRight");
    press(p, "Escape");
    expect(p.mode).toBe("off");
  });

  it("PRS-31 a command from the public window drives the presenter, not a second state", () => {
    const { presentation: p } = new Link().device("a");
    p.start("presenter");
    p.command("next");
    p.command("next");
    expect(p.pos).toEqual({ slide: 2, step: 1 });
    p.command("black");
    expect(p.screen).toBe("black");
    p.command("quit");
    expect(p.mode).toBe("off");
  });

  it("PRS-39 a live edit that changes the number of slides keeps the position", () => {
    const link = new Link();
    const { presentation: p, preview } = link.device("a");
    p.start("presenter");
    press(p, "ArrowRight", "ArrowRight"); // slide 2, step 1
    preview.deck = {
      minutes: 20,
      pages: [...(preview.deck?.pages ?? []), { slide: 4, step: 0, hidden: false, note: "" }],
    };
    preview.pageCount = 6; // a slide was added after: the same numbers still name the same place
    expect(p.pos).toEqual({ slide: 2, step: 1 });
    preview.deck = { minutes: 0, pages: [{ slide: 1, step: 0, hidden: false, note: "" }] };
    preview.pageCount = 1; // a deck much shorter: clamped, not lost
    expect(p.pos).toEqual({ slide: 1, step: 0 });
    p.stop();
  });

  it("asks for the deck while presenting, and stops asking when it ends", () => {
    const { presentation: p, preview } = new Link().device("a");
    p.start("presenter");
    expect(preview.deckWanted).toBe(true);
    p.stop();
    expect(preview.deckWanted).toBe(false);
  });
});

describe("Presentation over the network", () => {
  it("PRS-33 a public machine follows the presenter: slide, step, black screen, laser", () => {
    const link = new Link();
    const b = link.device("b");
    const a = link.device("a");
    a.presentation.start("public");
    b.presentation.start("presenter");
    expect(a.presentation.isPilot).toBe(false);
    press(b.presentation, "ArrowRight", "ArrowRight");
    expect(a.presentation.pos).toEqual({ slide: 2, step: 1 });
    expect(a.presentation.page).toBe(3);
    press(b.presentation, "b");
    expect(a.presentation.screen).toBe("black");
    press(b.presentation, "b", "l");
    b.presentation.moveLaser(0.25, 0.75);
    expect(a.presentation.laser).toEqual({ x: 0.25, y: 0.75 });
    // The public machine's own keys move nothing.
    press(a.presentation, "End");
    expect(a.presentation.pos).toEqual({ slide: 2, step: 1 });
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-35 two presenters: one pilots, the other asks, is granted, and then pilots", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("presenter");
    expect(a.presentation.isPilot).toBe(true);
    expect(b.presentation.isPilot).toBe(false);
    expect(b.presentation.pilotName).toBe("Nom de a");
    press(b.presentation, "ArrowRight"); // not the pilot, not browsing: nothing
    expect(a.presentation.pos).toEqual({ slide: 1, step: 0 });
    expect(b.presentation.canAsk).toBe(true);
    b.presentation.ask();
    expect(a.presentation.requests).toEqual([{ peer: "b", name: "Nom de b" }]);
    a.presentation.grant("b");
    expect(b.presentation.isPilot).toBe(true);
    expect(a.presentation.isPilot).toBe(false);
    press(b.presentation, "ArrowRight");
    expect(a.presentation.pos).toEqual({ slide: 2, step: 0 });
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-35 a refused request leaves the pilot where it was", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("follower");
    b.presentation.ask();
    a.presentation.deny("b");
    expect(b.presentation.denied).toBe(true);
    expect(a.presentation.isPilot).toBe(true);
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-36 browsing freely leaves the pilot's slide; coming back re-aligns", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("follower");
    press(a.presentation, "ArrowRight");
    expect(b.presentation.pos).toEqual({ slide: 2, step: 0 });
    b.presentation.setFollow("free");
    press(b.presentation, "End");
    expect(b.presentation.pos).toEqual({ slide: 3, step: 0 });
    press(a.presentation, "ArrowRight"); // the pilot goes on: b does not move
    expect(a.presentation.pos).toEqual({ slide: 2, step: 1 });
    expect(b.presentation.pos).toEqual({ slide: 3, step: 0 });
    expect(b.presentation.following).toBe(false);
    b.presentation.setFollow("follow");
    expect(b.presentation.pos).toEqual({ slide: 2, step: 1 });
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-36 freely browsing starts from where the pilot is", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("follower");
    press(a.presentation, "ArrowRight", "ArrowRight");
    b.presentation.setFollow("free");
    expect(b.presentation.pos).toEqual({ slide: 2, step: 1 });
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-37 the pilot disconnects: the public view keeps the slide, says so, and resumes", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("public");
    press(a.presentation, "ArrowRight", "ArrowRight");
    expect(b.presentation.away).toBe(false);
    b.presentation.peerGone("a");
    expect(b.presentation.away).toBe(true);
    expect(b.presentation.page).toBe(3); // the last slide stays
    link.now += 2000;
    vi.advanceTimersByTime(2000); // the pilot beats again: it is back
    expect(b.presentation.away).toBe(false);
    expect(b.presentation.page).toBe(3);
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-37 silence for long enough is a disconnection too", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("public");
    link.cut.add("a|b");
    for (let i = 0; i < 8; i++) {
      link.now += 1000;
      vi.advanceTimersByTime(1000);
    }
    expect(b.presentation.away).toBe(true);
    link.cut.clear();
    link.now += 1000;
    vi.advanceTimersByTime(2000);
    expect(b.presentation.away).toBe(false);
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-38 a reader shows and follows but cannot pilot, take, or ask", () => {
    const link = new Link();
    const a = link.device("a");
    const r = link.device("r", "reader");
    a.presentation.start("presenter");
    r.presentation.start("presenter"); // a reader asking to present only follows
    expect(r.presentation.isPilot).toBe(false);
    expect(r.presentation.mayPilot).toBe(false);
    expect(r.presentation.canAsk).toBe(false);
    expect(r.presentation.take()).toBe(false);
    press(a.presentation, "ArrowRight");
    expect(r.presentation.pos).toEqual({ slide: 2, step: 0 });
    press(r.presentation, "End"); // follows, so keys do nothing
    expect(a.presentation.pos).toEqual({ slide: 2, step: 0 });
    a.presentation.stop();
    r.presentation.stop();
  });

  it("PRS-38 a reader's forged messages move nobody", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    const r = link.device("r", "reader");
    a.presentation.start("presenter");
    b.presentation.start("follower");
    // The reader's device speaks as a pilot would.
    link.channel("r").broadcast({ t: "take", epoch: 50 });
    link
      .channel("r")
      .broadcast({ t: "state", epoch: 50, seq: 1, slide: 3, step: 0, screen: "black" });
    expect(a.presentation.isPilot).toBe(true);
    expect(b.presentation.pos).toEqual({ slide: 1, step: 0 });
    expect(b.presentation.screen).toBe("show");
    expect(r.presentation.mode).toBe("off");
    a.presentation.stop();
    b.presentation.stop();
  });

  it("PRS-37 a person who may write takes over from a pilot who left, from the same slide", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("presenter");
    press(a.presentation, "ArrowRight", "ArrowRight");
    b.presentation.peerGone("a");
    expect(b.presentation.canTake).toBe(true);
    expect(b.presentation.take()).toBe(true);
    expect(b.presentation.pos).toEqual({ slide: 2, step: 1 });
    expect(link.sent.at(-1)?.m).toMatchObject({ t: "state", slide: 2, step: 1 });
    a.presentation.stop();
    b.presentation.stop();
  });

  it("a presenter that leaves ends the presentation for the others", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("public");
    press(a.presentation, "ArrowRight");
    a.presentation.stop();
    expect(b.presentation.ended).toBe(true);
    expect(b.presentation.page).toBe(2); // the last slide stays
    b.presentation.stop();
  });

  it("a slide change switches the laser off for the others", () => {
    const link = new Link();
    const a = link.device("a");
    const b = link.device("b");
    a.presentation.start("presenter");
    b.presentation.start("public");
    press(a.presentation, "l");
    a.presentation.moveLaser(0.5, 0.5);
    expect(b.presentation.laser).not.toBeNull();
    press(a.presentation, "ArrowRight");
    expect(b.presentation.laser).toBeNull();
    a.presentation.stop();
    b.presentation.stop();
  });
});
