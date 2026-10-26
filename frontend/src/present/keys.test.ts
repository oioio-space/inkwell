import { describe, expect, it } from "vitest";
import { KeyReader } from "./keys";

const read = (r: KeyReader, ...keys: string[]) => keys.map((key) => r.read({ key }));

describe("KeyReader", () => {
  it("PRS-05 reads the keys of a presentation", () => {
    const r = new KeyReader();
    const kinds = read(r, "ArrowRight", " ", "PageDown", "ArrowLeft", "PageUp", "Home", "End").map(
      (a) => a?.kind,
    );
    expect(kinds).toEqual(["next", "next", "next", "prev", "prev", "first", "last"]);
  });

  it("PRS-12 reads B, W and L in either case, and Esc", () => {
    const r = new KeyReader();
    expect(read(r, "b", "W", "l", "Escape").map((a) => a?.kind)).toEqual([
      "black",
      "white",
      "laser",
      "quit",
    ]);
  });

  it("PRS-05 a number then Enter goes to that slide", () => {
    const r = new KeyReader();
    expect(read(r, "1", "2")).toEqual([undefined, undefined]);
    expect(r.buffer).toBe("12");
    expect(r.read({ key: "Enter" })).toEqual({ kind: "goto", n: 12 });
    expect(r.buffer).toBe("");
  });

  it("PRS-05 Esc and Backspace undo the digits before they leave or go back", () => {
    const r = new KeyReader();
    read(r, "4", "5");
    expect(r.read({ key: "Backspace" })).toBeUndefined();
    expect(r.buffer).toBe("4");
    expect(r.read({ key: "Escape" })).toBeUndefined(); // clears the number, does not leave
    expect(r.buffer).toBe("");
    expect(r.read({ key: "Escape" })?.kind).toBe("quit");
    expect(r.read({ key: "Backspace" })?.kind).toBe("prev");
  });

  it("Enter alone is next; another key drops the digits; five digits are too many", () => {
    const r = new KeyReader();
    expect(r.read({ key: "Enter" })?.kind).toBe("next");
    read(r, "3");
    expect(r.read({ key: "ArrowRight" })?.kind).toBe("next");
    expect(r.buffer).toBe("");
    read(r, "1", "2", "3", "4", "5");
    expect(r.buffer).toBe("1234");
  });

  it("leaves shortcuts with a modifier to the page", () => {
    const r = new KeyReader();
    expect(r.read({ key: "w", ctrlKey: true })).toBeUndefined();
    expect(r.read({ key: "ArrowRight", altKey: true })).toBeUndefined();
    expect(r.read({ key: "x" })).toBeUndefined();
  });
});
