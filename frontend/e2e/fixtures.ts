import { test as base } from "@playwright/test";

export { expect } from "@playwright/test";

/**
 * `test` with one guarantee: every browser context a test opens is closed when the test ends.
 *
 * A context made with `browser.newContext()` is not closed by Playwright until the browser
 * itself goes away, so its page would live on through the next tests. An Inkwell page that
 * lives on keeps talking to its instance: it reconnects to the store stream (which serves one
 * page at a time) and takes it from the page under test, writes its own text to main.typ, and
 * keeps the CPU busy retrying. Specs share the instances, so each must leave nothing behind.
 */
export const test = base.extend<{ closeContexts: void }>({
  closeContexts: [
    async ({ browser }, use) => {
      const opened: Awaited<ReturnType<typeof browser.newContext>>[] = [];
      const newContext = browser.newContext.bind(browser);
      browser.newContext = async (options) => {
        const context = await newContext(options);
        opened.push(context);
        return context;
      };
      try {
        await use();
      } finally {
        browser.newContext = newContext;
        await Promise.all(opened.map((c) => c.close().catch(() => {})));
      }
    },
    { auto: true },
  ],
});
