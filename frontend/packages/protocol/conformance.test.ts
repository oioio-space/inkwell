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
  .flatMap((f) => JSON.parse(readFileSync(`${dir}/${f}`, "utf8")).vectors)
  // Spec §21: scope "transport" is the libp2p layer (linkprobe, §8.5), which never reaches the
  // webview; @inkwell/protocol does not implement it, the Go runner does.
  .filter((v: Vector) => v.scope !== "transport");
const step = Number(process.env.INKWELL_PROTOCOL_STEP ?? 1);
test(`shared protocol corpus, step ${step}`, async () => {
  const results = await runConformance(vectors, (v) => reference(v, vectors), step);
  const failures = results
    .filter((r) => r.required && !r.passed)
    .map((r) => `${r.vector.id}: ${r.error}`);
  expect(failures).toEqual([]);
  expect(results.length).toBe(vectors.length);
});
