import type { Transaction } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import { enterNativeTrace, nativeProbesOn } from "./nativeTrace";

type Recorder = (transactions: readonly Transaction[], forward: () => void) => void;
// Only the perf build references this registry; a flag alone never registers a recorder.
const recorders = /* @__PURE__ */ new WeakMap<EditorView, Recorder>();
type Stage = { event: string; start: number; end: number; trusted: boolean; call?: number; parent?: number };
let stageRecorder: ((stage: Stage) => void) | undefined;
export function registerNativeStages(recorder: (stage: Stage) => void): () => void {
  if (stageRecorder) throw new Error("Native stage recorder already registered");
  stageRecorder = recorder;
  return () => {
    if (stageRecorder === recorder) stageRecorder = undefined;
  };
}
export function profileNativeStage<T>(event: string, work: () => T): T {
  const recorder = stageRecorder;
  if (!recorder || !nativeProbesOn()) return work();
  const trace = enterNativeTrace();
  const start = performance.now();
  try {
    return work();
  } finally {
    const end = performance.now();
    trace.close();
    recorder({ event, start, end, trusted: true, call: trace.call, parent: trace.parent });
  }
}

/** Preserve method receiver, arguments, result and exceptions; perf capability
 * installs these wrappers only for the measured view, and restores them. */
export function profileNativeMethods(target: object, methods: Record<string,string>): () => void {
  const undo: (() => void)[] = [];
  for (const [name,label] of Object.entries(methods)) {
    const descriptor = Object.getOwnPropertyDescriptor(target,name);
    const method = Reflect.get(target,name);
    if (typeof method !== "function") continue;
    Object.defineProperty(target,name,{ configurable:true,writable:true,value:function(this:unknown,...args:unknown[]) {
      return profileNativeStage(label,()=>Reflect.apply(method,this,args));
    }});
    undo.push(()=>{ if(descriptor) Object.defineProperty(target,name,descriptor); else Reflect.deleteProperty(target,name); });
  }
  return ()=>{ for(const restore of undo.reverse()) restore(); };
}
/** Labels every Wails stream send (`stream-send:<name>`): Automerge sync, storage, LSP, preview. */
export function profileStreamSends(prototype: { send(data: unknown): void }): () => void {
  const send = prototype.send;
  prototype.send = function (this: { name?: string }, data: unknown) {
    return profileNativeStage(`stream-send:${this.name ?? "?"}`, () => Reflect.apply(send, this, [data]));
  };
  return () => {
    prototype.send = send;
  };
}
export function dispatchNativeTransactions(
  transactions: readonly Transaction[],
  view: EditorView,
): void {
  const recorder = recorders.get(view);
  if (recorder) recorder(transactions, () => view.update(transactions));
  else view.update(transactions);
}
export function registerNativeRecorder(view: EditorView, recorder: Recorder): () => void {
  if (recorders.has(view)) throw new Error("Native recorder already registered for this editor");
  recorders.set(view, recorder);
  return () => recorders.delete(view);
}
