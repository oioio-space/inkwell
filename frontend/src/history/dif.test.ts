import { describe, expect, it } from "vitest";
import { cellDiff } from "./csvdiff";
import { attribute, compareTexts, entryDiff, hunksOf } from "./diffview";
import { filterSessions, NO_FILTER, sectionsTouched } from "./filters";
import { claimsOf, history, stepsOf, textAt } from "./log";
import { authorBook } from "./people";
import { moveSections, stamp } from "./stamp";
import { summarize } from "./summary";
import { Device } from "./testkit";
import { blameOf } from "./track";
import { clock, when } from "./when";
import { diffWithMoves, wordDiff } from "./words";

const DOC = `= Bascule vers le site de secours
Avant toute action, vérifier l'état de la réplication. Prévenir le responsable de permanence.

== Étapes
Geler les écritures.
Promouvoir le réplica.
`;

const kinds = (ops: ReturnType<typeof wordDiff>) =>
  ops.filter((o) => o.kind !== "eq").map((o) => [o.kind, o.text]);

describe("readable differences (DIF-01 to DIF-04)", () => {
  it("DIF-01 an added sentence, a removed one: marks, exact summary, author, colour and time", () => {
    const alice = new Device("aa01").type(DOC, 0).claim("Alice");
    const marie = new Device("bb02", alice.doc, alice.clock + 3600).claim("Marie");
    const before = marie.doc.text;
    marie.set(
      before
        .replace(" Prévenir le responsable de permanence.", "")
        .replace("Geler les écritures.", "Geler les écritures. Couper les accès externes."),
      2,
    );
    alice.sync(marie);
    const sessions = history(alice.doc);
    const session = sessions.find(
      (s) => s.actor === "bb02" && !s.action && s.end > alice.clock - 100,
    );
    const last = sessions.at(-1);
    expect(last?.actor).toBe("bb02");
    const after = textAt(alice.doc, last?.after ?? []);
    const e = entryDiff(before, after, last as NonNullable<typeof last>);
    expect(session).toBeDefined();
    expect(kinds(e.ops)).toEqual([
      ["del", " Prévenir le responsable de permanence."],
      ["add", " Couper les accès externes."],
    ]);
    const who = authorBook(claimsOf(stepsOf(alice.doc)), { myActors: new Set(["aa01"]) })("bb02");
    const summary = summarize(before, after, e.ops, who.name);
    expect(summary.text).toBe(
      "Marie a supprimé 1 phrase dans « Bascule vers le site de secours » et ajouté 1 phrase dans « Étapes »",
    );
    expect(who.name).toBe("Marie");
    expect(who.color).toMatch(/^var\(--person-[1-8]\)$/);
    expect(e.frags.filter((f) => f.kind !== "eq").every((f) => f.actor === "bb02")).toBe(true);
    // The time in plain words.
    const now = (last?.end ?? 0) + 60_000;
    expect(stamp(last?.end ?? 0, false, "Marie", now)).toBe(`aujourd'hui ${clock(last?.end ?? 0)}`);
  });

  it("DIF-01 a title modified is counted as a title, named by its own section", () => {
    const a = DOC;
    const b = DOC.replace("== Étapes", "== Étapes détaillées");
    const ops = diffWithMoves(a, b);
    expect(summarize(a, b, ops, "Marie").text).toBe(
      "Marie a modifié 1 titre dans « Étapes détaillées »",
    );
  });

  it("DIF-02 one letter fixed in a word: the whole word is marked, the letter is underlined", () => {
    const ops = wordDiff("La sauvegarde est prète.", "La sauvegarde est prête.");
    const [del, add] = ops.filter((o) => o.kind !== "eq");
    expect(del).toMatchObject({ kind: "del", text: "prète", inner: [2, 3] });
    expect(add).toMatchObject({ kind: "add", text: "prête", inner: [2, 3] });
  });

  it("DIF-03 a paragraph moved is shown as moved, not as removed plus added", () => {
    const a =
      "= Un\nPremier paragraphe assez long pour compter.\n\n= Deux\nSecond paragraphe qui change de place ici.\n\n= Trois\nFin du document.\n";
    const b =
      "= Un\nPremier paragraphe assez long pour compter.\n\n= Deux\n\n= Trois\nSecond paragraphe qui change de place ici.\nFin du document.\n";
    const ops = diffWithMoves(a, b);
    const moved = ops.filter((o) => o.moved !== undefined);
    expect(moved.map((o) => o.kind).sort()).toEqual(["add", "del"]);
    expect(moved[0]?.moved).toBe(moved[1]?.moved);
    const s = summarize(a, b, ops, "Paul");
    expect(s.text).toBe("Paul a déplacé 1 paragraphe dans « Trois »");
    expect(s.text).not.toMatch(/supprimé|ajouté/);
    expect([...moveSections(a, b, ops).values()]).toEqual([{ from: "Deux", to: "Trois" }]);
  });

  it("DIF-04 two people changed the same paragraph: each fragment keeps its author", async () => {
    const alice = new Device("aa01").type("Alpha beta gamma delta epsilon.", 0).claim("Alice");
    const marie = new Device("bb02", alice.doc).claim("Marie");
    const paul = new Device("cc03", alice.doc).claim("Paul");
    const base = alice.doc.text;
    marie.set(base.replace("beta", "BÊTA"), 5);
    paul.set(base.replace("delta", "DELTA"), 9);
    alice.sync(marie).sync(paul);
    const sessions = history(alice.doc);
    const blame = await blameOf(alice.doc, sessions);
    const e = compareTexts(base, alice.doc.text, blame);
    const who = (actor?: string) => authorBook(claimsOf(stepsOf(alice.doc)), {})(actor ?? "?").name;
    const changed = e.frags
      .filter((f) => f.kind !== "eq")
      .map((f) => [f.kind, f.text, who(f.actor)]);
    expect(changed).toEqual([
      ["del", "beta", "Marie"],
      ["add", "BÊTA", "Marie"],
      ["del", "delta", "Paul"],
      ["add", "DELTA", "Paul"],
    ]);
    expect(hunksOf(e.frags)).toHaveLength(2);
    // A stretch nobody can be found for stays without an author rather than a wrong one.
    const nobody = attribute(wordDiff("a b", "a c"), undefined);
    expect(nobody.every((f) => f.actor === undefined)).toBe(true);
  });

  it("words are Unicode: accents, emoji and CJK do not break a word", () => {
    const ops = wordDiff("café 🙂 日本語", "café 🙃 日本語");
    expect(kinds(ops).map(([k]) => k)).toEqual(["del", "add"]);
    expect(wordDiff("a\r\nb", "a\r\nb")).toHaveLength(1);
  });
});

describe("filters (DIF-06)", () => {
  it("DIF-06 by person, by period and by section: the list follows and the counter is right", () => {
    const alice = new Device("aa01").type("= Un\nuno\n\n= Deux\ndos\n", 0).claim("Alice");
    const marie = new Device("bb02", alice.doc, alice.clock + 10).claim("Marie");
    marie.type("!", 7, 1);
    const paul = new Device("cc03", alice.doc, alice.clock + 10_000_000).claim("Paul");
    paul.type("fin\n", paul.doc.text.length, 1);
    alice.sync(marie).sync(paul);
    const sessions = history(alice.doc);
    const author = authorBook(claimsOf(stepsOf(alice.doc)), { myActors: new Set(["aa01"]) });
    const now = alice.clock * 1000 + 10_000_000_000 / 1000;
    const ctx = { author, now, doc: alice.doc };

    const all = filterSessions(sessions, NO_FILTER, ctx);
    expect(all.shown).toHaveLength(all.total);
    const byPerson = filterSessions(sessions, { ...NO_FILTER, person: "Marie" }, ctx);
    expect(byPerson.shown.every((s) => author(s.actor).name === "Marie")).toBe(true);
    expect(byPerson.shown.length).toBeGreaterThan(0);
    expect(byPerson.total).toBe(all.total);

    const recent = filterSessions(
      sessions,
      { ...NO_FILTER, period: "today" },
      { ...ctx, now: paul.clock * 1000 },
    );
    expect(recent.shown.map((s) => s.actor)).toEqual(["cc03"]);

    const inDeux = filterSessions(sessions, { ...NO_FILTER, section: "Deux" }, ctx);
    expect(inDeux.shown.map((s) => s.actor)).toContain("cc03");
    expect(inDeux.shown.map((s) => s.actor)).not.toContain("bb02");
    expect(
      sectionsTouched(alice.doc, byPerson.shown[0] as NonNullable<(typeof byPerson.shown)[0]>),
    ).toEqual(["Un"]);
  });
});

describe("tables and clocks", () => {
  it("DIF-09 a table: « B3 : 12 → 15 », cell by cell", () => {
    const a = "nom,qté\nvis,10\nécrou,12\n";
    const b = "nom,qté\nvis,10\nécrou,15\nrondelle,3\n";
    expect(cellDiff(a, b).map((c) => `${c.ref} : ${c.before ?? "∅"} → ${c.after ?? "∅"}`)).toEqual([
      "B3 : 12 → 15",
      "A4 : ∅ → rondelle",
      "B4 : ∅ → 3",
    ]);
    // Semicolons and tabs are found, quoted cells with the delimiter inside stay whole.
    expect(cellDiff('a;"b;c"\n', 'a;"b;d"\n')[0]).toMatchObject({
      ref: "B1",
      before: "b;c",
      after: "b;d",
    });
  });

  it("DIF-10 a device with its clock two hours ahead: causal order, and the time says whose clock", () => {
    const alice = new Device("aa01").type("un ", 0).claim("Alice");
    const marie = new Device("bb02", alice.doc, alice.clock + 2 * 3600)
      .type("deux ", 3, 1)
      .claim("Marie");
    alice.sync(marie);
    alice.type("trois", alice.doc.text.length, 5);
    const nowMs = (alice.clock + 600) * 1000;
    const sessions = history(alice.doc, nowMs);
    const author = authorBook(claimsOf(stepsOf(alice.doc)), { myActors: new Set(["aa01"]) });
    const bySkew = sessions.filter((s) => s.skewed);
    expect(bySkew.length).toBeGreaterThan(0);
    const s = bySkew[0] as NonNullable<(typeof bySkew)[0]>;
    expect(stamp(s.end, s.skewed, author(s.actor).name, nowMs)).toContain(
      `selon l'appareil de ${author(s.actor).name}`,
    );
    const calm = sessions.find((x) => !x.skewed) as NonNullable<(typeof sessions)[0]>;
    expect(stamp(calm.end, false, "x", nowMs)).not.toContain("selon");
    // Plain French dates.
    const d = new Date(2026, 9, 20, 14, 10).getTime();
    const day = (n: number) => d + n * 86_400_000;
    expect(when(d, d + 1000)).toBe("aujourd'hui 14:10");
    expect(when(d, day(1))).toBe("hier 14:10");
    expect(when(d, day(3))).toMatch(/^[a-zé]+ 14:10$/);
    expect(when(d, day(30))).toMatch(/oct\.? 14:10$/);
  });
});
