import { type PeerId, Repo } from "@automerge/automerge-repo";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, it, vi } from "vitest";
import { amSync, flushTyped } from "../editor/amSync";
import { activity } from "../ext/activity";
import { SimNet } from "../net/simnet";
import { WailsNetworkAdapter } from "../net/WailsNetworkAdapter";

const bridge = vi.hoisted(() => ({
  Descriptors: vi.fn(async () => []),
  Declare: vi.fn(async (_ds: object[]) => {}),
  Flush: vi.fn(async (_batch: object[]) => {}),
  Network: vi.fn(async () => ({ links: [] })),
}));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  StatsService: bridge,
}));

type Doc = { text: string };
const repos: Repo[] = [];
afterEach(async () => {
  await Promise.all(repos.splice(0).map((r) => r.shutdown()));
  vi.unstubAllGlobals();
});

function peer(net: SimNet, id: string) {
  const adapter = new WailsNetworkAdapter(() => net.socket(id), {
    reconnectDelayMs: 5,
    admitted: () => true,
  });
  const repo = new Repo({
    peerId: id as PeerId,
    network: [adapter],
    sharePolicy: async () => true,
  });
  repos.push(repo);
  const connected = new Set<string>();
  adapter.on("peer-candidate", ({ peerId }) => connected.add(peerId));
  return { repo, connected };
}

async function until(cond: () => boolean, ms = 5000): Promise<void> {
  const end = Date.now() + ms;
  while (!cond()) {
    if (Date.now() > end) throw new Error("timed out");
    await new Promise((r) => setTimeout(r, 5));
  }
}

it("OBS1 P-3 a webview session sends nothing but the person's edits", async () => {
  // Any network API a statistic could leave by is a trap.
  const traps = {
    fetch: vi.fn(),
    WebSocket: vi.fn(),
    XMLHttpRequest: vi.fn(),
    EventSource: vi.fn(),
    RTCPeerConnection: vi.fn(),
  };
  for (const [name, trap] of Object.entries(traps)) vi.stubGlobal(name, trap);
  const beacon = vi.fn();
  vi.stubGlobal("navigator", { ...navigator, sendBeacon: beacon });

  const net = new SimNet(1);
  const frames: Uint8Array[] = [];
  const route = net.route.bind(net);
  net.route = (from, to, payload) => {
    frames.push(payload.slice());
    route(from, to, payload);
  };
  const { repo: a, connected } = peer(net, "a");
  const { repo: b } = peer(net, "b");
  net.link("a", "b");
  await until(() => connected.has("b"));
  const handle = a.create<Doc>({ text: "" });
  const view = new EditorView({
    parent: document.body,
    doc: "",
    extensions: [amSync({ handle, path: ["text"] }), activity(handle.documentId)],
  });
  const typed = "Notre rapport final";
  for (const [i, ch] of [...typed].entries())
    view.dispatch({ changes: { from: i, insert: ch }, userEvent: "input.type" });
  flushTyped(handle);
  const there = await b.find<Doc>(handle.url);
  await until(() => there.doc()?.text === typed);
  // The window ends, and the page goes: the statistics are flushed, to Go only.
  view.destroy();
  window.dispatchEvent(new Event("pagehide"));
  await until(() => bridge.Flush.mock.calls.length > 0);
  await net.settle();

  const batch = bridge.Flush.mock.calls.flatMap(([batch]) => batch);
  expect(batch).toContainEqual({ m: "activity.words", doc: handle.documentId, v: 3 });
  // The repositories hold the person's text and nothing else: one document, one field.
  expect(Object.keys(a.handles)).toEqual([handle.documentId]);
  expect(Object.keys(b.handles)).toEqual([handle.documentId]);
  expect(Object.keys(there.doc() ?? {})).toEqual(["text"]);
  // On the wire: the person's edits, and no frame carries a metric, its label or the batch.
  expect(frames.length).toBeGreaterThan(0);
  const wire = frames.map((f) => new TextDecoder("latin1").decode(f)).join("\n");
  expect(wire).toContain("final");
  for (const leak of ["activity.", "metric.", '"m":', "words", "opened"])
    expect(wire).not.toContain(leak);
  for (const trap of [...Object.values(traps), beacon]) expect(trap).not.toHaveBeenCalled();
});
