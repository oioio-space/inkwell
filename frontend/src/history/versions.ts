import { next as A } from "@automerge/automerge";
import type { DocHandle, Repo } from "@automerge/automerge-repo";
import { t } from "../i18n";
import { applyEdits } from "../store/chunked";
import { growthRefused, tooBigToEdit } from "../store/limits";
import {
  applyEditsToText,
  changeBetween,
  integrate,
  planRewrite,
  planUndo,
  type Skip,
  type Skipped,
  type Taken,
  undoIntegration,
} from "../store/merge";
import { textAt } from "./log";
import { actionMessage, type HDoc, type Heads, type Variant, type Version } from "./types";

// Nothing here deletes history: going back, merging and abandoning each add an operation.
// Every write goes through the size gate of ADR 0028 and the chunked writer, and takes the
// `skip` hook of the locked zones (ADR 0022) the way an outside edit does.

/** The action would put more text into the document than it may hold (ADR 0028 D1). */
export class TooBig extends Error {
  constructor() {
    super("too big");
    this.name = "TooBig";
  }
}

/** The gate: refuses before Automerge sees anything. */
function mayWrite(current: number, after = current): void {
  if (tooBigToEdit(current) || tooBigToEdit(after) || growthRefused(current, after)) {
    throw new TooBig();
  }
}

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
  /** Edits a locked zone held back (the rest of the text did go back). */
  held: number;
}

export interface Guarded {
  /** Whether a stretch of the text is in a locked zone. */
  skip?: Skip;
}

/**
 * Goes back to the text of a moment, as one change (HIS-07). The text before is first kept as a
 * version, so "now" can be found again; typing others did meanwhile is kept by the merge, never
 * lost (HIS-08). The text is rewritten by line regions, not replaced whole, and in pieces when a
 * big region goes back. Returns undefined when the text already is that.
 * @throws {TooBig} when the document or the result is past the size limits.
 */
export function revertTo(
  handle: DocHandle<HDoc>,
  at: Heads,
  label: string,
  opts: Guarded = {},
): Reverted | undefined {
  const doc = handle.doc();
  if (!doc) return undefined;
  mayWrite(doc.text.length);
  const target = textAt(doc, at);
  mayWrite(doc.text.length, target.length);
  if (target === doc.text) return undefined;
  const before = A.getHeads(doc);
  const actor = A.getActorId(doc);
  const plan = planRewrite(doc.text, target, opts.skip);
  applyEdits(handle, plan.edits, ["text"], {
    message: actionMessage("revert", label),
    first: (d) => {
      const doc = d as unknown as HDoc;
      doc.meta ??= {};
      const meta = doc.meta;
      meta.versions ??= [];
      meta.versions.push({
        name: t("history.before.revert", { name: label }),
        heads: before,
        by: actor,
        at: Date.now(),
        auto: true,
      });
    },
  });
  return { before, after: heads(handle), held: plan.skipped.length };
}

/** What a change did, to take it back: from `before` to `after`, as it stands now. */
export const whatChanged = (
  handle: DocHandle<HDoc>,
  before: Heads,
  after: Heads,
): Taken | undefined => {
  const doc = handle.doc();
  return doc && !tooBigToEdit(doc.text.length) ? changeBetween(doc, before, after) : undefined;
};

/** The text after taking a change back, without applying it: the preview (DIF-08). */
export function previewUndo(handle: DocHandle<HDoc>, done: Taken, opts: Guarded = {}): string {
  const doc = handle.doc();
  return doc ? applyEditsToText(doc.text, planUndo(doc, done, opts.skip).edits) : "";
}

/**
 * « Annuler cette modification »: the inverse of one change, in one change; later ones stay.
 * Returns how many edits a locked zone held back.
 * @throws {TooBig}
 */
export function undoChange(
  handle: DocHandle<HDoc>,
  done: Taken,
  label: string,
  opts: Guarded = {},
): number {
  const doc = handle.doc();
  if (doc) mayWrite(doc.text.length);
  return undoIntegration(handle, done, {
    message: actionMessage("undo", label),
    ...(opts.skip ? { skip: opts.skip } : {}),
  }).length;
}

/** Takes a revert back (Ctrl+Z) when it is still the last thing that happened to the document. */
export function undoRevert(handle: DocHandle<HDoc>, reverted: Reverted): boolean {
  const doc = handle.doc();
  if (!doc || A.getHeads(doc).join() !== reverted.after.join()) return false;
  const done = changeBetween(doc, reverted.before, reverted.after);
  undoIntegration(handle, done, { message: actionMessage("undo", t("history.undo.revert")) });
  return true;
}

/**
 * Starts a variant: a document of its own, cut from the main one at `at`, listed in the main
 * document so every person can find it (HIS-10, HIS-16).
 * @throws {TooBig}
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
  mayWrite(doc.text.length);
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

export interface Merged {
  merged: boolean;
  /** Changes in locked zones, kept apart to be offered as a suggestion. */
  held: Skipped[];
}

/**
 * Merges a variant into the main document (HIS-11): a version "Avant fusion de X" is kept, the
 * texts are joined by the CRDT, the variant is marked merged. Merging again does nothing (HIS-12).
 * With a `skip` hook the variant's net text is integrated outside the locked zones instead (X-11:
 * the link with the variant's history is lost, the zones are left alone).
 * @throws {TooBig} when either document is too big, or the merge would grow the text past the limit.
 */
export function mergeVariant(
  main: DocHandle<HDoc>,
  variantHandle: DocHandle<HDoc>,
  variant: Variant,
  opts: Guarded = {},
): Merged {
  const doc = main.doc();
  const theirs = variantHandle.doc();
  if (!doc || !theirs) return { merged: false, held: [] };
  const entry = doc.meta?.variants?.[indexOfVariant(doc, variant.url)];
  if (!entry || entry.state === "merged") return { merged: false, held: [] };
  // The size after the merge is at most the main text plus what the variant added to its cut.
  mayWrite(doc.text.length);
  mayWrite(theirs.text.length);
  const from = textAt(doc, entry.from).length;
  mayWrite(doc.text.length, doc.text.length + Math.max(0, theirs.text.length - from));
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
  let held: Skipped[] = [];
  if (opts.skip) {
    held = integrate(main, entry.from as A.Heads, theirs.text, { skip: opts.skip })?.skipped ?? [];
  } else {
    main.merge(variantHandle);
  }
  setState(main, variant.url, "merged", heads(main));
  return { merged: true, held };
}

/** Sets a variant aside: hidden by default, kept, and brought back by {@link reopenVariant}. */
export const abandonVariant = (main: DocHandle<HDoc>, url: string): void =>
  setState(main, url, "abandoned");

export const reopenVariant = (main: DocHandle<HDoc>, url: string): void =>
  setState(main, url, "open");
