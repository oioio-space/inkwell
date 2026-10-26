import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { describe, expect, it } from "vitest";
import { buildGraph } from "./graph";
import { history, stepsOf, textAt } from "./log";
import { normalizeHeadings } from "./normalize";
import { authorBook, claimActor } from "./people";
import { memoryRepo } from "./testkit";
import type { HDoc } from "./types";
import {
  abandonVariant,
  fork,
  mergeVariant,
  nameVersion,
  previewUndo,
  reopenVariant,
  revertTo,
  undoChange,
  undoRevert,
  whatChanged,
} from "./versions";

const make = (text: string): { repo: ReturnType<typeof memoryRepo>; h: DocHandle<HDoc> } => {
  const repo = memoryRepo();
  const h = repo.create<HDoc>({ text, meta: { versions: [], variants: [], actors: {} } });
  return { repo, h };
};

const heads = (h: DocHandle<HDoc>) => A.getHeads(h.doc());
const type = (h: DocHandle<HDoc>, text: string, at = h.doc().text.length) =>
  h.change((d) => A.splice(d, ["text"], at, 0, text));

/** What another device does while this one is away, merged in afterwards. */
function remote(h: DocHandle<HDoc>, actor: string, edit: (d: HDoc) => void): void {
  const copy = A.change(A.clone(h.doc(), { actor }), edit);
  h.update((d) => A.merge(d, copy));
}

describe("versions and going back (HIS-06 to HIS-08)", () => {
  it("HIS-06 a named version carries its name, its heads and its author's claim", () => {
    const { h } = make("un");
    claimActor(h, { id: "peer-a", name: "Alice" });
    const v = nameVersion(h, "  Relecture DSI ", heads(h));
    // What another replica reads from the document: the name and who says they made it.
    const other = A.load<HDoc>(A.save(h.doc()));
    const entry = other.meta?.versions?.at(-1);
    expect(entry).toMatchObject({ name: "Relecture DSI", by: v.by });
    expect(authorBook(other, { myPeer: "peer-b" })(entry?.by ?? "").name).toBe("Alice");
    expect(history(other).at(-1)?.action).toEqual({ kind: "version", label: "Relecture DSI" });
  });

  it("HIS-07 going back: the text of the version, one entry, Ctrl+Z, and back to now", () => {
    const { h } = make("un deux trois");
    const v = nameVersion(h, "V1");
    type(h, " quatre");
    const sessionsBefore = history(h.doc()).length;
    const done = revertTo(h, v.heads, "V1");
    expect(h.doc().text).toBe("un deux trois");
    // One new entry, « Revenu à V1 ».
    const sessions = history(h.doc());
    expect(sessions).toHaveLength(sessionsBefore + 1);
    expect(sessions.at(-1)?.action).toEqual({ kind: "revert", label: "V1" });
    expect(sessions.at(-1)?.count).toBe(1);
    // Nothing was erased: « Avant retour à V1 » is a version, and going back to it restores now.
    const auto = h.doc().meta?.versions?.find((x) => x.auto);
    expect(auto?.name).toBe("Avant retour à « V1 »");
    // Ctrl+Z takes the revert back.
    expect(done && undoRevert(h, done)).toBe(true);
    expect(h.doc().text).toBe("un deux trois quatre");
    // Going back to « now » from the automatic version.
    revertTo(h, auto?.heads ?? [], "maintenant");
    expect(h.doc().text).toBe("un deux trois quatre");
    // Going back to what the text already is does nothing.
    expect(revertTo(h, heads(h), "x")).toBeUndefined();
  });

  it("HIS-07 Ctrl+Z only works while the revert is the last thing that happened", () => {
    const { h } = make("a");
    const v = nameVersion(h, "V");
    type(h, "b");
    const done = revertTo(h, v.heads, "V");
    type(h, "c");
    expect(done && undoRevert(h, done)).toBe(false);
  });

  it("HIS-08 going back while someone types: their typing is kept, never lost", () => {
    const { h } = make("un deux trois");
    const v = nameVersion(h, "V1");
    type(h, " quatre");
    // Marie, away, types at the end of what the text was, then comes back.
    const marieCopy = A.change(A.clone(h.doc(), { actor: "bb02" }), (d) =>
      A.splice(d, ["text"], 13, 0, " et demi"),
    );
    revertTo(h, v.heads, "V1");
    h.update((d) => A.merge(d, marieCopy));
    expect(h.doc().text).toContain("et demi");
    expect(h.doc().text.startsWith("un deux trois")).toBe(true);
  });
});

describe("variants (HIS-10 to HIS-13, HIS-17)", () => {
  it("HIS-10 a variant is edited apart: the common text is unchanged, the branch starts where it was cut", () => {
    const { repo, h } = make("un deux");
    const v = nameVersion(h, "Relecture");
    type(h, " trois");
    const { variant, handle } = fork(repo, h, "procédure courte", v.heads);
    type(handle, " variante");
    expect(h.doc().text).toBe("un deux trois");
    expect(handle.doc().text).toBe("un deux variante");
    expect(h.doc().meta?.variants?.[0]).toMatchObject({
      name: "procédure courte",
      state: "open",
      url: variant.url,
    });

    const rows = buildGraph({
      order: stepsOf(h.doc()).map((s) => s.hash),
      sessions: history(h.doc()),
      versions: h.doc().meta?.versions ?? [],
      variants: h.doc().meta?.variants ?? [],
    });
    const at = (kind: string) => rows.findIndex((r) => r.kind === kind);
    // Newest first: the variant sits just above the version it was cut from.
    expect(rows.at(0)?.kind).toBe("now");
    const vr = rows.findIndex((r) => r.kind === "variant");
    expect(rows[vr + 1]?.kind).toBe("version");
    expect(rows[vr + 1]?.forkFrom).toBe(rows[vr]?.lane);
    expect(rows[vr]?.down).toBe(true);
    expect(at("variant")).toBeLessThan(rows.findIndex((r) => r.kind === "version"));
  });

  it("HIS-11 merging a variant edited while the main one went on: union, an « avant fusion » version, merged", () => {
    const { repo, h } = make("a\n\nb\n");
    const { variant, handle } = fork(repo, h, "V");
    handle.change((d) => A.splice(d, ["text"], 0, 0, "avant-variante\n"));
    h.change((d) => A.splice(d, ["text"], d.text.length, 0, "après-principale\n"));
    expect(mergeVariant(h, handle, variant)).toBe(true);
    const text = h.doc().text;
    expect(text).toContain("avant-variante");
    expect(text).toContain("après-principale");
    expect(h.doc().meta?.variants?.[0]?.state).toBe("merged");
    expect(h.doc().meta?.versions?.some((v) => v.name === "Avant fusion de « V »")).toBe(true);
    // The merge is drawn coming back to the main line.
    const rows = buildGraph({
      order: stepsOf(h.doc()).map((s) => s.hash),
      sessions: history(h.doc()),
      versions: h.doc().meta?.versions ?? [],
      variants: h.doc().meta?.variants ?? [],
    });
    const merge = rows.find((r) => r.kind === "merge");
    expect(merge).toMatchObject({ lane: 0, mergeFrom: 1 });
    const variantRow = rows.findIndex((r) => r.kind === "variant");
    expect(rows.findIndex((r) => r.kind === "merge")).toBeLessThan(variantRow);
    expect(rows[variantRow]?.up).toBe(true);
  });

  it("HIS-12 merging twice changes nothing", () => {
    const { repo, h } = make("a");
    const { variant, handle } = fork(repo, h, "V");
    handle.change((d) => A.splice(d, ["text"], 1, 0, "b"));
    mergeVariant(h, handle, variant);
    const once = h.doc().text;
    const heads1 = heads(h);
    expect(mergeVariant(h, handle, variant)).toBe(false);
    expect(h.doc().text).toBe(once);
    expect(heads(h)).toEqual(heads1);
  });

  it("HIS-13 abandoning hides a variant, keeps it, and it can come back", () => {
    const { repo, h } = make("a");
    const { variant } = fork(repo, h, "V");
    abandonVariant(h, variant.url);
    const input = () => ({
      order: stepsOf(h.doc()).map((s) => s.hash),
      sessions: history(h.doc()),
      versions: [],
      variants: h.doc().meta?.variants ?? [],
    });
    expect(h.doc().meta?.variants?.[0]?.state).toBe("abandoned");
    expect(buildGraph(input()).some((r) => r.kind === "variant")).toBe(false);
    expect(buildGraph({ ...input(), showAbandoned: true }).some((r) => r.kind === "variant")).toBe(
      true,
    );
    reopenVariant(h, variant.url);
    expect(buildGraph(input()).some((r) => r.kind === "variant")).toBe(true);
  });

  it("HIS-17 the graph is in causal order, newest first, whatever the clocks say", () => {
    const { h } = make("un");
    type(h, " deux");
    nameVersion(h, "V1");
    type(h, " trois");
    remote(h, "bb02", (d) => A.splice(d, ["text"], 0, 0, ">"));
    const rows = buildGraph({
      order: stepsOf(h.doc()).map((s) => s.hash),
      sessions: history(h.doc()),
      versions: h.doc().meta?.versions ?? [],
      variants: [],
    });
    const pos = rows.slice(1).map((r) => r.pos);
    expect(pos).toEqual([...pos].sort((a, b) => b - a));
    expect(rows[0]?.kind).toBe("now");
  });
});

describe("undoing one change (DIF-08)", () => {
  it("DIF-08 an old change is taken back alone: later ones stay, and the preview says so first", () => {
    const { h } = make("Premier.\n");
    const before = heads(h);
    type(h, "Deuxième.\n");
    const after = heads(h);
    type(h, "Troisième.\n");
    h.change((d) => A.splice(d, ["text"], 0, 0, "Ah. "));
    const done = whatChanged(h, before, after) as NonNullable<ReturnType<typeof whatChanged>>;
    const preview = previewUndo(h, done);
    expect(preview).toBe("Ah. Premier.\nTroisième.\n");
    // Nothing applied yet.
    expect(h.doc().text).toBe("Ah. Premier.\nDeuxième.\nTroisième.\n");
    const steps = history(h.doc()).length;
    undoChange(h, done, "Deuxième");
    expect(h.doc().text).toBe(preview);
    expect(history(h.doc())).toHaveLength(steps + 1);
    expect(history(h.doc()).at(-1)?.action?.kind).toBe("undo");
  });

  it("taking back a removal puts the text back", () => {
    const { h } = make("un deux trois");
    const before = heads(h);
    h.change((d) => A.updateText(d, ["text"], "un trois"));
    const after = heads(h);
    undoChange(
      h,
      whatChanged(h, before, after) as NonNullable<ReturnType<typeof whatChanged>>,
      "x",
    );
    expect(h.doc().text).toBe("un deux trois");
  });
});

describe("X-04 two people change the level of one heading", () => {
  it("ends with one level, the same on both sides, not `=====`", () => {
    const base = A.change(A.init<HDoc>({ actor: "aa01" }), (d) => {
      d.text = "= Titre\ncorps\n";
    });
    const repo = memoryRepo();
    const ha = repo.import<HDoc>(A.save(base));
    const hb = repo.import<HDoc>(A.save(base));
    // A makes it level 2, B makes it level 3, at the same time.
    ha.change((d) => A.splice(d, ["text"], 0, 0, "="));
    hb.change((d) => A.splice(d, ["text"], 0, 0, "=="));
    const beforeA = ha.doc();
    const beforeB = hb.doc();
    const docA = ha.doc();
    const docB = hb.doc();
    ha.update((d) => A.merge(d, docB));
    hb.update((d) => A.merge(d, docA));
    expect(ha.doc().text.startsWith("====")).toBe(true);
    // Each side runs the rule on the merge; only one writes the correction.
    const fixedA = normalizeHeadings(ha, beforeA);
    const fixedB = normalizeHeadings(hb, beforeB);
    expect([fixedA, fixedB].filter(Boolean)).toHaveLength(1);
    // The correction travels to the other side.
    ha.update((d) => A.merge(d, hb.doc()));
    hb.update((d) => A.merge(d, ha.doc()));
    expect(ha.doc().text).toBe(hb.doc().text);
    expect(ha.doc().text).toBe("=== Titre\ncorps\n");
    expect(textAt(ha.doc(), heads(ha))).toBe(ha.doc().text);
  });
});
