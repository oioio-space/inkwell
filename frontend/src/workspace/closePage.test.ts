import { expect, it, vi } from "vitest";
import { ClosePage } from "./closePage";

it("confirms an empty cut before workspace startup without waiting on presence", async () => {
  const save = vi.fn(async () => false);
  const goodbye = vi.fn();
  const restore = vi.fn();
  const send = vi.fn();
  const page = new ClosePage({ save, goodbye, restore, describe: () => ({}) });

  page.receive({ close: "prepare", attempt: 1 }, send);
  await Promise.resolve();

  expect(send.mock.calls).toEqual([[{ close: "empty", attempt: 1 }]]);
  expect(goodbye).not.toHaveBeenCalled();
  expect(restore).not.toHaveBeenCalled();
  page.receive({ close: "abort", attempt: 1 }, send);
  expect(restore).toHaveBeenCalledOnce();
});
