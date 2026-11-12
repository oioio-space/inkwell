import { Transaction } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, it, vi } from "vitest";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import { startNativePerf } from "./native";
import { dispatchNativeTransactions } from "./nativeDispatch";
import { closeNativeEarly, installNativeEarly } from "./nativeEarly";
import { NATIVE_CONTROLS } from "./nativeProtocol";
import { BrowserClock } from "./nativeTiming";
import { nativeProbesOn } from "./nativeTrace";

afterEach(() => {
  closeNativeEarly();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.useRealTimers();
});

function trustedShift(target: EventTarget = document.body) {
  const event = new KeyboardEvent("keydown", { key: "Shift", bubbles: true });
  Object.defineProperty(event, "isTrusted", { value: true });
  target.dispatchEvent(event);
}

/** Drive the actual harness handshake and animation callbacks, not a heartbeat helper. */
/** App work writing the document outside the editor (paths it changed), for NULL isolation tests. */
let foreignWrite: string[] | undefined;
function runningHarness(phase: "cold" | "warm") {
  foreignWrite = undefined;
  let foreignWritten = false;
  vi.useFakeTimers();
  let now = 100;
  let clockTicks = 0;
  vi.spyOn(performance, "now").mockImplementation(() => now + Math.floor(clockTicks++ / 16));
  const callbacks = new Map<number, FrameRequestCallback>();
  let nextFrame = 0;
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    callbacks.set(++nextFrame, callback);
    return nextFrame;
  });
  vi.stubGlobal("cancelAnimationFrame", (id: number) => callbacks.delete(id));
  const tasks: (() => void)[] = [];
  vi.stubGlobal(
    "MessageChannel",
    class {
      closed = false;
      port1 = {
        onmessage: null as ((event: { data: unknown }) => void) | null,
        close: () => {
          this.closed = true;
        },
      };
      port2 = {
        postMessage: (data: unknown) => {
          tasks.push(() => {
            if (!this.closed) this.port1.onmessage?.({ data });
          });
        },
        close: () => {
          this.closed = true;
        },
      };
    },
  );
  const flushTasks = () => {
    const batch = tasks.splice(0);
    for (const task of batch) task();
  };
  let clockReads = 0;
  let failClockAt = Number.POSITIVE_INFINITY;
  let browserReads = 0;
  const browserRead = BrowserClock.prototype.read;
  vi.spyOn(BrowserClock.prototype, "read").mockImplementation(function (this: BrowserClock) {
    if (++browserReads === failClockAt) throw new Error("injected clock failure");
    return browserRead.call(this);
  });
  vi.stubGlobal(
    "XMLHttpRequest",
    class {
      status = 200;
      responseText = "";
      open() {}
      send() {
        clockReads++;
        this.responseText = JSON.stringify({
          ms: now + Math.floor(clockTicks / 16) + clockReads / 1e6,
          resolutionMs: 0.000001,
        });
      }
    },
  );
  vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  installNativeEarly(true);
  const parent = document.body.appendChild(document.createElement("div"));
  const view = new EditorView({ parent });
  vi.spyOn(view, "hasFocus", "get").mockReturnValue(true);
  const sent: Record<string, unknown>[] = [];
  const socket = {
    binaryType: "arraybuffer",
    onopen: null,
    onmessage: null,
    onclose: null,
    send: (data: Uint8Array) => sent.push(JSON.parse(new TextDecoder().decode(data))),
    close: vi.fn(),
  } as unknown as SocketLike;
  const stop = startNativePerf({
    open: () => socket,
    view: () => view,
    revision: () => (view.state.doc.length ? "head-B" : "head-A") + (foreignWritten ? "+meta" : ""),
    foreignChange: (before, after) => (before !== after ? foreignWrite : []),
  });
  const deliver = (value: unknown) =>
    socket.onmessage?.({ data: JSON.stringify(value) } as MessageEvent);
  const pulse = (step = 16) => {
    now += step;
    vi.advanceTimersByTime(step);
    const batch = [...callbacks.values()];
    callbacks.clear();
    for (const callback of batch) callback(now);
    flushTasks();
    return now;
  };
  /** `drift`: event.timeStamp minus performance.now(), as seen on the host (-4..+5 ms). */
  let drift = 0;
  const trusted = (event: Event) => {
    Object.defineProperty(event, "isTrusted", { value: true });
    Object.defineProperty(event, "timeStamp", { value: performance.now() + drift });
    view.contentDOM.dispatchEvent(event);
  };
  return {
    sent,
    socket,
    deliver,
    pulse,
    callbacks,
    view,
    flushTasks,
    work(ms: number) {
      now += ms;
    },
    failClockAt(read: number) {
      failClockAt = browserReads + read;
    },
    typeKey(eventClockDrift = 0) {
      drift = eventClockDrift;
      trusted(new KeyboardEvent("keydown", { key: "x", bubbles: true }));
      trusted(new InputEvent("beforeinput", { inputType: "insertText", data: "x", bubbles: true }));
      dispatchNativeTransactions(
        [
          view.state.update({
            changes: { from: 0, insert: "x" },
            annotations: Transaction.userEvent.of("input"),
          }),
        ],
        view,
      );
      trusted(new InputEvent("input", { inputType: "insertText", data: "x", bubbles: true }));
    },
    /** The ownTyping prototype: CodeMirror prevents the typed insertText and dispatches it. */
    typeOwnedKey({ prevent = true, input = false } = {}) {
      drift = 0;
      trusted(new KeyboardEvent("keydown", { key: "x", bubbles: true }));
      const own = (event: Event) => {
        if (!prevent) return;
        event.preventDefault();
        dispatchNativeTransactions(
          [
            view.state.update({
              changes: { from: 0, insert: "x" },
              annotations: Transaction.userEvent.of("input"),
            }),
          ],
          view,
        );
      };
      view.contentDOM.addEventListener("beforeinput", own, { once: true });
      trusted(
        new InputEvent("beforeinput", {
          inputType: "insertText",
          data: "x",
          bubbles: true,
          cancelable: true,
        }),
      );
      if (input)
        trusted(new InputEvent("input", { inputType: "insertText", data: "x", bubbles: true }));
    },
    nullKey() {
      const target = document.querySelector<HTMLTextAreaElement>("[data-native-perf-null]");
      if (!target) throw new Error("missing NULL target");
      // A real NULL key reaches the focused textarea, never CodeMirror.
      expect(document.activeElement).toBe(target);
      const dispatch = (event: Event) => {
        Object.defineProperty(event, "isTrusted", { value: true });
        Object.defineProperty(event, "timeStamp", { value: performance.now() });
        target.dispatchEvent(event);
      };
      dispatch(new KeyboardEvent("keydown", { key: "x", bubbles: true }));
      dispatch(
        new InputEvent("beforeinput", { inputType: "insertText", data: "x", bubbles: true }),
      );
      target.value = "x"; // Emulate the browser's default textarea edit.
      dispatch(new InputEvent("input", { inputType: "insertText", data: "x", bubbles: true }));
      if (foreignWrite) foreignWritten = true; // e.g. the 2 s device claim lands mid-trial.
    },
    start(prove = true) {
      socket.onopen?.({} as Event);
      deliver({
        type: "capability",
        protocol: 1,
        challenge: sent[0]?.challenge,
        engine: "WebKitGTK",
        native: true,
        runId: "native-heartbeat",
        phase,
        sampleCount: phase === "cold" ? 1 : 30,
        refreshHz: 60,
        inputPermissionVerified: phase === "cold" && prove,
        replicateIndex: 1,
        replicateCount: 30,
      });
      if (prove && phase === "warm") trustedShift(view.contentDOM);
    },
    clockReads: () => clockReads,
    stall() {
      now += 3000;
      vi.advanceTimersByTime(3000);
    },
    close() {
      stop();
      view.destroy();
      parent.remove();
    },
  };
}

it.each(["cold", "warm"] as const)(
  "starts and renews the %s native heartbeat after the clock preflight, before arming",
  (phase) => {
    const harness = runningHarness(phase);
    try {
      expect(harness.sent).toHaveLength(0);
      harness.start();
      expect(harness.clockReads()).toBe(4);
      expect(harness.sent.some((message) => message.type === "ready")).toBe(true);
      expect(harness.sent.find((message) => message.type === "ready")?.clock).toMatchObject({
        resolutionMs: 1,
        precise: { source: "browser-bounded", resolutionMs: 1, batchSize: 1000 },
      });
      expect(harness.callbacks.size).toBeGreaterThan(0);
      harness.deliver({ type: "trial", id: 1, control: phase === "cold" ? "none" : "handler" });
      for (let i = 0; i < 5; i++) harness.pulse();
      expect(harness.sent.some((message) => message.type === "armed")).toBe(false);
      harness.pulse();
      expect(harness.sent.filter((message) => message.type === "armed")).toHaveLength(1);
      // Replay the host's actual state: healthy callbacks, four clock reads,
      // no delivered native key. It remains a failure, not a display diagnosis.
      for (let i = 0; i < 188; i++) harness.pulse();
      const failure = harness.sent.find((message) => message.type === "failure");
      expect(failure?.reason).toMatch(/Native input missing.*DOM keydowns=0/);
      expect(failure?.reason).not.toMatch(/frame heartbeat absent/);
      expect(harness.sent.some((message) => message.type === "sample")).toBe(false);
      expect(harness.clockReads()).toBe(4);
      expect(harness.socket.close).not.toHaveBeenCalled();
    } finally {
      harness.close();
    }
  },
);

it("keeps a stopped heartbeat fatal after arming", () => {
  const harness = runningHarness("warm");
  try {
    harness.start();
    harness.deliver({ type: "trial", id: 1, control: "handler" });
    for (let i = 0; i < 6; i++) harness.pulse();
    expect(harness.sent.some((message) => message.type === "armed")).toBe(true);
    harness.stall();
    expect(harness.sent.find((message) => message.type === "failure")?.reason).toMatch(
      /frame heartbeat absent/,
    );
    expect(harness.sent.some((message) => message.type === "sample")).toBe(false);
  } finally {
    harness.close();
  }
});

it("reports focus loss with the code focus-lost", () => {
  const h = runningHarness("warm");
  try {
    h.start();
    // Another window takes the focus between the handshake and the first armed trial.
    vi.mocked(document.hasFocus).mockReturnValue(false);
    h.deliver({ type: "trial", id: 1, control: "handler" });
    const failure = h.sent.find((message) => message.type === "failure");
    expect(failure?.reason).toMatch(
      /^\[fault:focus-lost\] native editor must be visible and focused/,
    );
    expect(h.sent.some((message) => message.type === "armed")).toBe(false);
  } finally {
    h.close();
  }
});

it("reports a focus the page moved itself as a product failure, without the fault code", () => {
  const h = runningHarness("warm");
  try {
    h.start();
    // The window keeps the focus; the editor lost it inside the page: the product moved it, not the host.
    vi.spyOn(h.view, "hasFocus", "get").mockReturnValue(false);
    h.deliver({ type: "trial", id: 1, control: "handler" });
    const failure = h.sent.find((message) => message.type === "failure");
    expect(failure?.reason).toMatch(/^native editor lost the focus inside the page/);
    expect(String(failure?.reason)).not.toContain("[fault:");
    expect(h.sent.some((message) => message.type === "armed")).toBe(false);
  } finally {
    h.close();
  }
});

it("measures a trusted NULL key through identical brackets without editing CodeMirror or doing synchronous I/O", () => {
  const h = runningHarness("warm");
  try {
    h.start();
    for (let i = 0; i < 6; i++) h.pulse();
    h.deliver({ type: "trial", id: 38, control: "null" });
    h.nullKey();
    h.flushTasks();
    for (let i = 0; i < 90; i++) h.pulse();
    const sample = h.sent.find((m) => m.type === "sample") as unknown as {
      nullInserted: boolean;
      changed: boolean;
      handlers: { event: string }[];
      preInputHeartbeatCount: number;
    };
    expect(sample, JSON.stringify(h.sent)).toBeDefined();
    expect(sample.nullInserted).toBe(true);
    expect(sample.changed).toBe(false);
    expect(h.view.state.doc.toString()).toBe("");
    expect(sample.handlers.map((x) => x.event)).toEqual([
      "keydown",
      "keystroke-task",
      "beforeinput",
      "input",
      "null-input",
    ]);
    expect(sample.preInputHeartbeatCount).toBeGreaterThan(0);
    expect(h.clockReads()).toBe(4);
  } finally {
    h.close();
  }
});

it.each([5, -4])(
  "starts the keystroke at the capture listener's first clock read; event.timeStamp drift %d ms is a diagnostic",
  (drift) => {
    const h = runningHarness("warm");
    try {
      h.start();
      for (let i = 0; i < 6; i++) h.pulse();
      h.deliver({ type: "trial", id: 8, control: "none" });
      h.typeKey(drift);
      h.flushTasks();
      for (let i = 0; i < 90; i++) h.pulse();
      const sample = h.sent.find((m) => m.type === "sample") as unknown as {
        keyDownAt: number;
        keyEventTimeStamp: number;
        docChangedAt: number;
        handlers: { event: string; start: number; eventTimeStamp?: number }[];
      };
      expect(sample, JSON.stringify(h.sent)).toBeDefined();
      const keydown = sample.handlers.find((x) => x.event === "keydown");
      const outer = sample.handlers.find((x) => x.event === "keystroke-task");
      expect(sample.keyDownAt).toBe(keydown?.start);
      expect(sample.keyDownAt).toBe(outer?.start);
      expect(sample.keyEventTimeStamp).toBe(keydown?.eventTimeStamp);
      expect(sample.keyEventTimeStamp).toBe(sample.keyDownAt + drift);
      // By construction, not by luck of the event clock.
      expect(sample.docChangedAt).toBeGreaterThanOrEqual(sample.keyDownAt);
      for (const handler of sample.handlers)
        expect(handler.start).toBeGreaterThanOrEqual(sample.keyDownAt);
    } finally {
      h.close();
    }
  },
);

it("runs a NULL-off trial with every recording hook off, keeping only the outer bracket", () => {
  const h = runningHarness("warm");
  try {
    h.start();
    for (let i = 0; i < 6; i++) h.pulse();
    const sampleOf = (id: number) =>
      h.sent.find((m) => m.type === "sample" && m.id === id) as unknown as {
        nullInserted: boolean;
        keyDownAt: number;
        tasks: unknown[];
        listeners: unknown[];
        stages: unknown[];
        handlers: {
          event: string;
          start: number;
          end: number;
          fine?: { start: number; end: number };
        }[];
      };
    const trial = (id: number, control: "null" | "null-off") => {
      // Arm from inside a recorded callback, as the Wails stream message does on the host.
      setTimeout(() => h.deliver({ type: "trial", id, control }), 0);
      vi.advanceTimersByTime(0);
      expect(nativeProbesOn()).toBe(control === "null");
      h.nullKey();
      setTimeout(() => {}, 0); // App work scheduled inside the trial.
      h.flushTasks();
      for (let i = 0; i < 90; i++) h.pulse();
      expect(nativeProbesOn()).toBe(true);
      return sampleOf(id);
    };
    const off = trial(38, "null-off");
    expect(off, JSON.stringify(h.sent)).toBeDefined();
    expect(off.nullInserted).toBe(true);
    expect(off.handlers.map((x) => x.event)).toEqual(["keystroke-task"]);
    const [outer] = off.handlers;
    expect(outer?.start).toBe(off.keyDownAt);
    expect(outer?.end).toBeGreaterThanOrEqual(outer?.start ?? Infinity);
    expect(outer?.fine?.end).toBeGreaterThanOrEqual(outer?.fine?.start ?? Infinity);
    expect(off.tasks).toEqual([]);
    expect(off.listeners).toEqual([]);
    expect(off.stages).toEqual([]);
    // The interleaved probes-on trial records again.
    const on = trial(39, "null");
    expect(on.handlers.map((x) => x.event)).toContain("null-input");
    expect(on.tasks.length).toBeGreaterThan(0);
  } finally {
    h.close();
  }
});

it.each([
  { paths: ["meta/claimedAt"], accepted: true },
  { paths: ["meta", "text/3"], accepted: false },
  { paths: [], accepted: false },
])("NULL accepts a mid-trial App write only outside the text: $paths", ({ paths, accepted }) => {
  const h = runningHarness("warm");
  try {
    h.start();
    for (let i = 0; i < 6; i++) h.pulse();
    h.deliver({ type: "trial", id: 38, control: "null" });
    foreignWrite = paths;
    h.nullKey();
    h.flushTasks();
    for (let i = 0; i < 90; i++) h.pulse();
    const sample = h.sent.find((m) => m.type === "sample") as
      | {
          nullInserted: boolean;
          nullForeignPaths?: string[];
          revisionBefore: string;
          revisionAfter: string;
        }
      | undefined;
    const failure = h.sent.find((m) => m.type === "failure");
    if (accepted) {
      expect(sample, JSON.stringify(h.sent)).toBeDefined();
      expect(sample?.nullInserted).toBe(true);
      expect(sample?.nullForeignPaths).toEqual(paths);
      expect(sample?.revisionAfter).not.toBe(sample?.revisionBefore);
      expect(h.view.state.doc.toString()).toBe("");
    } else {
      expect(sample).toBeUndefined();
      expect(failure?.reason).toMatch(/NULL insertion\/isolation failed.*documentPaths/);
    }
  } finally {
    h.close();
  }
});

it("requires actual trusted XTest delivery before ready, and fails fast without granting itself permission", () => {
  const h = runningHarness("warm");
  try {
    h.start(false);
    expect(h.sent.some((m) => m.type === "probe-armed")).toBe(true);
    expect(h.sent.some((m) => m.type === "ready")).toBe(false);
    for (let i = 0; i < 190; i++) h.pulse();
    expect(h.sent.find((m) => m.type === "failure")?.reason).toMatch(
      /XTest input delivery\/permission not validated/,
    );
    expect(h.sent.some((m) => m.type === "armed" || m.type === "sample")).toBe(false);
  } finally {
    h.close();
  }
});

it("accepts input proof only from a trusted Shift keydown inside CodeMirror", () => {
  const h = runningHarness("warm");
  try {
    h.start(false);
    trustedShift(document.body);
    h.view.contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "Shift", bubbles: true }));
    expect(h.sent.some((m) => m.type === "input-proof" || m.type === "ready")).toBe(false);
    trustedShift(h.view.contentDOM);
    expect(h.sent.filter((m) => m.type === "input-proof")).toHaveLength(1);
    expect(h.sent.filter((m) => m.type === "ready")).toHaveLength(1);
    expect(h.sent.some((m) => m.type === "sample")).toBe(false);
  } finally {
    h.close();
  }
});

it("snapshots the LAST six pre-input cadence callbacks rather than the slow startup six", () => {
  const h = runningHarness("cold");
  try {
    h.start();
    for (let i = 0; i < 6; i++) h.pulse(33);
    const recent = Array.from({ length: 6 }, () => h.pulse());
    h.deliver({ type: "trial", id: 1, control: "none" });
    h.typeKey();
    h.flushTasks();
    for (let i = 0; i < 90; i++) h.pulse();
    const sample = h.sent.find((m) => m.type === "sample");
    expect(sample, JSON.stringify(h.sent)).toBeDefined();
    expect(sample?.baseline).toEqual(recent);
    expect(h.clockReads()).toBe(4); // No synchronous XHRs in the input path.
  } finally {
    h.close();
  }
});

it("brackets a document-bubble keydown listener and records a post-input task outside rAF", () => {
  const h = runningHarness("cold");
  const slow = () => h.work(10);
  document.addEventListener("keydown", slow);
  try {
    h.start();
    for (let i = 0; i < 6; i++) h.pulse();
    h.deliver({ type: "trial", id: 1, control: "none" });
    h.typeKey();
    setTimeout(() => h.work(30), 0);
    for (let i = 0; i < 90; i++) h.pulse();
    const sample = h.sent.find((m) => m.type === "sample") as unknown as {
      handlers: { event: string; fine?: { start: number; end: number } }[];
      busy: { start: number; end: number }[];
    };
    expect(sample, JSON.stringify(h.sent)).toBeDefined();
    const outer = sample.handlers.find((x) => x.event === "keystroke-task")?.fine;
    expect((outer?.end ?? 0) - (outer?.start ?? 0)).toBeGreaterThanOrEqual(10);
    expect(sample.busy.some((x) => x.end - x.start >= 30)).toBe(true);
  } finally {
    document.removeEventListener("keydown", slow);
    h.close();
  }
});

it("includes document capture and microtasks from a later window-bubble listener", async () => {
  const h = runningHarness("cold");
  let microtaskRan = false;
  const capture = () => h.work(10);
  const bubble = () =>
    queueMicrotask(() => {
      h.work(10);
      microtaskRan = true;
    });
  try {
    h.start();
    // Registered after the early hooks, as App's listeners are in production.
    document.addEventListener("keydown", capture, true);
    window.addEventListener("input", bubble);
    for (let i = 0; i < 6; i++) h.pulse();
    h.deliver({ type: "trial", id: 1, control: "none" });
    h.typeKey();
    expect(microtaskRan).toBe(false);
    await Promise.resolve();
    vi.runAllTicks();
    expect(microtaskRan).toBe(true);
    h.flushTasks();
    expect(h.clockReads()).toBe(4);
    requestAnimationFrame(function attributedFrame() {
      h.work(5);
      queueMicrotask(() => h.work(1));
    });
    h.pulse();
    await Promise.resolve(); // Let the frame's microtask run, as WebKit would after the callback.
    for (let i = 0; i < 89; i++) h.pulse();
    const sample = h.sent.find((m) => m.type === "sample") as unknown as {
      handlers: { event: string; fine?: { start: number; end: number } }[];
    };
    expect(sample, JSON.stringify(h.sent)).toBeDefined();
    const outer = sample.handlers.find((entry) => entry.event === "keystroke-task")?.fine;
    expect((outer?.end ?? 0) - (outer?.start ?? 0)).toBeGreaterThanOrEqual(20);
    // Per-listener attribution: identities are sent once, spans reference them,
    // and a microtask names the recorded call that scheduled it.
    const full = sample as unknown as {
      listeners: { start: number; end: number; call: number; id: number }[];
      callbacks: {
        id: number;
        target: string;
        event: string;
        capture: boolean;
        name: string;
        scheduledBy?: number;
      }[];
    };
    // (happy-dom's window/document bypass the patched EventTarget prototype, so this
    // checks scheduler callbacks; WebKit's listener identities use the same path.)
    const identity = (pick: (c: (typeof full.callbacks)[number]) => boolean) =>
      full.callbacks.find(pick);
    const frame = identity(
      (c) => c.event === "requestAnimationFrame" && c.name === "attributedFrame",
    );
    expect(frame, JSON.stringify(full.callbacks)).toBeDefined();
    expect(new Set(full.callbacks.map((c) => c.id)).size).toBe(full.callbacks.length);
    const spanOf = (id?: number) => full.listeners.find((l) => l.id === id);
    expect((spanOf(frame?.id)?.end ?? 0) - (spanOf(frame?.id)?.start ?? 0)).toBeGreaterThanOrEqual(
      5,
    );
    const micro = identity(
      (c) => c.event === "queueMicrotask" && c.scheduledBy === spanOf(frame?.id)?.call,
    );
    expect(micro, JSON.stringify(full.callbacks)).toBeDefined();
    expect(full.listeners.every((l) => full.callbacks.some((c) => c.id === l.id))).toBe(true);
  } finally {
    document.removeEventListener("keydown", capture, true);
    window.removeEventListener("input", bubble);
    h.close();
  }
});

it("never reconfigures a CodeMirror transaction synchronously when the ending clock read fails", () => {
  const h = runningHarness("cold");
  try {
    h.start();
    for (let i = 0; i < 6; i++) h.pulse();
    h.deliver({ type: "trial", id: 1, control: "none" });
    h.failClockAt(2);
    expect(() => h.typeKey()).not.toThrow();
    expect(h.view.state.doc.toString()).toBe("x");
    expect(() => {
      h.flushTasks();
      vi.runAllTicks();
    }).not.toThrow();
    expect(h.view.state.doc.toString()).toBe("x");
    expect(h.sent.find((m) => m.type === "failure")?.reason).toMatch(/native handler clock failed/);
    expect(h.sent.some((m) => m.type === "sample")).toBe(false);
  } finally {
    h.close();
  }
});

it.each(NATIVE_CONTROLS)(
  "executes the independent %s control and observes the whole settle tail",
  (control) => {
    const h = runningHarness("warm");
    try {
      h.start();
      for (let i = 0; i < 6; i++) h.pulse();
      h.deliver({ type: "trial", id: NATIVE_CONTROLS.indexOf(control) + 1, control });
      h.typeKey();
      for (let i = 0; i < 100; i++) h.pulse();
      const sample = h.sent.find((m) => m.type === "sample") as unknown as {
        keyDownAt: number;
        settledAt: number;
        control: { kind: string; start: number; end: number; iterations: number };
        frames: { start: number; end: number }[];
        busy: { start: number; end: number }[];
      };
      expect(sample, JSON.stringify(h.sent)).toBeDefined();
      expect(sample.control.kind).toBe(control);
      expect(sample.control.end - sample.control.start).toBeGreaterThanOrEqual(
        ["task", "deferred"].includes(control) ? 30 : 80,
      );
      expect(sample.control.iterations).toBeGreaterThan(0);
      expect(sample.settledAt - sample.keyDownAt).toBeGreaterThanOrEqual(1200);
      if (control === "deferred")
        expect(sample.control.start - sample.keyDownAt).toBeGreaterThanOrEqual(300);
      if (control === "early-raf") {
        expect(
          sample.frames.some((x) => x.start <= sample.control.start && x.end >= sample.control.end),
        ).toBe(false);
        expect(
          sample.busy.some((x) => x.start <= sample.control.start && x.end >= sample.control.end),
        ).toBe(true);
      }
    } finally {
      h.close();
    }
  },
);

it("fails a visible native view that never delivers its first frame instead of waiting forever", () => {
  vi.useFakeTimers();
  let browserTime = 0;
  vi.spyOn(performance, "now").mockImplementation(() => Math.floor(browserTime++ / 16));
  let nativeTime = 0;
  vi.stubGlobal(
    "XMLHttpRequest",
    class {
      status = 200;
      responseText = "";
      open() {}
      send() {
        this.responseText = JSON.stringify({ ms: ++nativeTime, resolutionMs: 0.000001 });
      }
    },
  );
  vi.stubGlobal(
    "requestAnimationFrame",
    vi.fn(() => 1),
  );
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
  vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
  installNativeEarly(true);
  const parent = document.body.appendChild(document.createElement("div"));
  const view = new EditorView({ parent });
  vi.spyOn(view, "hasFocus", "get").mockReturnValue(true);
  const sent: Record<string, unknown>[] = [];
  const socket = {
    binaryType: "arraybuffer",
    onopen: undefined,
    onmessage: undefined,
    onclose: undefined,
    send: (data: Uint8Array) => sent.push(JSON.parse(new TextDecoder().decode(data))),
    close: vi.fn(),
  } as unknown as SocketLike;
  const stop = startNativePerf({ open: () => socket, view: () => view, revision: () => "head-A" });
  const deliver = (value: unknown) =>
    socket.onmessage?.({ data: JSON.stringify(value) } as MessageEvent);
  socket.onopen?.({} as Event);
  deliver({
    type: "capability",
    protocol: 1,
    challenge: sent[0]?.challenge,
    engine: "WebKitGTK",
    native: true,
    runId: "real-native",
    phase: "warm",
    sampleCount: 30,
    refreshHz: 60,
  });
  trustedShift(view.contentDOM);
  expect(sent.some((value) => value.type === "ready")).toBe(true);
  deliver({ type: "trial", id: 1, control: "handler" });
  vi.advanceTimersByTime(3000);
  try {
    expect(sent.find((value) => value.type === "failure")?.reason).toMatch(/frame heartbeat/);
    expect(sent.some((value) => value.type === "armed")).toBe(false);
    expect(socket.close).not.toHaveBeenCalled(); // Let Wails deliver the failure before closing.
  } finally {
    stop();
    view.destroy();
    parent.remove();
  }
});

it.each([
  ["CodeMirror inserts the key itself: keydown and a prevented beforeinput", {}, true],
  [
    "the browser inserts it (not prevented): no input end, the sample never completes",
    { prevent: false, input: true },
    false,
  ],
  ["prevented and an input event as well: incomplete", { input: true }, false],
] as const)("ownTyping prototype (VITE_PERF_CM_TYPING): %s", (_name, key, complete) => {
  vi.stubEnv("VITE_PERF_CM_TYPING", "1");
  const h = runningHarness("cold");
  try {
    h.start();
    for (let i = 0; i < 6; i++) h.pulse();
    h.deliver({ type: "trial", id: 1, control: "none" });
    h.typeOwnedKey(key);
    for (let i = 0; i < 90; i++) h.pulse();
    const sample = h.sent.find((m) => m.type === "sample") as
      | { changed: boolean; trusted: boolean; handlers: { event: string }[] }
      | undefined;
    if (complete) {
      expect(sample, JSON.stringify(h.sent.filter((m) => m.type === "failure"))).toBeDefined();
      expect(sample?.changed).toBe(true);
      expect(sample?.handlers.map((x) => x.event)).not.toContain("input");
    } else {
      expect(sample).toBeUndefined();
      expect(h.sent.some((m) => m.type === "failure")).toBe(true);
    }
  } finally {
    h.close();
  }
});
