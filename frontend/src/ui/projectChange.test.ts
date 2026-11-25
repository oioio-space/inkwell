import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { type AutomergeUrl, generateAutomergeUrl, Repo } from "@automerge/automerge-repo";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  Projects,
  Session,
} from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { registry } from "../extensions";
import { configure, type Shell, useRegistry } from "../extensions/commands";
import { setLive } from "../extensions/live.svelte";
import fr from "../i18n/fr.json";
import { withMutant } from "../mutant";
import { encodeEnvelope, encodeRoles, Kind } from "../net/envelope";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { type LeaveKind, markLeave, takeLeaveMark } from "../restart/mark";
import {
  APP_URL,
  GIVE_UP_MS,
  JOIN_REFUSED_URL,
  POLL_MS,
  REFUSED_URL,
  RETURN_REFUSED_URL,
  type RestartDeps,
  startRestart,
} from "../restart/restart";
import { FakeGoStore } from "../store/fakeStore";
import { GoStorageAdapter } from "../store/GoStorageAdapter";
import { JOIN_RETRY_MS, LEAVE_FLUSH_MS, workspace } from "../workspace/workspace.svelte";
import Editor from "./Editor.svelte";
import { session } from "./editorSession.svelte";
import JoinWaiting from "./JoinWaiting.svelte";
import { notice } from "./notice.svelte";
import AccessBanner from "./people/AccessBanner.svelte";
import type { Role, Standing } from "./people/roster";
import ShareDialog from "./ShareDialog.svelte";
import HeaderBar from "./shell/HeaderBar.svelte";
import { showStartupNotice } from "./startupNotice";
import { cleanup, click, render } from "./testkit.svelte";

/**
 * PC1a, the page side (plans 2026-11-15-pc1a-take-over.md and
 * 2026-11-14-pc1-project-change-by-restart.md): N-9 (« Reprendre ce document ici » asks once and
 * says that Inkwell restarts; the restart page asks Go once and loads the application only on
 * another startup id), N-12 (the notice of the next start, once), N-15 (the page flushes, closes
 * its repo and navigates before any request). PC1b (plan 2026-11-15-pc1b-join.md): J-2 (what a
 * join says, the wait for the document of the inviter, the notice, once) and the page's half of
 * J-1 (the join is completed only after the document is stored). Each rule's check also runs on a copy of the real
 * source with one edit, and must fail there (« PC1 mutant … »).
 */

vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: { Current: vi.fn(), SetRootURL: vi.fn() },
  Session: {
    Invite: vi.fn(),
    InvitationState: vi.fn(),
    Join: vi.fn(),
    FinishJoin: vi.fn(),
    CancelChange: vi.fn(),
    ReturnToPrevious: vi.fn(),
    PendingJoin: vi.fn(),
    CompleteJoin: vi.fn(),
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
/** Whether the repo holds the document asked of it: `find` fails otherwise, as for a peer away. */
let available = true;
/** How many times the repo was asked for a document, found or not. */
let finds = 0;
/** What a peer announces, as the test hears it from the adapter. */
let announce: ((url: AutomergeUrl, peer: string) => void) | undefined;

/** A document of the repo of spies: what `Workspace.open` needs of a handle. */
const handleOf = (url: AutomergeUrl) => ({
  url,
  documentId: url.slice("automerge:".length),
  on: () => {},
  off: () => {},
  doc: () => ({ text: "le texte de la personne qui invite" }),
});

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
  vi.spyOn(adapter, "onProject").mockImplementation((cb) => {
    announce = cb as typeof announce;
    return () => {
      announce = undefined;
    };
  });
  const repo = {
    handles: {},
    find: async (url: AutomergeUrl) => {
      finds++;
      if (!available) throw new Error(`Document ${url} is unavailable`);
      calls.push("find");
      return handleOf(url);
    },
    create: () => {
      calls.push("create");
      return handleOf(generateAutomergeUrl());
    },
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
  available = true;
  finds = 0;
  announce = undefined;
  vi.mocked(Projects.Current).mockResolvedValue({ RootURL: "" } as never);
  vi.mocked(Projects.SetRootURL).mockResolvedValue(undefined as never);
  vi.mocked(Session.PendingJoin).mockResolvedValue({
    waiting: false,
    root: "",
    inviter: "",
    name: "",
  } as never);
  vi.mocked(Session.CompleteJoin).mockImplementation((async () => {
    calls.push("complete");
  }) as never);
  vi.mocked(Session.FinishJoin).mockImplementation((async () => {
    calls.push("request");
  }) as never);
  vi.mocked(Session.CancelChange).mockResolvedValue(undefined as never);
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
  workspace.joinOffer = undefined;
  workspace.handle = undefined;
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
    expect(leave).toHaveBeenCalledWith("restart.html", "takeOver");
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
    confirmed: LeaveKind | false = "takeOver",
  ) {
    const target = document.createElement("main");
    const loaded: string[] = [];
    const log: string[] = [];
    let asked = 0;
    const deps: RestartDeps = {
      confirmed: () => confirmed || undefined,
      takeOver: async () => {
        log.push("takeOver");
        if (answer instanceof Error) throw answer;
        return { restarting: answer };
      },
      finishJoin: async () => {
        log.push("finishJoin");
        if (answer instanceof Error) throw answer;
      },
      returnToPrevious: async () => {
        log.push("returnToPrevious");
        if (answer instanceof Error) throw answer;
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
    expect(takeLeaveMark()).toBeUndefined();
    markLeave("takeOver");
    expect([takeLeaveMark(), takeLeaveMark()]).toEqual(["takeOver", undefined]);
    // The mark says which change was confirmed; anything else in its place is no mark.
    markLeave("join");
    expect([takeLeaveMark(), takeLeaveMark()]).toEqual(["join", undefined]);
    sessionStorage.setItem("inkwell.leave", "1");
    expect(takeLeaveMark()).toBeUndefined();
  });

  it("PC1 mutant N-9 request without the confirmation is caught", async () => {
    await withMutant<{ startRestart: Start }, void>(
      "restart/restart.ts",
      "pc1mutant-n9mark",
      "if (!kind) return deps.load(APP_URL);",
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
      confirmed: () => "takeOver",
      takeOver: async () => ({ restarting: true }),
      finishJoin: async () => {},
      returnToPrevious: async () => {},
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
    const left = w.leave("restart.html", "takeOver", navigate);
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
    expect(takeLeaveMark()).toBeUndefined();
    const navigate = vi.fn(() =>
      expect(takeLeaveMark(), "marked before the navigation").toBe("takeOver"),
    );
    expect(await workspace.leave("restart.html", "takeOver", navigate)).toBe(true);
    expect(navigate).toHaveBeenCalledTimes(1);
  });

  it("PC1 a repo that does not close does not hold the page", async () => {
    await arm(workspace, "member", "writer");
    closed = new Promise<void>(() => {}); // the repo never ends its shutdown
    vi.useFakeTimers();
    const navigate = vi.fn();
    const left = workspace.leave("restart.html", "takeOver", navigate);
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
    const left = workspace.leave("restart.html", "takeOver", navigate);
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
    // RT1 rule A-2: a read cut by the stream is sent again, once, on the next stream; cut again, it fails.
    await vi.waitFor(() => expect(store.log.filter(([, k]) => k === "held")).toHaveLength(2));
    store.dropConnections();
    await expect(read).rejects.toThrow("store stream closed");
    expect(refused, "a read lost with the stream is no lost write").not.toHaveBeenCalled();

    const write = storage.save(["b"], new Uint8Array([2]));
    write.catch(() => undefined);
    await vi.waitFor(() => expect(store.log.filter(([, k]) => k === "held")).toHaveLength(3));
    store.dropConnections();
    await expect(write).rejects.toThrow("store stream closed");
    expect(refused, "N-16 a write without its reply is reported").toHaveBeenCalledTimes(1);

    // The page's own close is no refusal.
    const own = storage.save(["c"], new Uint8Array([3]));
    own.catch(() => undefined);
    await vi.waitFor(() => expect(store.log.filter(([, k]) => k === "held")).toHaveLength(4));
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

describe("PC1 a join restarts Inkwell (J-1, J-2)", () => {
  type Dialog = typeof ShareDialog;
  const INVITER = "host";
  const ROOT = generateAutomergeUrl();
  const JOINED = {
    peer: INVITER,
    restart: true,
    title: "Roman",
    folder: "Roman (2)",
    current: "Mon livre",
  };
  const CONFIRM_BODY =
    "Inkwell va redémarrer pour ouvrir ce document dans un nouveau dossier, « Roman (2) », à côté du vôtre.";
  const CONFIRM_CURRENT = "Votre document actuel ne change pas : il reste dans « Mon livre ».";
  const confirmation = () => document.querySelector("[data-testid=join-confirm]");

  afterEach(async () => {
    await workspace.stop();
  });

  /** Go's answer to `Session.PendingJoin`: a record waits, with or without its root. */
  const waiting = (root: string, name = "Alice") =>
    vi
      .mocked(Session.PendingJoin)
      .mockResolvedValue({ waiting: true, root, inviter: INVITER, name } as never);

  /** Fills the invitation in the dialog of `host` and presses « Rejoindre ». */
  function submit(host: Element): HTMLFormElement {
    const form = host.querySelector("form") as HTMLFormElement;
    const input = form.querySelector("input") as HTMLInputElement;
    input.value = " invitation ";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    flushSync();
    return form;
  }

  /**
   * « Rejoindre »: what is said before; the progress while the person who invites answers; then
   * the question, which stays until the person chooses, and only « Redémarrer et ouvrir » makes
   * the page leave. The page itself asks Go for nothing more.
   */
  async function joinSaysChecks(Component: Dialog): Promise<void> {
    await arm(workspace, "member", "admin");
    workspace.handle = handleOf(ROOT) as never; // a text of one's own is open
    let answer: (joined: typeof JOINED) => void = () => {};
    vi.mocked(Session.Join).mockReturnValue(
      new Promise((r) => {
        answer = r;
      }) as never,
    );
    let left: (ok: boolean) => void = () => {};
    const leave = vi
      .spyOn(workspace, "leave")
      .mockImplementation(() => new Promise<boolean>((r) => (left = r)));
    const host = render(Component, { open: true });
    const form = host.querySelector("form") as HTMLFormElement;
    expect(text(form), "J-2 says that Inkwell restarts, before « Rejoindre »").toContain(
      "Inkwell va redémarrer pour ouvrir ce document.",
    );
    expect(text(form)).toContain(fr["join.restart.note"]);
    expect(text(form), "J-2 says before the click that the current document is kept").toContain(
      "Votre document actuel est conservé.",
    );
    expect(text(form), "the folder is not known before the invitation is used").not.toContain(
      "nouveau dossier",
    );
    expect(form.querySelector("button[type=submit]")?.getAttribute("aria-busy")).not.toBe("true");

    submit(host);
    await vi.waitFor(() => expect(Session.Join).toHaveBeenCalledWith("invitation"));
    flushSync();
    // D3: while the person who invites has not answered, the button and a sentence say so.
    const go = form.querySelector("button[type=submit]") as HTMLButtonElement;
    expect([go.disabled, go.getAttribute("aria-busy")], "J-2 « Rejoindre » shows progress").toEqual(
      [true, "true"],
    );
    const busy = form.querySelector("[role=status]");
    expect(busy && text(busy), "J-2 says that it connects").toBe(
      "Connexion à la personne qui vous invite…",
    );
    expect(text(form)).not.toContain(fr["join.restart.note"]);
    expect(confirmation()).toBeNull();

    answer(JOINED);
    await vi.waitFor(() => expect(confirmation()).not.toBeNull());
    flushSync();
    const asked = confirmation() as HTMLElement;
    expect(asked.hasAttribute("open")).toBe(true);
    expect(text(asked.querySelector("h2") as Element)).toBe("Rejoindre « Roman » ?");
    expect(text(asked.querySelector("p") as Element), "J-2 says where the document opens").toBe(
      `${CONFIRM_BODY} ${CONFIRM_CURRENT}`,
    );
    expect(buttons(asked).map(text)).toEqual(["Annuler", "Redémarrer et ouvrir"]);
    // The question stays until the person chooses: nothing leaves, nothing is asked of Go.
    await new Promise((r) => setTimeout(r, 20));
    expect(leave, "J-2 nothing restarts before the confirmation").not.toHaveBeenCalled();
    expect(confirmation()).not.toBeNull();
    expect(Session.FinishJoin).not.toHaveBeenCalled();
    expect(Session.CancelChange).not.toHaveBeenCalled();

    click(button(asked, "Redémarrer et ouvrir"));
    await vi.waitFor(() => expect(leave).toHaveBeenCalledTimes(1));
    expect(
      leave,
      "J-2 after « Redémarrer et ouvrir » the page leaves for the restart page",
    ).toHaveBeenCalledWith("restart.html", "join");
    expect(Session.FinishJoin, "N-15 the page itself never asks Go").not.toHaveBeenCalled();
    expect(Session.CancelChange).not.toHaveBeenCalled();
    left(true);
    await vi.waitFor(() => expect(workspace.joining).toBe(false));
    expect(workspace.error).toBe("");
  }

  /**
   * The held invitation, in the workspace: nothing leaves before the confirmation, and
   * « Annuler » drops what Go holds, so that another « Rejoindre » works.
   */
  async function heldChecks(w: Workspace): Promise<void> {
    await arm(w, "member", "admin");
    vi.mocked(Session.Join).mockResolvedValue(JOINED as never);
    const leave = vi.spyOn(w, "leave").mockResolvedValue(true);
    await w.join("invitation");
    expect(leave, "J-2 nothing restarts before the confirmation").not.toHaveBeenCalled();
    // A first launch has no text of its own: no folder of one's own is named.
    expect(w.joinOffer).toEqual({ title: "Roman", folder: "Roman (2)", current: "" });
    expect(w.joining).toBe(false);

    await w.cancelJoin();
    expect(
      vi.mocked(Session.CancelChange).mock.calls.length,
      "J-2 « Annuler » drops the held invitation",
    ).toBe(1);
    expect(w.joinOffer).toBeUndefined();
    expect(leave).not.toHaveBeenCalled();
    expect(Session.FinishJoin).not.toHaveBeenCalled();

    await w.join("invitation");
    expect(Session.Join, "a second « Rejoindre » works afterwards").toHaveBeenCalledTimes(2);
    expect(w.joinOffer?.folder).toBe("Roman (2)");
    await w.confirmJoin();
    expect(leave).toHaveBeenCalledWith("restart.html", "join");
  }

  it("PC1 join says that Inkwell restarts", async () => {
    await joinSaysChecks(ShareDialog);
    // In a copy of this very project the invitation changes nothing here: no restart is announced.
    cleanup();
    await arm(workspace, "notNamed");
    const form = render(ShareDialog, { open: true }).querySelector("form") as HTMLFormElement;
    expect(text(form)).toContain(fr["share.join.foreignHelp"]);
    expect(text(form)).not.toContain(fr["join.restart.note"]);
  });

  it("PC1 mutant J-2 a join that does not say it restarts is caught", async () => {
    await withMutant<{ default: Dialog }, void>(
      "ui/ShareDialog.svelte",
      "pc1mutant-j2note",
      '{t("join.restart.note")}',
      "",
      async (m) => {
        await expect(joinSaysChecks(m.default)).rejects.toThrow(/J-2 says that Inkwell restarts/);
      },
    );
  });

  it("PC1 a join waits for the confirmation, and « Annuler » drops it", async () => {
    await heldChecks(workspace);
  });

  it("PC1 mutant J-2 a restart without the confirmation is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "pc1mutant-j2confirm",
      "        this.joinOffer = {",
      '        void this.leave("restart.html", "join");\n        this.joinOffer = {',
      async (m) => {
        await expect(heldChecks(m.workspace)).rejects.toThrow(
          /J-2 nothing restarts before the confirmation/,
        );
        await m.workspace.stop();
      },
    );
  });

  it("PC1 mutant J-2 « Annuler » that leaves the held invitation is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "pc1mutant-j2cancel",
      "    this.joinOffer = undefined;\n    await Session.CancelChange().catch(() => undefined);",
      "    this.joinOffer = undefined;",
      async (m) => {
        await expect(heldChecks(m.workspace)).rejects.toThrow(
          /J-2 « Annuler » drops the held invitation/,
        );
        await m.workspace.stop();
      },
    );
  });

  it("PC1 the confirmation of a join: « Annuler », and a first launch names no folder of its own", async () => {
    await arm(workspace, "member", "admin");
    vi.mocked(Session.Join).mockResolvedValue(JOINED as never);
    const leave = vi.spyOn(workspace, "leave").mockResolvedValue(true);
    const host = render(ShareDialog, { open: true });
    submit(host);
    await vi.waitFor(() => expect(confirmation()).not.toBeNull());
    flushSync();
    const asked = confirmation() as HTMLElement;
    expect(text(asked.querySelector("p") as Element)).toBe(CONFIRM_BODY);
    click(button(asked, "Annuler"));
    await vi.waitFor(() => expect(Session.CancelChange).toHaveBeenCalledTimes(1));
    flushSync();
    expect(confirmation()).toBeNull();
    expect(leave).not.toHaveBeenCalled();
    expect(
      host.querySelector("dialog")?.hasAttribute("open"),
      "the invitation is still there",
    ).toBe(true);

    // D3: after a failed attempt the restart sentence does not sit above the error.
    vi.mocked(Session.Join).mockRejectedValue(new Error("no answer") as never);
    const form = submit(host);
    await vi.waitFor(() => expect(workspace.error).toBe(fr["error.join"]));
    flushSync();
    expect(text(form)).not.toContain(fr["join.restart.note"]);
    expect(text(host.querySelector("[role=alert]") as Element)).toBe(fr["error.join"]);
  });

  /**
   * An invitation is used once (rule J-2): after « Annuler » the field is emptied and the card
   * says to ask for a new one, where the restart note was; no error is shown.
   */
  async function cancelledChecks(Component: Dialog): Promise<Element> {
    await arm(workspace, "member", "admin");
    vi.mocked(Session.Join).mockResolvedValue(JOINED as never);
    const host = render(Component, { open: true });
    const form = submit(host);
    await vi.waitFor(() => expect(confirmation()).not.toBeNull());
    flushSync();
    click(button(confirmation() as HTMLElement, "Annuler"));
    await vi.waitFor(() => expect(Session.CancelChange).toHaveBeenCalledTimes(1));
    flushSync();
    const said = form.querySelector("[role=status]");
    expect(said && text(said), "J-2 says that a cancelled invitation is used").toBe(
      "Invitation annulée. Pour rejoindre ce document plus tard, demandez une nouvelle invitation.",
    );
    expect(
      (form.querySelector("input") as HTMLInputElement).value,
      "J-2 a cancelled invitation leaves the field",
    ).toBe("");
    expect(text(form)).not.toContain(fr["join.restart.note"]);
    expect(host.querySelector("[role=alert]")).toBeNull();
    return host;
  }

  it("PC1 a cancelled invitation leaves the field, and the page asks for a new one", async () => {
    const host = await cancelledChecks(ShareDialog);
    // The next « Rejoindre » starts afresh: the sentence goes.
    submit(host);
    await vi.waitFor(() => expect(confirmation()).not.toBeNull());
    expect(workspace.joinCancelled).toBe(false);
  });

  it("PC1 mutant J-2 a cancelled invitation kept in the field is caught", async () => {
    await withMutant<{ default: Dialog }, void>(
      "ui/ShareDialog.svelte",
      "pc1mutant-j2field",
      'if (workspace.joinCancelled) address = "";',
      "void workspace.joinCancelled;",
      async (m) => {
        await expect(cancelledChecks(m.default)).rejects.toThrow(
          /J-2 a cancelled invitation leaves the field/,
        );
      },
    );
  });

  /**
   * The share dialog of an admin who made an invitation, of which Go says `state` (rule J-8): an
   * open invitation is offered to copy; a used or expired one is not, and a new one is offered.
   * Before Go answers nothing is offered: a spent invitation never flashes by.
   */
  async function invitationChecks(Component: Dialog, state: "open" | "used" | "expired") {
    await arm(workspace, "member", "admin");
    workspace.invitation = "inkwell1:made";
    workspace.invitationState = "open";
    let answer: (s: string) => void = () => {};
    vi.mocked(Session.InvitationState).mockReturnValue(
      new Promise<string>((resolve) => (answer = resolve)) as never,
    );
    const host = render(Component, { open: true });
    const card = () => host.querySelector("section") as Element;
    flushSync();
    expect(
      button(card(), fr["share.copy"]),
      "J-8 nothing is offered before Go answers",
    ).toBeUndefined();
    answer(state);
    await vi.waitFor(() => {
      flushSync();
      if (state === "open")
        expect(
          button(card(), fr["share.copy"]),
          "J-8 an open invitation is offered to copy",
        ).toBeDefined();
      else
        expect(
          button(card(), fr["share.renew"]),
          "J-8 a used invitation is not offered: a new one is offered in its place",
        ).toBeDefined();
    });
    if (state !== "open") {
      expect(
        button(card(), fr["share.copy"]),
        "J-8 a used invitation is not offered",
      ).toBeUndefined();
      expect(text(card())).toContain(fr[state === "used" ? "share.used" : "share.expired"]);
    }
    expect(Session.InvitationState).toHaveBeenCalledWith("inkwell1:made");
    cleanup();
    workspace.invitation = "";
  }

  it("PC1 the share dialog follows what Go says of the invitation", async () => {
    for (const state of ["open", "used", "expired"] as const)
      await invitationChecks(ShareDialog, state);
  });

  it("PC1 mutant J-8 a dialog that does not ask Go is caught", async () => {
    await withMutant<{ default: Dialog }, void>(
      "ui/ShareDialog.svelte",
      "pc1mutant-j8ask",
      "untrack(() => void workspace.refreshInvitation().finally(() => (checked = true)));",
      "checked = true;",
      async (m) => {
        // Without the question the old invitation shows at once: caught before or after Go's answer.
        await expect(invitationChecks(m.default, "used")).rejects.toThrow(
          /J-8 (nothing is offered before Go answers|a used invitation is not offered)/,
        );
        cleanup();
        workspace.invitation = "";
      },
    );
  });

  it("PC1 mutant J-8 a dialog that offers the invitation before Go answers is caught", async () => {
    await withMutant<{ default: Dialog }, void>(
      "ui/ShareDialog.svelte",
      "pc1mutant-j8wait",
      "{#if !checked}",
      "{#if false}",
      async (m) => {
        await expect(invitationChecks(m.default, "open")).rejects.toThrow(
          /J-8 nothing is offered before Go answers/,
        );
        cleanup();
        workspace.invitation = "";
      },
    );
  });

  it("PC1 a join the page could not save for is dropped", async () => {
    // N-15: past the flush deadline nothing is sent, the held invitation is dropped, and the
    // person is told; the document open now stays writable.
    await arm(workspace, "member", "admin");
    vi.mocked(Session.Join).mockResolvedValue(JOINED as never);
    flushed = new Promise<void>(() => {});
    vi.useFakeTimers();
    await workspace.join("invitation");
    const joined = workspace.confirmJoin();
    await vi.advanceTimersByTimeAsync(LEAVE_FLUSH_MS);
    await joined;
    expect(Session.CancelChange, "N-15 the held invitation is dropped").toHaveBeenCalledTimes(1);
    expect(Session.FinishJoin).not.toHaveBeenCalled();
    expect(takeLeaveMark(), "no mark: the restart page would ask for nothing").toBeUndefined();
    expect([notice.text, notice.role]).toEqual([fr["change.flushFailed"], "alert"]);
    expect([workspace.locked, workspace.joinOffer]).toEqual([false, undefined]);
    expect(workspace.joinCancelled, "J-2 the invitation is used all the same").toBe(true);
  });

  it("PC1 a join into the open project does not restart", async () => {
    // J-5: Go answers « no restart »; the page stays, and follows the inviter as before.
    await arm(workspace, "notNamed");
    vi.mocked(Session.Join).mockResolvedValue({ peer: INVITER, restart: false } as never);
    const leave = vi.spyOn(workspace, "leave");
    const joined = workspace.join("invitation");
    await vi.waitFor(() => expect(announce).toBeDefined());
    announce?.(ROOT, INVITER);
    await joined;
    expect(leave).not.toHaveBeenCalled();
    expect(workspace.handle?.url).toBe(ROOT);
    expect(workspace.error).toBe("");
  });

  it("PC1 the page that leaves for a join marks it, and the restart page finishes it", async () => {
    await arm(workspace, "member", "admin");
    const navigate = vi.fn();
    expect(await workspace.leave("restart.html", "join", navigate)).toBe(true);
    const kind = takeLeaveMark();
    expect(kind).toBe("join");

    // The restart page: « Inkwell redémarre… », FinishJoin once, the application on another id.
    const target = document.createElement("main");
    const log: string[] = [];
    const ids = ["a", "a", "b"];
    await startRestart(target, {
      confirmed: () => kind,
      takeOver: async () => {
        log.push("takeOver");
        return { restarting: true };
      },
      finishJoin: async () => void log.push("finishJoin"),
      returnToPrevious: async () => void log.push("returnToPrevious"),
      startupId: async () => {
        const id = ids.shift() as string;
        log.push(`id:${id}`);
        return id;
      },
      load: (url) => void log.push(`load:${url}`),
      wait: (ms) => (ms === POLL_MS ? Promise.resolve() : new Promise<void>(() => {})),
    });
    expect(text(target)).toBe(fr["foreign.takeOver.busy"]);
    expect(log, "J-1 the restart page asks Go to finish the join, once").toEqual([
      "id:a",
      "finishJoin",
      "id:a",
      "id:b",
      `load:${APP_URL}`,
    ]);
  });

  it("PC1 a join Go refuses to finish says that nothing changed", async () => {
    // The request failed and the same image still answers: the application comes back and says so.
    const target = document.createElement("main");
    const loaded: string[] = [];
    const ids = ["a", "a"];
    await startRestart(target, {
      confirmed: () => "join",
      takeOver: async () => ({ restarting: true }),
      finishJoin: async () => {
        throw new Error("no invitation is held");
      },
      returnToPrevious: async () => undefined,
      startupId: async () => ids.shift() as string,
      load: (url) => void loaded.push(url),
      wait: (ms) => (ms === POLL_MS ? Promise.resolve() : new Promise<void>(() => {})),
    });
    expect(loaded).toEqual([JOIN_REFUSED_URL]);
    const replaced: string[] = [];
    await showStartupNotice(
      () => {},
      Promise.resolve(),
      new URL(JOIN_REFUSED_URL, "http://127.0.0.1:1/"),
      (clean) => replaced.push(clean),
    );
    expect([notice.text, notice.role]).toEqual([
      "L'invitation n'a pas pu être terminée. Rien n'a changé.",
      "alert",
    ]);
    expect(replaced).toEqual(["/"]);

    // J-4: the next start found what an unfinished join left, and Go says so, once.
    notice.hide();
    vi.mocked(Session.StartupNotice).mockResolvedValueOnce("join.failed" as never);
    await showStartupNotice(() => {});
    expect([notice.text, notice.role]).toEqual([fr["join.failed"], "alert"]);
  });

  /**
   * A start on a folder whose record waits: the sentence in place of the editor for as long as
   * the document of the inviter is not there, and never a fresh document.
   */
  async function waitChecks(w: Workspace): Promise<void> {
    vi.useFakeTimers();
    await arm(w, "member", "writer");
    waiting(ROOT);
    available = false; // the inviter is away
    const started = w.openStart();
    started.catch(() => undefined); // a start that opens something else fails on the checks below
    await vi.advanceTimersByTimeAsync(0);
    for (let turn = 0; turn < 5; turn++) {
      expect(calls, "J-2 never a fresh document while a record waits").not.toContain("create");
      expect(w.waitingFor, "J-2 waits for the inviter").toBe("Alice");
      expect(w.handle, "nothing in the editor's place but the sentence").toBeUndefined();
      await vi.advanceTimersByTimeAsync(JOIN_RETRY_MS);
    }
    expect(finds, "asked again for as long as needed").toBeGreaterThan(4);
    expect(Session.CompleteJoin).not.toHaveBeenCalled();
    expect(
      Projects.SetRootURL,
      "no root is recorded before the join is completed",
    ).not.toHaveBeenCalled();

    available = true; // the inviter is back
    await vi.advanceTimersByTimeAsync(JOIN_RETRY_MS);
    await started;
    expect(w.waitingFor).toBeUndefined();
    expect(w.handle?.url, "the document of the inviter").toBe(ROOT);
    expect(calls).not.toContain("create");
  }

  it("PC1 a pending join waits for its root and never creates a document", async () => {
    await waitChecks(workspace);
    vi.useRealTimers();

    // The sentence, in place of the editor, announced as a status.
    workspace.waitingFor = "Alice";
    const host = render(JoinWaiting, {});
    const said = host.querySelector("[role=status]");
    expect(said && text(said)).toBe("En attente de Alice pour ouvrir le document…");
    expect(fr["join.waitingRoot"]).toBe("En attente de {name} pour ouvrir le document…");
    workspace.waitingFor = undefined;
    flushSync();
    expect(text(host)).toBe("");
  });

  it("PC1 the waiting page says what to do and offers the way back", async () => {
    await arm(workspace, "member", "writer");
    workspace.waitingFor = "";
    workspace.canReturn = false;
    const host = render(JoinWaiting, {});
    expect(text(host), "J-2 the waiting page says what happens next").toBe(
      "En attente de la personne qui vous a invité pour ouvrir le document… Le document s'ouvrira dès que cette personne sera connectée. Vous pouvez fermer Inkwell : l'attente reprendra à la prochaine ouverture.",
    );
    expect(buttons(host), "no way back without a previous document").toEqual([]);
    expect(workspace.status, "J-2 never « Seul » while a join waits").toBe("connecting");

    workspace.canReturn = true;
    flushSync();
    const back = button(host, "Revenir à mon document");
    expect(back, "J-2 the waiting page offers the way back").toBeDefined();
    const leave = vi.spyOn(workspace, "leave").mockResolvedValue(true);
    click(back);
    expect(leave).toHaveBeenCalledWith("restart.html", "return");
    expect(Session.ReturnToPrevious, "N-15 the page itself never asks Go").not.toHaveBeenCalled();
    workspace.waitingFor = undefined;
    workspace.canReturn = false;
  });

  it("PC1 the restart page asks Go for the return, and a refused return says so", async () => {
    const run = async (ids: string[], returnToPrevious: () => Promise<void>) => {
      const log: string[] = [];
      await startRestart(document.createElement("main"), {
        confirmed: () => "return",
        takeOver: async () => ({ restarting: void log.push("takeOver") === undefined }),
        finishJoin: async () => void log.push("finishJoin"),
        returnToPrevious: async () => {
          log.push("returnToPrevious");
          await returnToPrevious();
        },
        startupId: async () => ids.shift() as string,
        load: (url) => void log.push(`load:${url}`),
        wait: (ms) => (ms === POLL_MS ? Promise.resolve() : new Promise<void>(() => {})),
      });
      return log;
    };
    expect(await run(["a", "a", "b"], async () => undefined)).toEqual([
      "returnToPrevious",
      `load:${APP_URL}`,
    ]);
    const refused = await run(["a", "a"], async () => {
      throw new Error("no previous document");
    });
    expect(refused).toEqual(["returnToPrevious", `load:${RETURN_REFUSED_URL}`]);
    await showStartupNotice(
      () => {},
      Promise.resolve(),
      new URL(RETURN_REFUSED_URL, "http://127.0.0.1:1/"),
      () => {},
    );
    expect([notice.text, notice.role]).toEqual([
      "Impossible de revenir à votre document. Rien n'a changé.",
      "alert",
    ]);
  });

  it("PC1 the joined notice names the folder when it is not the one announced", async () => {
    await arm(workspace, "member", "writer");
    vi.mocked(Session.PendingJoin).mockResolvedValue({
      waiting: true,
      root: ROOT,
      inviter: INVITER,
      name: "Alice",
      folder: "Roman (3)",
      canReturn: true,
    } as never);
    await workspace.openStart();
    expect([notice.text, notice.role]).toEqual([
      "Vous avez rejoint ce document. Il est dans le dossier « Roman (3) ».",
      "status",
    ]);
    expect(workspace.canReturn, "J-9 the way back stays once the document is open").toBe(true);
    workspace.canReturn = false;
  });

  it("PC1 the main menu offers the way back after a join", async () => {
    // J-9: Go says the way back stays once the join completed; every start asks it again.
    await arm(workspace, "member", "writer");
    vi.mocked(Session.PendingJoin).mockResolvedValue({ waiting: false, canReturn: true } as never);
    vi.mocked(Projects.Current).mockResolvedValue({ RootURL: ROOT } as never);
    await workspace.openStart();
    expect(workspace.canReturn, "J-9 the way back after a join survives a relaunch").toBe(true);

    // The header bar's menu is the registry's (EX0b1): « Revenir à mon document » is `app.return`, shown
    // when `canReturn` holds, and runs the shell's way back.
    const menu = async (canReturn: boolean, onReturn: () => void) => {
      useRegistry(registry, { system: "linux" });
      configure({ shell: { returnToPrevious: onReturn } as unknown as Shell });
      await registry.loadCommand(registry.command("app.return") as never);
      setLive({ canReturn });
      const host = render(HeaderBar, {
        title: "Inkwell",
        subtitle: "main.typ",
        status: "alone",
        me: { id: "me", name: "Moi" },
        peers: [],
      });
      click(host.querySelector(`button[aria-label="${fr["menu.main"]}"]`) as HTMLElement);
      flushSync();
      return [...document.querySelectorAll("[role=menuitem]")] as HTMLElement[];
    };
    const onReturn = vi.fn();
    const items = await menu(true, onReturn);
    const back = items.find((i) => text(i) === fr["join.return"]);
    expect(back, "J-9 the main menu offers « Revenir à mon document »").toBeDefined();
    // The project's group comes before the document's (the view's toggles and full screen are above).
    const firstDocumentAction = items.findIndex((i) => text(i).startsWith(fr["menu.export"]));
    expect(firstDocumentAction).toBeGreaterThan(0);
    expect(
      items.indexOf(back as HTMLElement),
      "J-9 the way back comes before the document's actions",
    ).toBeLessThan(firstDocumentAction);
    expect(items.slice(0, items.indexOf(back as HTMLElement)).map((i) => i.dataset.cmd)).toEqual([
      "app.fullscreen",
    ]);
    click(back as HTMLElement);
    expect(onReturn).toHaveBeenCalledTimes(1);
    cleanup();
    expect(
      (await menu(false, onReturn)).some((i) => text(i) === fr["join.return"]),
      "no way back without a previous document",
    ).toBe(false);
    workspace.canReturn = false;
  });

  it("PC1 mutant J-2 fresh document is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "pc1mutant-j2fresh",
      "if (pending.waiting) return this.#resumeJoin(pending, generation);",
      "",
      async (m) => {
        try {
          await expect(waitChecks(m.workspace)).rejects.toThrow(
            /J-2 never a fresh document while a record waits/,
          );
        } finally {
          await m.workspace.stop();
        }
      },
    );
  });

  it("PC1 a pending join without a root waits for the one the inviter announces", async () => {
    await arm(workspace, "member", "writer");
    waiting("", ""); // the invitation stated no root, and the list gives the inviter no name
    const started = workspace.openStart();
    await vi.waitFor(() => expect(announce).toBeDefined());
    // No name to show: the sentence names nobody, never a neutral « Personne 12D3 » (every device
    // id starts alike).
    expect(workspace.waitingFor, "J-2 no made-up name for the inviter").toBe("");
    const said = render(JoinWaiting, {}).querySelector("[role=status]");
    expect(said && text(said), "J-2 the sentence when the list gives the inviter no name").toBe(
      "En attente de la personne qui vous a invité pour ouvrir le document…",
    );
    expect(fr["join.waitingRootUnnamed"]).toBe(said && text(said));
    expect(finds, "nothing to ask for before a root is announced").toBe(0);
    // A device the list does not name announces a document: not the one waited for.
    announce?.(generateAutomergeUrl(), "stranger");
    await new Promise((r) => setTimeout(r, 0));
    expect(finds).toBe(0);
    announce?.(ROOT, INVITER);
    await started;
    expect(Session.CompleteJoin).toHaveBeenCalledWith(ROOT, INVITER);
    expect(workspace.handle?.url).toBe(ROOT);
    expect(calls).not.toContain("create");
  });

  /**
   * The record holds no root, and nothing signed states one yet: only the inviter's announcement
   * counts. Another member of the joined list announces first and serves its document: it is
   * neither asked for nor recorded, however long the inviter stays away.
   */
  async function inviterOnlyChecks(w: Workspace): Promise<void> {
    vi.useFakeTimers();
    await arm(w, "member", "writer");
    waiting("");
    vi.spyOn(WailsNetworkAdapter.prototype, "roleOf").mockImplementation(((peer: string) =>
      peer === "mallory" || peer === "me" ? "writer" : undefined) as never);
    const started = w.openStart();
    started.catch(() => undefined); // a start that follows another member fails on the checks below
    await vi.advanceTimersByTimeAsync(0);
    announce?.(generateAutomergeUrl(), "mallory");
    await vi.advanceTimersByTimeAsync(2 * JOIN_RETRY_MS);
    expect(finds, "J-1 only the inviter announces the root of a record without one").toBe(0);
    expect(
      Session.CompleteJoin,
      "J-1 only the inviter announces the root of a record without one",
    ).not.toHaveBeenCalled();
    expect(w.waitingFor, "J-2 still waiting for the inviter").toBe("Alice");
    expect(w.handle).toBeUndefined();

    announce?.(ROOT, INVITER);
    await vi.advanceTimersByTimeAsync(0);
    await started;
    expect(Session.CompleteJoin).toHaveBeenCalledTimes(1);
    expect(Session.CompleteJoin).toHaveBeenCalledWith(ROOT, INVITER);
    expect(w.handle?.url).toBe(ROOT);
  }

  it("PC1 a pending join without a root ignores what another member announces", async () => {
    await inviterOnlyChecks(workspace);
  });

  it("PC1 mutant J-1 a root announced by another member is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "pc1mutant-j1inviter",
      "if (from !== peer) return;",
      "if (from !== peer && adapter.roleOf(from) === undefined) return;",
      async (m) => {
        try {
          await expect(inviterOnlyChecks(m.workspace)).rejects.toThrow(
            /J-1 only the inviter announces the root of a record without one/,
          );
        } finally {
          await m.workspace.stop();
        }
      },
    );
  });

  /** The record is consumed only once the document is there and the storage has it. */
  async function orderChecks(w: Workspace): Promise<void> {
    await arm(w, "member", "writer");
    waiting(ROOT);
    let release: () => void = () => {};
    flushed = new Promise<void>((r) => (release = r));
    const started = w.openStart();
    await vi.waitFor(() => expect(calls).toContain("flush"));
    await new Promise((r) => setTimeout(r, 0));
    expect(calls, "J-1 completed only after the flush returned").toEqual(["find", "flush"]);
    expect(w.waitingFor, "still waiting while the document is being stored").toBe("Alice");
    release();
    await started;
    expect(calls.slice(0, 3), "J-1 completed only after the flush returned").toEqual([
      "find",
      "flush",
      "complete",
    ]);
    expect(Session.CompleteJoin).toHaveBeenCalledTimes(1);
    expect(Session.CompleteJoin).toHaveBeenCalledWith(ROOT, INVITER);
    expect(Projects.SetRootURL).toHaveBeenCalledWith(ROOT);
  }

  it("PC1 the join is completed only after the document is flushed", async () => {
    await orderChecks(workspace);
  });

  it("PC1 mutant J-1 completion before the flush is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "pc1mutant-j1flush",
      "await repo.flush([handle.documentId]);\n        await Session.CompleteJoin(root, inviter);",
      "await Session.CompleteJoin(root, inviter);\n        await repo.flush([handle.documentId]);",
      async (m) => {
        try {
          await expect(orderChecks(m.workspace)).rejects.toThrow(
            /J-1 completed only after the flush returned/,
          );
        } finally {
          flushed = Promise.resolve();
          await m.workspace.stop();
        }
      },
    );
  });

  it("PC1 a join Go does not complete yet is asked again, and nothing opens meanwhile", async () => {
    vi.useFakeTimers();
    await arm(workspace, "member", "writer");
    waiting(ROOT);
    vi.mocked(Session.CompleteJoin).mockRejectedValueOnce(
      new Error("join: the document is not stored yet") as never,
    );
    const started = workspace.openStart();
    await vi.advanceTimersByTimeAsync(0);
    expect(Session.CompleteJoin).toHaveBeenCalledTimes(1);
    expect([workspace.waitingFor, workspace.handle]).toEqual(["Alice", undefined]);
    expect(notice.text).toBe("");
    await vi.advanceTimersByTimeAsync(JOIN_RETRY_MS);
    await started;
    expect(Session.CompleteJoin).toHaveBeenCalledTimes(2);
    expect(workspace.handle?.url).toBe(ROOT);
  });

  it("PC1 a consumed join is never asked of Go again", async () => {
    // Once Go consumed the record, a failure of what follows is no reason to complete again.
    vi.useFakeTimers();
    await arm(workspace, "member", "writer");
    waiting(ROOT);
    vi.mocked(Projects.SetRootURL).mockRejectedValueOnce(new Error("disk full") as never);
    const started = workspace.openStart();
    const failed = expect(started).rejects.toThrow("disk full");
    await vi.advanceTimersByTimeAsync(3 * JOIN_RETRY_MS);
    await failed;
    expect(Session.CompleteJoin, "J-1 consumed once").toHaveBeenCalledTimes(1);
    expect(workspace.waitingFor).toBeUndefined();
  });

  it("PC1 the joined notice shows once", async () => {
    await arm(workspace, "member", "writer");
    waiting(ROOT);
    await workspace.openStart();
    expect([notice.text, notice.role], "J-2 said by the image that consumed the record").toEqual([
      "Vous avez rejoint ce document.",
      "status",
    ]);
    expect(notice.text).toBe(fr["join.done"]);
    expect(workspace.waitingFor).toBeUndefined();

    // The record is consumed: the next start opens the recorded root and says nothing.
    notice.hide();
    await workspace.stop();
    await arm(workspace, "member", "writer");
    vi.mocked(Session.PendingJoin).mockResolvedValue({
      waiting: false,
      root: "",
      inviter: "",
      name: "",
    } as never);
    vi.mocked(Projects.Current).mockResolvedValue({ RootURL: ROOT } as never);
    await workspace.openStart();
    expect(workspace.handle?.url).toBe(ROOT);
    expect(notice.text, "J-2 shown once").toBe("");
    expect(Session.CompleteJoin).toHaveBeenCalledTimes(1);
    expect(calls).not.toContain("create");
  });

  it("PC1 a start with no record opens a fresh document as before", async () => {
    await arm(workspace, "member", "writer");
    await workspace.openStart();
    expect(calls).toContain("create");
    expect(workspace.waitingFor).toBeUndefined();
    expect(Session.CompleteJoin).not.toHaveBeenCalled();
  });
});
