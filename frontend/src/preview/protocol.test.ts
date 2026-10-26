import { describe, expect, it } from "vitest";
import { encodeRequest, MAX_DIAGNOSTICS, MAX_PAGES, parseMessage } from "./protocol";

const frame = (v: unknown) => JSON.stringify(v);

describe("preview protocol", () => {
  it("PRV-50 reads a result with its pages, count and diagnostics", () => {
    const m = parseMessage(
      frame({
        type: "result",
        seq: 7,
        rev: 3,
        pageCount: 12,
        first: 4,
        pages: ["<svg/>", "<svg/>"],
        diagnostics: [
          { severity: "error", file: "a.typ", message: "oups", line: 3, column: 9 },
          { severity: "warning", message: "hm" },
        ],
      }),
    );
    expect(m).toMatchObject({ type: "result", seq: 7, rev: 3, pageCount: 12, first: 4 });
    if (m?.type !== "result") throw new Error("not a result");
    expect(m.pages).toHaveLength(2);
    expect(m.diagnostics).toEqual([
      { severity: "error", file: "a.typ", message: "oups", line: 3, column: 9 },
      { severity: "warning", message: "hm" },
    ]);
    expect(m.failure).toBeUndefined();
  });

  it("PRV-50 reads a failure with the missing packages", () => {
    const m = parseMessage(
      frame({
        type: "result",
        seq: 1,
        failure: { kind: "package", packages: ["@preview/x:1.0.0"] },
      }),
    );
    expect(m).toMatchObject({ failure: { kind: "package", packages: ["@preview/x:1.0.0"] } });
  });

  it("PRV-50 reads compiling and anchors", () => {
    expect(parseMessage(frame({ type: "compiling", seq: 2 }))).toEqual({
      type: "compiling",
      seq: 2,
    });
    expect(
      parseMessage(
        frame({ type: "anchors", anchors: [{ page: 1, x: 70, y: 71, text: "T", heading: true }] }),
      ),
    ).toMatchObject({ type: "anchors", anchors: [{ heading: true, page: 1, text: "T" }] });
  });

  it("PRV-51 refuses what is not a known message, without throwing", () => {
    for (const bad of [
      "",
      "{",
      "null",
      "[]",
      '"result"',
      frame({}),
      frame({ type: "result" }),
      frame({ type: "result", seq: -1 }),
      frame({ type: "result", seq: 1.5 }),
      frame({ type: "nope", seq: 1 }),
      frame({ type: "compiling" }),
    ]) {
      expect(parseMessage(bad), bad).toBeUndefined();
    }
    expect(parseMessage(new Uint8Array([0xff, 0xfe, 0x00]))).toBeUndefined();
  });

  it("PRV-51 drops malformed entries and bounds the lists", () => {
    const many = Array.from({ length: MAX_DIAGNOSTICS + 50 }, () => ({
      severity: "warning",
      message: "w",
    }));
    const m = parseMessage(
      frame({
        type: "result",
        seq: 1,
        pages: Array.from({ length: MAX_PAGES + 5 }, () => "<svg/>").concat([42 as never]),
        diagnostics: [{ severity: "fatal", message: "x" }, { message: "no severity" }, ...many],
        failure: { kind: "made-up" },
      }),
    );
    if (m?.type !== "result") throw new Error("not a result");
    expect(m.pages).toHaveLength(MAX_PAGES);
    expect(m.diagnostics).toHaveLength(MAX_DIAGNOSTICS - 2); // the 500 first entries are read, two are bad
    expect(m.failure).toBeUndefined();
  });

  it("PRV-51 ignores positions that are not numbers and pages that are not whole", () => {
    const m = parseMessage(
      frame({
        type: "anchors",
        anchors: [
          { page: 0, x: 1, y: 1, text: "a" },
          { page: 1, x: "1", y: 1, text: "a" },
          { page: 1, x: 1, y: 1, text: 5 },
          { page: 1, x: 1, y: 1, text: "ok" },
        ],
      }),
    );
    if (m?.type !== "anchors") throw new Error("not anchors");
    expect(m.anchors.map((a) => a.text)).toEqual(["ok"]);
  });

  it("PRV-52 writes requests Go accepts", () => {
    const text = (r: Parameters<typeof encodeRequest>[0]) =>
      new TextDecoder().decode(encodeRequest(r));
    expect(text({ op: "view", first: 2, last: 5 })).toBe('{"op":"view","first":2,"last":5}');
    expect(text({ op: "edit", time: 1700000000 })).toBe('{"op":"edit","time":1700000000}');
    expect(text({ op: "refresh" })).toBe('{"op":"refresh"}');
    expect(text({ op: "anchors" })).toBe('{"op":"anchors"}');
  });
});
