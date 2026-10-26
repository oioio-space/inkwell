import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { t } from "../i18n";
import { PERSON_COLORS, personIndex } from "../ui/people";
import type { DeviceClaim } from "./log";
import { actionMessage, type HDoc } from "./types";

/** What the shell knows about people: the signed member list and who is present now. */
export interface Directory {
  /** The Automerge actors this device has written under (kept on this device only): the only ones that are mine. */
  myActors?: ReadonlySet<string>;
  /** The name the member list gives to the person who owns this device, if it does. */
  nameOfPeer?: (peer: string) => string | undefined;
  /** Names announced by presence, by peer id. */
  present?: Readonly<Record<string, string>>;
}

export interface Author {
  actor: string;
  name: string;
  /** 1 to 8, the slot of the person's colour (the same one presence shows). */
  index: number;
  color: string;
  /** The device this actor says it is. */
  peer: string | undefined;
  /** This device (or one of mine). */
  mine: boolean;
  /** The actor is in the document's list: false for an actor nobody has introduced. */
  known: boolean;
}

/**
 * Who wrote what. Each device says, in a change of its own, which device it is and which name it
 * uses. That is a claim, and the word of that device only: the screen says so (« selon l'appareil
 * de Marie », X-20). A claim is read from the change's own actor, so nobody speaks for another.
 * What is mine is decided here, locally, from the actors this device has used: no claim, however
 * it names this device, can make an edit mine and hide it from my margin.
 */
export function authorBook(
  claims: Readonly<Record<string, DeviceClaim>> | undefined,
  dir: Directory,
): (actor: string) => Author {
  const known = claims ?? {};
  const cache = new Map<string, Author>();
  return (actor) => {
    let a = cache.get(actor);
    if (a) return a;
    const claim: DeviceClaim | undefined = known[actor];
    const peer = claim?.peer;
    // The name the device gave itself; only without one, what presence or the member list know.
    const name =
      claim?.name ||
      (peer ? dir.present?.[peer] : undefined) ||
      (peer ? (dir.nameOfPeer?.(peer) ?? undefined) : undefined) ||
      t("history.someone");
    const index = personIndex(peer ?? actor);
    a = {
      actor,
      name,
      index,
      color: PERSON_COLORS[index - 1] as string,
      peer,
      mine: dir.myActors?.has(actor) ?? false,
      known: claim !== undefined,
    };
    cache.set(actor, a);
    return a;
  };
}

/** The Automerge actor this device writes under in `handle`. */
export const actorOf = (handle: DocHandle<HDoc>): string | undefined => {
  const doc = handle.doc();
  return doc ? A.getActorId(doc) : undefined;
};

/**
 * Says, in a change of its own (it changes no text), who this device is: its peer and its name.
 * Done once per actor and name; nothing is written when the claim is already right.
 */
export function claimActor(
  handle: DocHandle<HDoc>,
  me: { id: string; name: string },
  known: DeviceClaim | undefined,
): void {
  const doc = handle.doc();
  if (!doc) return;
  if (known && known.peer === me.id && known.name === me.name) return;
  const message = actionMessage("claim", JSON.stringify({ name: me.name, peer: me.id }));
  // A change with no operation is not carried by every peer: the claim also stamps the time.
  handle.change(
    (d) => {
      d.meta ??= {};
      d.meta.claimedAt = Date.now();
    },
    { message },
  );
}
