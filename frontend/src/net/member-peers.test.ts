// @vitest-environment node
import type { PeerId } from "@automerge/automerge-repo";
import { encode } from "cbor-x";
import { expect, it, vi } from "vitest";
import { Admission } from "./admission";
import { encodeEnvelope, encodeRoles, Kind } from "./envelope";
import { type SocketLike, WailsNetworkAdapter } from "./WailsNetworkAdapter";

it("A14 admission allocates only members and prunes departed devices and cooldowns", () => {
  const a = new Admission(() => 0);
  for (let i = 0; i < 2000; i++) expect(a.arrive(`stranger${i}`, {})).toBe(false);
  a.setMembers(["member"]);
  expect(a.arrive("member", {})).toBe(true);
  for (let i = 0; i < 5; i++) a.hold("member", { documentId: "doc" } as never);
  expect(a.blocked("member")).toBe(true);
  a.setMembers([]);
  expect(a.retryAfter("member")).toBe(0);
  expect(a.held("member")).toBe(0);
  a.setMembers(["member"]);
  expect(a.arrive("member", {})).toBe(true);
});

it.each(["removed", "suspended", "self-suspended", "down"])(
  "A14 %s disconnects Repo and cancels pending and held work",
  (reason) => {
    vi.useFakeTimers();
    let ready = false;
    const socket: SocketLike = {
      binaryType: "",
      send: vi.fn(),
      close: vi.fn(),
      onopen: null,
      onclose: null,
      onmessage: null,
    };
    const adapter = new WailsNetworkAdapter(() => socket, {
      admitted: () => true,
      ready: () => ready,
    });
    const up = vi.fn(),
      down = vi.fn(),
      message = vi.fn();
    adapter.on("peer-candidate", up);
    adapter.on("peer-disconnected", down);
    adapter.on("message", message);
    adapter.connect("me" as PeerId);
    const frame = (kind: Kind, peer: string, payload: Uint8Array = new Uint8Array()) => {
      const b = encodeEnvelope({ kind, peer, payload });
      socket.onmessage?.(new MessageEvent("message", { data: b.buffer }));
    };
    const roles = (suspended = false, self = false, removed = false) =>
      frame(
        Kind.Roles,
        "me",
        encodeRoles([
          { peer: "me", role: "admin", suspended: self },
          ...(removed ? [] : [{ peer: "member", role: "reader" as const, suspended }]),
        ]),
      );
    for (let i = 0; i < 2000; i++)
      frame(Kind.Message, `s${i}`, encode({ type: "arrive", senderId: `s${i}` }));
    expect(up).not.toHaveBeenCalled();
    roles();
    const arrive = () =>
      frame(Kind.Message, "member", encode({ type: "arrive", senderId: "member" }));
    arrive();
    expect(up).toHaveBeenCalledTimes(1);
    frame(
      Kind.Message,
      "member",
      encode({ type: "doc-unavailable", senderId: "member", targetId: "me", documentId: "doc" }),
    );
    for (let i = 0; i < 4; i++) arrive(); // one deferred handshake
    down.mockClear();
    if (reason === "down") frame(Kind.PeerDown, "member");
    else roles(reason === "suspended", reason === "self-suspended", reason === "removed");
    expect(down).toHaveBeenCalledTimes(1);
    const before = up.mock.calls.length;
    ready = true;
    adapter.drain();
    vi.advanceTimersByTime(600000);
    expect(up).toHaveBeenCalledTimes(before);
    expect(message).not.toHaveBeenCalled();
    adapter.disconnect();
    vi.useRealTimers();
  },
);
