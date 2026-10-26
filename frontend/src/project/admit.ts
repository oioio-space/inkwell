import { next as A } from "@automerge/automerge";
import { isValidAutomergeUrl } from "@automerge/automerge-repo";
import { HOLD } from "../net/coverage";
import {
  type IndexDoc,
  type IndexNode,
  isHash,
  isId,
  isPackageKey,
  MAX_CREATE_PER_CHANGE,
  MAX_NODES,
  plain,
  plainValue,
} from "./model";
import { MAX_DEPTH, validateName } from "./names";
import { deletionMode } from "./ops";

/**
 * Admission of the changes of the project index (ADR 0023, B5; same chain as ADR 0022).
 *
 * T10 ({@link ../net/coverage}) already keeps out the changes no current writer vouches for.
 * What it cannot see is a writer who forges a change that a client of Inkwell would never make:
 * an invalid name, a new url for a file, a deletion the rules reserve to admins. These are pure
 * functions of the change and of the index it applies to, so every replica refuses the same
 * ones. A refused change is simply not delivered, as one that nobody vouches for.
 */

/**
 * Why a change is refused; undefined when it is admitted. A reason that starts with {@link HOLD}
 * is not a refusal: the change must wait (for an admin's attestation, for what it builds on) and
 * is judged again later.
 */
export type Refusal = string | undefined;

export { HOLD };

const NODE_FIELDS: ReadonlySet<string> = new Set([
  "type",
  "name",
  "parent",
  "url",
  "hash",
  "size",
  "by",
  "at",
  "trashed",
  "deletion",
  "who",
  "ftype",
]);
const TOP_KEYS: ReadonlySet<string> = new Set([
  "text",
  "schema",
  "main",
  "nodes",
  "packages",
  "settings",
  "decisions",
]);

const isObj = (v: unknown): v is Record<string, unknown> =>
  v !== null && typeof v === "object" && !Array.isArray(v);
const isStamp = (v: unknown): boolean =>
  isObj(v) &&
  typeof v.by === "string" &&
  v.by.length > 0 &&
  v.by.length <= 64 &&
  typeof v.at === "number" &&
  Number.isFinite(v.at);

/** The problem with the shape of a whole node, or undefined. Used at creation and on load. */
export function nodeProblem(id: string, n: unknown): Refusal {
  if (!isId(id)) return "bad node id";
  if (!isObj(n)) return "node is not a map";
  for (const k of Object.keys(n)) if (!NODE_FIELDS.has(k)) return `unknown node field ${k}`;
  if (n.type !== "dir" && n.type !== "text" && n.type !== "blob") return "bad node type";
  if (typeof n.name !== "string" || validateName(n.name) !== undefined) return "invalid name";
  if (n.parent !== null && !isId(n.parent)) return "bad parent";
  if (!isStamp(n)) return "bad stamp";
  if (
    n.ftype !== undefined &&
    !(typeof n.ftype === "string" && /^[a-z0-9-]{1,32}$/.test(n.ftype))
  ) {
    return "bad file type";
  }
  if (n.who !== undefined && (typeof n.who !== "string" || validateName(n.who) !== undefined)) {
    return "bad author name";
  }
  if (n.type === "text") {
    if (typeof n.url !== "string" || n.url.length > 255 || !isValidAutomergeUrl(n.url)) {
      return "bad url";
    }
  } else if (n.url !== undefined) return "url on a non-text node";
  if (n.type === "blob") {
    if (!isHash(n.hash)) return "bad hash";
    if (n.size !== undefined && !(Number.isSafeInteger(n.size) && (n.size as number) >= 0)) {
      return "bad size";
    }
  } else if (n.hash !== undefined || n.size !== undefined) return "blob field on a non-blob node";
  if (n.trashed !== undefined && n.trashed !== null && !stampProblemFree(n.trashed, true)) {
    return "bad trash mark";
  }
  if (n.deletion !== undefined && n.deletion !== null && !requestOk(n.deletion)) {
    return "bad deletion request";
  }
  return undefined;
}

function stampProblemFree(v: unknown, optionalReq: boolean): boolean {
  if (!isStamp(v)) return false;
  const req = (v as { req?: unknown }).req;
  return req === undefined ? optionalReq : isId(req);
}

function requestOk(v: unknown): boolean {
  return stampProblemFree(v, false);
}

function decisionOk(v: unknown): boolean {
  return (
    isStamp(v) &&
    isId((v as { req?: unknown }).req) &&
    (v as { verdict?: unknown }).verdict === "refused"
  );
}

function packageOk(k: string, v: unknown): boolean {
  return (
    isPackageKey(k) &&
    isStamp(v) &&
    isHash((v as { hash?: unknown }).hash) &&
    Number.isSafeInteger((v as { size?: unknown }).size)
  );
}

/**
 * Checks a whole index: every node, package and decision well formed. It cannot know who made
 * what, so the rules of authority are not part of it. Used for a document that arrives in one
 * piece, and for the index a person already has on disk.
 */
export function checkDocument(doc: IndexDoc): Refusal {
  const d = doc as Record<string, unknown>;
  for (const k of Object.keys(d)) if (!TOP_KEYS.has(k)) return `unknown key ${k}`;
  if (d.schema !== undefined && d.schema !== 1) return "unknown schema";
  if (d.main !== undefined && !isId(d.main)) return "bad main";
  if (d.text !== undefined && typeof d.text !== "string") return "bad text";
  if (d.nodes !== undefined) {
    if (!isObj(d.nodes)) return "bad nodes";
    for (const [id, n] of Object.entries(d.nodes)) {
      const p = nodeProblem(id, n);
      if (p) return p;
    }
  }
  if (d.settings !== undefined) {
    if (
      !isObj(d.settings) ||
      (d.settings.deletion !== "approval" && d.settings.deletion !== "free")
    ) {
      return "bad settings";
    }
  }
  if (d.decisions !== undefined) {
    if (!isObj(d.decisions)) return "bad decisions";
    for (const [id, v] of Object.entries(d.decisions)) {
      if (!isId(id) || !decisionOk(v)) return "bad decision";
    }
  }
  if (d.packages !== undefined) {
    if (!isObj(d.packages)) return "bad packages";
    for (const [k, v] of Object.entries(d.packages)) if (!packageOk(k, v)) return "bad package";
  }
  return undefined;
}

type Patch = A.Patch;

/** What a verdict is made from (see {@link checkChange}). */
export interface Judgement {
  /** What the change did, from its dependencies to itself. */
  forward: readonly Patch[];
  /**
   * True if the value at `path` (a top-level key, or a node of `nodes`) existed before the change,
   * that is, if the change did not create it.
   */
  existedBefore: (...path: string[]) => boolean;
  /** Whether an admin vouches for the change. */
  admin: boolean;
  /**
   * The index as the change leaves it, read when a rule needs more than the patches say (a
   * rename does not; a creation, a move, a deletion do). Never the receiver's current state: the
   * dependencies of the change and the change itself.
   */
  after: () => IndexDoc;
  /** A node the change created, as it left it. */
  createdNode: (id: string) => IndexNode | undefined;
  /** True if the index has more nodes than it may after the change. */
  tooMany: () => boolean;
}

const pathKey = (path: readonly unknown[]): string => path.join("\u0000");

/** Fields of an existing node whose new value the patches alone settle. */
const PATCHED_FIELDS: ReadonlySet<string> = new Set(["name", "parent"]);

/**
 * Checks the effect of one change. The rules:
 *
 * - names, urls, hashes, ids and shapes are always valid, and a file's type, url and author
 *   never change (PRJ-10, PRJ-15);
 * - `trashed` is a writer's act only when the project lets files go freely, an admin's
 *   otherwise, and the main file is never trashed (FM-14, FM-15, FM-18);
 * - a deletion request is a writer's act, lifting it is an admin's (under "approval");
 * - decisions, the main file, the deletion rule and the purge of a node are an admin's;
 * - a file is not moved or created into the trash, and the tree stays under its caps.
 *
 * A rule that needs an admin gives a reason that starts with {@link HOLD}: the change waits for
 * an admin's attestation instead of being refused. The verdict is a function of the change and
 * its own history, so replicas that get the same changes in any order agree.
 */
export function checkChange(j: Judgement): Refusal {
  const needAdmin = (what: string): Refusal =>
    j.admin ? undefined : `${HOLD}${what} needs an admin`;
  const existedBefore = j.existedBefore;
  const written = new Map<string, unknown>();
  const touched = new Map<string, Set<string>>();
  const created = new Set<string>();
  const purged = new Set<string>();
  let sections = false;
  for (const p of j.forward) {
    if (p.action === "conflict") continue;
    const [root, a, b] = p.path as [string, string?, string?];
    if (typeof root !== "string" || !TOP_KEYS.has(root)) return `unknown key ${String(root)}`;
    if (p.action === "put")
      written.set(pathKey(p.path), plainValue((p as { value?: unknown }).value));
    switch (root) {
      case "text":
        break;
      case "schema":
        if (existedBefore("schema")) return "schema is fixed";
        if (j.after().schema !== 1) return "unknown schema";
        break;
      case "main": {
        if (existedBefore("main")) {
          const r = needAdmin("changing the main file");
          if (r) return r;
        }
        sections = true;
        break;
      }
      case "nodes": {
        if (a === undefined) {
          if (p.action !== "put" || existedBefore("nodes")) return "nodes replaced";
          break;
        }
        if (!isId(a)) return "bad node id";
        if (b === undefined) {
          if (p.action === "del") {
            const r = needAdmin("purging a file");
            if (r) return r;
            purged.add(a);
          } else if (p.action === "put") {
            if (existedBefore("nodes", a)) return "node replaced";
            created.add(a);
          } else return "bad node patch";
          break;
        }
        if (!NODE_FIELDS.has(b)) return `unknown node field ${b}`;
        const set = touched.get(a) ?? new Set<string>();
        touched.set(a, set);
        set.add(b);
        break;
      }
      case "settings": {
        if (existedBefore("settings")) {
          const r = needAdmin("changing the deletion rule");
          if (r) return r;
        }
        sections = true;
        break;
      }
      case "decisions": {
        if (existedBefore("decisions") || a !== undefined) {
          const r = needAdmin("deciding on a request");
          if (r) return r;
        }
        sections = true;
        break;
      }
      case "packages":
        if (p.action === "del") {
          const r = needAdmin("removing a package");
          if (r) return r;
        }
        sections = true;
        break;
    }
  }

  // Shapes. A new node is read whole; an existing one by what was written into it, and whole
  // only when a field the patches do not settle was written.
  for (const id of created) {
    const p = nodeProblem(id, j.createdNode(id));
    if (p) return p;
  }
  for (const [id, fields] of touched) {
    if (created.has(id) || purged.has(id) || fields.size === 0) continue;
    let whole = false;
    for (const f of fields) {
      if (!PATCHED_FIELDS.has(f)) {
        whole = true;
        continue;
      }
      const v = written.get(pathKey(["nodes", id, f]));
      if (f === "name" && (typeof v !== "string" || validateName(v) !== undefined)) {
        return "invalid name";
      }
      if (f === "parent" && v !== null && !isId(v)) return "bad parent";
    }
    if (whole) {
      const n = j.after().nodes?.[id];
      const p = n ? nodeProblem(id, n) : undefined;
      if (p) return p;
    }
  }
  if (sections) {
    const after = j.after();
    if (
      after.settings !== undefined &&
      after.settings.deletion !== "approval" &&
      after.settings.deletion !== "free"
    ) {
      return "bad settings";
    }
    for (const [id, v] of Object.entries(after.decisions ?? {})) {
      if (!isId(id) || !decisionOk(v)) return "bad decision";
    }
    for (const [k, v] of Object.entries(after.packages ?? {})) {
      if (!packageOk(k, v)) return "bad package";
    }
    if (after.main !== undefined && !isId(after.main)) return "bad main";
  }

  if (created.size > MAX_CREATE_PER_CHANGE) return "too many files in one change";
  if (created.size > 0 && j.tooMany()) return "too many files";
  for (const id of created) {
    const n = j.createdNode(id);
    if (n?.trashed || (n?.parent != null && inTrash(j.after(), n.parent))) {
      return "a file is not created in the trash";
    }
  }

  for (const [id, fields] of touched) {
    if (purged.has(id)) continue; // purged in the same change
    const fresh = created.has(id);
    for (const f of fields) {
      if (!fresh && (f === "type" || f === "url" || f === "by" || f === "at" || f === "who")) {
        return `${f} of a file never changes`;
      }
      if (f === "parent" && !fresh) {
        const after = j.after();
        if (inTrash(after, after.nodes?.[id]?.parent ?? null)) {
          return "a file is not moved into the trash";
        }
      }
      if (f === "trashed") {
        const after = j.after();
        if (after.nodes?.[id]?.trashed && id === after.main) {
          return "the main file cannot be trashed";
        }
        if (deletionMode(after) === "approval") {
          const r = needAdmin("trashing a file");
          if (r) return r;
        }
      }
      if (f === "deletion") {
        const after = j.after();
        if (!after.nodes?.[id]?.deletion && deletionMode(after) === "approval") {
          const r = needAdmin("lifting a deletion request");
          if (r) return r;
        }
      }
    }
  }
  return undefined;
}

/** True if the folder `parent`, or one above it, is in the trash. */
function inTrash(doc: IndexDoc, parent: string | null): boolean {
  let at = parent;
  for (let i = 0; at !== null && i <= MAX_DEPTH + 1; i++) {
    const n = doc.nodes?.[at];
    if (!n) return false;
    if (n.trashed) return true;
    at = n.parent ?? null;
  }
  return false;
}

/** The part of an index a verdict reads: the small sections whole, the nodes one by one. */
function lite(view: A.Doc<IndexDoc>): IndexDoc {
  const v = view as unknown as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const k of ["schema", "main", "settings", "decisions", "packages"]) {
    if (v[k] !== undefined) out[k] = plainValue(v[k]);
  }
  const nodes = v.nodes as Record<string, unknown> | undefined;
  if (nodes !== undefined) {
    const seen = new Map<string, unknown>();
    out.nodes = new Proxy(
      {},
      {
        get(_target, id) {
          if (typeof id !== "string") return undefined;
          if (!seen.has(id))
            seen.set(id, nodes[id] === undefined ? undefined : plainValue(nodes[id]));
          return seen.get(id);
        },
      },
    );
  }
  return out as IndexDoc;
}

/**
 * True if the change only makes nodes that are new (none of `unseen` was named before), and what
 * is inside them: every operation is the creation of a new node in the nodes map, or happens in
 * an object that the change itself made. Nothing a concurrent change did can have hidden any of
 * it.
 */
function createsOnly(
  work: A.Doc<IndexDoc>,
  decoded: ReturnType<typeof A.decodeChange>,
  unseen: ReadonlySet<string>,
): boolean {
  const nodesObj = A.getObjectId(work, "nodes");
  if (nodesObj === null || unseen.size === 0) return false;
  const made = new Set<string>();
  decoded.ops.forEach((op, i) => {
    if (op.action.startsWith("make")) made.add(`${decoded.startOp + i}@${decoded.actor}`);
  });
  return decoded.ops.every(
    (op) =>
      made.has(op.obj) ||
      (op.obj === nodesObj &&
        op.action.startsWith("make") &&
        typeof op.key === "string" &&
        unseen.has(op.key)),
  );
}

/** A patch that sets the name of a node. */
const isRename = (p: Patch): boolean =>
  p.action === "put" && p.path.length === 3 && p.path[0] === "nodes" && p.path[2] === "name";

/**
 * What the working copy says about the keys a change writes, read before the change is applied
 * (the copy is replaced by the application): which top-level sections and which of the nodes the
 * change names already existed.
 */
function factsBefore(
  work: A.Doc<IndexDoc>,
  ops: readonly { obj: string; key?: unknown }[],
): (...path: string[]) => boolean {
  const root = work as unknown as Record<string, unknown>;
  const sections = new Map<string, boolean>();
  for (const k of ["schema", "main", "settings", "decisions", "packages", "nodes"]) {
    sections.set(k, root[k] !== undefined);
  }
  const nodesObj = A.getObjectId(work, "nodes");
  const nodes = root.nodes as Record<string, unknown> | undefined;
  const had = new Set<string>();
  if (nodesObj !== null) {
    for (const op of ops) {
      if (op.obj === nodesObj && typeof op.key === "string" && nodes?.[op.key] !== undefined) {
        had.add(op.key);
      }
    }
  }
  return (...path) => {
    const [top, id] = path;
    if (top === undefined) return true;
    if (id !== undefined) return top === "nodes" ? had.has(id) : true;
    return sections.get(top) ?? true;
  };
}

/** Most changes whose verdict is kept (held or refused); past it new ones wait, untouched. */
const MAX_TRACKED = 100_000;

/**
 * The state of the checks for one index document: a working copy of the changes seen, so that a
 * change is judged without waiting for the Repo, which applies messages later. Local changes
 * reach it through `source`, the Repo's own copy.
 *
 * Every change is applied once to the one working copy, then judged on what it did
 * ({@link A.diff} between its dependencies and itself) and on the few nodes it touches, read at
 * its own dependencies and just after it ({@link A.view}), never against the state the receiver
 * happens to have reached: two replicas that get the same changes in any order agree. The cost
 * does not grow with the length of the history.
 *
 * The working copy also holds the changes that were refused or are held (a change cannot be
 * taken out of a document); their hashes are remembered, and whatever builds on one shares its
 * fate. Only a change judged fit is ever handed to the Repo.
 */
export class IndexValidator {
  #work: A.Doc<IndexDoc> | undefined;
  #source: () => A.Doc<IndexDoc> | undefined;
  /** Changes that wait: hash → why. Judged again each time they are offered. */
  #held = new Map<string, string>();
  /** Changes refused for good: hash → why. */
  #refused = new Map<string, string>();
  /** Every node id any change seen so far names (refused and purged ones too). */
  #ids = new Set<string>();

  constructor(source: () => A.Doc<IndexDoc> | undefined) {
    this.#source = source;
  }

  /** The heads of every change seen so far (refused ones included), sorted. */
  heads(): string[] {
    return [...A.getHeads(this.#sync())].sort();
  }

  /** Brings the working copy up to the Repo's own copy (local edits, and what the Repo applied). */
  #sync(): A.Doc<IndexDoc> {
    const live = this.#source();
    if (!this.#work) {
      this.#work = live ? A.clone(live) : A.init<IndexDoc>();
      this.#remember();
    } else if (live && !A.hasHeads(this.#work, A.getHeads(live))) {
      this.#work = A.merge(this.#work, A.clone(live));
      this.#remember();
    }
    return this.#work;
  }

  /** Takes the ids of the nodes of the working copy into the ones seen. */
  #remember(): void {
    const nodes = (this.#work as { nodes?: object } | undefined)?.nodes;
    for (const id of Object.keys(nodes ?? {})) this.#ids.add(id);
  }

  /**
   * Judges one change. `adminCovered` is asked only when a rule needs it. The answer is
   * undefined (admitted), a reason that starts with {@link HOLD} (wait: an admin has not vouched
   * yet, or what it builds on is not here or not admitted), or a refusal for good.
   */
  check(bytes: Uint8Array, adminCovered: (hash: string) => boolean): Refusal {
    let decoded: ReturnType<typeof A.decodeChange>;
    try {
      decoded = A.decodeChange(bytes);
    } catch {
      return "undecodable change";
    }
    const { hash, deps } = decoded;
    const refused = this.#refused.get(hash);
    if (refused !== undefined) return refused;
    let work = this.#sync();
    if (!A.hasHeads(work, deps)) return `${HOLD}dependencies unknown`;
    const known = A.hasHeads(work, [hash]);
    if (known && !this.#held.has(hash)) return undefined; // judged fit before
    for (const d of deps) {
      const why = this.#refused.get(d);
      if (why !== undefined) return this.#verdict(hash, `builds on a refused change (${why})`);
      if (this.#held.has(d)) return this.#verdict(hash, `${HOLD}builds on a change that waits`);
    }
    // A change made on exactly what the working copy holds leaves it as the change's own state:
    // its patches are what it did, and reading what it replaced costs nothing. Anything else
    // (a change made concurrently with others) is read at its own heads, which costs a pass over
    // the history, and is asked only for what a rule needs.
    const heads = A.getHeads(work);
    const linear = !known && heads.length === deps.length && deps.every((d) => heads.includes(d));
    if (!known && this.#held.size + this.#refused.size >= MAX_TRACKED) {
      return `${HOLD}too many changes wait`;
    }
    const textObj = A.getObjectId(work, "text");
    if (textObj && decoded.ops.length > 0 && decoded.ops.every((op) => op.obj === textObj)) {
      // A burst of typing: every operation is in the text of the main file.
      if (!known) this.#work = A.applyChanges(work, [bytes])[0];
      return undefined;
    }
    const after = [...deps, hash];
    let forward: Patch[] = [];
    let existedBefore: (...path: string[]) => boolean;
    let state: A.Doc<IndexDoc> | undefined;
    let fresh = false; // a change that only creates nodes no other change has named
    let unseen = new Set<string>();
    if (linear) {
      existedBefore = factsBefore(work, decoded.ops);
      work = A.applyChanges(work, [bytes], {
        patchCallback: (patches) => {
          forward = patches;
        },
      })[0];
      this.#work = work;
      state = work;
      unseen = this.#unseen(work, decoded);
    } else {
      let seen: Patch[] | undefined;
      if (!known) {
        work = A.applyChanges(work, [bytes], {
          patchCallback: (patches) => {
            seen = patches;
          },
        })[0];
        this.#work = work;
        unseen = this.#unseen(work, decoded);
        fresh = createsOnly(work, decoded, unseen);
      }
      // What the application reported is the change's effect on the merged state, enough when
      // nothing the change wrote can have been hidden by a concurrent write: it renames nodes, or
      // it only creates nodes nobody else has named (what it writes is all new). Anything else
      // is read exactly, between the change's own heads, which costs a pass over the history.
      const cheap =
        seen !== undefined &&
        (fresh || (seen.length === decoded.ops.length && seen.every(isRename)));
      forward = cheap && seen ? seen : A.diff(work, deps, after);
      let undone: Map<string, Patch> | undefined;
      existedBefore = (...path) => {
        if (path[0] === "nodes" && path.length === 2 && unseen.has(path[1] as string)) return false;
        if (!undone) {
          undone = new Map();
          for (const p of A.diff(work, after, deps)) {
            const k = pathKey(p.path);
            if (!undone.has(k)) undone.set(k, p);
          }
        }
        const gone = undone;
        // Created with its parent: the deletion is reported once, at the parent.
        return !path.some((_, i) => gone.get(pathKey(path.slice(0, i + 1)))?.action === "del");
      };
    }
    const pinned = work;
    let liteAfter: IndexDoc | undefined;
    const stateAfter = (): A.Doc<IndexDoc> => (state ??= A.view(pinned, after));
    const named = this.#ids.size + unseen.size;
    const verdict = checkChange({
      forward,
      existedBefore,
      admin: adminCovered(hash),
      after: () => {
        liteAfter ??= lite(stateAfter());
        return liteAfter;
      },
      createdNode: (id) => {
        if (linear || fresh) {
          const n = (pinned as { nodes?: Record<string, unknown> }).nodes?.[id];
          return n === undefined ? undefined : (plainValue(n) as IndexNode);
        }
        liteAfter ??= lite(stateAfter());
        return liteAfter.nodes?.[id];
      },
      // Nodes in the index after the change never exceed the ones ever named: only past that
      // many is the exact count worth a pass over the history.
      tooMany: () =>
        named > MAX_NODES &&
        Object.keys((stateAfter() as { nodes?: object }).nodes ?? {}).length > MAX_NODES,
    });
    for (const id of unseen) this.#ids.add(id);
    return this.#verdict(hash, verdict);
  }

  /** The node ids the change names that no earlier change did. */
  #unseen(work: A.Doc<IndexDoc>, decoded: ReturnType<typeof A.decodeChange>): Set<string> {
    const out = new Set<string>();
    const nodesObj = A.getObjectId(work, "nodes");
    if (nodesObj === null) return out;
    for (const op of decoded.ops) {
      if (op.obj === nodesObj && typeof op.key === "string" && !this.#ids.has(op.key)) {
        out.add(op.key);
      }
    }
    return out;
  }

  /** Remembers the verdict on a change (a refusal for good, or that it waits). */
  #verdict(hash: string, verdict: Refusal): Refusal {
    if (verdict === undefined) {
      this.#held.delete(hash);
    } else if (verdict.startsWith(HOLD)) {
      this.#held.set(hash, verdict);
    } else {
      this.#held.delete(hash);
      this.#refused.set(hash, verdict);
    }
    return verdict;
  }

  /**
   * Judges a document that arrives whole: its shape, then each of its changes in causal order,
   * as if they had come one by one. It is admitted only if every change is; a change that waits
   * makes the whole document wait.
   */
  checkDocument(doc: A.Doc<IndexDoc>, adminCovered: (hash: string) => boolean): Refusal {
    const shape = checkDocument(plain(doc));
    if (shape !== undefined) return shape;
    let waiting: Refusal;
    for (const bytes of A.getAllChanges(doc)) {
      const verdict = this.check(bytes, adminCovered);
      if (verdict === undefined) continue;
      if (!verdict.startsWith(HOLD)) return verdict;
      waiting ??= verdict;
    }
    return waiting;
  }
}
