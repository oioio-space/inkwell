import { readFileSync, rmSync, writeFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";

/**
 * Test helper: an in-test mutant. Copies a source file beside itself with one edit (the anchor
 * must occur exactly once), imports the copy, and removes it. The copy's relative imports resolve
 * as the original's do. A test then runs the same check on the original (it passes) and on the
 * mutant (it must fail): proof that the check sees the rule it names.
 *
 * @param source a path relative to `src/`, such as `ui/Foo.svelte`.
 * @param tag names the copy (`Foo.<tag>.svelte`); one per mutant, since Vite caches a module by its path.
 */
export async function withMutant<M, R>(
  source: string,
  tag: string,
  anchor: string,
  replacement: string,
  use: (mutant: M) => Promise<R>,
): Promise<R> {
  const path = fileURLToPath(new URL(source, import.meta.url));
  const text = readFileSync(path, "utf8");
  const parts = text.split(anchor).length - 1;
  if (parts !== 1) throw new Error(`mutant ${tag}: anchor found ${parts} times in ${path}`);
  const copy = path.replace(/(\.svelte\.ts|\.svelte|\.ts)$/, `.${tag}$1`);
  if (copy === path) throw new Error(`mutant ${tag}: unsupported file ${path}`);
  writeFileSync(
    copy,
    text.replace(anchor, () => replacement),
  );
  try {
    return await use((await import(/* @vite-ignore */ copy)) as M);
  } finally {
    rmSync(copy, { force: true });
  }
}
