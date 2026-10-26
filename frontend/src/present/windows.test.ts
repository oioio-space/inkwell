import { describe, expect, it } from "vitest";
import { type LayoutAnswer, PublicWindows } from "./windows";

function setup(answer: LayoutAnswer) {
  const calls: string[] = [];
  const popup = { closed: false, close: () => (popup.closed = true) };
  const windows = new PublicWindows(
    {
      Open: async (swapped) => {
        calls.push(`open:${swapped}`);
        return answer;
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
  it("PRS-30 on the desktop Go opens the window, and says whether it has a screen of its own", async () => {
    const { windows, calls } = setup({ Native: true, Dual: true });
    expect(await windows.open(false)).toEqual({ dual: true, native: true });
    expect(await windows.open(true)).toEqual({ dual: true, native: true }); // swap
    expect(calls).toEqual(["open:false", "open:true"]);
  });

  it("PRS-32 one screen: a movable window", async () => {
    const { windows } = setup({ Native: true, Dual: false });
    expect(await windows.open(false)).toEqual({ dual: false, native: true });
  });

  it("PRS-32 in the browser build the same view opens as a pop-up, once", async () => {
    const { windows, calls, popup } = setup({ Native: false, Dual: false });
    expect(await windows.open(false)).toEqual({ dual: false, native: false });
    await windows.open(false);
    expect(calls.filter((c) => c.startsWith("popup"))).toEqual([
      "popup:http://127.0.0.1:1/?view=public:inkwell-public",
    ]);
    popup.closed = true; // the person closed it: opening again makes a new one
    await windows.open(false);
    expect(calls.filter((c) => c.startsWith("popup"))).toHaveLength(2);
  });

  it("closes both the pop-up and the native window", async () => {
    const { windows, calls, popup } = setup({ Native: false, Dual: false });
    await windows.open(false);
    await windows.close();
    expect(popup.closed).toBe(true);
    expect(calls.at(-1)).toBe("close");
  });
});
