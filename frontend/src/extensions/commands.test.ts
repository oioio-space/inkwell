import { readFileSync } from "node:fs";
import { history } from "@codemirror/commands";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { activeBlock, formatsOf, setBlock, textHandlers, toggleMark } from "../editor/format";
import { insert } from "../editor/format/insert";
import en from "../i18n/en.json";
import fr from "../i18n/fr.json";
import { requestSignature } from "../language/extension";
import { clampZoom, stepZoom } from "../preview/layout";
import { previewZoom } from "../preview/zoom.svelte";
import { gridHandlers } from "../ui/sheet/commands";
import { readUserBindings } from "./bindings";
import {
  type CommandContext,
  commandStats,
  configure,
  installKeys,
  menuOf,
  registerSurface,
  run,
  type Shell,
  useRegistry,
} from "./commands";
import {
  chordProblem,
  currentSystem,
  eventChord,
  renderChord,
  SYSTEMS,
  type System,
  systemOf,
} from "./keys";
import { OWNED_KEYS } from "./keytables";
import { setLive } from "./live.svelte";
import { builtinSources, globLoader } from "./loader";
import {
  DELEGABLE,
  LOCATIONS,
  ManifestError,
  type ManifestSource,
  type Origin,
  parseManifest,
} from "./manifest";
import { buildRegistry, type Registry } from "./registry";
import {
  CONTEXT_KEYS,
  type Context,
  emptyContext,
  evalWhen,
  keysOf,
  parseWhen,
  WhenError,
  whenStats,
} from "./when";

vi.mock("../language/extension", async (original) => ({
  ...(await original<typeof import("../language/extension")>()),
  requestSignature: vi.fn(async () => undefined),
}));

const VECTORS = import.meta.glob("./testdata/manifests/*/*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const vectors = (folder: "valid" | "invalid") =>
  Object.entries(VECTORS)
    .filter(([path]) => path.includes(`/manifests/${folder}/`) && path.includes("command"))
    .map(([path, text]) => ({ name: path.slice(path.lastIndexOf("/") + 1), text }));

const source = (text: string, origin: Origin = "builtin", folder = "sample"): ManifestSource => ({
  origin,
  folder,
  text,
  files: new Set(),
});

function verdict(s: ManifestSource): string {
  try {
    parseManifest(s);
    return "accepted";
  } catch (error) {
    if (error instanceof ManifestError) return error.reason;
    throw error;
  }
}

/** A manifest of one command, bound or shown as `extra` says. */
const manifest = (
  id: string,
  extra: Record<string, unknown> = {},
  commands = [{ id: `${id}.go`, label: "Go", category: "edit" }],
) => ({
  id,
  version: "1.0.0",
  contributes: { commands, ...extra },
});
const src = (o: { id: string }, origin: Origin = "builtin") =>
  source(JSON.stringify(o), origin, o.id);

/** The built-ins, and fixtures after them (the `format` built-in left out where it would collide). */
const builtins = (extra: ManifestSource[] = [], disabled: string[] = []): Registry =>
  buildRegistry([...builtinSources(), ...extra], globLoader(), { disabled });

/** A shell whose every action is recorded. */
function spyShell(): { shell: Shell; calls: string[] } {
  const calls: string[] = [];
  const shell = new Proxy({} as Shell, {
    get:
      (_, name) =>
      (...args: unknown[]) =>
        calls.push(args.length ? `${String(name)}(${args.join(",")})` : String(name)),
  });
  return { shell, calls };
}

/** A keydown as a browser makes it, with the fields happy-dom does not take in its init. */
function key(
  init: KeyboardEventInit & { keyCode?: number; isComposing?: boolean; repeat?: boolean },
): KeyboardEvent {
  const e = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  for (const f of ["keyCode", "isComposing", "repeat"] as const)
    if (init[f] !== undefined) Object.defineProperty(e, f, { value: init[f] });
  return e;
}

let removeKeys = () => {};
beforeEach(() => {
  setLive({ ...emptyContext(), formats: new Set() });
});
afterEach(() => {
  removeKeys();
  configure({ beforeCommand: () => {}, announce: () => {}, blocked: () => false });
  document.body.replaceChildren();
});

describe("EX0b1 commands, menus and keybindings", () => {
  it("EX0b1 C-1 a malformed command, menu or keybinding is refused with its reason", () => {
    const invalid = vectors("invalid");
    expect(invalid.length).toBeGreaterThanOrEqual(17);
    for (const v of invalid) {
      const reason = v.name.slice(0, v.name.indexOf("."));
      const origin: Origin = v.name.includes("plugin-") ? "plugin" : "builtin";
      expect({ vector: v.name, reason: verdict(source(v.text, origin)) }).toEqual({
        vector: v.name,
        reason,
      });
    }
    // The three the gate names, each for its own reason.
    const names = invalid.map((v) => v.name);
    for (const word of ["foreign-command", "other-manifest-binding", "plugin-command-entry"])
      expect(
        names.some((n) => n.includes(word)),
        word,
      ).toBe(true);
    // `Mod` is never a modifier: each system's chord is written.
    const bound = (chord: string) =>
      JSON.stringify(
        manifest("sample", {
          keybindings: [{ command: "sample.go", linux: [chord], scope: "window" }],
        }),
      );
    expect(verdict(source(bound("Mod+g")))).toBe("bad-chord");
    expect(verdict(source(bound("Ctrl+g")))).toBe("accepted");

    const valid = vectors("valid");
    expect(valid.length).toBeGreaterThan(0);
    for (const v of valid) {
      const parsed = parseManifest(source(v.text));
      const raw = JSON.parse(v.text) as { contributes: Record<string, Record<string, unknown>[]> };
      // Accepted unchanged: defaults fill only what the vector left out; `when` is parsed once.
      const plain = (x: unknown) =>
        JSON.parse(JSON.stringify(x, (k, val) => (k === "when" ? undefined : val)));
      expect(plain(parsed.contributes.commands)).toEqual(
        raw.contributes.commands?.map(({ when: _, ...c }) => ({
          keywords: [],
          repeat: false,
          capabilities: [],
          ...c,
        })),
      );
      expect(plain(parsed.contributes.menus)).toEqual(
        raw.contributes.menus?.map(({ when: _, ...m }) => m),
      );
      expect(plain(parsed.contributes.keybindings)).toEqual(
        raw.contributes.keybindings?.map(({ when: _, ...b }) => ({
          linux: [],
          mac: [],
          win: [],
          masks: [],
          ...b,
        })),
      );
      for (const point of ["commands", "menus", "keybindings"] as const)
        (raw.contributes[point] ?? []).forEach((x, i) => {
          if (typeof x.when === "string")
            expect(parsed.contributes[point][i]?.when).toEqual(parseWhen(x.when));
        });
    }
  });

  it("EX0b1 C-2 a when clause is checked against the closed context and evaluated by its table", () => {
    const ctx = (over: Partial<Context> = {}): Context => ({ ...emptyContext(), ...over });
    const t = { canPresent: true, readOnly: false, previewOpen: true };
    const rows: [string, Partial<Context>, boolean | string][] = [
      ["canPresent", t, true],
      ["!canPresent", t, false],
      ["!!canPresent", t, true],
      ["canPresent && readOnly", t, false],
      ["canPresent || readOnly", t, true],
      // `&&` binds tighter than `||`: a || (b && c).
      [
        "canPresent || readOnly && previewOpen",
        { canPresent: true, readOnly: false, previewOpen: false },
        true,
      ],
      [
        "(canPresent || readOnly) && previewOpen",
        { canPresent: true, readOnly: false, previewOpen: false },
        false,
      ],
      ["readOnly && previewOpen || canPresent", { canPresent: true }, true],
      ["!readOnly && canPresent", t, true],
      ["focus == editor", { focus: "editor" }, true],
      ["focus != editor", { focus: "grid" }, true],
      ["fileType == typst", { fileType: "typst" }, true],
      ["bold in formats", { formats: new Set(["bold"]) }, true],
      ["underline in formats", { formats: new Set(["bold"]) }, false],
      // A type no loaded extension contributes is valid, and false until it is there.
      ["fileType == csv", { fileType: "typst" }, false],
      ["unknownKey", {}, "unknown-key"],
      ["focus == sidebar", {}, "bad-value"],
      ["fileType =~ typ", {}, "bad-type"],
      ["fileType =~ /typ/", {}, "bad-type"],
      ["focus", {}, "bad-type"],
      ["bold in focus", {}, "bad-type"],
      ["sparkle in formats", {}, "bad-value"],
      ["canPresent &&", {}, "bad-syntax"],
      ["(canPresent", {}, "bad-syntax"],
      [`canPresent || ${"readOnly || ".repeat(30)}field`.padEnd(257, " "), {}, "too-long"],
      [`${"(".repeat(9)}canPresent${")".repeat(9)}`, {}, "too-deep"],
      [`${"(".repeat(8)}canPresent${")".repeat(8)}`, t, true],
    ];
    expect(rows.length).toBeGreaterThanOrEqual(15);
    for (const [text, over, want] of rows) {
      let got: boolean | string;
      try {
        got = evalWhen(parseWhen(text), ctx(over));
      } catch (error) {
        if (!(error instanceof WhenError)) throw error;
        got = error.reason;
      }
      expect({ text, got }).toEqual({ text, got: want });
    }
    // The context is closed and every key is used by a built-in clause or toggle: none is dead.
    const r = builtins();
    const used = new Set<string>([
      ...r.commands.flatMap((c) => [
        ...(c.when ? keysOf(c.when) : []),
        ...(c.toggled ? [c.toggled] : []),
      ]),
      ...r.keybindings.flatMap((b) => (b.when ? keysOf(b.when) : [])),
      // Every location's items and the status bar's (EX0c1) read the context too.
      ...Object.keys(LOCATIONS).flatMap((loc) =>
        r.menu(loc).flatMap((m) => (m.when ? keysOf(m.when) : [])),
      ),
      ...r.status.flatMap((x) => (x.when ? keysOf(x.when) : [])),
    ]);
    expect([...used].sort()).toEqual(Object.keys(CONTEXT_KEYS).sort());
  });

  it("EX0b1 C-3 a reserved or forbidden chord is refused on each system", () => {
    const refused = (chord: string, system: System, scope = "editor" as const) =>
      chordProblem(chord, system, scope);
    for (const system of SYSTEMS) expect(refused("Ctrl+Alt+x", system), system).toBe("ctrl-alt");
    // macOS: Option types a character, unless the key types nothing.
    expect(refused("Alt+e", "mac")).toBe("types-a-character");
    expect(refused("Alt+Shift+e", "mac")).toBe("types-a-character");
    expect(refused("Alt+1", "mac")).toBe("types-a-character");
    expect(refused("Alt+F5", "mac")).toBeUndefined();
    expect(refused("Alt+ArrowDown", "mac")).toBeUndefined();
    // Windows: Alt and a key are the menus' access keys.
    expect(refused("Alt+a", "win")).toBe("types-a-character");
    expect(refused("Alt+Shift+a", "win")).toBe("types-a-character");
    expect(refused("Alt+1", "win")).toBe("types-a-character");
    expect(refused("Alt+e", "linux")).toBeUndefined();
    // Each system's reserved chords.
    for (const chord of ["Meta+h", "Alt+Meta+h", "Meta+m", "Meta+q", "Ctrl+Space", "Meta+?"])
      expect(refused(chord, "mac"), chord).toBe("reserved");
    for (const chord of ["Ctrl+.", "Ctrl+Shift+u"])
      expect(refused(chord, "linux"), chord).toBe("reserved");
    expect(refused("Ctrl+Escape", "win")).toBe("reserved");
    // The system's Help is described and never captured.
    expect(chordProblem("Meta+?", "mac", "window", "system")).toBeUndefined();
    // A window binding never takes a key a text field uses.
    for (const chord of ["Ctrl+a", "Ctrl+z", "Ctrl+ArrowLeft", "Home"])
      expect(chordProblem(chord, "linux", "window"), chord).toMatch(/field-key|bare-key/);
    expect(chordProblem("Ctrl+a", "linux", "window")).toBe("field-key");
    expect(chordProblem("Meta+a", "mac", "window")).toBe("field-key");
    // A bare key types: no scope takes one.
    for (const scope of ["window", "editor", "grid", "diff"] as const) {
      expect(chordProblem("b", "linux", scope)).toBe("bare-key");
      expect(chordProblem("Enter", "linux", scope)).toBe("bare-key");
    }
    // K-7: a function key wants Fn on a Mac laptop, so its command has a macOS chord without one.
    const fkey = (mac: string[]) =>
      verdict(
        source(
          JSON.stringify(
            manifest("sample", {
              keybindings: [{ command: "sample.go", linux: ["F5"], mac, scope: "window" }],
            }),
          ),
        ),
      );
    expect(fkey(["F5"])).toBe("forbidden-chord");
    expect(fkey(["Alt+Meta+p"])).toBe("accepted");
    // A delegated binding is shown, never captured.
    const r = builtins([
      src(
        manifest("help", {
          keybindings: [
            { command: "help.go", mac: ["Meta+?"], scope: "window", delegated: "system" },
          ],
        }),
      ),
    ]);
    expect(r.refused).toEqual([]);
    useRegistry(r, { system: "mac" });
    const { shell, calls } = spyShell();
    configure({ shell });
    removeKeys = installKeys();
    const e = key({ key: "?", keyCode: 191, metaKey: true, shiftKey: true });
    document.body.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(false);
    expect(calls).toEqual([]);
  });

  it("EX0b1 C-4 a taken chord is refused in either order and never taken from the core", () => {
    const a = src(
      manifest("alpha", {
        keybindings: [{ command: "alpha.go", linux: ["Ctrl+b"], scope: "editor" }],
      }),
    );
    const b = src(
      manifest("beta", {
        keybindings: [{ command: "beta.go", linux: ["Ctrl+b"], scope: "editor" }],
      }),
    );
    const ab = builtins([a, b], ["format"]);
    const ba = builtins([b, a], ["format"]);
    expect(ab.bindings("linux").refused).toEqual([
      { folder: "beta", reason: "conflict on Ctrl+b (linux)", with: ["alpha.go", "beta.go"] },
    ]);
    expect(ba.bindings("linux").refused).toEqual([
      { folder: "alpha", reason: "conflict on Ctrl+b (linux)", with: ["beta.go", "alpha.go"] },
    ]);
    // The rest identical: the same commands, and each order keeps the first binding.
    expect(ab.commands.map((c) => c.id).sort()).toEqual(ba.commands.map((c) => c.id).sort());
    const on = (r: Registry) =>
      r
        .bindings("linux")
        .active.filter((x) => x.chord === "Ctrl+b")
        .map((x) => x.command);
    expect(on(ab)).toEqual(["alpha.go"]);
    expect(on(ba)).toEqual(["beta.go"]);
    // A window binding against a surface's on one chord: a conflict, unless the narrower names the other.
    const win = src(
      manifest("wide", {
        keybindings: [{ command: "wide.go", linux: ["Ctrl+Shift+l"], scope: "window" }],
      }),
    );
    const grid = (masks?: string[]) =>
      src(
        manifest("narrow", {
          keybindings: [
            {
              command: "narrow.go",
              linux: ["Ctrl+Shift+l"],
              scope: "grid",
              ...(masks ? { masks } : {}),
            },
          ],
        }),
      );
    expect(builtins([win, grid()]).bindings("linux").refused).toEqual([
      {
        folder: "narrow",
        reason: "conflict on Ctrl+Shift+l (linux)",
        with: ["wide.go", "narrow.go"],
      },
    ]);
    expect(builtins([win, grid(["wide.go"])]).bindings("linux").refused).toEqual([]);
    expect(builtins([grid(["wide.go"]), win]).bindings("linux").refused).toEqual([]);
    // Masks are the core's: a plugin cannot shadow anything.
    expect(
      verdict(
        source(
          JSON.stringify(
            manifest("plug", {
              keybindings: [
                { command: "plug.go", linux: ["Ctrl+Shift+l"], scope: "grid", masks: ["wide.go"] },
              ],
            }),
          ),
          "plugin",
        ),
      ),
    ).toBe("bad-value");
    // A plugin on the core's chord stays inactive, listed; its commands are kept.
    const plugin = src(
      manifest("plug", {
        keybindings: [{ command: "plug.go", linux: ["Ctrl+s"], scope: "window" }],
      }),
      "plugin",
    );
    const withPlugin = builtins([plugin]);
    const linux = withPlugin.bindings("linux");
    expect(linux.inactive).toEqual([
      { folder: "plug", reason: "conflict on Ctrl+s (linux)", with: ["app.save", "plug.go"] },
    ]);
    expect(linux.active.filter((x) => x.chord === "Ctrl+s").map((x) => x.command)).toEqual([
      "app.save",
    ]);
    expect(withPlugin.command("plug.go")).toBeDefined();
    // A viewer handles its own commands and the delegable ones, never another's.
    const viewer = (commands: string[]) =>
      verdict(
        source(
          JSON.stringify({
            id: "text",
            version: "1.0.0",
            contributes: {
              viewers: [
                {
                  id: "text.other",
                  fileTypes: ["text"],
                  label: "T",
                  entry: "native/v.ts",
                  commands,
                },
              ],
            },
          }),
          "builtin",
        ),
      );
    expect(DELEGABLE.has("format.bold") && DELEGABLE.has("app.zoom-in")).toBe(true);
    expect(viewer(["format.bold", "app.zoom-in"])).toBe("bad-entry"); // accepted up to its (absent) entry
    expect(viewer(["typst.present"])).toBe("not-own");
    expect(viewer(["app.save"])).toBe("not-own");
  });

  it("EX0b1 C-5 the person's own bindings win and are checked like the defaults", () => {
    const r = builtins();
    const active = (system: System, user: Parameters<Registry["bindings"]>[1]) =>
      r.bindings(system, user);
    const chords = (res: ReturnType<typeof active>, command: string) =>
      res.active.filter((a) => a.command === command).map((a) => a.chord);
    // Hers replaces the default of that command.
    const bold = active("linux", [{ command: "format.bold", key: "Ctrl+Shift+b" }]);
    expect(chords(bold, "format.bold")).toEqual(["Ctrl+Shift+b"]);
    expect(bold.active.some((a) => a.chord === "Ctrl+b")).toBe(false);
    // On a default's chord, that default is switched off on this device, and said to be.
    const strike = active("linux", [{ command: "format.strike", key: "Ctrl+s" }]);
    expect(chords(strike, "app.save")).toEqual([]);
    expect(chords(strike, "format.strike")).toEqual(["Ctrl+s"]);
    expect(strike.replaced).toEqual([
      { command: "app.save", chord: "Ctrl+s", by: "format.strike" },
    ]);
    // `null` removes.
    expect(chords(active("linux", [{ command: "format.bold", key: null }]), "format.bold")).toEqual(
      [],
    );
    // Checked like the defaults: reserved, forbidden, or already hers.
    expect(active("mac", [{ command: "app.save", key: "Meta+h" }]).mine).toEqual([
      { command: "app.save", key: "Meta+h", reason: "reserved" },
    ]);
    expect(active("linux", [{ command: "app.save", key: "Ctrl+Alt+x" }]).mine[0]?.reason).toBe(
      "ctrl-alt",
    );
    expect(active("mac", [{ command: "app.save", key: "Alt+e" }]).mine[0]?.reason).toBe(
      "types-a-character",
    );
    const twice = active("linux", [
      { command: "app.save", key: "Ctrl+F12" },
      { command: "app.preview", key: "Ctrl+F12" },
    ]);
    expect(twice.mine).toEqual([{ command: "app.preview", key: "Ctrl+F12", reason: "taken" }]);
    // A binding of hers that is refused leaves the command's default in force.
    expect(chords(active("mac", [{ command: "app.save", key: "Meta+h" }]), "app.save")).toEqual([
      "Meta+s",
    ]);
    // Hers keeps the default's `when`: a binding kept out of fields stays out of them.
    const guarded = builtins([
      src(
        manifest("guard", {
          keybindings: [
            { command: "guard.go", linux: ["Ctrl+Shift+y"], scope: "window", when: "!field" },
          ],
        }),
      ),
    ]);
    const mine = guarded
      .bindings("linux", [{ command: "guard.go", key: "Ctrl+F12" }])
      .active.find((a) => a.command === "guard.go");
    expect(mine?.chord).toBe("Ctrl+F12");
    expect(mine?.when).toEqual(parseWhen("!field"));
    expect(chords(twice, "app.save")).toEqual(["Ctrl+F12"]);
    // A stored list that cannot be read leaves the defaults in force.
    expect(readUserBindings("{not json")).toEqual([]);
    expect(readUserBindings('[{"command":"format.bold","key":"Ctrl+q","when":"field"}]')).toEqual(
      [],
    );
    expect(readUserBindings('{"command":"format.bold"}')).toEqual([]);
    expect(readUserBindings('[{"command":"format.bold","key":null}]')).toEqual([
      { command: "format.bold", key: null },
    ]);
    expect(chords(active("linux", readUserBindings("{not json")), "format.bold")).toEqual([
      "Ctrl+b",
    ]);
  });

  it("EX0b1 C-7 every built-in command is complete on the three systems and does its own job", async () => {
    const r = builtins();
    expect(r.refused).toEqual([]);
    for (const system of SYSTEMS) {
      const res = r.bindings(system);
      expect({ system, refused: res.refused, inactive: res.inactive }).toEqual({
        system,
        refused: [],
        inactive: [],
      });
    }
    // Labels, tooltips and reasons exist in both languages, short enough to be read (ND-4).
    const catalogs = { fr: fr as Record<string, string>, en: en as Record<string, string> };
    for (const c of r.commands)
      for (const [lang, cat] of Object.entries(catalogs)) {
        for (const k of [c.label, ...(c.unavailable ? [c.unavailable] : []), ...c.keywords])
          expect(cat[k], `${lang} ${c.id} ${k}`).toBeDefined();
        expect((cat[c.label] ?? "").length, `${lang} ${c.id}`).toBeLessThanOrEqual(40);
        if (c.tooltip) expect((cat[c.tooltip] ?? "").length).toBeLessThanOrEqual(80);
        if (c.unavailable) expect((cat[c.unavailable] ?? "").length).toBeLessThanOrEqual(80);
      }
    // Every row of spec §8.2 marked « fait » names commands whose chords are the row's, per system.
    const spec = readFileSync(`${process.cwd()}/../docs/design/spec.md`, "utf8");
    const table = spec.slice(spec.indexOf("### 8.2 "), spec.indexOf("### 8.3 "));
    const CHORD = /^(Ctrl|Alt|Maj|F\d|[⌃⌥⇧⌘])/;
    let rows = 0;
    for (const line of table.split("\n")) {
      const cells = line
        .trim()
        .replace(/^\||\|$/g, "")
        .split("|")
        .map((c) => c.trim());
      if (cells.length !== 5 || !cells[4]?.startsWith("fait")) continue;
      const ids = [...(cells[4] ?? "").matchAll(/`([a-z][a-z0-9-]*\.[a-z][a-z0-9-]*)`/g)].map(
        (m) => m[1] as string,
      );
      expect(ids.length, cells[0]).toBeGreaterThan(0);
      const shown = (cell: string) =>
        cell
          .replace(/\*\*/g, "")
          .replace(/\s*\(.*\)\s*$/, "")
          .split(" / ")
          .map((x) => x.trim());
      (["linux", "mac", "win"] as const).forEach((system, i) => {
        const parts = shown(cells[i + 1] ?? "");
        if (!parts.every((p) => CHORD.test(p))) return; // a sentence, not a chord (« le même raccourci… »)
        expect(parts.length, `${cells[0]} ${system}`).toBe(ids.length);
        const res = r.bindings(system);
        ids.forEach((id, k) => {
          const first = res.active.find((a) => a.command === id)?.chord;
          expect(first && renderChord(first, system, "fr"), `${id} ${system}`).toBe(parts[k]);
        });
      });
      rows++;
    }
    expect(rows).toBeGreaterThanOrEqual(15);
    // A chord delegated to a widget is the one the widget binds (CodeMirror's undo and redo).
    for (const system of SYSTEMS)
      for (const a of r.bindings(system).active.filter((x) => x.delegated === "widget")) {
        const owner = a.command === "app.undo" ? "undo" : "redo";
        const bound = OWNED_KEYS.filter((k) => k.name === owner).flatMap((k) => k[system]);
        expect(bound, `${a.command} ${a.chord} ${system}`).toContain(a.chord);
      }

    // One row per built-in command: run on mounted surfaces, it does its own job and no other's.
    for (const c of r.commands) await r.loadCommand(c);
    useRegistry(r, { system: "linux" });
    const { shell, calls } = spyShell();
    configure({ shell });
    const parent = document.createElement("div");
    document.body.append(parent);
    const fresh = (doc: string, anchor: number, head: number) =>
      EditorState.create({ doc, selection: { anchor, head }, extensions: [history()] });
    const view = new EditorView({ parent, state: fresh("un mot", 3, 6) });
    const reset = (text = "un mot", from = 3, to = 6) => view.setState(fresh(text, from, to));
    const editor = document.createElement("div");
    editor.append(parent);
    document.body.append(editor);
    registerSurface(
      editor,
      "editor",
      textHandlers(
        "typst",
        (cmd) => cmd(view),
        () => false,
      ),
      {
        viewer: r.essentialViewer("text"),
        view: () => view,
      },
    );
    const gridEl = document.createElement("div");
    document.body.append(gridEl);
    const sheet: string[] = [];
    const ctrl = new Proxy({ zoom: 1 } as Record<string, unknown>, {
      get: (o, name) =>
        name === "zoom"
          ? o.zoom
          : (...args: unknown[]) =>
              sheet.push(args.length ? `${String(name)}(${args.join(",")})` : String(name)),
    });
    let stepping = false;
    registerSurface(
      gridEl,
      "grid",
      gridHandlers({
        ctrl: () => ctrl as never,
        find: (replace) => sheet.push(`find(${replace})`),
        step: () => (stepping ? (d) => sheet.push(`step(${d})`) : undefined),
      }),
      { viewer: r.viewers.find((v) => v.id === "tables.grid") as never },
    );
    const diffEl = document.createElement("div");
    document.body.append(diffEl);
    registerSurface(diffEl, "diff", {
      "app.next-change": () => sheet.push("go(1)"),
      "app.previous-change": () => sheet.push("go(-1)"),
    });
    setLive({
      formats: formatsOf("typst"),
      readOnly: false,
      hasToolbar: true,
      canPresent: true,
      canReturn: true,
      fileType: "typst",
    });
    const own = (cmd: (target: EditorView) => boolean) => {
      const s = EditorState.create({ doc: "un mot", selection: { anchor: 3, head: 6 } });
      let out = s;
      cmd({ state: s, dispatch: (tr: { state: EditorState }) => (out = tr.state) } as never);
      return out.doc.toString();
    };
    type Row = {
      target?: Element;
      shell?: string;
      sheet?: string[];
      text?: string;
      extra?: () => void;
      /** What a command that is neither the shell's nor a surface's changed (the preview's zoom). */
      check?: () => void;
    };
    const TABLE: Record<string, Row> = {
      "app.save": { shell: "save" },
      "app.toolbar": { shell: "toggleToolbar" },
      "app.preview": { shell: "togglePreview" },
      "app.sidebar": { shell: "toggleSidebar" },
      "app.review": { shell: "toggleReview" },
      "app.fullscreen": { shell: "fullscreen" },
      "app.share": { shell: "share" },
      "app.people": { shell: "people" },
      "app.return": { shell: "returnToPrevious" },
      "app.preferences": { shell: "preferences" },
      "app.about": { shell: "about" },
      "app.problem-next": { shell: "problem(false)" },
      "app.problem-previous": { shell: "problem(true)" },
      "app.zoom-in": { shell: "zoom(in)" },
      "app.zoom-out": { shell: "zoom(out)" },
      "app.zoom-reset": { shell: "zoom(reset)" },
      "app.palette": { shell: "palette" },
      "app.shortcuts": { shell: "shortcuts" },
      "app.undo": {
        target: editor,
        text: "un mot",
        extra: () => view.dispatch({ changes: { from: 0, insert: "x" }, userEvent: "input" }),
      },
      "app.redo": { target: editor, text: "un mot" },
      "app.find": { target: gridEl, sheet: ["find(false)"] },
      "app.replace": { target: gridEl, sheet: ["find(true)"] },
      "app.find-next": { target: gridEl, sheet: ["step(1)"], extra: () => (stepping = true) },
      "app.find-previous": { target: gridEl, sheet: ["step(-1)"], extra: () => (stepping = true) },
      "app.next-change": { target: diffEl, sheet: ["go(1)"] },
      "app.previous-change": { target: diffEl, sheet: ["go(-1)"] },
      "typst.present": { shell: "present(start)" },
      "typst.present-here": { shell: "present(here)" },
      "typst.present-public": { shell: "present(public)" },
      "typst.export": { shell: "exportPdf" },
      "typst.jump-to-preview": { shell: "jumpToPreview" },
      "typst.preview-zoom-out": {
        extra: () => {
          setLive({ main: true, previewOpen: true });
          Object.assign(previewZoom, { mode: "fit", current: 1 });
        },
        check: () => expect(previewZoom.mode).toBe(clampZoom(stepZoom(1, -1))),
      },
      "typst.preview-zoom-in": {
        extra: () => {
          setLive({ main: true, previewOpen: true });
          Object.assign(previewZoom, { mode: "fit", current: 1 });
        },
        check: () => expect(previewZoom.mode).toBe(clampZoom(stepZoom(1, 1))),
      },
      "typst.preview-fit": {
        extra: () => {
          setLive({ main: true, previewOpen: true });
          Object.assign(previewZoom, { mode: 2, current: 2 });
        },
        check: () => expect(previewZoom.mode).toBe("fit"),
      },
      "typst.signature-help": { target: editor },
      "tables.fill-down": { target: gridEl, sheet: ["fillDown"] },
      "tables.fill-right": { target: gridEl, sheet: ["fillRight"] },
      "tables.insert-today": { target: gridEl, sheet: ["insertToday"] },
      "tables.insert-cells": { target: gridEl, sheet: ["insertAtSelection"] },
      "tables.delete-cells": { target: gridEl, sheet: ["deleteAtSelection"] },
      "format.bold": { target: editor, text: own(toggleMark("typst", "bold")) },
      "format.italic": { target: editor, text: own(toggleMark("typst", "italic")) },
      "format.underline": { target: editor, text: own(toggleMark("typst", "underline")) },
      "format.strike": { target: editor, text: own(toggleMark("typst", "strike")) },
      "format.link": { target: editor, text: own(toggleMark("typst", "link")) },
      "format.code": { target: editor, text: own(toggleMark("typst", "code")) },
      "format.codeblock": { target: editor, text: own(setBlock("typst", "codeblock")) },
      "format.formula": { target: editor, text: own(insert("typst", "formula")) },
      "format.ul": { target: editor, text: own(setBlock("typst", "ul")) },
      "format.ol": { target: editor, text: own(setBlock("typst", "ol")) },
      "format.task": { target: editor, text: "un mot" }, // Typst has no task list: nothing happens
      "format.quote": { target: editor, text: own(setBlock("typst", "quote")) },
      "format.table": { target: editor, text: own(insert("typst", "table")) },
      "format.h1": { target: editor, text: "= un mot" },
      "format.h2": { target: editor, text: "== un mot" },
      "format.h3": { target: editor, text: "=== un mot" },
      "format.h4": { target: editor, text: "==== un mot" },
      "format.paragraph": { target: editor, text: "un mot", extra: () => reset("== un mot", 6, 9) },
    };
    expect(Object.keys(TABLE).sort()).toEqual(r.commands.map((c) => c.id).sort());
    const texts = new Set(Object.values(TABLE).flatMap((row) => (row.text ? [row.text] : [])));
    expect(texts.size).toBeGreaterThan(10);
    for (const [id, row] of Object.entries(TABLE)) {
      calls.length = 0;
      sheet.length = 0;
      stepping = false;
      vi.mocked(requestSignature).mockClear();
      reset();
      if (id === "app.redo") {
        view.dispatch({ changes: { from: 0, insert: "x" }, userEvent: "input" });
        run("app.undo", editor);
      }
      row.extra?.();
      run(id, row.target);
      expect({ id, shell: calls }).toEqual({ id, shell: row.shell ? [row.shell] : [] });
      expect({ id, sheet }).toEqual({ id, sheet: row.sheet ?? [] });
      if (row.text !== undefined)
        expect({ id, text: view.state.doc.toString() }).toEqual({
          id,
          text: id === "app.redo" ? "xun mot" : row.text,
        });
      row.check?.();
      if (id === "typst.signature-help")
        expect(vi.mocked(requestSignature)).toHaveBeenCalledWith(view, true);
      else expect(vi.mocked(requestSignature), id).not.toHaveBeenCalled();
    }
    expect(activeBlock(view.state, "typst")).toBe("paragraph");
    view.destroy();
  });

  it("EX0b1 C-8 a key runs the command its chord names in the focused context, and nothing else", () => {
    const has =
      (...chords: string[]) =>
      (c: string) =>
        chords.includes(c);
    const chord = (e: KeyboardEvent, system: System) =>
      eventChord(
        e,
        system,
        has(
          "Ctrl+b",
          "Ctrl+1",
          "Ctrl+Alt+e",
          "Alt+e",
          "Alt+Shift+e",
          "Alt+Shift+a",
          "Ctrl+Shift+x",
        ),
      );
    expect(chord(key({ key: "b", keyCode: 66, ctrlKey: true }), "linux")).toBe("Ctrl+b");
    // macOS reports ⌘⇧ keys unshifted: ⇧⌘G is the previous match, ⇧⌘' the previous problem, ⇧⌘= is ⌘+.
    const mac = has(
      "Meta+g",
      "Shift+Meta+g",
      "Meta+'",
      "Shift+Meta+'",
      "Meta+=",
      "Meta++",
      "Meta+4",
    );
    expect(
      eventChord(key({ key: "g", keyCode: 71, metaKey: true, shiftKey: true }), "mac", mac),
    ).toBe("Shift+Meta+g");
    expect(
      eventChord(key({ key: "'", keyCode: 222, metaKey: true, shiftKey: true }), "mac", mac),
    ).toBe("Shift+Meta+'");
    expect(
      eventChord(key({ key: "=", keyCode: 187, metaKey: true, shiftKey: true }), "mac", mac),
    ).toBe("Meta++");
    expect(eventChord(key({ key: "g", keyCode: 71, metaKey: true }), "mac", mac)).toBe("Meta+g");
    // A French Mac: ⌘4 types « ' »; the digit comes first, so Titre 4 is ⌘4, never ⌘'.
    expect(eventChord(key({ key: "'", keyCode: 52, metaKey: true }), "mac", mac)).toBe("Meta+4");
    expect(eventChord(key({ key: "'", keyCode: 222, metaKey: true }), "mac", mac)).toBe("Meta+'");
    expect(chord(key({ key: "X", keyCode: 88, ctrlKey: true, shiftKey: true }), "linux")).toBe(
      "Ctrl+Shift+x",
    );
    // Caps Lock on: Shift+X types « x », still Ctrl+Shift+x.
    expect(chord(key({ key: "x", keyCode: 88, ctrlKey: true, shiftKey: true }), "linux")).toBe(
      "Ctrl+Shift+x",
    );
    // AZERTY: the digit row types & without Shift; the key's base is the digit.
    expect(chord(key({ key: "&", keyCode: 49, ctrlKey: true }), "linux")).toBe("Ctrl+1");
    // Cyrillic: the letter typed is и, the key is B.
    expect(chord(key({ key: "и", keyCode: 66, ctrlKey: true }), "linux")).toBe("Ctrl+b");
    // AltGr on Windows is Ctrl+Alt: it types a character.
    expect(
      chord(key({ key: "€", keyCode: 69, ctrlKey: true, altKey: true }), "win"),
    ).toBeUndefined();
    // macOS Option types a character.
    expect(chord(key({ key: "é", keyCode: 69, altKey: true }), "mac")).toBeUndefined();
    expect(
      chord(key({ key: "É", keyCode: 69, altKey: true, shiftKey: true }), "mac"),
    ).toBeUndefined();
    expect(
      chord(key({ key: "A", keyCode: 65, altKey: true, shiftKey: true }), "win"),
    ).toBeUndefined();
    // An input method composing: never a chord.
    expect(
      chord(key({ key: "b", keyCode: 66, ctrlKey: true, isComposing: true }), "linux"),
    ).toBeUndefined();
    expect(chord(key({ key: "Process", keyCode: 229, ctrlKey: true }), "linux")).toBeUndefined();

    // Dispatch on the built-ins.
    const r = builtins();
    useRegistry(r, { system: "linux" });
    const { shell, calls } = spyShell();
    configure({ shell });
    removeKeys = installKeys();
    const ran: string[] = [];
    const editor = document.createElement("div");
    const field = document.createElement("input");
    const diff = document.createElement("div");
    document.body.append(editor, field, diff);
    registerSurface(
      editor,
      "editor",
      { "format.bold": () => ran.push("bold"), "format.underline": () => ran.push("underline") },
      { viewer: r.essentialViewer("text") },
    );
    registerSurface(diff, "diff", { "app.next-change": () => ran.push("next") });
    setLive({ formats: formatsOf("markdown") });
    const bold = key({ key: "b", keyCode: 66, ctrlKey: true });
    editor.dispatchEvent(bold);
    expect(ran).toEqual(["bold"]);
    expect(bold.defaultPrevented).toBe(true);
    // In a field: Ctrl+B is the field's.
    const inField = key({ key: "b", keyCode: 66, ctrlKey: true });
    field.dispatchEvent(inField);
    expect(ran).toEqual(["bold"]);
    expect(inField.defaultPrevented).toBe(false);
    // A `when` that does not hold lets the key through (no underline in Markdown: CodeMirror's Ctrl+U).
    const underline = key({ key: "u", keyCode: 85, ctrlKey: true });
    editor.dispatchEvent(underline);
    expect(underline.defaultPrevented).toBe(false);
    expect(ran).toEqual(["bold"]);
    // A held key repeats only a command made for it.
    diff.dispatchEvent(key({ key: "F7", keyCode: 118 }));
    diff.dispatchEvent(key({ key: "F7", keyCode: 118, repeat: true }));
    expect(ran).toEqual(["bold", "next", "next"]);
    void r.loadCommand(r.command("app.save") as never);
    return r.loadCommand(r.command("app.save") as never)?.then(() => {
      editor.dispatchEvent(key({ key: "s", keyCode: 83, ctrlKey: true }));
      const held = key({ key: "s", keyCode: 83, ctrlKey: true, repeat: true });
      editor.dispatchEvent(held);
      expect(calls).toEqual(["save"]);
      expect(held.defaultPrevented).toBe(true);
      // While the window closes, nothing runs.
      configure({ blocked: () => true });
      editor.dispatchEvent(key({ key: "s", keyCode: 83, ctrlKey: true }));
      editor.dispatchEvent(key({ key: "b", keyCode: 66, ctrlKey: true }));
      expect(calls).toEqual(["save"]);
      expect(ran).toEqual(["bold", "next", "next"]);
      configure({ blocked: () => false });
      // A window chord whose command cannot run is still the application's: taken, and why is said.
      const announced: string[] = [];
      configure({ announce: (text) => announced.push(text) });
      setLive({ canPresent: false });
      const f5 = key({ key: "F5", keyCode: 116 });
      editor.dispatchEvent(f5);
      expect(f5.defaultPrevented).toBe(true);
      expect(announced).toEqual([fr["cmd.present.unavailable"]]);
      expect(calls).toEqual(["save"]);
      // Ctrl+Shift+- (Ctrl+_) zooms out too, as before.
      editor.dispatchEvent(key({ key: "_", keyCode: 189, ctrlKey: true, shiftKey: true }));
      expect(calls).toEqual(["save", "zoom(out)"]);

      // Inside a modal dialog only its own surface acts: nothing of the window behind it.
      const dialog = document.createElement("dialog");
      const name = document.createElement("input");
      dialog.append(name);
      dialog.setAttribute("open", "");
      document.body.append(dialog);
      for (const k of [
        { key: "F5", keyCode: 116 },
        { key: "F9", keyCode: 120 },
        { key: "T", keyCode: 84, ctrlKey: true, shiftKey: true },
        { key: ",", keyCode: 188, ctrlKey: true },
        { key: "s", keyCode: 83, ctrlKey: true },
      ]) {
        const e = key(k);
        name.dispatchEvent(e);
        expect(e.defaultPrevented, k.key).toBe(false);
      }
      expect(calls).toEqual(["save", "zoom(out)"]);
      dialog.remove();
      // The diff window is a modal surface: F7 there, even with the focus fallen to the body.
      // The system: what Wails reports, read when a chord comes (it may be set late), else the browser's.
      expect([systemOf("darwin"), systemOf("windows"), systemOf("linux")]).toEqual([
        "mac",
        "win",
        "linux",
      ]);
      const g = globalThis as { _wails?: unknown };
      const nav = navigator as Navigator & { userAgentData?: unknown };
      const before = g._wails;
      const platform = Object.getOwnPropertyDescriptor(nav, "platform");
      const setPlatform = (value: string) =>
        Object.defineProperty(nav, "platform", { value, configurable: true });
      g._wails = undefined;
      setPlatform("MacIntel");
      expect(currentSystem()).toBe("mac");
      setPlatform("Win32");
      expect(currentSystem()).toBe("win");
      setPlatform("Linux x86_64");
      expect(currentSystem()).toBe("linux");
      Object.defineProperty(nav, "userAgentData", {
        value: { platform: "macOS" },
        configurable: true,
      });
      expect(currentSystem()).toBe("mac");
      delete (nav as { userAgentData?: unknown }).userAgentData;
      g._wails = { environment: { OS: "windows" } };
      expect(currentSystem()).toBe("win");
      // The tables follow when Wails reports its system after the start: ⌘S on macOS.
      useRegistry(r);
      g._wails = { environment: { OS: "darwin" } };
      editor.dispatchEvent(key({ key: "s", keyCode: 83, metaKey: true }));
      expect(calls).toEqual(["save", "zoom(out)", "save"]);
      g._wails = before;
      if (platform) Object.defineProperty(nav, "platform", platform);
      else delete (nav as { platform?: string }).platform;
      useRegistry(r, { system: "linux" });
      const diffDialog = document.createElement("dialog");
      diffDialog.setAttribute("open", "");
      document.body.append(diffDialog);
      registerSurface(diffDialog, "diff", { "app.next-change": () => ran.push("modal next") });
      document.body.dispatchEvent(key({ key: "F7", keyCode: 118 }));
      expect(ran.at(-1)).toBe("modal next");
      const save = key({ key: "s", keyCode: 83, ctrlKey: true });
      document.body.dispatchEvent(save);
      expect(save.defaultPrevented).toBe(false);
      diffDialog.remove();
    });
  });

  it("EX0b1 C-9 a command runs on the surface the key was pressed in, after the pending text is written", async () => {
    const r = builtins();
    for (const c of r.commands) await r.loadCommand(c);
    useRegistry(r, { system: "linux" });
    const { shell, calls } = spyShell();
    const order: string[] = [];
    configure({ shell, beforeCommand: () => order.push("flush") });
    removeKeys = installKeys();
    const editor = document.createElement("div");
    const grid = document.createElement("div");
    document.body.append(editor, grid);
    registerSurface(editor, "editor", {}, { viewer: r.essentialViewer("text") });
    registerSurface(
      grid,
      "grid",
      { "app.zoom-in": () => order.push("grid zoom") },
      { viewer: r.viewers.find((v) => v.id === "tables.grid") as never },
    );
    grid.dispatchEvent(key({ key: "=", keyCode: 187, ctrlKey: true }));
    expect(order).toEqual(["flush", "grid zoom"]);
    expect(calls).toEqual([]);
    editor.dispatchEvent(key({ key: "=", keyCode: 187, ctrlKey: true }));
    expect(calls).toEqual(["zoom(in)"]);
    expect(order).toEqual(["flush", "grid zoom", "flush"]);

    // A command whose entry loads later still acts where its key was pressed.
    let release: (m: unknown) => void = () => {};
    const seen: (string | undefined)[] = [];
    const lazy = buildRegistry(
      [
        ...builtinSources(),
        {
          origin: "builtin",
          folder: "later",
          text: JSON.stringify({
            id: "later",
            version: "1.0.0",
            essential: true,
            contributes: {
              commands: [
                {
                  id: "later.go",
                  label: "Later",
                  category: "edit",
                  entry: "native/later.ts",
                  capabilities: ["ui.native"],
                },
              ],
              keybindings: [{ command: "later.go", linux: ["Ctrl+Shift+l"], scope: "window" }],
            },
          }),
          files: new Set(["native/later.ts"]),
        },
      ],
      (folder, entry) =>
        folder === "later"
          ? new Promise((resolve) => (release = resolve))
          : globLoader()(folder, entry),
    );
    expect(lazy.refused).toEqual([]);
    useRegistry(lazy, { system: "linux" });
    grid.dispatchEvent(key({ key: "L", keyCode: 76, ctrlKey: true, shiftKey: true }));
    editor.focus();
    release({ commands: { "later.go": (ctx: CommandContext) => seen.push(ctx.surface?.kind) } });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(seen).toEqual(["grid"]);
    expect(order.at(-1)).toBe("flush");
  });

  it("EX0b1 C-10 the main menu is drawn from the registry in its fixed groups", () => {
    const r = builtins();
    useRegistry(r, { system: "linux" });
    setLive({ hasToolbar: true, canReturn: true, canPresent: true });
    const groups = menuOf("main");
    expect(groups.map((g) => g.group)).toEqual(["view", "project", "document", "app"]);
    expect(groups.map((g) => g.items.map((c) => c.id))).toEqual([
      ["app.toolbar", "app.preview", "app.fullscreen"],
      ["app.return", "app.palette"],
      ["typst.present", "typst.present-here", "typst.present-public", "typst.export"],
      ["app.preferences", "app.shortcuts", "app.about"],
    ]);
    // Within a group: by order, then by extension.
    const item = (id: string, group: string, order: number) => ({
      command: `${id}.go`,
      location: "main",
      group,
      order,
    });
    const mixed = builtins([
      src(manifest("zeta", { menus: [item("zeta", "view", 20)] })),
      src(manifest("beta", { menus: [item("beta", "view", 20)] })),
    ]);
    useRegistry(mixed, { system: "linux" });
    expect(menuOf("main")[0]?.items.map((c) => c.id)).toEqual([
      "app.toolbar",
      "app.preview",
      "beta.go",
      "zeta.go",
      "app.fullscreen",
    ]);
    // The project's and the application's groups are the core's.
    for (const group of ["app", "project"]) {
      const plugin = builtins([
        src(manifest("plug", { menus: [item("plug", group, 1)] }), "plugin"),
      ]);
      expect(plugin.refused).toEqual([
        { folder: "plug", reason: `protected group ${group}`, with: ["plug.go"] },
      ]);
      expect(plugin.menu("main").some((m) => m.command === "plug.go")).toBe(false);
    }
    expect(
      builtins([src(manifest("plug", { menus: [item("plug", "document", 1)] }), "plugin")]).refused,
    ).toEqual([]);
    // A presentation needs a Typst document: greyed, and saying why when chosen.
    useRegistry(r, { system: "linux" });
    setLive({ canPresent: false, canReturn: false });
    const announced: string[] = [];
    const { shell, calls } = spyShell();
    configure({ shell, announce: (text) => announced.push(text) });
    const shown = menuOf("main").flatMap((g) => g.items.map((c) => c.id));
    expect(shown).not.toContain("app.return");
    expect(shown).toContain("typst.present");
    expect(run("typst.present")).toBe(false);
    expect(announced).toEqual([fr["cmd.present.unavailable"]]);
    expect(calls).toEqual([]);
    expect(r.command("app.toolbar")?.toggled).toBe("toolbarVisible");
  });

  it("EX0b1 C-12 commands cost nothing at start nor while typing", () => {
    // Built from the manifests alone: no entry is loaded, each clause parsed once.
    const sources = builtinSources();
    const clauses = sources
      .map((s) => JSON.parse(s.text) as { contributes: Record<string, { when?: string }[]> })
      .flatMap((m) =>
        ["commands", "menus", "status", "keybindings"].flatMap((p) =>
          (m.contributes[p] ?? []).filter((x) => x.when),
        ),
      );
    expect(clauses.length).toBeGreaterThan(20);
    let loads = 0;
    const parsed = whenStats.parsed;
    const r = buildRegistry(sources, (folder, entry) => {
      loads++;
      return globLoader()(folder, entry);
    });
    expect(loads).toBe(0);
    expect(whenStats.parsed - parsed).toBe(clauses.length);
    // Typing: no lookup, no clause evaluated.
    useRegistry(r, { system: "linux" });
    removeKeys = installKeys();
    const editor = document.createElement("div");
    document.body.append(editor);
    registerSurface(editor, "editor", {}, { viewer: r.essentialViewer("text") });
    const lookups = commandStats.lookups;
    const evaluated = whenStats.evaluated;
    const letters = "abcdefghijklmnopqrstuvwxyz 0123456789éèà,.;";
    for (let i = 0; i < 1000; i++) {
      const ch = letters[i % letters.length] as string;
      editor.dispatchEvent(
        key({ key: ch, keyCode: ch.toUpperCase().charCodeAt(0), shiftKey: i % 3 === 0 }),
      );
    }
    for (const k of ["Enter", "Backspace", "ArrowLeft", "Tab", "Escape"])
      editor.dispatchEvent(key({ key: k }));
    expect(commandStats.lookups - lookups).toBe(0);
    expect(whenStats.evaluated - evaluated).toBe(0);
  });
});
