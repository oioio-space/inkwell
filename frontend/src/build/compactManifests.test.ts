// @vitest-environment node
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { compactManifests } from "./compactManifests";

const dirs: string[] = [];
afterEach(() => {
  for (const dir of dirs.splice(0)) rmSync(dir, { recursive: true, force: true });
});

/** What the plugin hands the bundle for a manifest file holding `text`. */
function loaded(text: string, query = "?raw"): unknown {
  const dir = mkdtempSync(join(tmpdir(), "compact-manifests-"));
  dirs.push(dir);
  const file = join(dir, "inkwell-plugin.json");
  writeFileSync(file, text);
  const load = compactManifests().load as (id: string) => string | undefined;
  return load(`${file}${query}`);
}
const value = (code: unknown) =>
  JSON.parse(
    String(code)
      .replace(/^export default /, "")
      .replace(/;$/, ""),
  );

describe("compactManifests (EX0c1 review: the built-ins' manifests ship without their whitespace)", () => {
  it("a manifest read as text reaches the bundle as the same JSON, without whitespace", () => {
    const text = '{\n  "id": "sample",\n  "contributes": { "commands": [ ] }\n}\n';
    const out = value(loaded(text)) as string;
    expect(out).toBe('{"id":"sample","contributes":{"commands":[]}}');
    expect(JSON.parse(out)).toEqual(JSON.parse(text));
  });

  it("a manifest that is not JSON is left as it is, for the registry to refuse with its reason", () => {
    expect(value(loaded("{ not json"))).toBe("{ not json");
  });

  it("any other file or query is not its business", () => {
    expect(loaded("{}", "")).toBeUndefined();
    expect(loaded("{}", "?url")).toBeUndefined();
  });
});
