import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { materialize } from "./materialize";
import { idFrom, plain } from "./model";
import * as ops from "./ops";
import {
  admin2,
  alice,
  apply,
  bob,
  carol,
  ctx,
  HASH,
  type Index,
  idOf,
  newProject,
  paths,
  sync,
  textUrl,
  view,
} from "./testkit";

/** Creates a text file as `who` and returns the new document. */
function addText(doc: Index, who: typeof alice, name: string, parent: string | null = null) {
  return apply(
    doc,
    ops.create(doc, view(doc), ctx(who), [{ type: "text", name, parent, url: textUrl() }]),
  );
}

function addDir(doc: Index, who: typeof alice, name: string, parent: string | null = null) {
  return apply(doc, ops.create(doc, view(doc), ctx(who), [{ type: "dir", name, parent }]));
}

describe("migration of the root document", () => {
  it("the main.typ node points to the document itself and keeps its text", () => {
    const url = textUrl();
    const doc = newProject(url);
    const main = view(doc).entries[0];
    expect(main?.path).toBe("main.typ");
    expect(main?.url).toBe(url);
    expect(main?.main).toBe(true);
    expect(main?.id).toBe(idFrom(url));
    expect(doc.text).toBe("");
  });

  it("two people migrating the same project write the same node", () => {
    const url = textUrl();
    const a = newProject(url);
    const b = newProject(url);
    expect(Object.keys(a.nodes ?? {})).toEqual(Object.keys(b.nodes ?? {}));
  });
});

describe("PRJ-04 PRJ-05 FM-01 create, rename and the identity of a file", () => {
  it("FM-01 creates a file in a folder; the history message says who and what", () => {
    let doc = newProject();
    doc = addDir(doc, bob, "chapitres");
    doc = addText(doc, bob, "notes.md", idOf(doc, "chapitres"));
    expect(paths(doc)).toEqual(["chapitres", "chapitres/notes.md", "main.typ"]);
    const changes = A.getAllChanges(doc).map((c) => A.decodeChange(c).message);
    const last = JSON.parse(changes.at(-1) as string);
    expect(last).toMatchObject({
      k: "file",
      op: "create",
      who: "Bob",
      items: [{ name: "notes.md" }],
    });
  });

  it("PRJ-05 a rename keeps the url of the document", () => {
    let doc = addText(newProject(), alice, "ch1.typ");
    const before = view(doc).byPath.get("ch1.typ");
    doc = apply(doc, ops.rename(view(doc), ctx(bob), before?.id as string, "intro.typ"));
    const after = view(doc).byPath.get("intro.typ");
    expect(after?.id).toBe(before?.id);
    expect(after?.url).toBe(before?.url);
  });

  it("PRJ-09 renaming a folder is one write, however many files it holds", () => {
    let doc = addDir(newProject(), alice, "a");
    const dir = idOf(doc, "a");
    for (let i = 0; i < 5; i++) doc = addText(doc, alice, `f${i}.typ`, dir);
    const changes = A.getAllChanges(doc).length;
    doc = apply(doc, ops.rename(view(doc), ctx(alice), dir, "b"));
    expect(A.getAllChanges(doc).length).toBe(changes + 1);
    expect(paths(doc)).toContain("b/f4.typ");
    const last = A.decodeChange(A.getAllChanges(doc).at(-1) as A.Change);
    expect(last.ops).toHaveLength(1);
  });
});

describe("FM-03 renaming with an invalid or a taken name", () => {
  it("says why and changes nothing", () => {
    const doc = addText(newProject(), alice, "a.typ");
    const id = idOf(doc, "a.typ");
    const m = view(doc);
    expect(ops.rename(m, ctx(alice), id, "a/b")).toEqual({
      ok: false,
      error: { code: "invalid_name", reason: "bad_char", name: "a/b" },
    });
    expect(ops.rename(m, ctx(alice), id, "CON.txt")).toMatchObject({
      error: { code: "invalid_name", reason: "device" },
    });
    expect(ops.rename(m, ctx(alice), id, "MAIN.typ")).toMatchObject({ error: { code: "exists" } });
    expect(ops.rename(m, ctx(alice), id, "a.typ")).toMatchObject({ error: { code: "unchanged" } });
  });

  it("allows changing only the case of a name", () => {
    const doc = addText(newProject(), alice, "a.typ");
    const next = apply(doc, ops.rename(view(doc), ctx(alice), idOf(doc, "a.typ"), "A.typ"));
    expect(paths(next)).toContain("A.typ");
  });
});

describe("FM-04 moving", () => {
  it("moves a folder into another and refuses its own descendant", () => {
    let doc = addDir(newProject(), alice, "a");
    doc = addDir(doc, alice, "b");
    doc = addDir(doc, alice, "c", idOf(doc, "b"));
    doc = apply(doc, ops.move(view(doc), ctx(alice), [idOf(doc, "b")], idOf(doc, "a")));
    expect(paths(doc)).toContain("a/b/c");
    const m = view(doc);
    expect(ops.move(m, ctx(alice), [idOf(doc, "a")], idOf(doc, "a/b/c"))).toMatchObject({
      error: { code: "cycle" },
    });
    expect(ops.move(m, ctx(alice), [idOf(doc, "a")], idOf(doc, "a"))).toMatchObject({
      error: { code: "cycle" },
    });
  });

  it("refuses a move that would collide, and one that is too deep", () => {
    let doc = addDir(newProject(), alice, "a");
    doc = addText(doc, alice, "x.typ");
    doc = addText(doc, alice, "X.typ", idOf(doc, "a"));
    expect(ops.move(view(doc), ctx(alice), [idOf(doc, "x.typ")], idOf(doc, "a"))).toMatchObject({
      error: { code: "exists" },
    });

    let deep = newProject();
    let parent: string | null = null;
    for (let i = 0; i < 32; i++) {
      deep = addDir(deep, alice, `d${i}`, parent);
      parent = view(deep).entries.find((e) => e.name === `d${i}`)?.id ?? null;
    }
    expect(
      ops.create(deep, view(deep), ctx(alice), [{ type: "dir", name: "x", parent }]),
    ).toMatchObject({ error: { code: "depth" } });
  });
});

describe("PRJ-06 PRJ-07 PRJ-08 PRJ-16 PRJ-17 concurrent edits of the tree", () => {
  it("PRJ-07 two notes.md created together: no overwrite, the second gets the author's name", () => {
    const base = newProject();
    const a = addText(A.clone(base), alice, "notes.md");
    const b = addText(A.clone(base), bob, "notes.md");
    const [merged, same] = sync(a, b);
    const names = view(merged).entries.map((e) => e.name);
    expect(names.filter((n) => n.startsWith("notes"))).toHaveLength(2);
    expect(names).toContain("notes.md");
    const suffixed = view(merged).entries.find((e) => e.renamedFrom === "notes.md");
    expect(suffixed?.name).toMatch(/^notes \((Bob|Alice)\)\.md$/);
    // The one that keeps the name is the smaller id, on both replicas.
    expect(view(same).entries.map((e) => e.path)).toEqual(view(merged).entries.map((e) => e.path));
  });

  it("PRJ-17 case and NFC/NFD names collide", () => {
    const base = newProject();
    const a = addText(A.clone(base), alice, "Café.md");
    const b = addText(A.clone(base), bob, "café.MD");
    const [merged] = sync(a, b);
    expect(
      view(merged).entries.filter((e) => e.type === "text" && e.name !== "main.typ"),
    ).toHaveLength(2);
    const keys = new Set(paths(merged).map((p) => p.normalize("NFC").toLowerCase()));
    expect(keys.size).toBe(3);
  });

  it("PRJ-08 two renames of one file converge on the same name; the other stays in history", () => {
    const base = addText(newProject(), alice, "ch1.typ");
    const id = idOf(base, "ch1.typ");
    const a = apply(A.clone(base), ops.rename(view(base), ctx(alice), id, "alpha.typ"));
    const b = apply(A.clone(base), ops.rename(view(base), ctx(bob), id, "beta.typ"));
    const [x, y] = sync(a, b);
    expect(paths(x)).toEqual(paths(y));
    expect(paths(x).filter((p) => p.endsWith(".typ") && p !== "main.typ")).toHaveLength(1);
    const messages = A.getAllChanges(x).map((c) => A.decodeChange(c).message ?? "");
    expect(messages.join()).toContain("alpha.typ");
    expect(messages.join()).toContain("beta.typ");
  });

  it("PRJ-16 two crossed moves make a cycle: the smaller id goes to the root, on both replicas", () => {
    let base = addDir(newProject(), alice, "a");
    base = addDir(base, alice, "b");
    const ia = idOf(base, "a");
    const ib = idOf(base, "b");
    const a = apply(A.clone(base), ops.move(view(base), ctx(alice), [ia], ib));
    const b = apply(A.clone(base), ops.move(view(base), ctx(bob), [ib], ia));
    const [x, y] = sync(a, b);
    expect(paths(x)).toEqual(paths(y));
    const m = view(x);
    const roots = m.entries.filter((e) => e.parentId === null && e.type === "dir");
    expect(roots).toHaveLength(1);
    expect(roots[0]?.id).toBe(ia < ib ? ia : ib);
    expect(roots[0]?.cycle).toBe(true);
    expect(m.entries).toHaveLength(3);
  });

  it("a node whose parent is gone is attached to the root and marked", () => {
    let doc = addDir(newProject(), alice, "d");
    doc = addText(doc, alice, "f.typ", idOf(doc, "d"));
    const f = idOf(doc, "d/f.typ");
    const broken = A.change(doc, (d) => {
      delete d.nodes?.[idOf(doc, "d")];
    });
    const e = view(broken).byId.get(f);
    expect(e?.path).toBe("f.typ");
    expect(e?.orphan).toBe(true);
  });

  it("past the cap, nodes are ignored in id order and reported", () => {
    let doc = newProject();
    for (let i = 0; i < 6; i++) doc = addText(doc, alice, `f${i}.typ`);
    const m = materialize(plain(doc), { maxNodes: 4 });
    expect(m.entries).toHaveLength(4);
    expect(m.warnings).toContainEqual({ kind: "capped", count: 3 });
    const kept = Object.keys(plain(doc).nodes ?? {})
      .sort()
      .slice(0, 4);
    expect(m.entries.map((e) => e.id).sort()).toEqual(kept);
  });

  it("PRJ-06 a file deleted while it is edited stays a document in the trash", () => {
    let doc = addText(newProject(), alice, "ch2.typ");
    const id = idOf(doc, "ch2.typ");
    const url = view(doc).byId.get(id)?.url;
    doc = apply(doc, ops.remove(doc, view(doc), ctx(alice), [id]));
    const m = view(doc);
    expect(m.byId.has(id)).toBe(false);
    expect(m.trash).toMatchObject([{ id, name: "ch2.typ", by: "alice" }]);
    expect(plain(doc).nodes?.[id]?.url).toBe(url);
    doc = apply(doc, ops.restore(doc, view(doc), ctx(alice), [id]));
    expect(paths(doc)).toContain("ch2.typ");
  });

  it("PRJ-13 a replaced image keeps its node and gets a new hash", () => {
    const start = newProject();
    let doc = apply(
      start,
      ops.create(start, view(start), ctx(bob), [
        { type: "blob", name: "fig.png", parent: null, hash: HASH, size: 3 },
      ]),
    );
    const id = idOf(doc, "fig.png");
    const next = `blake3:${"cd".repeat(32)}`;
    doc = apply(doc, ops.replaceBlob(view(doc), ctx(bob), id, { hash: next, size: 9 }));
    expect(view(doc).byId.get(id)).toMatchObject({ hash: next, size: 9, id });
    expect(view(doc).refs).toContain(next);
  });
});

describe("PRJ-12 FM-13 a reader changes nothing", () => {
  it("every operation is refused to a reader", () => {
    const doc = addText(newProject(), alice, "a.typ");
    const m = view(doc);
    const id = idOf(doc, "a.typ");
    const c = ctx(carol);
    expect(ops.create(doc, m, c, [{ type: "dir", name: "x", parent: null }])).toMatchObject({
      error: { code: "forbidden" },
    });
    for (const r of [
      ops.rename(m, c, id, "b.typ"),
      ops.move(m, c, [id], null),
      ops.remove(doc, m, c, [id]),
      ops.restore(doc, m, c, [id]),
      ops.replaceBlob(m, c, id, { hash: HASH, size: 1 }),
    ]) {
      expect(r).toMatchObject({ ok: false, error: { code: "forbidden" } });
    }
  });
});

describe("deleting on approval (B5)", () => {
  function withRequest() {
    let doc = addText(newProject(), alice, "ch2.typ");
    const id = idOf(doc, "ch2.typ");
    doc = apply(doc, ops.remove(doc, view(doc), ctx(bob), [id]));
    return { doc, id };
  }

  it("FM-05 a writer's delete is a request: the file stays, struck, with the request on it", () => {
    const { doc, id } = withRequest();
    const e = view(doc).byId.get(id);
    expect(e?.deletion).toMatchObject({ by: "bob" });
    expect(view(doc).pending.map((p) => p.id)).toEqual([id]);
    expect(view(doc).trash).toHaveLength(0);
    // Only an admin can accept.
    expect(ops.accept(view(doc), ctx(bob), [id])).toMatchObject({ error: { code: "forbidden" } });
    expect(ops.refuse(view(doc), ctx(bob), [id])).toMatchObject({ error: { code: "forbidden" } });
  });

  it("FM-06 an admin accepts: the file goes to the trash, restorable", () => {
    let { doc, id } = withRequest();
    doc = apply(doc, ops.accept(view(doc), ctx(alice), [id]));
    expect(view(doc).byId.has(id)).toBe(false);
    expect(view(doc).trash[0]?.id).toBe(id);
    doc = apply(doc, ops.restore(doc, view(doc), ctx(alice), [id]));
    expect(view(doc).byId.get(id)?.deletion).toBeUndefined();
  });

  it("FM-07 an admin refuses: the request is lifted and the refusal is on record", () => {
    let { doc, id } = withRequest();
    doc = apply(doc, ops.refuse(view(doc), ctx(alice), [id]));
    const e = view(doc).byId.get(id);
    expect(e?.deletion).toBeUndefined();
    expect(e?.refused).toMatchObject({ by: "alice", verdict: "refused" });
    expect(view(doc).pending).toHaveLength(0);
    const last = JSON.parse(A.decodeChange(A.getAllChanges(doc).at(-1) as A.Change).message ?? "");
    expect(last).toMatchObject({ op: "refuse", who: "Alice" });
    // A new request is pending again.
    doc = apply(doc, ops.remove(doc, view(doc), ctx(bob), [id]));
    expect(view(doc).byId.get(id)?.deletion).toBeDefined();
  });

  it("FM-08 one admin accepts while another refuses: the refusal wins, the same everywhere", () => {
    const { doc, id } = withRequest();
    const a = apply(A.clone(doc), ops.accept(view(doc), ctx(alice), [id]));
    const b = apply(A.clone(doc), ops.refuse(view(doc), ctx(admin2), [id]));
    const [x, y] = sync(a, b);
    for (const d of [x, y]) {
      expect(view(d).byId.has(id)).toBe(true);
      expect(view(d).trash).toHaveLength(0);
      expect(view(d).byId.get(id)?.refused).toBeDefined();
    }
    // Both acts are in the history.
    const ops_ = A.getAllChanges(x).map((c) => JSON.parse(A.decodeChange(c).message ?? "null")?.op);
    expect(ops_).toContain("accept");
    expect(ops_).toContain("refuse");
  });

  it("FM-09 an admin deletes at once a file someone edits: it is in the trash, the document lives", () => {
    const doc = addText(newProject(), alice, "live.typ");
    const id = idOf(doc, "live.typ");
    const url = view(doc).byId.get(id)?.url;
    const gone = apply(A.clone(doc), ops.remove(doc, view(doc), ctx(alice), [id]));
    expect(view(gone).trash.map((t) => t.id)).toEqual([id]);
    expect(plain(gone).nodes?.[id]?.url).toBe(url);
  });

  it("FM-10 restoring a file whose name was taken suffixes it and overwrites nothing", () => {
    let doc = addText(newProject(), alice, "notes.md");
    const first = idOf(doc, "notes.md");
    doc = apply(doc, ops.remove(doc, view(doc), ctx(alice), [first]));
    doc = addText(doc, bob, "notes.md");
    doc = apply(doc, ops.restore(doc, view(doc), ctx(alice), [first]));
    expect(paths(doc).filter((p) => p.startsWith("notes"))).toEqual([
      "notes (restauré).md",
      "notes.md",
    ]);
    expect(view(doc).byId.get(first)?.name).toBe("notes (restauré).md");
  });

  it("FM-11 restoring a file brings back its deleted folder too", () => {
    let doc = addDir(newProject(), alice, "dossier");
    doc = addText(doc, alice, "f.typ", idOf(doc, "dossier"));
    const file = idOf(doc, "dossier/f.typ");
    doc = apply(doc, ops.remove(doc, view(doc), ctx(alice), [file]));
    doc = apply(doc, ops.remove(doc, view(doc), ctx(alice), [idOf(doc, "dossier")]));
    expect(paths(doc)).toEqual(["main.typ"]);
    doc = apply(doc, ops.restore(doc, view(doc), ctx(alice), [file]));
    expect(paths(doc)).toEqual(["dossier", "dossier/f.typ", "main.typ"]);
  });

  it("FM-12 emptying the trash: admins only; the document and its url remain in the history", () => {
    let doc = addText(newProject(), alice, "x.typ");
    const id = idOf(doc, "x.typ");
    doc = apply(doc, ops.remove(doc, view(doc), ctx(alice), [id]));
    expect(ops.purge(doc, view(doc), ctx(bob), [id])).toMatchObject({
      error: { code: "forbidden" },
    });
    const before = doc;
    doc = apply(doc, ops.purge(doc, view(doc), ctx(alice), [id]));
    expect(doc.nodes?.[id]).toBeUndefined();
    expect(view(doc).trash).toHaveLength(0);
    // "Revenir" to the earlier version brings it back: the history keeps the node.
    expect(plain(A.view(doc, A.getHeads(before))).nodes?.[id]?.url).toBeDefined();
  });

  it("FM-12 purging a folder removes what is under it", () => {
    let doc = addDir(newProject(), alice, "d");
    doc = addText(doc, alice, "f.typ", idOf(doc, "d"));
    const d = idOf(doc, "d");
    doc = apply(doc, ops.remove(doc, view(doc), ctx(alice), [d]));
    doc = apply(doc, ops.purge(doc, view(doc), ctx(alice), [d]));
    expect(Object.keys(doc.nodes ?? {})).toHaveLength(1);
  });

  it("FM-15 with the rule set to free, a writer trashes and restores directly", () => {
    let doc = addText(newProject(), alice, "a.typ");
    doc = apply(doc, ops.setDeletionMode(doc, ctx(alice), "free"));
    const id = idOf(doc, "a.typ");
    doc = apply(doc, ops.remove(doc, view(doc), ctx(bob), [id]));
    expect(view(doc).trash.map((t) => t.id)).toEqual([id]);
    doc = apply(doc, ops.restore(doc, view(doc), ctx(bob), [id]));
    expect(paths(doc)).toContain("a.typ");
  });

  it("FM-16 only an admin changes the rule, and it is in the history", () => {
    const doc = newProject();
    expect(ops.setDeletionMode(doc, ctx(bob), "free")).toMatchObject({
      error: { code: "forbidden" },
    });
    const next = apply(doc, ops.setDeletionMode(doc, ctx(alice), "free"));
    expect(plain(next).settings?.deletion).toBe("free");
    const last = JSON.parse(A.decodeChange(A.getAllChanges(next).at(-1) as A.Change).message ?? "");
    expect(last).toMatchObject({ op: "mode", mode: "free" });
  });

  it("FM-17 offline: B renames a file while A deletes it; it ends in the trash under its new name", () => {
    const base = addText(newProject(), alice, "ch1.typ");
    const id = idOf(base, "ch1.typ");
    const a = apply(A.clone(base), ops.remove(base, view(base), ctx(alice), [id]));
    const b = apply(A.clone(base), ops.rename(view(base), ctx(bob), id, "intro.typ"));
    const [x] = sync(a, b);
    expect(view(x).trash).toMatchObject([{ id, name: "intro.typ" }]);
  });

  it("FM-18 the main file cannot be deleted; moving and renaming it is fine; only an admin picks another", () => {
    let doc = addText(newProject(), alice, "autre.typ");
    const main = idOf(doc, "main.typ");
    expect(ops.remove(doc, view(doc), ctx(alice), [main])).toMatchObject({
      error: { code: "main" },
    });
    doc = apply(doc, ops.rename(view(doc), ctx(bob), main, "principal.typ"));
    expect(view(doc).byId.get(main)?.main).toBe(true);
    const other = idOf(doc, "autre.typ");
    expect(ops.setMain(view(doc), ctx(bob), other)).toMatchObject({ error: { code: "forbidden" } });
    doc = apply(doc, ops.setMain(view(doc), ctx(alice), other));
    expect(view(doc).mainId).toBe(other);
    expect(view(doc).byId.get(main)?.main).toBe(false);
  });

  it("FM-19 a rename, a move and a delete are undone by their exact inverse, in one change", () => {
    let doc = addText(addDir(newProject(), alice, "d"), alice, "a.typ");
    const id = idOf(doc, "a.typ");
    const dir = idOf(doc, "d");
    const renamed = apply(A.clone(doc), ops.rename(view(doc), ctx(alice), id, "b.typ"));
    const back = apply(renamed, ops.rename(view(renamed), ctx(alice), id, "a.typ"));
    expect(paths(back)).toEqual(paths(doc));
    const moved = apply(A.clone(doc), ops.move(view(doc), ctx(alice), [id], dir));
    const moveBack = apply(moved, ops.move(view(moved), ctx(alice), [id], null));
    expect(paths(moveBack)).toEqual(paths(doc));
    const gone = apply(A.clone(doc), ops.remove(doc, view(doc), ctx(alice), [id]));
    const again = apply(gone, ops.restore(gone, view(gone), ctx(alice), [id]));
    expect(paths(again)).toEqual(paths(doc));
    doc = again;
  });
});

describe("importing and duplicating", () => {
  it("FM-02 a batch builds a folder tree in one change", () => {
    const doc = newProject();
    const r = ops.create(doc, view(doc), ctx(bob), [
      { type: "dir", name: "figs", parent: null },
      { type: "blob", name: "a.png", parent: { spec: 0 }, hash: HASH, size: 10 },
      { type: "text", name: "t.typ", parent: { spec: 0 }, url: textUrl() },
    ]);
    const changes = A.getAllChanges(doc).length;
    const next = apply(doc, r);
    expect(paths(next)).toEqual(["figs", "figs/a.png", "figs/t.typ", "main.typ"]);
    expect(A.getAllChanges(next)).toHaveLength(changes + 1);
  });

  it("uniqueName finds a free name next to the existing one", () => {
    const doc = addText(newProject(), alice, "a.typ");
    expect(ops.uniqueName(view(doc), null, "a.typ")).toBe("a (2).typ");
    expect(ops.uniqueName(view(doc), null, "z.typ")).toBe("z.typ");
  });
});
