import { describe, expect, it, vi } from "vitest";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import { drainAfterPaint } from "../ui/afterPaint";
import { LanguageClient, reconnectDelay } from "./client";

function setup(
  capabilities: Record<string, unknown> = {
    textDocumentSync: { openClose: true, change: 2 },
    completionProvider: {},
    signatureHelpProvider: {},
    hoverProvider: true,
  },
) {
  const sent: Record<string, unknown>[] = [];
  let receive: SocketLike["onmessage"] = null;
  const socket: SocketLike = {
    binaryType: "",
    send: (bytes) => sent.push(JSON.parse(new TextDecoder().decode(bytes))),
    close() {},
    onopen: null,
    onclose: null,
    // The client handles a message after the next paint; the test plays that paint at once.
    get onmessage() {
      return receive && ((e: MessageEvent) => (receive?.(e), drainAfterPaint()));
    },
    set onmessage(handler) {
      receive = handler;
    },
  };
  const client = new LanguageClient(
    () => socket,
    () => {},
  );
  client.start();
  socket.onopen?.(new Event("open"));
  socket.onmessage?.(
    new MessageEvent("message", {
      data: JSON.stringify({ jsonrpc: "2.0", id: 1, result: { capabilities } }),
    }),
  );
  client.open("main.typ", "#text()");
  return { client, socket, sent };
}
it("LSP-5 keeps syntax-only while any open document exceeds Grand", () => {
  const { client, sent } = setup();
  client.change("main.typ", "x".repeat(2 * 1024 * 1024 + 1));
  client.open("small.typ", "#let x=1");
  const configurations = sent.filter((m) => m.method === "workspace/didChangeConfiguration");
  expect(configurations.at(-1)?.params).toEqual({ settings: { syntaxOnly: "enable" } });
  client.stop();
});
describe("LSP-STALE", () => {
  it("discards replies after local or remote document changes", async () => {
    const { client, socket, sent } = setup();
    const result = client.request("textDocument/hover", "main.typ", { line: 0, character: 2 });
    const id = sent.at(-1)?.id;
    client.change("main.typ", "#text(1)");
    socket.onmessage?.(
      new MessageEvent("message", { data: JSON.stringify({ id, result: { contents: "stale" } }) }),
    );
    expect(await result).toBeUndefined();
    client.stop();
  });
  it("discards replies when Automerge heads change even if text is identical", async () => {
    const { client, socket, sent } = setup();
    client.change("main.typ", "#text()", "heads-A");
    const result = client.request("textDocument/hover", "main.typ", { line: 0, character: 2 });
    const id = sent.at(-1)?.id;
    client.change("main.typ", "#text()", "heads-B");
    socket.onmessage?.(
      new MessageEvent("message", { data: JSON.stringify({ id, result: { contents: "stale" } }) }),
    );
    expect(await result).toBeUndefined();
    client.stop();
  });
  it("discards a main reply when an imported open document changes", async () => {
    const { client, socket, sent } = setup();
    client.open("chart.typ", "#let name=1");
    const result = client.request("textDocument/hover", "main.typ", { line: 0, character: 2 });
    const id = sent.at(-1)?.id;
    client.change("chart.typ", "#let name=2");
    socket.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify({ id, result: { contents: "stale imported name" } }),
      }),
    );
    expect(await result).toBeUndefined();
    client.stop();
  });
  it("discards responses from an old server generation", async () => {
    const { client, socket, sent } = setup();
    const result = client.request("textDocument/completion", "main.typ", { line: 0, character: 2 });
    const id = sent.at(-1)?.id;
    client.stop();
    socket.onmessage?.(
      new MessageEvent("message", { data: JSON.stringify({ id, result: [{ label: "unsafe" }] }) }),
    );
    expect(await result).toBeUndefined();
  });
  it("keeps a slow response when its document and workspace are still current", async () => {
    vi.useFakeTimers();
    const { client, socket, sent } = setup();
    const result = client.request("textDocument/signatureHelp", "main.typ", {
      line: 0,
      character: 6,
    });
    const id = sent.at(-1)?.id;
    await vi.advanceTimersByTimeAsync(150);
    socket.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify({ id, result: { signatures: [{ label: "text(body)" }] } }),
      }),
    );
    expect(await result).toEqual({ signatures: [{ label: "text(body)" }] });
    client.stop();
    vi.useRealTimers();
  });
});

describe("capability-negotiated changes and cancellation", () => {
  it("sends a minimal UTF-16 edit for the advertised incremental sync mode", () => {
    const { client, sent } = setup();
    client.change("main.typ", "#text(😀)");
    expect(sent.at(-1)).toMatchObject({
      method: "textDocument/didChange",
      params: {
        textDocument: { version: 2 },
        contentChanges: [
          {
            range: { start: { line: 0, character: 6 }, end: { line: 0, character: 6 } },
            text: "😀",
          },
        ],
      },
    });
    client.stop();
  });
  it("uses full text only for a server advertising full document sync", () => {
    const { client, sent } = setup({ textDocumentSync: 1, hoverProvider: true });
    client.change("main.typ", "#text(1)");
    expect(sent.at(-1)).toMatchObject({
      method: "textDocument/didChange",
      params: { contentChanges: [{ text: "#text(1)" }] },
    });
    client.stop();
  });
  it("does not request features absent from the server capabilities", async () => {
    const { client, sent } = setup({ textDocumentSync: 2 });
    const count = sent.length;
    expect(
      await client.request("textDocument/hover", "main.typ", { line: 0, character: 2 }),
    ).toBeUndefined();
    expect(sent).toHaveLength(count);
    client.stop();
  });
  it("cancels work superseded by a document change", async () => {
    const { client, sent } = setup();
    const pending = client.request("textDocument/hover", "main.typ", { line: 0, character: 2 });
    const id = sent.at(-1)?.id;
    client.change("main.typ", "#text(1)");
    expect(sent).toContainEqual({ jsonrpc: "2.0", method: "$/cancelRequest", params: { id } });
    expect(await pending).toBeUndefined();
    client.stop();
  });
  it("cancels an older request replaced by another of the same feature", async () => {
    const { client, sent, socket } = setup();
    const old = client.request("textDocument/hover", "main.typ", { line: 0, character: 2 });
    const oldId = sent.at(-1)?.id;
    const fresh = client.request("textDocument/hover", "main.typ", { line: 0, character: 3 });
    const freshId = sent.at(-1)?.id;
    expect(sent).toContainEqual({
      jsonrpc: "2.0",
      method: "$/cancelRequest",
      params: { id: oldId },
    });
    expect(await old).toBeUndefined();
    socket.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify({ id: freshId, result: { contents: "current" } }),
      }),
    );
    expect(await fresh).toEqual({ contents: "current" });
    client.stop();
  });
});
describe("reconnecting to the language service", () => {
  it("backs off exponentially from 1 s, with jitter, capped at 30 s", () => {
    for (let n = 0; n < 12; n++) {
      const ceiling = Math.min(30_000, 1000 * 2 ** n);
      expect(reconnectDelay(n, () => 0)).toBe(ceiling / 2);
      expect(reconnectDelay(n, () => 0.5)).toBe(ceiling * 0.75);
      expect(reconnectDelay(n, () => 0.999_999)).toBeLessThanOrEqual(ceiling);
    }
    expect(reconnectDelay(10_000, () => 0.999_999)).toBeLessThanOrEqual(30_000);
  });
  it("waits longer after each failed connection, and starts over once one works", () => {
    vi.useFakeTimers();
    vi.spyOn(Math, "random").mockReturnValue(0.999_999);
    const sockets: SocketLike[] = [];
    const client = new LanguageClient(
      () => {
        const socket: SocketLike = {
          binaryType: "",
          send() {},
          close() {},
          onopen: null,
          onclose: null,
          onmessage: null,
        };
        sockets.push(socket);
        return socket;
      },
      () => {},
    );
    client.start();
    const failAndWait = () => {
      sockets.at(-1)?.onclose?.(new Event("close"));
      const before = sockets.length;
      let waited = 0;
      while (sockets.length === before && waited < 60_000) {
        vi.advanceTimersByTime(100);
        waited += 100;
      }
      return waited;
    };
    const waits = Array.from({ length: 7 }, failAndWait);
    expect(waits).toEqual([1000, 2000, 4000, 8000, 16_000, 30_000, 30_000]);
    const socket = sockets.at(-1);
    socket?.onopen?.(new Event("open"));
    socket?.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify({ jsonrpc: "2.0", id: 1, result: { capabilities: {} } }),
      }),
    );
    drainAfterPaint();
    expect(client.ready).toBe(true);
    expect(failAndWait()).toBe(1000);
    client.stop();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });
});
