/** What the document keeps besides the text: named versions, variants, and who is who. */

export type Heads = string[];

/** A version the person named (B2). `by` is a claim: the actor that wrote it, not checked by trust. */
export interface Version {
  name: string;
  heads: Heads;
  /** The Automerge actor of the device that wrote the entry. */
  by: string;
  /** Milliseconds since the epoch, on the writer's clock. */
  at: number;
  /** Made by Inkwell before an action ("Avant retour à…"), not named by a person. */
  auto?: boolean;
}

export type VariantState = "open" | "merged" | "abandoned";

/** A variant is a document of its own, cut from the main one at `from`. */
export interface Variant {
  url: string;
  name: string;
  from: Heads;
  by: string;
  at: number;
  state: VariantState;
  /** The heads of the main document right after the merge. */
  mergedHeads?: Heads;
}

export interface Meta {
  versions?: Version[];
  variants?: Variant[];
}

export interface HDoc {
  text: string;
  meta?: Meta;
}

/** The prefix of a change message that marks an action of Inkwell's own (shown as its own entry). */
const ACTION = "inkwell:";
export type ActionKind = "revert" | "undo" | "merge" | "version" | "variant" | "fix" | "claim";

export function actionMessage(kind: ActionKind, label: string): string {
  return `${ACTION}${kind}:${label}`;
}

export function parseAction(
  message: string | null,
): { kind: ActionKind; label: string } | undefined {
  const m = message?.match(/^inkwell:(revert|undo|merge|version|variant|fix|claim):([\s\S]*)$/);
  return m ? { kind: m[1] as ActionKind, label: m[2] as string } : undefined;
}
