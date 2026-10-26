import { next as A } from "@automerge/automerge";
import type { DocHandle, Repo } from "@automerge/automerge-repo";
import { t } from "../i18n";
import { applyEdits, changeBetween, planUndo, type Taken, undoIntegration } from "../store/merge";
import { textAt } from "./log";
import { actionMessage, type HDoc, type Heads, type Variant, type Version } from "./types";

// Nothing here deletes history: going back, merging and abandoning each add an operation.

const heads = (handle: DocHandle<HDoc>): Heads => {
  const doc = handle.doc();
  return doc ? A.getHeads(doc) : [];
};

const actorOf = (handle: DocHandle<HDoc>): string => {
  const doc = handle.doc();
  return doc ? A.getActorId(doc) : "";
};

/** Names a version: a point of the history that can be found again. */
export function nameVersion(
  handle: DocHandle<HDoc>,
  name: string,
  at: Heads = heads(handle),
): Version {
  const version: Version = { name: name.trim(), heads: at, by: actorOf(handle), at: Date.now() };
  handle.change(
    (d) => {
      d.meta ??= {};
      d.meta.versions ??= [];
      d.meta.versions.push(version);
    },
    { message: actionMessage("version", version.name) },
  );
  return version;
}

export interface Reverted {
  /** The heads before and after: enough to take the revert back with Ctrl+Z. */
  before: Heads;
  after: Heads;
}

/**
 * Goes back to the text of a moment, as one change that rewrites the text (HIS-07). The text
 * before is first kept as a version, so "now" can be found again; typing others did meanwhile
 * is kept by the merge, never lost (HIS-08). Returns undefined when the text already is that.
 */
export function revertTo(handle: DocHandle<HDoc>, at: Heads, label: string): Reverted | undefined {
  const doc = handle.doc();
  if (!doc) return undefined;
  const target = textAt(doc, at);
  if (target === doc.text) return undefined;
  const before = A.getHeads(doc);
  handle.change(
    (d) => {
      d.meta ??= {};
      d.meta.versions ??= [];
      d.meta.versions.push({
        name: t("history.before.revert", { name: label }),
        heads: before,
        by: A.getActorId(doc),
        at: Date.now(),
        auto: true,
      });
      A.updateText(d, ["text"], target);
    },
    { message: actionMessage("revert", label) },
  );
  return { before, after: heads(handle) };
}

/** What a change did, to take it back: from `before` to `after`, as it stands now. */
export const whatChanged = (
  handle: DocHandle<HDoc>,
  before: Heads,
  after: Heads,
): Taken | undefined => {
  const doc = handle.doc();
  return doc ? changeBetween(doc, before, after) : undefined;
};

/** The text after taking a change back, without applying it: the preview (DIF-08). */
export function previewUndo(handle: DocHandle<HDoc>, done: Taken): string {
  const doc = handle.doc();
  return doc ? applyEdits(doc.text, planUndo(doc, done)) : "";
}

/** « Annuler cette modification »: the inverse of one change, in one change; later ones stay. */
export function undoChange(handle: DocHandle<HDoc>, done: Taken, label: string): void {
  undoIntegration(handle as DocHandle<{ text: string }>, done, actionMessage("undo", label));
}

/** Takes a revert back (Ctrl+Z) when it is still the last thing that happened to the document. */
export function undoRevert(handle: DocHandle<HDoc>, reverted: Reverted): boolean {
  const doc = handle.doc();
  if (!doc || A.getHeads(doc).join() !== reverted.after.join()) return false;
  const done = changeBetween(doc, reverted.before, reverted.after);
  undoIntegration(
    handle as DocHandle<{ text: string }>,
    done,
    actionMessage("undo", t("history.undo.revert")),
  );
  return true;
}

/**
 * Starts a variant: a document of its own, cut from the main one at `at`, listed in the main
 * document so every person can find it (HIS-10, HIS-16).
 */
export function fork(
  repo: Repo,
  handle: DocHandle<HDoc>,
  name: string,
  at: Heads = heads(handle),
): {
  variant: Variant;
  handle: DocHandle<HDoc>;
} {
  const doc = handle.doc();
  if (!doc) throw new Error("no document");
  const cut = A.clone(at.length > 0 ? A.view(doc, at) : doc);
  const copy = repo.import<HDoc>(A.save(cut));
  const variant: Variant = {
    url: copy.url,
    name: name.trim(),
    from: at,
    by: A.getActorId(doc),
    at: Date.now(),
    state: "open",
  };
  handle.change(
    (d) => {
      d.meta ??= {};
      d.meta.variants ??= [];
      d.meta.variants.push(variant);
    },
    { message: actionMessage("variant", variant.name) },
  );
  return { variant, handle: copy };
}

const indexOfVariant = (doc: HDoc, url: string): number =>
  doc.meta?.variants?.findIndex((v) => v.url === url) ?? -1;

function setState(
  handle: DocHandle<HDoc>,
  url: string,
  state: Variant["state"],
  mergedHeads?: Heads,
): void {
  handle.change((d) => {
    const i = indexOfVariant(d, url);
    const v = d.meta?.variants?.[i];
    if (!v) return;
    v.state = state;
    if (mergedHeads) v.mergedHeads = mergedHeads;
  });
}

/**
 * Merges a variant into the main document (HIS-11): a version "Avant fusion de X" is kept, the
 * texts are joined by the CRDT, the variant is marked merged. Merging again does nothing (HIS-12).
 */
export function mergeVariant(
  main: DocHandle<HDoc>,
  variantHandle: DocHandle<HDoc>,
  variant: Variant,
): boolean {
  const doc = main.doc();
  if (!doc) return false;
  const entry = doc.meta?.variants?.[indexOfVariant(doc, variant.url)];
  if (!entry || entry.state === "merged") return false;
  const before = A.getHeads(doc);
  main.change(
    (d) => {
      d.meta ??= {};
      d.meta.versions ??= [];
      d.meta.versions.push({
        name: t("history.before.merge", { name: variant.name }),
        heads: before,
        by: A.getActorId(doc),
        at: Date.now(),
        auto: true,
      });
    },
    { message: actionMessage("merge", variant.name) },
  );
  main.merge(variantHandle);
  setState(main, variant.url, "merged", heads(main));
  return true;
}

/** Sets a variant aside: hidden by default, kept, and brought back by {@link reopenVariant}. */
export const abandonVariant = (main: DocHandle<HDoc>, url: string): void =>
  setState(main, url, "abandoned");

export const reopenVariant = (main: DocHandle<HDoc>, url: string): void =>
  setState(main, url, "open");
