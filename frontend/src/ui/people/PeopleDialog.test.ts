import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { workspace } from "../../workspace/workspace.svelte";
import { cleanup, click, render } from "../testkit.svelte";
import PeopleDialog from "./PeopleDialog.svelte";
import type { Member } from "./roster";

vi.mock("../../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Session: {
    Members: vi.fn(async () => null),
    SetRole: vi.fn(async () => undefined),
    Suspend: vi.fn(async () => undefined),
    Resume: vi.fn(async () => undefined),
  },
  Projects: {},
}));

beforeEach(() => {
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
  workspace.roster = [];
});

const member = (id: string, over: Partial<Member> = {}): Member => ({
  id,
  name: id,
  role: "writer",
  suspended: false,
  self: false,
  devices: [{ peer: `peer-${id}`, suspended: false }],
  ...over,
});

function show(members: Member[]) {
  workspace.roster = members;
  const host = render(PeopleDialog, { open: true });
  flushSync();
  return host;
}

describe("people panel", () => {
  it("ACL-04 a reader sees the people and their roles, and no control at all", () => {
    const host = show([
      member("alice", { role: "admin" }),
      member("lea", { role: "reader", self: true }),
    ]);
    const text = host.textContent ?? "";
    expect(text).toContain("alice");
    expect(text).toContain("Admin");
    expect(host.querySelector("select")).toBeNull();
    // Only the close button: nothing to disconnect, hand over or let back, not even disabled.
    expect([...host.querySelectorAll("button")].map((b) => b.getAttribute("aria-label"))).toEqual([
      "Fermer",
    ]);
  });

  it("ACL-01 an admin disconnects only after the question, then Go is asked", async () => {
    const { Session } = await import(
      "../../../bindings/github.com/oioio-space/inkwell/internal/app/index"
    );
    const host = show([member("alice", { role: "admin", self: true }), member("paul")]);
    const ask = [...host.querySelectorAll("button")].find(
      (b) => b.textContent?.trim() === "Déconnecter",
    );
    expect(ask).toBeDefined();
    if (ask) click(ask);
    flushSync();
    expect(Session.Suspend).not.toHaveBeenCalled();
    expect(host.textContent).toContain("Vous pourrez le laisser revenir");
    const confirm = [...host.querySelectorAll(".ask button")].find(
      (b) => b.textContent?.trim() === "Déconnecter",
    );
    if (confirm) click(confirm);
    await vi.waitFor(() => expect(Session.Suspend).toHaveBeenCalledWith("paul"));
  });

  it("ACL-03 a disconnected person shows who and offers to let them back", () => {
    const host = show([
      member("alice", { role: "admin", self: true }),
      member("paul", { suspended: true, suspendedBy: "Alice" }),
    ]);
    const labels = [...host.querySelectorAll("button")].map((b) => b.textContent?.trim());
    expect(labels).toContain("Laisser revenir");
    expect(labels).not.toContain("Déconnecter");
    expect(host.textContent).toContain("Déconnecté par Alice");
  });
});
