import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { entryDiff } from "../../history/diffview";
import type { HDoc } from "../../history/types";
import { workspace } from "../../workspace/workspace.svelte";
import { cleanup, render } from "../testkit.svelte";
import DiffView from "./DiffView.svelte";
import HistoryDrawer from "./HistoryDrawer.svelte";
import { history } from "./history.svelte";

const members = vi.hoisted(() => ({ role: "writer" }));
vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {
    Members: async () => [
      { name: "Moi", role: members.role, self: true, devices: [{ peer: "me" }] },
    ],
  },
}));

const labels = (host: HTMLElement) =>
  [...host.querySelectorAll("button")].map((b) => b.textContent?.trim());
const button = (host: HTMLElement, text: string) =>
  [...host.querySelectorAll("button")].find((b) => b.textContent?.trim() === text);

async function open(role: "writer" | "reader") {
  members.role = role;
  const repo = new Repo({});
  const handle = repo.create<HDoc>({
    text: "Un.\n",
    meta: { versions: [], variants: [] },
  });
  handle.change((d) => A.splice(d, ["text"], 3, 0, "Deux.\n"));
  workspace.myId = "me";
  history.attach(handle);
  history.useInline();
  await history.refresh();
  history.open = true;
  return handle;
}

beforeEach(() => {
  history.diff = undefined;
  history.apply = undefined;
});
afterEach(() => {
  history.detach();
  history.open = false;
  cleanup();
});

describe("the history drawer", () => {
  it("HIS-09 a reader sees the history and compares; going back, merging and forking are disabled with the reason", async () => {
    await open("reader");
    const host = render(HistoryDrawer, {});
    history.selected = history.rows.find((r) => r.kind === "session")?.key;
    flushSync();
    expect(button(host, "Comparer avec maintenant")?.disabled).toBe(false);
    for (const name of [
      "Créer une variante ici",
      "Nommer cette version",
      "Revenir à cette version",
    ]) {
      const b = button(host, name);
      expect(b?.disabled, name).toBe(true);
      expect(b?.title, name).toMatch(/lecteur/);
    }
  });

  it("a writer can go back, with no reason in the way", async () => {
    await open("writer");
    const host = render(HistoryDrawer, {});
    history.selected = history.rows.find((r) => r.kind === "session")?.key;
    flushSync();
    expect(button(host, "Revenir à cette version")?.disabled).toBe(false);
    expect(labels(host)).toContain("Historique");
    expect(labels(host)).toContain("Commentaires");
    expect(labels(host)).toContain("Activité");
  });
});

describe("the differences window", () => {
  const session = () => {
    const s = history.entries.at(-1);
    if (!s) throw new Error("no entry");
    return s;
  };

  it("DIF-11 a reader sees everything; « Revenir » and « Annuler » are absent", async () => {
    await open("reader");
    const s = session();
    const req = {
      title: "t",
      before: "a b",
      after: "a c",
      session: s,
      attributed: false,
      revert: { heads: s.after, label: "x" },
      undo: { session: s },
    };
    const host = render(DiffView, { req });
    expect(host.querySelector("[data-testid=diff-summary]")?.textContent).toBeTruthy();
    expect(labels(host)).not.toContain("Revenir à ce moment");
    expect(labels(host)).not.toContain("Annuler cette modification");
    cleanup();
    members.role = "writer";
    await history.loadMembers();
    const again = render(DiffView, { req });
    expect(labels(again)).toContain("Revenir à ce moment");
    expect(labels(again)).toContain("Annuler cette modification");
  });

  it("DIF-07 / HIS-14 side by side, F7 and Shift+F7 step from change to change", async () => {
    await open("writer");
    const s = session();
    const before = "un deux\n\n\n\n\n\n\n\n\n\n\n\ntrois quatre\n";
    const after = "un DEUX\n\n\n\n\n\n\n\n\n\n\n\ntrois QUATRE\n";
    const host = render(DiffView, {
      req: { title: "t", before, after, session: s, attributed: false },
    });
    const info = () => host.querySelector("[data-testid=diff-info]")?.textContent ?? "";
    expect(entryDiff(before, after, s).hunks).toHaveLength(2);
    expect(info()).toContain("Changement 1 sur 2");
    button(host, "Côte à côte")?.click();
    flushSync();
    await Promise.resolve();
    flushSync();
    expect(host.querySelector(".cm-mergeView")).toBeTruthy();
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "F7" }));
    flushSync();
    expect(info()).toContain("Changement 2 sur 2");
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "F7", shiftKey: true }));
    flushSync();
    expect(info()).toContain("Changement 1 sur 2");
  });

  it("DIF-09 a table is shown cell by cell", async () => {
    await open("writer");
    const s = session();
    const host = render(DiffView, {
      req: {
        title: "t",
        before: "a,b\n1,12\n",
        after: "a,b\n1,15\n",
        session: s,
        attributed: false,
      },
      file: "stock.csv",
    });
    expect(host.querySelector("[data-testid=diff-cells]")?.textContent).toContain("B2 : 12 → 15");
  });
});
