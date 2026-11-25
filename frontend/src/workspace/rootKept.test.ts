import { type AutomergeUrl, generateAutomergeUrl, type Repo } from "@automerge/automerge-repo";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  Projects,
  Session,
} from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import fr from "../i18n/fr.json";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { notice } from "../ui/notice.svelte";
import { workspace } from "./workspace.svelte";

/**
 * RT1, the start's half (plan 2026-11-25-rt1-root-kept.md): the start replaces the project's root only when Go
 * proves the saved document absent (A-1), through ReplaceMissingRoot before any SetRootURL (A-3), and a start that
 * cannot open a stored root writes nothing and says so (A-4).
 */

vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {
    Current: vi.fn(),
    SetRootURL: vi.fn(),
    RootState: vi.fn(),
    ReplaceMissingRoot: vi.fn(),
  },
  Session: { PendingJoin: vi.fn() },
}));

/** What the start did, in order: the repo's creations and the bound calls that record a root. */
let calls: string[] = [];
/** The documents whose load fails, as when the store stream is cut under it. */
let failing = new Set<string>();

const handleOf = (url: AutomergeUrl) => ({
  url,
  documentId: url.slice("automerge:".length),
  on: () => {},
  off: () => {},
  doc: () => ({ text: "" }),
});

function arm(): void {
  const link: SocketLike = {
    binaryType: "",
    send: () => {},
    close: () => {},
    onopen: null,
    onclose: null,
    onmessage: null,
  };
  const adapter = new WailsNetworkAdapter(() => link, { admitted: () => true });
  const repo = {
    handles: {},
    find: async (url: AutomergeUrl) => {
      if (failing.has(url)) throw new Error("store stream closed");
      return handleOf(url);
    },
    create: () => {
      const url = generateAutomergeUrl();
      calls.push(`create ${url}`);
      return handleOf(url);
    },
    delete: (url: AutomergeUrl) => {
      calls.push(`delete ${url}`);
    },
  };
  workspace.attach(adapter, repo as unknown as Repo);
}

/** Starts on the saved root `saved`, whose load fails, with Go answering `state` (or the call rejecting). */
async function start(saved: string, state: string | Error): Promise<void> {
  failing = new Set([saved]);
  vi.mocked(Projects.Current).mockResolvedValue({ RootURL: saved } as never);
  if (state instanceof Error) vi.mocked(Projects.RootState).mockRejectedValue(state);
  else vi.mocked(Projects.RootState).mockResolvedValue(state as never);
  arm();
  await workspace.openStart();
}

const written = () => calls.filter((c) => !c.startsWith("root-state"));

beforeEach(() => {
  calls = [];
  workspace.handle = undefined;
  notice.hide();
  vi.mocked(Session.PendingJoin).mockResolvedValue({
    waiting: false,
    root: "",
    inviter: "",
    name: "",
  } as never);
  vi.mocked(Projects.SetRootURL).mockImplementation((async (url: string) => {
    calls.push(`set ${url}`);
  }) as never);
  vi.mocked(Projects.ReplaceMissingRoot).mockImplementation((async (old: string, url: string) => {
    calls.push(`replace ${old} ${url}`);
  }) as never);
});

describe("RT1 the start keeps a root that storage holds", () => {
  it("RT1 A-1 a failed load of a stored root never creates a document nor records a root", async () => {
    await start(generateAutomergeUrl(), "stored");
    expect(written()).toEqual([]);
    expect(workspace.handle).toBeUndefined();
  });

  it("RT1 A-1 a failed load whose storage state is unknown never creates a document nor records a root", async () => {
    await start(generateAutomergeUrl(), "unknown");
    expect(written()).toEqual([]);
    expect(workspace.handle).toBeUndefined();
  });

  it("RT1 A-1 a RootState call that rejects never creates a document nor records a root", async () => {
    await start(generateAutomergeUrl(), new Error("bridge down"));
    expect(written()).toEqual([]);
    expect(workspace.handle).toBeUndefined();
  });

  it("RT1 A-1 an empty or invalid saved root is absent: one document is created and recorded", async () => {
    // Empty: no root is recorded yet, so the first one is recorded as such.
    await start("", "absent");
    const [first] = calls;
    const fresh = first?.split(" ")[1];
    expect(calls).toEqual([`create ${fresh}`, `set ${fresh}`]);
    expect(workspace.handle?.url).toBe(fresh);

    // Invalid: Go counts it absent, and the fresh document replaces it.
    calls = [];
    await start("automerge:not-a-document", "absent");
    const made = calls[0]?.split(" ")[1];
    expect(calls).toEqual([
      `create ${made}`,
      `replace automerge:not-a-document ${made}`,
      `set ${made}`,
    ]);
    expect(workspace.handle?.url).toBe(made);
  });

  it("RT1 A-3 a root proven absent is replaced through ReplaceMissingRoot before any SetRootURL", async () => {
    const saved = generateAutomergeUrl();
    let release: () => void = () => {};
    const replaced = new Promise<void>((resolve) => {
      release = resolve;
    });
    vi.mocked(Projects.ReplaceMissingRoot).mockImplementation((async (old: string, url: string) => {
      calls.push(`replace ${old} ${url}`);
      await replaced;
      calls.push("replaced");
    }) as never);
    failing = new Set([saved]);
    vi.mocked(Projects.Current).mockResolvedValue({ RootURL: saved } as never);
    vi.mocked(Projects.RootState).mockResolvedValue("absent" as never);
    arm();
    const started = workspace.openStart();
    await vi.waitFor(() => expect(calls.some((c) => c.startsWith("replace"))).toBe(true));
    expect(
      calls.some((c) => c.startsWith("set")),
      "no SetRootURL while the replacement is pending",
    ).toBe(false);
    release();
    await started;
    const fresh = calls[0]?.split(" ")[1];
    expect(calls).toEqual([
      `create ${fresh}`,
      `replace ${saved} ${fresh}`,
      "replaced",
      `set ${fresh}`,
    ]);
  });

  it("RT1 A-4 a start that cannot open a stored root writes nothing and says so", async () => {
    await start(generateAutomergeUrl(), "stored");
    expect(written()).toEqual([]);
    expect(workspace.handle).toBeUndefined();
    expect([notice.text, notice.role]).toEqual([fr["start.rootUnreadable"], "alert"]);
  });

  it("RT1 A-3 a refused replacement removes the fresh document and says so", async () => {
    // Go refuses (the saved document became stored or unreadable meanwhile): nothing is recorded, the document made
    // for the replacement is removed, not left orphaned in the project folder, and the person is told (review m3).
    const saved = generateAutomergeUrl();
    vi.mocked(Projects.ReplaceMissingRoot).mockImplementation((async (old: string, url: string) => {
      calls.push(`replace ${old} ${url}`);
      throw new Error("project: the recorded root is not proven absent");
    }) as never);
    await start(saved, "absent");
    const fresh = calls[0]?.split(" ")[1];
    expect(calls).toEqual([`create ${fresh}`, `replace ${saved} ${fresh}`, `delete ${fresh}`]);
    expect(workspace.handle).toBeUndefined();
    expect([notice.text, notice.role]).toEqual([fr["start.rootUnreadable"], "alert"]);
  });
});
