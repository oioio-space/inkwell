import type { APIResponse, Page } from "@playwright/test";

// The page reaches Go's streams through the Wails poll transport (@wailsio/runtime, stream.js):
// each stream is opened by a POST to /wails/stream/send that names it and gives its connection
// number, and one long GET to /wails/stream/poll per page brings the frames of every stream:
// « WS1\0 », a flag byte, a count (u32, big endian), then per frame the connection (u32), a kind
// (u8, 0 = data) and a length (u32) before the bytes.
const HEADER = 9;
const SESSION = "x-wails-stream-session";

/** A hand on one of the page's streams: what `send` is given reaches the page as if Go had sent it. */
export interface BridgeTap {
  send(frame: Uint8Array): void;
}

/**
 * Sits between `page` and Go on the stream `name`, for every page loaded after the call. Frames
 * given to `send` ride the next poll answer, or their own if Go has nothing to say; Go's own
 * frames are never dropped (an answer that was overtaken is given to the page's next poll).
 */
export async function tapBridge(page: Page, name: string): Promise<BridgeTap> {
  let conn: number | undefined;
  let queue: Buffer[] = [];
  let wake: () => void = () => {};
  /** Go's answer to a poll the page no longer waits on, kept for its next poll of the same session. */
  let carried: { session: string | undefined; answer: Promise<APIResponse> } | undefined;

  await page.route("**/wails/stream/send", (route) => {
    const h = route.request().headers();
    if (h["x-wails-stream-name"] === name) {
      conn = Number(h["x-wails-stream-conn"]);
      queue = []; // a new stream: what was queued was for the old one
    }
    return route.continue();
  });

  const framed = (frames: Buffer[], base?: Buffer): Buffer => {
    const head = base
      ? Buffer.from(base.subarray(0, HEADER))
      : Buffer.from("WS1\0\0\0\0\0\0", "latin1");
    head.writeUInt32BE((base ? base.readUInt32BE(5) : 0) + frames.length, 5);
    const parts = [head, ...(base ? [base.subarray(HEADER)] : [])];
    for (const f of frames) {
      const h = Buffer.alloc(HEADER);
      h.writeUInt32BE(conn ?? 0, 0);
      h.writeUInt32BE(f.length, 5);
      parts.push(h, f);
    }
    return Buffer.concat(parts);
  };

  await page.route("**/wails/stream/poll", async (route) => {
    const session = route.request().headers()[SESSION];
    const answer = carried?.session === session ? carried.answer : route.fetch({ timeout: 0 });
    carried = undefined;
    const ready = () => queue.length > 0 && conn !== undefined;
    const queued = new Promise<"queued">((resolve) => {
      wake = () => ready() && resolve("queued");
      wake();
    });
    try {
      const first = await Promise.race([answer, queued]);
      if (first === "queued") {
        carried = { session, answer };
        answer.catch(() => {});
        const body = framed(queue.splice(0));
        return await route.fulfill({ status: 200, contentType: "application/octet-stream", body });
      }
      if (first.status() !== 200 || !ready()) return await route.fulfill({ response: first });
      const body = framed(queue.splice(0), await first.body());
      return await route.fulfill({ response: first, body });
    } catch {
      // Go is between two process images, or the page is gone: the page's poll fails and retries.
      await route.abort().catch(() => {});
    }
  });

  return {
    send(frame) {
      queue.push(Buffer.from(frame));
      wake();
    },
  };
}
