import { expect, test } from "@playwright/test";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";
import { ALICE_URL, BOB_URL } from "./ports";

// FMT-29: A bolds a word while B types inside it. Both converge; the delimiters wrap the
// word, extended by B's letter or not, and no letter is lost.
//
// The commands are wired to Ctrl+B by the editor's keymap (task A4).

test("FMT-29 one person bolds a word while the other types inside it: both converge, no letter lost", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);

  await typeAtEnd(alice, "un mot ici");
  await expect.poll(() => docText(bob)).toBe("un mot ici");

  // Alice selects "mot" with the keyboard while Bob puts a caret after "mo" and types "x".
  await editor(alice).click();
  await alice.keyboard.press("ControlOrMeta+Home");
  for (let i = 0; i < 3; i++) await alice.keyboard.press("ArrowRight");
  for (let i = 0; i < 3; i++) await alice.keyboard.press("Shift+ArrowRight");
  await editor(bob).click();
  await bob.keyboard.press("ControlOrMeta+Home");
  for (let i = 0; i < 5; i++) await bob.keyboard.press("ArrowRight");

  await Promise.all([alice.keyboard.press("ControlOrMeta+b"), bob.keyboard.type("x")]);

  await expect.poll(async () => (await docText(alice)) === (await docText(bob))).toBe(true);
  const text = await docText(alice);
  // The delimiters (Typst `*`, Markdown `**`) come off; what is left is Bob's word, whole.
  expect(text.replace(/\*+/g, "")).toBe("un moxt ici");
  expect(text.match(/\*+/g)?.length).toBe(2);
});
