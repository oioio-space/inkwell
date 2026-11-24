import { describe, expect, it } from "vitest";
import { registry } from "../extensions";
import { offersTable } from "./viewChoice.svelte";

const TAB = "nom\tqte\nA\t1\nB\t2\nC\t3\n";

describe("the « open as a table » offer (code review, finding 2)", () => {
  it("a text that reads as a table is offered as one, never forced, and nothing else is", () => {
    const text = (name: string) => registry.open(name, { content: "text" });
    expect(offersTable(text("notes.txt"), TAB)).toBe(true);
    expect(offersTable(text("README"), TAB)).toBe(true);
    expect(offersTable(text("notes.txt"), "Bonjour,\nceci est un texte\nsans tableau.\n")).toBe(
      false,
    );
    expect(offersTable(text("notes.txt"), undefined)).toBe(false);
    // Already a table, a type of its own, or the person's choice: no offer.
    expect(offersTable(text("data.csv"), TAB)).toBe(false);
    expect(offersTable(text("chapter.typ"), TAB)).toBe(false);
    expect(offersTable(registry.open("notes.txt", { content: "text", choice: "csv" }), TAB)).toBe(
      false,
    );
    expect(offersTable(registry.open("blob.bin", { content: "binary" }), TAB)).toBe(false);
  });
});
