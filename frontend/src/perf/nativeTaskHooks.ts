import { currentNativeCall, enterNativeTrace, nativeProbesOn } from "./nativeTrace";
export interface CallbackIdentity {
  id: number;
  target: string;
  event: string;
  capture: boolean;
  name: string;
  registration: string;
  /** Recorded call that scheduled this timer/microtask/rAF (0 = none). */
  scheduledBy?: number;
}
/** Registration-time only (never inside an executing callback): a few caller frames. */
const registrationStack = () => (new Error().stack ?? "").split("\n").slice(0, 8).join("\n");
export interface NativeTaskSpan {
  start: number;
  end: number;
  call: number;
  parent: number;
  callback: CallbackIdentity;
}
type Span = NativeTaskSpan;
const internal = new WeakSet<object>();
export function internalCallback<T extends object>(callback: T): T {
  internal.add(callback);
  return callback;
}

/** Record actual callback intervals; idle gaps are never summed as frame work.
 * Installed before App imports, but the recorder is inert until native authority.
 */
export function installNativeTaskHooks(record: (span: Span) => void): () => void {
  let next = 0;
  function wrap<T>(callback: T, detail: Partial<CallbackIdentity> = {}): T {
    if (typeof callback !== "function" || internal.has(callback)) return callback;
    const identity: CallbackIdentity = {
      id: ++next,
      target: "scheduler",
      event: "callback",
      capture: false,
      name: callback.name || "anonymous",
      registration: "",
      ...detail,
    };
    return function (this: unknown, ...args: unknown[]) {
      if (!nativeProbesOn()) return Reflect.apply(callback, this, args);
      const trace = enterNativeTrace();
      const start = performance.now();
      try {
        return Reflect.apply(callback, this, args);
      } finally {
        const end = performance.now();
        trace.close();
        record({ start, end, call: trace.call, parent: trace.parent, callback: identity });
      }
    } as unknown as T;
  }
  const timeout = window.setTimeout;
  const interval = window.setInterval;
  const raf = window.requestAnimationFrame;
  const microtask = window.queueMicrotask;
  // DOM timers accept string handlers too; preserve the original API without
  // evaluating strings here. Ambient Node timer overloads also add __promisify__.
  window.setTimeout = ((callback: TimerHandler, delay?: number, ...args: unknown[]) =>
    Reflect.apply(timeout, window, [
      wrap(callback, { event: "setTimeout", scheduledBy: currentNativeCall() }),
      delay,
      ...args,
    ])) as unknown as typeof window.setTimeout;
  window.setInterval = ((callback: TimerHandler, delay?: number, ...args: unknown[]) =>
    Reflect.apply(interval, window, [
      wrap(callback, { event: "setInterval", scheduledBy: currentNativeCall() }),
      delay,
      ...args,
    ])) as unknown as typeof window.setInterval;
  window.requestAnimationFrame = (callback) =>
    raf.call(
      window,
      wrap(callback, { event: "requestAnimationFrame", scheduledBy: currentNativeCall() }),
    );
  window.queueMicrotask = (callback) =>
    microtask.call(
      window,
      wrap(callback, { event: "queueMicrotask", scheduledBy: currentNativeCall() }),
    );
  // Observer callbacks (CodeMirror's DOM observer reads WebKit's text edit) are App work too.
  const observers: [string, unknown][] = [];
  for (const name of ["MutationObserver", "ResizeObserver", "IntersectionObserver"] as const) {
    const Original = (window as unknown as Record<string, unknown>)[name] as
      | (new (
          callback: (...args: unknown[]) => void,
          ...rest: unknown[]
        ) => object)
      | undefined;
    if (typeof Original !== "function") continue;
    observers.push([name, Original]);
    (window as unknown as Record<string, unknown>)[name] = class extends Original {
      constructor(callback: (...args: unknown[]) => void, ...rest: unknown[]) {
        super(
          wrap(callback, { target: name, event: "observer", registration: registrationStack() }),
          ...rest,
        );
      }
    };
  }
  const add = EventTarget.prototype.addEventListener;
  const remove = EventTarget.prototype.removeEventListener;
  const listeners = new WeakMap<EventTarget, Map<string, WeakMap<object, EventListener>>>();
  function listenerFor(
    target: EventTarget,
    type: string,
    listener: EventListenerOrEventListenerObject,
    options?: boolean | AddEventListenerOptions,
  ): EventListener {
    const key = `${type}:${typeof options === "boolean" ? options : !!options?.capture}`;
    let groups = listeners.get(target);
    if (!groups) {
      groups = new Map();
      listeners.set(target, groups);
    }
    let group = groups.get(key);
    if (!group) {
      group = new WeakMap();
      groups.set(key, group);
    }
    let wrapped = group.get(listener);
    if (!wrapped) {
      const callback =
        typeof listener === "function" ? listener : (event: Event) => listener.handleEvent(event);
      const owner =
        target === window
          ? "window"
          : target === document
            ? "document"
            : target instanceof Element
              ? target.classList.contains("cm-content")
                ? "view.contentDOM"
                : target.closest(".cm-editor")
                  ? `view.${target.className || target.tagName}`
                  : target.tagName.toLowerCase()
              : target.constructor.name;
      wrapped = wrap(callback, {
        target: owner,
        event: type,
        capture: typeof options === "boolean" ? options : !!options?.capture,
        // Registration only: never create stacks inside an executing listener.
        registration: registrationStack(),
      });
      group.set(listener, wrapped);
    }
    return wrapped;
  }
  EventTarget.prototype.addEventListener = function (type, listener, options) {
    add.call(this, type, listener ? listenerFor(this, type, listener, options) : listener, options);
  };
  EventTarget.prototype.removeEventListener = function (type, listener, options) {
    remove.call(
      this,
      type,
      listener ? listenerFor(this, type, listener, options) : listener,
      options,
    );
    if (listener) remove.call(this, type, listener, options); // Also remove a pre-bootstrap registration.
  };
  const undoMessages: (() => void)[] = [];
  const prototypes = [
    MessagePort.prototype,
    ...(typeof Worker !== "undefined" ? [Worker.prototype] : []),
    ...(typeof WebSocket !== "undefined" ? [WebSocket.prototype] : []),
  ];
  for (const prototype of prototypes) {
    const message = Object.getOwnPropertyDescriptor(prototype, "onmessage");
    if (!message?.set) continue;
    const assigned = new WeakMap<object, unknown>();
    Object.defineProperty(prototype, "onmessage", {
      ...message,
      get() {
        return assigned.has(this) ? assigned.get(this) : (message.get?.call(this) ?? null);
      },
      set(callback) {
        assigned.set(this, callback);
        message.set?.call(
          this,
          wrap(callback, { target: prototype.constructor.name, event: "message" }),
        );
      },
    });
    undoMessages.push(() => Object.defineProperty(prototype, "onmessage", message));
  }
  return () => {
    window.setTimeout = timeout;
    window.setInterval = interval;
    window.requestAnimationFrame = raf;
    window.queueMicrotask = microtask;
    EventTarget.prototype.addEventListener = add;
    EventTarget.prototype.removeEventListener = remove;
    for (const undo of undoMessages) undo();
    for (const [name, original] of observers)
      (window as unknown as Record<string, unknown>)[name] = original;
  };
}
