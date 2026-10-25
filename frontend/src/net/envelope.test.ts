import { describe, expect, it } from "vitest";
import vectors from "../../../internal/bridge/testdata/envelopes.json";
import { decodeEnvelope, encodeEnvelope, Kind, MalformedEnvelope } from "./envelope";

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

describe("decodeEnvelope rejects", () => {
  const cases: [string, string][] = [
    ["empty", ""],
    ["unknown version", "0203016161"],
    ["unknown kind", "0109016161"],
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
