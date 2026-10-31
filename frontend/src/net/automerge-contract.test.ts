// @vitest-environment node
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { expect, it } from "vitest";

const dist = join(
  dirname(createRequire(import.meta.url).resolve("@automerge/automerge-repo")),
  "..",
);
const source = (file: string) => readFileSync(join(dist, file), "utf8");
// Deliberate source tripwires for private, unobservable retention in pinned 2.5.6.
// A library update must re-audit these assumptions before changing the assertions.
it("1 contract: dedup keys include session, counts must increase, no pruning", () => {
  const s = source("network/NetworkSubsystem.js");
  // biome-ignore lint/suspicious/noTemplateCurlyInString: asserting the library source literally
  expect(s).toContain("`${message.senderId}:${message.sessionId}`");
  expect(s).toContain("msg.count > this.#ephemeralSessionCounts[source]");
  expect(s).not.toContain("delete this.#ephemeralSessionCounts");
});
it("1 contract: requests are retained before access (boundary must filter first)", () => {
  const s = source("synchronizer/CollectionSynchronizer.js");
  expect(s.indexOf("this.#hasRequested.set(documentId")).toBeLessThan(
    s.indexOf("await this.repo.shareConfig.access"),
  );
});
it("1 contract: ephemeral peers skip load/save; absent storageId skips sync info", () => {
  expect(source("synchronizer/CollectionSynchronizer.js")).toMatch(
    /if \(!storageId \|\| isEphemeral\) \{\s+return;/,
  );
  const s = source("Repo.js");
  expect(s).toMatch(/if \(!storageId \|\| isEphemeral\) \{\s+return;/);
  expect(s).toMatch(/if \(!storageId\) \{\s+return;/);
});
it("1 contract: loading retains pending sync; ephemeral messages relay", () => {
  const s = source("synchronizer/DocSynchronizer.js");
  expect(s).toContain("this.#pendingSyncMessages.push");
  expect(s).toContain('case "ephemeral"');
  expect(s).toContain('this.emit("message"');
});
it("1 contract: registration announces synchronously before import returns", () => {
  expect(source("Repo.js")).toContain("this.synchronizer.addDocument(handle)");
  const s = source("synchronizer/CollectionSynchronizer.js");
  expect(s).toContain("addDocument(handle)");
  expect(s).toContain("this.#documentGenerousPeers(handle.documentId)");
});
