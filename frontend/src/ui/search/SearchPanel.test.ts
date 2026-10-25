import { next as A } from "@automerge/automerge";
import { flushSync, mount, tick, unmount } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TestHandle, typst } from "../../editor/search/testkit";
import { HISTORY_SIZE, loadHistory, pushHistory } from "./history";
import SearchPanel from "./SearchPanel.svelte";
import { live } from "./testprops.svelte";

// The interface language is read when the catalogue loads: make it French first.
vi.hoisted(() => {
  Object.defineProperty(navigator, "language", { value: "fr-FR", configurable: true });
});

type Props = Parameters<typeof SearchPanel>[1];

let host: HTMLDivElement;
let mounted: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  localStorage.clear();
  host = document.createElement("div");
  document.body.append(host);
});
afterEach(() => {
  if (mounted) unmount(mounted);
  mounted = undefined;
  host.remove();
});

function panel(props: Partial<Props> = {}) {
  const navigate = vi.fn();
  const docs = new Map([
    ["a.typ", TestHandle.of("un foo ici\nrien\nfoo et foo")],
    ["b.typ", TestHandle.of("deux foo")],
  ]);
  live.version = 0;
  const p = {
    projectKey: "p1",
    files: () => [...docs].map(([path, h]) => typst(path, h.text)),
    anchor: (path: string) => docs.get(path)?.current,
    handleOf: (path: string) => docs.get(path),
    onnavigate: navigate,
    get version() {
      return live.version;
    },
    ...props,
  } as Props;
  mounted = mount(SearchPanel, { target: host, props: p });
  return { navigate, docs, api: mounted as unknown as { step(b?: boolean): Promise<void> } };
}

async function type(value: string) {
  const input = host.querySelector<HTMLInputElement>('input[role="searchbox"]');
  if (!input) throw new Error("no field");
  input.value = value;
  input.dispatchEvent(new Event("input", { bubbles: true }));
  await tick();
}

const button = (label: RegExp) =>
  [...host.querySelectorAll("button")].find((b) =>
    label.test(b.textContent ?? ""),
  ) as HTMLButtonElement;

async function settle() {
  for (let i = 0; i < 3; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await tick();
  }
  flushSync();
}

describe("search panel", () => {
  it("SRC-06 shows an invalid regex as a message under the field, not an exception", async () => {
    panel();
    button(/^Regex$/).click();
    await type("(abc");
    const alert = host.querySelector('[role="alert"]');
    expect(alert?.textContent).toMatch(/parenthèse non fermée/i);
    expect(host.querySelector('input[role="searchbox"]')?.getAttribute("aria-invalid")).toBe(
      "true",
    );
    expect(button(/Tout trouver/).disabled).toBe(true);
    await type("(abc)");
    expect(host.querySelector('[role="alert"]')).toBeNull();
  });

  it("SRC-20 results are an ARIA tree: files, then lines, with a counter", async () => {
    const { navigate } = panel();
    await type("foo");
    button(/Tout trouver/).click();
    await settle();

    const tree = host.querySelector('[role="tree"]');
    expect(tree).not.toBeNull();
    const files = host.querySelectorAll('[role="treeitem"][aria-level="1"]');
    expect([...files].map((f) => f.getAttribute("aria-expanded"))).toEqual(["true", "true"]);
    expect(host.querySelectorAll('[role="treeitem"][aria-level="2"]')).toHaveLength(4);
    expect(host.textContent).toContain("4 résultats dans 2 fichiers");
    // The hit is highlighted inside its line, and line numbers are shown.
    expect(host.querySelector("mark")?.textContent?.toLowerCase()).toBe("foo");

    // Enter on a result jumps to it.
    const hit = host.querySelectorAll<HTMLElement>('[role="treeitem"][aria-level="2"]')[2];
    hit?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    expect(navigate).toHaveBeenCalledWith({ path: "a.typ", from: 23, to: 26 });

    // Left on a file collapses its group.
    const file = files[0] as HTMLElement;
    file.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
    await settle();
    expect(file.getAttribute("aria-expanded")).toBe("false");
    expect(host.querySelectorAll('[role="treeitem"][aria-level="2"]')).toHaveLength(1);
  });

  it("SRC-11 (unit side) clicking a result of another file emits its path and range", async () => {
    const { navigate } = panel();
    await type("foo");
    button(/Tout trouver/).click();
    await settle();
    const last = [...host.querySelectorAll<HTMLElement>('[role="treeitem"][aria-level="2"]')].pop();
    last?.click();
    expect(navigate).toHaveBeenCalledWith({ path: "b.typ", from: 5, to: 8 });
  });

  it("F3 and Shift+F3 step through the results and wrap", async () => {
    const { navigate, api } = panel();
    await type("foo");
    await api.step();
    await api.step();
    expect(navigate).toHaveBeenLastCalledWith({ path: "a.typ", from: 16, to: 19 });
    await api.step(true);
    expect(navigate).toHaveBeenLastCalledWith({ path: "a.typ", from: 3, to: 6 });
    await api.step(true); // wraps to the last result
    expect(navigate).toHaveBeenLastCalledWith({ path: "b.typ", from: 5, to: 8 });
    // The key itself, from inside the panel.
    host
      .querySelector("section")
      ?.dispatchEvent(new KeyboardEvent("keydown", { key: "F3", bubbles: true }));
    await settle();
    expect(navigate).toHaveBeenLastCalledWith({ path: "a.typ", from: 3, to: 6 });
  });

  it("SRC-12 a result changed by someone else is greyed out and never jumped to", async () => {
    const { navigate, docs } = panel();
    await type("foo");
    button(/Tout trouver/).click();
    await settle();
    const a = docs.get("a.typ") as TestHandle;
    // A peer adds a line above (results follow) and rewrites the first foo (greyed out).
    a.current = A.change(a.current, (d) => A.splice(d, ["text"], 0, 0, "NEW\n"));
    a.current = A.change(a.current, (d) => A.splice(d, ["text"], 7, 3, "bar"));
    live.version++;
    await settle();
    const hits = host.querySelectorAll<HTMLElement>('[role="treeitem"][aria-level="2"]');
    expect(hits[0]?.classList.contains("stale")).toBe(true);
    expect(hits[0]?.getAttribute("aria-disabled")).toBe("true");
    hits[0]?.click();
    expect(navigate).not.toHaveBeenCalled();
    hits[1]?.click();
    expect(navigate).toHaveBeenCalledWith({ path: "a.typ", from: 20, to: 23 });
    // F3 skips the greyed-out one.
    navigate.mockClear();
    await (mounted as unknown as { step(b?: boolean): Promise<void> }).step();
    expect(navigate).toHaveBeenCalledTimes(1);
  });

  it("SRC-18 a reader can search but has no replace controls", async () => {
    panel({ readOnly: true });
    expect(button(/Remplacer/)).toBeUndefined();
    expect(button(/Tout remplacer/)).toBeUndefined();
    await type("foo");
    button(/Tout trouver/).click();
    await settle();
    expect(host.textContent).toContain("4 résultats");
  });

  it("replace all in the project shows a preview first, then changes one document once", async () => {
    const { docs } = panel();
    button(/Remplacer…/).click();
    await settle();
    await type("foo");
    const repl = host.querySelector<HTMLInputElement>('input[aria-label="Remplacer par"]');
    if (!repl) throw new Error("no replace field");
    repl.value = "BAR";
    repl.dispatchEvent(new Event("input", { bubbles: true }));
    await tick();
    button(/Tout remplacer/).click();
    await settle();
    expect(host.querySelector('[role="dialog"]')).not.toBeNull();
    expect((docs.get("a.typ") as TestHandle).text).toContain("foo");
    button(/Remplacer 4/).click();
    await settle();
    expect((docs.get("a.typ") as TestHandle).text).toBe("un BAR ici\nrien\nBAR et BAR");
    expect((docs.get("a.typ") as TestHandle).changes).toBe(1);
    expect((docs.get("b.typ") as TestHandle).text).toBe("deux BAR");
    expect(host.textContent).toContain("4 remplacée(s)");
  });

  it("SRC-16 occurrences in locked zones are reported, not replaced", async () => {
    const { docs } = panel({ isLocked: (path, r) => path === "a.typ" && r.from >= 15 });
    button(/Remplacer…/).click();
    await settle();
    await type("foo");
    const repl = host.querySelector<HTMLInputElement>('input[aria-label="Remplacer par"]');
    if (!repl) throw new Error("no replace field");
    repl.value = "BAR";
    repl.dispatchEvent(new Event("input", { bubbles: true }));
    await tick();
    button(/Tout remplacer/).click();
    await settle();
    expect(host.textContent).toContain("2 dans des zones verrouillées");
    button(/Remplacer 2/).click();
    await settle();
    expect((docs.get("a.typ") as TestHandle).text).toBe("un BAR ici\nrien\nfoo et foo");
  });

  it("SRC-21 keeps the last 20 searches per project, with mode and options", async () => {
    panel();
    button(/Regex/).click();
    await type("fo+");
    button(/Respecter la casse/).click();
    button(/Tout trouver/).click();
    await settle();
    expect(loadHistory("p1")[0]).toMatchObject({
      query: "fo+",
      mode: "regex",
      caseSensitive: true,
    });
    expect(loadHistory("other")).toEqual([]);

    for (let i = 0; i < 25; i++) {
      pushHistory("p1", {
        ...(loadHistory("p1")[0] as ReturnType<typeof loadHistory>[number]),
        query: `q${i}`,
      });
    }
    const h = loadHistory("p1");
    expect(h).toHaveLength(HISTORY_SIZE);
    expect(h[0]?.query).toBe("q24");
    // The stored text is untrusted: garbage is dropped, not thrown on.
    localStorage.setItem(
      "inkwell.search.history.p1",
      '[{"query":1},{"query":"ok","mode":"zzz"},null,"x"]',
    );
    expect(loadHistory("p1")).toMatchObject([{ query: "ok", mode: "normal", scope: "project" }]);
    localStorage.setItem("inkwell.search.history.p1", "not json");
    expect(loadHistory("p1")).toEqual([]);
  });

  it("history survives a browser that refuses storage", () => {
    const spy = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(() =>
      pushHistory("p", {
        query: "x",
        mode: "normal",
        extended: false,
        caseSensitive: false,
        wholeWord: false,
        ignoreAccents: false,
        dotAll: false,
        visibleText: false,
        filters: "",
        scope: "project",
      }),
    ).not.toThrow();
    spy.mockRestore();
  });

  it("stacks earlier searches and clears them", async () => {
    panel();
    await type("foo");
    button(/Tout trouver/).click();
    await settle();
    await type("rien");
    button(/Tout trouver/).click();
    await settle();
    expect(host.querySelectorAll(".run")).toHaveLength(2);
    button(/Effacer/).click();
    await settle();
    expect(host.querySelectorAll(".run")).toHaveLength(0);
  });

  it("search in these results narrows to the lines already found", async () => {
    panel();
    await type("foo");
    button(/Tout trouver/).click();
    await settle();
    await type("et");
    button(/Chercher dans ces résultats/).click();
    await settle();
    expect(host.textContent).toContain("1 résultat dans 1 fichier");
  });
});
