// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { type AutomergeUrl, parseAutomergeUrl } from "@automerge/automerge-repo";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SimNet } from "../net/simnet";
import { Kit, type TestPeer, until } from "../net/testkit";
import { IndexValidator } from "./admit";
import { type IndexDoc, imm, type Role } from "./model";
import { ProjectIndex } from "./ProjectIndex";

type Text = { text: string };
type Nodes = Record<string, Record<string, unknown>>;
const node = (d: IndexDoc, id: string) =>
  (d.nodes as unknown as Nodes)[id] as Record<string, unknown>;

const kit = new Kit();
afterEach(() => kit.shutdown());
vi.setConfig({ testTimeout: 30_000 });

interface Member {
  peer: TestPeer;
  index: ProjectIndex;
  role: Role;
}

/** The tree as two replicas can be compared on it. */
const shown = (m: Member) => ({
  entries: m.index.tree.entries.map((e) => [e.id, e.path, e.type, e.renamedFrom]),
  trash: m.index.tree.trash.map((t) => t.id),
  pending: m.index.tree.pending.map((e) => e.id),
});

const converged = (...ms: Member[]) => new Set(ms.map((m) => JSON.stringify(shown(m)))).size === 1;

/** Plugs the admission rules of the index into a device, before the index can reach it. */
function guard(peer: TestPeer, url: AutomergeUrl): void {
  const { documentId } = parseAutomergeUrl(url);
  peer.adapter.validate(
    documentId,
    new IndexValidator(() => {
      try {
        return peer.repo.handles[documentId]?.doc() as A.Doc<IndexDoc> | undefined;
      } catch {
        return undefined;
      }
    }),
  );
}

function makeMember(
  peer: TestPeer,
  role: Role,
  handle: ConstructorParameters<typeof ProjectIndex>[0],
) {
  const m: Member = {
    peer,
    role,
    index: new ProjectIndex(handle, {
      me: () => ({ id: peer.id, name: peer.id.toUpperCase() }),
      role: () => m.role,
      createText: (text) => peer.repo.create<Text>({ text }).url,
      textOf: (u) => {
        const { documentId } = parseAutomergeUrl(u as AutomergeUrl);
        return peer.repo.handles[documentId]?.doc()?.text as string | undefined;
      },
    }),
  };
  return m;
}

async function join(peer: TestPeer, role: Role, url: AutomergeUrl): Promise<Member> {
  guard(peer, url);
  try {
    return makeMember(peer, role, await peer.repo.find<IndexDoc>(url));
  } catch (err) {
    throw new Error(`${String(err)} ${JSON.stringify(peer.adapter.coverageStats)}`);
  }
}

/** A, B and C; A and B are linked, B and C are, A and C never are (X-19). */
async function line(roles: [Role, Role, Role] = ["admin", "writer", "writer"]) {
  const net = new SimNet(7);
  const [pa, pb, pc] = ["a", "b", "c"].map((id) => kit.peer(net, id)) as [
    TestPeer,
    TestPeer,
    TestPeer,
  ];
  ["a", "b", "c"].forEach((id, i) => {
    net.setRole(id, roles[i] as Role);
  });
  // B learns the project from A first; C joins through B later, so a peer that does not have the
  // document yet is never the one B asks first.
  net.link("a", "b");
  await until(() => pa.connected.size === 1 && pb.connected.size === 1);
  net.announceRoles();

  const root = pa.repo.create<IndexDoc>({ text: "" });
  guard(pa, root.url);
  const a = makeMember(pa, roles[0], root);
  a.index.ensureMigrated();
  const b = await join(pb, roles[1], root.url);
  net.link("b", "c");
  await until(() => pb.connected.size === 2 && pc.connected.size === 1);
  const c = await join(pc, roles[2], root.url);
  await until(() => c.index.tree.byPath.has("main.typ"), 10_000, "C did not get the index");
  return { net, a, b, c };
}

describe("X-19 three people, A and B linked, B and C linked, A and C never", () => {
  it("the index reaches everyone, and every change travels through B", async () => {
    const { a, b, c } = await line();
    expect(converged(a, b, c)).toBe(true);

    expect(c.index.createFile(null, "from-c.md", "bonjour").ok).toBe(true);
    await until(() => a.index.tree.byPath.has("from-c.md"), 10_000, "A did not see C's file");

    // Concurrent: A renames it, B makes a folder, C makes a file.
    const id = a.index.tree.byPath.get("from-c.md")?.id as string;
    a.index.rename(id, "renamed.md");
    b.index.createFolder(null, "docs");
    c.index.createFile(null, "c2.typ");
    await until(
      () => converged(a, b, c) && a.index.tree.entries.length === 4,
      10_000,
      "no convergence",
    );
    expect(a.index.tree.entries.map((e) => e.path)).toEqual([
      "c2.typ",
      "docs",
      "main.typ",
      "renamed.md",
    ]);
  });

  it("the text of a file follows the same road: C reads what A wrote and A reads what C adds", async () => {
    const { a, c } = await line();
    const made = a.index.createFile(null, "n.md", "hello");
    const entry = a.index.tree.byId.get((made as { ids: string[] }).ids[0] as string);
    await until(() => c.index.tree.byPath.has("n.md"), 10_000);
    const hc = await c.peer.repo.find<Text>(entry?.url as AutomergeUrl);
    expect(hc.doc()?.text).toBe("hello");
    hc.change((d) => A.splice(d, ["text"], 5, 0, " world"));
    const { documentId } = parseAutomergeUrl(entry?.url as AutomergeUrl);
    await until(
      () => a.peer.repo.handles[documentId]?.doc()?.text === "hello world",
      10_000,
      "A did not get the typing",
    );
  });
});

describe("PRJ-10 FM-14 what a forging writer cannot do", () => {
  it("a name written by hand reaches nobody", async () => {
    const { net, a, b, c } = await line();
    const made = b.index.createFile(null, "ok.md", "x");
    const id = (made as { ids: string[] }).ids[0] as string;
    await until(() => a.index.tree.byPath.has("ok.md") && c.index.tree.byPath.has("ok.md"), 10_000);
    // B skips its own client.
    for (const name of ["../../etc/passwd", "CON", "a\\b"]) {
      b.index.handle.change((d) => {
        node(d, id).name = imm(name);
      });
      await new Promise((r) => setTimeout(r, 100));
    }
    await net.settle();
    expect(a.index.tree.byId.get(id)?.name).toBe("ok.md");
    expect(c.index.tree.byId.get(id)?.name).toBe("ok.md");
    expect(a.peer.adapter.coverageStats.removed).toBeGreaterThan(0);
  });

  it("FM-14 a direct deletion forged by a writer is not applied, while an admin's is", async () => {
    const { net, a, b, c } = await line();
    const made = b.index.createFile(null, "ch.typ", "x");
    const id = (made as { ids: string[] }).ids[0] as string;
    await until(
      () => a.index.tree.byPath.has("ch.typ") && c.index.tree.byPath.has("ch.typ"),
      10_000,
    );

    b.index.handle.change((d) => {
      node(d, id).trashed = { by: imm("b"), at: 1 };
    });
    await new Promise((r) => setTimeout(r, 200));
    await net.settle();
    expect(a.index.tree.byId.has(id)).toBe(true);
    expect(c.index.tree.byId.has(id)).toBe(true);

    // The honest road, with another writer (a forger's later changes hang on its forged one, and
    // go nowhere): C asks, A, an admin, accepts, and the file is in everyone's trash.
    const second = (c.index.createFile(null, "ch2.typ", "y") as { ids: string[] }).ids[0] as string;
    await until(() => a.index.tree.byPath.has("ch2.typ"), 10_000, "A did not get C's file");
    c.index.remove([second]);
    await until(() => a.index.tree.pending.length === 1, 10_000, "A did not see the request");
    expect(c.index.tree.pending.map((e) => e.id)).toEqual([second]);
    expect(a.index.accept([second]).ok).toBe(true);
    await until(() => c.index.tree.trash.some((t) => t.id === second), 10_000, "C lacks the trash");
    expect(c.index.tree.byId.has(second)).toBe(false);
  });

  it("FM-08 two admins, one accepts and one refuses: the refusal wins everywhere", async () => {
    const { net, a, b, c } = await line(["admin", "admin", "writer"]);
    const id = (c.index.createFile(null, "d.typ", "x") as { ids: string[] }).ids[0] as string;
    await until(() => a.index.tree.byId.has(id) && b.index.tree.byId.has(id), 10_000);
    c.index.remove([id]);
    await until(
      () => a.index.tree.pending.length === 1 && b.index.tree.pending.length === 1,
      10_000,
    );
    net.partition("a", "b");
    a.index.accept([id]);
    b.index.refuse([id]);
    net.link("a", "b");
    await until(() => converged(a, b, c), 10_000, "no convergence");
    for (const m of [a, b, c]) {
      expect(m.index.tree.byId.has(id)).toBe(true);
      expect(m.index.tree.trash).toHaveLength(0);
    }
  });

  it("PRJ-12 a reader receives everything and changes nothing", async () => {
    const { net, a, c } = await line(["admin", "writer", "reader"]);
    expect(c.index.createFile(null, "x.md")).toMatchObject({
      ok: false,
      error: { code: "forbidden" },
    });
    // Even by hand, nothing it writes goes out.
    const id = a.index.tree.byPath.get("main.typ")?.id as string;
    c.index.handle.change((d) => {
      node(d, id).name = imm("hacked.typ");
    });
    await net.settle();
    expect(a.index.tree.byId.get(id)?.name).toBe("main.typ");
  });
});
