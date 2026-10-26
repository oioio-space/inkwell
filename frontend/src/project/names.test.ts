import fc from "fast-check";
import { describe, expect, it } from "vitest";
import vectors from "../../../internal/project/testdata/names.json";
import {
  collisionKey,
  sanitizeName,
  splitExt,
  validateName,
  validateNameBytes,
  validatePath,
  validatePathBytes,
} from "./names";

const unhex = (s: string) => Uint8Array.from(s.match(/../g) ?? [], (h) => Number.parseInt(h, 16));

interface Vector {
  name: string;
  input?: string;
  inputHex?: string;
  valid: boolean;
  reason?: string;
}

describe("PRJ-10 name validator, golden vectors shared with Go", () => {
  for (const v of vectors.names as Vector[]) {
    it(`name: ${v.name}`, () => {
      const got = v.inputHex
        ? validateNameBytes(unhex(v.inputHex))
        : validateName(v.input as string);
      expect(got).toBe(v.reason);
      expect(got === undefined).toBe(v.valid);
    });
  }
  for (const v of vectors.paths as Vector[]) {
    it(`path: ${v.name}`, () => {
      const got = v.inputHex
        ? validatePathBytes(unhex(v.inputHex))
        : validatePath(v.input as string);
      expect(got).toBe(v.reason);
    });
  }
  for (const v of vectors.keys as { input: string; key: string }[]) {
    it(`key of ${JSON.stringify(v.input)}`, () => {
      expect(collisionKey(v.input)).toBe(v.key);
    });
  }
});

describe("PRJ-17 collisions", () => {
  it("case and NFC/NFD names share a key", () => {
    expect(collisionKey("Notes.md")).toBe(collisionKey("notes.MD"));
    expect(collisionKey("Café")).toBe(collisionKey("CAFÉ"));
    expect(collisionKey("a")).not.toBe(collisionKey("b"));
  });
});

describe("sanitizeName", () => {
  it("leaves a valid name alone", () => {
    for (const ok of ["chapter1.typ", "Résumé final.md", ".gitignore", "COM10"]) {
      expect(sanitizeName(ok)).toBe(ok);
    }
  });

  it("repairs the names a drop can bring", () => {
    expect(sanitizeName("a/b")).toBe("a_b");
    expect(sanitizeName("CON.txt")).toBe("_CON.txt");
    expect(sanitizeName("note. ")).toBe("note");
    expect(sanitizeName(".git")).toBe("_git");
    expect(sanitizeName("..")).toBe("_");
    expect(sanitizeName("")).toBe("_");
  });

  it("PRJ-15 always yields a valid name, whatever the input", () => {
    const arb = fc.oneof(
      fc.string({ unit: "binary" }),
      fc.string({ unit: "grapheme", maxLength: 300 }),
      fc.constantFrom("CON", "nul.txt", "LPT¹", "INKWEL~1", ".GIT", "a".repeat(400), " ", "..."),
      fc.string({ unit: "binary-ascii" }),
    );
    fc.assert(
      fc.property(arb, (raw) => {
        expect(validateName(sanitizeName(raw))).toBeUndefined();
      }),
      { numRuns: 2000 },
    );
  });

  it("is idempotent", () => {
    fc.assert(
      fc.property(fc.string({ unit: "binary" }), (raw) => {
        const once = sanitizeName(raw);
        expect(sanitizeName(once)).toBe(once);
      }),
      { numRuns: 1000 },
    );
  });
});

describe("splitExt", () => {
  it("splits the last extension and ignores a leading dot", () => {
    expect(splitExt("a.tar.gz")).toEqual({ stem: "a.tar", ext: ".gz" });
    expect(splitExt(".env")).toEqual({ stem: ".env", ext: "" });
    expect(splitExt("notes")).toEqual({ stem: "notes", ext: "" });
  });
});
