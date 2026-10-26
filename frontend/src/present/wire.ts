/**
 * What a presenter says to the others, as ephemeral messages of the project document (never
 * saved: they ride the same channel as the cursors). Whatever arrives from the network is checked
 * here before anything looks at it: a damaged or hostile message is `undefined`, never a throw.
 */

/** The key of the ephemeral message that carries a presentation message. */
export const MARKER = "__present";

export type ScreenMode = "show" | "black" | "white";

/** What the pilot shows: the whole of it, repeated now and then so that anyone arriving catches up. */
export interface ViewState {
  slide: number;
  step: number;
  screen: ScreenMode;
}

export type WireMessage =
  | ({ t: "state"; epoch: number; seq: number } & ViewState)
  /** The laser pointer, as fractions of the slide (0 to 1); null when it is off. */
  | { t: "laser"; epoch: number; x: number | null; y: number | null }
  /** "I am the pilot now." */
  | { t: "take"; epoch: number }
  /** "Who pilots?": a device that has just arrived; the pilot answers with its state at once. */
  | { t: "hello" }
  /** "May I have the controls?" */
  | { t: "ask" }
  /** The pilot hands the controls to `to`. */
  | { t: "give"; to: string; epoch: number }
  | { t: "deny"; to: string }
  /** The pilot stops presenting. */
  | { t: "end"; epoch: number };

export const MAX_EPOCH = 2 ** 31;
export const MAX_SLIDE = 100_000;
const MAX_PEER = 64;

const isObject = (v: unknown): v is Record<string, unknown> =>
  typeof v === "object" && v !== null && !Array.isArray(v);

const int = (v: unknown, max: number): number | undefined =>
  typeof v === "number" && Number.isInteger(v) && v >= 0 && v <= max ? v : undefined;

const fraction = (v: unknown): number | undefined =>
  typeof v === "number" && Number.isFinite(v) && v >= 0 && v <= 1 ? v : undefined;

function peer(v: unknown): string | undefined {
  if (typeof v !== "string" || v.length === 0 || v.length > MAX_PEER) return undefined;
  for (let i = 0; i < v.length; i++) {
    const c = v.charCodeAt(i);
    if (c <= 0x20 || c > 0x7e) return undefined;
  }
  return v;
}

/** Reads the value of the `__present` key of an ephemeral message; undefined if it is not a valid message. */
export function parseWire(v: unknown): WireMessage | undefined {
  if (!isObject(v)) return undefined;
  switch (v.t) {
    case "state": {
      const epoch = int(v.epoch, MAX_EPOCH);
      const seq = int(v.seq, Number.MAX_SAFE_INTEGER);
      const slide = int(v.slide, MAX_SLIDE);
      const step = int(v.step, MAX_SLIDE);
      if (epoch === undefined || seq === undefined || !slide || step === undefined)
        return undefined;
      if (v.screen !== "show" && v.screen !== "black" && v.screen !== "white") return undefined;
      return { t: "state", epoch, seq, slide, step, screen: v.screen };
    }
    case "laser": {
      const epoch = int(v.epoch, MAX_EPOCH);
      if (epoch === undefined) return undefined;
      if (v.x === null && v.y === null) return { t: "laser", epoch, x: null, y: null };
      const x = fraction(v.x);
      const y = fraction(v.y);
      return x === undefined || y === undefined ? undefined : { t: "laser", epoch, x, y };
    }
    case "take":
    case "end": {
      const epoch = int(v.epoch, MAX_EPOCH);
      return epoch === undefined ? undefined : { t: v.t, epoch };
    }
    case "ask":
      return { t: "ask" };
    case "hello":
      return { t: "hello" };
    case "give": {
      const epoch = int(v.epoch, MAX_EPOCH);
      const to = peer(v.to);
      return epoch === undefined || to === undefined ? undefined : { t: "give", to, epoch };
    }
    case "deny": {
      const to = peer(v.to);
      return to === undefined ? undefined : { t: "deny", to };
    }
  }
  return undefined;
}
