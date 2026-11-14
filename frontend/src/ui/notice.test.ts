import { afterEach, describe, expect, it, vi } from "vitest";
import { notice } from "./notice.svelte";

describe("notice queue (AdwToastOverlay: one toast at a time)", () => {
  afterEach(() => {
    vi.useRealTimers();
    while (notice.text) notice.hide();
  });

  it("shows a queued toast once the one up has gone, with its own role", () => {
    vi.useFakeTimers();
    notice.queue("« absent.md » est introuvable.", "alert");
    notice.queue("« brouillon.md » n'est dans aucun projet.");
    expect([notice.text, notice.role]).toEqual(["« absent.md » est introuvable.", "alert"]);
    vi.advanceTimersByTime(7000);
    expect([notice.text, notice.role]).toEqual([
      "« brouillon.md » n'est dans aucun projet.",
      "status",
    ]);
    notice.hide(); // the close button
    expect(notice.text).toBe("");
  });
});
