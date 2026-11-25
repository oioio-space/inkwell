import { describe, expect, it, vi } from "vitest";
import { builtinSources } from "./loader";
import { ManifestError, type ManifestSource, parseManifest } from "./manifest";
import { buildRegistry, type EntryLoader } from "./registry";

const source = (o: object, extra: Partial<ManifestSource> = {}): ManifestSource => ({
  origin: "builtin",
  folder: "meter",
  text: JSON.stringify(o),
  files: new Set(["debug/read.ts"]),
  ...extra,
});

const metric = (fields: object = {}) => ({
  id: "meter.words",
  kind: "volume",
  unit: "words",
  scope: "document",
  label: "metric.words",
  max: 1000,
  ...fields,
});

const provider = (fields: object = {}) => ({
  id: "meter.state",
  label: "debug.state",
  entry: "debug/read.ts",
  fields: { count: "number", on: "boolean", kind: ["lan", "relayed"], event: "code" },
  ...fields,
});

const manifest = (contributes: object, id = "meter") => ({ id, version: "1.0.0", contributes });

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

/** A loader whose entry is `module`, counting the loads. */
function loaderOf(module: unknown): EntryLoader & { loads: string[] } {
  const loads: string[] = [];
  return Object.assign(
    (folder: string, entry: string) => {
      loads.push(`${folder}/${entry}`);
      return Promise.resolve(module);
    },
    { loads },
  );
}

/** The built-ins and one more manifest, through the registry. */
const withBuiltins = (
  s: ManifestSource,
  loader: EntryLoader = loaderOf({}),
  goMetrics?: unknown[],
) => buildRegistry([...builtinSources(), s], loader, goMetrics ? { goMetrics } : {});

describe("OBS1 contribution points", () => {
  it("OBS1 M-1 a metrics or debug contribution is validated like the other points", () => {
    // Accepted as data, frozen, on the registry, owned by its extension.
    const r = withBuiltins(source(manifest({ metrics: [metric()], debug: [provider()] })));
    expect(r.refused).toEqual([]);
    const m = r.metric("meter.words");
    expect(m).toMatchObject({
      kind: "volume",
      unit: "words",
      scope: "document",
      max: 1000,
      extension: "meter",
      from: "manifest",
    });
    expect(Object.isFrozen(m)).toBe(true);
    expect(r.debug.map((d) => d.id)).toContain("meter.state");
    // The built-in activity extension declares its metrics through the same point.
    expect(r.metrics.filter((x) => x.extension === "activity").map((x) => x.id)).toEqual([
      "activity.words",
      "activity.active",
      "activity.opened",
      "activity.open-time",
      "activity.received",
    ]);
    expect(r.debug.find((d) => d.id === "activity.network")?.entry).toBe("debug/network.ts");
    // The same checks as every point: unknown keys, another extension's id, a bad entry, a pattern.
    const refused: [object, string][] = [
      [{ metrics: [metric({ color: "red" })] }, "unknown-key"],
      [{ metrics: [metric({ id: "other.words" })] }, "not-own"],
      [{ metrics: [metric({ id: "words" })] }, "bad-id"],
      [{ metrics: [{ ...metric(), label: undefined }] }, "missing-key"],
      [
        { metrics: Array.from({ length: 65 }, (_, i) => metric({ id: `meter.m${i}` })) },
        "too-many",
      ],
      [{ debug: [provider({ entry: "../escape.ts" })] }, "bad-entry"],
      [{ debug: [provider({ entry: "debug/absent.ts" })] }, "bad-entry"],
      [{ debug: [provider({ id: "other.state" })] }, "not-own"],
      [{ debug: [provider({ fields: {} })] }, "missing-key"],
      [{ debug: [provider({ fields: { "Bad Name": "number" } })] }, "bad-id"],
      [{ debug: [provider({ extra: 1 })] }, "unknown-key"],
    ];
    for (const [contributes, reason] of refused)
      expect(verdict(source(manifest(contributes))), JSON.stringify(contributes)).toBe(reason);
    // A metric is a claim: the same id twice refuses the manifest, naming it.
    const twice = withBuiltins(source(manifest({ metrics: [metric(), metric()] })));
    expect(twice.refused).toEqual([
      { folder: "meter", reason: "conflict on metric meter.words", with: ["meter", "meter"] },
    ]);
    // A debug module is not native code: it needs no `ui.native`, while a `native/` file still does.
    expect(
      verdict(source(manifest({ debug: [provider()] }), { files: new Set(["debug/read.ts"]) })),
    ).toBe("accepted");
    expect(
      verdict(
        source(manifest({ debug: [provider()] }), {
          files: new Set(["debug/read.ts", "native/x.ts"]),
        }),
      ),
    ).toBe("native-unused");
  });

  it("OBS1 M-2 a free-text, unbounded, unknown-enum or inconsistent field is refused", () => {
    const cases: [object, string][] = [
      [{ metrics: [metric({ values: "text" })] }, "free-text"],
      [{ metrics: [metric({ values: "string" })] }, "free-text"],
      [{ metrics: [metric({ values: ["syntax", "a file name"] })] }, "free-text"],
      [{ debug: [provider({ fields: { title: "text" } })] }, "free-text"],
      [{ debug: [provider({ fields: { name: "string" } })] }, "free-text"],
      [{ debug: [provider({ fields: { path: ["ok", "Mémoire.typ"] } })] }, "free-text"],
      [{ metrics: [{ ...metric(), max: undefined }] }, "unbounded"],
      [{ metrics: [metric({ max: Number.POSITIVE_INFINITY })] }, "unbounded"],
      [{ metrics: [metric({ max: 2 ** 60 })] }, "unbounded"],
      [{ metrics: [metric({ max: "1000" })] }, "unbounded"],
      [{ metrics: [metric({ max: 0 })] }, "bad-value"],
      [{ metrics: [metric({ max: 2.5 })] }, "bad-value"],
      [{ metrics: [metric({ kind: "gauge" })] }, "bad-value"],
      [{ metrics: [metric({ unit: "seconds" })] }, "bad-value"],
      [{ metrics: [metric({ scope: "peer" })] }, "bad-value"],
      [{ debug: [provider({ fields: { n: "float" } })] }, "bad-value"],
      [{ debug: [provider({ fields: { n: [] } })] }, "bad-value"],
      [{ metrics: [metric({ kind: "counter", unit: "ms" })] }, "inconsistent"],
      [{ metrics: [metric({ kind: "duration", unit: "count" })] }, "inconsistent"],
      [{ metrics: [metric({ kind: "volume", unit: "ms" })] }, "inconsistent"],
    ];
    for (const [contributes, reason] of cases)
      expect(verdict(source(manifest(contributes))), JSON.stringify(contributes)).toBe(reason);
    // A bound written past what a number holds reads as Infinity.
    const huge = source(manifest({ metrics: [metric()] }));
    expect(verdict({ ...huge, text: huge.text.replace('"max":1000', '"max":1e999') })).toBe(
      "unbounded",
    );
    for (const [kind, unit] of [
      ["counter", "count"],
      ["duration", "ms"],
      ["volume", "bytes"],
      ["volume", "words"],
    ])
      expect(
        verdict(
          source(manifest({ metrics: [metric({ kind, unit, values: ["a", "b-c", "d_e"] })] })),
        ),
      ).toBe("accepted");
  });

  it("OBS1 M-3 a metric id already taken is refused in either order", () => {
    const go = [
      {
        id: "meter.words",
        kind: "volume",
        unit: "words",
        scope: "app",
        label: "metric.words",
        max: 10,
      },
      {
        id: "preview.compilations",
        kind: "counter",
        unit: "count",
        scope: "app",
        label: "metric.preview.compilations",
        max: 10,
      },
    ];
    const s = source(manifest({ metrics: [metric(), metric({ id: "meter.lines" })] }));
    const conflict = {
      folder: "meter",
      reason: "conflict on metric meter.words",
      with: ["go", "meter"],
    };
    // Go's descriptors first, then the manifests: the manifest's metric is refused.
    const before = withBuiltins(s, loaderOf({}), go);
    expect(before.refused).toEqual([conflict]);
    expect(before.metric("meter.words")?.from).toBe("go");
    expect(before.metric("meter.lines")?.from).toBe("manifest");
    // The registry first, then Go's descriptors: the same refusal, the same view.
    const view = withBuiltins(s).withGoMetrics(go);
    expect(view.refused).toEqual([conflict]);
    expect(view.metrics.map((m) => `${m.id}:${m.from}`)).toEqual(
      before.metrics.map((m) => `${m.id}:${m.from}`),
    );
    // Go's own descriptors pass the same validator, and Go cannot declare one twice.
    const bad = withBuiltins(s).withGoMetrics([{ ...go[1], values: "text" }, go[1], go[1]]);
    expect(bad.refused).toEqual([
      { folder: "go", reason: "free-text" },
      { folder: "go", reason: "bad-id" },
    ]);
  });

  it("OBS1 V-1 every debug read passes the value rule", async () => {
    const values = {
      count: 3,
      on: true,
      kind: "relayed",
      event: "net.conn.close",
      "3fa9c2e1d4b5a607.count": 7,
      "2.kind": "lan",
      // Each of these breaks rule V and is dropped, counted, never kept.
      title: "Mémoire final",
      "canary.count": 1,
      "3fa9c2e1d4b5a607.title": "x",
      "kind.count": 2,
      negative: -1,
      "x.on": false,
      nan: Number.NaN,
      infinite: Number.POSITIVE_INFINITY,
      wrongEnum: "wifi",
      notCode: "typst.compile.success",
      nested: { count: 1 },
      list: [1, 2],
    };
    const fields = {
      count: "number",
      on: "boolean",
      kind: ["lan", "relayed"],
      event: "code",
      wrongEnum: ["lan"],
      notCode: "code",
      negative: "number",
      nan: "number",
      infinite: "number",
      nested: "number",
      list: "number",
    };
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const r = withBuiltins(
      source(manifest({ debug: [provider({ fields })] })),
      loaderOf({ read: () => values }),
    );
    const p = r.debug.find((d) => d.id === "meter.state");
    if (!p) throw new Error("no provider");
    const read = await r.readDebug(p);
    expect({ ...read.values }).toEqual({
      count: 3,
      on: true,
      kind: "relayed",
      event: "net.conn.close",
      "3fa9c2e1d4b5a607.count": 7,
      "2.kind": "lan",
    });
    expect(read.dropped).toBe(12);
    expect(Object.isFrozen(read.values)).toBe(true);
    expect(warn).toHaveBeenCalledWith("debug meter.state: 12 values broke rule V");
    // A provider whose read is not a record, or fails, reads as empty and is reported.
    for (const module of [
      { read: () => "text" },
      { read: () => [1] },
      {
        read: () => {
          throw new Error("x");
        },
      },
      {},
    ]) {
      const rr = withBuiltins(source(manifest({ debug: [provider()] })), loaderOf(module));
      const pp = rr.debug.find((d) => d.id === "meter.state");
      if (!pp) throw new Error("no provider");
      const empty = await rr.readDebug(pp);
      expect({ ...empty.values }).toEqual({});
      expect(empty.dropped).toBe(1);
    }
    warn.mockRestore();
  });

  it("OBS1 Z-4 a debug provider runs only when read", async () => {
    const read = vi.fn(() => ({ count: 1 }));
    const loader = loaderOf({ read });
    const r = withBuiltins(source(manifest({ debug: [provider()] })), loader);
    // Building the registry, opening files and loading the other entries run no provider.
    r.open("main.typ", { content: "text" });
    for (const c of r.commands) await r.loadCommand(c);
    for (const g of r.grammars) await r.languageSupport(g.id);
    expect(loader.loads.filter((l) => l.startsWith("meter/"))).toEqual([]);
    expect(read).not.toHaveBeenCalled();
    const p = r.debug.find((d) => d.id === "meter.state");
    if (!p) throw new Error("no provider");
    await r.readDebug(p);
    await r.readDebug(p);
    // Each read runs it once; its module is loaded once.
    expect(read).toHaveBeenCalledTimes(2);
    expect(loader.loads.filter((l) => l.startsWith("meter/"))).toEqual(["meter/debug/read.ts"]);
  });
});
