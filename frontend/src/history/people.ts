import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { t } from "../i18n";
import { PERSON_COLORS, personIndex } from "../ui/people";
import { actionMessage, type Claim, type HDoc } from "./types";

/** What the shell knows about people: the signed member list and who is present now. */
export interface Directory {
  /** This device's peer id. */
  myPeer: string;
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
 * Who wrote what. Each device writes, for its own Automerge actor, which device it is and which
 * name it uses (`meta.actors`). That is a claim: nothing checks it, so the attribution is only as
 * true as that device's word, and the screen says so (« selon l'appareil de Marie », X-20). The
 * member list and presence give the name when they know the device.
 */
export function authorBook(doc: HDoc | undefined, dir: Directory): (actor: string) => Author {
  const claims = doc?.meta?.actors ?? {};
  const cache = new Map<string, Author>();
  return (actor) => {
    let a = cache.get(actor);
    if (a) return a;
    const claim: Claim | undefined = claims[actor];
    const peer = claim?.peer;
    const name =
      (peer ? (dir.nameOfPeer?.(peer) ?? undefined) : undefined) ||
      claim?.name ||
      (peer ? dir.present?.[peer] : undefined) ||
      t("history.someone");
    const index = personIndex(peer ?? actor);
    a = {
      actor,
      name,
      index,
      color: PERSON_COLORS[index - 1] as string,
      peer,
      mine: peer !== undefined && peer === dir.myPeer,
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
 * Says, in the document, who this device is: its peer and its name, for its current actor. Done
 * once per actor and name; nothing is written when the claim is already right.
 */
export function claimActor(handle: DocHandle<HDoc>, me: { id: string; name: string }): void {
  const doc = handle.doc();
  if (!doc) return;
  const actor = A.getActorId(doc);
  const old = doc.meta?.actors?.[actor];
  if (old && old.peer === me.id && old.name === me.name) return;
  handle.change(
    (d) => {
      d.meta ??= {};
      d.meta.actors ??= {};
      d.meta.actors[actor] = { name: me.name, peer: me.id, at: Date.now() };
    },
    { message: actionMessage("claim", me.name) },
  );
}
