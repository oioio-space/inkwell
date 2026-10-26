import type { Patch } from "@automerge/automerge";
import {
  Annotation,
  ChangeSet,
  type ChangeSpec,
  type EditorState,
  type Transaction,
} from "@codemirror/state";
import type { EditorView } from "@codemirror/view";

/*
 * Vendored from @automerge/automerge-codemirror 0.2.0 (MIT, github.com/automerge/automerge-codemirror,
 * src/amToCodemirror.ts and the reconcile annotation of src/plugin.ts), trimmed to what Inkwell
 * uses and typed. The package exposes only its own sync plugin, which cannot chunk a large paste
 * (ADR 0028 D3), and reaching into its build output is fragile: it has no `exports` map.
 */

/** Marks a transaction that only brings the editor up to date with the document. */
export const reconcileAnnotationType = Annotation.define<object>();

/** Whether the transaction came from the document (a peer's patches), not from the person. */
export const isReconcileTx = (tr: Transaction): boolean => !!tr.annotation(reconcileAnnotationType);

/** Applies the document's patches to the editor, keeping the selection where it was. */
export function applyAmPatchesToCm(view: EditorView, target: string[], patches: Patch[]): void {
  let selection = view.state.selection;
  for (const patch of patches) {
    const spec = changeOf(patch, target, view.state);
    if (spec == null) continue;
    const changes = ChangeSet.of(spec, view.state.doc.length, "\n");
    selection = selection.map(changes, 1);
    view.dispatch({ changes, annotations: reconcileAnnotationType.of({}) });
  }
  view.dispatch({ selection, annotations: reconcileAnnotationType.of({}) });
}

function changeOf(patch: Patch, target: string[], state: EditorState): ChangeSpec | null {
  switch (patch.action) {
    case "insert": {
      const at = charIndex(target, patch.path);
      if (at == null) return [];
      return [
        { from: at, to: at, insert: patch.values.map((v) => (v ? v.toString() : "")).join("") },
      ];
    }
    case "splice": {
      const at = charIndex(target, patch.path);
      return at == null ? [] : [{ from: at, insert: patch.value }];
    }
    case "del": {
      const at = charIndex(target, patch.path);
      return at == null ? [] : [{ from: at, to: at + (patch.length || 1) }];
    }
    case "put": {
      if (charIndex(target, [...patch.path, 0]) == null) return [];
      if (typeof patch.value !== "string") return [];
      return [{ from: 0, to: state.doc.length, insert: patch.value }];
    }
    default:
      return null;
  }
}

/** The index when `path` is `target` followed by a character position, else null. */
function charIndex(target: string[], path: readonly (string | number)[]): number | null {
  if (path.length !== target.length + 1) return null;
  for (let i = 0; i < target.length; i++) if (target[i] !== path[i]) return null;
  const index = path[path.length - 1];
  return typeof index === "number" ? index : null;
}
