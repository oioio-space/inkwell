import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import fr from "../i18n/fr.json";
import { WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { workspace } from "../workspace/workspace.svelte";
import AccessBanner from "./people/AccessBanner.svelte";
import type { Standing } from "./people/roster";
import { cleanup, render } from "./testkit.svelte";

/**
 * UX-W58a-2 (gates/W58a/UX.md): a person who has just pasted an invitation, in a folder that
 * is a copy of the project, is told so while the member list that names her is on its way. Without
 * it the banner kept saying « ce document vient d'un autre appareil » (and offered the invitation
 * she had just used) for as long as the list took to arrive: 14 s measured, with « Synchronisé »
 * in the header.
 */

vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: { Invite: vi.fn(), Join: vi.fn(), Standing: vi.fn(), Members: vi.fn(async () => []) },
}));

const text = (el: Element) => el.textContent?.replace(/\s+/g, " ").trim() ?? "";
let standingNow: Standing = "notNamed";

beforeEach(() => {
  vi.mocked(Session.Standing).mockImplementation((async () => standingNow) as never);
  vi.mocked(Session.Join).mockResolvedValue("host" as never);
  vi.spyOn(WailsNetworkAdapter.prototype, "projectOf").mockReturnValue("automerge:x" as never);
  vi.spyOn(workspace, "open").mockResolvedValue(undefined as never);
  // Not started: the workspace has no adapter, join() reaches projectOf through this stand-in.
  (workspace as unknown as { attach: (a: unknown, r: unknown) => void }).attach(
    new WailsNetworkAdapter(() => ({}) as never, { admitted: () => true }),
    {},
  );
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  workspace.standing = undefined;
  workspace.listPending = false;
});

describe("UX-W58a-2 an invitation used, the list not yet here", () => {
  it("UX-W58a-2 says the invitation is accepted and drops the action, then goes away", async () => {
    workspace.standing = "notNamed";
    standingNow = "notNamed"; // the copy's list does not name her until the host's reaches it
    await workspace.join("invitation");
    flushSync();
    const host = render(AccessBanner, {});
    const banner = host.querySelector("[data-testid=access-banner]");
    expect(banner && text(banner), "UX-W58a-2 waiting sentence").toBe(fr["foreign.joinWaiting"]);
    expect([...host.querySelectorAll("button")], "UX-W58a-2 no second ask").toEqual([]);

    standingNow = "member"; // the list arrives: the roles event asks again
    await workspace.refreshStanding();
    flushSync();
    expect(workspace.listPending, "UX-W58a-2 cleared once she is named").toBe(false);
    expect(workspace.access).toBeUndefined();
  });

  it("UX-W58a-2 a plain foreign copy keeps its sentence and its action", () => {
    workspace.standing = "notNamed";
    const host = render(AccessBanner, {});
    expect(text(host.querySelector("[data-testid=access-banner]") as Element)).toBe(
      fr["foreign.notNamed"],
    );
    expect(host.querySelectorAll("button")).toHaveLength(1);
  });
});
