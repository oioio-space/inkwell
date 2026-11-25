/**
 * The keys Inkwell's registry does not own, as data (spec §8.2, K-6): CodeMirror's keymaps as
 * installed, the grid's keys (WAI-ARIA grid pattern), the search keys of UX1-2 (written, not yet
 * wired), the field-editing keys, the keys each system reserves, and the key-code tables chords
 * are read with. A registry chord meeting one of these in an overlapping scope must name it in
 * `masks` (`keytables.test.ts` recomputes CodeMirror's rows from the installed packages).
 */
import type { Scope, System } from "./keys";

/** A key someone else binds: its owner's name, the scope it acts in, its chords per system. */
export interface OwnedKey {
  readonly name: string;
  readonly scope: Exclude<Scope, "window">;
  readonly linux: readonly string[];
  readonly mac: readonly string[];
  readonly win: readonly string[];
  /** An i18n key: the shortcuts window lists the key under « Écrire » (never a bare cursor move). */
  readonly label?: string;
}

type Row = readonly [map: string, name: string, linux: string[], mac: string[], win: string[]];

/**
 * CodeMirror's `defaultKeymap`, `historyKeymap` (editor/setup.ts) and `completionKeymap`
 * (language/extension.ts), in package order, each chord as the registry writes it.
 */
export const CODEMIRROR_ROWS: readonly Row[] = [
  [
    "defaultKeymap",
    "cursorSyntaxLeft",
    ["Alt+ArrowLeft", "Alt+Shift+ArrowLeft"],
    ["Ctrl+ArrowLeft", "Ctrl+Shift+ArrowLeft"],
    ["Alt+ArrowLeft", "Alt+Shift+ArrowLeft"],
  ],
  [
    "defaultKeymap",
    "cursorSyntaxRight",
    ["Alt+ArrowRight", "Alt+Shift+ArrowRight"],
    ["Ctrl+ArrowRight", "Ctrl+Shift+ArrowRight"],
    ["Alt+ArrowRight", "Alt+Shift+ArrowRight"],
  ],
  ["defaultKeymap", "moveLineUp", ["Alt+ArrowUp"], ["Alt+ArrowUp"], ["Alt+ArrowUp"]],
  [
    "defaultKeymap",
    "copyLineUp",
    ["Alt+Shift+ArrowUp"],
    ["Alt+Shift+ArrowUp"],
    ["Alt+Shift+ArrowUp"],
  ],
  ["defaultKeymap", "moveLineDown", ["Alt+ArrowDown"], ["Alt+ArrowDown"], ["Alt+ArrowDown"]],
  [
    "defaultKeymap",
    "copyLineDown",
    ["Alt+Shift+ArrowDown"],
    ["Alt+Shift+ArrowDown"],
    ["Alt+Shift+ArrowDown"],
  ],
  [
    "defaultKeymap",
    "addCursorAbove",
    ["Ctrl+Alt+ArrowUp"],
    ["Alt+Meta+ArrowUp"],
    ["Ctrl+Alt+ArrowUp"],
  ],
  [
    "defaultKeymap",
    "addCursorBelow",
    ["Ctrl+Alt+ArrowDown"],
    ["Alt+Meta+ArrowDown"],
    ["Ctrl+Alt+ArrowDown"],
  ],
  ["defaultKeymap", "simplifySelection", ["Escape"], ["Escape"], ["Escape"]],
  ["defaultKeymap", "insertBlankLine", ["Ctrl+Enter"], ["Meta+Enter"], ["Ctrl+Enter"]],
  ["defaultKeymap", "selectLine", ["Alt+l"], ["Ctrl+l"], ["Alt+l"]],
  ["defaultKeymap", "selectParentSyntax", ["Ctrl+i"], ["Meta+i"], ["Ctrl+i"]],
  ["defaultKeymap", "indentLess", ["Ctrl+["], ["Meta+["], ["Ctrl+["]],
  ["defaultKeymap", "indentMore", ["Ctrl+]"], ["Meta+]"], ["Ctrl+]"]],
  ["defaultKeymap", "indentSelection", ["Ctrl+Alt+\\"], ["Alt+Meta+\\"], ["Ctrl+Alt+\\"]],
  ["defaultKeymap", "deleteLine", ["Ctrl+Shift+k"], ["Shift+Meta+k"], ["Ctrl+Shift+k"]],
  [
    "defaultKeymap",
    "cursorMatchingBracket",
    ["Ctrl+Shift+\\"],
    ["Shift+Meta+\\"],
    ["Ctrl+Shift+\\"],
  ],
  ["defaultKeymap", "toggleComment", ["Ctrl+/"], ["Meta+/"], ["Ctrl+/"]],
  ["defaultKeymap", "toggleBlockComment", ["Alt+Shift+a"], ["Ctrl+Shift+a"], ["Alt+Shift+a"]],
  ["defaultKeymap", "toggleTabFocusMode", ["Ctrl+m"], ["Alt+Shift+m"], ["Ctrl+m"]],
  [
    "defaultKeymap",
    "cursorCharLeft",
    ["ArrowLeft", "Shift+ArrowLeft"],
    ["ArrowLeft", "Shift+ArrowLeft"],
    ["ArrowLeft", "Shift+ArrowLeft"],
  ],
  [
    "defaultKeymap",
    "cursorGroupLeft",
    ["Ctrl+ArrowLeft", "Ctrl+Shift+ArrowLeft"],
    ["Alt+ArrowLeft", "Alt+Shift+ArrowLeft"],
    ["Ctrl+ArrowLeft", "Ctrl+Shift+ArrowLeft"],
  ],
  ["defaultKeymap", "cursorLineBoundaryLeft", [], ["Meta+ArrowLeft", "Shift+Meta+ArrowLeft"], []],
  [
    "defaultKeymap",
    "cursorCharRight",
    ["ArrowRight", "Shift+ArrowRight"],
    ["ArrowRight", "Shift+ArrowRight"],
    ["ArrowRight", "Shift+ArrowRight"],
  ],
  [
    "defaultKeymap",
    "cursorGroupRight",
    ["Ctrl+ArrowRight", "Ctrl+Shift+ArrowRight"],
    ["Alt+ArrowRight", "Alt+Shift+ArrowRight"],
    ["Ctrl+ArrowRight", "Ctrl+Shift+ArrowRight"],
  ],
  [
    "defaultKeymap",
    "cursorLineBoundaryRight",
    [],
    ["Meta+ArrowRight", "Shift+Meta+ArrowRight"],
    [],
  ],
  [
    "defaultKeymap",
    "cursorLineUp",
    ["ArrowUp", "Shift+ArrowUp"],
    ["ArrowUp", "Shift+ArrowUp"],
    ["ArrowUp", "Shift+ArrowUp"],
  ],
  ["defaultKeymap", "cursorDocStart", [], ["Meta+ArrowUp", "Shift+Meta+ArrowUp"], []],
  ["defaultKeymap", "cursorPageUp", [], ["Ctrl+ArrowUp", "Ctrl+Shift+ArrowUp"], []],
  [
    "defaultKeymap",
    "cursorLineDown",
    ["ArrowDown", "Shift+ArrowDown"],
    ["ArrowDown", "Shift+ArrowDown"],
    ["ArrowDown", "Shift+ArrowDown"],
  ],
  ["defaultKeymap", "cursorDocEnd", [], ["Meta+ArrowDown", "Shift+Meta+ArrowDown"], []],
  ["defaultKeymap", "cursorPageDown", [], ["Ctrl+ArrowDown", "Ctrl+Shift+ArrowDown"], []],
  [
    "defaultKeymap",
    "cursorPageUp",
    ["PageUp", "Shift+PageUp"],
    ["PageUp", "Shift+PageUp"],
    ["PageUp", "Shift+PageUp"],
  ],
  [
    "defaultKeymap",
    "cursorPageDown",
    ["PageDown", "Shift+PageDown"],
    ["PageDown", "Shift+PageDown"],
    ["PageDown", "Shift+PageDown"],
  ],
  [
    "defaultKeymap",
    "cursorLineBoundaryBackward",
    ["Home", "Shift+Home"],
    ["Home", "Shift+Home"],
    ["Home", "Shift+Home"],
  ],
  [
    "defaultKeymap",
    "cursorDocStart",
    ["Ctrl+Home", "Ctrl+Shift+Home"],
    ["Meta+Home", "Shift+Meta+Home"],
    ["Ctrl+Home", "Ctrl+Shift+Home"],
  ],
  [
    "defaultKeymap",
    "cursorLineBoundaryForward",
    ["End", "Shift+End"],
    ["End", "Shift+End"],
    ["End", "Shift+End"],
  ],
  [
    "defaultKeymap",
    "cursorDocEnd",
    ["Ctrl+End", "Ctrl+Shift+End"],
    ["Meta+End", "Shift+Meta+End"],
    ["Ctrl+End", "Ctrl+Shift+End"],
  ],
  [
    "defaultKeymap",
    "insertNewlineAndIndent",
    ["Enter", "Shift+Enter"],
    ["Enter", "Shift+Enter"],
    ["Enter", "Shift+Enter"],
  ],
  ["defaultKeymap", "selectAll", ["Ctrl+a"], ["Meta+a"], ["Ctrl+a"]],
  [
    "defaultKeymap",
    "deleteCharBackward",
    ["Backspace", "Shift+Backspace"],
    ["Backspace", "Shift+Backspace"],
    ["Backspace", "Shift+Backspace"],
  ],
  ["defaultKeymap", "deleteCharForward", ["Delete"], ["Delete"], ["Delete"]],
  [
    "defaultKeymap",
    "deleteGroupBackward",
    ["Ctrl+Backspace"],
    ["Alt+Backspace"],
    ["Ctrl+Backspace"],
  ],
  ["defaultKeymap", "deleteGroupForward", ["Ctrl+Delete"], ["Alt+Delete"], ["Ctrl+Delete"]],
  ["defaultKeymap", "deleteLineBoundaryBackward", [], ["Meta+Backspace"], []],
  ["defaultKeymap", "deleteLineBoundaryForward", [], ["Meta+Delete"], []],
  ["defaultKeymap", "cursorCharLeft", [], ["Ctrl+b", "Ctrl+Shift+b"], []],
  ["defaultKeymap", "cursorCharRight", [], ["Ctrl+f", "Ctrl+Shift+f"], []],
  ["defaultKeymap", "cursorLineUp", [], ["Ctrl+p", "Ctrl+Shift+p"], []],
  ["defaultKeymap", "cursorLineDown", [], ["Ctrl+n", "Ctrl+Shift+n"], []],
  ["defaultKeymap", "cursorLineStart", [], ["Ctrl+a", "Ctrl+Shift+a"], []],
  ["defaultKeymap", "cursorLineEnd", [], ["Ctrl+e", "Ctrl+Shift+e"], []],
  ["defaultKeymap", "deleteCharForward", [], ["Ctrl+d"], []],
  ["defaultKeymap", "deleteCharBackward", [], ["Ctrl+h"], []],
  ["defaultKeymap", "deleteToLineEnd", [], ["Ctrl+k"], []],
  ["defaultKeymap", "deleteGroupBackward", [], ["Ctrl+Alt+h"], []],
  ["defaultKeymap", "splitLine", [], ["Ctrl+o"], []],
  ["defaultKeymap", "transposeChars", [], ["Ctrl+t"], []],
  ["defaultKeymap", "cursorPageDown", [], ["Ctrl+v"], []],
  ["historyKeymap", "undo", ["Ctrl+z"], ["Meta+z"], ["Ctrl+z"]],
  ["historyKeymap", "redo", ["Ctrl+y"], ["Shift+Meta+z"], ["Ctrl+y"]],
  ["historyKeymap", "redo", ["Ctrl+Shift+z"], [], []],
  ["historyKeymap", "undoSelection", ["Ctrl+u"], ["Meta+u"], ["Ctrl+u"]],
  ["historyKeymap", "redoSelection", ["Alt+u"], ["Shift+Meta+u"], ["Alt+u"]],
  ["completionKeymap", "startCompletion", ["Ctrl+Space"], ["Ctrl+Space"], ["Ctrl+Space"]],
  ["completionKeymap", "startCompletion", [], ["Alt+`"], []],
  ["completionKeymap", "startCompletion", [], ["Alt+i"], []],
  ["completionKeymap", "closeCompletion", ["Escape"], ["Escape"], ["Escape"]],
  ["completionKeymap", "moveCompletionDown", ["ArrowDown"], ["ArrowDown"], ["ArrowDown"]],
  ["completionKeymap", "moveCompletionUp", ["ArrowUp"], ["ArrowUp"], ["ArrowUp"]],
  ["completionKeymap", "moveCompletionPageDown", ["PageDown"], ["PageDown"], ["PageDown"]],
  ["completionKeymap", "moveCompletionPageUp", ["PageUp"], ["PageUp"], ["PageUp"]],
  ["completionKeymap", "acceptCompletion", ["Enter"], ["Enter"], ["Enter"]],
];

const same = (chords: string[]) => ({ linux: chords, mac: chords, win: chords });
/** A Ctrl chord as macOS writes it, with ⌘ (canonical order: Ctrl, Alt, Shift, Meta). */
const onMac = (c: string) =>
  c.startsWith("Ctrl+Shift+")
    ? `Shift+Meta+${c.slice(11)}`
    : c.startsWith("Ctrl+")
      ? `Meta+${c.slice(5)}`
      : c;
/** The same chords with ⌘ for Ctrl on macOS. */
const mod = (chords: string[]) => ({ linux: chords, win: chords, mac: chords.map(onMac) });
const arrows = ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"];

/**
 * The grid's own keys (`ui/sheet/keys.ts` and `Grid.svelte`): the WAI-ARIA grid pattern, after
 * Excel. The modified ones are exactly what `gridKey` handles (`keytables.test.ts` checks it);
 * Ctrl+Z, Ctrl+Y and Ctrl+Shift+Z are the grid's undo, the delegated `app.undo` and `app.redo`.
 */
export const GRID_KEYS: readonly OwnedKey[] = [
  { name: "grid.move", scope: "grid", ...same([...arrows, ...arrows.map((a) => `Shift+${a}`)]) },
  {
    name: "grid.edge",
    scope: "grid",
    ...mod([...arrows.map((a) => `Ctrl+${a}`), ...arrows.map((a) => `Ctrl+Shift+${a}`)]),
  },
  { name: "grid.row-edge", scope: "grid", ...same(["Home", "End", "Shift+Home", "Shift+End"]) },
  {
    name: "grid.sheet-edge",
    scope: "grid",
    ...mod(["Ctrl+Home", "Ctrl+End", "Ctrl+Shift+Home", "Ctrl+Shift+End"]),
  },
  {
    name: "grid.page",
    scope: "grid",
    ...same(["PageUp", "PageDown", "Shift+PageUp", "Shift+PageDown"]),
  },
  { name: "grid.walk", scope: "grid", ...same(["Enter", "Shift+Enter", "Tab", "Shift+Tab"]) },
  { name: "grid.edit", scope: "grid", ...same(["F2", "Space"]), label: "keys.cell-edit" },
  {
    name: "grid.newline-in-cell",
    scope: "grid",
    ...same(["Alt+Enter"]),
    label: "keys.cell-newline",
  },
  { name: "grid.cancel", scope: "grid", ...same(["Escape"]) },
  { name: "grid.clear", scope: "grid", ...same(["Delete", "Backspace"]), label: "keys.cell-clear" },
  {
    name: "grid.context-menu",
    scope: "grid",
    ...same(["ContextMenu", "Shift+F10"]),
    label: "keys.cell-menu",
  },
  { name: "grid.filter", scope: "grid", ...same(["Alt+ArrowDown"]), label: "keys.cell-list" },
  {
    name: "grid.select-columns",
    scope: "grid",
    ...mod(["Ctrl+Space"]),
    label: "keys.select-columns",
  },
  { name: "grid.select-rows", scope: "grid", ...same(["Shift+Space"]), label: "keys.select-rows" },
  { name: "grid.select-all", scope: "grid", ...mod(["Ctrl+a"]), label: "keys.select-all" },
  { name: "grid.undo", scope: "grid", ...mod(["Ctrl+z"]) },
  { name: "grid.redo", scope: "grid", ...mod(["Ctrl+Shift+z", "Ctrl+y"]) },
];

/** The search keys of `editor/search/keymap.ts` (UX1-2: written, not wired; checked already). */
export const SEARCH_KEYS: readonly OwnedKey[] = [
  { name: "search.find", scope: "editor", ...mod(["Ctrl+f"]) },
  { name: "search.replace", scope: "editor", ...mod(["Ctrl+h"]) },
  { name: "search.find-in-project", scope: "editor", ...mod(["Ctrl+Shift+f"]) },
  { name: "search.next", scope: "editor", ...same(["F3", "Shift+F3"]) },
  { name: "search.next-result", scope: "editor", ...same(["F4", "Shift+F4"]) },
];

/** Typst's writing help (language/extension.ts): Escape closes the help on a function. */
export const WRITING_HELP_KEYS: readonly OwnedKey[] = [
  { name: "writingHelp.close", scope: "editor", ...same(["Escape"]) },
];

/** The editor keys the shortcuts window shows under « Écrire », by CodeMirror's name. */
const EDITOR_LABELS: Readonly<Record<string, string>> = {
  selectAll: "keys.select-all",
  selectLine: "keys.select-line",
  indentMore: "keys.indent-more",
  indentLess: "keys.indent-less",
  toggleComment: "keys.comment",
  deleteLine: "keys.delete-line",
  moveLineUp: "keys.move-line-up",
  moveLineDown: "keys.move-line-down",
  copyLineUp: "keys.copy-line-up",
  copyLineDown: "keys.copy-line-down",
};

/** CodeMirror's rows as owned keys of the editor, and the writing help's. */
export const EDITOR_KEYS: readonly OwnedKey[] = [
  ...CODEMIRROR_ROWS.map(([, name, linux, mac, win]) => ({
    name,
    scope: "editor" as const,
    linux,
    mac,
    win,
    ...(EDITOR_LABELS[name] ? { label: EDITOR_LABELS[name] } : {}),
  })),
  ...WRITING_HELP_KEYS,
];

/** Every key the registry must not take unnoticed, by name. */
export const OWNED_KEYS: readonly OwnedKey[] = [...EDITOR_KEYS, ...GRID_KEYS, ...SEARCH_KEYS];

/**
 * What a text field does with keys (select all, clipboard, undo, word moves): a `window` binding
 * never takes one of these, or typing in a field would break.
 */
export const FIELD_KEYS: Readonly<Record<System, readonly string[]>> = {
  linux: fieldKeys("Ctrl", "Ctrl"),
  win: fieldKeys("Ctrl", "Ctrl"),
  mac: [
    ...fieldKeys("Meta", "Alt"),
    ...arrows.flatMap((a) => [`Meta+${a}`, `Shift+Meta+${a}`]),
    "Meta+Backspace",
    "Meta+Delete",
  ],
};

function fieldKeys(command: string, word: string): string[] {
  const shifted = (m: string, k: string) => (m === "Meta" ? `Shift+Meta+${k}` : `${m}+Shift+${k}`);
  return [
    ...["a", "c", "v", "x", "z", "y"].map((k) => `${command}+${k}`),
    shifted(command, "z"),
    shifted(command, "v"),
    ...arrows.flatMap((a) => [`${word}+${a}`, shifted(word, a)]),
    `${word}+Backspace`,
    `${word}+Delete`,
    "Home",
    "End",
    "Shift+Home",
    "Shift+End",
    `${command}+Home`,
    `${command}+End`,
  ];
}

/** A chord the system takes; `typing` ones only where text is typed (window and editor scopes). */
export interface Reserved {
  readonly chord: string;
  readonly typing?: true;
}

const fkeys = (from: number, to: number, prefix: string) =>
  Array.from({ length: to - from + 1 }, (_, i) => `${prefix}F${from + i}`);

/**
 * The reserved chords of spec §8.2 per system. On Linux and Windows every chord with Super or
 * Windows (`Meta`) is reserved too, and every Ctrl+Alt chord everywhere (K-7): those are rules of
 * `keys.ts`, not rows.
 */
export const RESERVED: Readonly<Record<System, readonly Reserved[]>> = {
  linux: [
    ...["Alt+Tab", "Alt+F2", "Alt+F4", "Alt+F7", "Alt+F8", "Alt+F10", "Alt+Space"].map((chord) => ({
      chord,
    })),
    ...["PrintScreen", "Shift+PrintScreen", "Alt+PrintScreen"].map((chord) => ({ chord })),
    // GTK 4 and WebKitGTK in a text field: the emoji chooser and Unicode entry; the web inspector.
    ...["Ctrl+.", "Ctrl+;", "Ctrl+Shift+u"].map((chord) => ({ chord, typing: true as const })),
    { chord: "Ctrl+Shift+i" },
  ],
  mac: [
    ...[
      "Meta+h",
      "Alt+Meta+h",
      "Meta+m",
      "Alt+Meta+m",
      "Meta+q",
      "Meta+Tab",
      "Meta+`",
      "Meta+Space",
      "Alt+Meta+Space",
      "Ctrl+Space",
      "Ctrl+Meta+Space",
      "Alt+Meta+Escape",
      "Shift+Meta+3",
      "Shift+Meta+4",
      "Shift+Meta+5",
      "Ctrl+Meta+q",
      "Shift+Meta+q",
      "Alt+Shift+Meta+q",
      "Meta+?",
      ...arrows.map((a) => `Ctrl+${a}`),
      "Alt+Meta+8",
      "Alt+Meta+=",
      "Alt+Meta+-",
      "Meta+F5",
      "F11",
      ...fkeys(1, 8, "Ctrl+"),
      "Ctrl+Meta+d",
      "Alt+Meta+t",
    ].map((chord) => ({ chord })),
  ],
  win: [
    ...["Alt+Tab", "Alt+F4", "Alt+Space", "Ctrl+Escape", "Ctrl+Shift+Escape"].map((chord) => ({
      chord,
    })),
    { chord: "Ctrl+Space", typing: true },
  ],
};

/** `KeyboardEvent.keyCode` → the key without Shift on a US layout (after w3c-keyname's `base`). */
export const BASE: Readonly<Record<number, string>> = {
  ...Object.fromEntries(Array.from({ length: 10 }, (_, i) => [48 + i, String(i)])),
  ...Object.fromEntries(
    Array.from({ length: 26 }, (_, i) => [65 + i, String.fromCharCode(97 + i)]),
  ),
  59: ";",
  61: "=",
  173: "-",
  186: ";",
  187: "=",
  188: ",",
  189: "-",
  190: ".",
  191: "/",
  192: "`",
  219: "[",
  220: "\\",
  221: "]",
  222: "'",
};

/** `KeyboardEvent.keyCode` → the key with Shift on a US layout (w3c-keyname's `shift`). */
export const SHIFTED: Readonly<Record<number, string>> = {
  48: ")",
  49: "!",
  50: "@",
  51: "#",
  52: "$",
  53: "%",
  54: "^",
  55: "&",
  56: "*",
  57: "(",
  59: ":",
  61: "+",
  173: "_",
  186: ":",
  187: "+",
  188: "<",
  189: "_",
  190: ">",
  191: "?",
  192: "~",
  219: "{",
  220: "|",
  221: "}",
  222: '"',
};

/** Characters typed with Shift on a US layout: a chord on one carries no `Shift` (`Ctrl+?`). */
export const SHIFT_CHARS: ReadonlySet<string> = new Set(Object.values(SHIFTED));

/** The base character under each shifted one, for showing the Shift it needs (Windows, macOS). */
export const UNSHIFTED: Readonly<Record<string, string>> = Object.fromEntries(
  Object.entries(SHIFTED).map(([code, char]) => [char, BASE[Number(code)] ?? char]),
);
