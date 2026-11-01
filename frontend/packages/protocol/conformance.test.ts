// SPDX-License-Identifier: MPL-2.0
// @vitest-environment node
import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { expect, test } from "vitest";
import { reference } from "./reference";
import { runConformance, type Vector } from "./src/conformance";

const dir = fileURLToPath(new URL("../../../protocol/vectors/", import.meta.url));
const vectors: Vector[] = readdirSync(dir)
  .filter((f) => f.endsWith(".json"))
  .sort()
  .flatMap((f) => JSON.parse(readFileSync(`${dir}/${f}`, "utf8")).vectors);
test("shared protocol corpus, step 1", async () => {
  const results = await runConformance(vectors, (v) => reference(v, vectors));
  const failures = results
    .filter((r) => r.required && !r.passed)
    .map((r) => `${r.vector.id}: ${r.error}`);
  expect(failures).toEqual([]);
  expect(results.length).toBe(vectors.length);
});
