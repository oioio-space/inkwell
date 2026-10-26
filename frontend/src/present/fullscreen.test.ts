import { describe, expect, it } from "vitest";
import { type FullscreenHost, toggleFullscreen } from "./fullscreen";

function host(refuse = false) {
  const h = {
    fullscreenElement: null as Element | null,
    documentElement: {
      requestFullscreen: async () => {
        if (refuse) throw new Error("denied");
        h.fullscreenElement = {} as Element;
      },
    },
    exitFullscreen: async () => {
      h.fullscreenElement = null;
    },
  };
  return h satisfies FullscreenHost;
}

describe("toggleFullscreen", () => {
  it("PRS-30 goes full screen and back", async () => {
    const h = host();
    expect(await toggleFullscreen(h)).toBe(true);
    expect(h.fullscreenElement).not.toBeNull();
    expect(await toggleFullscreen(h)).toBe(false);
    expect(h.fullscreenElement).toBeNull();
  });

  it("stays as it was when the browser refuses", async () => {
    expect(await toggleFullscreen(host(true))).toBe(false);
  });
});
