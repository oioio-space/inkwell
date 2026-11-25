/**
 * Words written (rule T-1 of plan docs/plans/2026-11-24-obs1-observability.md): only the person's
 * own edits count, never what others wrote in the same window. The tracker keeps one change set,
 * `undo`: applied to the current text it removes the person's edits since the window began,
 * remote edits staying. A local edit composes into it, a remote one maps it: one operation per
 * transaction, linear in the edits since the window began, never in the document's size
 * (`ChangeSet.compose`, `ChangeSet.map`, https://codemirror.net/docs/ref/#state.ChangeSet). At
 * idle the touched lines are counted before and after with `countPlainWords`, and the difference
 * is the words the person wrote (none if it is negative).
 */
import { ChangeSet, type Text } from "@codemirror/state";
import { countPlainWords } from "../../editor/stats/words";

export class WordTracker {
  #undo: ChangeSet;

  constructor(doc: Text) {
    this.#undo = ChangeSet.empty(doc.length);
  }

  /** A change of the person's own, made on `before`. */
  local(changes: ChangeSet, before: Text): void {
    this.#undo = changes.invert(before).compose(this.#undo);
  }

  /** A change of someone else, or of the program, made on the current text. */
  remote(changes: ChangeSet): void {
    this.#undo = this.#undo.map(changes);
  }

  /** Whether the person changed something since the window began. */
  get touched(): boolean {
    return !this.#undo.empty;
  }

  /**
   * Ends the window over `doc`, the current text: the words the person's edits added, counted in
   * the lines they touched, before and after.
   */
  take(doc: Text): number {
    if (this.#undo.empty) return 0;
    const before = this.#undo.apply(doc);
    let words = 0;
    // The touched ranges, widened to whole lines on both sides and merged where they meet: the
    // other lines are the same before and after, and a word never crosses a line.
    let span: [number, number, number, number] | undefined;
    const count = ([a, b, c, d]: [number, number, number, number]) => {
      words += countPlainWords(doc.sliceString(a, b)) - countPlainWords(before.sliceString(c, d));
    };
    this.#undo.iterChangedRanges((fromA, toA, fromB, toB) => {
      const next: [number, number, number, number] = [
        doc.lineAt(fromA).from,
        doc.lineAt(toA).to,
        before.lineAt(fromB).from,
        before.lineAt(toB).to,
      ];
      if (span && next[0] <= span[1]) {
        span[1] = Math.max(span[1], next[1]);
        span[3] = Math.max(span[3], next[3]);
      } else {
        if (span) count(span);
        span = next;
      }
    });
    if (span) count(span);
    this.#undo = ChangeSet.empty(doc.length);
    return Math.max(0, words);
  }
}
