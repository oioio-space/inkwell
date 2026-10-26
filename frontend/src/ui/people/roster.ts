/** The people of a project as the « Personnes » panel shows them (Go: `app.Person`). */

export type Role = "reader" | "writer" | "admin";

export const ROLES: readonly Role[] = ["reader", "writer", "admin"];

export interface Member {
  /** Names the person in `Suspend`, `Resume` and `SetRole`. */
  id: string;
  /** The name an admin gave the person; empty for the project's creator. */
  name: string;
  role: Role;
  suspended: boolean;
  /** Who disconnected the person, and when this device heard of it (see ADR 0021). */
  suspendedBy?: string;
  suspendedByPeer?: string;
  suspendedAt?: string;
  /** True for the person that this device belongs to. */
  self: boolean;
  devices: { peer: string; suspended: boolean }[];
}

/** What the viewer may do with one person. All false and empty means: show no control at all. */
export interface Actions {
  /** The roles the person can be given: never the current one. */
  roles: Role[];
  suspend: boolean;
  resume: boolean;
  /** Make the person an admin and step down: only the project's creator. */
  transfer: boolean;
}

export interface Row {
  member: Member;
  online: boolean;
  actions: Actions;
}

const NONE: Actions = { roles: [], suspend: false, resume: false, transfer: false };

/**
 * One row per person, in the order Go gives them (seniority: the creator first). An admin acts
 * only on people who joined after them (ADR 0017) and never on themselves; a disconnected admin
 * acts on nobody (what they would sign is dead, ADR 0021). Anyone else gets `NONE`: a reader or a
 * writer sees no control, not even a disabled one.
 *
 * @param connected the devices currently connected to this one.
 */
export function rowsOf(members: readonly Member[], connected: ReadonlySet<string>): Row[] {
  const me = members.findIndex((m) => m.self);
  const viewer = members[me];
  const admin = viewer?.role === "admin" && !viewer.suspended;
  return members.map((member, i) => {
    const online =
      !member.suspended && (member.self || member.devices.some((d) => connected.has(d.peer)));
    const mayAct = admin && i > me;
    if (!mayAct) return { member, online, actions: NONE };
    return {
      member,
      online,
      actions: {
        roles: ROLES.filter((r) => r !== member.role && !(r === "admin" && member.suspended)),
        suspend: !member.suspended,
        resume: member.suspended,
        transfer: me === 0 && !member.suspended,
      },
    };
  });
}

/** Why this device cannot edit, if it cannot: its role, or a suspension (which wins). */
export function accessOf(
  role: Role | undefined,
  suspended: boolean,
): "reader" | "suspended" | undefined {
  if (suspended) return "suspended";
  return role === "reader" ? "reader" : undefined;
}
