// @vitest-environment node
import { type PeerId, Repo } from "@automerge/automerge-repo";
import { encode } from "cbor-x";
import { expect, it, vi } from "vitest";
import { encodeEnvelope, encodeRoles, Kind } from "./envelope";
import { type SocketLike, WailsNetworkAdapter } from "./WailsNetworkAdapter";

function setup(ready = () => true) {
  const socket: SocketLike = {
    binaryType: "",
    send: vi.fn(),
    close: vi.fn(),
    onopen: null,
    onclose: null,
    onmessage: null,
  };
  const load = vi.fn();
  const adapter = new WailsNetworkAdapter(() => socket, {
    admitted: (id) => id === "doc",
    ready,
    load,
  });
  adapter.connect("me" as PeerId);
  const deliver = (message: unknown) => {
    const frame = encodeEnvelope({ kind: Kind.Message, peer: "peer", payload: encode(message) });
    socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  };
  const rolesFrame = encodeEnvelope({
    kind: Kind.Roles,
    peer: "me",
    payload: encodeRoles([{ peer: "peer", role: "writer", suspended: false }]),
  });
  socket.onmessage?.(new MessageEvent("message", { data: rolesFrame.buffer }));
  const handshake = () =>
    deliver({
      type: "arrive",
      senderId: "peer",
      peerMetadata: { storageId: Math.random().toString() },
    });
  const messages: unknown[] = [];
  adapter.on("message", (m) => messages.push(m));
  const up = vi.fn();
  adapter.on("peer-candidate", up);
  const down = vi.fn();
  adapter.on("peer-disconnected", down);
  handshake();
  return { adapter, socket, load, deliver, handshake, messages, up, down };
}
it("3 boundary prevents invented ids from reaching Repo access or handles", async () => {
  const x = setup();
  const access = vi.fn(async () => true);
  const repo = new Repo({
    network: [x.adapter],
    shareConfig: { access, announce: async () => false },
  });
  await new Promise((r) => setTimeout(r, 0));
  for (let i = 0; i < 50000; i++)
    x.deliver({
      type: "request",
      senderId: "peer",
      targetId: "me",
      documentId: `invented${i}`,
      data: new Uint8Array(),
    });
  expect(access).not.toHaveBeenCalled();
  expect(Object.keys(repo.handles)).toHaveLength(0);
  await repo.shutdown();
});
it("5 filters presence before Repo, rewrites one session and drops outbound relay", () => {
  vi.useFakeTimers();
  const x = setup();
  for (let i = 0; i < 10000; i++)
    x.deliver({
      type: "ephemeral",
      senderId: "peer",
      targetId: "me",
      documentId: "doc",
      sessionId: i.toString(36),
      count: 1,
      data: encode({ __presence: { type: "heartbeat" } }),
    });
  expect(x.messages).toHaveLength(128);
  expect(new Set(x.messages.map((m) => (m as { sessionId: string }).sessionId)).size).toBe(1);
  const before = vi.mocked(x.socket.send).mock.calls.length;
  x.adapter.send({ type: "ephemeral", senderId: "peer", targetId: "other" } as never);
  expect(x.socket.send).toHaveBeenCalledTimes(before);
  x.adapter.disconnect();
  vi.useRealTimers();
});
it("6 overrides every storageId and throttles forty re-arrives", () => {
  vi.useFakeTimers();
  const x = setup();
  for (let i = 0; i < 40; i++) x.handshake();
  expect(x.up).toHaveBeenCalledTimes(3);
  for (const [event] of x.up.mock.calls) expect(event.peerMetadata).toEqual({ isEphemeral: true });
  vi.advanceTimersByTime(20000);
  expect(x.up).toHaveBeenCalledTimes(4);
  x.adapter.disconnect();
  vi.useRealTimers();
});
it("7 holds at most four, resets once, keeps cooldown across reconnect", () => {
  const x = setup(() => false);
  for (let i = 0; i < 10000; i++)
    x.deliver({ type: "doc-unavailable", senderId: "peer", targetId: "me", documentId: "doc" });
  expect(x.load).toHaveBeenCalledTimes(4);
  expect(x.down).toHaveBeenCalledTimes(1);
  expect(x.messages).toHaveLength(0);
  x.handshake();
  expect(x.up).toHaveBeenCalledTimes(1);
  x.adapter.disconnect();
});
it("7 PeerDown and epoch cancel held messages; requesting drains", () => {
  let ready = false;
  const x = setup(() => ready);
  const message = { type: "doc-unavailable", senderId: "peer", targetId: "me", documentId: "doc" };
  x.deliver(message);
  ready = true;
  x.adapter.drain();
  expect(x.messages).toHaveLength(1);
  ready = false;
  x.deliver(message);
  x.adapter.scopeChanged();
  ready = true;
  x.adapter.drain();
  expect(x.messages).toHaveLength(1);
  ready = false;
  x.deliver(message);
  const frame = encodeEnvelope({ kind: Kind.PeerDown, peer: "peer", payload: new Uint8Array() });
  x.socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  ready = true;
  x.adapter.drain();
  expect(x.messages).toHaveLength(1);
  x.adapter.disconnect();
});

it("5 duplicate welcomes cannot refill the ephemeral bucket", () => {
  vi.useFakeTimers();
  const x = setup();
  const message = {
    type: "ephemeral",
    senderId: "peer",
    targetId: "me",
    documentId: "doc",
    sessionId: "a",
    count: 1,
    data: encode({ __presence: { type: "heartbeat" } }),
  };
  for (let i = 1; i <= 128; i++) x.deliver({ ...message, count: i });
  x.deliver({ type: "welcome", senderId: "peer", peerMetadata: {} });
  x.deliver({ ...message, count: 129 });
  expect(x.messages).toHaveLength(128);
  x.adapter.disconnect();
  vi.useRealTimers();
});

it("7 reconnect handshake waits for reset cooldown", () => {
  vi.useFakeTimers();
  const x = setup(() => false);
  for (let i = 0; i < 5; i++)
    x.deliver({ type: "doc-unavailable", senderId: "peer", targetId: "me", documentId: "doc" });
  const before = vi.mocked(x.socket.send).mock.calls.length;
  const frame = encodeEnvelope({ kind: Kind.PeerUp, peer: "peer", payload: new Uint8Array() });
  x.socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  expect(x.socket.send).toHaveBeenCalledTimes(before);
  vi.advanceTimersByTime(30000);
  expect(x.socket.send).toHaveBeenCalledTimes(before + 1);
  x.adapter.disconnect();
  vi.useRealTimers();
});

it("presentation navigation survives session rewriting and shares the presence rate budget", () => {
  vi.useFakeTimers();
  const x = setup();
  try {
    const send = (count: number, message: unknown) =>
      x.deliver({
        type: "ephemeral",
        senderId: "peer",
        targetId: "me",
        documentId: "doc",
        sessionId: "presenter",
        count,
        data: encode(message),
      });
    send(1, { __present: { t: "take", epoch: 1 } });
    send(2, { __present: { t: "state", epoch: 1, seq: 1, slide: 2, step: 0, screen: "show" } });
    expect(x.messages).toHaveLength(2);
    send(2, { __present: { t: "hello" } }); // duplicate count
    expect(x.messages).toHaveLength(2);
    for (let i = 3; i <= 129; i++) send(i, { __presence: { type: "heartbeat" } });
    expect(x.messages).toHaveLength(128);
    expect(new Set(x.messages.map((m) => (m as { sessionId: string }).sessionId)).size).toBe(1);
    vi.advanceTimersByTime(1000);
    send(130, { __present: { t: "hello" } });
    expect(x.messages).toHaveLength(129);
  } finally {
    x.adapter.disconnect();
    vi.useRealTimers();
  }
});

it("drops loading-document cursor bursts without cooldown", () => {
  vi.useFakeTimers();
  let ready = false;
  const x = setup(() => ready);
  try {
    for (let count = 1; count <= 1000; count++)
      x.deliver({
        type: "ephemeral",
        senderId: "peer",
        targetId: "me",
        documentId: "doc",
        sessionId: "cursor",
        count,
        data: encode({ __presence: { type: "heartbeat" } }),
      });
    expect(x.down).not.toHaveBeenCalled();
    expect(x.load).not.toHaveBeenCalled();
    ready = true;
    x.deliver({ type: "doc-unavailable", senderId: "peer", targetId: "me", documentId: "doc" });
    expect(x.messages).toHaveLength(1);
  } finally {
    x.adapter.disconnect();
    vi.useRealTimers();
  }
});

it("PeerDown reconnect loops cannot refill admission budgets", () => {
  vi.useFakeTimers();
  const x = setup();
  try {
    const presence = (count: number) =>
      x.deliver({
        type: "ephemeral",
        senderId: "peer",
        targetId: "me",
        documentId: "doc",
        sessionId: "cursor",
        count,
        data: encode({ __presence: { type: "heartbeat" } }),
      });
    for (let i = 1; i <= 128; i++) presence(i);
    for (let i = 0; i < 10; i++) {
      const frame = encodeEnvelope({
        kind: Kind.PeerDown,
        peer: "peer",
        payload: new Uint8Array(),
      });
      x.socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
      x.handshake();
      presence(129 + i);
    }
    expect(x.up).toHaveBeenCalledTimes(3);
    expect(x.messages).toHaveLength(128);
  } finally {
    x.adapter.disconnect();
    vi.useRealTimers();
  }
});
