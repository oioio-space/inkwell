import { expect, test } from "@playwright/test";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";

// FMT-29: A bolds a word while B types inside it. Both converge; the delimiters wrap the
// word, extended by B's letter or not, and no letter is lost.
//
// The formatting commands live in frontend/src/editor/format/ (task A1) and are covered by unit
// and property tests. They are not wired into the editor yet: the Ctrl+B shortcut and the bar
// come with the Adwaita shell (task A4), and no test hook drives the commands from outside.
// Until A4 lands this stays fixme; remove the `.fixme` then.

test.fixme("FMT-29 one person bolds a word while the other types inside it: both converge, no letter lost", async ({
  browser,
}) => {
  const alice = await open(browser, "http://127.0.0.1:18081/");
  const bob = await open(browser, "http://127.0.0.1:18082/");
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);

  await typeAtEnd(alice, "un mot ici");
  await expect.poll(() => docText(bob)).toBe("un mot ici");

  // Alice selects "mot" (double-click) while Bob puts a caret after "mo" and types "x".
  await editor(alice)
    .getByText("un mot ici")
    .dblclick({ position: { x: 40, y: 8 } });
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
