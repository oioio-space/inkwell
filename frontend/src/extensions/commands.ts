/**
 * Running commands (ADR 0027, « Commands, menus and keybindings »): `run()`, the one key listener
 * and the identity every control bound to a command carries. A command runs on a surface (the
 * text editor, the grid, the diff view) when that surface declared it, else through its entry with
 * the native context. Pending text is written first, so a save never misses the last keystroke.
 */
import { lang, type MessageKey, t } from "../i18n";
import type { Active } from "./bindings";
import { ariaKeys, currentSystem, eventChord, mayBeChord, renderChord, type System } from "./keys";
import { keymap, live } from "./live.svelte";
import { DELEGABLE } from "./manifest";
import type { Command, Registry, Viewer } from "./registry";
import { type Context, evalWhen, type Focus } from "./when";

/** What the application offers a built-in command's entry (`shell`, ext/app/native/commands.ts). */
export interface Shell {
  save(): void;
  toggleToolbar(): void;
  togglePreview(): void;
  toggleSidebar(): void;
  toggleReview(): void;
  share(): void;
  people(): void;
  exportPdf(): void;
  preferences(): void;
  about(): void;
  returnToPrevious(): void;
  problem(reverse: boolean): void;
  zoom(step: "in" | "out" | "reset"): void;
  present(how: "start" | "here" | "public"): void;
  jumpToPreview(): void;
  fullscreen(): void;
  /** The command palette and the shortcuts window (EX0b2, ui/command/dialogs.svelte.ts). */
  palette(): void;
  shortcuts(): void;
}

/** Where a command acts: a mounted view that declared it, by its kind. */
export interface Surface {
  readonly kind: Exclude<Focus, "other">;
  readonly element: Element;
  readonly handlers: ReadonlyMap<string, Handler>;
  /** The editor's view, for a text surface (typed loosely: no CodeMirror here). */
  readonly view?: (() => unknown) | undefined;
}

/** A surface's handler; `false` says it declined. */
export type Handler = () => unknown;

/** What an entry's handler receives: the shell, and the surface the command was aimed at. */
export interface CommandContext {
  readonly shell: Shell;
  readonly surface?: Surface | undefined;
  readonly view?: unknown;
}

/** What a command entry exports. */
export interface CommandModule {
  readonly commands: Readonly<Record<string, (ctx: CommandContext) => unknown>>;
}

interface Hooks {
  shell?: Shell;
  /** Writes the pending edits (the typing path), before any command. */
  beforeCommand: () => void;
  /** Says why a command cannot run (a greyed item activated). */
  announce: (text: string) => void;
  /** Nothing runs while the window closes or a presentation covers it. */
  blocked: () => boolean;
}

const surfaces = new WeakMap<Element, Surface>();
const hooks: Hooks = { beforeCommand: () => {}, announce: () => {}, blocked: () => false };
let registry: Registry | undefined;
/** The system the tables were built for; `fixed` when a caller chose it (tests). */
let system: System = "linux";
let fixed: System | undefined;
let user: Parameters<Registry["bindings"]>[1] = [];
let built = false;
/** The active chords → the bindings on them; and each command's chords, delegated ones too. */
let byChord = new Map<string, Active[]>();
let byCommand = new Map<string, Active[]>();

/** Lookups made by the listener (C-12: none for a typed character). */
export const commandStats = { lookups: 0 };

/** Sets the registry commands come from, the system and the person's bindings. */
export function useRegistry(
  r: Registry,
  opts: { system?: System; user?: Parameters<Registry["bindings"]>[1] } = {},
): void {
  registry = r;
  fixed = opts.system;
  user = opts.user ?? [];
  built = false;
}

/**
 * The tables of the system in force, built on first use and rebuilt when the system turns out
 * to be another (Wails may report it after the page's scripts start).
 */
function tables(): { byChord: Map<string, Active[]>; byCommand: Map<string, Active[]> } {
  const now = fixed ?? currentSystem();
  if (registry && (!built || now !== system)) {
    performance.mark("inkwell:keymap:start");
    // A rebuild (not the first build) redraws what shows chords, after the current update.
    if (built) queueMicrotask(() => keymap.version++);
    system = now;
    built = true;
    byChord = new Map();
    byCommand = new Map();
    const resolution = registry.bindings(system, user);
    for (const p of [...resolution.refused, ...resolution.inactive])
      console.error(`keybinding of ${p.folder} left out on ${system}: ${p.reason}`);
    for (const a of resolution.active) {
      byCommand.set(a.command, [...(byCommand.get(a.command) ?? []), a]);
      if (!a.delegated) byChord.set(a.chord, [...(byChord.get(a.chord) ?? []), a]);
    }
    performance.measure("inkwell:keymap", "inkwell:keymap:start");
  }
  return { byChord, byCommand };
}

/** The application's part: its actions, the flush of pending text, the announcer, the guard. */
export function configure(next: Partial<Hooks>): void {
  Object.assign(hooks, next);
}

export const commandSystem = (): System => {
  tables();
  return system;
};

/**
 * Declares a mounted view as a surface. A viewer's surface takes only the commands its manifest
 * declares; a core surface (the diff view) only the delegable ones.
 */
export function registerSurface(
  element: Element,
  kind: Surface["kind"],
  handlers: Readonly<Record<string, Handler>>,
  opts: { viewer?: Viewer; view?: () => unknown } = {},
): () => void {
  const allowed = (id: string) =>
    opts.viewer ? opts.viewer.commands.includes(id) : DELEGABLE.has(id);
  const map = new Map<string, Handler>();
  for (const [id, h] of Object.entries(handlers)) {
    if (allowed(id)) map.set(id, h);
    else console.error(`surface ${opts.viewer?.id ?? kind}: ${id} is not declared`);
  }
  surfaces.set(element, Object.freeze({ kind, element, handlers: map, view: opts.view }));
  return () => surfaces.delete(element);
}

/** `use:surface={…}`: a mounted view declares itself a surface while it is in the page. */
export function surface(
  node: Element,
  params: {
    kind: Surface["kind"];
    handlers: Readonly<Record<string, Handler>>;
    viewer?: Viewer;
    view?: () => unknown;
  },
): { update(next: typeof params): void; destroy(): void } {
  const add = (p: typeof params) =>
    registerSurface(node, p.kind, p.handlers, {
      ...(p.viewer ? { viewer: p.viewer } : {}),
      ...(p.view ? { view: p.view } : {}),
    });
  let remove = add(params);
  return {
    update(next) {
      remove();
      remove = add(next);
    },
    destroy: () => remove(),
  };
}

/** The innermost surface holding a node. */
export function surfaceAt(node: EventTarget | null | undefined): Surface | undefined {
  for (let n = node instanceof Node ? node : null; n; n = n.parentNode) {
    const s = n instanceof Element ? surfaces.get(n) : undefined;
    if (s) return s;
  }
  return undefined;
}

/** A text field has the keyboard (not the editor's own content). */
function isField(node: EventTarget | null): boolean {
  if (!(node instanceof HTMLElement)) return false;
  if (node.closest(".cm-editor")) return false;
  return (
    node instanceof HTMLInputElement ||
    node instanceof HTMLTextAreaElement ||
    node.isContentEditable
  );
}

function context(surface: Surface | undefined, field: boolean): Context {
  return { ...live, focus: surface?.kind ?? "other", field };
}

const toSurface = (target: Element | Surface | undefined) =>
  target === undefined ? undefined : "kind" in target ? target : surfaceAt(target);

/** A command's text (label, reason, keyword): an i18n key read in this language for a built-in. */
export const textOf = (c: Command, text: string): string =>
  c.origin === "builtin" ? t(text as MessageKey) : text;

/** The command's label, read in this language for a built-in. */
export function labelOf(c: Command): string {
  return textOf(c, c.label);
}

/** A command's chords in force on this system, the first one shown. */
export const chordsOf = (id: string): readonly string[] => {
  void keymap.version;
  return (tables().byCommand.get(id) ?? []).map((a) => a.chord);
};

/** The first chord of a command as this system shows it, if it has one. */
export function shortcutOf(id: string): string | undefined {
  const chord = chordsOf(id)[0];
  return chord === undefined ? undefined : renderChord(chord, system, lang);
}

/** Whether a command can run now on a target, and if not, why (its `unavailable`, read). */
export function availability(
  id: string,
  target?: Element | Surface,
): { ok: boolean; reason?: string } {
  const c = registry?.command(id);
  if (!c) return { ok: false };
  const ok = evalWhen(c.when, context(toSurface(target), false));
  if (ok || c.unavailable === undefined) return { ok };
  return { ok, reason: textOf(c, c.unavailable) };
}

/**
 * Whether a command can run on a target now: its `when` holds, and the target's surface or the
 * command's entry handles it (a surface-only command, undo or find, is greyed where no surface
 * declares it). Every refusal has a reason: its `unavailable`, else a general one (EX0b2 P-5).
 */
export function runnable(id: string, target?: Element | Surface): { ok: boolean; reason: string } {
  const c = registry?.command(id);
  const { ok, reason } = availability(id, target);
  if (ok && c && canHandle(c, toSurface(target))) return { ok: true, reason: "" };
  const own = ok && c?.unavailable !== undefined ? textOf(c, c.unavailable) : reason;
  return { ok: false, reason: own ?? t("cmd.unavailable.here") };
}

/** Says a text to the person, as a greyed command's reason is said (the application's announcer). */
export function announce(text: string): void {
  hooks.announce(text);
}

/** The registry commands come from, for what lists them (the palette, the shortcuts window). */
export const commandRegistry = (): Registry | undefined => registry;

/** A command's bindings in force on this system, delegated and personal ones included. */
export const activeOf = (id: string): readonly Active[] => {
  void keymap.version;
  return tables().byCommand.get(id) ?? [];
};

/** The last focused element inside a surface, while it is in the page: where a menu returns. */
let lastSurface: Element | undefined;
export const returnTarget = (): Element | undefined =>
  lastSurface?.isConnected ? lastSurface : undefined;
function noteFocus(e: FocusEvent): void {
  if (e.target instanceof Element && surfaceAt(e.target)) lastSurface = e.target;
}

/** Whether a toggle command is on (its `toggled` key). */
export function isOn(id: string): boolean | undefined {
  const key = registry?.command(id)?.toggled;
  return key === undefined ? undefined : (live as unknown as Record<string, unknown>)[key] === true;
}

/**
 * The identity of a control bound to a command (C-11): `data-cmd`, its name, a title with this
 * system's chord, and `aria-keyshortcuts`. A toggle keeps one label; its state is `aria-pressed`.
 */
export function cmdAttrs(id: string): Record<string, string> {
  const c = registry?.command(id);
  if (!c) return { "data-cmd": id };
  const name = labelOf(c);
  const chord = chordsOf(id)[0];
  const shown = chord === undefined ? undefined : renderChord(chord, system, lang);
  return {
    "data-cmd": id,
    "aria-label": name,
    title: shown === undefined ? name : `${name} (${shown})`,
    ...(chord === undefined ? {} : { "aria-keyshortcuts": ariaKeys(chord) }),
  };
}

/**
 * Runs a command: pending text is written first, then the target surface's handler, else the
 * command's entry with the native context. A command whose `when` does not hold says why and runs
 * nothing. Returns whether something handled it.
 */
export function run(id: string, target?: Element | Surface): boolean {
  const c = registry?.command(id);
  if (!c) return false;
  const surface = toSurface(target);
  if (!evalWhen(c.when, context(surface, false))) {
    announceUnavailable(c, surface);
    return false;
  }
  return execute(c, surface);
}

function announceUnavailable(c: Command, surface: Surface | undefined): void {
  const reason = availability(c.id, surface).reason;
  if (reason) hooks.announce(reason);
}

/** Runs a command whose `when` holds: the pending text first, then its handler. */
function execute(c: Command, surface: Surface | undefined): boolean {
  hooks.beforeCommand();
  const handler = surface?.handlers.get(c.id);
  if (handler) return handler() !== false;
  return runEntry(c, surface);
}

function runEntry(c: Command, surface: Surface | undefined): boolean {
  if (!registry || c.entry === undefined) return false;
  // The surface is captured now: an entry that loads later still acts where the key was pressed.
  const ctx: CommandContext = {
    shell: hooks.shell ?? (noShell as Shell),
    surface,
    view: surface?.view?.(),
  };
  const call = (m: unknown) => {
    (m as CommandModule | undefined)?.commands?.[c.id]?.(ctx);
  };
  const loaded = registry.loadedCommand(c);
  if (loaded) call(loaded);
  else
    void registry
      .loadCommand(c)
      ?.then(call)
      .catch((error: unknown) => console.warn(`command ${c.id}:`, error));
  return true;
}

const noShell = new Proxy({}, { get: () => () => {} });

function canHandle(c: Command, surface: Surface | undefined): boolean {
  return surface?.handlers.has(c.id) === true || c.entry !== undefined;
}

/**
 * The one key listener (capture phase, so it acts before the editor and the grid): a chord whose
 * binding is in force in the focused scope, whose `when` holds and which has a handler there, runs
 * its command and takes the key; anything else goes on untouched.
 */
export function onKeydown(e: KeyboardEvent): void {
  if (e.key === "Escape") for (const f of [...escapes]) f();
  if (!mayBeChord(e) || !registry) return;
  commandStats.lookups++;
  if (hooks.blocked()) return;
  const { byChord } = tables();
  const chord = eventChord(e, system, (x) => byChord.has(x));
  if (chord === undefined) return;
  // Inside a modal dialog only that dialog's own surface acts: nothing runs behind it (spec §3).
  // A key whose focus fell to the page's body while the dialog is open was pressed in it.
  const modal = document.querySelector("dialog[open]");
  const target = e.target instanceof Node ? e.target : null;
  let surface = surfaceAt(target);
  if (modal) {
    const inside = target !== null && modal.contains(target);
    if (!inside && target !== document.body && target !== document.documentElement) return;
    surface = inside ? surface : surfaceAt(modal);
    if (!surface || !(surface.element === modal || modal.contains(surface.element))) return;
  }
  const ctx = context(surface, isField(e.target));
  // The focused surface's own binding before the window's; none of the window's in a modal.
  const candidates = (byChord.get(chord) ?? [])
    .filter((a) => (a.scope === "window" && !modal) || a.scope === ctx.focus)
    .sort((a, b) => Number(a.scope === "window") - Number(b.scope === "window"));
  for (const a of candidates) {
    const c = registry.command(a.command);
    if (!c || !evalWhen(a.when, ctx) || !canHandle(c, surface)) continue;
    const available = evalWhen(c.when, ctx);
    // A surface's chord that cannot act goes on to the surface (Ctrl+U in Markdown is
    // CodeMirror's); a window chord is the application's either way: taken, and its reason said
    // (F5 on a file that is not Typst must not reach the webview's own reload).
    if (!available && a.scope !== "window") continue;
    e.preventDefault();
    e.stopPropagation();
    // A held key repeats only a command made for it (zoom, next change); Ctrl+S saves once.
    if (e.repeat && !c.repeat) return;
    if (available) execute(c, surface);
    else announceUnavailable(c, surface);
    return;
  }
}

/** A menu's items shown now (their `when`), in their groups' fixed order. */
export function menuOf(location: "main"): { group: string; items: Command[] }[] {
  const r = registry;
  if (!r) return [];
  const ctx = context(undefined, false);
  const groups: { group: string; items: Command[] }[] = [];
  for (const item of r.menu(location)) {
    const c = r.command(item.command);
    if (!c || !evalWhen(item.when, ctx)) continue;
    const last = groups.at(-1);
    if (last?.group === item.group) last.items.push(c);
    else groups.push({ group: item.group, items: [c] });
  }
  return groups;
}

const escapes = new Set<() => void>();

/**
 * Calls `fn` at each Escape pressed in the window, through the one key listener (a dialog whose
 * chunk is still loading is then dropped, EX0b2); returns its removal.
 */
export function onEscape(fn: () => void): () => void {
  escapes.add(fn);
  return () => escapes.delete(fn);
}

/** Installs the listener on a window, and the note of the last focused surface; returns their removal. */
export function installKeys(win: Window = window): () => void {
  win.addEventListener("keydown", onKeydown, true);
  win.addEventListener("focusin", noteFocus, true);
  return () => {
    win.removeEventListener("keydown", onKeydown, true);
    win.removeEventListener("focusin", noteFocus, true);
  };
}
