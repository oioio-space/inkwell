import { describe, expect, it } from "vitest";
import { PublicWindows } from "./windows";

function setup(native: boolean) {
  const calls: string[] = [];
  const popup = { closed: false, close: () => (popup.closed = true) };
  const windows = new PublicWindows(
    {
      Open: async () => {
        calls.push("open");
        return { Native: native };
      },
      Close: async () => void calls.push("close"),
    },
    {
      location: { origin: "http://127.0.0.1:1" },
      open: (url, name) => {
        calls.push(`popup:${url}:${name}`);
        return popup;
      },
    },
  );
  return { windows, calls, popup };
}

describe("PublicWindows", () => {
  it("PRS-30 PRS-32 on the desktop Go opens an ordinary window, and no pop-up is made", async () => {
    const { windows, calls } = setup(true);
    expect(await windows.open()).toBe(true);
    expect(calls).toEqual(["open"]);
  });

  it("PRS-32 in the browser build the same view opens as a pop-up, once", async () => {
    const { windows, calls, popup } = setup(false);
    expect(await windows.open()).toBe(false);
    await windows.open();
    expect(calls.filter((c) => c.startsWith("popup"))).toEqual([
      "popup:http://127.0.0.1:1/?view=public:inkwell-public",
    ]);
    popup.closed = true; // the person closed it: opening again makes a new one
    await windows.open();
    expect(calls.filter((c) => c.startsWith("popup"))).toHaveLength(2);
  });

  it("PRS-31 a window this page did not open is not one it can close (a stale roster must not turn Open into Close)", async () => {
    const browser = setup(false);
    expect(browser.windows.canClose()).toBe(false);
    await browser.windows.open();
    expect(browser.windows.canClose()).toBe(true);
    browser.popup.closed = true;
    expect(browser.windows.canClose()).toBe(false);

    const desktop = setup(true);
    expect(desktop.windows.canClose()).toBe(false);
    await desktop.windows.open();
    expect(desktop.windows.canClose()).toBe(true);
    await desktop.windows.close();
    expect(desktop.windows.canClose()).toBe(false);
  });

  it("closes both the pop-up and the native window", async () => {
    const { windows, calls, popup } = setup(false);
    await windows.open();
    await windows.close();
    expect(popup.closed).toBe(true);
    expect(calls.at(-1)).toBe("close");
  });
});
