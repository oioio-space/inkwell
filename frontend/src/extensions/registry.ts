/**
 * The registry of contributions (ADR 0027): built once from manifests alone, never by running an
 * extension's code, then frozen. It answers what a file is (its type, from its name), what shows
 * it (a viewer, and the alternatives offered in the view bar) and which grammar colours it.
 * Entries are loaded on demand, once each, through the loader it was given.
 */
import { LanguageDescription, type LanguageSupport } from "@codemirror/language";
import {
  type FileTypeContribution,
  type GrammarContribution,
  ManifestError,
  type ManifestSource,
  type Origin,
  parseManifest,
  type Storage,
  type ViewerContribution,
} from "./manifest";

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
  ];
}

/**
 * Builds the registry from manifest sources, in their order: a manifest whose claim (a type, an
 * extension, a whole name, an interpreter, an id) is already held is refused, naming both. The
 * ones in `disabled` are left out unless essential. No entry is loaded here.
 */
export function buildRegistry(
  sources: readonly ManifestSource[],
  loader: EntryLoader,
  opts: { disabled?: Iterable<string> } = {},
): Registry {
  const disabled = new Set(opts.disabled ?? []);
  const holder = new Map<string, string>();
  const extensions: ExtensionInfo[] = [];
  const fileTypes: FileType[] = [];
  const viewers: Viewer[] = [];
  const grammars: Grammar[] = [];
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
  }

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
  const load = (c: Viewer | Grammar): Promise<unknown> => {
    const key = `${c.origin}:${c.folder}/${c.entry}`;
    let p = loads.get(key);
    if (!p) {
      p = loader(c.folder, c.entry);
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

  return freeze({
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
