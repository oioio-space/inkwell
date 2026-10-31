// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { type PeerId, Repo } from "@automerge/automerge-repo";
import { expect, it } from "vitest";
import { BootstrapLoader } from "../workspace/loader";
import { AdmittedSet } from "./scope";
import { shareAdmitted } from "./sharing";
import { SimNet } from "./simnet";
import { WailsNetworkAdapter } from "./WailsNetworkAdapter";

it.each([false, true])(
  "8 R1 three repos, loader=%s: new imports survive the creator leaving",
  async (pull) => {
    const net = new SimNet(6);
    const peers = ["a", "b", "c"].map((id) => {
      const set = new AdmittedSet();
      const adapter = new WailsNetworkAdapter(() => net.socket(id), { admitted: set.has });
      const repo = new Repo({
        peerId: id as PeerId,
        network: [adapter],
        isEphemeral: true,
        shareConfig: shareAdmitted(set),
      });
      const loader = new BootstrapLoader(repo, set);
      set.onChange(() => {
        repo.shareConfigChanged();
        if (pull) for (const id of set.ids) loader.ensure(id);
      });
      return { repo, set };
    });
    try {
      const [a, b, c] = peers;
      if (!a || !b || !c) throw Error();
      const root = a.repo.create({ text: "root" });
      for (const p of peers) p.set.setRoot(root.url);
      await new Promise((r) => setTimeout(r, 0));
      net.link("a", "b");
      net.link("a", "c");
      net.link("b", "c");
      await net.settle();
      const variant = c.repo.import<{ text: string }>(
        A.save(A.from({ text: "variant from Carol" })),
      );
      // import happens before the listing change; shareConfigChanged must re-announce it.
      for (const p of peers) p.set.refresh({ meta: { variants: [{ url: variant.url }] } });
      await expect
        .poll(() => b.repo.handles[variant.documentId]?.doc()?.text)
        .toBe("variant from Carol");
      net.partition("b", "c");
      net.partition("a", "c");
      expect((await b.repo.find<{ text: string }>(variant.url)).doc().text).toBe(
        "variant from Carol",
      );
    } finally {
      await Promise.all(peers.map((p) => p.repo.shutdown()));
    }
  },
);

it("R1 root listing changes admit and bootstrap variants without opening them", async () => {
  const net = new SimNet(6);
  const peers = ["a", "b", "c"].map((id) => {
    const set = new AdmittedSet();
    let repo: Repo;
    let loader: BootstrapLoader;
    const adapter = new WailsNetworkAdapter(() => net.socket(id), {
      admitted: set.has,
      ready: (id) =>
        !!repo.handles[id as import("@automerge/automerge-repo").DocumentId]?.inState([
          "ready",
          "requesting",
          "unavailable",
        ]),
      load: (id) => loader.ensure(id),
    });
    repo = new Repo({
      peerId: id as PeerId,
      network: [adapter],
      isEphemeral: true,
      shareConfig: shareAdmitted(set),
    });
    loader = new BootstrapLoader(repo, set, () => adapter.drain());
    set.onChange(() => {
      repo.shareConfigChanged();
      for (const id of set.ids) loader.ensure(id);
    });
    return { repo, set };
  });
  try {
    const [a, b, c] = peers;
    if (!a || !b || !c) throw Error();
    type Root = {
      text: string;
      meta: { variants: { url: import("@automerge/automerge-repo").AutomergeUrl }[] };
    };
    const root = a.repo.create<Root>({ text: "root", meta: { variants: [] } });
    for (const p of peers) p.set.setRoot(root.url);
    await new Promise((r) => setTimeout(r, 0));
    net.link("a", "b");
    net.link("a", "c");
    net.link("b", "c");
    await net.settle();
    for (const p of peers) {
      const handle = await p.repo.find<Root>(root.url);
      const refresh = () => p.set.refresh(handle.doc());
      refresh();
      handle.on("change", refresh);
    }
    const variant = c.repo.import<{ text: string }>(A.save(A.from({ text: "Carol's variant" })));
    (await c.repo.find<Root>(root.url)).change((doc) => {
      doc.meta.variants.push({ url: variant.url });
    });
    await expect
      .poll(() => b.repo.handles[variant.documentId]?.doc()?.text)
      .toBe("Carol's variant");
    net.partition("a", "c");
    net.partition("b", "c");
    expect((await b.repo.find<{ text: string }>(variant.url)).heads()).toEqual(variant.heads());
  } finally {
    await Promise.all(peers.map((p) => p.repo.shutdown()));
  }
});
