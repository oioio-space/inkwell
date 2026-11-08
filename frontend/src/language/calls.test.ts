import { expect, it } from "vitest";
import { calleeOffset } from "./calls";

it("locates the callee for incomplete nested calls without treating quoted parentheses as calls", () => {
  expect(calleeOffset("#demo(", 6)).toBe(1);
  expect(calleeOffset("#demo(other(), ", 15)).toBe(1);
  expect(calleeOffset('#demo("(", ', 11)).toBe(1);
  expect(calleeOffset("#module.f(1, ", 13)).toBe(1);
  expect(calleeOffset("ordinary text", 13)).toBeUndefined();
});
