import { type MessageKey, t, tn } from "../i18n";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import { type ClosePage, readCloseControl } from "./closePage";

/**
 * The `open` stream carries what Inkwell was asked to open from the desktop: a file chosen with
 * « Ouvrir avec Inkwell », or a second launch handed over to this window (W41; Go side
 * `cmd/inkwell/open.go`). Go has already raised the window and decided what to do (plan
 * docs/plans/2026-11-11-oi-single-instance-open.md, rows R3-R9): show a file of the open project,
 * and a notice for every other argument. The window says them, from the catalogue, never from Go:
 * the requests that end within one short burst give one toast per kind of outcome, naming the
 * files, and the failures their own toast, which interrupts a screen reader (UX-OI-1 to UX-OI-3).
 * The launch that asked has already ended: this window owns the outcome (rule D-ACK), and reports
 * each request handled ({"handled":id}) once it is said, so that Go keeps it, and a closing window
 * waits for it, until then.
 */

/** Why an argument was not opened (internal/launch Kind and Reason). */
export type NoticeKind = "show" | "otherProject" | "loose" | "folder" | "refused";
export type NoticeReason =
  | "missing"
  | "unreadable"
  | "special"
  | "internal"
  | "tooLong"
  | "tooMany"
  | "badName";

export interface OpenNotice {
  kind: NoticeKind;
  reason?: NoticeReason;
  /** The base name of the argument as given. */
  name: string;
  /** For tooMany: the arguments not taken. */
  count: number;
}

export interface OpenRequest {
  /** The request's number, reported back once handled; 0 when Go gave none. */
  id: number;
  /** A slash path in the open project. */
  show?: string;
  /** One for each other argument, in order. */
  notices: OpenNotice[];
}

type InfoKind = Exclude<NoticeKind, "refused">;
type FailReason = Exclude<NoticeReason, "tooMany">;

const KINDS: Record<InfoKind, MessageKey> = {
  show: "open.notice.notShown",
  otherProject: "open.notice.otherProject",
  loose: "open.notice.loose",
  folder: "open.notice.folder",
};
const REASONS: Record<FailReason, MessageKey> = {
  missing: "open.notice.missing",
  unreadable: "open.notice.unreadable",
  special: "open.notice.special",
  internal: "open.notice.internal",
  tooLong: "open.notice.tooLong",
  badName: "open.notice.badName",
};
/** A failure as an item of a list (« « a.md » est introuvable »). */
const FAILS: Record<FailReason, MessageKey> = {
  missing: "open.fail.missing",
  unreadable: "open.fail.unreadable",
  special: "open.fail.special",
  internal: "open.fail.internal",
  tooLong: "open.fail.tooLong",
  badName: "open.fail.badName",
};
/** Several files of one kind, with their names. */
const SUMMARIES: Record<InfoKind, string> = {
  otherProject: "open.summary.otherProject",
  loose: "open.summary.loose",
  folder: "open.summary.folder",
  show: "open.summary.notShown",
};

/** Bounds of a frame: a name is a base name (at most 4096 bytes on the Go side). */
const MAX_FRAME = 256 << 10;
const MAX_TEXT = 4096;
const MAX_COUNT = 1 << 20;
/** Notices of one request: at most 32 arguments and the TooMany of the rest on the Go side. */
const MAX_NOTICES = 64;
/** Requests received and not applied yet; beyond, a request is left and said in one toast. */
export const MAX_PENDING = 8;
/** Quiet time after the last request ended before its burst is said (UX-OI-1: about a second). */
export const OPEN_BURST_MS = 800;

const isObject = (v: unknown): v is Record<string, unknown> =>
  typeof v === "object" && v !== null && !Array.isArray(v);
const text = (v: unknown): string | undefined =>
  v === undefined ? "" : typeof v === "string" && v.length <= MAX_TEXT ? v : undefined;
const count = (v: unknown): number | undefined => {
  if (v === undefined) return 0;
  return typeof v === "number" && Number.isInteger(v) && v >= 0 && v <= MAX_COUNT ? v : undefined;
};

function parseNotice(v: unknown): OpenNotice | undefined {
  if (!isObject(v)) return undefined;
  const { kind, reason } = v;
  const name = text(v.name);
  const n = count(v.count);
  if (name === undefined || n === undefined) return undefined;
  if (kind === "refused") {
    if (typeof reason !== "string" || !(Object.hasOwn(REASONS, reason) || reason === "tooMany"))
      return undefined;
    return { kind, reason: reason as NoticeReason, name, count: n };
  }
  if (typeof kind !== "string" || !Object.hasOwn(KINDS, kind) || reason !== undefined)
    return undefined;
  return { kind: kind as NoticeKind, name, count: n };
}

/** Reads a frame of the stream; undefined (never a throw) for anything else. */
export function parseOpen(frame: Uint8Array | string): OpenRequest | undefined {
  if (frame.length > MAX_FRAME) return undefined;
  let v: unknown;
  try {
    v = JSON.parse(typeof frame === "string" ? frame : new TextDecoder().decode(frame));
  } catch {
    return undefined;
  }
  if (!isObject(v)) return undefined;
  const id = v.id === undefined ? 0 : v.id;
  if (typeof id !== "number" || !Number.isSafeInteger(id) || id < 0) return undefined;
  const out: OpenRequest = { id, notices: [] };
  if (v.show !== undefined) {
    const show = text(v.show);
    if (!show) return undefined;
    out.show = show;
  }
  if (v.notices !== undefined) {
    if (!Array.isArray(v.notices) || v.notices.length > MAX_NOTICES) return undefined;
    for (const raw of v.notices) {
      const n = parseNotice(raw);
      if (!n) return undefined;
      out.notices.push(n);
    }
  }
  return out;
}

/** The sentence of one notice, as a toast of its own. */
export function noticeText(n: OpenNotice): string {
  if (n.kind !== "refused") return t(KINDS[n.kind], { name: n.name });
  if (n.reason === "tooMany") return tn("open.notice.tooMany", n.count);
  return t(REASONS[n.reason ?? "missing"], { name: n.name });
}

/** What one request ended with, to be said with the others of its burst. */
export type Said =
  | { kind: "shown"; name: string }
  | { kind: "failed"; name: string }
  | { kind: "notice"; notice: OpenNotice };

/** A toast to show: « alert » interrupts a screen reader, for what failed. */
export interface Toast {
  text: string;
  role: "alert" | "status";
}

const isFailure = (s: Said) =>
  s.kind === "failed" || (s.kind === "notice" && s.notice.kind === "refused");

/** A failure as an item of a list. */
function failItem(s: Said): string {
  if (s.kind === "failed") return t("open.fail.failed", { name: s.name });
  if (s.kind !== "notice") return "";
  if (s.notice.reason === "tooMany") return tn("open.fail.tooMany", s.notice.count);
  return t(FAILS[(s.notice.reason ?? "missing") as FailReason], { name: s.notice.name });
}

/** A failure as a toast of its own: today's sentence. */
function failSentence(s: Said): string {
  if (s.kind === "failed") return t("open.notice.failed", { name: s.name });
  return s.kind === "notice" ? noticeText(s.notice) : "";
}

/** Up to three items, then « et N autres » (more is the plural key of that tail). */
function list(items: string[], more: string): string {
  const head = items.slice(0, 3).join(", ");
  return items.length > 3 ? `${head} ${tn(more, items.length - 3)}` : head;
}

const names = (items: string[]) =>
  list(
    items.map((name) => t("open.summary.name", { name })),
    "open.summary.more",
  );

/**
 * What a burst of requests says: the failures first, in one toast that interrupts a screen reader
 * (each file and why, up to three, then how many others); then one toast per kind of notice, in the
 * order they first came, and
 * one for the files shown when there are several, each naming up to three files then how many
 * others. A single item keeps its sentence of one request.
 */
export function summarize(said: Said[]): Toast[] {
  const out: Toast[] = [];
  const fails = said.filter(isFailure);
  const [first] = fails;
  if (fails.length === 1 && first) out.push({ role: "alert", text: failSentence(first) });
  else if (fails.length > 1) {
    const sentence = list(fails.map(failItem), "open.summary.moreFailed");
    out.push({ role: "alert", text: `${sentence.charAt(0).toUpperCase()}${sentence.slice(1)}.` });
  }
  const kinds = new Set<InfoKind>();
  for (const s of said)
    if (s.kind === "notice" && s.notice.kind !== "refused") kinds.add(s.notice.kind);
  for (const kind of kinds) {
    const of = said.flatMap((s) =>
      s.kind === "notice" && s.notice.kind === kind ? [s.notice] : [],
    );
    const [one] = of;
    if (of.length === 1 && one) out.push({ role: "status", text: noticeText(one) });
    else if (of.length > 1)
      out.push({
        role: "status",
        text: tn(SUMMARIES[kind], of.length, { names: names(of.map((n) => n.name)) }),
      });
  }
  const shown = said.flatMap((s) => (s.kind === "shown" ? [s.name] : []));
  if (shown.length > 1)
    out.push({
      role: "status",
      text: tn("open.summary.shown", shown.length, { names: names(shown) }),
    });
  return out;
}

/** What an open request acts on: the workspace once it is ready, and the toast. */
export interface OpenTarget {
  closePage?: ClosePage;
  /** Settles once the workspace has its project: a request that comes earlier waits for it. */
  ready: Promise<unknown>;
  /**
   * Gets the workspace ready to show a file (the file list read again, a sheet view left). Once
   * signal is aborted (the stream was superseded) it should stop early.
   */
  prepare(signal: AbortSignal): Promise<void> | void;
  /** Shows a file of the open project in the editor; false when it could not (not in the list). */
  show(path: string, signal: AbortSignal): Promise<boolean> | boolean;
  /** Shows a toast (the existing notice at the bottom of the window), after those already up. */
  toast(text: string, role: "alert" | "status"): void;
}

/** The base name of a slash path, for a toast. */
const baseName = (path: string) => path.slice(path.lastIndexOf("/") + 1);

/**
 * How long the workspace may take to be ready, and the showing of one file (prepare, show), before
 * the window says that it is taking longer than expected.
 */
export interface Bounds {
  readyMs: number;
  stepMs: number;
}

/** The first opening of a project can be slow; showing a file of an open workspace cannot. */
export const BOUNDS: Bounds = { readyMs: 60_000, stepMs: 10_000 };

/** Settles like p, or rejects after ms: waiting for the workspace changes nothing, so it may stop. */
function within<T>(p: Promise<T>, ms: number): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  return Promise.race([
    p,
    new Promise<never>((_, reject) => {
      timer = setTimeout(() => reject(new Error(`no answer within ${ms} ms`)), ms);
    }),
  ]).finally(() => clearTimeout(timer));
}

const warn = (err: unknown) =>
  console.warn("open request:", err instanceof Error ? err.message : "failed");

/**
 * Applies one request: the file (Go raised the window), and returns what is to be said about it:
 * the file shown, or that it could not be (a workspace that failed to open, a step that threw or
 * answered false), and the notices of its other arguments. A file whose showing passes its bound
 * is not a failure: the window says at once that it is taking longer than expected, and the
 * request keeps its slot until the step really ends. It resolves only once its steps have ended,
 * so that two requests never run at once: a step that never ends holds the next ones until the
 * window closes (the close residual). Once stop is aborted (the stream was superseded), nothing is
 * shown: Go sends the request again on the new stream.
 */
export async function applyOpen(
  r: OpenRequest,
  target: OpenTarget,
  bounds: Bounds = BOUNDS,
  stop: AbortSignal = new AbortController().signal,
): Promise<Said[]> {
  const notices: Said[] = r.notices.map((notice) => ({ kind: "notice", notice }));
  if (!r.show) {
    // Nothing to show: the notices wait for the workspace at most its bound.
    try {
      await within(Promise.resolve(target.ready), bounds.readyMs);
    } catch (err) {
      warn(err);
    }
    return notices;
  }
  const name = baseName(r.show);
  const failed: Said[] = [{ kind: "failed", name }, ...notices];
  let said = false;
  const slow = () => {
    if (said || stop.aborted) return;
    said = true;
    target.toast(t("open.notice.slow", { name }), "status");
  };
  let timer = setTimeout(slow, bounds.readyMs);
  try {
    await target.ready;
  } catch (err) {
    warn(err);
    return failed;
  } finally {
    clearTimeout(timer);
  }
  timer = setTimeout(slow, bounds.stepMs);
  let shown = false;
  try {
    await target.prepare(stop);
    if (!stop.aborted) shown = await target.show(r.show, stop);
  } catch (err) {
    warn(err);
  } finally {
    clearTimeout(timer);
  }
  return shown ? [{ kind: "shown", name }, ...notices] : failed;
}

/** One connection of the open stream and what its window holds (rule D-OWNER). */
interface Session {
  socket: SocketLike;
  /** Aborted once the connection is closed or superseded: its requests are discarded. */
  stop: AbortController;
  /** Requests received and not reported handled yet. */
  pending: number;
  /** Requests left beyond MAX_PENDING, said in one toast once the others are handled. */
  dropped: number[];
}

/** A request applied and not said yet, with its session. */
interface Unsaid {
  s: Session;
  id: number;
  said: Said[];
}

/**
 * Reads the `open` stream for as long as the window lives, reconnecting when it drops, and applies
 * its requests one after the other, in the order they came. What they end with is said once no
 * request has ended for burstMs: one toast per kind of outcome (summarize), then each is reported
 * handled. At most MAX_PENDING wait; beyond, a request is not applied and one toast says how many
 * were left (Go sends at most as many). Each connection starts afresh: what the previous one held
 * is discarded, said or not (Go sends it again on the new one), and its reports are never sent
 * through the new one.
 */
export class OpenRequests {
  readonly #open: () => SocketLike;
  readonly #target: OpenTarget;
  readonly #reconnectMs: number;
  readonly #onApplied: (r: OpenRequest, n: number) => void;
  readonly #bounds: Bounds;
  readonly #burstMs: number;
  #session: Session | undefined;
  #stopped = true;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #chain: Promise<void> = Promise.resolve();
  #received = 0;
  #unsaid: Unsaid[] = [];
  #burst:
    | { timer: ReturnType<typeof setTimeout> | undefined; done: Promise<void>; end: () => void }
    | undefined;

  constructor(
    open: () => SocketLike,
    target: OpenTarget,
    reconnectMs = 1000,
    onApplied: (r: OpenRequest, n: number) => void = () => {},
    bounds: Bounds = BOUNDS,
    burstMs = OPEN_BURST_MS,
  ) {
    this.#bounds = bounds;
    this.#open = open;
    this.#target = target;
    this.#reconnectMs = reconnectMs;
    this.#onApplied = onApplied;
    this.#burstMs = burstMs;
  }

  start(): void {
    if (!this.#stopped) return;
    this.#stopped = false;
    this.#connect();
  }

  stop(): void {
    this.#stopped = true;
    clearTimeout(this.#timer);
    const s = this.#retire();
    s?.socket.close();
  }

  /** Settles once every request received so far is applied and said. */
  async idle(): Promise<void> {
    for (;;) {
      const chain = this.#chain;
      await chain;
      const burst = this.#burst;
      if (burst) await burst.done;
      if (chain === this.#chain && !this.#burst) return;
    }
  }

  /** Ends the current session: what it holds is discarded. */
  #retire(): Session | undefined {
    const s = this.#session;
    this.#session = undefined;
    s?.stop.abort(new Error("open stream superseded"));
    if (s) this.#target.closePage?.disconnect();
    return s;
  }

  /** Keeps what a request ended with for its burst, said once no request has ended for burstMs. */
  #keep(s: Session, id: number, said: Said[]): void {
    this.#unsaid.push({ s, id, said });
    if (!this.#burst) {
      let end = () => {};
      const done = new Promise<void>((r) => (end = r));
      this.#burst = { timer: undefined, done, end };
    }
    clearTimeout(this.#burst.timer);
    this.#burst.timer = setTimeout(() => this.#sayBurst(), this.#burstMs);
  }

  /** Says the burst (summarize), then reports each of its requests handled. */
  #sayBurst(): void {
    const burst = this.#burst;
    this.#burst = undefined;
    const live = this.#unsaid.filter((u) => u.s === this.#session);
    this.#unsaid = [];
    for (const toast of summarize(live.flatMap((u) => u.said)))
      this.#target.toast(toast.text, toast.role);
    for (const u of live) this.#handled(u.s, u.id);
    burst?.end();
  }

  /** Tells Go, on the session's own stream, that the request is handled: it frees its slot. */
  #handled(s: Session, id: number): void {
    if (this.#session !== s) return;
    s.pending--;
    const ids = id > 0 ? [id] : [];
    if (s.pending === 0 && s.dropped.length > 0) {
      this.#target.toast(tn("open.notice.dropped", s.dropped.length), "status");
      ids.push(...s.dropped.filter((d) => d > 0));
      s.dropped = [];
    }
    for (const d of ids) s.socket.send(new TextEncoder().encode(JSON.stringify({ handled: d })));
  }

  #connect(): void {
    if (this.#stopped) return;
    this.#retire();
    const socket = this.#open();
    socket.binaryType = "arraybuffer";
    const s: Session = { socket, stop: new AbortController(), pending: 0, dropped: [] };
    this.#session = s;
    socket.onmessage = (ev) => {
      if (this.#session !== s) return;
      const data = ev.data as ArrayBuffer | string;
      const bytes = typeof data === "string" ? data : new Uint8Array(data);
      const close = this.#target.closePage && readCloseControl(bytes);
      if (close) {
        this.#target.closePage?.receive(close, (reply) => {
          if (this.#session === s) s.socket.send(new TextEncoder().encode(JSON.stringify(reply)));
        });
        return;
      }
      const r = parseOpen(bytes);
      if (!r) return;
      if (s.pending >= MAX_PENDING) {
        // Never queued without limit (Go sends at most as many); said once the queue is free.
        s.dropped.push(r.id);
        return;
      }
      s.pending++;
      const n = ++this.#received;
      this.#chain = this.#chain.then(async () => {
        if (s.stop.signal.aborted) return; // superseded: Go sends it again on the new stream
        try {
          const said = await applyOpen(r, this.#target, this.#bounds, s.stop.signal);
          if (s.stop.signal.aborted) return;
          this.#onApplied(r, n);
          this.#keep(s, r.id, said);
        } catch (err) {
          warn(err);
        }
      });
    };
    socket.onclose = () => {
      if (this.#session !== s) return;
      this.#retire();
      if (!this.#stopped) this.#timer = setTimeout(() => this.#connect(), this.#reconnectMs);
    };
  }
}
