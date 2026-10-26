import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Entry } from "../../project/materialize";
import { cleanup, click, press, render } from "../testkit.svelte";
import QuickOpen from "./QuickOpen.svelte";
import { shimDialog } from "./testkit";

beforeEach(shimDialog);
afterEach(cleanup);

const file = (id: string, path: string, type: Entry["type"] = "text"): Entry => ({
  id,
  path,
  name: path.split("/").at(-1) as string,
  type,
  parentId: null,
  size: 0,
  main: false,
  by: "x",
  at: 0,
});

const entries = [
  file("1", "main.typ"),
  file("2", "chapitres/intro.typ"),
  file("3", "chapitres/fin.typ"),
  file("4", "figures/schema.png", "blob"),
  file("5", "chapitres", "dir"),
];

const options = (host: HTMLElement) =>
  [...host.querySelectorAll<HTMLElement>('[role="option"]')].map((o) => o.textContent?.trim());

describe("FM-20 Ctrl+P opens a file by its name", () => {
  it("lists files, never folders; the latest opened come first when nothing is typed", () => {
    const host = render(QuickOpen, { open: true, entries, recent: ["3", "2"], onopen: () => {} });
    expect(options(host)).toEqual([
      "chapitres/intro.typ",
      "chapitres/fin.typ",
      "figures/schema.png",
      "main.typ",
    ]);
  });

  it("filters while typing, and Enter opens the best match", () => {
    const onopen = vi.fn();
    const host = render(QuickOpen, { open: true, entries, recent: [], onopen });
    const input = host.querySelector("input") as HTMLInputElement;
    input.value = "fin.t";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    flushSync();
    expect(options(host)).toEqual(["chapitres/fin.typ"]);
    press(input, "Enter");
    expect(onopen).toHaveBeenCalledWith("3");
  });

  it("the arrows move the active choice, and it is exposed to assistive technology", () => {
    const onopen = vi.fn();
    const host = render(QuickOpen, { open: true, entries, recent: [], onopen });
    const input = host.querySelector("input") as HTMLInputElement;
    expect(input.getAttribute("role")).toBe("combobox");
    press(input, "ArrowDown");
    const active = input.getAttribute("aria-activedescendant") as string;
    expect(host.querySelector(`#${active}`)?.getAttribute("aria-selected")).toBe("true");
    press(input, "Enter");
    expect(onopen).toHaveBeenCalledWith("2");
  });

  it("says when nothing matches, and a click opens", () => {
    const onopen = vi.fn();
    const host = render(QuickOpen, { open: true, entries, recent: [], onopen });
    const input = host.querySelector("input") as HTMLInputElement;
    input.value = "zzzz";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    flushSync();
    expect(host.textContent).toContain("Aucun fichier ne correspond.");
    input.value = "schema";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    flushSync();
    click(host.querySelector('[role="option"]') as HTMLElement);
    expect(onopen).toHaveBeenCalledWith("4");
  });
});
