import { describe, expect, it } from "vitest";
import { accessOf, type Member, rowsOf } from "./roster";

const member = (id: string, over: Partial<Member> = {}): Member => ({
  id,
  name: id,
  role: "writer",
  suspended: false,
  self: false,
  devices: [{ peer: `peer-${id}`, suspended: false }],
  ...over,
});

const alice = member("alice", { role: "admin", self: true });
const aliceElsewhere = { ...alice, self: false };
const paul = member("paul");
const lea = member("lea", { role: "reader" });

describe("rowsOf: what an admin may do", () => {
  const rows = rowsOf([alice, paul, lea], new Set());

  it("ACL-04 offers no action on oneself", () => {
    const own = rows[0];
    expect(own?.actions).toEqual({ roles: [], suspend: false, resume: false, transfer: false });
  });

  it("offers the other roles, never the current one", () => {
    expect(rows[1]?.actions.roles).toEqual(["reader", "admin"]);
    expect(rows[2]?.actions.roles).toEqual(["writer", "admin"]);
  });

  it("offers to disconnect a connected person and to let a disconnected one back", () => {
    expect(rows[1]?.actions.suspend).toBe(true);
    expect(rows[1]?.actions.resume).toBe(false);
    const back = rowsOf([alice, { ...paul, suspended: true }], new Set())[1];
    expect(back?.actions.suspend).toBe(false);
    expect(back?.actions.resume).toBe(true);
  });

  it("does not offer the admin role to a disconnected person: let them back first", () => {
    const row = rowsOf([alice, { ...paul, suspended: true }], new Set())[1];
    expect(row?.actions.roles).toEqual(["reader"]);
  });

  it("ACL-10 the creator, and only the creator, may hand over to a connected person", () => {
    expect(rows[1]?.actions.transfer).toBe(true);
    const bob = member("bob", { role: "admin", self: true });
    const second = rowsOf([aliceElsewhere, bob, paul], new Set())[2];
    expect(second?.actions.transfer).toBe(false);
    const gone = rowsOf([alice, { ...paul, suspended: true }], new Set())[1];
    expect(gone?.actions.transfer).toBe(false);
  });

  it("ACL-05 acts only on people who joined after oneself", () => {
    const senior = member("zoe", { role: "admin" });
    const me = member("me", { role: "admin", self: true });
    const [first, own, junior] = rowsOf([senior, me, paul], new Set());
    expect(first?.actions).toEqual({ roles: [], suspend: false, resume: false, transfer: false });
    expect(own?.actions.suspend).toBe(false);
    expect(junior?.actions.suspend).toBe(true);
  });
});

describe("rowsOf: nobody else gets controls", () => {
  const none = { roles: [], suspend: false, resume: false, transfer: false };

  it("ACL-04 a writer has none", () => {
    const rows = rowsOf([aliceElsewhere, { ...paul, self: true }, lea], new Set());
    for (const row of rows) expect(row.actions).toEqual(none);
  });

  it("a reader has none", () => {
    const rows = rowsOf([aliceElsewhere, paul, { ...lea, self: true }], new Set());
    for (const row of rows) expect(row.actions).toEqual(none);
  });

  it("a disconnected admin has none: what they sign would be dead", () => {
    const adm = member("adm", { role: "admin", self: true, suspended: true });
    const rows = rowsOf([aliceElsewhere, adm, paul], new Set());
    for (const row of rows) expect(row.actions).toEqual(none);
  });

  it("before the list is known there is nothing to act on", () => {
    expect(rowsOf([], new Set())).toEqual([]);
    for (const row of rowsOf([paul], new Set())) expect(row.actions).toEqual(none);
  });
});

describe("rowsOf: online", () => {
  it("is online when one device is connected, or it is this device", () => {
    const rows = rowsOf([alice, paul, lea], new Set(["peer-paul"]));
    expect(rows.map((r) => r.online)).toEqual([true, true, false]);
  });

  it("a disconnected person is not shown as online", () => {
    const rows = rowsOf([alice, { ...paul, suspended: true }], new Set(["peer-paul"]));
    expect(rows[1]?.online).toBe(false);
  });
});

describe("accessOf", () => {
  it("lets a writer, an admin and an unknown role edit", () => {
    expect(accessOf("writer", false)).toBeUndefined();
    expect(accessOf("admin", false)).toBeUndefined();
    expect(accessOf(undefined, false)).toBeUndefined();
  });

  it("ACL-08 locks a reader", () => {
    expect(accessOf("reader", false)).toBe("reader");
  });

  it("ACL-01 a disconnected person is locked whatever their role, and says why", () => {
    expect(accessOf("writer", true)).toBe("suspended");
    expect(accessOf("reader", true)).toBe("suspended");
  });
});
