// Shared by highlight.equivalence.test.ts and highlight.perf.test.ts (block LF0a2): the corpora, the
// layout-independent comparison of two trees, and a seeded random generator.
import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { IterMode, type MountedTree, NodeProp, type Parser, type Tree } from "@lezer/common";
import { getStyleTags, highlightTree, type Tag, tags } from "@lezer/highlight";
import { highlightStyle } from "./highlight";

const here = (path: string) => fileURLToPath(new URL(path, import.meta.url));

export interface CorpusFile {
  name: string;
  text: string;
}

/** typst/typst tests/suite, one entry per original file (see the header of the corpus file). */
export function typstSuite(): CorpusFile[] {
  const text = readFileSync(here("./testdata/highlight/typst-suite.typ"), "utf8");
  return text
    .split(/^\/\/@@ file /m)
    .slice(1)
    .map((part) => {
      const nl = part.indexOf("\n");
      return { name: part.slice(0, nl), text: part.slice(nl + 1) };
    });
}

/** The repo's own Typst documents (the LSP perf fixtures). */
export function repoTypst(): CorpusFile[] {
  const dir = here("../../../testdata/perf/lsp/");
  return readdirSync(dir)
    .filter((n) => n.endsWith(".typ"))
    .sort()
    .map((name) => ({ name, text: readFileSync(dir + name, "utf8") }));
}

/**
 * The repo's Markdown documents listed in testdata/highlight/markdown-corpus.txt, in that order. The
 * list is pinned: a new document joins the corpus only when added there, so it never changes the
 * gate's verdict unreviewed; a listed document that is gone throws.
 */
export function repoMarkdown(): CorpusFile[] {
  const list = readFileSync(here("./testdata/highlight/markdown-corpus.txt"), "utf8");
  return list
    .split("\n")
    .filter((l) => l && !l.startsWith("#"))
    .map((name) => ({ name, text: readFileSync(here(`../../../${name}`), "utf8") }));
}

/** A fixed Markdown regression case: a full parse then one edit, or a parse resumed at `stopAt`. */
export interface MarkdownCase extends CorpusFile {
  why: string;
  edit?: { from: number; to: number; insert: string };
  stopAt?: number;
}

/** The minimised Markdown cases the random edits found (testdata/highlight/markdown-cases.json). */
export function markdownCases(): MarkdownCase[] {
  return JSON.parse(readFileSync(here("./testdata/highlight/markdown-cases.json"), "utf8"));
}

/** A nested-language tree (mount) at absolute [from, to). */
export interface Mount {
  key: string;
  from: number;
  to: number;
  /** Where its own parse first recovered from a syntax error (an error node), or null. */
  firstError: number | null;
  parser: Parser;
}

/**
 * Every mount of `tree` at any depth (overlay mounts too, which iteration does not enter), in
 * document order. A mount's errors are its own: those of the mounts nested in it are not counted.
 */
export function mounts(tree: Tree): Mount[] {
  const out: Mount[] = [];
  const walk = (t: Tree, at: number, parser: Parser | null) => {
    const c = t.cursor(IterMode.IgnoreMounts | IterMode.IncludeAnonymous);
    let firstError: number | null = null;
    const nested: [MountedTree, number][] = [];
    do {
      if (c.type.isError && firstError === null) firstError = at + c.from;
      const m = c.tree?.prop(NodeProp.mounted);
      if (m) nested.push([m, at + c.from + (m.overlay?.[0]?.from ?? 0)]);
    } while (c.next());
    if (parser) {
      const to = at + t.length;
      const key = `${t.type.name}@${at}-${to}:${firstError === null ? "clean" : "recovered"}`;
      out.push({ key, from: at, to, firstError, parser });
    }
    for (const [m, start] of nested) walk(m.tree, start, m.parser);
  };
  walk(tree, 0, null);
  return out.sort((a, b) => a.from - b.from || a.to - b.to);
}

/** A text of `size` UTF-16 units made of the given documents, repeated. */
export function repeatTo(files: CorpusFile[], size: number): string {
  const unit = `${files.map((f) => f.text).join("\n\n")}\n\n`;
  return unit.repeat(Math.ceil(size / unit.length)).slice(0, size);
}

/** Named-node structure; anonymous (balancing or chunk) nodes are not part of it. */
export function structure(tree: Tree): string[] {
  const out: string[] = [];
  tree.iterate({
    enter: (n) => {
      out.push(`${n.name}@${n.from}-${n.to}`);
    },
  });
  return out;
}

const tagNames = new Map<Tag, string>();
for (const [k, v] of Object.entries(tags)) if (typeof v !== "function") tagNames.set(v as Tag, k);
tagNames.set(tags.function(tags.variableName), "function(variableName)");

/**
 * The colour tags of every node, matched against the node's real parent chain. It does not
 * depend on how the tree is split into buffers, unlike `highlightTree`, whose TreeCursor reads a
 * stale parent after `sibling()` moves from a Tree child into a TreeBuffer (@lezer/common 1.5.4).
 */
export function nodeTags(tree: Tree, extra: Record<string, Tag> = {}): string[] {
  const names = new Map(tagNames);
  for (const [k, v] of Object.entries(extra)) names.set(v, `extra.${k}`);
  const out: string[] = [];
  tree.iterate({
    enter: (n) => {
      const rule = getStyleTags(n.node);
      if (!rule) return;
      out.push(
        `${n.name}@${n.from}-${n.to}:${rule.tags.map((t) => names.get(t) ?? "?").join(",")}`,
      );
      if (rule.opaque) return false;
    },
  });
  return out;
}

/** What the editor paints: the ranges and CSS classes of the app's highlightStyle. */
export function spans(tree: Tree): string[] {
  const out: string[] = [];
  highlightTree(tree, highlightStyle, (from, to, cls) => {
    out.push(`${from}-${to}:${cls}`);
  });
  return out;
}

/** The first index where two lists differ, with both values; null when equal. */
export function firstDiff(
  a: string[],
  b: string[],
): { i: number; a: string | undefined; b: string | undefined } | null {
  const n = Math.max(a.length, b.length);
  for (let i = 0; i < n; i++) if (a[i] !== b[i]) return { i, a: a[i], b: b[i] };
  return null;
}

/** A deterministic generator (LCG), so a failing seed replays. */
export function rng(seed: number): (n: number) => number {
  let s = seed >>> 0;
  return (n) => {
    s = (s * 1664525 + 1013904223) >>> 0;
    return s % n;
  };
}

/** A random edit of a text of `length`: an insertion of `pieces[i]`, or a short deletion. */
export function randomChange(
  rand: (n: number) => number,
  length: number,
  pieces: readonly string[],
): { from: number; to: number; insert: string } {
  const from = rand(length + 1);
  if (rand(3) === 0 && length > 0)
    return { from, to: Math.min(length, from + 1 + rand(8)), insert: "" };
  return { from, to: from, insert: pieces[rand(pieces.length)] ?? "" };
}
