import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { Repo } from "@automerge/automerge-repo";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import fr from "../i18n/fr.json";
import { withMutant } from "../mutant";
import { encodeEnvelope, encodeRoles, Kind } from "../net/envelope";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { markLeave, takeLeaveMark } from "../restart/mark";
import {
  APP_URL,
  GIVE_UP_MS,
  POLL_MS,
  REFUSED_URL,
  type RestartDeps,
  startRestart,
} from "../restart/restart";
import { FakeGoStore } from "../store/fakeStore";
import { GoStorageAdapter } from "../store/GoStorageAdapter";
import { LEAVE_FLUSH_MS, workspace } from "../workspace/workspace.svelte";
import Editor from "./Editor.svelte";
import { session } from "./editorSession.svelte";
import { notice } from "./notice.svelte";
import AccessBanner from "./people/AccessBanner.svelte";
import type { Role, Standing } from "./people/roster";
import { showStartupNotice } from "./startupNotice";
import { cleanup, click, render } from "./testkit.svelte";

/**
 * PC1a, the page side (plans 2026-11-15-pc1a-take-over.md and
 * 2026-11-14-pc1-project-change-by-restart.md): N-9 (« Reprendre ce document ici » asks once and
 * says that Inkwell restarts; the restart page asks Go once and loads the application only on
 * another startup id), N-12 (the notice of the next start, once), N-15 (the page flushes, closes
 * its repo and navigates before any request). Each rule's check also runs on a copy of the real
 * source with one edit, and must fail there (« PC1 mutant … »).
 */

vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {
    Invite: vi.fn(),
    Join: vi.fn(),
    Standing: vi.fn(),
    CanTakeOver: vi.fn(),
    Members: vi.fn(async () => []),
    TakeOver: vi.fn(),
    StartupNotice: vi.fn(),
  },
}));

type Workspace = typeof workspace;
type Banner = typeof AccessBanner;

const text = (el: Element) => el.textContent?.replace(/\s+/g, " ").trim() ?? "";
const buttons = (host: Element) => [...host.querySelectorAll("button")];
const button = (host: Element, label: string) =>
  buttons(host).find((b) => text(b) === label) as HTMLButtonElement;

/** What the page did, in order: the spies of the repo, the navigation and the bound calls. */
let calls: string[] = [];
let standingNow: Standing = "member";
/** Ends when the test lets the repo's flush end; never by itself when `held`. */
let flushed: Promise<void> = Promise.resolve();
/** Ends when the test lets the repo's shutdown end. */
let closed: Promise<void> = Promise.resolve();

/** Gives `w` a network adapter on a fake link, a repo of spies, Go's standing and maybe its role. */
async function arm(w: Workspace, standing: Standing, role?: Role): Promise<void> {
  standingNow = standing;
  let socket: SocketLike | undefined;
  const fake: SocketLike = {
    binaryType: "",
    send: () => {},
    close: () => {},
    onopen: null,
    onclose: null,
    onmessage: null,
  };
  const adapter = new WailsNetworkAdapter(
    () => {
      socket = fake;
      return fake;
    },
    { admitted: () => true },
  );
  const repo = {
    handles: {},
    flush: async () => {
      calls.push("flush");
      await flushed;
    },
    shutdown: async () => {
      calls.push("shutdown");
      await closed;
    },
  };
  w.myId = "me";
  w.attach(adapter, repo as unknown as Repo);
  adapter.connect("me" as never);
  if (role) {
    const frame = encodeEnvelope({
      kind: Kind.Roles,
      peer: "me",
      payload: encodeRoles([{ peer: "me", role, suspended: false }]),
    });
    socket?.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  }
  await w.refreshStanding();
  flushSync();
}

beforeEach(() => {
  calls = [];
  flushed = Promise.resolve();
  closed = Promise.resolve();
  vi.mocked(Session.Standing).mockImplementation((async () => standingNow) as never);
  vi.mocked(Session.CanTakeOver).mockResolvedValue(true as never);
  vi.mocked(Session.TakeOver).mockImplementation((async () => {
    calls.push("request");
    return { restarting: true };
  }) as never);
  vi.mocked(Session.StartupNotice).mockResolvedValue("" as never);
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
    this.dispatchEvent(new Event("close"));
  };
});
afterEach(() => {
  vi.useRealTimers();
  cleanup();
  vi.restoreAllMocks();
  workspace.standing = undefined;
  workspace.leaving = false;
  workspace.error = "";
  takeLeaveMark();
  while (notice.text) notice.hide();
});

describe("PC1 the banner of a device the list does not name (N-9)", () => {
  /** The banner of a device the list does not name: its sentence and its two actions, no other. */
  async function bannerChecks(Component: Banner): Promise<void> {
    await arm(workspace, "notNamed");
    const host = render(Component, {});
    const banner = host.querySelector("[data-testid=access-banner]");
    expect(banner && text(banner)).toBe(fr["foreign.notNamed"]);
    expect(buttons(host).map(text), "N-9 two actions, nothing else").toEqual([
      fr["foreign.ask"],
      fr["foreign.takeOver"],
    ]);
  }

  it("PC1 not a member: one sentence, two actions", async () => {
    await bannerChecks(AccessBanner);
    // A person the list removed has the same two ways out; a member has none.
    cleanup();
    await arm(workspace, "removed");
    expect(buttons(render(AccessBanner, {})).map(text)).toEqual([
      fr["foreign.ask"],
      fr["foreign.takeOver"],
    ]);
    cleanup();
    await arm(workspace, "member", "reader");
    expect(buttons(render(AccessBanner, {}))).toEqual([]);
  });

  it("PC1 mutant N-9 an extra action is caught", async () => {
    await withMutant<{ default: Banner }, void>(
      "ui/people/AccessBanner.svelte",
      "pc1mutant-n9extra",
      "{/if}\n      {/if}\n    {/snippet}",
      '{/if}\n        <button type="button" class="btn">+</button>\n      {/if}\n    {/snippet}',
      async (m) => {
        await expect(bannerChecks(m.default)).rejects.toThrow(/N-9 two actions, nothing else/);
      },
    );
  });

  /** Where Inkwell cannot restart itself, the take-over is not offered (residual R-8). */
  async function noRestartChecks(Component: Banner): Promise<void> {
    vi.mocked(Session.CanTakeOver).mockResolvedValue(false as never);
    await arm(workspace, "notNamed");
    expect(buttons(render(Component, {})).map(text), "R-8 no take-over where no restart").toEqual([
      fr["foreign.ask"],
    ]);
  }

  it("PC1 no take-over where Inkwell cannot restart", async () => {
    await noRestartChecks(AccessBanner);
  });

  it("PC1 mutant R-8 take-over offered without a restart is caught", async () => {
    await withMutant<{ default: Banner }, void>(
      "ui/people/AccessBanner.svelte",
      "pc1mutant-r8",
      "{#if workspace.canTakeOver}",
      "{#if true}",
      async (m) => {
        await expect(noRestartChecks(m.default)).rejects.toThrow(/R-8 no take-over/);
      },
    );
  });

  /** « Reprendre ce document ici »: one confirmation that says Inkwell restarts, then the page leaves. */
  async function confirmationChecks(Component: Banner): Promise<void> {
    await arm(workspace, "notNamed");
    const leave = vi.spyOn(workspace, "leave").mockResolvedValue(true);
    const host = render(Component, {});
    click(button(host, fr["foreign.takeOver"]));
    expect(leave, "N-9 one confirmation before anything").not.toHaveBeenCalled();
    const dialog = host.querySelector("dialog");
    expect(dialog?.hasAttribute("open"), "N-9 one confirmation before anything").toBe(true);
    const said = text(dialog as Element);
    expect(said).toContain(fr["foreign.takeOver.confirm.title"]);
    expect(said).toContain(fr["foreign.takeOver.confirm.body"]);
    expect(said, "N-9 the confirmation says that Inkwell restarts").toContain(
      "Inkwell va redémarrer",
    );
    expect(Session.TakeOver, "the page itself never asks Go").not.toHaveBeenCalled();

    // Cancelling changes nothing and the question can be asked again.
    click(button(dialog as Element, fr["people.cancel"]));
    expect(host.querySelector("dialog")).toBeNull();
    expect(leave).not.toHaveBeenCalled();

    click(button(host, fr["foreign.takeOver"]));
    click(button(host.querySelector("dialog") as Element, fr["foreign.takeOver"]));
    expect(leave, "asked once, left once").toHaveBeenCalledTimes(1);
    expect(leave).toHaveBeenCalledWith("restart.html");
    expect(host.querySelector("dialog"), "the question is not asked twice").toBeNull();
  }

  it("PC1 take-over asks once and says that Inkwell restarts", async () => {
    await confirmationChecks(AccessBanner);
  });

  it("PC1 mutant N-9 confirmation is caught", async () => {
    await withMutant<{ default: Banner }, void>(
      "ui/people/AccessBanner.svelte",
      "pc1mutant-n9confirm",
      "onclick={() => (confirming = true)}",
      "onclick={takeOver}",
      async (m) => {
        await expect(confirmationChecks(m.default)).rejects.toThrow(
          /N-9 one confirmation before anything/,
        );
      },
    );
  });
});

describe("PC1 the restart page (N-9)", () => {
  type Start = typeof startRestart;

  /**
   * The restart page on a Go that answers `ids` in turn to the startup id (an Error is a call
   * that fails: the process is between two images) and `answer` to the take-over. `wait` ends at
   * once while an id is left, then never: the page then waits for good.
   */
  function page(
    start: Start,
    ids: (string | Error)[],
    answer: boolean | Error = true,
    confirmed = true,
  ) {
    const target = document.createElement("main");
    const loaded: string[] = [];
    const log: string[] = [];
    let asked = 0;
    const deps: RestartDeps = {
      confirmed: () => confirmed,
      takeOver: async () => {
        log.push("takeOver");
        if (answer instanceof Error) throw answer;
        return { restarting: answer };
      },
      startupId: async () => {
        const id = ids[asked++];
        log.push(id instanceof Error ? "id:error" : `id:${id}`);
        if (id instanceof Error) throw id;
        if (id === undefined) throw new Error("asked past the script");
        return id;
      },
      load: (url) => {
        loaded.push(url);
        log.push(`load:${url}`);
      },
      // The polls end at once while an id is left; the wait before giving up never ends here
      // (« PC1 the restart page gives up… » plays it with fake timers).
      wait: (ms) =>
        ms === POLL_MS && asked < ids.length ? Promise.resolve() : new Promise<void>(() => {}),
    };
    const done = start(target, deps);
    return {
      target,
      loaded,
      log,
      done,
      settle: () => vi.waitFor(() => expect(asked).toBe(ids.length)),
    };
  }

  it("PC1 the restart page asks Go once and waits", async () => {
    const p = page(startRestart, ["a", "a", "a", "a"]);
    await p.settle();
    expect(text(p.target)).toBe(fr["foreign.takeOver.busy"]);
    expect(p.target.querySelector("[role=status]")).not.toBeNull();
    expect(
      p.log.filter((c) => c === "takeOver"),
      "N-9 asked once",
    ).toHaveLength(1);
    expect(p.log.slice(0, 2), "the id of the old image is read before the request").toEqual([
      "id:a",
      "takeOver",
    ]);
    expect(p.loaded, "N-9 waits while the same image answers").toEqual([]);
  });

  /** Without the mark of the page that left, the restart page asks Go for nothing. */
  async function markChecks(start: Start): Promise<void> {
    const p = page(start, ["a", "a"], true, false);
    await vi.waitFor(() => expect(p.log).not.toEqual([]));
    expect(p.log, "N-9 no request without the confirmation's mark").toEqual([`load:${APP_URL}`]);
  }

  it("PC1 the restart page asks nothing when opened without the confirmation", async () => {
    await markChecks(startRestart);
    // The mark is the leaving page's, and it is taken once: a reloaded restart page has none.
    expect(takeLeaveMark()).toBe(false);
    markLeave();
    expect([takeLeaveMark(), takeLeaveMark()]).toEqual([true, false]);
  });

  it("PC1 mutant N-9 request without the confirmation is caught", async () => {
    await withMutant<{ startRestart: Start }, void>(
      "restart/restart.ts",
      "pc1mutant-n9mark",
      "if (!deps.confirmed()) return deps.load(APP_URL);",
      "",
      async (m) => {
        await expect(markChecks(m.startRestart)).rejects.toThrow(/N-9 no request without/);
      },
    );
  });

  /** The application is loaded on another startup id, and on nothing else. */
  async function reloadChecks(start: Start): Promise<void> {
    const down = new Error("connection refused");
    const p = page(start, ["a", "a", down, "a", "b"]);
    await p.done;
    expect(p.log, "N-9 reload only on another id").toEqual([
      "id:a",
      "takeOver",
      "id:a",
      "id:error",
      "id:a",
      "id:b",
      `load:${APP_URL}`,
    ]);
  }

  it("PC1 the restart page reloads only on another startup id", async () => {
    await reloadChecks(startRestart);
  });

  it("PC1 mutant N-9 reload on the same id is caught", async () => {
    await withMutant<{ startRestart: Start }, void>(
      "restart/restart.ts",
      "pc1mutant-n9id",
      "if (id !== before) return load(APP_URL);",
      "if (id) return load(APP_URL);",
      async (m) => {
        await expect(reloadChecks(m.startRestart)).rejects.toThrow(/N-9 reload only on another id/);
      },
    );
  });

  it("PC1 a refused take-over reloads the application unchanged", async () => {
    // Go answers « not restarting » (a member, N-8): the application comes back at once.
    const refused = page(startRestart, ["a"], false);
    await refused.done;
    expect(refused.log).toEqual(["id:a", "takeOver", `load:${REFUSED_URL}`]);

    // The request fails and the same image still answers: nothing changed either.
    const failed = page(startRestart, ["a", new Error("down"), "a"], new Error("drain timed out"));
    await failed.done;
    expect(failed.log).toEqual(["id:a", "takeOver", "id:error", "id:a", `load:${REFUSED_URL}`]);

    // The reply was lost with the old image, and another one answers: Go's own notice decides.
    const lost = page(startRestart, ["a", "b"], new Error("connection reset"));
    await lost.done;
    expect(lost.loaded).toEqual([APP_URL]);

    // Back in the application: the sentence, once, and the address is clean again.
    const replaced: string[] = [];
    const url = new URL(REFUSED_URL, "http://127.0.0.1:1/");
    await showStartupNotice(
      () => {},
      Promise.resolve(),
      url,
      (clean) => replaced.push(clean),
    );
    expect([notice.text, notice.role]).toEqual([fr["foreign.takeOver.failed"], "alert"]);
    expect(replaced).toEqual(["/"]);
  });

  it("PC1 a refused take-over says so even when the document does not open", async () => {
    // After a failed drain the bridge stays closed (N-16): the application cannot open the
    // document, and the sentence must not wait for it.
    const focus = vi.fn();
    const never = new Promise<void>(() => {});
    const url = new URL(REFUSED_URL, "http://127.0.0.1:1/");
    void showStartupNotice(focus, never, url, () => {});
    await vi.waitFor(() =>
      expect([notice.text, notice.role], "N-9 the sentence does not wait for the document").toEqual(
        [fr["foreign.takeOver.failed"], "alert"],
      ),
    );
    expect(focus).not.toHaveBeenCalled();
  });

  /**
   * The restart page on real (fake) time: `answer(ms)` is what the startup id call gives at `ms`
   * after the start (an Error: nothing answers). Time moves with the fake timers.
   */
  function timedPage(start: Start, answer: (ms: number) => string | Error) {
    const target = document.createElement("main");
    const loaded: string[] = [];
    const t0 = Date.now();
    const done = start(target, {
      confirmed: () => true,
      takeOver: async () => ({ restarting: true }),
      startupId: async () => {
        const id = answer(Date.now() - t0);
        if (id instanceof Error) throw id;
        return id;
      },
      load: (url) => loaded.push(url),
      wait: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
    });
    return { target, loaded, done };
  }

  /**
   * UX-PC1a-2: past {@link GIVE_UP_MS} without another image, the page stops saying that Inkwell
   * restarts and says what happened and what to do; it still loads the application if an image
   * answers later (the person opened Inkwell again).
   */
  async function giveUpChecks(start: Start): Promise<void> {
    vi.useFakeTimers();
    try {
      const down = new Error("connection refused");
      // The old image goes after 1 s; nothing answers until the person opens Inkwell again at 40 s.
      const dead = timedPage(start, (ms) => (ms < 1000 ? "a" : ms < 40_000 ? down : "b"));
      // The old image never goes: its shutdown is stuck.
      const stuck = timedPage(start, () => "a");
      await vi.advanceTimersByTimeAsync(GIVE_UP_MS - 1);
      for (const p of [dead, stuck]) {
        expect(text(p.target), "busy until the bound").toBe(fr["foreign.takeOver.busy"]);
      }
      await vi.advanceTimersByTimeAsync(1);
      for (const p of [dead, stuck]) {
        expect(text(p.target), "N-12 the restart page gives up after the bound").toBe(
          fr["change.restartFailed"],
        );
        expect(p.target.querySelector("[role=alert]"), "announced as an alert").not.toBeNull();
        expect(p.target.querySelector("[role=status]")).toBeNull();
        expect(p.loaded, "nothing is loaded on the same image or on none").toEqual([]);
      }
      await vi.advanceTimersByTimeAsync(40_000);
      expect(dead.loaded, "an image that answers later is still loaded").toEqual([APP_URL]);
      expect(stuck.loaded).toEqual([]);
    } finally {
      vi.useRealTimers();
    }
  }

  it("PC1 the restart page gives up when no other image answers", async () => {
    expect(GIVE_UP_MS, "the bound: three times the 5 s of the termination barrier").toBe(15_000);
    await giveUpChecks(startRestart);
  });

  it("PC1 a restart within the bound never shows the failure", async () => {
    vi.useFakeTimers();
    try {
      const p = timedPage(startRestart, (ms) => (ms < 1000 ? "a" : "b"));
      await vi.advanceTimersByTimeAsync(2 * GIVE_UP_MS);
      await p.done;
      expect(p.loaded).toEqual([APP_URL]);
      expect(text(p.target), "the page that left says nothing else").toBe(
        fr["foreign.takeOver.busy"],
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it("PC1 mutant N-12 a restart page that waits for good is caught", async () => {
    await withMutant<{ startRestart: Start }, void>(
      "restart/restart.ts",
      "pc1mutant-n12giveup",
      "void deps.wait(GIVE_UP_MS).then(giveUp);",
      "",
      async (m) => {
        await expect(giveUpChecks(m.startRestart)).rejects.toThrow(
          /N-12 the restart page gives up after the bound/,
        );
      },
    );
  });

  it("PC1 the restart page holds no repo, adapter or storage handle", () => {
    // The module graph of the page's entry: every file it imports, directly or not.
    const src = resolve(dirname(fileURLToPath(import.meta.url)), "..");
    const seen = new Set<string>();
    const packages = new Set<string>();
    const visit = (file: string) => {
      if (seen.has(file)) return;
      seen.add(file);
      if (!/\.[cm]?[jt]s$/.test(file)) return;
      const source = readFileSync(file, "utf8");
      for (const m of source.matchAll(/(?:\bfrom\s*|\bimport\s*\(?\s*)["']([^"']+)["']/g)) {
        const spec = m[1] as string;
        if (!spec.startsWith(".")) {
          packages.add(spec);
          continue;
        }
        const base = resolve(dirname(file), spec);
        const found = [base, `${base}.ts`, base.replace(/\.js$/, ".ts"), `${base}/index.ts`].find(
          (f) => {
            try {
              return readFileSync(f) !== undefined;
            } catch {
              return false;
            }
          },
        );
        if (!found) throw new Error(`unresolved import ${spec} in ${file}`);
        visit(found);
      }
    };
    visit(resolve(src, "restart/main.ts"));
    const files = [...seen].map((f) => f.slice(src.length - "src".length));
    expect(
      files.some((f) => f.endsWith("internal/app/session.ts")),
      "it does call Session",
    ).toBe(true);
    expect([...packages], "N-15 the restart page imports only the bridge runtime").toEqual([
      "@wailsio/runtime",
    ]);
    const allowed = /^(src\/restart\/|src\/i18n\/|src\/theme\/[\w.-]+\.css$|bindings\/)/;
    expect(
      files.filter((f) => !allowed.test(f)),
      "N-15 no repo, adapter, editor, storage or store in the restart page",
    ).toEqual([]);
    expect(files.filter((f) => /workspace|store|net\/|editor|svelte|automerge/i.test(f))).toEqual(
      [],
    );
  });
});

describe("PC1 the page leaves its context before the request (N-15)", () => {
  /** After the confirmation: read-only, flush, repo closed, navigation; no request from this page. */
  async function leaveChecks(w: Workspace): Promise<void> {
    await arm(w, "member", "writer");
    expect(w.locked).toBe(false);
    let release: () => void = () => {};
    flushed = new Promise<void>((r) => (release = r));
    const navigate = vi.fn((url: string) => void calls.push(`navigate:${url}`));
    const left = w.leave("restart.html", navigate);
    await vi.waitFor(() => expect(calls).toEqual(["flush"]));
    expect(w.locked, "N-15 read-only while the page leaves").toBe(true);
    expect(navigate).not.toHaveBeenCalled();
    release();
    expect(await left).toBe(true);
    expect(calls, "N-15 no request before the navigation").toEqual([
      "flush",
      "shutdown",
      "navigate:restart.html",
    ]);
    expect(w.repo, "the repo is gone").toBeUndefined();
  }

  it("PC1 the page flushes, closes its repo and navigates before any request", async () => {
    await leaveChecks(workspace);
  });

  it("PC1 mutant N-15 request before navigation is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "pc1mutant-n15",
      "    navigate(to);",
      "    await Session.TakeOver();\n    navigate(to);",
      async (m) => {
        await expect(leaveChecks(m.workspace)).rejects.toThrow(
          /N-15 no request before the navigation/,
        );
      },
    );
  });

  it("PC1 the leaving page marks the restart page's one request", async () => {
    await arm(workspace, "member", "writer");
    expect(takeLeaveMark()).toBe(false);
    const navigate = vi.fn(() =>
      expect(takeLeaveMark(), "marked before the navigation").toBe(true),
    );
    expect(await workspace.leave("restart.html", navigate)).toBe(true);
    expect(navigate).toHaveBeenCalledTimes(1);
  });

  it("PC1 a repo that does not close does not hold the page", async () => {
    await arm(workspace, "member", "writer");
    closed = new Promise<void>(() => {}); // the repo never ends its shutdown
    vi.useFakeTimers();
    const navigate = vi.fn();
    const left = workspace.leave("restart.html", navigate);
    await vi.advanceTimersByTimeAsync(LEAVE_FLUSH_MS - 1);
    expect(navigate, "still closing just before the deadline").not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(await left, "N-15 the page leaves whatever the close says").toBe(true);
    expect(navigate).toHaveBeenCalledWith("restart.html");
  });

  it("PC1 a flush past its deadline sends nothing and changes nothing", async () => {
    await arm(workspace, "member", "writer");
    flushed = new Promise<void>(() => {}); // the storage never answers
    vi.useFakeTimers();
    const navigate = vi.fn();
    const left = workspace.leave("restart.html", navigate);
    await vi.advanceTimersByTimeAsync(LEAVE_FLUSH_MS - 1);
    expect(workspace.locked, "still leaving just before the deadline").toBe(true);
    expect(notice.text).toBe("");
    await vi.advanceTimersByTimeAsync(1);
    expect(await left).toBe(false);
    expect(
      calls,
      "N-15 nothing sent past the deadline: no close, no navigation, no request",
    ).toEqual(["flush"]);
    expect(navigate).not.toHaveBeenCalled();
    expect(Session.TakeOver).not.toHaveBeenCalled();
    expect(workspace.repo, "the repo is still the page's").toBeDefined();
    expect(workspace.locked, "the editor is unlocked again").toBe(false);
    expect([notice.text, notice.role]).toEqual([fr["change.flushFailed"], "alert"]);
  });

  it("PC1 a write refused because the session is leaving says so", async () => {
    // N-16 seen from the page: Go answers « leaving » to a write it did not take.
    const store = new FakeGoStore();
    const storage = new GoStorageAdapter(() => store.socket());
    const refused = vi.fn();
    storage.onLeaving(refused);
    await storage.save(["a"], new Uint8Array([1]));
    expect(refused).not.toHaveBeenCalled();
    store.leaving = true;
    await expect(storage.save(["b"], new Uint8Array([2]))).rejects.toThrow("leaving");
    expect(refused).toHaveBeenCalledTimes(1);
    storage.close();
  });

  it("PC1 a write left unanswered when the stream ends says so", async () => {
    // N-16 seen from the page: Go's drain may end the stream before its refusal is out. A write
    // without its reply is not saved as far as the page knows: the person is told all the same.
    const store = new FakeGoStore();
    const storage = new GoStorageAdapter(() => store.socket());
    const refused = vi.fn();
    storage.onLeaving(refused);
    await storage.save(["a"], new Uint8Array([1]));
    store.held = true;
    const read = storage.load(["a"]);
    read.catch(() => undefined);
    await vi.waitFor(() => expect(store.log.at(-1)?.[1]).toBe("held"));
    store.dropConnections();
    await expect(read).rejects.toThrow("store stream closed");
    expect(refused, "a read lost with the stream is no lost write").not.toHaveBeenCalled();

    const write = storage.save(["b"], new Uint8Array([2]));
    write.catch(() => undefined);
    await vi.waitFor(() => expect(store.log.filter(([, k]) => k === "held")).toHaveLength(2));
    store.dropConnections();
    await expect(write).rejects.toThrow("store stream closed");
    expect(refused, "N-16 a write without its reply is reported").toHaveBeenCalledTimes(1);

    // The page's own close is no refusal.
    const own = storage.save(["c"], new Uint8Array([3]));
    own.catch(() => undefined);
    await vi.waitFor(() => expect(store.log.filter(([, k]) => k === "held")).toHaveLength(3));
    storage.close();
    await expect(own).rejects.toThrow("store closed");
    expect(refused).toHaveBeenCalledTimes(1);
  });
});

describe("PC1 the notice of the next start (N-12)", () => {
  const repos: Repo[] = [];
  afterEach(() => repos.splice(0).forEach((r) => void r.shutdown()));

  /** The real editor on a document holding "hello", as the shell mounts it. */
  function mountEditor() {
    const repo = new Repo({ network: [] });
    repos.push(repo);
    const handle = repo.create<{ text: string }>({ text: "hello" });
    render(Editor, { handle, me: { id: "me", name: "Me" }, lang: "typst" });
    const view = session.view;
    if (!view) throw new Error("the editor did not mount");
    return view;
  }

  it("PC1 the startup notice shows once and the text has the focus", async () => {
    await arm(workspace, "member", "admin");
    const view = mountEditor();
    const start = vi.mocked(Session.StartupNotice);
    start.mockResolvedValueOnce("foreign.takeOver.done" as never); // Go gives it once
    // The focus is elsewhere, as after a page load.
    const elsewhere = document.body.appendChild(document.createElement("button"));
    elsewhere.focus();
    expect(view.hasFocus).toBe(false);
    await showStartupNotice(session.focus);
    expect([notice.text, notice.role]).toEqual([fr["foreign.takeOver.done"], "status"]);
    expect(view.hasFocus, "N-12 the text has the focus").toBe(true);
    expect(view.state.readOnly, "writable").toBe(false);

    notice.hide();
    await showStartupNotice(session.focus);
    expect(notice.text, "N-12 shown once").toBe("");
    expect(start).toHaveBeenCalledTimes(2);
  });

  it("PC1 a failed take-over says so and stays readable", async () => {
    await arm(workspace, "notNamed");
    const view = mountEditor();
    vi.mocked(Session.StartupNotice).mockResolvedValueOnce("foreign.takeOver.failed" as never);
    await showStartupNotice(session.focus);
    expect([notice.text, notice.role]).toEqual([fr["foreign.takeOver.failed"], "alert"]);
    // The document is as it was: readable, read-only, with its sentence and its two actions.
    expect(view.state.doc.toString()).toBe("hello");
    expect(view.state.readOnly).toBe(true);
    const host = render(AccessBanner, {});
    expect(text(host.querySelector("[data-testid=access-banner]") as Element)).toBe(
      fr["foreign.notNamed"],
    );
    expect(buttons(host).map(text)).toEqual([fr["foreign.ask"], fr["foreign.takeOver"]]);

    // A key Go would not send is never shown as a sentence.
    notice.hide();
    vi.mocked(Session.StartupNotice).mockResolvedValueOnce("share.title" as never);
    await showStartupNotice(session.focus);
    expect(notice.text).toBe("");
  });
});
