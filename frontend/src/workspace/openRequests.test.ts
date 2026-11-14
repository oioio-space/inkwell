import { describe, expect, it, vi } from "vitest";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import {
  type Bounds,
  MAX_PENDING,
  noticeText,
  OpenRequests,
  type OpenTarget,
  parseOpen,
} from "./openRequests";

/** A stream end the test writes frames into, as Go would. */
class FakeSocket implements SocketLike {
  binaryType = "";
  onopen: ((ev: Event) => unknown) | null = null;
  onclose: ((ev: Event) => unknown) | null = null;
  onmessage: ((ev: MessageEvent) => unknown) | null = null;
  closed = false;
  sent: unknown[] = [];
  send(data: Uint8Array): void {
    this.sent.push(JSON.parse(new TextDecoder().decode(data)));
  }
  close(): void {
    this.closed = true;
  }
  frame(v: unknown): void {
    this.onmessage?.(
      new MessageEvent("message", { data: new TextEncoder().encode(JSON.stringify(v)).buffer }),
    );
  }
}

function setup(ready: Promise<unknown> = Promise.resolve(), bounds?: Bounds) {
  const socket = new FakeSocket();
  const target: OpenTarget & { shown: string[]; toasts: string[]; roles: string[] } = {
    ready,
    shown: [],
    toasts: [],
    roles: [],
    prepare() {},
    show(path) {
      this.shown.push(path);
      return true;
    },
    toast(text, role) {
      this.toasts.push(text);
      this.roles.push(role);
    },
  };
  const applied: number[] = [];
  const requests = new OpenRequests(
    () => socket,
    target,
    10,
    (_, n) => applied.push(n),
    bounds,
    10,
  );
  requests.start();
  return { socket, target, requests, applied };
}

describe("open requests (W41, block OI)", () => {
  it("OI open request shows a file of the current project", async () => {
    const { socket, target, requests } = setup();
    socket.frame({ show: "chapitres/intro.typ" });
    await requests.idle();
    expect(target.shown).toEqual(["chapitres/intro.typ"]);
    expect(target.toasts).toEqual([]);
    expect(socket.sent).toEqual([]); // the launch has ended: nothing to answer
    requests.stop();
    expect(socket.closed).toBe(true);
  });

  it("OI open request with no file only raises", async () => {
    const { socket, target, requests } = setup();
    socket.frame({});
    await requests.idle();
    expect(target.shown).toEqual([]);
    expect(target.toasts).toEqual([]);
  });

  it("OI open request shows one toast with the catalogue text", async () => {
    const { socket, target, requests } = setup();
    socket.frame({ notices: [{ kind: "loose", name: "notes.md" }] });
    socket.frame({ show: "main.typ", notices: [{ kind: "otherProject", name: "rapport.typ" }] });
    await requests.idle();
    expect(target.toasts).toEqual([
      "« notes.md » n'est dans aucun projet. Inkwell ne sait pas encore ouvrir un fichier seul.",
      "« rapport.typ » fait partie d'un autre projet. Inkwell n'ouvre qu'un projet à la fois pour l'instant.",
    ]);
    expect(target.shown).toEqual(["main.typ"]);
  });

  it("OI open request before the workspace is ready waits", async () => {
    let ready = () => {};
    const { socket, target, requests } = setup(new Promise<void>((r) => (ready = r)));
    socket.frame({ show: "main.typ" });
    socket.frame({ notices: [{ kind: "folder", name: "Téléchargements" }] });
    await new Promise((r) => setTimeout(r, 20));
    expect(target.shown).toEqual([]);
    expect(target.toasts).toEqual([]);
    ready();
    await requests.idle();
    expect(target.shown).toEqual(["main.typ"]);
    expect(target.toasts).toEqual(["« Téléchargements » n'est pas un projet Inkwell."]);
  });

  it("says in a toast that a file could not be shown, and goes on with the next", async () => {
    const { socket, target, requests } = setup();
    target.show = (path) => path !== "chapitres/.cache.md"; // not in the project's list
    socket.frame({ show: "chapitres/.cache.md", notices: [{ kind: "loose", name: "x.md" }] });
    target.prepare = () => Promise.reject(new Error("store unavailable"));
    socket.frame({ show: "b.typ" });
    await requests.idle();
    expect(target.toasts).toEqual([
      "« .cache.md » n'a pas pu s'ouvrir, « b.typ » n'a pas pu s'ouvrir.",
      "« x.md » n'est dans aucun projet. Inkwell ne sait pas encore ouvrir un fichier seul.",
    ]);
    expect(target.roles).toEqual(["alert", "status"]);
  });

  it("writes every notice of the decision table from the catalogue", () => {
    const n = (kind: string, extra: object = {}) =>
      noticeText({ kind, name: "x.typ", count: 0, ...extra } as never);
    expect(n("show")).toBe(
      "« x.typ » n'a pas été ouvert : Inkwell affiche un document à la fois pour l'instant.",
    );
    expect(n("refused", { reason: "missing" })).toBe("« x.typ » est introuvable.");
    expect(n("refused", { reason: "unreadable" })).toBe(
      "Inkwell n'a pas le droit de lire « x.typ ».",
    );
    expect(n("refused", { reason: "special" })).toBe("« x.typ » n'est pas un fichier ordinaire.");
    expect(n("refused", { reason: "internal" })).toBe(
      "« x.typ » est un fichier interne d'Inkwell.",
    );
    expect(n("refused", { reason: "tooLong" })).toBe("Ce chemin est trop long.");
    expect(n("refused", { reason: "badName" })).toBe(
      "Inkwell ne sait pas lire le nom de « x.typ ».",
    );
    expect(n("refused", { reason: "tooMany", count: 8 })).toBe(
      "8 fichiers non ouverts : 32 au plus à la fois.",
    );
    expect(n("refused", { reason: "tooMany", count: 1 })).toBe(
      "1 fichier non ouvert : 32 au plus à la fois.",
    );
  });

  it("drops a frame that is not an open request, and reconnects when the stream drops", async () => {
    for (const bad of [
      "not json",
      "[]",
      '{"show":""}',
      '{"show":3}',
      '{"notices":{"kind":"loose","name":"x"}}',
      '{"notices":[{"kind":"nope","name":"x"}]}',
      '{"notices":[{"kind":"loose","name":"x"},{"kind":"refused","reason":"nope","name":"x"}]}',
      '{"notices":[{"kind":"loose","reason":"missing","name":"x"}]}',
      '{"notices":[{"kind":"toString","name":"x"}]}',
      '{"notices":[{"kind":"refused","reason":"constructor","name":"x"}]}',
      '{"notices":[{"kind":"loose","name":"x","count":1.5}]}',
      `{"notices":[${Array(65).fill('{"kind":"loose","name":"x"}').join(",")}]}`,
      `{"show":"${"a".repeat(5000)}"}`,
      '{"id":-1}',
      '{"id":1.5}',
    ]) {
      expect(parseOpen(bad), bad).toBeUndefined();
    }
    expect(parseOpen('{"notices":[{"kind":"refused","reason":"tooMany","count":3}]}')).toEqual({
      id: 0,
      notices: [{ kind: "refused", reason: "tooMany", name: "", count: 3 }],
    });
    const sockets: FakeSocket[] = [];
    const target = {
      ready: Promise.resolve(),
      prepare: vi.fn(),
      show: vi.fn(() => true),
      toast: vi.fn(),
    };
    const requests = new OpenRequests(
      () => sockets[sockets.push(new FakeSocket()) - 1] as FakeSocket,
      target,
      1,
    );
    requests.start();
    sockets[0]?.onclose?.(new Event("close"));
    await new Promise((r) => setTimeout(r, 20));
    expect(sockets.length).toBe(2);
    sockets[1]?.frame({ show: "b.typ" });
    await requests.idle();
    expect(target.show).toHaveBeenCalledWith("b.typ", expect.any(AbortSignal));
    requests.stop();
  });

  it("keeps at most MAX_PENDING requests waiting and says in one toast how many were left", async () => {
    let release = () => {};
    const gate = new Promise<void>((r) => (release = r));
    const { socket, target, requests, applied } = setup();
    target.prepare = () => gate;
    for (let i = 0; i < MAX_PENDING + 3; i++) socket.frame({ show: `f${i}.typ` });
    await new Promise((r) => setTimeout(r, 20));
    expect(target.shown).toEqual([]);
    release();
    await requests.idle();
    expect(target.shown).toEqual(Array.from({ length: MAX_PENDING }, (_, i) => `f${i}.typ`));
    expect(applied).toEqual(Array.from({ length: MAX_PENDING }, (_, i) => i + 1));
    expect(target.toasts).toEqual([
      "8 fichiers ouverts : « f0.typ », « f1.typ », « f2.typ » et 5 autres.",
      "3 demandes d'ouverture laissées : trop à la fois.",
    ]);
  });

  it("reports each request handled once its file is shown or its toast is up", async () => {
    const { socket, target, requests } = setup();
    target.show = (path) => path === "a.typ";
    socket.frame({ id: 1, show: "a.typ" });
    socket.frame({ id: 2, notices: [{ kind: "folder", name: "Musique" }] });
    socket.frame({ id: 3, show: ".cache.md" });
    await requests.idle();
    expect(socket.sent).toEqual([{ handled: 1 }, { handled: 2 }, { handled: 3 }]);
    expect(target.toasts).toEqual([
      "Inkwell n'a pas pu ouvrir « .cache.md ».",
      "« Musique » n'est pas un projet Inkwell.",
    ]);
  });

  it("gives each request its toast when the workspace failed to open, and reports it handled", async () => {
    const { socket, target, requests } = setup(Promise.reject(new Error("project unreadable")));
    socket.frame({ id: 1, show: "a.typ" });
    socket.frame({ id: 2, show: "b.typ", notices: [{ kind: "loose", name: "c.md" }] });
    await requests.idle();
    expect(target.shown).toEqual([]);
    expect(target.toasts).toEqual([
      "« a.typ » n'a pas pu s'ouvrir, « b.typ » n'a pas pu s'ouvrir.",
      "« c.md » n'est dans aucun projet. Inkwell ne sait pas encore ouvrir un fichier seul.",
    ]);
    expect(socket.sent).toEqual([{ handled: 1 }, { handled: 2 }]);
  });

  it("says a step past its bound is slow and keeps its slot until it ends: nothing else starts", async () => {
    const { socket, target } = setup(Promise.resolve(), { readyMs: 50, stepMs: 30 });
    let prepared = 0;
    target.prepare = () => {
      prepared++;
      return new Promise<void>(() => {}); // never ends: a window that stops answering
    };
    for (let id = 1; id <= 3; id++) socket.frame({ id, show: `f${id}.typ` });
    await new Promise((r) => setTimeout(r, 90));
    expect(target.toasts).toEqual(["L'ouverture de « f1.typ » prend plus de temps que prévu."]);
    expect(socket.sent).toEqual([]); // the slot stays taken: Go keeps the request
    expect(prepared).toBe(1);
    expect(target.shown).toEqual([]);
  });

  it("starts afresh after a reconnection: what Go sends again is applied, every slot freed", async () => {
    // One owner at a time (rule D-OWNER): the old stream's requests are discarded, Go sends them again.
    const sockets: FakeSocket[] = [];
    let release = () => {};
    const gate = new Promise<void>((r) => (release = r));
    const target = {
      ready: Promise.resolve(),
      shown: [] as string[],
      toasts: [] as string[],
      prepare: (): Promise<void> => gate,
      show(path: string) {
        this.shown.push(path);
        return true;
      },
      toast(text: string) {
        this.toasts.push(text);
      },
    };
    const requests = new OpenRequests(
      () => sockets[sockets.push(new FakeSocket()) - 1] as FakeSocket,
      target,
      1,
      () => {},
      undefined,
      10,
    );
    requests.start();
    const [first] = sockets as [FakeSocket];
    for (let id = 1; id <= MAX_PENDING; id++) first.frame({ id, show: `f${id}.typ` });
    first.onclose?.(new Event("close")); // superseded with all eight pending
    await new Promise((r) => setTimeout(r, 20));
    const second = sockets[1] as FakeSocket;
    for (let id = 1; id <= MAX_PENDING; id++) second.frame({ id, show: `f${id}.typ` });
    release();
    await requests.idle();
    expect(first.sent).toEqual([]);
    expect(second.sent).toEqual(
      Array.from({ length: MAX_PENDING }, (_, i) => ({ handled: i + 1 })),
    );
    expect(target.shown).toEqual(Array.from({ length: MAX_PENDING }, (_, i) => `f${i + 1}.typ`));
    expect(target.toasts).toEqual([
      "8 fichiers ouverts : « f1.typ », « f2.typ », « f3.typ » et 5 autres.",
    ]); // nothing said for the superseded stream
    requests.stop();
  });

  it("never starts a request while the previous one's show is still running", async () => {
    const { socket, target, requests } = setup(Promise.resolve(), { readyMs: 50, stepMs: 20 });
    let active = 0;
    let most = 0;
    target.prepare = () => {
      active++;
      most = Math.max(most, active);
    };
    target.show = async (path) => {
      // The first is longer than its bound, and not cancellable.
      if (path === "slow.typ") await new Promise((r) => setTimeout(r, 60));
      active--;
      target.shown.push(path);
      return true;
    };
    socket.frame({ id: 1, show: "slow.typ" });
    socket.frame({ id: 2, show: "next.typ" });
    await requests.idle();
    expect(most).toBe(1);
    expect(target.shown).toEqual(["slow.typ", "next.typ"]);
    expect(socket.sent).toEqual([{ handled: 1 }, { handled: 2 }]);
  });

  it("says a late success is slow, then opens the file, with no failure toast", async () => {
    const { socket, target, requests } = setup(Promise.resolve(), { readyMs: 50, stepMs: 20 });
    target.show = async (path) => {
      await new Promise((r) => setTimeout(r, 60));
      target.shown.push(path);
      return true;
    };
    socket.frame({ id: 1, show: "late.typ" });
    await new Promise((r) => setTimeout(r, 40));
    expect(target.toasts).toEqual(["L'ouverture de « late.typ » prend plus de temps que prévu."]);
    expect(socket.sent).toEqual([]);
    await requests.idle();
    expect(target.shown).toEqual(["late.typ"]);
    expect(target.toasts).toEqual(["L'ouverture de « late.typ » prend plus de temps que prévu."]);
    expect(socket.sent).toEqual([{ handled: 1 }]);
  });

  it("says a late failure is slow, then that the file could not be opened", async () => {
    const { socket, target, requests } = setup(Promise.resolve(), { readyMs: 50, stepMs: 20 });
    target.show = async () => {
      await new Promise((r) => setTimeout(r, 60));
      throw new Error("unreadable");
    };
    socket.frame({ id: 1, show: "late.typ" });
    await new Promise((r) => setTimeout(r, 40));
    expect(socket.sent).toEqual([]);
    await requests.idle();
    expect(target.shown).toEqual([]);
    expect(target.toasts).toEqual([
      "L'ouverture de « late.typ » prend plus de temps que prévu.",
      "Inkwell n'a pas pu ouvrir « late.typ ».",
    ]);
    expect(socket.sent).toEqual([{ handled: 1 }]);
  });

  it("says a workspace slow to become ready is slow, and keeps the request", async () => {
    const { socket, target } = setup(new Promise(() => {}), { readyMs: 30, stepMs: 30 });
    socket.frame({ id: 1, show: "a.typ" });
    await new Promise((r) => setTimeout(r, 60));
    expect(target.toasts).toEqual(["L'ouverture de « a.typ » prend plus de temps que prévu."]);
    expect(socket.sent).toEqual([]);
  });

  it("says a burst of files of one kind in one toast that names them (UX-OI-1)", async () => {
    const { socket, target, requests } = setup();
    for (const name of ["un.md", "deux.md", "trois.md"])
      socket.frame({ notices: [{ kind: "loose", name }] });
    await requests.idle();
    expect(target.toasts).toEqual([
      "3 fichiers ne sont dans aucun projet : « un.md », « deux.md », « trois.md ». Inkwell ne sait pas encore ouvrir un fichier seul.",
    ]);
    expect(target.roles).toEqual(["status"]);
  });

  it("names three files, then counts the others with the right plural", async () => {
    const { socket, target, requests } = setup();
    socket.frame({ notices: ["a", "b", "c", "d"].map((name) => ({ kind: "folder", name })) });
    await requests.idle();
    socket.frame({ notices: ["a", "b", "c", "d", "e"].map((name) => ({ kind: "folder", name })) });
    await requests.idle();
    expect(target.toasts).toEqual([
      "4 dossiers ne sont pas des projets Inkwell : « a », « b », « c » et 1 autre.",
      "5 dossiers ne sont pas des projets Inkwell : « a », « b », « c » et 2 autres.",
    ]);
  });

  it("says every failure of a burst in one alert toast, each file with its reason", async () => {
    const { socket, target, requests } = setup();
    target.show = (path) => path !== "b.md";
    socket.frame({ notices: [{ kind: "refused", reason: "missing", name: "absent.md" }] });
    socket.frame({ show: "b.md" });
    await requests.idle();
    socket.frame({
      notices: ["p", "q", "r", "s", "t"].map((name) => ({
        kind: "refused",
        reason: "missing",
        name,
      })),
    });
    await requests.idle();
    expect(target.toasts).toEqual([
      "« absent.md » est introuvable, « b.md » n'a pas pu s'ouvrir.",
      "« p » est introuvable, « q » est introuvable, « r » est introuvable et 2 autres fichiers non ouverts.",
    ]);
    expect(target.roles).toEqual(["alert", "alert"]);
  });

  it("keeps the sentence of a single failure, in an alert toast (UX-OI-3)", async () => {
    const { socket, target, requests } = setup();
    target.show = () => false;
    socket.frame({ id: 1, show: ".cache.md" });
    await requests.idle();
    expect(target.toasts).toEqual(["Inkwell n'a pas pu ouvrir « .cache.md »."]);
    expect(target.roles).toEqual(["alert"]);
  });

  it("names every other argument of one request: the failure, then the rest (UX-OI-2)", async () => {
    const { socket, target, requests } = setup();
    socket.frame({
      show: "a.md",
      notices: [
        { kind: "refused", reason: "missing", name: "absent.md" },
        { kind: "loose", name: "brouillon.md" },
      ],
    });
    await requests.idle();
    expect(target.shown).toEqual(["a.md"]);
    expect(target.toasts).toEqual([
      "« absent.md » est introuvable.",
      "« brouillon.md » n'est dans aucun projet. Inkwell ne sait pas encore ouvrir un fichier seul.",
    ]);
    expect(target.roles).toEqual(["alert", "status"]);
  });

  it("says the files shown by a burst when there are several", async () => {
    const { socket, target, requests } = setup();
    socket.frame({ show: "a.typ" });
    socket.frame({ show: "b.typ" });
    await requests.idle();
    expect(target.shown).toEqual(["a.typ", "b.typ"]);
    expect(target.toasts).toEqual(["2 fichiers ouverts : « a.typ », « b.typ »."]);
  });
});
