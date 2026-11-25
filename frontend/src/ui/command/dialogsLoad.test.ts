import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { registry } from "../../extensions";
import { installKeys, useRegistry } from "../../extensions/commands";
import { closeDialog, dialogs, loaders, openDialog } from "./dialogs.svelte";

// A loader that resolves when the test says so, with the real component.
function held(which: "palette" | "shortcuts") {
  const real = loaders[which];
  let release = () => {};
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  vi.spyOn(loaders, which).mockImplementationOnce(() => gate.then(() => real()));
  return () => release();
}

let removeKeys = () => {};
beforeEach(() => {
  useRegistry(registry, { system: "linux" });
  removeKeys = installKeys();
});
afterEach(() => {
  removeKeys();
  closeDialog();
  vi.restoreAllMocks();
});

describe("EX0b2 dialogs loading", () => {
  it("EX0b2 P-10b a dialog still loading never opens after Escape or after a later request", async () => {
    // Escape while the palette's chunk loads: it never opens.
    const palette = held("palette");
    const first = openDialog("palette", document.body);
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    palette();
    await first;
    expect(dialogs.open).toBeUndefined();

    // The shortcuts window asked first and still loading, then the palette: the palette stays.
    const shortcuts = held("shortcuts");
    const slow = openDialog("shortcuts", document.body);
    await openDialog("palette", document.body);
    expect(dialogs.open).toBe("palette");
    shortcuts();
    await slow;
    expect(dialogs.open).toBe("palette");
  });
});
