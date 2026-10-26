import { expect, test } from "./fixtures";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";
import { ALICE_URL, BOB_URL } from "./ports";

// LINK-01: pairing the same two instances again and again must never lose the link; typing a
// first line that starts with "= " (a Typst heading) right after the third pairing reaches the
// other side.
test("LINK-01 the link survives a third pairing in a row and a heading typed right after", async ({
  browser,
}) => {
  test.setTimeout(150_000);
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(bob);
  for (let i = 0; i < 3; i++) await pair(alice, bob);

  await typeAtEnd(alice, "Un paragraphe.\n");
  await expect.poll(() => docText(bob), { timeout: 20_000 }).toBe("Un paragraphe.");
  await editor(alice).click();
  await alice.keyboard.press("ControlOrMeta+Home");
  await alice.keyboard.type("= Titre\n");
  await expect.poll(() => docText(bob), { timeout: 20_000 }).toBe("= TitreUn paragraphe.");
  await typeAtEnd(bob, " suite");
  await expect.poll(() => docText(alice), { timeout: 20_000 }).toBe("= TitreUn paragraphe. suite");
  await expect(alice.getByRole("status")).toHaveText(/Synchronisé/);
  await expect(bob.getByRole("status")).toHaveText(/Synchronisé/);
});
