import { next as A } from "@automerge/automerge";
import { isValidAutomergeUrl } from "@automerge/automerge-repo";
import {
  type IndexDoc,
  isHash,
  isId,
  isPackageKey,
  MAX_CREATE_PER_CHANGE,
  MAX_NODES,
  plain,
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

/** Why a change is refused; undefined when it is admitted. */
export type Refusal = string | undefined;

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

/**
 * Checks the effect of one change: `patches` is what it did, from `before` to `after`, and
 * `admin` says whether an admin vouches for it. The rules:
 *
 * - names, urls, hashes, ids and shapes are always valid, and a file's type, url and author
 *   never change (PRJ-10, PRJ-15);
 * - `trashed` is a writer's act only when the project lets files go freely, an admin's
 *   otherwise, and the main file is never trashed (FM-14, FM-15, FM-18);
 * - a deletion request is a writer's act, lifting it is an admin's (under "approval");
 * - decisions, the main file, the deletion rule and the purge of a node are an admin's;
 * - a file is not moved or created into the trash, and the tree stays under its caps.
 *
 * `before` is the index as the change's own dependencies left it, never the receiver's current
 * state: the verdict is a function of the change and its history alone, so replicas that get
 * the same changes in any order agree.
 */
export function checkChange(
  patches: readonly Patch[],
  before: IndexDoc,
  after: IndexDoc,
  admin: boolean,
): Refusal {
  const needAdmin = (what: string): Refusal => (admin ? undefined : `${what} needs an admin`);
  const mode = deletionMode(after);
  const touched = new Map<string, Set<string>>();
  const created = new Set<string>();
  for (const p of patches) {
    if (p.action === "conflict") continue;
    const [root, a, b] = p.path as [string, string?, string?];
    if (typeof root !== "string" || !TOP_KEYS.has(root)) return `unknown key ${String(root)}`;
    switch (root) {
      case "text":
        break;
      case "schema":
        if (before.schema !== undefined) return "schema is fixed";
        if (after.schema !== 1) return "unknown schema";
        break;
      case "main": {
        if (before.main !== undefined) {
          const r = needAdmin("changing the main file");
          if (r) return r;
        }
        if (after.main !== undefined && !isId(after.main)) return "bad main";
        break;
      }
      case "nodes": {
        if (a === undefined) {
          if (p.action !== "put" || before.nodes !== undefined) return "nodes replaced";
          break;
        }
        if (!isId(a)) return "bad node id";
        if (b === undefined) {
          if (p.action === "del") {
            const r = needAdmin("purging a file");
            if (r) return r;
          } else if (p.action === "put") {
            if (before.nodes?.[a] !== undefined) return "node replaced";
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
        if (before.settings !== undefined) {
          const r = needAdmin("changing the deletion rule");
          if (r) return r;
        }
        break;
      }
      case "decisions": {
        if (before.decisions !== undefined || a !== undefined) {
          const r = needAdmin("deciding on a request");
          if (r) return r;
        }
        break;
      }
      case "packages":
        if (p.action === "del") {
          const r = needAdmin("removing a package");
          if (r) return r;
        }
        break;
    }
  }

  const problem = checkSections(after, created, touched);
  if (problem) return problem;

  if (created.size > MAX_CREATE_PER_CHANGE) return "too many files in one change";
  if (created.size > 0 && Object.keys(after.nodes ?? {}).length > MAX_NODES) {
    return "too many files";
  }
  for (const id of created) {
    if (inTrash(after, after.nodes?.[id]?.parent ?? null)) {
      return "a file is not created in the trash";
    }
  }

  for (const [id, fields] of touched) {
    const was = before.nodes?.[id];
    const now = after.nodes?.[id];
    if (!now) continue; // purged in the same change
    const fresh = was === undefined;
    for (const f of fields) {
      if (!fresh && (f === "type" || f === "url" || f === "by" || f === "at" || f === "who")) {
        return `${f} of a file never changes`;
      }
      if (f === "parent" && !fresh && inTrash(after, now.parent ?? null)) {
        return "a file is not moved into the trash";
      }
      if (f === "trashed") {
        if (now.trashed && id === after.main) return "the main file cannot be trashed";
        if (mode === "approval") {
          const r = needAdmin("trashing a file");
          if (r) return r;
        }
      }
      if (f === "deletion" && !now.deletion && mode === "approval") {
        const r = needAdmin("lifting a deletion request");
        if (r) return r;
      }
    }
  }
  for (const id of created) {
    if (after.nodes?.[id]?.trashed) return "a file is not created in the trash";
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

/** The shape checks of everything the change touched. */
function checkSections(
  after: IndexDoc,
  created: ReadonlySet<string>,
  touched: ReadonlyMap<string, ReadonlySet<string>>,
): Refusal {
  for (const id of created) {
    const p = nodeProblem(id, after.nodes?.[id]);
    if (p) return p;
  }
  for (const [id, fields] of touched) {
    const n = after.nodes?.[id];
    if (!n) continue;
    if (created.has(id)) continue; // checked whole above
    const p = nodeProblem(id, n);
    if (p && fields.size > 0) return p;
  }
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
  for (const [k, v] of Object.entries(after.packages ?? {}))
    if (!packageOk(k, v)) return "bad package";
  if (after.main !== undefined && !isId(after.main)) return "bad main";
  return undefined;
}

/**
 * The state of the checks for one index document: a shadow copy of what has been admitted, so
 * that a change is judged without waiting for the Repo, which applies messages later. Local
 * changes reach it through `source`, the Repo's own copy.
 *
 * A change is judged against the index as its own dependencies left it ({@link A.view}), not
 * against the shadow as it is now: what the receiver happens to hold already must not change
 * the verdict, or two replicas could refuse different changes and never agree again.
 */
export class IndexValidator {
  #shadow: A.Doc<IndexDoc> | undefined;
  #source: () => A.Doc<IndexDoc> | undefined;

  constructor(source: () => A.Doc<IndexDoc> | undefined) {
    this.#source = source;
  }

  /** The heads of what has been admitted so far, sorted. */
  heads(): string[] {
    return [...A.getHeads(this.#sync())].sort();
  }

  /** Brings the shadow up to the Repo's own copy (local edits, and what the Repo applied). */
  #sync(): A.Doc<IndexDoc> {
    const live = this.#source();
    if (!this.#shadow) {
      this.#shadow = live ? A.clone(live) : A.init<IndexDoc>();
    } else if (live && !A.hasHeads(this.#shadow, A.getHeads(live))) {
      this.#shadow = A.merge(this.#shadow, A.clone(live));
    }
    return this.#shadow;
  }

  /**
   * Judges one change and, if it is admitted, takes it into the shadow. `adminCovered` is asked
   * only when a rule needs it. A change whose dependencies are unknown cannot be judged: it is
   * not admitted, and the sync protocol offers it again once they are here.
   */
  check(bytes: Uint8Array, adminCovered: (hash: string) => boolean): Refusal {
    const judged = this.#judge(this.#sync(), bytes, adminCovered);
    if (judged.refusal === undefined) this.#shadow = judged.next;
    return judged.refusal;
  }

  /**
   * Judges a document that arrives whole: its shape, then each of its changes in causal order,
   * as if they had come one by one. All or nothing: the shadow takes the document only if every
   * change is admitted.
   */
  checkDocument(doc: A.Doc<IndexDoc>, adminCovered: (hash: string) => boolean): Refusal {
    const shape = checkDocument(plain(doc));
    if (shape !== undefined) return shape;
    let work = A.clone(this.#sync());
    for (const bytes of A.getAllChanges(doc)) {
      const judged = this.#judge(work, bytes, adminCovered);
      if (judged.refusal !== undefined) return judged.refusal;
      work = judged.next;
    }
    this.#shadow = work;
    return undefined;
  }

  /** The verdict on one change over `shadow`, and the shadow with the change if admitted. */
  #judge(
    shadow: A.Doc<IndexDoc>,
    bytes: Uint8Array,
    adminCovered: (hash: string) => boolean,
  ): { refusal: Refusal; next: A.Doc<IndexDoc> } {
    let decoded: ReturnType<typeof A.decodeChange>;
    try {
      decoded = A.decodeChange(bytes);
    } catch {
      return { refusal: "undecodable change", next: shadow };
    }
    if (!A.hasHeads(shadow, decoded.deps)) {
      return { refusal: "dependencies unknown", next: shadow };
    }
    if (A.hasHeads(shadow, [decoded.hash])) return { refusal: undefined, next: shadow };
    // A burst of typing: every operation is in the text of the main file.
    const textObj = A.getObjectId(shadow, "text");
    if (textObj && decoded.ops.length > 0 && decoded.ops.every((op) => op.obj === textObj)) {
      return { refusal: undefined, next: A.applyChanges(shadow, [bytes])[0] };
    }
    const heads = A.getHeads(shadow);
    const linear =
      heads.length === decoded.deps.length && decoded.deps.every((d) => heads.includes(d));
    const before = linear ? shadow : A.view(shadow, decoded.deps);
    let patches: Patch[] = [];
    const [after] = A.applyChanges(A.clone(before), [bytes], {
      patchCallback: (p) => {
        patches = p;
      },
    });
    const refusal = checkChange(patches, plain(before), plain(after), adminCovered(decoded.hash));
    if (refusal !== undefined) return { refusal, next: shadow };
    return { refusal: undefined, next: linear ? after : A.applyChanges(shadow, [bytes])[0] };
  }
}
