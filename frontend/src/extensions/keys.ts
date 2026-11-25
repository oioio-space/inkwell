/**
 * Chords (spec §8.2, ADR 0027 « Commands, menus and keybindings »): how a manifest writes one,
 * which ones a system forbids, how a key event is read as one (as CodeMirror's `runHandlers`
 * does), and how each system shows it. A chord is written for each system, never derived: there
 * is no `Mod`.
 *
 * Written form: `Ctrl+Alt+Shift+Meta+<key>`, modifiers in that order, each at most once. The key
 * is a lower-case letter, a digit, a character, a function key or a named key (`ArrowDown`,
 * `Space`). A character typed with Shift carries no `Shift` (`Ctrl+?`, never `Ctrl+Shift+?`).
 */
import { BASE, FIELD_KEYS, RESERVED, SHIFT_CHARS, SHIFTED, UNSHIFTED } from "./keytables";

export type System = "linux" | "mac" | "win";
export const SYSTEMS: readonly System[] = ["linux", "mac", "win"];

/** Where a binding acts: the whole window, or one kind of surface (the innermost focused). */
export type Scope = "window" | "editor" | "grid" | "diff";
export const SCOPES: readonly Scope[] = ["window", "editor", "grid", "diff"];

/** A binding the registry shows but never captures: the system's own (Help), or a widget's (undo). */
export type Delegated = "system" | "widget";

export interface Chord {
  readonly ctrl: boolean;
  readonly alt: boolean;
  readonly shift: boolean;
  readonly meta: boolean;
  readonly key: string;
}

/** The system the webview runs on, as Wails reports it: `darwin` → mac, `windows` → win. */
export function systemOf(os: string | undefined): System {
  return os === "darwin" ? "mac" : os === "windows" ? "win" : "linux";
}

/**
 * This window's system: what the Wails bootstrap reports (`window._wails.environment.OS`, which
 * may be set after the page's scripts start: read it each time), else what the browser says it
 * runs on (`navigator.userAgentData.platform`, then `navigator.platform`); Linux is left for the
 * browser test server and anything unknown.
 */
export function currentSystem(): System {
  const wails = (globalThis as { _wails?: { environment?: { OS?: string } } })._wails;
  const os = wails?.environment?.OS;
  if (os) return systemOf(os);
  const nav = globalThis.navigator as
    | (Navigator & { userAgentData?: { platform?: string } })
    | undefined;
  const platform = nav?.userAgentData?.platform || nav?.platform || "";
  return /mac/i.test(platform) ? "mac" : /win/i.test(platform) ? "win" : "linux";
}

const MODIFIERS = ["Ctrl", "Alt", "Shift", "Meta"] as const;
const FUNCTION_KEY = /^F([1-9]|1[0-9]|2[0-4])$/;
const NAMED = new Set([
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "Home",
  "End",
  "PageUp",
  "PageDown",
  "Enter",
  "Escape",
  "Tab",
  "Backspace",
  "Delete",
  "Insert",
  "Space",
  "ContextMenu",
  "PrintScreen",
]);
const BASE_CHARS = new Set("abcdefghijklmnopqrstuvwxyz0123456789`-=[]\\;',./");

export const isFunctionKey = (key: string): boolean => FUNCTION_KEY.test(key);
/** A key that types something (a character, or a space). */
const printable = (key: string): boolean => key.length === 1 || key === "Space";

export class ChordError extends Error {
  constructor(readonly chord: string) {
    super(`not a chord: ${chord}`);
    this.name = "ChordError";
  }
}

/** Reads a written chord; throws a {@link ChordError} for anything not in the written form. */
export function parseChord(text: string): Chord {
  let rest = text;
  const on = { Ctrl: false, Alt: false, Shift: false, Meta: false };
  for (const m of MODIFIERS)
    if (rest.startsWith(`${m}+`) && rest.length > m.length + 1) {
      on[m] = true;
      rest = rest.slice(m.length + 1);
    }
  const key = rest;
  const known = BASE_CHARS.has(key) || SHIFT_CHARS.has(key) || NAMED.has(key) || isFunctionKey(key);
  // Out of order, twice, `Mod`, an upper-case letter, or a Shift a shifted character already has.
  if (!known || (on.Shift && SHIFT_CHARS.has(key))) throw new ChordError(text);
  return { ctrl: on.Ctrl, alt: on.Alt, shift: on.Shift, meta: on.Meta, key };
}

export function formatChord(c: Chord): string {
  const on = [c.ctrl, c.alt, c.shift, c.meta];
  return [...MODIFIERS.filter((_, i) => on[i]), c.key].join("+");
}

/** Why a chord is refused on a system in a scope (spec §8.2 K-6, K-7), or undefined. */
export function chordProblem(
  text: string,
  system: System,
  scope: Scope,
  delegated?: Delegated,
): string | undefined {
  let c: Chord;
  try {
    c = parseChord(text);
  } catch {
    return "bad-chord";
  }
  if (c.ctrl && c.alt) return "ctrl-alt";
  // macOS Option, Windows access keys: Alt (with or without Shift) on a key that types.
  if (system !== "linux" && c.alt && !c.ctrl && !c.meta && printable(c.key))
    return "types-a-character";
  if (system !== "mac" && c.meta) return "reserved";
  if (delegated !== "system") {
    const typing = scope === "window" || scope === "editor";
    if (RESERVED[system].some((r) => r.chord === text && (!r.typing || typing))) return "reserved";
  }
  if (scope === "window" && delegated === undefined && FIELD_KEYS[system].includes(text))
    return "field-key";
  if (!c.ctrl && !c.alt && !c.meta && !isFunctionKey(c.key)) return "bare-key";
  return undefined;
}

/**
 * Whether an event can be a chord at all: Ctrl, Alt or Meta held, or a function key. Everything
 * else (typing) leaves the registry at once, without a lookup (C-12).
 */
export function mayBeChord(e: KeyboardEvent): boolean {
  return e.ctrlKey || e.altKey || e.metaKey || isFunctionKey(e.key);
}

/**
 * The key an event names, as w3c-keyname's `keyName` (CodeMirror) reads it: on macOS, ⌘⇧ reports
 * the unshifted key in `key` (⇧⌘G gives « g », ⇧⌘' gives « ' »), so the key code's shifted name is
 * taken instead; an `Unidentified` key falls back to its code too.
 */
function keyName(e: KeyboardEvent, system: System): string {
  const ignoreKey =
    (system === "mac" && e.metaKey && e.shiftKey && !e.ctrlKey && !e.altKey) ||
    e.key === "Unidentified";
  const fromCode = () => {
    const plain = BASE[e.keyCode];
    if (!e.shiftKey) return plain;
    return SHIFTED[e.keyCode] ?? (plain && /^[a-z]$/.test(plain) ? plain.toUpperCase() : plain);
  };
  const name = (ignoreKey ? fromCode() : undefined) ?? e.key;
  return name === " " ? "Space" : name;
}

/**
 * The chord an event means, among those `has` accepts, the way CodeMirror resolves its keymaps:
 * the key as named (see {@link keyName}), then the key's base under a modifier (`Ctrl+&` on AZERTY
 * is `Ctrl+1`, `Ctrl+и` is `Ctrl+b`, Caps Lock's `Ctrl+Shift+t` is `Ctrl+Shift+t`), then its shifted
 * form. On the digit row the digit comes first: on a French Mac ⌘4 types « ' », and must stay ⌘4,
 * never ⌘' (a digit chord is never shadowed by the character a layout puts on that key). None during
 * composition, and none for what types a character: AltGr (Ctrl+Alt) on Windows, Option on macOS,
 * Alt alone on Windows.
 */
export function eventChord(
  e: KeyboardEvent,
  system: System,
  has: (chord: string) => boolean,
): string | undefined {
  if (e.isComposing || e.keyCode === 229 || !mayBeChord(e)) return undefined;
  const name = keyName(e, system);
  const isChar = [...name].length === 1;
  if (system === "win" && e.ctrlKey && e.altKey) return undefined;
  if (system !== "linux" && e.altKey && !e.ctrlKey && !e.metaKey && (isChar || name === "Space"))
    return undefined;
  const make = (key: string, shift: boolean) =>
    formatChord({ ctrl: e.ctrlKey, alt: e.altKey, shift, meta: e.metaKey, key });
  const base = BASE[e.keyCode];
  const digit = base !== undefined && e.keyCode >= 48 && e.keyCode <= 57;
  const candidates = [
    ...(isChar && digit ? [make(base, e.shiftKey)] : []),
    make(name, !isChar && e.shiftKey),
  ];
  if (isChar) {
    // With Caps Lock on, Shift+T types « t »: the base under Shift is tried even when it is the name.
    if (base !== undefined && (base !== name || e.shiftKey))
      candidates.push(make(base, e.shiftKey));
    const shifted = SHIFTED[e.keyCode];
    if (e.shiftKey && shifted !== undefined && shifted !== name && shifted !== base)
      candidates.push(make(shifted, false));
  }
  return candidates.find(has);
}

type Lang = "fr" | "en";

const KEY_NAMES: Record<Lang, Record<string, string>> = {
  fr: {
    Space: "Espace",
    Enter: "Entrée",
    Escape: "Échap",
    Delete: "Suppr",
    Backspace: "Retour arrière",
    Insert: "Inser",
    PageUp: "Page↑",
    PageDown: "Page↓",
    Home: "Début",
    End: "Fin",
    ContextMenu: "Menu",
    PrintScreen: "Impr",
  },
  en: {
    Escape: "Esc",
    PageUp: "PgUp",
    PageDown: "PgDn",
    ContextMenu: "Menu",
    PrintScreen: "PrtSc",
  },
};
const ARROWS: Record<string, string> = {
  ArrowUp: "↑",
  ArrowDown: "↓",
  ArrowLeft: "←",
  ArrowRight: "→",
};
const MAC_KEYS: Record<string, string> = {
  Enter: "↩",
  Escape: "⎋",
  Backspace: "⌫",
  Delete: "⌦",
  Tab: "⇥",
};

function keyLabel(key: string, system: System, lang: Lang): string {
  if (key.length === 1) return key.toUpperCase();
  return (
    ARROWS[key] ?? (system === "mac" ? MAC_KEYS[key] : undefined) ?? KEY_NAMES[lang][key] ?? key
  );
}

/**
 * A chord as its system shows it: Linux as typed (« Ctrl+? », GNOME), Windows with the Shift the
 * character needs (« Ctrl+Maj+? »), macOS ⌃⌥⇧⌘ then the key (« ⇧⌘? »).
 */
export function renderChord(text: string, system: System, lang: Lang): string {
  const c = parseChord(text);
  const shift = c.shift || (system !== "linux" && SHIFT_CHARS.has(c.key));
  const key = keyLabel(c.key, system, lang);
  if (system === "mac")
    return `${c.ctrl ? "⌃" : ""}${c.alt ? "⌥" : ""}${shift ? "⇧" : ""}${c.meta ? "⌘" : ""}${key}`;
  const parts = [
    ...(c.ctrl ? ["Ctrl"] : []),
    ...(c.alt ? ["Alt"] : []),
    ...(shift ? [lang === "fr" ? "Maj" : "Shift"] : []),
    ...(c.meta ? [system === "win" ? "Win" : "Super"] : []),
    key,
  ];
  return parts.join("+");
}

/** A chord as `aria-keyshortcuts` names it (`Control+Shift+X`, `Meta+?`). */
export function ariaKeys(text: string): string {
  const c = parseChord(text);
  return [
    ...(c.ctrl ? ["Control"] : []),
    ...(c.alt ? ["Alt"] : []),
    ...(c.shift ? ["Shift"] : []),
    ...(c.meta ? ["Meta"] : []),
    c.key.length === 1 ? c.key.toUpperCase() : c.key,
  ].join("+");
}

/** The base key under a shifted character, for tests and the shortcuts window (EX0b2). */
export const unshifted = (char: string): string => UNSHIFTED[char] ?? char;
