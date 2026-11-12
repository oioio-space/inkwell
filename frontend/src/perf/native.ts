import { Compartment, StateEffect } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import {
  profileNativeMethods,
  profileStreamSends,
  registerNativeRecorder,
  registerNativeStages,
} from "./nativeDispatch";
import {
  BusyProbe,
  type EarlyFrame,
  scheduleEarlierFrame,
  subscribeNativeEarly,
} from "./nativeEarly";
import { ContinuousHeartbeat } from "./nativeHeartbeat";
import {
  authorizeNativeRun,
  exactTrustedInsertion,
  isNullControl,
  type NativeCapability,
  type NativeSample,
  NULL_OFF,
  parseTrial,
  rawEventTiming,
  type TrialCommand,
  visibleAndFocused,
} from "./nativeProtocol";
import {
  BrowserClock,
  FOCUS_INTERRUPTED,
  FRAME_UNAVAILABLE,
  FrameTasks,
  NativeClock,
  observeClock,
} from "./nativeTiming";
import { enterNativeTrace, setNativeProbes } from "./nativeTrace";

const CONTROL_MS = 80;
const TRIAL_TIMEOUT_MS = 3000;
/** Code of an environment fault in a failure reason; mirrors nativeperf.FaultCode. */
const FAULT_FOCUS_LOST = "[fault:focus-lost]";
const SETTLE_MS = 1200;
/** Oracle delivery grace is not a latency sample: only engine timestamps are reported. */
const ORACLE_DELIVERY_MS = 100;
interface Trial {
  command: TrialCommand;
  sample: NativeSample;
  startedAt: number;
  key: string;
  inputData: string;
  changeProof: { from: number; to: number; text: string }[];
  frameScheduled: boolean;
  nativeInputObserved: boolean;
  observedKeydowns: number;
  outsideEditorKeydowns: number;
  outer?: NativeSample["handlers"][number];
  timers: ReturnType<typeof setTimeout>[];
  frames: number[];
  identities: Set<number>;
  /** NULL trials: the exact CodeMirror document at arming; it must still be equal at the end. */
  nullDocBefore?: EditorView["state"]["doc"];
}
export interface NativePerfOptions {
  open: () => SocketLike;
  view: () => EditorView | undefined;
  revision: () => string;
  /** WailsSocket.prototype: its sends are labelled per stream for attribution. */
  streams?: { send(data: unknown): void };
  /**
   * Paths ("meta/claimedAt") the document changed between two revisions, or undefined when they
   * cannot be computed. A NULL trial accepts a revision change only when none touches "text".
   */
  foreignChange?: (before: string, after: string) => string[] | undefined;
}
const touchesText = (path: string) => path === "text" || path.startsWith("text/");
const ownsTyping = () => import.meta.env.VITE_PERF_CM_TYPING === "1";

/** No hooks, observers, edits, globals or controls exist before a native-only Go capability. */
export function startNativePerf(options: NativePerfOptions): () => void {
  const socket = options.open();
  socket.binaryType = "arraybuffer";
  const challenge = Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) =>
    b.toString(16).padStart(2, "0"),
  ).join("");
  const fineClock = new BrowserClock();
  const diagnosticClock = new NativeClock(challenge);
  let capability: NativeCapability | undefined;
  let view: EditorView | undefined;
  let nullTarget: HTMLTextAreaElement | undefined;
  let nullMode = false;
  let active: Trial | undefined;
  let observer: PerformanceObserver | undefined;
  let paintOracle: "event-timing" | "unavailable" = "unavailable";
  let armFrame = 0;
  let armTimer: ReturnType<typeof setTimeout> | undefined;
  let frameTasks: FrameTasks | undefined;
  let busyProbe: BusyProbe | undefined;
  let inputVerified = false;
  let probeWaiting = false;
  let readyRecord: unknown;
  let probeTimer: ReturnType<typeof setTimeout> | undefined;
  const baseline: number[] = [];
  const heartbeatBuffer = new ContinuousHeartbeat();
  let stopped = false;
  const compartment = new Compartment();
  const remove: (() => void)[] = [];
  const seen = new Set<number>();
  const pendingEvents = new WeakMap<Event, NativeSample["handlers"][number]>();
  /** Input events of a probes-off NULL trial: captured without any record. */
  const offEvents = new WeakSet<Event>();
  function send(value: unknown) {
    if (!stopped) socket.send(new TextEncoder().encode(JSON.stringify(value)));
  }
  function environment() {
    return (
      visibleAndFocused(document.visibilityState, document.hasFocus()) &&
      view !== undefined &&
      options.view() === view &&
      view.dom.isConnected &&
      (view.hasFocus || (nullMode && document.activeElement === nullTarget))
    );
  }
  function fail(reason: string) {
    send({
      type: "failure",
      runId: capability?.runId ?? "",
      ...(active ? { id: active.command.id } : {}),
      reason,
    });
    stop(false); // Wails sends asynchronously: Go must receive this failure before close.
  }
  function clearTrial() {
    if (!active) return;
    for (const timer of active.timers) clearTimeout(timer);
    for (const frame of active.frames) cancelAnimationFrame(frame);
    active = undefined;
    setNativeProbes(true);
    if (nullMode) {
      nullMode = false;
      view?.focus();
    }
  }
  function fineNow(): number {
    try {
      return fineClock.read();
    } catch (error) {
      fail(`native handler clock failed: ${String(error)}`);
      return Number.NaN; // Failure is sent before any sample; never use a coarse fallback.
    }
  }
  function stop(closeSocket = true) {
    if (stopped) {
      if (closeSocket) socket.close();
      return;
    }
    stopped = true;
    clearTrial();
    cancelAnimationFrame(armFrame);
    if (armTimer !== undefined) clearTimeout(armTimer);
    clearTimeout(probeTimer);
    observer?.disconnect();
    frameTasks?.close();
    busyProbe?.close();
    for (const off of remove) off();
    nullTarget?.remove();
    // Never change view.state from inside dispatchTransactions/updateListener.
    const attachedView = view;
    queueMicrotask(() => {
      if (!attachedView?.dom.isConnected || options.view() !== attachedView) return;
      try {
        attachedView.dispatch({ effects: compartment.reconfigure([]) });
      } catch {
        /* View already destroyed. */
      }
    });
    if (closeSocket) socket.close();
  }
  function listen(target: EventTarget, name: string, listener: EventListener, capture = false) {
    target.addEventListener(name, listener, capture);
    remove.push(() => target.removeEventListener(name, listener, capture));
  }
  function heartbeat(frame: EarlyFrame) {
    const { start: time, timestamp } = frame;
    if (stopped) return;
    if (!heartbeatBuffer.push(time)) {
      fail("native heartbeat overflow or nonmonotonic clock");
      return;
    }
    if (active) active.sample.heartbeat.push(time);
    baseline.push(timestamp);
    if (baseline.length > 6) baseline.shift();
    const trial = active;
    if (trial && trial.sample.keyDownAt > 0) {
      trial.sample.frames.push(frame);
      if (trial.frameScheduled && trial.sample.docChangedAt > 0 && trial.sample.raf.length < 2)
        trial.sample.raf.push(time);
    }
  }
  function forceLayout(trial: Trial) {
    if (!view) return;
    const original = view.dom.style.paddingLeft;
    const start = performance.now();
    let iterations = 0;
    do {
      view.dom.style.paddingLeft = iterations % 2 === 0 ? "1px" : "2px";
      view.dom.getBoundingClientRect();
      iterations++;
    } while (
      performance.now() - start <
      (["task", "deferred"].includes(trial.command.control) ? 30 : CONTROL_MS)
    );
    view.dom.style.paddingLeft = original;
    view.dom.getBoundingClientRect();
    trial.sample.control = {
      kind: trial.command.control,
      start,
      end: performance.now(),
      iterations,
    };
  }
  function finish(trial: Trial) {
    if (active !== trial || !view) return;
    if (!environment()) {
      fail(document.visibilityState !== "visible" ? FRAME_UNAVAILABLE : FOCUS_INTERRUPTED);
      return;
    }
    const sample = trial.sample;
    if (sample.frames.some((frame) => frame.end === 0)) {
      frameTasks?.post(() => finish(trial));
      return;
    }
    const tail = busyProbe?.boundary();
    if (!tail) {
      fail("uncovered main-thread work: task probe unavailable");
      return;
    }
    sample.busy.push(tail);
    sample.settledAt = tail.end;
    sample.visible = document.visibilityState === "visible";
    sample.focused = document.hasFocus();
    sample.revisionAfter = options.revision();
    sample.lengthAfter = view.state.doc.length;
    sample.changed =
      sample.docChangedAt > 0 &&
      sample.revisionAfter !== sample.revisionBefore &&
      sample.lengthAfter === sample.lengthBefore + 1 &&
      trial.inputData === trial.key &&
      exactTrustedInsertion(trial.key, trial.changeProof);
    if (isNullControl(trial.command.control)) {
      sample.changed = false;
      // The key must land in the textarea and nowhere in the editor document. App work that
      // writes outside "text" (the device claim after a 2 s pause) is not editor input: it is
      // accepted only with its paths recorded, and its cost stays in the NULL sample.
      const revisionChanged = sample.revisionAfter !== sample.revisionBefore;
      const foreign = revisionChanged
        ? options.foreignChange?.(sample.revisionBefore, sample.revisionAfter)
        : [];
      if (revisionChanged) sample.nullForeignPaths = foreign ?? [];
      const editorUnchanged =
        sample.lengthAfter === sample.lengthBefore &&
        trial.nullDocBefore !== undefined &&
        view.state.doc.eq(trial.nullDocBefore);
      const documentIsolated =
        !revisionChanged ||
        (foreign !== undefined && foreign.length > 0 && !foreign.some(touchesText));
      sample.nullInserted =
        trial.inputData === trial.key &&
        nullTarget?.value === trial.key &&
        editorUnchanged &&
        documentIsolated;
    }
    if (!sample.changed && !sample.nullInserted) {
      fail(
        isNullControl(trial.command.control)
          ? `NULL insertion/isolation failed: key=${JSON.stringify(trial.key)}, input=${JSON.stringify(trial.inputData)}, value=${JSON.stringify(nullTarget?.value)}, editorLengthUnchanged=${sample.lengthAfter === sample.lengthBefore}, editorRevisionUnchanged=${sample.revisionAfter === sample.revisionBefore}, documentPaths=${JSON.stringify(sample.nullForeignPaths ?? [])}`
          : "trusted input did not produce the exact CodeMirror insertion",
      );
      return;
    }
    // Probes off: the outer bracket is the only record, and its two reads the only clock reads.
    const outer = sample.handlers[0];
    const incomplete =
      trial.command.control === NULL_OFF
        ? sample.handlers.length !== 1 ||
          outer?.event !== "keystroke-task" ||
          outer.end < outer.start ||
          outer.end === 0 ||
          !outer.fine?.end
        : sample.handlers.filter((h) => ["keydown", "beforeinput", "input"].includes(h.event))
            .length !== (ownsTyping() && !nullMode ? 2 : 3) ||
          !sample.handlers.some(
            (h) => h.event === (sample.nullInserted ? "null-input" : "codemirror-input"),
          ) ||
          sample.handlers.some((h) => h.end < h.start || h.end === 0) ||
          sample.raf.length !== 2;
    if (incomplete) {
      fail("incomplete native event or frame boundaries");
      return;
    }
    send(sample);
    heartbeatBuffer.complete();
    clearTrial();
  }
  function scheduleFrames(trial: Trial) {
    trial.frameScheduled = true;
  }
  function expireTrial(trial: Trial) {
    if (active !== trial) return;
    const points = heartbeatBuffer.snapshot();
    const last = points.at(-1);
    const age = last === undefined ? null : performance.now() - last;
    const diagnostic = `heartbeat callbacks=${points.length}, last age ms=${age}, DOM keydowns=${trial.observedKeydowns}, outside editor=${trial.outsideEditorKeydowns}`;
    if (age === null || age > 50) {
      fail(`${FRAME_UNAVAILABLE} [${diagnostic}]`);
    } else if (trial.sample.keyDownAt === 0) {
      // raf contains only POST-INPUT frames; an empty array is not evidence
      // that the continuous heartbeat stopped. Keep missing native input fatal.
      fail(`Native input missing: no trusted keydown reached CodeMirror; ${diagnostic}`);
    } else {
      fail(
        `Entrée ou image incomplète avant le délai de mesure; ${diagnostic}; input=${trial.nativeInputObserved}, changed=${trial.sample.docChangedAt > 0}, input frames=${trial.sample.raf.length}`,
      );
    }
  }
  function capture(event: Event) {
    // The keystroke starts at this listener's first clock read, before any other work
    // (decision 2026-11-07); event.timeStamp drifts against performance.now() and stays a diagnostic.
    const keyStart = event.type === "keydown" ? performance.now() : 0;
    if (!inputVerified) {
      if (
        probeWaiting &&
        event.type === "keydown" &&
        (event as KeyboardEvent).key === "Shift" &&
        event.isTrusted &&
        view?.contentDOM.contains(event.target as Node) &&
        environment()
      ) {
        inputVerified = true;
        probeWaiting = false;
        clearTimeout(probeTimer);
        send({ type: "input-proof", runId: capability?.runId, trusted: true, key: "Shift" });
        send(readyRecord);
      }
      return;
    }
    if (!active || !(nullMode ? nullTarget : view?.contentDOM)?.contains(event.target as Node))
      return;
    if (!environment()) {
      fail(document.visibilityState !== "visible" ? FRAME_UNAVAILABLE : FOCUS_INTERRUPTED);
      return;
    }
    if (!event.isTrusted) {
      fail("synthetic input is forbidden in the native measurement");
      return;
    }
    const trial = active;
    if (trial.command.control === NULL_OFF) {
      if (event.type === "keydown") acceptKey(trial, event as KeyboardEvent, keyStart);
      else if (acceptInput(trial, event as InputEvent)) offEvents.add(event);
      return;
    }
    const handler = {
      event: event.type,
      start: event.type === "keydown" ? keyStart : performance.now(),
      end: 0,
      trusted: event.isTrusted,
      eventTimeStamp: event.timeStamp,
    };
    trial.sample.handlers.push(handler);
    pendingEvents.set(event, handler);
    if (event.type === "keydown") {
      if (!acceptKey(trial, event as KeyboardEvent, keyStart)) return;
      // Preserve which entries precede input even when WebKit rounds both
      // timestamps into the same millisecond. Never skip a post-input frame.
      trial.sample.heartbeat = heartbeatBuffer.snapshot();
      trial.sample.preInputHeartbeatCount = trial.sample.heartbeat.length;
      trial.sample.baseline = [...baseline];
    } else acceptInput(trial, event as InputEvent);
  }
  /** Opens the outer keystroke bracket at `start`; the fine read is the bracket's second clock read. */
  function acceptKey(trial: Trial, key: KeyboardEvent, start: number): boolean {
    if (
      trial.sample.keyDownAt !== 0 ||
      key.repeat ||
      key.ctrlKey ||
      key.metaKey ||
      key.altKey ||
      !/^[ -~]$/.test(key.key)
    ) {
      fail("invalid native keystroke");
      return false;
    }
    trial.key = key.key;
    trial.sample.keyDownAt = start;
    trial.sample.keyEventTimeStamp = key.timeStamp;
    trial.sample.trusted = true;
    trial.outer = {
      event: "keystroke-task",
      start,
      end: 0,
      fine: { start: fineNow(), end: 0 },
      trusted: true,
    };
    trial.sample.handlers.push(trial.outer);
    return true;
  }
  function acceptInput(trial: Trial, input: InputEvent): boolean {
    if (
      !trial.sample.keyDownAt ||
      input.isComposing ||
      input.inputType !== "insertText" ||
      typeof input.data !== "string"
    ) {
      fail("input did not follow the trusted native key path");
      return false;
    }
    trial.nativeInputObserved = true;
    if (input.type === "input") trial.inputData = input.data;
    return true;
  }
  function bubble(event: Event) {
    if (offEvents.has(event)) {
      if (event.type === "input" && active) afterInput(active);
      return;
    }
    const handler = pendingEvents.get(event);
    if (!handler || !active) return;
    handler.end = performance.now();
    if (!active) return;
    if (event.type === "beforeinput" && ownsTyping() && !nullMode) {
      // Prototype: CodeMirror inserted the key itself (no input event follows). Only a prevented
      // insertText from the trusted key counts; an input event as well makes the sample incomplete.
      if (!event.defaultPrevented) return;
      active.inputData = (event as InputEvent).data ?? "";
      afterInput(active);
      return;
    }
    if (event.type === "input") {
      const trial = active;
      if (nullMode) {
        trial.sample.docChangedAt = handler.end;
        trial.sample.handlers.push({
          event: "null-input",
          start: handler.start,
          end: handler.end,
          trusted: true,
        });
      }
      afterInput(trial);
    }
  }
  /** Closing task of the outer bracket (its end reads), controls and settle timer. */
  function afterInput(trial: Trial) {
    scheduleFrames(trial);
    frameTasks?.post(() => {
      if (active !== trial || !trial.outer?.fine) return;
      // Fine clock at the two outer boundaries only: no synchronous RPC in the input path.
      trial.outer.fine.end = fineNow();
      trial.outer.end = performance.now();
    });
    const control = trial.command.control;
    if (control === "layout")
      trial.frames.push(
        requestAnimationFrame(() => {
          if (active === trial) forceLayout(trial);
        }),
      );
    if (control === "early-raf")
      scheduleEarlierFrame(() => {
        if (active === trial) forceLayout(trial);
      });
    if (control === "measure")
      view?.requestMeasure({
        read: () => null,
        write: () => {
          if (active === trial) forceLayout(trial);
        },
      });
    if (control === "task" || control === "deferred")
      trial.timers.push(
        setTimeout(
          () => {
            if (active === trial) forceLayout(trial);
          },
          control === "deferred" ? 300 : 0,
        ),
      );
    trial.timers.push(setTimeout(() => finish(trial), SETTLE_MS + ORACLE_DELIVERY_MS));
  }
  function arm(command: TrialCommand) {
    if (isNullControl(command.control) && capability?.phase === "warm" && nullTarget) {
      nullTarget.value = "";
      nullMode = true;
      nullTarget.focus();
      nullTarget.setSelectionRange(0, 0);
    }
    if (!capability || !view || !environment()) {
      // The window hidden or unfocused is a host fault (another program took it): the Go verifier turns the code
      // into status `invalid` (plan section 8). The window still focused but the editor not is the page's own
      // doing (the product moved the focus): a failure, without the code.
      fail(
        visibleAndFocused(document.visibilityState, document.hasFocus())
          ? "native editor lost the focus inside the page: the product moved it"
          : `${FAULT_FOCUS_LOST} native editor must be visible and focused before arming`,
      );
      return;
    }
    if (
      active ||
      seen.has(command.id) ||
      (capability.phase === "cold" && (seen.size !== 0 || command.control !== "none"))
    ) {
      fail("duplicate, overlapping or non-cold trial");
      return;
    }
    const heartbeatHistory = heartbeatBuffer.snapshot();
    if (baseline.length < 6) {
      armTimer ??= setTimeout(() => fail(FRAME_UNAVAILABLE), TRIAL_TIMEOUT_MS);
      armFrame = requestAnimationFrame(() => {
        if (!stopped) arm(command);
      });
      return;
    }
    if (armTimer !== undefined) clearTimeout(armTimer);
    armTimer = undefined;
    seen.add(command.id);
    const sample: NativeSample = {
      type: "sample",
      id: command.id,
      runId: capability.runId,
      phase: capability.phase,
      handlers: [],
      listeners: [],
      callbacks: [],
      settledAt: 0,
      busy: [],
      tasks: [],
      frames: [],
      baseline: [...baseline],
      stages: [],
      keyDownAt: 0,
      docChangedAt: 0,
      revisionBefore: options.revision(),
      revisionAfter: "",
      lengthBefore: view.state.doc.length,
      lengthAfter: 0,
      raf: [],
      heartbeat: heartbeatHistory,
      eventTiming: [],
      control: { kind: command.control, start: 0, end: 0, iterations: 0 },
      visible: true,
      focused: true,
      trusted: false,
      changed: false,
    };
    active = {
      command,
      sample,
      startedAt: performance.now(),
      key: "",
      inputData: "",
      changeProof: [],
      frameScheduled: false,
      nativeInputObserved: false,
      observedKeydowns: 0,
      outsideEditorKeydowns: 0,
      timers: [],
      frames: [],
      identities: new Set(),
      ...(nullMode ? { nullDocBefore: view.state.doc } : {}),
    };
    const trial = active;
    active.timers.push(setTimeout(() => expireTrial(trial), TRIAL_TIMEOUT_MS));
    // Overhead arm: every recording hook stays off until this trial is cleared.
    if (command.control === NULL_OFF) setNativeProbes(false);
    send({ type: "armed", id: command.id, runId: capability.runId, visible: true, focused: true });
  }
  function attach() {
    view = options.view();
    if (!view || !environment() || !options.revision()) {
      fail("native CodeMirror document is unavailable, hidden or unfocused");
      return;
    }
    nullTarget = document.createElement("textarea");
    nullTarget.tabIndex = 0;
    nullTarget.dataset.nativePerfNull = "true";
    nullTarget.setAttribute("aria-label", "Native performance null control");
    nullTarget.style.cssText =
      "position:fixed;left:0;top:0;width:1px;height:1px;opacity:0.01;pointer-events:none";
    document.body.append(nullTarget);
    remove.push(
      profileNativeMethods(view, {
        update: "codemirror-update",
        measure: "codemirror-measure",
        requestMeasure: "codemirror-requestMeasure",
      }),
    );
    if (options.streams) remove.push(profileStreamSends(options.streams));
    remove.push(
      registerNativeStages((stage) => {
        if (active?.sample.trusted) active.sample.stages?.push(stage);
      }),
      registerNativeRecorder(view, (transactions, forward) => {
        const trial = active;
        if (
          !trial?.sample.trusted ||
          !trial.nativeInputObserved ||
          !transactions.some((tr) => tr.isUserEvent("input"))
        ) {
          forward();
          return;
        }
        const trace = enterNativeTrace();
        const handler: NativeSample["handlers"][number] = {
          event: "codemirror-input",
          start: performance.now(),
          end: 0,
          trusted: true,
          call: trace.call,
          parent: trace.parent,
        };
        trial.sample.handlers.push(handler);
        try {
          forward();
        } finally {
          handler.end = performance.now();
          trace.close();
        }
      }),
    );
    view.dispatch({
      effects: StateEffect.appendConfig.of(
        compartment.of(
          EditorView.updateListener.of((update) => {
            if (!update.docChanged) return;
            if (!active) {
              fail("document changed outside the armed native input");
              return;
            }
            if (!active.sample.keyDownAt) {
              fail("document changed outside the armed native input");
              return;
            }
            if (nullMode) {
              fail("NULL trial changed the CodeMirror document");
              return;
            }
            active.sample.docChangedAt = performance.now();
            update.changes.iterChanges((from, to, _fromB, _toB, insert) =>
              active?.changeProof.push({ from, to, text: insert.toString() }),
            );
          }),
        ),
      ),
    });
    // Label each ViewPlugin's update (status/gutter/presence/... work inside
    // codemirror-update). Its minified source prefix is mapped back offline.
    const pluginSources: Record<string, string> = {};
    const instances = (view as unknown as { plugins?: { value?: object | null }[] }).plugins ?? [];
    instances.forEach((instance, index) => {
      const value = instance.value as { update?: unknown; constructor?: { name?: string } } | null;
      if (!value || typeof value.update !== "function") return;
      const label = `cm-plugin:${index}:${value.constructor?.name || "anonymous"}`;
      pluginSources[label] = String(value.update).slice(0, 160);
      remove.push(profileNativeMethods(value, { update: label }));
    });
    const early = subscribeNativeEarly({
      capture,
      bubble,
      frame: heartbeat,
      frameEnd: () => {},
      task: (span) => {
        // A probes-off trial records no callback, not even the arming task that began before it.
        if (active && active.command.control !== NULL_OFF) {
          active.sample.tasks.push({ start: span.start, end: span.end });
          if (active.sample.trusted) {
            const { callback } = span;
            active.sample.listeners?.push({
              start: span.start,
              end: span.end,
              call: span.call,
              parent: span.parent,
              id: callback.id,
            });
            // Each identity (with its registration frames) is sent once per sample.
            if (!active.identities.has(callback.id)) {
              active.identities.add(callback.id);
              active.sample.callbacks?.push(callback);
            }
          }
        }
      },
    });
    remove.push(() => early.close());
    for (const [target, kind] of [
      [view.contentDOM, "handler"],
      [document, "document"],
    ] as const)
      listen(target, ownsTyping() ? "beforeinput" : "input", (event) => {
        if (
          active?.command.control === kind &&
          event.isTrusted &&
          view?.contentDOM.contains(event.target as Node)
        )
          forceLayout(active);
      });
    // Diagnostics only: never treat a window-level or synthetic event as an
    // accepted editor input. This distinguishes XTest delivery from DOM focus.
    listen(
      window,
      "keydown",
      (event) => {
        if (!active) return;
        active.observedKeydowns++;
        if (!view?.contentDOM.contains(event.target as Node)) active.outsideEditorKeydowns++;
      },
      true,
    );
    listen(document, "visibilitychange", () => {
      if (!environment())
        fail(document.visibilityState !== "visible" ? FRAME_UNAVAILABLE : FOCUS_INTERRUPTED);
    });
    listen(window, "blur", () => {
      if (inputVerified) fail(FOCUS_INTERRUPTED);
    });
    try {
      if (PerformanceObserver.supportedEntryTypes.includes("event")) {
        observer = new PerformanceObserver((list) => {
          if (!active) return;
          for (const entry of list.getEntries()) {
            const raw = rawEventTiming(entry);
            if (raw && raw.startTime >= active.startedAt) active.sample.eventTiming.push(raw);
          }
        });
        observer.observe({
          type: "event",
          buffered: true,
          durationThreshold: 16,
        } as PerformanceObserverInit);
        paintOracle = "event-timing";
      }
    } catch {
      observer?.disconnect();
      observer = undefined;
      paintOracle = "unavailable";
    }
    frameTasks = new FrameTasks();
    type Candidate = ReturnType<NativeClock["observe"]> | { source: string; error: string };
    let clock: ReturnType<typeof observeClock> & {
      precise: ReturnType<BrowserClock["observe"]>;
      candidates: Candidate[];
    };
    try {
      const browser = observeClock();
      let candidate: Candidate;
      try {
        candidate = diagnosticClock.observe(browser.resolutionMs);
      } catch (error) {
        candidate = { source: "native-monotonic", error: String(error) };
      }
      clock = {
        ...browser,
        precise: fineClock.observe(browser.resolutionMs),
        candidates: [candidate],
        refreshHz: capability?.refreshHz ?? 0,
      };
    } catch (error) {
      fail(`native handler clock preflight failed: ${String(error)}`);
      return;
    }
    busyProbe = new BusyProbe((span) => {
      if (active) active.sample.busy.push(span);
    });
    readyRecord = {
      type: "ready",
      visible: true,
      focused: true,
      userAgent: navigator.userAgent,
      clock,
      paintOracle,
      earlyCapture: early.beforeApp,
      inputVerified: true,
      pluginSources,
    };
    if (capability?.phase === "cold") {
      if (!capability.inputPermissionVerified) {
        fail("cold XTest permission was not preflighted by the driver; no warm-up input permitted");
        return;
      }
      inputVerified = true;
      send(readyRecord);
      return;
    }
    probeTimer = setTimeout(() => {
      const last = heartbeatBuffer.snapshot().at(-1);
      fail(
        last === undefined || performance.now() - last > 50
          ? FRAME_UNAVAILABLE
          : "XTest input delivery/permission not validated: no trusted Shift key reached the measured native window; grant GNOME remote-interaction permission and rerun",
      );
    }, TRIAL_TIMEOUT_MS);
    probeWaiting = true;
    send({ type: "probe-armed", runId: capability?.runId, visible: true, focused: true });
  }
  socket.onopen = () => send({ type: "hello", protocol: 1, challenge });
  socket.onmessage = (event) => {
    if (stopped) return;
    let value: unknown;
    try {
      value = JSON.parse(
        typeof event.data === "string" ? event.data : new TextDecoder().decode(event.data),
      );
    } catch {
      fail("invalid native harness frame");
      return;
    }
    if (!capability) {
      capability = authorizeNativeRun(value, challenge);
      if (!capability) {
        fail("native-only runtime capability was not granted");
        return;
      }
      attach();
      return;
    }
    const command = parseTrial(value);
    if (command) {
      arm(command);
      return;
    }
    if (value && typeof value === "object" && (value as { type?: unknown }).type === "complete") {
      stop();
      return;
    }
    fail("unexpected native harness command");
  };
  socket.onclose = () => stop(false);
  return stop;
}
