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

type Op = ReturnType<typeof A.decodeChange>["ops"][number];
type Decoded = ReturnType<typeof A.decodeChange>;

/** What an object of the index document is, so that an operation on it can be read. */
type Kind =
  | { t: "root" }
  | { t: "text" }
  | { t: "nodes" }
  | { t: "node"; id: string }
  | { t: "stamp"; id: string; field: "trashed" | "deletion" }
  | { t: "settings" }
  | { t: "decisions" }
  | { t: "decision"; id: string }
  | { t: "packages" }
  | { t: "package"; key: string };

const ROOT: Kind = { t: "root" };

/** What a new object made under `parent` at `key` is (undefined: nothing the index has). */
function classify(parent: Kind | undefined, key: unknown): Kind | undefined {
  if (parent === undefined || typeof key !== "string") return undefined;
  switch (parent.t) {
    case "root":
      return key === "text"
        ? { t: "text" }
        : key === "nodes"
          ? { t: "nodes" }
          : key === "settings"
            ? { t: "settings" }
            : key === "decisions"
              ? { t: "decisions" }
              : key === "packages"
                ? { t: "packages" }
                : undefined;
    case "nodes":
      return isId(key) ? { t: "node", id: key } : undefined;
    case "node":
      return key === "trashed" || key === "deletion"
        ? { t: "stamp", id: parent.id, field: key }
        : undefined;
    case "decisions":
      return isId(key) ? { t: "decision", id: key } : undefined;
    case "packages":
      return isPackageKey(key) ? { t: "package", key } : undefined;
    default:
      return undefined;
  }
}

/** The value an operation writes, as plain data (a null is a set whose value is missing). */
const valueWritten = (op: Op): unknown => (op.value === undefined ? null : op.value);

/** Fields of the pieces a node and the sections are made of. */
const STAMP_FIELDS: ReadonlySet<string> = new Set(["by", "at", "req"]);
const DECISION_FIELDS: ReadonlySet<string> = new Set(["by", "at", "req", "verdict"]);
const PACKAGE_FIELDS: ReadonlySet<string> = new Set(["hash", "size", "by", "at"]);

/** The reason, once judged, of a change that must wait: for an admin, for what it builds on. */
const waitsForAdmin = (what: string): string => `${HOLD}${what} needs an admin`;

/** Judgements that read the index at a change's own heads (a pass over the history). */
const BUDGET = 8;
/** Of them, per second: a hostile message cannot take the UI thread for more than this. */
const BUDGET_PER_SECOND = 8;
/** Judgements of that kind a whole document may take in one go (it arrives once, and whole). */
const DOCUMENT_BUDGET = 512;
/** Refused changes remembered, so that what builds on one is refused too. */
const MAX_REFUSED = 20_000;

/** Options of {@link IndexValidator}. */
export interface ValidatorOptions {
  /** The clock for the budget, in milliseconds (the browser's by default). */
  now?: () => number;
}

/**
 * The checks for one index document. A change is judged from its own operations and, for the few
 * rules that need it, from the index as its own dependencies left it ({@link A.view}), never from
 * the state the receiver happens to have reached: two replicas that get the same changes, honest
 * or crafted, in any order give the same verdict on each. Nothing but the verdict decides:
 * what is read is the change's operations (kind of object, key, value, what each replaces), and
 * a change is applied to the working copy only once it is admitted, so refused or waiting changes
 * cannot slow down or bend a later verdict.
 *
 * What waits (for an admin, for a change it builds on, for the budget) is told apart from what
 * is refused for good by a reason that starts with {@link HOLD}; whoever holds the change judges
 * it again when {@link resolved}, {@link hasBudget} or the admin's attestation says it may pass.
 */
export class IndexValidator {
  #work: A.Doc<IndexDoc> | undefined;
  #source: () => A.Doc<IndexDoc> | undefined;
  /** What each object of the working copy is. */
  #objs = new Map<string, Kind>([["_root", ROOT]]);
  /** Every node id the admitted changes name (purged ones too): an upper bound of the nodes. */
  #ids = new Set<string>();
  /** Changes refused for good: hash → why. */
  #refused = new Map<string, string>();
  /** The hash of every change in the working copy (asking the document costs a pass over it). */
  #hashes = new Set<string>();
  #now: () => number;
  #tokens = BUDGET;
  #stamp: number;
  #docTokens = 0;

  constructor(source: () => A.Doc<IndexDoc> | undefined, opts: ValidatorOptions = {}) {
    this.#source = source;
    this.#now = opts.now ?? (() => performance.now());
    this.#stamp = this.#now();
  }

  /** The heads of the admitted changes, sorted. */
  heads(): string[] {
    return [...A.getHeads(this.#sync())].sort();
  }

  /** True once the change is in the working copy, or was refused: nothing waits on it any more. */
  resolved(hash: string): boolean {
    this.#sync();
    return this.#refused.has(hash) || this.#hashes.has(hash);
  }

  /** True if a judgement that reads the history can be afforded now. */
  hasBudget(): boolean {
    this.#refill();
    return this.#tokens >= 1;
  }

  #refill(): void {
    const t = this.#now();
    this.#tokens = Math.min(BUDGET, this.#tokens + ((t - this.#stamp) * BUDGET_PER_SECOND) / 1000);
    this.#stamp = t;
  }

  /** Pays for a judgement that reads the history; false if the budget is spent. */
  #spend(): boolean {
    if (this.#docTokens > 0) {
      this.#docTokens--;
      return true;
    }
    this.#refill();
    if (this.#tokens < 1) return false;
    this.#tokens--;
    return true;
  }

  /** Brings the working copy up to the Repo's own copy (local edits, and what the Repo applied). */
  #sync(): A.Doc<IndexDoc> {
    const live = this.#source();
    if (!this.#work) {
      this.#work = live ? A.clone(live) : A.init<IndexDoc>();
      this.#hashes = new Set(A.getChangesMetaSince(this.#work, []).map((c) => c.hash));
      this.#scan();
    } else if (live && !A.hasHeads(this.#work, A.getHeads(live))) {
      const before = A.getHeads(this.#work);
      this.#work = A.merge(this.#work, A.clone(live));
      for (const c of A.getChangesMetaSince(this.#work, before)) this.#hashes.add(c.hash);
      this.#scan();
    }
    return this.#work;
  }

  /** Reads what every object of the working copy is, from the document itself. */
  #scan(): void {
    const doc = this.#work as A.Doc<IndexDoc>;
    const root = doc as unknown as Record<string, unknown>;
    this.#objs = new Map([["_root", ROOT]]);
    const put = (value: unknown, kind: Kind) => {
      if (value === null || typeof value !== "object") return;
      const id = A.getObjectId(value as A.Doc<unknown>);
      if (id !== null) this.#objs.set(id, kind);
    };
    const text = A.getObjectId(doc, "text");
    if (text !== null) this.#objs.set(text, { t: "text" });
    const nodes = root.nodes as Record<string, Record<string, unknown>> | undefined;
    if (nodes) {
      put(nodes, { t: "nodes" });
      for (const id of Object.keys(nodes)) {
        this.#ids.add(id);
        const node = nodes[id];
        put(node, { t: "node", id });
        for (const field of ["trashed", "deletion"] as const)
          put(node?.[field], { t: "stamp", id, field });
      }
    }
    put(root.settings, { t: "settings" });
    const decisions = root.decisions as Record<string, unknown> | undefined;
    if (decisions) {
      put(decisions, { t: "decisions" });
      for (const id of Object.keys(decisions)) put(decisions[id], { t: "decision", id });
    }
    const packages = root.packages as Record<string, unknown> | undefined;
    if (packages) {
      put(packages, { t: "packages" });
      for (const key of Object.keys(packages)) put(packages[key], { t: "package", key });
    }
  }

  /** Takes an admitted change into the working copy, and what it made into the objects known. */
  #take(bytes: Uint8Array, decoded: Decoded): void {
    this.#work = A.applyChanges(this.#sync(), [bytes])[0];
    this.#hashes.add(decoded.hash);
    decoded.ops.forEach((op, i) => {
      if (!op.action.startsWith("make")) return;
      const parent = op.obj === "_root" ? ROOT : this.#objs.get(op.obj);
      const kind = classify(parent, op.key);
      if (kind) this.#objs.set(`${decoded.startOp + i}@${decoded.actor}`, kind);
      if (parent?.t === "nodes" && typeof op.key === "string") this.#ids.add(op.key);
    });
  }

  #refuse(hash: string, why: string): string {
    this.#refused.set(hash, why);
    for (const h of this.#refused.keys()) {
      if (this.#refused.size <= MAX_REFUSED) break;
      this.#refused.delete(h);
    }
    return why;
  }

  /**
   * Judges one change. `adminCovered` is asked only when a rule needs it. The answer is
   * undefined (admitted), a reason that starts with {@link HOLD} (it waits: an admin has not
   * vouched yet, what it builds on is not here, the budget is spent), or a refusal for good.
   */
  check(bytes: Uint8Array, adminCovered: (hash: string) => boolean): Refusal {
    let decoded: Decoded;
    try {
      decoded = A.decodeChange(bytes);
    } catch {
      return "undecodable change";
    }
    const { hash, deps } = decoded;
    const refused = this.#refused.get(hash);
    if (refused !== undefined) return refused;
    const work = this.#sync();
    if (this.#hashes.has(hash)) return undefined; // admitted before
    const missing = deps.filter((d) => !this.#hashes.has(d));
    for (const d of deps) {
      const why = this.#refused.get(d);
      if (why !== undefined) return this.#refuse(hash, `builds on a refused change (${why})`);
    }
    if (missing.length > 0) return `${HOLD}dependencies unknown`;
    const heads = A.getHeads(work);
    const linear = heads.length === deps.length && deps.every((d) => heads.includes(d));
    const verdict = this.#judge(decoded, work, linear, adminCovered(hash));
    if (verdict === undefined) this.#take(bytes, decoded);
    else if (!verdict.startsWith(HOLD)) this.#refuse(hash, verdict);
    return verdict;
  }

  /**
   * Judges a document that arrives whole: its shape, then each of its changes in causal order,
   * as if they had come one by one. It is admitted only if every change is; a change that waits
   * makes the whole document wait (what was admitted on the way stays admitted).
   */
  checkDocument(doc: A.Doc<IndexDoc>, adminCovered: (hash: string) => boolean): Refusal {
    const shape = checkDocument(plain(doc));
    if (shape !== undefined) return shape;
    this.#docTokens = DOCUMENT_BUDGET;
    try {
      // In causal order, and again for what waited for a change that came later in the list.
      let pending = A.getAllChanges(doc);
      let waiting: Refusal;
      for (let progress = true; progress && pending.length > 0; ) {
        progress = false;
        const rest: Uint8Array[] = [];
        waiting = undefined;
        for (const bytes of pending) {
          const verdict = this.check(bytes, adminCovered);
          if (verdict === undefined) progress = true;
          else if (!verdict.startsWith(HOLD)) return verdict;
          else {
            rest.push(bytes);
            waiting ??= verdict;
          }
        }
        pending = rest;
      }
      return pending.length > 0 ? waiting : undefined;
    } finally {
      this.#docTokens = 0;
    }
  }

  /** The verdict on a change whose dependencies are all in the working copy. */
  #judge(decoded: Decoded, work: A.Doc<IndexDoc>, linear: boolean, admin: boolean): Refusal {
    const ops = decoded.ops;
    const made = new Map<string, Kind>();
    const kindOf = (obj: string): Kind | undefined =>
      obj === "_root" ? ROOT : (made.get(obj) ?? this.#objs.get(obj));
    // Typing in the main file: every operation is in its text.
    if (ops.length > 0 && ops.every((op) => kindOf(op.obj)?.t === "text")) return undefined;

    const needAdmin = (what: string): Refusal => (admin ? undefined : waitsForAdmin(what));
    const created = new Map<string, Record<string, unknown>>();
    const stamps = new Map<string, Record<string, unknown>>();
    const decisions = new Map<string, Record<string, unknown>>();
    const packages = new Map<string, Record<string, unknown>>();
    /** Fields written into nodes that already existed, with the value (undefined: deleted). */
    const touched = new Map<string, Map<string, unknown>>();
    const purged = new Set<string>();
    let setsMain: unknown;
    let setsMode: unknown;
    const madeStamps = new Set<string>();

    for (let i = 0; i < ops.length; i++) {
      const op = ops[i] as Op;
      const obj = kindOf(op.obj);
      if (obj === undefined) return "unknown object";
      const isMake = op.action.startsWith("make");
      const key = op.key;
      const existed = op.pred.length > 0;
      const fresh = made.has(op.obj);
      const here = `${decoded.startOp + i}@${decoded.actor}`;
      switch (obj.t) {
        case "text":
          break;
        case "root": {
          if (typeof key !== "string" || !TOP_KEYS.has(key)) return `unknown key ${String(key)}`;
          switch (key) {
            case "text":
              if (op.action !== "makeText") return "bad text";
              break;
            case "schema":
              if (existed) return "schema is fixed";
              if (op.action !== "set" || valueWritten(op) !== 1) return "unknown schema";
              break;
            case "main":
              if (existed) {
                const gate = needAdmin("changing the main file");
                if (gate) return gate;
              }
              if (op.action === "set") {
                if (!isId(valueWritten(op))) return "bad main";
                setsMain = valueWritten(op);
              } else if (op.action !== "del") return "bad main";
              break;
            case "nodes":
              if (op.action !== "makeMap" || existed) return "nodes replaced";
              break;
            case "settings":
              if (existed) {
                const gate = needAdmin("changing the deletion rule");
                if (gate) return gate;
              }
              if (op.action !== "makeMap") return "bad settings";
              break;
            case "decisions":
              if (existed) {
                const gate = needAdmin("deciding on a request");
                if (gate) return gate;
              }
              if (op.action !== "makeMap") return "bad decision";
              break;
            case "packages":
              if (op.action === "del") {
                const gate = needAdmin("removing a package");
                if (gate) return gate;
              }
              if (op.action !== "makeMap" && op.action !== "del") return "bad package";
              break;
          }
          break;
        }
        case "nodes": {
          if (typeof key !== "string" || !isId(key)) return "bad node id";
          if (isMake) {
            if (op.action !== "makeMap") return "bad node patch";
            if (existed) return "node replaced";
            created.set(key, {});
          } else if (op.action === "del") {
            const r = needAdmin("purging a file");
            if (r) return r;
            purged.add(key);
          } else return "bad node patch";
          break;
        }
        case "node": {
          if (typeof key !== "string" || !NODE_FIELDS.has(key)) return `unknown node field ${key}`;
          const isStamp = key === "trashed" || key === "deletion";
          if (isMake && !(isStamp && op.action === "makeMap")) {
            return key === "name" ? "invalid name" : `bad ${key}`;
          }
          if (isMake) madeStamps.add(`${obj.id}/${key}`);
          if (fresh) {
            if (!isMake) (created.get(obj.id) as Record<string, unknown>)[key] = valueWritten(op);
          } else {
            const set = touched.get(obj.id) ?? new Map<string, unknown>();
            touched.set(obj.id, set);
            set.set(
              key,
              op.action === "del" ? undefined : isMake ? { stamp: true } : valueWritten(op),
            );
          }
          break;
        }
        case "stamp": {
          if (typeof key !== "string" || !STAMP_FIELDS.has(key) || isMake) return "bad stamp";
          const slot = `${obj.id}/${obj.field}`;
          const buf = stamps.get(slot) ?? {};
          stamps.set(slot, buf);
          if (op.action === "set") buf[key] = valueWritten(op);
          if (!created.has(obj.id)) {
            const set = touched.get(obj.id) ?? new Map<string, unknown>();
            touched.set(obj.id, set);
            if (!set.has(obj.field)) set.set(obj.field, { stamp: true });
          }
          break;
        }
        case "settings": {
          if (!fresh) {
            const gate = needAdmin("changing the deletion rule");
            if (gate) return gate;
          }
          const v = valueWritten(op);
          if (key !== "deletion" || op.action !== "set" || (v !== "approval" && v !== "free")) {
            return "bad settings";
          }
          setsMode = v;
          break;
        }
        case "decisions": {
          const r = needAdmin("deciding on a request");
          if (r) return r;
          if (typeof key !== "string" || !isId(key)) return "bad decision";
          if (op.action === "del") break; // the decision goes with the node it was about (a purge)
          if (!isMake) return "bad decision";
          decisions.set(key, {});
          break;
        }
        case "decision": {
          const r = needAdmin("deciding on a request");
          if (r) return r;
          if (typeof key !== "string" || !DECISION_FIELDS.has(key) || op.action !== "set") {
            return "bad decision";
          }
          const buf = decisions.get(obj.id) ?? {};
          decisions.set(obj.id, buf);
          buf[key] = valueWritten(op);
          break;
        }
        case "packages": {
          if (op.action === "del") {
            const r = needAdmin("removing a package");
            if (r) return r;
          } else if (!isMake || typeof key !== "string" || !isPackageKey(key)) return "bad package";
          else packages.set(key, {});
          break;
        }
        case "package": {
          if (typeof key !== "string" || !PACKAGE_FIELDS.has(key) || op.action !== "set") {
            return "bad package";
          }
          const buf = packages.get(obj.key) ?? {};
          packages.set(obj.key, buf);
          buf[key] = valueWritten(op);
          break;
        }
      }
      if (isMake) {
        const kind = classify(op.obj === "_root" ? ROOT : kindOf(op.obj), key);
        if (kind) made.set(here, kind);
        else if (obj.t !== "text") return "bad value";
      }
    }

    // Shapes: a new thing is read whole, an existing node by the fields written into it.
    for (const [id, fields] of created) {
      for (const f of ["trashed", "deletion"] as const) {
        const buf = stamps.get(`${id}/${f}`);
        if (buf) fields[f] = buf;
      }
      const p = nodeProblem(id, fields);
      if (p) return p;
    }
    for (const [id, v] of decisions) {
      if (!isId(id) || !decisionOk(v)) return "bad decision";
    }
    for (const [k, v] of packages) if (!packageOk(k, v)) return "bad package";
    for (const [id, fields] of touched) {
      if (purged.has(id)) continue;
      for (const [f, v] of fields) {
        if (f === "type" || f === "url" || f === "by" || f === "at" || f === "who") {
          return `${f} of a file never changes`;
        }
        if (f === "name" && (typeof v !== "string" || validateName(v) !== undefined)) {
          return "invalid name";
        }
        if (f === "parent" && v !== null && !isId(v)) return "bad parent";
        if (f === "ftype" && !(typeof v === "string" && /^[a-z0-9-]{1,32}$/.test(v))) {
          return "bad file type";
        }
        if (f === "size" && !(Number.isSafeInteger(v) && (v as number) >= 0)) return "bad size";
        if (f === "hash" && !isHash(v)) return "bad hash";
        if ((f === "trashed" || f === "deletion") && madeStamps.has(`${id}/${f}`)) {
          const buf = stamps.get(`${id}/${f}`) ?? {};
          if (!stampProblemFree(buf, f === "trashed")) {
            return f === "trashed" ? "bad trash mark" : "bad deletion request";
          }
        }
      }
    }
    for (const [slot, buf] of stamps) {
      const id = slot.slice(0, slot.indexOf("/"));
      const f = slot.slice(slot.indexOf("/") + 1) as "trashed" | "deletion";
      if (created.has(id) || madeStamps.has(slot)) continue;
      // fields written into a stamp that already existed: each must be well formed
      if (
        ("by" in buf && !isStamp({ by: buf.by, at: 0 })) ||
        ("at" in buf && !isStamp({ by: "x", at: buf.at })) ||
        ("req" in buf && !isId(buf.req))
      ) {
        return f === "trashed" ? "bad trash mark" : "bad deletion request";
      }
    }

    if (created.size > MAX_CREATE_PER_CHANGE) return "too many files in one change";

    // What follows reads the index as the change's dependencies left it.
    const needsState =
      created.size > 0 ||
      [...touched.values()].some(
        (f) =>
          f.has("parent") ||
          f.has("trashed") ||
          f.has("deletion") ||
          f.has("hash") ||
          f.has("size"),
      );
    if (!needsState) return undefined;
    let at: A.Doc<IndexDoc> | undefined;
    const state = (): A.Doc<IndexDoc> | undefined => {
      if (at) return at;
      if (linear) {
        at = work;
        return at;
      }
      if (!this.#spend()) return undefined;
      at = A.view(work, decoded.deps);
      return at;
    };
    const root = (): Record<string, unknown> | undefined =>
      state() as unknown as Record<string, unknown> | undefined;
    const waiting = `${HOLD}budget`;
    const nodeAt = (id: string): IndexNode | undefined => {
      const r = root();
      const n = (r?.nodes as Record<string, unknown> | undefined)?.[id];
      return n === undefined ? undefined : (plainValue(n) as IndexNode);
    };
    /**
     * True if the folder `parent`, or one above it, is in the trash after the change (undefined:
     * the budget for reading the history is spent).
     */
    const inTrash = (parent: string | null): boolean | undefined => {
      let cur = parent;
      for (let i = 0; cur !== null && i <= MAX_DEPTH + 1; i++) {
        const c = created.get(cur);
        if (c) {
          if (c.trashed) return true;
          cur = (c.parent as string | null | undefined) ?? null;
          continue;
        }
        if (state() === undefined) return undefined;
        const n = nodeAt(cur);
        if (!n) return false;
        const t = touched.get(cur);
        const trashedNow = t?.has("trashed") ? t.get("trashed") !== undefined : Boolean(n.trashed);
        if (trashedNow) return true;
        cur = t?.has("parent") ? ((t.get("parent") as string | null) ?? null) : (n.parent ?? null);
      }
      return false;
    };

    if (created.size > 0 && this.#ids.size + created.size > MAX_NODES) {
      const r = root();
      if (r === undefined) return waiting;
      const count = Object.keys((r.nodes as object | undefined) ?? {}).length;
      if (count + created.size > MAX_NODES) return "too many files";
    }
    for (const [id, n] of created) {
      if (n.trashed) return "a file is not created in the trash";
      const parent = (n.parent as string | null | undefined) ?? null;
      if (parent !== null) {
        const t = inTrash(parent);
        if (t === undefined) return waiting;
        if (t) return "a file is not created in the trash";
      }
      void id;
    }
    for (const [id, fields] of touched) {
      if (purged.has(id)) continue;
      if (fields.has("parent")) {
        const t = inTrash((fields.get("parent") as string | null) ?? null);
        if (t === undefined) return waiting;
        if (t) return "a file is not moved into the trash";
      }
      if (fields.has("hash") || fields.has("size")) {
        if (state() === undefined) return waiting;
        const n = nodeAt(id);
        if (n && n.type !== "blob") return "blob field on a non-blob node";
      }
      if (fields.has("trashed") || fields.has("deletion")) {
        if (state() === undefined) return waiting;
        const r = root() as Record<string, unknown>;
        const mode =
          setsMode ??
          (plainValue(r.settings) as { deletion?: string } | undefined)?.deletion ??
          "approval";
        if (fields.has("trashed") && fields.get("trashed") !== undefined) {
          const main = setsMain ?? plainValue(r.main);
          if (id === main) return "the main file cannot be trashed";
          if (mode === "approval") {
            const need = needAdmin("trashing a file");
            if (need) return need;
          }
        }
        if (fields.has("deletion") && fields.get("deletion") === undefined && mode === "approval") {
          const need = needAdmin("lifting a deletion request");
          if (need) return need;
        }
      }
    }
    return undefined;
  }
}
