// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { type DocHandle, Repo } from "@automerge/automerge-repo";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExternalSync, type Notice, type SyncStorage } from "./externalSync";
import { ExternalEditError, NotTextError } from "./GoStorageAdapter";
import { type Projection, type ProjectionSink, startProjection } from "./projection";
import type { ExternalPolicy } from "./settings.svelte";

type Doc = { text: string };

/** The project folder as the store stream shows it: one file, refused after an outside edit. */
class Disk implements SyncStorage, ProjectionSink {
  text: string | undefined;
  readError: Error | undefined;
  writeError: Error | undefined;
  /** Edited outside and not acknowledged: Go refuses to write. */
  edited = false;
  writes: string[] = [];
  acks = 0;
  constructor(text?: string) {
    this.text = text;
  }
  async readProjection(): Promise<string | undefined> {
    if (this.readError) throw this.readError;
    return this.text;
  }
  async acknowledge(): Promise<void> {
    this.edited = false;
    this.acks++;
  }
  async writeProjection(_name: string, text: string): Promise<void> {
    if (this.writeError) throw this.writeError;
    if (this.edited) throw new ExternalEditError();
    this.text = text;
    this.writes.push(text);
  }
  /** A program writes the file. */
  edit(text: string | undefined): void {
    this.text = text;
    this.edited = true;
  }
}

interface Rig {
  h: DocHandle<Doc>;
  disk: Disk;
  sync: ExternalSync;
  projection: Projection;
  notices: (Notice | null)[];
  dirty: boolean[];
  errors: unknown[];
  settings: { policy: ExternalPolicy; manual: boolean; delay: number };
  /** An outside edit, and Go's notice for it. */
  outside(text: string | undefined): Promise<void>;
  type(index: number, s: string): void;
  last(): Notice | null | undefined;
}

const repo = new Repo();

async function rig(
  text: string,
  opts: Partial<Rig["settings"]> & { skip?: (r: { from: number; to: number }) => boolean } = {},
  existing?: { h: DocHandle<Doc>; disk: Disk },
): Promise<Rig> {
  const h = existing?.h ?? repo.create<Doc>({ text });
  const disk = existing?.disk ?? new Disk(opts.manual ? text : undefined);
  const settings = { policy: "import" as ExternalPolicy, manual: false, delay: 1000, ...opts };
  const notices: (Notice | null)[] = [];
  const dirty: boolean[] = [];
  const errors: unknown[] = [];
  let projection: Projection | undefined;
  const sync = new ExternalSync({
    handle: h,
    name: "main.typ",
    storage: disk,
    projection: () => projection,
    policy: () => settings.policy,
    manual: () => settings.manual,
    onNotice: (n) => notices.push(n),
    notText: (err) => (err instanceof NotTextError ? err : undefined),
    skip: opts.skip ? (_name, range) => opts.skip?.(range) ?? false : undefined,
  });
  projection = startProjection(h, disk, {
    schedule: () => (settings.manual ? undefined : settings.delay),
    known: settings.manual ? disk.text : undefined,
    onWritten: (w) => sync.recordWritten(w),
    onDirty: (d) => dirty.push(d),
    onError: (e) => errors.push(e),
    retryable: (e) => !(e instanceof ExternalEditError),
    retryMs: 5000,
  });
  await vi.advanceTimersByTimeAsync(0);
  const r: Rig = {
    h,
    disk,
    sync,
    projection,
    notices,
    dirty,
    errors,
    settings,
    async outside(t) {
      disk.edit(t);
      sync.notify("main.typ");
      await vi.advanceTimersByTimeAsync(250);
    },
    type: (index, s) => {
      h.change((d) => A.splice(d, ["text"], index, 0, s));
    },
    last: () => notices.at(-1),
  };
  return r;
}

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe("saving (SAV-*)", () => {
  it("SAV-01 auto: nothing is written while typing, one write after the pause", async () => {
    const r = await rig("");
    r.disk.writes.length = 0;
    r.type(0, "a");
    await vi.advanceTimersByTimeAsync(600);
    r.type(1, "b");
    await vi.advanceTimersByTimeAsync(600);
    expect(r.disk.writes).toEqual([]); // still typing: each key restarts the wait
    await vi.advanceTimersByTimeAsync(400);
    expect(r.disk.writes).toEqual(["ab"]);
    await vi.advanceTimersByTimeAsync(5000);
    expect(r.disk.writes).toEqual(["ab"]);
  });

  it("SAV-02 manual: the file is unchanged and the document dirty until Ctrl+S", async () => {
    const r = await rig("one", { manual: true });
    r.type(3, " two");
    await vi.advanceTimersByTimeAsync(10_000);
    expect(r.disk.text).toBe("one");
    expect(r.projection.dirty()).toBe(true);
    expect(r.dirty.at(-1)).toBe(true);
    await r.projection.flush();
    expect(r.disk.text).toBe("one two");
    expect(r.projection.dirty()).toBe(false);
    expect(r.dirty.at(-1)).toBe(false);
  });

  it("SAV-03 manual: closing with unwritten changes loses nothing and reopening is still unsaved", async () => {
    const r = await rig("one", { manual: true });
    r.type(3, " two");
    r.projection.stop(); // the window closes: no dialog, no write
    r.sync.stop();
    expect(r.disk.text).toBe("one");
    // The history is kept by the store; the project reopens on the same document and file.
    const again = await rig("", { manual: true }, { h: r.h, disk: r.disk });
    expect(again.h.doc().text).toBe("one two");
    expect(again.projection.dirty()).toBe(true);
    expect(again.dirty.at(-1)).toBe(true);
    expect(r.disk.writes).toEqual([]);
  });

  it("SAV-04 manual to auto writes at once when the document is dirty", async () => {
    const r = await rig("one", { manual: true });
    r.type(3, "!");
    expect(r.disk.text).toBe("one");
    r.settings.manual = false;
    r.projection.reschedule();
    await vi.advanceTimersByTimeAsync(0);
    expect(r.disk.text).toBe("one!");
  });

  it("SAV-04 auto to manual drops the write that was waiting", async () => {
    const r = await rig("one");
    await vi.advanceTimersByTimeAsync(0);
    r.type(3, "!");
    r.settings.manual = true;
    r.projection.reschedule();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(r.disk.text).toBe("one");
  });

  it("SAV-05 a refused write is reported, tried again, and nothing is lost", async () => {
    const r = await rig("one");
    r.disk.writeError = new Error("no space left on device");
    r.type(3, "!");
    await vi.advanceTimersByTimeAsync(1100);
    expect(r.errors).toHaveLength(1);
    expect(r.h.doc().text).toBe("one!");
    r.disk.writeError = undefined;
    await vi.advanceTimersByTimeAsync(5100);
    expect(r.disk.text).toBe("one!");
  });

  it("SAV-06 manual: a remote person's change shows in the text; the file waits for Ctrl+S", async () => {
    const r = await rig("shared", { manual: true });
    const remote = A.change(A.clone(r.h.doc()), (d) => A.splice(d, ["text"], 6, 0, " + them"));
    r.h.update((d) => A.merge(d, remote));
    await vi.advanceTimersByTimeAsync(10_000);
    expect(r.h.doc().text).toBe("shared + them");
    expect(r.disk.text).toBe("shared");
    await r.projection.flush();
    expect(r.disk.text).toBe("shared + them");
  });
});

describe("outside changes (EXT-*)", () => {
  it("EXT-01 a clean document takes the change, marks the lines, and Ctrl+Z takes it back in one step", async () => {
    const r = await rig("a\nb\nc\n");
    await r.outside("a\nB\nc\nd\n");
    expect(r.h.doc().text).toBe("a\nB\nc\nd\n");
    const n = r.last();
    expect(n).toMatchObject({ kind: "merged", name: "main.typ", lines: 2 });
    expect(n?.kind === "merged" && n.ranges.length).toBeGreaterThan(0);
    expect(r.sync.canKeyUndo).toBe(true);
    r.sync.undo();
    expect(r.h.doc().text).toBe("a\nb\nc\n");
    expect(r.last()).toBeNull();
  });

  it("UX-S9-2 a change that only removed lines can be viewed and undone: its place is marked", async () => {
    const r = await rig("a\nb\nc\n");
    await r.outside("a\nc\n");
    const n = r.last();
    expect(n?.kind === "merged" && n.ranges.length).toBeGreaterThan(0);
    r.sync.undo();
    expect(r.h.doc().text).toBe("a\nb\nc\n");
  });

  it("EXT-01 typing since turns Ctrl+Z back into the editor's own undo", async () => {
    const r = await rig("a\nb\n");
    await r.outside("a\nB\n");
    r.type(0, "x");
    expect(r.sync.canKeyUndo).toBe(false);
  });

  it("EXT-02 a change outside while I type elsewhere keeps both", async () => {
    const r = await rig("one\ntwo\nthree\n");
    await vi.advanceTimersByTimeAsync(1100);
    r.disk.edit("one\ntwo\nTHREE\n");
    r.type(0, "MINE "); // typed before the notice is handled
    r.sync.notify("main.typ");
    await vi.advanceTimersByTimeAsync(250);
    expect(r.h.doc().text).toBe("MINE one\ntwo\nTHREE\n");
  });

  it("EXT-03 the same spot: both insertions are present", async () => {
    const r = await rig("start\n");
    await vi.advanceTimersByTimeAsync(1100);
    r.disk.edit("start theirs\n");
    r.type(5, " mine");
    r.sync.notify("main.typ");
    await vi.advanceTimersByTimeAsync(250);
    const text = r.h.doc().text;
    expect(text).toContain("theirs");
    expect(text).toContain("mine");
    expect(r.last()).toMatchObject({ kind: "merged" });
  });

  it("EXT-04 manual with unwritten text: the change is integrated and Ctrl+S writes the union", async () => {
    const r = await rig("one\ntwo\n", { manual: true });
    r.type(0, "MINE ");
    await r.outside("one\nTWO\n");
    expect(r.h.doc().text).toBe("MINE one\nTWO\n");
    expect(r.disk.text).toBe("one\nTWO\n"); // not written until asked
    expect(r.projection.dirty()).toBe(true);
    await r.projection.flush();
    expect(r.disk.text).toBe("MINE one\nTWO\n");
  });

  it("EXT-06 the echo of a write of ours is not an outside change", async () => {
    const r = await rig("same");
    await vi.advanceTimersByTimeAsync(1100);
    expect(r.disk.text).toBe("same");
    r.disk.edited = true; // Go reported it, though the content is what we wrote
    r.sync.notify("main.typ");
    await vi.advanceTimersByTimeAsync(250);
    expect(r.notices.filter((n) => n?.kind === "merged")).toEqual([]);
    expect(r.h.doc().text).toBe("same");
    expect(r.disk.edited).toBe(false); // acknowledged: Inkwell may write again
  });

  it("EXT-07 fifty writes in a second are one integration", async () => {
    const r = await rig("start\n");
    await vi.advanceTimersByTimeAsync(1100);
    for (let i = 1; i <= 50; i++) {
      r.disk.edit(`start\nrun ${i}\n`);
      r.sync.notify("main.typ");
      await vi.advanceTimersByTimeAsync(20);
    }
    await vi.advanceTimersByTimeAsync(300);
    expect(r.notices.filter((n) => n?.kind === "merged")).toHaveLength(1);
    expect(r.h.doc().text).toBe("start\nrun 50\n");
  });

  it("EXT-08 a file deleted outside: the text stays and is not written back until asked", async () => {
    const r = await rig("keep me\n");
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside(undefined);
    expect(r.last()).toEqual({ kind: "deleted", name: "main.typ" });
    expect(r.h.doc().text).toBe("keep me\n");
    r.type(0, "more ");
    await vi.advanceTimersByTimeAsync(3000);
    expect(r.disk.text).toBeUndefined(); // typing does not bring the file back
    await r.sync.recreate();
    expect(r.disk.text).toBe("more keep me\n");
    expect(r.last()).toBeNull();
  });

  it("EXT-08 the banner goes when the file comes back", async () => {
    const r = await rig("keep me\n");
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside(undefined);
    expect(r.last()).toMatchObject({ kind: "deleted" });
    await r.outside("keep me\n"); // restored by the program, same content
    expect(r.last()).toBeNull();
  });

  it.todo(
    "EXT-09 a file renamed outside: offer to follow the new name (needs the rename plumbing of the file manager, B5)",
  );

  it("EXT-10 a file rewritten in another encoding is read as text and integrated unchanged in meaning", async () => {
    // Go decodes Windows-1252 (and keeps it when writing); the document sees plain UTF-8.
    const r = await rig("caf\n");
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside("café\n");
    expect(r.h.doc().text).toBe("café\n");
    expect(r.last()).toMatchObject({ kind: "merged" });
  });

  it("EXT-10 bytes that are not valid UTF-8 are not integrated and the line is given", async () => {
    const r = await rig("text\n");
    await vi.advanceTimersByTimeAsync(1100);
    r.disk.readError = new NotTextError("invalid-utf8", 7);
    await r.outside("ignored");
    expect(r.last()).toEqual({ kind: "invalid", name: "main.typ", line: 7 });
    expect(r.h.doc().text).toBe("text\n");
  });

  it("EXT-11 a file rewritten as binary is not integrated; keeping ours writes it again", async () => {
    const r = await rig("text\n");
    await vi.advanceTimersByTimeAsync(1100);
    r.disk.readError = new NotTextError("binary");
    await r.outside("\0\0");
    expect(r.last()).toEqual({ kind: "binary", name: "main.typ" });
    expect(r.h.doc().text).toBe("text\n");
    r.disk.readError = undefined;
    await r.sync.keepOurs();
    expect(r.disk.text).toBe("text\n");
    expect(r.last()).toBeNull();
  });

  it("EXT-12 'Annuler' after a peer typed removes only the integration", async () => {
    const r = await rig("alpha\nbeta\n");
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside("alpha\nbeta, outside\n");
    r.type(0, "peer ");
    r.sync.undo();
    expect(r.h.doc().text).toBe("peer alpha\nbeta\n");
  });

  it("EXT-13 'ask': nothing changes until the person answers; Integrate takes it", async () => {
    const r = await rig("a\nb\n", { policy: "ask" });
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside("a\nB\n");
    expect(r.last()).toEqual({ kind: "ask", name: "main.typ" });
    expect(r.h.doc().text).toBe("a\nb\n");
    await r.sync.integrateAsked();
    expect(r.h.doc().text).toBe("a\nB\n");
    expect(r.last()).toMatchObject({ kind: "merged" });
  });

  it("EXT-13 'ask': keeping Inkwell's version writes the file again", async () => {
    const r = await rig("a\nb\n", { policy: "ask" });
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside("a\nCHANGED\n");
    await r.sync.keepOurs();
    expect(r.disk.text).toBe("a\nb\n");
    expect(r.h.doc().text).toBe("a\nb\n");
    expect(r.last()).toBeNull();
  });

  it("EXT-13 'keep' never integrates", async () => {
    const r = await rig("a\n", { policy: "keep" });
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside("b\n");
    expect(r.h.doc().text).toBe("a\n");
    expect(r.disk.text).toBe("a\n");
  });

  it("the point of agreement survives a restart: a change made while closed is merged, not replaced", async () => {
    const store = new Map<string, string>();
    vi.stubGlobal("localStorage", {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
    });
    try {
      const h = repo.create<Doc>({ text: "one\ntwo\n" });
      const disk = new Disk();
      const key = "inkwell.sync.test";
      const mk = (): ExternalSync => {
        let projection: Projection | undefined;
        const sync = new ExternalSync({
          handle: h,
          name: "main.typ",
          storage: disk,
          projection: () => projection,
          policy: () => "import",
          manual: () => false,
          onNotice: () => {},
          persistKey: key,
        });
        projection = startProjection(h, disk, {
          delayMs: 10,
          onWritten: (w) => sync.recordWritten(w),
        });
        return sync;
      };
      const first = mk();
      await vi.advanceTimersByTimeAsync(50);
      first.stop();
      // Closed. Meanwhile I typed in another window of the document, and a program edited the file.
      h.change((d) => A.splice(d, ["text"], 0, 0, "MINE "));
      disk.edit("one\nTWO\n");
      const second = mk();
      second.notify("main.typ");
      await vi.advanceTimersByTimeAsync(300);
      expect(h.doc().text).toBe("MINE one\nTWO\n");
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

describe("locked zones (X-15)", () => {
  it("X-15 a change in a locked zone is set aside and told, the rest is integrated", async () => {
    const r = await rig("title\nlocked\nbody\n", {
      skip: (range) => range.from < 13 && range.to > 6,
    });
    await vi.advanceTimersByTimeAsync(1100);
    await r.outside("title\nLOCKED CHANGED\nbody edited\n");
    expect(r.h.doc().text).toBe("title\nlocked\nbody edited\n");
    const n = r.last();
    expect(n?.kind === "merged" && n.skipped).toEqual([
      { from: 6, to: 13, text: "LOCKED CHANGED\n" },
    ]);
  });
});

describe("two machines (X-16)", () => {
  it("X-16 the same file changed outside on both machines converges, with no write loop", async () => {
    const a = await rig("l1\nl2\nl3\n");
    const b = await rig("", {}, { h: repo.import<Doc>(A.save(a.h.doc())), disk: new Disk() });
    await vi.advanceTimersByTimeAsync(1100);
    // A program edits line 1 on one machine and line 3 on the other.
    await a.outside("L1\nl2\nl3\n");
    await b.outside("l1\nl2\nL3\n");
    // The documents sync (the network), both ways, until quiet.
    const exchange = () => {
      a.h.update((d) => A.merge(d, b.h.doc()));
      b.h.update((d) => A.merge(d, a.h.doc()));
    };
    exchange();
    await vi.advanceTimersByTimeAsync(1100);
    exchange();
    await vi.advanceTimersByTimeAsync(1100);
    expect(a.h.doc().text).toBe("L1\nl2\nL3\n");
    expect(b.h.doc().text).toBe("L1\nl2\nL3\n");
    expect(a.disk.text).toBe("L1\nl2\nL3\n");
    expect(b.disk.text).toBe("L1\nl2\nL3\n");
    // Quiet: nothing more is written on either disk, and nothing more is reported.
    const writes = [a.disk.writes.length, b.disk.writes.length];
    const notices = [a.notices.length, b.notices.length];
    await vi.advanceTimersByTimeAsync(20_000);
    expect([a.disk.writes.length, b.disk.writes.length]).toEqual(writes);
    expect([a.notices.length, b.notices.length]).toEqual(notices);
  });
});
