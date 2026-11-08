/** Native-only, raw performance protocol. A Vite flag is never authorization. */
export interface NativeCapability {
  inputPermissionVerified?: boolean;
  refreshHz: number;
  type: "capability";
  protocol: 1;
  challenge: string;
  engine: "WebKitGTK";
  native: true;
  runId: string;
  phase: "cold" | "warm";
  sampleCount: number;
  replicateIndex?: number;
  replicateCount?: number;
}
export const NATIVE_CONTROLS = [
  "handler",
  "layout",
  "early-raf",
  "measure",
  "document",
  "task",
  "deferred",
] as const;
/** NULL trial with every recording hook off; only the two outer clock reads remain. */
export const NULL_OFF = "null-off";
export type ControlKind = "none" | "null" | typeof NULL_OFF | (typeof NATIVE_CONTROLS)[number];
export const isNullControl = (kind: ControlKind) => kind === "null" || kind === NULL_OFF;
export interface TrialCommand {
  type: "trial";
  id: number;
  control: ControlKind;
}
export interface RawEventTiming {
  name: string;
  startTime: number;
  duration: number;
  processingStart: number;
  processingEnd: number;
  interactionId?: number;
}
export interface HandlerTiming {
  call?: number;
  parent?: number;
  fine?: { start: number; end: number };
  event: string;
  start: number;
  end: number;
  trusted: boolean;
  eventTimeStamp?: number;
}
export interface NativeSample {
  /** Recorded App callback spans after the trusted keydown; `id` names a `callbacks` entry. */
  listeners?: { start: number; end: number; call: number; parent: number; id: number }[];
  callbacks?: import("./nativeTaskHooks").CallbackIdentity[];
  preInputHeartbeatCount?: number;
  /** Diagnostic only: the keydown's event.timeStamp (its time base drifts against performance.now()). */
  keyEventTimeStamp?: number;
  nullInserted?: boolean;
  /** NULL only: document paths changed during the trial by App work outside "text". */
  nullForeignPaths?: string[];
  tasks: { start: number; end: number }[];
  settledAt: number;
  busy: { start: number; end: number }[];
  frames: { start: number; end: number; timestamp: number }[];
  baseline: number[];
  stages?: HandlerTiming[];
  type: "sample";
  id: number;
  runId: string;
  phase: "cold" | "warm";
  handlers: HandlerTiming[];
  /** Keystroke start: the first performance.now() read in the window capture keydown listener. */
  keyDownAt: number;
  docChangedAt: number;
  revisionBefore: string;
  revisionAfter: string;
  lengthBefore: number;
  lengthAfter: number;
  raf: number[];
  heartbeat: number[];
  eventTiming: RawEventTiming[];
  control: { kind: ControlKind; start: number; end: number; iterations: number };
  visible: boolean;
  focused: boolean;
  trusted: boolean;
  changed: boolean;
}
const object = (value: unknown): Record<string, unknown> | undefined =>
  value !== null && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
export function authorizeNativeRun(
  value: unknown,
  challenge: string,
): NativeCapability | undefined {
  const cap = object(value);
  if (
    cap?.type !== "capability" ||
    cap.protocol !== 1 ||
    cap.challenge !== challenge ||
    cap.native !== true ||
    cap.engine !== "WebKitGTK" ||
    typeof cap.runId !== "string" ||
    !/^[A-Za-z0-9_-]{1,128}$/.test(cap.runId) ||
    !Number.isSafeInteger(cap.sampleCount) ||
    typeof cap.refreshHz !== "number" ||
    !Number.isFinite(cap.refreshHz) ||
    cap.refreshHz <= 0
  )
    return;
  if (
    cap.phase === "warm" &&
    (cap.sampleCount as number) >= 30 &&
    (cap.sampleCount as number) <= 1000
  )
    return cap as unknown as NativeCapability;
  if (
    cap.phase === "cold" &&
    cap.sampleCount === 1 &&
    Number.isSafeInteger(cap.replicateIndex) &&
    Number.isSafeInteger(cap.replicateCount) &&
    (cap.replicateCount as number) >= 30 &&
    (cap.replicateIndex as number) >= 1 &&
    (cap.replicateIndex as number) <= (cap.replicateCount as number)
  )
    return cap as unknown as NativeCapability;
}
export function parseTrial(value: unknown): TrialCommand | undefined {
  const trial = object(value);
  return trial?.type === "trial" &&
    Number.isSafeInteger(trial.id) &&
    (trial.id as number) > 0 &&
    ["none", "null", NULL_OFF, ...NATIVE_CONTROLS].includes(trial.control as string)
    ? (trial as unknown as TrialCommand)
    : undefined;
}
/** Preserve engine timestamps and quantization verbatim; the Go oracle validates edit correlation. */
export function rawEventTiming(value: unknown): RawEventTiming | undefined {
  const entry = object(value);
  if (
    entry?.entryType !== "event" ||
    !["beforeinput", "input"].includes(entry.name as string) ||
    ![entry.startTime, entry.duration, entry.processingStart, entry.processingEnd].every(
      (v) => typeof v === "number" && Number.isFinite(v),
    )
  )
    return;
  return {
    name: entry.name as string,
    startTime: entry.startTime as number,
    duration: entry.duration as number,
    processingStart: entry.processingStart as number,
    processingEnd: entry.processingEnd as number,
    ...(typeof entry.interactionId === "number" ? { interactionId: entry.interactionId } : {}),
  };
}
export function exactTrustedInsertion(
  key: string,
  changes: readonly { from: number; to: number; text: string }[],
): boolean {
  const change = changes[0];
  return (
    /^[ -~]$/.test(key) &&
    changes.length === 1 &&
    change !== undefined &&
    change.from === change.to &&
    change.text === key
  );
}
export const visibleAndFocused = (visibility: string, focused: boolean) =>
  visibility === "visible" && focused;
