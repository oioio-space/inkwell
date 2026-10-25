import { describe, expect, it } from "vitest";
import { PERSON_COLORS, personColor } from "./people";

describe("personColor", () => {
  it("is stable for a given peer", () => {
    expect(personColor("k7f3abc")).toBe(personColor("k7f3abc"));
  });

  it("always picks one of the 8 Okabe-Ito tokens", () => {
    for (const id of ["a", "b", "zz", "k7f3abc", "0123456789abcdef"]) {
      expect(PERSON_COLORS).toContain(personColor(id));
    }
  });

  it("spreads peers across the palette", () => {
    const used = new Set(Array.from({ length: 64 }, (_, i) => personColor(`peer-${i}`)));
    expect(used.size).toBeGreaterThanOrEqual(6);
  });
});
