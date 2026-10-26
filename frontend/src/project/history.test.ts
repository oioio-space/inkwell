import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import { describe, expect, it } from "vitest";
import { parseMessage } from "./history";
import type { IndexDoc, Role } from "./model";
import { ProjectIndex } from "./ProjectIndex";

function open(role: Role = "admin", who = { id: "alice", name: "Alice" }, now = () => 5_000) {
  const repo = new Repo({});
  const root = repo.create<IndexDoc>({ text: "" });
  let current = role;
  const index = new ProjectIndex(root, {
    me: () => who,
    role: () => current,
    createText: (text) => repo.create<{ text: string }>({ text }).url,
    textOf: () => "contenu",
    tags: () => ({ restored: "restauré", copy: "copie" }),
    now,
  });
  index.ensureMigrated();
  return {
    index,
    repo,
    as(next: Role) {
      current = next;
    },
  };
}

describe("fileOps(since): what happened to the files, with who and when", () => {
  it("lists each operation with its author, its time and a sentence", () => {
    const { index } = open("admin", { id: "alice", name: "Alice" }, () => 5_000_000);
    const dir = index.createFolder(null, "chapitres");
    const file = index.createFile((dir as { ids: string[] }).ids[0] as string, "ch1.typ");
    const id = (file as { ids: string[] }).ids[0] as string;
    index.rename(id, "intro.typ");
    index.move([id], null);
    index.remove([id]);
    index.restore([id]);

    const ops = index.fileOps();
    expect(ops.map((o) => o.op)).toEqual([
      "create",
      "create",
      "rename",
      "move",
      "trash",
      "restore",
    ]);
    expect(ops.map((o) => o.text)).toEqual([
      "Alice a créé chapitres",
      "Alice a créé ch1.typ",
      "Alice a renommé ch1.typ en intro.typ",
      "Alice a déplacé intro.typ vers la racine du projet",
      "Alice a mis intro.typ à la corbeille",
      "Alice a restauré intro.typ",
    ]);
    expect(ops.every((o) => o.by === "alice" && o.who === "Alice" && o.time === 5_000_000)).toBe(
      true,
    );
    // A migration is not an operation on the files.
    expect(ops.some((o) => o.items.some((i) => i.name === "main.typ"))).toBe(false);
  });

  it("since keeps only the later ones, and the list grows as the index does", () => {
    let now = 10_000;
    const { index } = open("admin", { id: "alice", name: "Alice" }, () => now);
    index.createFile(null, "a.md");
    now = 20_000;
    index.createFile(null, "b.md");
    expect(index.fileOps(10_000).map((o) => o.items[0]?.name)).toEqual(["b.md"]);
    now = 30_000;
    index.createFile(null, "c.md");
    expect(index.fileOps().map((o) => o.items[0]?.name)).toEqual(["a.md", "b.md", "c.md"]);
  });

  it("a request, its refusal and its acceptance are all there, by name", () => {
    const { index, as } = open("writer", { id: "bob", name: "Bob" });
    const id = (index.createFile(null, "ch2.typ") as { ids: string[] }).ids[0] as string;
    index.remove([id]);
    as("admin");
    index.refuse([id]);
    as("writer");
    index.remove([id]);
    as("admin");
    index.accept([id]);
    expect(index.fileOps().map((o) => o.op)).toEqual([
      "create",
      "request",
      "refuse",
      "request",
      "accept",
    ]);
    expect(index.fileOps().at(2)?.text).toBe("Bob a refusé la suppression de ch2.typ");
  });

  it("a batch is one entry that names how many", () => {
    const { index } = open();
    index.createMany(
      Array.from({ length: 60 }, (_, i) => ({
        type: "text" as const,
        name: `f${i}.md`,
        parent: null,
        url: index.handle.url,
      })),
    );
    const [entry] = index.fileOps();
    expect(entry?.count).toBe(60);
    expect(entry?.items.length).toBeLessThanOrEqual(40);
    expect(entry?.text).toBe("Alice a créé f0.md et 59 autre(s)");
  });

  it("the deletion rule and the main file are in the history", () => {
    const { index } = open();
    index.setMode("free");
    const id = (index.createFile(null, "x.typ") as { ids: string[] }).ids[0] as string;
    index.setMain(id);
    expect(index.fileOps().map((o) => o.text)).toEqual([
      "Alice a permis de supprimer les fichiers librement",
      "Alice a créé x.typ",
      "Alice a défini x.typ comme fichier principal",
    ]);
  });
});

describe("FM-19 « Revenir » is the exact inverse, in one change", () => {
  it("undoes a rename, a move, a deletion and a rule change", () => {
    const { index } = open();
    const dir = (index.createFolder(null, "d") as { ids: string[] }).ids[0] as string;
    const id = (index.createFile(null, "a.typ") as { ids: string[] }).ids[0] as string;
    const paths = () => index.tree.entries.map((e) => e.path);
    const before = paths();
    const heads = () => A.getHeads(index.handle.doc() as A.Doc<IndexDoc>).length;

    index.rename(id, "b.typ");
    const rename = index.fileOps().at(-1);
    expect(rename && index.canRevert(rename)).toBe(true);
    expect(index.revert(rename as NonNullable<typeof rename>).ok).toBe(true);
    expect(paths()).toEqual(before);

    index.move([id], dir);
    const move = index.fileOps().at(-1);
    expect(index.revert(move as NonNullable<typeof move>).ok).toBe(true);
    expect(paths()).toEqual(before);

    index.remove([id]);
    const trash = index.fileOps().at(-1);
    const changes = A.getAllChanges(index.handle.doc() as A.Doc<IndexDoc>).length;
    expect(index.revert(trash as NonNullable<typeof trash>).ok).toBe(true);
    expect(paths()).toEqual(before);
    expect(A.getAllChanges(index.handle.doc() as A.Doc<IndexDoc>).length).toBe(changes + 1);
    expect(heads()).toBe(1);

    index.setMode("free");
    const mode = index.fileOps().at(-1);
    index.revert(mode as NonNullable<typeof mode>);
    expect(index.mode).toBe("approval");
  });

  it("what cannot be undone says so", () => {
    const { index } = open();
    const id = (index.createFile(null, "a.typ") as { ids: string[] }).ids[0] as string;
    index.setMain(id);
    const main = index.fileOps().at(-1);
    expect(main && index.canRevert(main)).toBe(false);
  });
});

describe("messages come from other people", () => {
  it("are read defensively: wrong shapes, odd characters and huge texts are cleaned or ignored", () => {
    expect(parseMessage(null)).toBeUndefined();
    expect(parseMessage("not json")).toBeUndefined();
    expect(parseMessage('{"k":"file","v":2,"op":"create"}')).toBeUndefined();
    expect(parseMessage('{"k":"file","v":1,"op":"explode"}')).toBeUndefined();
    const m = parseMessage(
      JSON.stringify({
        k: "file",
        v: 1,
        op: "rename",
        by: "x\u0000y",
        who: "Mal‮lory",
        items: [{ id: "1", name: "a\u0007b".repeat(300), from: 5 }, "junk", { name: "no id" }],
      }),
    );
    expect(m?.who).toBe("Mallory");
    expect(m?.by).toBe("xy");
    expect(m?.items).toHaveLength(1);
    expect(m?.items?.[0]?.name.length).toBeLessThanOrEqual(255);
    expect(m?.items?.[0]?.name.includes("\u0007")).toBe(false);
    expect(
      parseMessage(`{"k":"file","v":1,"op":"create","who":"${"x".repeat(30_000)}"}`),
    ).toBeUndefined();
  });
});
