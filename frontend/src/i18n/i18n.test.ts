import { describe, expect, it } from "vitest";
import en from "./en.json";
import fr from "./fr.json";
import { t } from "./index";

describe("i18n", () => {
  it("has the same keys in every language", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(fr).sort());
  });
  it("fills placeholders", () => {
    expect(t("error.join", { addr: "10.0.0.2:7419" })).toContain("10.0.0.2:7419");
  });
});
