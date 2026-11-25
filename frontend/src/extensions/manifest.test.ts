import { describe, expect, it } from "vitest";
import { ManifestError, type ManifestSource, parseManifest } from "./manifest";

const VECTORS = import.meta.glob("./testdata/manifests/*/*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const vectors = (folder: "valid" | "invalid") =>
  Object.entries(VECTORS)
    .filter(([path]) => path.includes(`/manifests/${folder}/`))
    .map(([path, text]) => ({ name: path.slice(path.lastIndexOf("/") + 1), text }));

const source = (text: string, extra: Partial<ManifestSource> = {}): ManifestSource => ({
  origin: "builtin",
  folder: "sample",
  text,
  files: new Set(),
  ...extra,
});

/** The reason a manifest is refused with, or "accepted". */
function verdict(s: ManifestSource): string {
  try {
    parseManifest(s);
    return "accepted";
  } catch (error) {
    if (error instanceof ManifestError) return error.reason;
    throw error;
  }
}

const json = (o: unknown) => JSON.stringify(o);
const sample = (patch: (o: Record<string, unknown>) => void = () => {}, options?: object) => {
  const o: Record<string, unknown> = {
    id: "sample",
    version: "1.0.0",
    contributes: {
      fileTypes: [
        {
          id: "sample",
          label: "Sample",
          icon: "text",
          extensions: [".smp"],
          storage: "text",
          options,
        },
      ],
    },
  };
  patch(o);
  return o;
};

/** A manifest with one native viewer, its entry `entry`. */
const withViewer = (entry: string, essential = true, capabilities = ["ui.native"]) =>
  json({
    id: "grid",
    version: "1.0.0",
    essential,
    contributes: {
      fileTypes: [
        { id: "grid-t", label: "Grid", icon: "csv", extensions: [".grd"], storage: "text" },
      ],
      viewers: [
        {
          id: "grid.view",
          fileTypes: ["grid-t"],
          label: "Grid",
          entry,
          capabilities,
        },
      ],
    },
  });

describe("manifest", () => {
  it("EX0 R-2 a malformed manifest is refused with its reason", () => {
    const invalid = vectors("invalid");
    const valid = vectors("valid");
    expect(invalid.length).toBeGreaterThan(0);
    expect(valid.length).toBeGreaterThan(0);
    for (const v of invalid) {
      const reason = v.name.slice(0, v.name.indexOf("."));
      // A vector about a plugin's limits is read as a plugin's (`plugin-` in its name).
      const origin = v.name.includes("plugin-") ? "plugin" : "builtin";
      expect({ vector: v.name, reason: verdict(source(v.text, { origin })) }).toEqual({
        vector: v.name,
        reason,
      });
    }
    for (const v of valid) {
      // A valid vector's entries are files of its folder (a panel's, EX0c2), as the loader's glob gives them.
      const files = new Set(
        [...v.text.matchAll(/"entry":\s*"([^"]+)"/g)].map((m) => m[1] as string),
      );
      const parsed = parseManifest(source(v.text, { files }));
      // Accepted unchanged: the defaults fill only what the vector left out.
      const raw = JSON.parse(v.text) as Record<string, unknown>;
      expect(parsed.id).toBe(raw.id);
      expect(JSON.parse(JSON.stringify(parsed.contributes.fileTypes))).toEqual(
        (raw.contributes as { fileTypes: Record<string, unknown>[] }).fileTypes.map((t) => ({
          extensions: [],
          filenames: [],
          shebangs: [],
          options: {},
          ...t,
        })),
      );
    }
    // Bounds on every list and string, and a regular expression is never a value.
    expect(verdict(source(json(sample((o) => (o.version = "1".repeat(40))))))).toBe("too-long");
    expect(
      verdict(
        source(
          json(
            sample((o) => {
              (o.contributes as { fileTypes: unknown[] }).fileTypes = Array.from(
                { length: 65 },
                (_, i) => ({
                  id: `t${i}`,
                  label: "T",
                  icon: "text",
                  storage: "text",
                }),
              );
            }),
          ),
        ),
      ),
    ).toBe("too-many");
    expect(verdict(source(json(sample((o) => (o.activation = ["onFileType:a|b"])))))).toBe(
      "bad-value",
    );
    expect(verdict(source("x".repeat(70_000)))).toBe("too-long");
    // Not JSON at all (a trailing comma): no vector file, Biome reads those as JSON.
    expect(verdict(source('{"id": "sample",}'))).toBe("not-json");
  });

  it("EX0 R-12 a manifest cannot reach the object prototype", () => {
    // The walk over keys is bounded: a deep nest is refused, not a stack overflow (finding 5).
    expect(verdict(source(`${"[".repeat(32_001)}${"]".repeat(32_001)}`))).toBe("too-deep");
    expect(verdict(source(`{"id": "sample", "x": ${"[".repeat(40)}${"]".repeat(40)}}`))).toBe(
      "too-deep",
    );
    const attempts = [
      `{"__proto__": {"polluted": 1}, "id": "sample", "version": "1.0.0", "contributes": {}}`,
      `{"id": "sample", "version": "1.0.0", "constructor": {"prototype": {"polluted": 1}}, "contributes": {}}`,
      `{"id": "sample", "version": "1.0.0", "contributes": {"__proto__": {"polluted": 1}}}`,
      `{"id": "sample", "version": "1.0.0", "contributes": {"fileTypes": [{"id": "s", "label": "S", "icon": "text", "storage": "text", "options": {"__proto__": {"polluted": 1}}}]}}`,
      `{"id": "sample", "version": "1.0.0", "contributes": {"fileTypes": [{"id": "s", "label": "S", "icon": "text", "storage": "text", "options": {"prototype": 1}}]}}`,
      `{"id": "sample", "version": "1.0.0", "contributes": {"fileTypes": [{"id": "s", "label": "S", "icon": "text", "storage": "text", "extensions": [{"constructor": 1}]}]}}`,
      `[{"__proto__": {"polluted": 1}}]`,
    ];
    for (const text of attempts) {
      expect({ text, reason: verdict(source(text)) }).toEqual({ text, reason: "proto-key" });
      expect(({} as { polluted?: unknown }).polluted).toBeUndefined();
    }
    const accepted = parseManifest(source(json(sample(undefined, { delimiter: ";" }))));
    const objects: object[] = [];
    const walk = (v: unknown) => {
      if (v === null || typeof v !== "object") return;
      if (!Array.isArray(v)) objects.push(v);
      for (const x of Object.values(v)) walk(x);
    };
    walk(accepted);
    expect(objects.length).toBeGreaterThan(3);
    for (const o of objects) expect(Object.getPrototypeOf(o)).toBeNull();
  });

  it("EX0 R-13 an entry cannot leave its extension's folder", () => {
    const own = new Set(["native/grid.ts"]);
    for (const entry of [
      "../x.ts",
      "native/../../x.ts",
      "/x.ts",
      "native\\x.ts",
      "C:x.ts",
      "file:x.ts",
      "native//x.ts",
      "%2e%2e/x.ts",
      "native/x.ts?x",
      "native/x.ts#y",
      "native/a b.ts",
      "./native/grid.ts",
      "",
    ]) {
      // The shape alone, as for a plugin (whose host resolves the entry in its package), then a
      // built-in's, which must also be a file of its folder.
      const plugin = source(withViewer(entry, false, []), { origin: "plugin" });
      const builtin = source(withViewer(entry), {
        folder: "grid",
        files: new Set([...own, entry]),
      });
      expect({ entry, plugin: verdict(plugin), builtin: verdict(builtin) }).toEqual({
        entry,
        plugin: "bad-entry",
        builtin: "bad-entry",
      });
    }
    expect(verdict(source(withViewer("native/grid.ts", false, []), { origin: "plugin" }))).toBe(
      "accepted",
    );
    expect(verdict(source(withViewer("native/grid.ts"), { folder: "grid", files: own }))).toBe(
      "accepted",
    );
    // Not a key of the folder's own glob: absent, or a file of another extension.
    expect(
      verdict(source(withViewer("native/grid.ts"), { folder: "grid", files: new Set() })),
    ).toBe("bad-entry");
    expect(
      verdict(
        source(withViewer("native/other.ts"), {
          folder: "grid",
          files: new Set(["native/grid.ts"]),
        }),
      ),
    ).toBe("bad-entry");
    // A plugin's entry is only checked for its shape; the plugin host resolves it in its package.
    expect(verdict(source(withViewer("native/grid.ts", true), { origin: "plugin" }))).toBe(
      "native-refused",
    );
  });
});
