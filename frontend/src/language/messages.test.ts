import { describe, expect, it } from "vitest";
import { documentation, readable } from "./documentation";
import { explain } from "./messages";

describe("ER-3 ER-4", () => {
  it("maps compiler messages deterministically and retains unknown raw text", () => {
    expect(explain("unknown variable: title").summary).toBe("Ce nom n'est pas défini.");
    expect(explain("unclosed delimiter").action).toBe("Ajoutez le signe de fermeture.");
    expect(explain("<script>attack()</script>").summary).toBe("<script>attack()</script>");
  });
  it("explains the common messages in French and keeps the code of a hover readable", () => {
    expect(explain("expected comma").summary).toBe("Il manque une virgule.");
    expect(explain("unknown font family: x").summary).toBe("Cette police n'est pas installée.");
    expect(readable("```typc\nlet f()\n```\n\n---\n\nSee [figure](https://typst.app/x).")).toBe(
      "let f()\n\nSee figure.",
    );
  });
  it("keeps peer documentation as plain text", () => {
    expect(documentation({ kind: "markdown", value: "<img src=x onerror=attack()>" })).toBe(
      "<img src=x onerror=attack()>",
    );
  });
});
