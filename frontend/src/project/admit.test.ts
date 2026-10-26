import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { Coverage } from "../net/coverage";
import { IndexValidator } from "./admit";
import { imm, plain } from "./model";
import * as ops from "./ops";
import {
  alice,
  apply,
  bob,
  ctx,
  HASH,
  type Index,
  idOf,
  newProject,
  textUrl,
  view,
} from "./testkit";

/** Judges the change `forge` makes to `base`, as a replica that holds `base` would. */
function judge(base: Index, forge: (d: Index) => Index, adminCovered = false): string | undefined {
  const forged = forge(A.clone(base));
  const bytes = A.getLastLocalChange(forged);
  if (!bytes) throw new Error("no change");
  const validator = new IndexValidator(() => A.clone(base));
  return validator.check(bytes, () => adminCovered);
}

const forge = (doc: Index, fn: (d: never) => void): Index =>
  A.change(doc, fn as never) as unknown as Index;

// biome-ignore lint/suspicious/noExplicitAny: a draft of the index, written the way a forger would
type Loose = Record<string, any>;
const loose = (d: never): Loose => d as unknown as Loose;

const edit = (fn: (d: Loose) => void) => (doc: Index) => A.change(doc, fn as never);

function project() {
  let doc = newProject();
  doc = apply(
    doc,
    ops.create(doc, view(doc), ctx(bob), [
      { type: "text", name: "ch1.typ", parent: null, url: textUrl() },
      { type: "dir", name: "sub", parent: null },
      { type: "blob", name: "fig.png", parent: null, hash: HASH, size: 3 },
    ]),
  );
  return doc;
}

describe("PRJ-10 PRJ-15 a forged name is refused by every replica", () => {
  const hostile = [
    "../x",
    "/etc/passwd",
    "a\\b",
    "CON",
    "NUL.txt",
    "a\u0000b",
    "x".repeat(300),
    ".git",
    "INKWEL~1",
    ".inkwe‌ll",
    "a ",
  ];

  for (const name of hostile) {
    it(`a rename to ${JSON.stringify(name.slice(0, 20))}`, () => {
      const base = project();
      const id = idOf(base, "ch1.typ");
      const refusal = judge(
        base,
        edit((d) => {
          d.nodes[id].name = imm(name);
        }),
      );
      expect(refusal).toBe("invalid name");
    });

    it(`a new file named ${JSON.stringify(name.slice(0, 20))}`, () => {
      const base = project();
      const refusal = judge(
        base,
        edit((d) => {
          d.nodes["f".repeat(32)] = {
            type: imm("text"),
            name: imm(name),
            parent: null,
            url: imm(textUrl()),
            by: imm("bob"),
            at: 1,
          };
        }),
      );
      expect(refusal).toBeDefined();
    });
  }

  it("a plain string name (a text object) is judged on its value too", () => {
    const base = project();
    const id = idOf(base, "ch1.typ");
    const refusal = judge(
      base,
      edit((d) => {
        d.nodes[id].name = "bad/name";
      }),
    );
    expect(refusal).toBe("invalid name");
  });

  it("refuses a bad url, a bad hash, an unknown field, a bad parent", () => {
    const base = project();
    const f = idOf(base, "fig.png");
    const t = idOf(base, "ch1.typ");
    const cases: Record<string, (d: never) => void> = {
      hash: (d: never) => {
        loose(d).nodes[f].hash = imm("blake3:zz");
      },
      field: (d: never) => {
        loose(d).nodes[t].secret = imm("x");
      },
      parent: (d: never) => {
        loose(d).nodes[t].parent = imm("not-an-id");
      },
      size: (d: never) => {
        loose(d).nodes[f].size = -4;
      },
    };
    for (const [what, fn] of Object.entries(cases)) {
      expect(
        judge(base, (doc) => A.change(doc, fn as never)),
        what,
      ).toBeDefined();
    }
  });

  it("a file's url, type and author never change", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    for (const field of ["url", "type", "by"]) {
      const refusal = judge(
        base,
        edit((d) => {
          d.nodes[t][field] = imm(
            field === "url" ? textUrl() : field === "type" ? "dir" : "mallory",
          );
        }),
      );
      expect(refusal, field).toMatch(/never changes|url on a non-text/);
    }
  });

  it("what an honest client writes is admitted", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    for (const r of [
      ops.rename(view(base), ctx(bob), t, "intro.typ"),
      ops.move(view(base), ctx(bob), [t], idOf(base, "sub")),
      ops.replaceBlob(view(base), ctx(bob), idOf(base, "fig.png"), {
        hash: `blake3:${"cd".repeat(32)}`,
        size: 5,
      }),
      ops.remove(base, view(base), ctx(bob), [t]), // a request
    ]) {
      if (!r.ok) throw new Error("refused");
      const refusal = judge(base, (d) =>
        A.change(d, { message: "x" }, (draft) => r.plan.apply(draft as never)),
      );
      expect(refusal).toBeUndefined();
    }
  });
});

describe("FM-13 FM-14 FM-16 FM-18 rules of authority", () => {
  it("FM-14 a writer forges a direct deletion under approval: refused; an admin's is admitted", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const forge = edit((d) => {
      d.nodes[t].trashed = { by: imm("bob"), at: 5 };
    });
    expect(judge(base, forge, false)).toBe("hold: trashing a file needs an admin");
    expect(judge(base, forge, true)).toBeUndefined();
  });

  it("FM-15 with the rule at free, a writer's direct deletion is admitted", () => {
    let base = project();
    base = apply(base, ops.setDeletionMode(base, ctx(alice), "free"));
    const t = idOf(base, "ch1.typ");
    const refusal = judge(
      base,
      edit((d) => {
        d.nodes[t].trashed = { by: imm("bob"), at: 5 };
      }),
    );
    expect(refusal).toBeUndefined();
  });

  it("a writer cannot lift a deletion request, decide, purge or pick the main file", () => {
    let base = project();
    const t = idOf(base, "ch1.typ");
    base = apply(base, ops.remove(base, view(base), ctx(bob), [t]));
    const forgeries: Record<string, (d: never) => void> = {
      lift: (d: never) => {
        delete loose(d).nodes[t].deletion;
      },
      decide: (d: never) => {
        loose(d).decisions[t] = {
          by: imm("bob"),
          at: 1,
          req: imm("a".repeat(32)),
          verdict: imm("refused"),
        };
      },
      purge: (d: never) => {
        delete loose(d).nodes[t];
      },
      main: (d: never) => {
        loose(d).main = imm(t);
      },
    };
    for (const [what, fn] of Object.entries(forgeries)) {
      expect(
        judge(base, (doc) => A.change(doc, fn as never)),
        what,
      ).toMatch(/needs an admin/);
      expect(
        judge(base, (doc) => A.change(doc, fn as never), true),
        `${what} by an admin`,
      ).toBeUndefined();
    }
  });

  it("FM-16 the deletion rule is an admin's to change", () => {
    const base = project();
    const forge = edit((d) => {
      d.settings = { deletion: imm("free") };
    });
    expect(judge(base, forge, false)).toBe("hold: changing the deletion rule needs an admin");
    expect(judge(base, forge, true)).toBeUndefined();
  });

  it("FM-18 even an admin cannot put the main file in the trash", () => {
    const base = project();
    const main = idOf(base, "main.typ");
    const refusal = judge(
      base,
      edit((d) => {
        d.nodes[main].trashed = { by: imm("alice"), at: 5 };
      }),
      true,
    );
    expect(refusal).toBe("the main file cannot be trashed");
  });

  it("a request is any writer's to make", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const r = ops.remove(base, view(base), ctx(bob), [t]);
    expect(r.ok).toBe(true);
    expect(
      judge(base, (d) =>
        A.change(d, (draft) => {
          if (r.ok) r.plan.apply(draft as never);
        }),
      ),
    ).toBeUndefined();
  });
});

describe("the validator keeps in step with the document", () => {
  it("typing in the text of the main file is admitted without a copy of the index", () => {
    const base = project();
    const typed = A.change(A.clone(base), (d) => A.splice(d, ["text"], 0, 0, "bonjour"));
    const validator = new IndexValidator(() => A.clone(base));
    const bytes = A.getLastLocalChange(typed) as Uint8Array;
    expect(validator.check(bytes, () => false)).toBeUndefined();
  });

  it("a change whose dependencies are unknown waits, it is not refused", () => {
    const base = project();
    const first = A.change(A.clone(base), (d) => A.splice(d, ["text"], 0, 0, "a"));
    const second = A.change(A.clone(first), (d) => A.splice(d, ["text"], 1, 0, "b"));
    const validator = new IndexValidator(() => A.clone(base));
    expect(validator.check(A.getLastLocalChange(second) as Uint8Array, () => false)).toBe(
      "hold: dependencies unknown",
    );
  });

  it("a whole document is judged on its content", () => {
    const base = project();
    const bad = forge(A.clone(base), (d: never) => {
      loose(d).nodes[idOf(base, "ch1.typ")].name = imm("x/y");
    });
    expect(new IndexValidator(() => undefined).checkDocument(bad, () => false)).toBe(
      "invalid name",
    );
    expect(
      new IndexValidator(() => undefined).checkDocument(project(), () => false),
    ).toBeUndefined();
    expect(plain(bad).nodes).toBeDefined();
  });
});

describe("the admission rules are part of write enforcement", () => {
  /** A sync message carrying the new changes of `doc` over `base`. */
  const message = (doc: Index, base: Index) =>
    A.encodeSyncMessage({
      heads: A.getHeads(doc),
      need: [],
      have: [],
      changes: A.getChanges(base, doc),
    });

  const hashes = (data: Uint8Array | undefined) =>
    data ? A.decodeSyncMessage(data).changes.map((c) => A.decodeChange(c).hash) : [];

  it("FM-13 a forged name from an attested writer is cut like a change nobody vouches for", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const forged = forge(A.clone(base), (d: never) => {
      loose(d).nodes[t].name = imm("../../etc");
    });
    const cov = new Coverage();
    cov.setValidator("doc", new IndexValidator(() => A.clone(base)));
    cov.attested("bob", "doc", A.getHeads(base));
    cov.attested("bob", "doc", A.getHeads(forged)); // the forger vouches for its own change
    const out = cov.filter("doc", "bob", message(forged, base));
    expect(hashes(out)).toEqual([]);
    expect(cov.stats.removed).toBe(1);
  });

  it("an honest rename passes through", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const honest = apply(A.clone(base), ops.rename(view(base), ctx(bob), t, "intro.typ"));
    const cov = new Coverage();
    cov.setValidator("doc", new IndexValidator(() => A.clone(base)));
    cov.attested("bob", "doc", A.getHeads(honest));
    const msg = message(honest, base);
    expect(cov.filter("doc", "bob", msg)).toBe(msg);
  });

  it("FM-14 a writer's direct deletion needs an admin who vouches for it", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const forged = forge(A.clone(base), (d: never) => {
      loose(d).nodes[t].trashed = { by: imm("bob"), at: 1 };
    });
    const msg = message(forged, base);

    const alone = new Coverage();
    alone.setValidator("doc", new IndexValidator(() => A.clone(base)));
    alone.setAdmins(["alice"]);
    alone.attested("bob", "doc", A.getHeads(forged));
    expect(hashes(alone.filter("doc", "bob", msg))).toEqual([]);

    // An admin who has this change in their heads vouches for it, and it goes through.
    const vouched = new Coverage();
    vouched.setValidator("doc", new IndexValidator(() => A.clone(base)));
    vouched.attested("alice", "doc", A.getHeads(forged));
    vouched.setAdmins(["alice"]); // the roles may arrive after the attestation
    expect(vouched.filter("doc", "bob", msg)).toBe(msg);
  });

  it("what an admin vouched for stays vouched for when they are demoted", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const trashed = forge(A.clone(base), (d: never) => {
      loose(d).nodes[t].trashed = { by: imm("alice"), at: 1 };
    });
    const cov = new Coverage();
    cov.setValidator("doc", new IndexValidator(() => A.clone(base)));
    cov.setAdmins(["alice"]);
    cov.attested("alice", "doc", A.getHeads(trashed));
    cov.setAdmins([]); // alice is demoted
    const msg = message(trashed, base);
    expect(cov.filter("doc", "alice", msg)).toBe(msg);
  });
});

describe("ADR 0027 the type chosen for a file (ftype)", () => {
  const retype = (value: unknown) => {
    const base = project();
    const id = idOf(base, "ch1.typ");
    return judge(
      base,
      edit((d) => {
        d.nodes[id].ftype = imm(value as string);
      }),
    );
  };

  it("accepts a short lowercase id", () => {
    expect(retype("json")).toBeUndefined();
  });

  it.each(["JSON", "a/b", "", "x".repeat(33), "../x"])("refuses %j", (bad) => {
    expect(retype(bad)).toBeDefined();
  });
});

describe("security audit: whole documents, stamps, trashed folders, caps", () => {
  const msgDoc = (doc: Index) =>
    A.encodeSyncMessage({ heads: A.getHeads(doc), need: [], have: [], changes: [A.save(doc)] });

  it("HIGH a whole-document chunk is judged change by change: a writer's forged deletion is cut", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const forged = forge(A.clone(base), (d: never) => {
      loose(d).nodes[t].trashed = { by: imm("bob"), at: 1 };
    });
    const cov = new Coverage();
    cov.setValidator("doc", new IndexValidator(() => A.clone(base)));
    cov.attested("bob", "doc", A.getHeads(forged));
    expect(cov.filter("doc", "bob", msgDoc(forged))).toBeUndefined();
    expect(cov.stats.dropped).toBe(1);
  });

  it("HIGH a whole document of admitted changes passes, and an admin's deletion in it too", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    const renamed = apply(A.clone(base), ops.rename(view(base), ctx(bob), t, "intro.typ"));
    const honest = apply(renamed, ops.remove(renamed, view(renamed), ctx(alice), [t]));
    const cov = new Coverage();
    cov.setValidator("doc", new IndexValidator(() => A.clone(base)));
    cov.setAdmins(["alice"]);
    cov.attested("alice", "doc", A.getHeads(honest));
    const msg = msgDoc(honest);
    expect(cov.filter("doc", "alice", msg)).toBe(msg);
  });

  it("MEDIUM a file is not moved into a trashed folder", () => {
    let base = project();
    const dir = idOf(base, "sub");
    const f = idOf(base, "ch1.typ");
    base = apply(base, ops.setDeletionMode(base, ctx(alice), "free"));
    base = apply(base, ops.remove(base, view(base), ctx(alice), [dir]));
    const into = edit((d) => {
      d.nodes[f].parent = imm(dir);
    });
    expect(judge(base, into)).toBe("a file is not moved into the trash");
  });

  it("MEDIUM the number of files is capped, in one change and in all", () => {
    const base = project();
    const many = (n: number, tag: string) => (doc: Index) =>
      A.change(doc, ((d: Loose) => {
        for (let i = 0; i < n; i++) {
          d.nodes[`${tag}${String(i).padStart(32 - tag.length, "0")}`] = {
            type: imm("dir"),
            name: imm(`${tag}${i}`),
            parent: null,
            by: imm("bob"),
            at: 1,
          };
        }
      }) as never);
    expect(judge(base, many(5001, "a"))).toBe("too many files in one change");
    expect(judge(base, many(5000, "a"))).toBeUndefined();
    const half = many(5000, "a")(A.clone(base));
    expect(judge(half, many(5000, "b"))).toBe("too many files");
  }, 120_000);
});

describe("every change an honest client makes is admitted, whatever came before", () => {
  it("a refusal, then the purge of the file it was about (it deletes the decision too)", () => {
    const base = project();
    const t = idOf(base, "ch1.typ");
    let doc = A.clone(base);
    const steps: ((d: Index) => Index)[] = [
      (d) => apply(d, ops.remove(d, view(d), ctx(bob), [t])), // a request
      (d) => apply(d, ops.refuse(view(d), ctx(alice), [t])), // the decision
      (d) => apply(d, ops.setDeletionMode(d, ctx(alice), "free")),
      (d) => apply(d, ops.remove(d, view(d), ctx(alice), [t])), // in the trash
      (d) => apply(d, ops.purge(d, view(d), ctx(alice), [t])), // gone, decision and all
    ];
    const validator = new IndexValidator(() => base);
    for (const step of steps) {
      const before = doc;
      doc = step(doc);
      for (const change of A.getChanges(before, doc)) {
        expect(validator.check(change, () => true)).toBeUndefined();
      }
    }
  });
});
