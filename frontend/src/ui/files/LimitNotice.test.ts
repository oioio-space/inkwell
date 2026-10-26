import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "../testkit.svelte";
import LimitNotice from "./LimitNotice.svelte";

afterEach(cleanup);

describe("a project over its number of files", () => {
  it("says so, in words, and offers nothing to add more", () => {
    const host = render(LimitNotice, { excess: 3 });
    expect(host.textContent).toContain("Le projet dépasse la limite de fichiers");
    expect(host.textContent).toContain("3");
    expect(host.querySelector("button")).toBeNull();
  });

  it("is silent when nothing is over", () => {
    expect(render(LimitNotice, { excess: 0 }).textContent?.trim()).toBe("");
  });
});
