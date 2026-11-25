/**
 * Which keybindings are in force on a system (ADR 0027, « Commands, menus and keybindings »;
 * spec §8.2 K-6). A conflict is refused when the registry is built, never settled by order: two
 * built-ins on one chord in overlapping scopes refuse the later one, naming both, unless the
 * narrower one names the other in `masks`; a binding of any other origin stays inactive and is
 * reported (ND-3). A chord another owner binds (CodeMirror, the grid, keytables.ts) is taken only
 * by naming it in `masks`. The person's own bindings win over the defaults and are checked
 * like them.
 */
import { chordProblem, type Delegated, type Scope, type System } from "./keys";
import { OWNED_KEYS } from "./keytables";
import type { KeybindingContribution, Origin } from "./manifest";
import type { When } from "./when";

/** A keybinding as the registry holds it: its manifest's fields and who contributed it. */
export interface Binding extends KeybindingContribution {
  readonly extension: string;
  readonly folder: string;
  readonly origin: Origin;
}

/** One chord in force: what it runs, where, and when. */
export interface Active {
  readonly command: string;
  readonly chord: string;
  readonly scope: Scope;
  readonly when?: When | undefined;
  /** Shown, never captured: the system's own, or a widget's (undo). */
  readonly delegated?: Delegated | undefined;
  /** The person's own binding. */
  readonly mine?: true;
}

export interface Problem {
  readonly folder: string;
  readonly reason: string;
  readonly with?: readonly string[];
}

/** The person's binding of a command on this device; `key: null` removes its defaults. */
export interface UserBinding {
  readonly command: string;
  readonly key: string | null;
}

export interface Resolution {
  readonly active: readonly Active[];
  /** Bindings of other origins left out by a conflict (ND-3): listed, never in force. */
  readonly inactive: readonly Problem[];
  /** Built-in bindings refused: a conflict, or an owned key met without naming it. */
  readonly refused: readonly Problem[];
  /** Defaults switched off by one of the person's bindings, and by which command. */
  readonly replaced: readonly {
    readonly command: string;
    readonly chord: string;
    readonly by: string;
  }[];
  /** The person's bindings refused, and why. */
  readonly mine: readonly {
    readonly command: string;
    readonly key: string;
    readonly reason: string;
  }[];
  /** `masks` entries that hid something on this system, as `<binding index>:<name>`. */
  readonly masked: ReadonlySet<string>;
}

/** `window` overlaps every scope; two surfaces overlap only themselves. */
export const overlaps = (a: Scope, b: Scope): boolean =>
  a === "window" || b === "window" || a === b;

/** Whether `a` and `b` on one chord are resolved by the narrower one naming the other. */
function masked(a: Binding, b: Binding): string | undefined {
  if (a.origin !== "builtin" || b.origin !== "builtin" || a.scope === b.scope) return undefined;
  if (a.scope === "window" && b.masks.includes(a.command)) return a.command;
  if (b.scope === "window" && a.masks.includes(b.command)) return b.command;
  return undefined;
}

/**
 * The bindings in force on `system`: the defaults, each checked against those before it and
 * against the owned keys, then the person's own (`user`), which win.
 */
export function resolve(
  bindings: readonly Binding[],
  system: System,
  user: readonly UserBinding[] = [],
  known: ReadonlySet<string> = new Set(bindings.map((b) => b.command)),
): Resolution {
  const active: (Active & { readonly at: number })[] = [];
  const inactive: Problem[] = [];
  const refused: Problem[] = [];
  const used = new Set<string>();
  // Built-ins first: a plugin never takes a chord from the core, whatever the load order.
  const order = bindings
    .map((b, at) => ({ b, at }))
    .sort((x, y) => Number(x.b.origin !== "builtin") - Number(y.b.origin !== "builtin"));
  for (const { b, at } of order) {
    for (const chord of b[system]) {
      if (b.delegated) {
        active.push({
          command: b.command,
          chord,
          scope: b.scope,
          when: b.when,
          delegated: b.delegated,
          at,
        });
        continue;
      }
      const reject = (reason: string, names: readonly string[]) =>
        (b.origin === "builtin" ? refused : inactive).push(
          Object.freeze({ folder: b.folder, reason, with: Object.freeze([...names]) }),
        );
      const owned = OWNED_KEYS.filter(
        (k) => overlaps(k.scope, b.scope) && k[system].includes(chord),
      );
      const unnamed = owned.find((k) => b.origin !== "builtin" || !b.masks.includes(k.name));
      for (const k of owned) if (b.masks.includes(k.name)) used.add(`${at}:${k.name}`);
      if (unnamed) {
        reject(`meets ${unnamed.name} on ${chord} (${system})`, [unnamed.name, b.command]);
        continue;
      }
      let clash: (typeof active)[number] | undefined;
      for (const a of active) {
        if (a.delegated || a.chord !== chord || !overlaps(a.scope, b.scope)) continue;
        const hidden = masked(bindings[a.at] as Binding, b);
        if (hidden === undefined) {
          clash = a;
          break;
        }
        used.add(`${hidden === a.command ? at : a.at}:${hidden}`);
      }
      if (clash) {
        reject(`conflict on ${chord} (${system})`, [clash.command, b.command]);
        continue;
      }
      active.push({ command: b.command, chord, scope: b.scope, when: b.when, at });
    }
  }
  return applyMine(active, bindings, system, user, known, { inactive, refused, used });
}

function applyMine(
  defaults: readonly (Active & { readonly at: number })[],
  bindings: readonly Binding[],
  system: System,
  user: readonly UserBinding[],
  commands: ReadonlySet<string>,
  rest: { inactive: Problem[]; refused: Problem[]; used: Set<string> },
): Resolution {
  // Hers takes the scope and the `when` of the command's own default binding.
  const defaultOf = (command: string) => bindings.find((b) => b.command === command);
  const mine: Active[] = [];
  const refusedMine: { command: string; key: string; reason: string }[] = [];
  // Only a binding of hers that is in force (or her removal) takes a command's defaults away.
  const rebound = new Set<string>();
  for (const u of user) {
    if (u.key === null) {
      rebound.add(u.command);
      continue;
    }
    const scope = defaultOf(u.command)?.scope ?? "window";
    const problem = !commands.has(u.command)
      ? "unknown-command"
      : (chordProblem(u.key, system, scope) ??
        (mine.some((m) => m.chord === u.key && overlaps(m.scope, scope)) ? "taken" : undefined));
    if (problem) refusedMine.push({ command: u.command, key: u.key, reason: problem });
    else {
      rebound.add(u.command);
      mine.push({
        command: u.command,
        chord: u.key,
        scope,
        when: defaultOf(u.command)?.when,
        mine: true,
      });
    }
  }
  const replaced: { command: string; chord: string; by: string }[] = [];
  const kept = defaults.filter((a) => {
    if (rebound.has(a.command)) return false;
    const by = mine.find((m) => m.chord === a.chord && overlaps(m.scope, a.scope));
    if (by) replaced.push({ command: a.command, chord: a.chord, by: by.command });
    return by === undefined;
  });
  return Object.freeze({
    active: Object.freeze([...kept.map(({ at: _, ...a }) => Object.freeze(a)), ...mine]),
    inactive: Object.freeze(rest.inactive),
    refused: Object.freeze(rest.refused),
    replaced: Object.freeze(replaced),
    mine: Object.freeze(refusedMine),
    masked: rest.used,
  });
}

/**
 * The person's bindings as stored on this device (`inkwell.keybindings`): a JSON list of
 * `{command, key}`. Anything else leaves the defaults in force: the whole list is ignored.
 */
export function readUserBindings(text: string | null): readonly UserBinding[] {
  if (text === null) return [];
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return [];
  }
  if (!Array.isArray(raw) || raw.length > 256) return [];
  const out: UserBinding[] = [];
  for (const x of raw) {
    if (x === null || typeof x !== "object" || Array.isArray(x)) return [];
    const keys = Object.keys(x);
    if (keys.some((k) => k !== "command" && k !== "key")) return [];
    const { command, key } = x as Record<string, unknown>;
    if (typeof command !== "string" || (key !== null && typeof key !== "string")) return [];
    out.push(Object.freeze({ command, key }));
  }
  return Object.freeze(out);
}

/**
 * The `masks` entries that hide nothing on any system, as `[command, name]`: a static check of the
 * built-in manifests, run by C-6 (keytables.test.ts) rather than at every start.
 */
export function unusedMasks(bindings: readonly Binding[]): [string, string][] {
  const used = new Set(
    (["linux", "mac", "win"] as const).flatMap((s) => [...resolve(bindings, s).masked]),
  );
  return bindings.flatMap((b, at) =>
    b.masks
      .filter((name) => !used.has(`${at}:${name}`))
      .map((name): [string, string] => [b.command, name]),
  );
}
