import { next as A } from "@automerge/automerge";
import { FileKind } from "../../store/files";
import { compileQuery, type Matcher, type QuerySpec } from "./query";
import type { ReplaceTarget } from "./replace";

/** Test helpers shared by the search suites (not shipped: only tests import this). */

export const baseSpec: QuerySpec = {
  text: "",
  mode: "normal",
  caseSensitive: false,
  wholeWord: false,
  ignoreAccents: false,
  dotAll: false,
};

export function compile(text: string, over: Partial<QuerySpec> = {}): Matcher {
  const r = compileQuery({ ...baseSpec, text, ...over });
  if (!r.ok) throw new Error(r.error);
  return r.matcher;
}

/** A real Automerge document behind the slice of DocHandle that replacing uses. */
export class TestHandle implements ReplaceTarget {
  changes = 0;
  constructor(public current: A.Doc<{ text: string }>) {}
  static of(text: string): TestHandle {
    return new TestHandle(A.from({ text }));
  }
  doc() {
    return this.current;
  }
  change(fn: (d: { text: string }) => void) {
    this.changes++;
    this.current = A.change(this.current, fn);
  }
  get text() {
    return this.current.text;
  }
}

export function typst(path: string, text: string) {
  return { path, kind: FileKind.typst, text: () => text };
}
