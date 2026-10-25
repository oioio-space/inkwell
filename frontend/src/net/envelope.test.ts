import { describe, expect, it } from "vitest";
import vectors from "../../../internal/bridge/testdata/envelopes.json";
import {
  decodeEnvelope,
  decodeHeads,
  decodeRoles,
  decodeSuspended,
  encodeEnvelope,
  encodeHeads,
  encodeRoles,
  Kind,
  MalformedEnvelope,
} from "./envelope";

const hex = (b: Uint8Array) => Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
const unhex = (s: string) => Uint8Array.from(s.match(/../g) ?? [], (h) => Number.parseInt(h, 16));

describe("envelope golden vectors (shared with Go)", () => {
  for (const v of vectors) {
    it(v.name, () => {
      const env = {
        kind: Kind[v.kind as keyof typeof Kind],
        peer: v.peer,
        payload: unhex(v.payloadHex),
      };
      expect(hex(encodeEnvelope(env))).toBe(v.wireHex);
      const back = decodeEnvelope(unhex(v.wireHex));
      expect(back.kind).toBe(env.kind);
      expect(back.peer).toBe(v.peer);
      expect(hex(back.payload)).toBe(v.payloadHex);
    });
  }
});

describe("golden vectors cover every kind", () => {
  it("has one vector per kind", () => {
    const kinds = new Set(vectors.map((v) => v.kind));
    expect([...kinds].sort()).toEqual(Object.keys(Kind).sort());
  });
});

describe("heads payload", () => {
  const h = {
    doc: "doc1",
    heads: ["01".repeat(32), "fe".repeat(32)],
  };
  it("round-trips and matches the Go vector", () => {
    const v = vectors.find((x) => x.name === "attest two heads");
    expect(hex(encodeHeads(h))).toBe(v?.payloadHex);
    expect(decodeHeads(unhex(v?.payloadHex ?? ""))).toEqual(h);
  });
  it("keeps a multi-byte document name", () => {
    expect(decodeHeads(encodeHeads({ doc: "é", heads: [] })).doc).toBe("é");
  });
  const bad: [string, () => unknown][] = [
    ["empty document", () => encodeHeads({ doc: "", heads: [] })],
    ["document too long", () => encodeHeads({ doc: "d".repeat(257), heads: [] })],
    ["65 heads", () => encodeHeads({ doc: "d", heads: Array(65).fill("00".repeat(32)) })],
    ["head not hex", () => encodeHeads({ doc: "d", heads: ["zz".repeat(32)] })],
    ["head too short", () => encodeHeads({ doc: "d", heads: ["00"] })],
    ["truncated", () => decodeHeads(unhex("00"))],
    ["short head", () => decodeHeads(unhex(`000164${"00".repeat(31)}`))],
    ["invalid UTF-8", () => decodeHeads(unhex("0001ff"))],
  ];
  for (const [name, fn] of bad) {
    it(`rejects ${name}`, () => {
      expect(fn).toThrow(MalformedEnvelope);
    });
  }
});

describe("roles payload", () => {
  const members = [
    { peer: "zz-9~", role: "admin", suspended: false },
    { peer: "a", role: "writer", suspended: true },
  ] as const;
  it("round-trips and matches the Go vector", () => {
    const v = vectors.find((x) => x.name === "roles two members");
    expect(hex(encodeRoles([...members]))).toBe(v?.payloadHex);
    expect(decodeRoles(unhex(v?.payloadHex ?? ""))).toEqual(members);
  });
  it("decodes the empty list", () => {
    expect(decodeRoles(unhex("0000"))).toEqual([]);
  });
  const bad: [string, () => unknown][] = [
    ["empty", () => decodeRoles(unhex(""))],
    ["truncated count", () => decodeRoles(unhex("00"))],
    ["count without entries", () => decodeRoles(unhex("0001"))],
    ["trailing byte", () => decodeRoles(unhex("000007"))],
    ["role 0", () => decodeRoles(unhex("000101700000"))],
    ["role 4", () => decodeRoles(unhex("000101700400"))],
    ["unknown flag", () => decodeRoles(unhex("000101700202"))],
    ["empty peer", () => decodeRoles(unhex("00010002" + "00"))],
    ["space in peer", () => decodeRoles(unhex("0001012002" + "00"))],
    ["truncated peer", () => decodeRoles(unhex("0001057002" + "00"))],
    ["more than 1024 members", () => decodeRoles(unhex("0401"))],
    ["encode: bad peer", () => encodeRoles([{ peer: "a b", role: "reader", suspended: false }])],
    [
      "encode: too many",
      () => encodeRoles(Array(1025).fill({ peer: "a", role: "reader", suspended: false })),
    ],
  ];
  for (const [name, fn] of bad) {
    it(`rejects ${name}`, () => {
      expect(fn).toThrow(MalformedEnvelope);
    });
  }
});

describe("suspended payload", () => {
  it("reads suspended and resumed", () => {
    expect(decodeSuspended(unhex("01"))).toBe(true);
    expect(decodeSuspended(unhex("00"))).toBe(false);
  });
  for (const [name, payload] of [
    ["empty", ""],
    ["2", "02"],
    ["two bytes", "0101"],
  ] as const) {
    it(`rejects ${name}`, () => {
      expect(() => decodeSuspended(unhex(payload))).toThrow(MalformedEnvelope);
    });
  }
});

describe("decodeEnvelope rejects", () => {
  const cases: [string, string][] = [
    ["empty", ""],
    ["unknown version", "0203016161"],
    ["unknown kind", "010a016161"],
    ["roles without payload", "0108016161"],
    ["roles with a trailing byte", "0108016161000007"],
    ["suspended without payload", "0109016161"],
    ["suspended with 2", "010901616102"],
    ["attest without payload", "0104016161"],
    ["attested with a short head", "0105016100016400"],
    ["payload on Revoked", "010601616162"],
    ["payload on Sealed", "010701616162"],
    ["truncated peer", "01030561"],
    ["empty peer", "010300"],
    ["control byte in peer", "0103010a"],
    ["payload on PeerUp", "010101616263"],
  ];
  for (const [name, wire] of cases) {
    it(name, () => {
      expect(() => decodeEnvelope(unhex(wire))).toThrow(MalformedEnvelope);
    });
  }
});
