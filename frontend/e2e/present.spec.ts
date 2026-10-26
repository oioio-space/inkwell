import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { clear, editor, open, pair } from "./helpers";
import { ALICE_URL, BOB_URL } from "./ports";

// Presentations (C4 and C4+): the presenter view and the public window on one machine, then one
// machine piloting another over the network. The browser build has no native window, so the
// public window is the same view opened as a pop-up (`/?view=public`); the native placement on
// two screens is Go's and is tested there (internal/app/windows_test.go).
const SLOW = { timeout: 45_000 };

/** Three slides; the second has two steps and a speaker note. Page 1 is the title slide. */
const DECK = `#import "@preview/touying:0.7.4": *
#import themes.simple: *
#show: simple-theme.with(aspect-ratio: "16-9")

= Premier titre

== Un

Bonjour #pause monde

#speaker-note[Souriez au public]

== Deux

Fin
`;

async function writeDeck(p: Page, text = DECK) {
  await clear(p);
  await editor(p).click();
  await p.keyboard.insertText(text);
  await expect(p.getByTestId("page-indicator")).toHaveText(/sur 4/, SLOW);
}

const view = (p: Page) => p.getByTestId("presenter-view");
const counter = (p: Page) => p.getByTestId("present-counter");

/** Opens the presenter view with F5 and waits for the first slide. */
async function present(p: Page) {
  await editor(p).click();
  await p.keyboard.press("F5");
  await expect(view(p)).toBeVisible();
  await expect(p.getByTestId("pilot-status")).toHaveText("Vous présentez");
  await expect(view(p).locator("img").first()).toBeVisible(SLOW);
}

async function leave(p: Page) {
  if (await view(p).isVisible()) await p.keyboard.press("Escape");
  await expect(view(p)).toBeHidden();
}

test("PRS-31 PRS-32 PRS-40 the public window follows the presenter, and shows only the slide", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  await writeDeck(alice);
  await present(alice);
  await expect(counter(alice)).toHaveText("Diapositive 1 sur 3");

  // One screen: the public view opens as a window of its own.
  const popup = alice.context().waitForEvent("page");
  await alice.getByTestId("present-public").click();
  const pub = await popup;
  await expect(pub).toHaveURL(/view=public/);
  const shown = pub.getByTestId("stage").locator("img");
  await expect(shown).toBeVisible(SLOW);

  // PRS-40: nothing but the slide, by the DOM.
  await pub.waitForLoadState();
  expect(await pub.locator("button, input, textarea, select, a, [contenteditable]").count()).toBe(
    0,
  );
  expect(await pub.locator("body").innerText()).not.toMatch(/Souriez|Notes|Commentaire/);
  await expect(pub.getByTestId("notes")).toHaveCount(0);

  // The person puts the window where they want and makes it full screen: a button that shows while
  // the mouse moves, and F11. At rest the window is the slide alone.
  await pub.mouse.move(200, 200);
  await pub.mouse.move(220, 210);
  const full = pub.getByTestId("public-fullscreen");
  await expect(full).toBeVisible();
  await expect(full).toBeHidden({ timeout: 6_000 });
  expect(await pub.locator("button").count()).toBe(0);
  await pub.keyboard.press("F11");
  await pub.keyboard.press("F11");
  await expect(pub.getByTestId("stage")).toBeVisible();

  // PRS-31: a step in the presenter moves the public window, with nothing to do there.
  const before = await shown.getAttribute("src");
  const t0 = Date.now();
  await alice.keyboard.press("ArrowRight");
  await expect(counter(alice)).toHaveText("Diapositive 2 sur 3");
  await expect(async () => {
    expect(await shown.getAttribute("src")).not.toBe(before);
  }).toPass({ timeout: 10_000, intervals: [10, 10, 20, 50] });
  console.log(
    `PRS-31 presenter key to public window: ${Date.now() - t0} ms (including the test's own waits)`,
  );

  // The note and the step belong to the presenter alone.
  await expect(alice.getByTestId("notes")).toHaveText("Souriez au public");
  await alice.keyboard.press("ArrowRight");
  await expect(counter(alice)).toHaveText(/Diapositive 2 sur 3 · Étape 2/);

  // The public window's own keys drive the same presentation: never two states to scroll.
  await pub.keyboard.press("ArrowRight");
  await expect(counter(alice)).toHaveText("Diapositive 3 sur 3");
  await expect(alice.getByTestId("present-last")).toBeVisible();
  await pub.keyboard.press("ArrowLeft");
  await expect(counter(alice)).toHaveText(/Diapositive 2 sur 3/);

  // Black and white screens.
  await alice.keyboard.press("b");
  await expect(pub.getByTestId("stage")).toHaveAttribute("data-screen", "black");
  await alice.keyboard.press("w");
  await expect(pub.getByTestId("stage")).toHaveAttribute("data-screen", "white");
  await pub.keyboard.press("w");
  await expect(pub.getByTestId("stage")).toHaveAttribute("data-screen", "show");

  // A number then Enter goes to that slide; Home and End.
  await alice.keyboard.press("End");
  await expect(counter(alice)).toHaveText("Diapositive 3 sur 3");
  await alice.keyboard.press("1");
  await expect(alice.getByTestId("present-typed")).toHaveText("Aller à la diapositive 1");
  await alice.keyboard.press("Enter");
  await expect(counter(alice)).toHaveText("Diapositive 1 sur 3");

  await alice.screenshot({ path: "test-results/present-presenter.png" });
  await pub.screenshot({ path: "test-results/present-public.png" });

  // Esc leaves, closes the public window, and the preview is back.
  await alice.keyboard.press("Escape");
  await expect(view(alice)).toBeHidden();
  await expect.poll(() => pub.isClosed(), SLOW).toBe(true);
  await expect(alice.locator("[data-page] img").first()).toBeVisible(SLOW);
  await clear(alice);
});

test("PRS-06 PRS-08 the timer starts at the first advance, and the size of the notes is remembered", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  await writeDeck(alice);
  await present(alice);
  await expect(alice.getByTestId("timer")).toHaveText("0:00");
  await alice.keyboard.press("ArrowRight");
  await expect(alice.getByTestId("timer")).not.toHaveText("0:00", { timeout: 5_000 });
  await alice.getByRole("button", { name: "Mettre le chronomètre en pause" }).click();
  await alice.getByRole("button", { name: "Remettre le chronomètre à zéro" }).click();
  await expect(alice.getByTestId("timer")).toHaveText("0:00");

  const size = () => alice.getByTestId("notes").evaluate((el) => getComputedStyle(el).fontSize);
  const before = Number.parseInt(await size(), 10);
  await alice.getByTestId("notes-larger").click();
  await expect.poll(async () => Number.parseInt(await size(), 10)).toBe(before + 2);
  await leave(alice);
  await present(alice);
  expect(Number.parseInt(await size(), 10)).toBe(before + 2); // remembered
  await alice.getByTestId("notes-smaller").click();
  await leave(alice);
  await clear(alice);
});

test("PRS-33 PRS-35 PRS-36 PRS-37 PRS-39 one machine pilots another; control is asked and handed over", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await writeDeck(alice);
  await pair(alice, bob);
  await expect(bob.getByTestId("page-indicator")).toHaveText(/sur 4/, SLOW);

  // Bob's machine is the audience view; Alice pilots it from hers.
  await bob.getByRole("button", { name: "Menu principal" }).click();
  await bob.getByTestId("present-as-public").click();
  await expect(bob.getByTestId("public-overlay")).toBeVisible();
  await expect(bob.getByText("En attente de la présentation…")).toBeVisible();
  await present(alice);

  const bobShows = bob.getByTestId("public-overlay").locator("img");
  await expect(bobShows).toHaveAttribute("alt", "Diapositive 1", SLOW);
  await alice.keyboard.press("ArrowRight");
  await expect(bobShows).toHaveAttribute("alt", "Diapositive 2", SLOW);
  const step1 = await bobShows.getAttribute("src");
  await alice.keyboard.press("ArrowRight");
  await expect(async () => {
    expect(await bobShows.getAttribute("src")).not.toBe(step1);
  }).toPass(SLOW);
  await alice.keyboard.press("b");
  await expect(bob.getByTestId("public-overlay").getByTestId("stage")).toHaveAttribute(
    "data-screen",
    "black",
  );
  await alice.keyboard.press("b");

  // The laser, drawn on Bob's screen where Alice points.
  await alice.keyboard.press("l");
  const slide = view(alice).getByTestId("stage").first();
  const box = await slide.boundingBox();
  if (!box) throw new Error("no slide");
  await alice.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await alice.mouse.move(box.x + box.width / 2 + 20, box.y + box.height / 2 + 10);
  await expect(bob.getByTestId("laser")).toBeVisible();
  await alice.keyboard.press("l");
  await expect(bob.getByTestId("laser")).toBeHidden();

  // PRS-39: Bob edits while Alice presents; both recompile, the position stays.
  await alice.keyboard.press("Home");
  await alice.keyboard.press("ArrowRight");
  await expect(counter(alice)).toHaveText("Diapositive 2 sur 3");
  await bob.keyboard.press("Escape"); // Bob leaves the audience view to edit
  await expect(bob.getByTestId("public-overlay")).toBeHidden();
  await editor(bob).click();
  await bob.keyboard.press("ControlOrMeta+End");
  await bob.keyboard.insertText("\n== Trois\n\nAjouté en direct\n");
  await expect(counter(alice)).toHaveText("Diapositive 2 sur 4", SLOW);

  // PRS-35 / PRS-36: Bob follows, browses on his own, comes back, then asks for the controls.
  await expect(bob.getByTestId("present-banner")).toContainText("Alice présente.");
  await bob.getByTestId("present-banner-follow").click();
  await expect(bob.getByTestId("pilot-status")).toHaveText("Alice présente");
  await expect(counter(bob)).toHaveText(/Diapositive 2 sur 4/);
  await bob.getByTestId("present-free").click();
  await bob.keyboard.press("End");
  await expect(counter(bob)).toHaveText("Diapositive 4 sur 4");
  await alice.keyboard.press("ArrowRight");
  await expect(counter(alice)).toHaveText(/Étape 2|Diapositive 3 sur 4/);
  await expect(counter(bob)).toHaveText("Diapositive 4 sur 4"); // not dragged along
  await bob.getByTestId("present-follow").click();
  await expect(counter(bob)).toHaveText(await counter(alice).innerText());

  await bob.getByTestId("present-ask").click();
  await expect(alice.getByRole("button", { name: "Refuser" })).toBeVisible();
  await alice.getByRole("button", { name: "Refuser" }).click();
  await expect(bob.getByText("La main n'a pas été accordée.")).toBeVisible();
  await bob.getByTestId("present-ask").click();
  await alice.getByRole("button", { name: "Accorder" }).click();
  await expect(bob.getByTestId("pilot-status")).toHaveText("Vous présentez");
  await expect(alice.getByTestId("pilot-status")).toHaveText("Bob présente");
  // Never two navigations: only the one who holds the controls moves the slides.
  await alice.keyboard.press("Home");
  await expect(counter(alice)).not.toHaveText("Diapositive 1 sur 4");
  await bob.keyboard.press("Home");
  await expect(counter(alice)).toHaveText("Diapositive 1 sur 4");

  await bob.screenshot({ path: "test-results/present-follower-dark.png" });
  await alice.screenshot({ path: "test-results/present-follower-light.png" });

  await leave(bob);
  await leave(alice);
  await clear(alice);
});

test("PRS-37 the presenter disappears: the audience view keeps the slide, says so, and resumes on return", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL);
  await writeDeck(alice);
  await pair(alice, bob);
  await expect(bob.getByTestId("page-indicator")).toHaveText(/sur 4/, SLOW);

  await bob.getByRole("button", { name: "Menu principal" }).click();
  await bob.getByTestId("present-as-public").click();
  await present(alice);
  await alice.keyboard.press("ArrowRight");
  const shows = bob.getByTestId("public-overlay").locator("img");
  await expect(shows).toHaveAttribute("alt", "Diapositive 2", SLOW);
  await expect(bob.getByTestId("away")).toHaveCount(0);

  // The presenter's window goes away without a word.
  await alice.close();
  await expect(bob.getByTestId("away")).toBeVisible({ timeout: 30_000 });
  await expect(shows).toHaveAttribute("alt", "Diapositive 2"); // the last slide stays
  await bob.screenshot({ path: "test-results/present-away.png" });

  // It comes back and presents again: the audience view resumes.
  const again = await open(browser, ALICE_URL);
  await present(again);
  await expect(bob.getByTestId("away")).toHaveCount(0, { timeout: 30_000 });
  await again.keyboard.press("End");
  await expect(shows).toHaveAttribute("alt", "Diapositive 3", SLOW);

  await leave(again);
  await bob.keyboard.press("Escape");
  await clear(again);
});
