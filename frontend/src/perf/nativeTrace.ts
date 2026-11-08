/** Synchronous nesting only. No Promise wall time is called CPU time. */
let sequence = 0;
let current = 0;
export function enterNativeTrace() {
  const parent = current;
  const call = ++sequence;
  current = call;
  return {
    call,
    parent,
    close: () => {
      current = parent;
    },
  };
}
/** The synchronously executing recorded call (0 = none): who scheduled a callback. */
export const currentNativeCall = () => current;
/**
 * Recording hooks ("sondes") on or off. Off only during an instrument-overhead NULL trial: the
 * wrapped callbacks, stages and stream sends then run without clock reads, trace or records.
 */
let probes = true;
export const nativeProbesOn = () => probes;
export function setNativeProbes(on: boolean): void {
  probes = on;
}
