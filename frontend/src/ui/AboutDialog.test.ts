import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import AboutDialog from "./AboutDialog.svelte";
import { cleanup, click, render } from "./testkit.svelte";

// happy-dom has no modal dialog: give it the two methods the component calls.
beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
    this.dispatchEvent(new Event("close"));
  };
});
afterEach(cleanup);

const info = { Version: "0.1.0-rc.1", Commit: "0123456789abcdef", Date: "2026-10-26T08:00:00Z" };

describe("about dialog", () => {
  it("PKG-07 shows the version Go reports, the short commit and the day", () => {
    const host = render(AboutDialog, { open: true, info });
    flushSync();
    const dialog = host.querySelector("dialog");
    expect(dialog?.hasAttribute("open")).toBe(true);
    expect(dialog?.getAttribute("aria-labelledby")).toBe("about-title");
    const text = dialog?.textContent ?? "";
    expect(text).toContain("Inkwell");
    expect(text).toContain("Version 0.1.0-rc.1");
    expect(text).toContain("Révision 0123456, 2026-10-26");
    expect(text).toContain("MPL-2.0");
  });

  it("says only what it knows while the version is not loaded or has no commit", () => {
    const host = render(AboutDialog, { open: true });
    flushSync();
    expect(host.textContent).not.toContain("Version ");
    cleanup();

    const dev = render(AboutDialog, {
      open: true,
      info: { Version: "0.0.0-dev", Commit: "", Date: "" },
    });
    flushSync();
    expect(dev.textContent).toContain("Version 0.0.0-dev");
    expect(dev.textContent).not.toContain("Révision");
  });

  it("closes with its button and tells its owner", () => {
    const host = render(AboutDialog, { open: true, info });
    flushSync();
    click(host.querySelector<HTMLElement>("button.close") as HTMLElement);
    expect(host.querySelector("dialog")?.hasAttribute("open")).toBe(false);
  });
});
