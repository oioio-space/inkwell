import { describe, expect, it } from "vitest";
import en from "./en.json";
import fr from "./fr.json";
import { t } from "./index";

describe("i18n", () => {
  it("has the same keys in every language", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(fr).sort());
  });
  it("never shows network jargon", () => {
    for (const text of [...Object.values(en), ...Object.values(fr)]) {
      expect(text).not.toMatch(/multiaddr|peer ?id|libp2p/i);
    }
  });
  it("fills placeholders", () => {
    expect(t("people.label", { count: 3 })).toContain("3");
  });
});
