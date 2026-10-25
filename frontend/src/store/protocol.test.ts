import { describe, expect, it } from "vitest";
import vectors from "../../../internal/bridge/testdata/storestream.json";
import {
  decodeRange,
  decodeRequest,
  decodeResponse,
  encodeRange,
  encodeRequest,
  encodeResponse,
  Op,
  ProtocolError,
  Status,
  validComponent,
} from "./protocol";

const hex = (b: Uint8Array) => Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
const unhex = (s: string) => Uint8Array.from(s.match(/../g) ?? [], (h) => Number.parseInt(h, 16));

describe("store protocol golden vectors (shared with Go)", () => {
  for (const v of vectors) {
    it(v.name, () => {
      if (v.type === "request") {
        const req = {
          id: v.id as number,
          op: Op[v.op as keyof typeof Op],
          key: v.key as string[],
          data: unhex(v.dataHex as string),
        };
        expect(hex(encodeRequest(req))).toBe(v.wireHex);
        const back = decodeRequest(unhex(v.wireHex));
        expect(back).toEqual(req);
      } else if (v.type === "response") {
        const resp = {
          id: v.id as number,
          status: Status[v.status as keyof typeof Status],
          payload: unhex(v.payloadHex as string),
        };
        expect(hex(encodeResponse(resp))).toBe(v.wireHex);
        expect(decodeResponse(unhex(v.wireHex))).toEqual(resp);
      } else {
        const entries = (v.entries as { key: string[]; dataHex: string }[]).map((e) => ({
          key: e.key,
          data: unhex(e.dataHex),
        }));
        expect(hex(encodeRange(entries))).toBe(v.wireHex);
        expect(decodeRange(unhex(v.wireHex))).toEqual(entries);
      }
    });
  }
});

describe("encodeRequest rejects what Go would", () => {
  const req = (key: string[], id = 1) => ({ id, op: Op.Load, key, data: new Uint8Array() });
  it.each([
    ["a slash", ["a/b"]],
    ["an empty component", [""]],
    ["a dot dot", [".."]],
    ["a space", ["a b"]],
    ["a non-ASCII character", ["é"]],
    ["a component over 128 characters", ["a".repeat(129)]],
    ["five components", ["a", "b", "c", "d", "e"]],
  ])("%s", (_name, key) => {
    expect(() => encodeRequest(req(key))).toThrow(ProtocolError);
  });
  it("id 0 (reserved for pushes)", () => {
    expect(() => encodeRequest(req(["a"], 0))).toThrow(ProtocolError);
  });
  it("accepts the longest valid component", () => {
    expect(encodeRequest(req(["a".repeat(128)])).length).toBe(4 + 1 + 1 + 1 + 128);
  });
});

describe("decoders reject malformed frames", () => {
  it("short response", () => expect(() => decodeResponse(unhex("000000"))).toThrow(ProtocolError));
  it("unknown status", () => {
    expect(() => decodeResponse(unhex("0000000109"))).toThrow(ProtocolError);
  });
  it.each([
    ["empty", ""],
    ["count without data", "00000001"],
    ["truncated key", "000000010201 61".replace(" ", "")],
    ["data past the end", "0000000101016100000009 01".replace(" ", "")],
    ["trailing bytes", "0000000007"],
  ])("range: %s", (_name, wire) => {
    expect(() => decodeRange(unhex(wire))).toThrow(ProtocolError);
  });
});

describe("validComponent", () => {
  it("follows ^[A-Za-z0-9._-]{1,128}$ without . and ..", () => {
    for (const ok of ["a", "A-z_0.9", "2akvofn6bxdksK3uVG6qU6m9fyEP", "main.typ", "..a", "a.."]) {
      expect(validComponent(ok), ok).toBe(true);
    }
    for (const bad of ["", ".", "..", "a/b", "a~", "a b", "é"]) {
      expect(validComponent(bad), bad).toBe(false);
    }
  });
});
