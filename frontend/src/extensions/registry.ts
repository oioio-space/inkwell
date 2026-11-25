/**
 * The registry of contributions (ADR 0027): built once from manifests alone, never by running an
 * extension's code, then frozen. It answers what a file is (its type, from its name), what shows
 * it (a viewer, and the alternatives offered in the view bar) and which grammar colours it.
 * Entries are loaded on demand, once each, through the loader it was given.
 */
import { LanguageDescription, type LanguageSupport } from "@codemirror/language";
import { type Binding, type Resolution, resolve, type UserBinding } from "./bindings";
import type { System } from "./keys";
import {
  type CommandContribution,
  type DebugContribution,
  type FileTypeContribution,
  type GrammarContribution,
  type GroupSpec,
  locationOf,
  ManifestError,
  type ManifestSource,
  type MenuContribution,
  type MetricContribution,
  type Origin,
  type PanelContribution,
  parseManifest,
  parseMetric,
  SIDES,
  type Side,
  STATUS,
  type StatusContribution,
  type Storage,
  type ViewerContribution,
} from "./manifest";
import { checkRead, type DebugRead } from "./values";

/** What Go found when it read a file as text: decodable text, or not (MF-15). */
export type ContentClass = "text" | "binary";

/** The essential types a file falls back to when no other type claims its name. */
export const TEXT_TYPE = "text";
export const BINARY_TYPE = "binary";

export interface ExtensionInfo {
  readonly id: string;
  readonly version: string;
  readonly origin: Origin;
  readonly folder: string;
  readonly essential: boolean;
  readonly activation: readonly string[];
}

interface Owned {
  /** The id of the extension that contributes it. */
  readonly extension: string;
  readonly folder: string;
  readonly origin: Origin;
}

export interface FileType extends FileTypeContribution, Owned {}
export interface Viewer extends ViewerContribution, Owned {}
export interface Grammar extends GrammarContribution, Owned {}
export interface Command extends CommandContribution, Owned {}
export interface MenuItem extends MenuContribution, Owned {}
export interface StatusItem extends StatusContribution, Owned {}
export interface Panel extends PanelContribution, Owned {}
export interface DebugProvider extends DebugContribution, Owned {}
/** A metric of a manifest, or one Go declares in code (`go`), which has no folder. */
export interface Metric extends MetricContribution {
  readonly extension: string;
  readonly from: "manifest" | "go";
}
/** The metrics of the manifests and of Go together, and the ones refused for it. */
export interface MetricsView {
  readonly metrics: readonly Metric[];
  readonly refused: readonly Refusal[];
}
/** What a debug provider's entry exports. */
export interface DebugModule {
  read(): unknown;
}
export type { Binding, DebugRead };

/** A manifest the registry did not take, and why. */
export interface Refusal {
  readonly folder: string;
  readonly reason: string;
  /** For a conflict: the extension that holds the claim, then the one refused. */
  readonly with?: readonly string[];
}

/** How a file opens: its type, the viewer that shows it, the other viewers offered. */
export interface Opening {
  readonly type: FileType;
  readonly viewer: Viewer;
  readonly alternatives: readonly Viewer[];
  /** What Go found in the bytes; absent for a type kept as bytes, which no one decodes. */
  readonly content?: ContentClass;
  /** A text type whose bytes are not text: shown as the file card, with this notice (ND-7). */
  readonly notice?: "not-text";
  /** The type is the person's choice for this file, not what its name says. */
  readonly chosen?: boolean;
}

/** What a grammar entry exports. */
export interface GrammarModule {
  grammar(ctx: { codeLanguages: readonly LanguageDescription[] }): LanguageSupport;
}

/** Loads the entry of the extension in `folder`. The registry calls it at most once per entry. */
export type EntryLoader = (folder: string, entry: string) => Promise<unknown>;

export interface NameHints {
  /** The type the person chose for this file. */
  choice?: string | undefined;
  /** The file's first line, for a `#!` interpreter. */
  firstLine?: string | undefined;
}

export interface Registry {
  readonly extensions: readonly ExtensionInfo[];
  readonly fileTypes: readonly FileType[];
  readonly viewers: readonly Viewer[];
  readonly grammars: readonly Grammar[];
  readonly refused: readonly Refusal[];
  fileType(id: string): FileType | undefined;
  /** The type a name says, in the order of ADR 0027 rule 6; undefined for an unknown name. */
  typeOf(name: string, hints?: NameHints): FileType | undefined;
  /** Whether the bytes must be read as text before the file can be opened. */
  needsContent(name: string, hints?: NameHints): boolean;
  /** How a file opens. `content` is required unless the name's type is kept as bytes. */
  open(name: string, hints?: NameHints & { content?: ContentClass | undefined }): Opening;
  viewersFor(type: FileType): { default: Viewer; alternatives: readonly Viewer[] };
  /** The text editor (`text`) or the file card (`blob`). */
  essentialViewer(storage: Storage): Viewer;
  /** A grammar by id or by one of its names. */
  grammar(name: string): Grammar | undefined;
  /** The grammars as CodeMirror language descriptions, for Markdown's fenced blocks. */
  codeLanguages(): readonly LanguageDescription[];
  /** A grammar's language, loaded on first use; undefined for an unknown or failing grammar. */
  languageSupport(id: string): Promise<LanguageSupport | undefined>;
  /** A grammar's language if it is already loaded. */
  loadedSupport(id: string): LanguageSupport | undefined;
  /** A contribution's entry module, loaded once whatever the number of callers. */
  load(c: Viewer | Grammar): Promise<unknown>;
  /** Whether a viewer failed to load this session (its stand-in shows instead). */
  hasFailed(viewer: Viewer): boolean;
  /**
   * The module of an opening's viewer. A viewer that fails to load is replaced by the essential
   * viewer of the content class, and `notify` is called once.
   */
  loadViewer(opening: Opening, notify: () => void): Promise<{ viewer: Viewer; module: unknown }>;
  readonly commands: readonly Command[];
  command(id: string): Command | undefined;
  /** Every keybinding as declared, in build order. */
  readonly keybindings: readonly Binding[];
  /** The bindings in force on a system, with the person's own (`user`) applied. */
  bindings(system: System, user?: readonly UserBinding[]): Resolution;
  /**
   * A location's items in its groups' fixed order, then by `order`, extension and command:
   * indexed once at build, the same array at every call (L-9).
   */
  menu(location: string): readonly MenuItem[];
  /** The status bar's items: start side then end, each in its groups' fixed order (L-7). */
  readonly status: readonly StatusItem[];
  /** A status item's entry module, loaded once; its `status[id]` is the item's component. */
  loadStatus(item: StatusItem): Promise<unknown>;
  /**
   * A side's panels: the core's first, then `order`, extension and id, never the build order;
   * indexed once at build, the same array at every call (EX0c2 P-2, P-11).
   */
  panels(side: Side): readonly Panel[];
  /** A panel's entry module, loaded once; its `panels[id]` is the body's component. */
  loadPanel(p: Panel): Promise<unknown>;
  /** A command's entry module, loaded once; undefined for a command without one. */
  loadCommand(c: Command): Promise<unknown> | undefined;
  /** A command's entry module if it has already loaded. */
  loadedCommand(c: Command): unknown;
  /** The metrics of the manifests, with Go's if they were given at build. */
  readonly metrics: readonly Metric[];
  metric(id: string): Metric | undefined;
  /**
   * The metrics with Go's own descriptors (M-3): one whose id another holds is refused, Go's
   * being the core's, whether Go's arrive before the registry is built or after.
   */
  withGoMetrics(descriptors: readonly unknown[]): MetricsView;
  readonly debug: readonly DebugProvider[];
  /**
   * Runs a debug provider (its entry is loaded on the first read, and nothing runs it
   * otherwise, Z-4) and passes its record through rule V (V-1): a value that breaks it is
   * dropped and counted, and a provider that fails reads as empty.
   */
  readDebug(p: DebugProvider): Promise<DebugRead>;
}

function freeze<T extends object>(fields: T): Readonly<T> {
  return Object.freeze(Object.assign(Object.create(null) as T, fields));
}

/** The last segment of a path, slash or backslash. */
function baseName(name: string): string {
  return name.slice(Math.max(name.lastIndexOf("/"), name.lastIndexOf("\\")) + 1);
}

/** The interpreter word of a `#!` line: `#!/bin/sh` → sh, `#!/usr/bin/env -S bash -e` → bash. */
function interpreter(firstLine: string): string | undefined {
  if (!firstLine.startsWith("#!")) return undefined;
  const words = firstLine.slice(2).trim().split(/\s+/);
  let program = baseName(words[0] ?? "");
  if (program === "env") program = words.slice(1).find((w) => !w.startsWith("-")) ?? "";
  return program ? program.toLowerCase() : undefined;
}

/** The claims a manifest makes, each one only one extension may hold. */
function claimsOf(m: ReturnType<typeof parseManifest>): string[] {
  const c = m.contributes;
  return [
    `extension id ${m.id}`,
    ...c.fileTypes.flatMap((t) => [
      `type ${t.id}`,
      ...t.extensions.map((e) => `extension ${e}`),
      ...t.filenames.map((n) => `name ${n.toLowerCase()}`),
      ...t.shebangs.map((s) => `interpreter ${s}`),
    ]),
    ...c.viewers.map((v) => `viewer ${v.id}`),
    ...c.grammars.flatMap((g) => [`grammar ${g.id}`, ...g.names.map((n) => `grammar name ${n}`)]),
    ...c.commands.map((x) => `command ${x.id}`),
    ...c.status.map((x) => `status ${x.id}`),
    ...c.panels.map((x) => `panel ${x.id}`),
    ...c.metrics.map((x) => `metric ${x.id}`),
    ...c.debug.map((x) => `debug ${x.id}`),
  ];
}

/**
 * Merges Go's metric descriptors into the manifests' metrics: each is checked like a manifest's
 * (its extension is its id's first word), and a manifest metric whose id Go holds is refused.
 */
function mergeGoMetrics(
  own: readonly Metric[],
  folderOf: ReadonlyMap<string, string>,
  descriptors: readonly unknown[],
): MetricsView {
  const refused: Refusal[] = [];
  const go: Metric[] = [];
  for (const [i, d] of descriptors.entries()) {
    const id = (d as { id?: unknown } | null)?.id;
    const extension = typeof id === "string" ? id.slice(0, Math.max(0, id.indexOf("."))) : "";
    try {
      const m = parseMetric(d, `go.metrics[${i}]`, extension);
      if (go.some((x) => x.id === m.id)) throw new ManifestError("bad-id", `go.metrics[${i}].id`);
      go.push(freeze({ ...m, extension, from: "go" as const }));
    } catch (error) {
      refused.push(
        freeze({ folder: "go", reason: error instanceof ManifestError ? error.reason : "invalid" }),
      );
    }
  }
  const taken = new Set(go.map((x) => x.id));
  const kept = own.filter((m) => {
    if (!taken.has(m.id)) return true;
    refused.push(
      freeze({
        folder: folderOf.get(m.extension) ?? m.extension,
        reason: `conflict on metric ${m.id}`,
        with: Object.freeze(["go", m.extension]),
      }),
    );
    return false;
  });
  return freeze({ metrics: Object.freeze([...go, ...kept]), refused: Object.freeze(refused) });
}

/**
 * Builds the registry from manifest sources, in their order: a manifest whose claim (a type, an
 * extension, a whole name, an interpreter, an id) is already held is refused, naming both. The
 * ones in `disabled` are left out unless essential. No entry is loaded here.
 */
export function buildRegistry(
  sources: readonly ManifestSource[],
  loader: EntryLoader,
  opts: { disabled?: Iterable<string>; goMetrics?: readonly unknown[] } = {},
): Registry {
  const disabled = new Set(opts.disabled ?? []);
  const holder = new Map<string, string>();
  const extensions: ExtensionInfo[] = [];
  const fileTypes: FileType[] = [];
  const viewers: Viewer[] = [];
  const grammars: Grammar[] = [];
  const commands: Command[] = [];
  const menus: MenuItem[] = [];
  const status: StatusItem[] = [];
  const panels: Panel[] = [];
  const keybindings: Binding[] = [];
  const metrics: Metric[] = [];
  const debug: DebugProvider[] = [];
  const refused: Refusal[] = [];
  // Every manifest is parsed first: any failure, even one that is not a refusal (a stack overflow,
  // a throwing source), refuses that manifest alone, and the essential extensions are known before
  // any claim is weighed.
  const parsed = sources.map((source) => {
    try {
      return { source, m: parseManifest(source) };
    } catch (error) {
      const reason = error instanceof ManifestError ? error.reason : "invalid";
      refused.push(freeze({ folder: source.folder, reason }));
      return undefined;
    }
  });
  // An essential type's extensions also cover every longer name ending in them (`main.typ`,
  // `.x.typ`): another extension claiming one would take the essential type's files.
  const essentialSuffix = new Map(
    parsed.flatMap((p) =>
      p?.m.essential
        ? p.m.contributes.fileTypes.flatMap((t) => t.extensions.map((e) => [e, p.m.id] as const))
        : [],
    ),
  );
  const coveredBy = (m: ReturnType<typeof parseManifest>): [string, string] | undefined => {
    for (const t of m.contributes.fileTypes)
      for (const [kind, names] of [
        ["name", t.filenames.map((n) => n.toLowerCase())],
        ["extension", t.extensions],
      ] as const)
        for (const name of names)
          for (let at = name.indexOf(".", 1); at > 0; at = name.indexOf(".", at + 1)) {
            const by = essentialSuffix.get(name.slice(at));
            if (by !== undefined && by !== m.id) return [`${kind} ${name}`, by];
          }
    return undefined;
  };
  for (const p of parsed) {
    if (!p) continue;
    const { source, m } = p;
    if (disabled.has(m.id) && !m.essential) continue;
    const claims = claimsOf(m);
    // Held by an earlier extension, claimed twice by this one, or inside an essential suffix.
    const covered = coveredBy(m);
    const taken =
      covered?.[0] ?? claims.find((claim, i) => holder.has(claim) || claims.indexOf(claim) !== i);
    if (taken !== undefined) {
      const by = covered?.[1] ?? holder.get(taken) ?? m.id;
      refused.push(
        freeze({
          folder: source.folder,
          reason: `conflict on ${taken}`,
          with: Object.freeze([by, m.id]),
        }),
      );
      continue;
    }
    for (const claim of claims) holder.set(claim, m.id);
    const owned = { extension: m.id, folder: source.folder, origin: source.origin };
    extensions.push(
      freeze({
        id: m.id,
        version: m.version,
        origin: source.origin,
        folder: source.folder,
        essential: m.essential,
        activation: m.activation,
      }),
    );
    for (const t of m.contributes.fileTypes) fileTypes.push(freeze({ ...t, ...owned }));
    for (const v of m.contributes.viewers) viewers.push(freeze({ ...v, ...owned }));
    for (const g of m.contributes.grammars) grammars.push(freeze({ ...g, ...owned }));
    for (const x of m.contributes.commands) commands.push(freeze({ ...x, ...owned }));
    for (const x of m.contributes.keybindings) keybindings.push(freeze({ ...x, ...owned }));
    for (const x of m.contributes.metrics)
      metrics.push(freeze({ ...x, extension: m.id, from: "manifest" as const }));
    for (const x of m.contributes.debug) debug.push(freeze({ ...x, ...owned }));
    // A protected group is the core's, a reserved location has no host yet: another origin's
    // item there is kept out and listed (ND-3); a built-in's was refused with its manifest.
    const keptOut = (x: MenuContribution): string | undefined => {
      const spec = locationOf(x.location);
      if (source.origin === "builtin" || !spec) return undefined;
      if (spec.reserved !== undefined) return `reserved location ${x.location}`;
      return spec.groups.find((g) => g.id === x.group)?.protected
        ? `protected group ${x.group}`
        : undefined;
    };
    for (const x of m.contributes.menus) {
      const why = keptOut(x);
      if (why)
        refused.push(
          freeze({ folder: source.folder, reason: why, with: Object.freeze([x.command]) }),
        );
      else menus.push(freeze({ ...x, ...owned }));
    }
    for (const x of m.contributes.status) status.push(freeze({ ...x, ...owned }));
    for (const x of m.contributes.panels) panels.push(freeze({ ...x, ...owned }));
  }

  const commandById = new Map(commands.map((x) => [x.id, x]));
  // A status item's click runs a command of any extension: one that no manifest gives is refused.
  const shownStatus = status.filter((x) => {
    if (x.command === undefined || commandById.has(x.command)) return true;
    refused.push(
      freeze({
        folder: x.folder,
        reason: `unknown command ${x.command}`,
        with: Object.freeze([x.id]),
      }),
    );
    return false;
  });
  const known = new Set(commandById.keys());
  // Each system's defaults are resolved when first asked, once: a start resolves only its own.
  const defaults = new Map<System, Resolution>();
  const defaultsOf = (system: System): Resolution => {
    let r = defaults.get(system);
    if (!r) {
      r = resolve(keybindings, system, [], known);
      defaults.set(system, r);
    }
    return r;
  };
  const cmp = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);
  /** Fixed group places, then `order`, extension and command: never the build order (L-2). */
  const placed = <T extends { group: string; order: number; extension: string }>(
    items: readonly T[],
    groups: readonly GroupSpec[],
    name: (x: T) => string,
  ): readonly T[] => {
    const at = (x: T) => groups.findIndex((g) => g.id === x.group);
    return Object.freeze(
      [...items].sort(
        (a, b) =>
          at(a) - at(b) ||
          a.order - b.order ||
          cmp(a.extension, b.extension) ||
          cmp(name(a), name(b)),
      ),
    );
  };
  const grouped = new Map<string, MenuItem[]>();
  for (const x of menus) {
    const list = grouped.get(x.location);
    if (list) list.push(x);
    else grouped.set(x.location, [x]);
  }
  const byLocation = new Map<string, readonly MenuItem[]>(
    [...grouped].map(([loc, items]) => [
      loc,
      placed(items, locationOf(loc)?.groups ?? [], (x) => x.command),
    ]),
  );
  const noItems: readonly MenuItem[] = Object.freeze([]);
  const statusItems = Object.freeze(
    (Object.keys(STATUS) as (keyof typeof STATUS)[]).flatMap((side) =>
      placed(
        shownStatus.filter((x) => x.side === side),
        STATUS[side].groups,
        (x) => x.id,
      ),
    ),
  );

  // A side has no groups: the core's (an essential built-in's) panels come first, then the rest.
  const core = new Set(
    extensions.filter((e) => e.origin === "builtin" && e.essential).map((e) => e.id),
  );
  const sideGroups: readonly GroupSpec[] = [{ id: "core" }, { id: "extensions" }];
  const bySide = new Map<Side, readonly Panel[]>(
    (Object.keys(SIDES) as Side[]).map((side) => [
      side,
      Object.freeze(
        placed(
          panels
            .filter((x) => x.side === side)
            .map((x) => ({
              x,
              group: core.has(x.extension) ? "core" : "extensions",
              order: x.order,
              extension: x.extension,
            })),
          sideGroups,
          (y) => y.x.id,
        ).map((y) => y.x),
      ),
    ]),
  );

  const byId = new Map(fileTypes.map((t) => [t.id, t]));
  const byName = new Map(
    fileTypes.flatMap((t) => t.filenames.map((n) => [n.toLowerCase(), t] as const)),
  );
  const byExtension = new Map(fileTypes.flatMap((t) => t.extensions.map((e) => [e, t] as const)));
  const byInterpreter = new Map(fileTypes.flatMap((t) => t.shebangs.map((s) => [s, t] as const)));
  const grammarByName = new Map(
    grammars.flatMap((g) => [g.id, ...g.names].map((n) => [n, g] as const)),
  );

  const ownViewer = (type: FileType) =>
    viewers.find((v) => v.extension === type.extension && v.fileTypes.includes(type.id));
  const essential = (id: string): { type: FileType; viewer: Viewer } => {
    const type = byId.get(id);
    const viewer = type && ownViewer(type);
    if (!type || !viewer)
      throw new Error(`registry: the essential type ${id} or its viewer is missing`);
    return { type, viewer };
  };
  const text = essential(TEXT_TYPE);
  const binary = essential(BINARY_TYPE);
  const essentialViewer = (storage: Storage) => (storage === "text" ? text.viewer : binary.viewer);

  const typeOf = (name: string, hints: NameHints = {}): FileType | undefined => {
    const chosen = hints.choice === undefined ? undefined : byId.get(hints.choice);
    if (chosen) return chosen;
    const lower = baseName(name).toLowerCase();
    const whole = byName.get(lower);
    if (whole) return whole;
    // The longest claimed suffix: `.tar.gz` before `.gz`. A name is never only its extension.
    for (let at = lower.indexOf(".", 1); at > 0; at = lower.indexOf(".", at + 1)) {
      const type = byExtension.get(lower.slice(at));
      if (type) return type;
    }
    const word = hints.firstLine === undefined ? undefined : interpreter(hints.firstLine);
    return word === undefined ? undefined : byInterpreter.get(word);
  };

  const viewersFor = (type: FileType) => {
    const own = ownViewer(type) ?? essentialViewer(type.storage);
    const others = viewers.filter(
      (v) => v !== own && v.extension !== type.extension && v.fileTypes.includes(type.id),
    );
    // Anything that is text can be read as text (ND-7): the editor is always offered for it.
    if (type.storage === "text" && own !== text.viewer && !others.includes(text.viewer))
      others.push(text.viewer);
    return freeze({ default: own, alternatives: Object.freeze(others) });
  };

  const opening = (
    type: FileType,
    viewer: Viewer,
    extra: { content?: ContentClass; notice?: "not-text"; chosen?: boolean } = {},
  ): Opening =>
    freeze({
      type,
      viewer,
      alternatives: extra.notice ? Object.freeze([]) : viewersFor(type).alternatives,
      ...(extra.content ? { content: extra.content } : {}),
      ...(extra.notice ? { notice: extra.notice } : {}),
      ...(extra.chosen ? { chosen: true } : {}),
    });

  const open = (
    name: string,
    hints: NameHints & { content?: ContentClass | undefined } = {},
  ): Opening => {
    const type = typeOf(name, hints);
    const chosen = type !== undefined && type.id === hints.choice;
    if (type?.storage === "blob") return opening(type, viewersFor(type).default, { chosen });
    const content = hints.content;
    if (content === undefined)
      throw new Error(`registry: ${name} must be read as text before it opens`);
    if (!type)
      return content === "text"
        ? opening(text.type, text.viewer, { content })
        : opening(binary.type, binary.viewer, { content });
    if (content === "binary")
      return opening(type, binary.viewer, { content, notice: "not-text", chosen });
    return opening(type, viewersFor(type).default, { content, chosen });
  };

  const loads = new Map<string, Promise<unknown>>();
  const load = (c: Viewer | Grammar | DebugProvider | StatusItem | Panel): Promise<unknown> => {
    const key = `${c.origin}:${c.folder}/${c.entry}`;
    let p = loads.get(key);
    if (!p) {
      // A failed load is not kept: the next ask tries again (a drawer's panel, EX0c2 review).
      p = loader(c.folder, c.entry).catch((error: unknown) => {
        loads.delete(key);
        throw error;
      });
      loads.set(key, p);
    }
    return p;
  };

  const supports = new Map<string, Promise<LanguageSupport | undefined>>();
  const loaded = new Map<string, LanguageSupport>();
  let descriptions: readonly LanguageDescription[] | undefined;
  const codeLanguages = (): readonly LanguageDescription[] => {
    descriptions ??= Object.freeze(
      grammars.map((g) =>
        LanguageDescription.of({
          name: g.id,
          alias: [...g.names],
          load: async () => {
            const support = await languageSupport(g.id);
            if (!support) throw new Error(`grammar ${g.id} did not load`);
            return support;
          },
        }),
      ),
    );
    return descriptions;
  };
  const languageSupport = (id: string): Promise<LanguageSupport | undefined> => {
    const g = grammarByName.get(id);
    if (!g) return Promise.resolve(undefined);
    let p = supports.get(g.id);
    if (!p) {
      p = load(g)
        .then((m) => {
          const support = (m as GrammarModule).grammar({ codeLanguages: codeLanguages() });
          loaded.set(g.id, support);
          return support;
        })
        .catch((error: unknown) => {
          console.warn(`grammar ${g.id}:`, error instanceof Error ? error.message : "failed");
          return undefined;
        });
      supports.set(g.id, p);
    }
    return p;
  };

  // A viewer that failed to load, and the essential viewer that stands in for it from then on.
  const substitutes = new Map<Viewer, Viewer>();
  const loadViewer = async (o: Opening, notify: () => void) => {
    const standIn = substitutes.get(o.viewer);
    if (standIn) return { viewer: standIn, module: await load(standIn) };
    try {
      return { viewer: o.viewer, module: await load(o.viewer) };
    } catch {
      const fallback = essentialViewer(o.content === "text" ? "text" : "blob");
      if (fallback === o.viewer) throw new Error(`viewer ${o.viewer.id} did not load`);
      if (!substitutes.has(o.viewer)) {
        substitutes.set(o.viewer, fallback);
        notify();
      }
      return { viewer: fallback, module: await load(fallback) };
    }
  };

  const folderOf = new Map(extensions.map((e) => [e.id, e.folder]));
  const view = mergeGoMetrics(metrics, folderOf, opts.goMetrics ?? []);
  refused.push(...view.refused);
  const metricById = new Map(view.metrics.map((x) => [x.id, x]));

  // A debug provider's entry is loaded on its first read only, once like any entry.
  const readDebug = async (p: DebugProvider): Promise<DebugRead> => {
    let read: DebugRead;
    try {
      read = checkRead(await ((await load(p)) as DebugModule).read(), p.fields);
    } catch {
      console.warn(`debug ${p.id}: the read failed`);
      return Object.freeze({ values: Object.freeze(Object.create(null)), dropped: 1 });
    }
    if (read.dropped > 0) console.warn(`debug ${p.id}: ${read.dropped} values broke rule V`);
    return read;
  };

  const commandLoads = new Map<string, Promise<unknown>>();
  const commandModules = new Map<string, unknown>();
  const loadCommand = (c: Command): Promise<unknown> | undefined => {
    if (c.entry === undefined) return undefined;
    const key = `${c.origin}:${c.folder}/${c.entry}`;
    let p = commandLoads.get(key);
    if (!p) {
      p = loader(c.folder, c.entry).then((m) => {
        commandModules.set(key, m);
        return m;
      });
      commandLoads.set(key, p);
    }
    return p;
  };

  return freeze({
    metrics: view.metrics,
    metric: (id: string) => metricById.get(id),
    withGoMetrics: (descriptors: readonly unknown[]) =>
      mergeGoMetrics(metrics, folderOf, [...(opts.goMetrics ?? []), ...descriptors]),
    debug: Object.freeze(debug),
    readDebug,
    commands: Object.freeze(commands),
    command: (id: string) => commandById.get(id),
    keybindings: Object.freeze(keybindings),
    bindings: (system: System, user: readonly UserBinding[] = []) =>
      user.length === 0 ? defaultsOf(system) : resolve(keybindings, system, user, known),
    menu: (location: string) => byLocation.get(location) ?? noItems,
    status: statusItems,
    loadStatus: (item: StatusItem) => load(item),
    panels: (side: Side) => bySide.get(side) ?? Object.freeze([]),
    loadPanel: (p: Panel) => load(p),
    loadCommand,
    loadedCommand: (c: Command) => commandModules.get(`${c.origin}:${c.folder}/${c.entry}`),
    extensions: Object.freeze(extensions),
    fileTypes: Object.freeze(fileTypes),
    viewers: Object.freeze(viewers),
    grammars: Object.freeze(grammars),
    refused: Object.freeze(refused),
    fileType: (id: string) => byId.get(id),
    typeOf,
    needsContent: (name: string, hints?: NameHints) => typeOf(name, hints)?.storage !== "blob",
    open,
    viewersFor,
    essentialViewer,
    grammar: (name: string) => grammarByName.get(name),
    codeLanguages,
    languageSupport,
    loadedSupport: (id: string) => {
      const g = grammarByName.get(id);
      return g ? loaded.get(g.id) : undefined;
    },
    load,
    hasFailed: (viewer: Viewer) => substitutes.has(viewer),
    loadViewer,
  });
}
