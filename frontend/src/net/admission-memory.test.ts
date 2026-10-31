// @vitest-environment node
import { encode } from "cbor-x";
import { expect, it } from "vitest";
import { Admission, parsePeerMessage } from "./admission";

const gc = (globalThis as { gc?: () => void }).gc;
const heap = () => {
  gc?.();
  return process.memoryUsage().heapUsed;
};
it.skipIf(!gc)("3 invented-id flood retains less than 4 MiB after GC", () => {
  const c = { from: "p", me: "me", announced: true, has: () => false };
  const before = heap();
  let accepted = 0;
  for (let i = 0; i < 50000; i++)
    if (
      parsePeerMessage(
        encode({
          type: "request",
          senderId: "p",
          targetId: "me",
          documentId: `invented${i}`,
          data: new Uint8Array(),
        }),
        c,
      )
    )
      accepted++;
  expect(accepted).toBe(0);
  expect(heap() - before).toBeLessThan(4 * 1024 * 1024);
});
it.skipIf(!gc)("5 session churn retains less than 2 MiB after GC", () => {
  const a = new Admission(() => 0);
  a.setMembers(["p"]);
  const sessions = new Set<string>();
  const before = heap();
  let accepted = 0;
  for (let i = 0; i < 10000; i++) {
    const m = a.ephemeral("p", {
      type: "ephemeral",
      sessionId: i.toString(36),
      count: 1,
    } as never) as unknown as { sessionId: string } | undefined;
    if (m) {
      sessions.add(m.sessionId);
      accepted++;
    }
  }
  expect(sessions.size).toBe(1);
  expect(accepted).toBe(128);
  expect(heap() - before).toBeLessThan(2 * 1024 * 1024);
});
