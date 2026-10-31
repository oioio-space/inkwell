// @vitest-environment node
import { Repo } from "@automerge/automerge-repo";
import { expect, it, vi } from "vitest";
import { AdmittedSet } from "../net/scope";
import { BootstrapLoader } from "./loader";

it("7 loader caps concurrency, deduplicates and cancels stale epochs", async () => {
  const real = new Repo();
  const root = real.create({}).url;
  const set = new AdmittedSet();
  set.setRoot(root);
  const urls = Array.from({ length: 10 }, () => real.create({}).url);
  set.refresh({ meta: { variants: urls.map((url) => ({ url })) } });
  const resolvers: (() => void)[] = [];
  const find = vi.fn(() => new Promise<void>((r) => resolvers.push(r)));
  const done = vi.fn();
  const loader = new BootstrapLoader({ find } as never, set, done);
  for (const id of set.ids) {
    loader.ensure(id);
    loader.ensure(id);
  }
  expect(find).toHaveBeenCalledTimes(4);
  set.setRoot(real.create({}).url);
  for (const resolve of resolvers) resolve();
  await new Promise((r) => setTimeout(r, 0));
  expect(done).not.toHaveBeenCalled();
  expect(find).toHaveBeenCalledTimes(4);
});

it("7 real blocked storage: one find, flood resets once, no delivery after unblock", async () => {
  const { WailsNetworkAdapter } = await import("../net/WailsNetworkAdapter");
  const { encodeEnvelope, encodeRoles, Kind } = await import("../net/envelope");
  const { encode } = await import("cbor-x");
  const source = new Repo();
  const url = source.create({}).url;
  const set = new AdmittedSet();
  set.setRoot(url);
  const [id] = set.ids;
  if (!id) throw Error();
  let unblock: () => void = () => {};
  const blocked = new Promise<void>((r) => {
    unblock = r;
  });
  const socket: import("../net/WailsNetworkAdapter").SocketLike = {
    binaryType: "",
    send: () => {},
    close: () => {},
    onopen: null,
    onclose: null,
    onmessage: null,
  };
  let loader: BootstrapLoader;
  let repo: Repo;
  const adapter = new WailsNetworkAdapter(() => socket, {
    admitted: set.has,
    ready: (id) =>
      Object.values(repo.handles).some(
        (h) => h.documentId === id && h.inState(["ready", "requesting", "unavailable"]),
      ),
    load: (id) => loader.ensure(id),
  });
  repo = new Repo({
    peerId: "me" as import("@automerge/automerge-repo").PeerId,
    network: [adapter],
    storage: {
      load: async () => undefined,
      save: async () => {},
      remove: async () => {},
      loadRange: async () => {
        await blocked;
        return [];
      },
      removeRange: async () => {},
    },
    isEphemeral: true,
  });
  loader = new BootstrapLoader(repo, set, () => adapter.drain());
  const find = vi.spyOn(repo, "find");
  const down = vi.fn();
  adapter.on("peer-disconnected", down);
  const delivered = vi.fn();
  adapter.on("message", delivered);
  await new Promise((r) => setTimeout(r, 0));
  const deliver = (v: unknown) => {
    const frame = encodeEnvelope({ kind: Kind.Message, peer: "peer", payload: encode(v) });
    socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  };
  const roles = encodeEnvelope({
    kind: Kind.Roles,
    peer: "me",
    payload: encodeRoles([{ peer: "peer", role: "writer", suspended: false }]),
  });
  socket.onmessage?.(new MessageEvent("message", { data: roles.buffer }));
  deliver({ type: "arrive", senderId: "peer" });
  const frame = encodeEnvelope({
    kind: Kind.Message,
    peer: "peer",
    payload: encode({
      type: "sync",
      senderId: "peer",
      targetId: "me",
      documentId: id,
      data: new Uint8Array(65536),
    }),
  });
  for (let i = 0; i < 10000; i++)
    socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  expect(find).toHaveBeenCalledTimes(1);
  expect(down).toHaveBeenCalledTimes(1);
  unblock();
  await new Promise((r) => setTimeout(r, 10));
  expect(delivered).not.toHaveBeenCalled();
  // The library flush assumes ready handles; this test deliberately leaves one unavailable.
  adapter.disconnect();
  await repo.shutdown().catch((error: Error) => {
    expect(error.message).toBe("DocHandle is not ready");
  });
  await source.shutdown();
});

it("7 project switch removes stale FIFO entries before enqueueing current ids", async () => {
  const { generateAutomergeUrl } = await import("@automerge/automerge-repo");
  const set = new AdmittedSet();
  set.setRoot(generateAutomergeUrl());
  set.refresh({
    meta: { variants: Array.from({ length: 256 }, () => ({ url: generateAutomergeUrl() })) },
  });
  const resolvers: (() => void)[] = [];
  const find = vi.fn(() => new Promise<void>((resolve) => resolvers.push(resolve)));
  const loader = new BootstrapLoader({ find } as never, set);
  for (const id of set.ids) loader.ensure(id);
  set.setRoot(generateAutomergeUrl());
  set.refresh({
    meta: { variants: Array.from({ length: 256 }, () => ({ url: generateAutomergeUrl() })) },
  });
  for (const id of set.ids) loader.ensure(id);
  // Settle all work in waves, including jobs started by earlier completions.
  for (let wave = 0; wave < 70; wave++) {
    for (const resolve of resolvers.splice(0)) resolve();
    await new Promise((r) => setTimeout(r, 0));
  }
  expect(find).toHaveBeenCalledTimes(4 + 257);
});
