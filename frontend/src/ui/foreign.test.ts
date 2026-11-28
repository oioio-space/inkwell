import { Repo } from "@automerge/automerge-repo";
import { deleteCharBackward, selectAll } from "@codemirror/commands";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { TYPST } from "../editor/format/testkit";
import fr from "../i18n/fr.json";
import { withMutant } from "../mutant";
import { encodeEnvelope, encodeRoles, Kind } from "../net/envelope";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { workspace } from "../workspace/workspace.svelte";
import Editor from "./Editor.svelte";
import { session } from "./editorSession.svelte";
import { notice } from "./notice.svelte";
import AccessBanner from "./people/AccessBanner.svelte";
import type { Role, Standing } from "./people/roster";
import ShareDialog from "./ShareDialog.svelte";
import { cleanup, click, render } from "./testkit.svelte";

/**
 * W58a: a project whose member list does not name this device. Rules N-2 (a refusal is a key,
 * never Go's text), N-3 (locked until the rights are known, then read-only, one sentence), N-4 (the
 * invitation controls wait for an admin role). Each rule's check also runs on a copy of the real
 * source with one edit, and must fail there (« W58a mutant … »). W58a's rule N-15 (one action, no
 * take-over) is reversed by PC1a (ADR 0043, part 2): the banner's two actions and the take-over
 * are checked in projectChange.test.ts.
 */

vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {
    Invite: vi.fn(),
    InvitationState: vi.fn(),
    Join: vi.fn(),
    Standing: vi.fn(),
    CanTakeOver: vi.fn(async () => true),
    Members: vi.fn(async () => []),
  },
}));

type Workspace = typeof workspace;

type ShareComponent = typeof ShareDialog;

const text = (el: Element) => el.textContent?.replace(/\s+/g, " ").trim() ?? "";
const button = (host: Element, label: string) =>
  [...host.querySelectorAll("button")].find((b) => text(b) === label) as HTMLButtonElement;

/** The repo `arm` gives the workspace: tests may put document handles in it. */
const armedRepo: { handles: Record<string, unknown> } = { handles: {} };
let standingNow: Standing = "member";
let deliver: (frame: Uint8Array) => void = () => {};

/** Delivers the roles of the member list as Go reports them: this device has `role`, or none. */
function reportRoles(role: Role | undefined) {
  deliver(
    encodeEnvelope({
      kind: Kind.Roles,
      peer: "me",
      payload: encodeRoles(role ? [{ peer: "me", role, suspended: false }] : []),
    }),
  );
}

/** Gives `w` a network adapter on a fake link, Go's standing, and (maybe) its role. */
async function arm(w: Workspace, standing: Standing, role?: Role, ask = true): Promise<void> {
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
  w.myId = "me";
  armedRepo.handles = {};
  w.attach(adapter, armedRepo as unknown as Repo);
  adapter.connect("me" as never);
  deliver = (frame) => socket?.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  if (role) reportRoles(role);
  if (ask) await w.refreshStanding();
  flushSync();
}

beforeEach(() => {
  vi.mocked(Session.Standing).mockImplementation((async () => standingNow) as never);
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
    this.dispatchEvent(new Event("close"));
  };
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.mocked(Session.Invite).mockReset();
  workspace.standing = undefined;
  workspace.invitation = "";
  workspace.invitationState = "open";
  workspace.error = "";
  notice.hide();
});

describe("W58a refusals (N-2)", () => {
  /** What a refusal and a bridge error show: our sentence, never Go's text. */
  async function refusalChecks(w: Workspace): Promise<void> {
    const logged = [
      vi.spyOn(console, "log"),
      vi.spyOn(console, "warn"),
      vi.spyOn(console, "error"),
      vi.spyOn(console, "info"),
    ];
    const invite = vi.mocked(Session.Invite);
    invite.mockResolvedValueOnce({ text: "", refused: "notMember" } as never);
    await w.host();
    expect(w.error, "N-2 refused by key").toBe(fr["share.host.notMember"]);
    invite.mockResolvedValueOnce({ text: "", refused: "notAdmin" } as never);
    await w.host();
    expect(w.error, "N-2 refused by key").toBe(fr["share.host.adminOnly"]);
    expect(w.invitation).toBe("");
    // A failure of the bridge itself reaches the webview as the raw Go text.
    invite.mockRejectedValueOnce(new Error("signer is not an admin"));
    await w.host();
    expect(w.error, "N-2 refused by key").toBe(fr["error.host"]);
    expect(w.error, "N-2 refused by key").not.toContain("signer");
    for (const spy of logged) expect(spy).not.toHaveBeenCalled();
  }

  it("W58a a refusal shows its sentence, never Go text", async () => {
    await refusalChecks(workspace);
  });

  it("W58a mutant N-2 raw error is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "w58mutant-n2",
      '} catch {\n      this.error = t("error.host");',
      "} catch (err) {\n      this.error = String(err);",
      async (m) => {
        await expect(refusalChecks(m.workspace)).rejects.toThrow(/N-2 refused by key/);
      },
    );
  });
});

describe("W58a access (N-3)", () => {
  async function readOnlyChecks(w: Workspace, standing: Standing): Promise<void> {
    await arm(w, standing);
    expect(w.access, "N-3 access follows the standing").toBe(standing);
  }

  it("W58a not a member: read-only, one sentence", async () => {
    await readOnlyChecks(workspace, "notNamed");
    const host = render(AccessBanner, {});
    const banner = host.querySelector("[data-testid=access-banner]");
    expect(banner && text(banner)).toBe(fr["foreign.notNamed"]);
  });

  it("W58a removed: its own sentence", async () => {
    await readOnlyChecks(workspace, "removed");
    const host = render(AccessBanner, {});
    const banner = host.querySelector("[data-testid=access-banner]");
    expect(banner && text(banner)).toBe(fr["foreign.removed"]);
    expect([...host.querySelectorAll("button")].map(text)).toEqual([
      fr["foreign.ask"],
      fr["foreign.takeOver"],
    ]);
  });

  it("W58a a member sees no foreign action", async () => {
    await arm(workspace, "member", "writer");
    const host = render(AccessBanner, {});
    expect(host.querySelector("button")).toBeNull();
    await arm(workspace, "member", "reader");
    expect(workspace.access).toBe("reader");
    expect(render(AccessBanner, {}).querySelector("button")).toBeNull();
  });

  it("W58a mutant N-3 access is caught", async () => {
    await withMutant<{ workspace: Workspace }, void>(
      "workspace/workspace.svelte.ts",
      "w58mutant-n3",
      "accessOf(me?.role, me?.suspended ?? false, this.standing)",
      "accessOf(me?.role, me?.suspended ?? false)",
      async (m) => {
        await expect(readOnlyChecks(m.workspace, "notNamed")).rejects.toThrow(
          /N-3 access follows the standing/,
        );
      },
    );
  });
});

describe("W58a the lock at start (N-3)", () => {
  const repos: Repo[] = [];
  afterEach(() => repos.splice(0).forEach((r) => void r.shutdown()));

  /** The real editor on a document holding "hello", as the shell mounts it. */
  function mountEditor(Component: typeof Editor) {
    const repo = new Repo({ network: [] });
    repos.push(repo);
    const handle = repo.create<{ text: string }>({ text: "hello" });
    render(Component, { handle, me: { id: "me", name: "Me" }, lang: TYPST });
    const view = session.view;
    if (!view) throw new Error("the editor did not mount");
    return view;
  }

  /** What a locked editor still does: shows, selects and copies, and refuses keys. */
  function lockedChecks(view: NonNullable<typeof session.view>, why: string) {
    expect(view.state.readOnly, why).toBe(true);
    expect(view.contentDOM.getAttribute("contenteditable"), "text stays selectable").toBe("true");
    expect(deleteCharBackward(view), why).toBe(false);
    expect(view.state.doc.toString(), "the text shows").toBe("hello");
    selectAll(view);
    const { from, to } = view.state.selection.main;
    expect(view.state.sliceDoc(from, to), "the selection can be copied").toBe("hello");
  }

  /** Standing delayed: locked before it, still locked with the role but no standing, then by right. */
  async function lockChecks(Component: typeof Editor): Promise<void> {
    // Every ask is held (the roles event asks again) until the test lets Go answer.
    const held: ((s: Standing) => void)[] = [];
    const answer = (s: Standing) => held.forEach((resolve) => void resolve(s));
    vi.mocked(Session.Standing).mockImplementation(
      (() => new Promise<Standing>((resolve) => held.push(resolve))) as never,
    );
    await arm(workspace, "member", undefined, false);
    const asked = workspace.refreshStanding();
    const view = mountEditor(Component);
    const banner = render(AccessBanner, {});
    flushSync();
    lockedChecks(view, "N-3 lock before the rights");
    expect(banner.querySelector("[data-testid=access-banner]"), "no banner flashes").toBeNull();
    reportRoles("writer");
    flushSync();
    expect(view.state.readOnly, "N-3 lock before the rights: the role alone is not enough").toBe(
      true,
    );
    answer("member");
    await asked;
    flushSync();
    expect(view.state.readOnly, "a member who may write can type").toBe(false);
    expect(banner.querySelector("[data-testid=access-banner]")).toBeNull();
  }

  it("W58a the editor is locked until standing and role are known", async () => {
    await lockChecks(Editor);
    // A device the list does not name stays locked, with the sentence.
    cleanup();
    workspace.standing = undefined;
    vi.mocked(Session.Standing).mockImplementation((async () => standingNow) as never);
    await arm(workspace, "notNamed");
    const view = mountEditor(Editor);
    const banner = render(AccessBanner, {});
    lockedChecks(view, "N-3 not a member stays locked");
    expect(banner.querySelector("[data-testid=access-banner]")?.textContent).toBe(
      fr["foreign.notNamed"],
    );
  });

  it("W58a the editor unlocks when the role arrives after the standing", async () => {
    // The other order of lockChecks: Go answers « member » first, the role comes later.
    vi.mocked(Session.Standing).mockImplementation((async () => "member") as never);
    await arm(workspace, "member", undefined, false);
    await workspace.refreshStanding();
    const view = mountEditor(Editor);
    flushSync();
    expect(view.state.readOnly, "N-3 role not reported yet: still locked").toBe(true);
    reportRoles("writer");
    flushSync();
    expect(view.state.readOnly, "N-3 the role arrived after the standing: writable").toBe(false);
  });

  it("W58a read-only text can be selected and copied", async () => {
    await arm(workspace, "notNamed");
    lockedChecks(mountEditor(Editor), "N-3 read-only");
  });

  it("W58a mutant N-3 lock before the rights is caught", async () => {
    await withMutant<{ default: typeof Editor }, void>(
      "ui/Editor.svelte",
      "w58mutant-n3lock",
      "editor?.setReadOnly(workspace.locked)",
      "editor?.setReadOnly(workspace.access !== undefined)",
      async (m) => {
        await expect(lockChecks(m.default)).rejects.toThrow(/N-3 lock before the rights/);
      },
    );
  });
});

describe("W58a invite capability (N-4)", () => {
  /** The share dialog with a used invitation (so the renew button is there), for one situation. */
  async function share(Dialog: ShareComponent, standing: Standing, role?: Role) {
    await arm(workspace, standing, role);
    workspace.invitation = "invitation";
    workspace.invitationState = "used";
    vi.mocked(Session.InvitationState).mockResolvedValue("used");
    const host = render(Dialog, { open: true });
    // The dialog shows the invitation's row once Go has answered about it (PC1b, J-8).
    await vi.waitFor(() => {
      flushSync();
      expect(button(host, fr["share.renew"])).toBeDefined();
    });
    return {
      text: text(host),
      invite: button(host, fr["share.host"]),
      renew: button(host, fr["share.renew"]),
    };
  }

  it("W58a invite capability: loading", async () => {
    const s = await share(ShareDialog, "member");
    expect(s.text).toContain(fr["share.host.loading"]);
    expect(s.invite.disabled, "N-4 invite waits").toBe(true);
    expect(s.renew.disabled, "N-4 renew waits").toBe(true);
  });

  it("W58a invite capability: admin", async () => {
    const s = await share(ShareDialog, "member", "admin");
    expect(s.text).toContain(fr["share.host.help"]);
    expect(s.text).not.toContain(fr["share.host.loading"]);
    expect(s.invite.disabled).toBe(false);
    expect(s.renew.disabled).toBe(false);
  });

  it("W58a invite capability: non-admin", async () => {
    const s = await share(ShareDialog, "member", "writer");
    expect(s.text).toContain(fr["share.host.adminOnly"]);
    expect(s.invite.disabled, "N-4 invite is for an admin").toBe(true);
    expect(s.renew.disabled, "N-4 renew is for an admin").toBe(true);
  });

  it("W58a invite capability: not a member", async () => {
    const s = await share(ShareDialog, "notNamed");
    expect(s.text).toContain(fr["share.host.notMember"]);
    expect(s.text).toContain(fr["share.join.foreignHelp"]);
    expect(s.invite.disabled).toBe(true);
    expect(s.renew.disabled).toBe(true);
  });

  it("W58a mutant N-4 renew is caught", async () => {
    await withMutant<{ default: ShareComponent }, void>(
      "ui/ShareDialog.svelte",
      "w58mutant-n4",
      'disabled={!workspace.mayInvite} onclick={() => workspace.host()}>{t("share.renew")}',
      'onclick={() => workspace.host()}>{t("share.renew")}',
      async (m) => {
        const s = await share(m.default, "member", "writer");
        expect(() => expect(s.renew.disabled, "N-4 renew is for an admin").toBe(true)).toThrow(
          /N-4 renew is for an admin/,
        );
      },
    );
  });
});

describe("W58a standing and the share dialog (N-1)", () => {
  it("W58a a join asks for the standing again", async () => {
    await arm(workspace, "notNamed");
    expect(workspace.access).toBe("notNamed");
    standingNow = "member"; // Go follows the new project before the reply of Join
    vi.mocked(Session.Join).mockResolvedValue({ peer: "host", restart: false } as never);
    vi.spyOn(WailsNetworkAdapter.prototype, "projectOf").mockReturnValue("automerge:x" as never);
    vi.spyOn(workspace, "open").mockResolvedValue(undefined as never);
    await workspace.join("invitation");
    expect(workspace.standing, "N-1 standing read again after a join").toBe("member");
  });

  it("W58a the indicator is refreshed when a document becomes ready", async () => {
    // The repo emits nothing when a handle turns ready after its heads changed: after a join the
    // few events left would otherwise leave « Vérification… » on screen for a quarter of a minute.
    await arm(workspace, "member");
    let ready: () => void = () => {};
    armedRepo.handles = {
      d: {
        on: () => {},
        isReady: () => false,
        doc: () => undefined,
        whenReady: () => new Promise<void>((resolve) => (ready = resolve)),
      },
    };
    workspace.refreshSync(); // meets the handle
    const refreshed = vi.spyOn(workspace, "refreshSync");
    ready();
    await vi.waitFor(() => expect(refreshed, "indicator refreshed on ready").toHaveBeenCalled());
  });

  it("W58a ask for an invitation opens the join dialog", async () => {
    await arm(workspace, "notNamed");
    const onAsk = vi.fn();
    const banner = render(AccessBanner, { onAsk });
    click(button(banner, fr["foreign.ask"]));
    expect(onAsk).toHaveBeenCalledTimes(1);
    const dialog = render(ShareDialog, { open: true });
    expect(dialog.querySelector("dialog")?.hasAttribute("open")).toBe(true);
    expect(text(dialog)).toContain(fr["share.join.foreignHelp"]);
    expect(dialog.querySelector("input[required]")).not.toBeNull();
  });
});
