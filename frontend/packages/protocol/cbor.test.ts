// SPDX-License-Identifier: MPL-2.0
import fc from "fast-check";
import { describe, expect, test } from "vitest";
import { type CBORValue, decodeCBOR, encodeCBOR, InvalidCBOR } from "./src/cbor";

const key = fc.string({
  unit: fc.constantFrom("a", "B", "é", "中", "🙂", "_", "-", "0"),
  minLength: 1,
  maxLength: 6,
});
const text = fc.string({
  unit: fc.constantFrom("a", "é", "中", "🙂", "!", "\u0000"),
  maxLength: 12,
});
const leaf: fc.Arbitrary<CBORValue> = fc.oneof(
  fc.constant(null),
  fc.boolean(),
  fc.integer({ min: 0, max: Number.MAX_SAFE_INTEGER }),
  text,
  fc.uint8Array({ maxLength: 16 }),
);
const value: fc.Arbitrary<CBORValue> = fc.letrec<{ value: CBORValue }>((tie) => ({
  value: fc.oneof(
    { depthSize: "small" },
    leaf,
    fc.array(tie("value"), { maxLength: 4 }),
    fc.dictionary(key, tie("value"), { maxKeys: 4 }),
  ),
})).value;

function depth(v: CBORValue): number {
  if (Array.isArray(v)) return 1 + Math.max(0, ...v.map(depth));
  if (v !== null && typeof v === "object" && !(v instanceof Uint8Array)) {
    return 1 + Math.max(0, ...Object.values(v).map(depth));
  }
  return 1;
}

describe("CBOR validity profile (§9.2.1)", () => {
  test("P1: decode(encode(v)) = v and encode(decode(b)) = b on canonical bytes", () => {
    fc.assert(
      fc.property(value, (v) => {
        if (depth(v) > 8) return;
        const canonical = encodeCBOR(v);
        const decoded = decodeCBOR(canonical);
        expect(decoded).toEqual(v);
        expect(encodeCBOR(decoded)).toEqual(canonical);
      }),
      { numRuns: 500 },
    );
  });

  test("accepts non-preferred length widths", () => {
    const bytes = Uint8Array.from([0xb8, 0x01, 0x61, 0x61, 0x01]);
    expect(decodeCBOR(bytes)).toEqual({ a: 1 });
  });

  test("encodes maps in canonical key order", () => {
    expect(encodeCBOR({ b: 1, a: 2 })).toEqual(
      Uint8Array.from([0xa2, 0x61, 0x61, 0x02, 0x61, 0x62, 0x01]),
    );
  });

  test.each([
    ["duplicate key", [0xa2, 0x61, 0x61, 0x01, 0x61, 0x61, 0x02]],
    ["duplicate key, non-preferred width", [0xa2, 0x61, 0x61, 0x01, 0x78, 0x01, 0x61, 0x02]],
    ["indefinite map", [0xbf, 0xff]],
    ["float", [0xf9, 0x00, 0x00]],
    ["tag", [0xc0, 0x01]],
    ["negative integer", [0x20]],
    ["trailing byte", [0x01, 0x00]],
    ["integer above 2^53-1", [0x1b, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]],
    ["non-text key", [0xa1, 0x00, 0x00]],
    ["bad UTF-8", [0x61, 0xff]],
    ["depth > 8", [0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x01]],
  ])("rejects %s", (_name, bytes) => {
    expect(() => decodeCBOR(Uint8Array.from(bytes))).toThrow(InvalidCBOR);
  });
});
