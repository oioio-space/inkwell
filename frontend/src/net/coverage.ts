import { next as A } from "@automerge/automerge";
import { MAX_GROWN_CHARS } from "../store/limits";

/** One change, reduced to what the coverage check needs. */
interface ChangeInfo {
  hash: string;
  deps: string[];
}

/** Heads kept per (document, writer): the most recent ones; older ones are ancestors of them. */
const MAX_KEPT_HEADS = 256;
/** Changes remembered as refused, per document, to keep them out of the heads we relay. */
const MAX_REJECTED = 1024;
/** Offset of the chunk type in an Automerge chunk header: magic (4) + checksum (4). */
const CHUNK_TYPE = 8;
/** Chunk type of a whole document; 1 and 2 are a change and a compressed change. */
const CHUNK_DOCUMENT = 0;
/** Documents tracked at most (those a verified attestation names), and by one signer. */
const MAX_DOCS = 10_000;
const MAX_DOCS_PER_SIGNER = 1024;
/** Rewritten-message digests kept: per peer, and over all peers. */
const MAX_REWRITTEN_PER_PEER = 256;
const MAX_REWRITTEN = 4096;

/** Limits of a {@link Coverage}; the defaults are the constants above. */
export interface CoverageLimits {
  maxDocs?: number;
  maxDocsPerSigner?: number;
}

/** A 64-bit-ish fingerprint (two FNV-1a passes and the length): enough to spot a repeat. */
function digest(bytes: Uint8Array): string {
  let a = 0x811c9dc5;
  let b = 0x01000193;
  for (const x of bytes) {
    a = Math.imul(a ^ x, 0x01000193);
    b = Math.imul(b ^ x, 0x85ebca6b);
  }
  return `${bytes.length}:${a >>> 0}:${b >>> 0}`;
}

/**
 * What the heads attested for one document cover. A change is covered when it is
 * an attested head or an ancestor of one, found through the `deps` of the changes.
 * Hashes commit to their dependencies, so edges learned from any message are
 * authentic; only the attested heads carry trust.
 *
 * Only covered changes are remembered, so what a stranger sends cannot grow the
 * graph. A covered change whose dependencies are not known yet stays "open": the
 * dependencies are covered as soon as the change shows up.
 */
class DocCoverage {
  #deps = new Map<string, string[]>();
  #covered = new Set<string>();
  #open = new Set<string>(); // covered, dependencies still unknown
  #heads = new Map<string, string[]>(); // writer → attested heads, oldest first
  #rejected = new Map<string, string[]>(); // refused change → its dependencies, oldest first

  attest(signer: string, heads: string[]): void {
    const kept = this.#heads.get(signer) ?? [];
    for (const h of heads) {
      if (kept.includes(h)) continue;
      kept.push(h);
      this.#cover(h);
    }
    if (kept.length > MAX_KEPT_HEADS) kept.splice(0, kept.length - MAX_KEPT_HEADS);
    this.#heads.set(signer, kept);
  }

  /** Whether a writer still has attested heads for this document. */
  get attested(): boolean {
    return this.#heads.size > 0;
  }

  /** Forgets what `signer` attested; what only they covered is not covered anymore. */
  revoke(signer: string): void {
    if (!this.#heads.delete(signer)) return;
    this.#covered.clear();
    this.#open.clear();
    for (const heads of this.#heads.values()) for (const h of heads) this.#cover(h);
  }

  /**
   * Learns the changes of one message and returns the hashes of those that are
   * covered. A change covered by a head of the same message counts, whatever the
   * order of the changes.
   */
  admit(changes: ChangeInfo[]): Set<string> {
    const added: string[] = [];
    for (const c of changes) {
      if (this.#deps.has(c.hash)) continue;
      this.#deps.set(c.hash, c.deps);
      added.push(c.hash);
    }
    for (const c of changes) {
      if (!this.#open.delete(c.hash)) continue;
      for (const d of c.deps) this.#cover(d);
    }
    for (const h of added) {
      if (!this.#covered.has(h)) this.#deps.delete(h); // never remember what is not covered
    }
    return new Set(changes.filter((c) => this.#covered.has(c.hash)).map((c) => c.hash));
  }

  /** Remembers refused changes (bounded: the oldest are forgotten). */
  reject(changes: ChangeInfo[]): number {
    let fresh = 0;
    for (const c of changes) {
      if (!this.#rejected.delete(c.hash)) fresh++; // delete + set moves it to the end
      this.#rejected.set(c.hash, c.deps);
    }
    for (const h of this.#rejected.keys()) {
      if (this.#rejected.size <= MAX_REJECTED) break;
      this.#rejected.delete(h);
    }
    return fresh;
  }

  /**
   * Removes the refused changes from `heads`: a peer whose heads are changes we
   * refused must not make us ask for them, or answer it forever.
   */
  withoutRejected(heads: string[]): string[] {
    if (this.#rejected.size === 0 || !heads.some((h) => this.#rejected.has(h))) return heads;
    return heads.filter((h) => !this.#rejected.has(h));
  }

  #cover(root: string): void {
    const todo = [root];
    for (let h = todo.pop(); h !== undefined; h = todo.pop()) {
      if (this.#covered.has(h)) continue;
      this.#covered.add(h);
      this.#rejected.delete(h); // a writer took it in after all
      const deps = this.#deps.get(h);
      if (deps) todo.push(...deps);
      else this.#open.add(h);
    }
  }
}

/** Counters of what {@link Coverage} did, for diagnostics and tests. */
export interface CoverageStats {
  /** Changes refused because no current writer covers them (each counted once). */
  removed: number;
  /** Messages dropped whole: undecodable, or a full-document fragment not fully covered. */
  dropped: number;
  /** Full-document fragments that were checked. */
  fragments: number;
  /** Rewritten messages not delivered because the peer had just sent the same. */
  repeated: number;
}

/**
 * Write enforcement of the webview (ADR 0018). Go signs and verifies the
 * attestations of the writers and reports them; this class, which alone can read
 * Automerge, keeps only the changes that an attestation of a current writer covers,
 * before the Repo sees them.
 *
 * Use: call {@link attested} for each `Attested` envelope and {@link revoked} for
 * each `Revoked` one, and pass every received sync message through {@link filter}.
 * Ordering is the transport's job: an attestation reaches us before the changes it
 * covers, since Go forwards it ahead of the repo frames that follow it.
 */
export class Coverage {
  readonly stats: CoverageStats = {
    removed: 0,
    dropped: 0,
    fragments: 0,
    repeated: 0,
  };
  #docs = new Map<string, DocCoverage>();
  /** Characters the peers' covered changes added to each document's text since this start. */
  #grown = new Map<string, number>();
  /** The changes already counted in `#grown`, by document. */
  #counted = new Map<string, Set<string>>();
  readonly #maxChars: number;
  readonly #maxDocs: number;
  readonly #maxDocsPerSigner: number;
  /** Per peer, then per document: the digest of the last message we had to cut (insertion order). */
  #lastRewritten = new Map<string, Map<string, string>>();
  #rewrittenCount = 0;
  /** The documents each signer attested, to bound what one signer can make us track. */
  #bySigner = new Map<string, Set<string>>();

  /** `maxChars`: the most characters peers may add to a text (default: what a document grows to). */
  constructor(maxChars: number = MAX_GROWN_CHARS, limits: CoverageLimits = {}) {
    this.#maxChars = maxChars;
    this.#maxDocs = limits.maxDocs ?? MAX_DOCS;
    this.#maxDocsPerSigner = limits.maxDocsPerSigner ?? MAX_DOCS_PER_SIGNER;
  }

  /** How much is kept, for tests and diagnostics. */
  get size(): { docs: number; counted: number; grown: number; rewritten: number } {
    return {
      docs: this.#docs.size,
      counted: this.#counted.size,
      grown: this.#grown.size,
      rewritten: this.#rewrittenCount,
    };
  }

  /**
   * `signer`, a current writer (Go verified the signature and the role), attested `heads` of
   * document `doc`. Only here does a document get state; past the limits the attestation is not
   * tracked (the changes it would cover are then refused, which is the safe side).
   */
  attested(signer: string, doc: string, heads: string[]): void {
    const mine = this.#bySigner.get(signer) ?? new Set<string>();
    if (!mine.has(doc)) {
      if (mine.size >= this.#maxDocsPerSigner) return;
      if (!this.#docs.has(doc) && this.#docs.size >= this.#maxDocs) return;
    }
    mine.add(doc);
    this.#bySigner.set(signer, mine);
    let d = this.#docs.get(doc);
    if (!d) {
      d = new DocCoverage();
      this.#docs.set(doc, d);
    }
    d.attest(signer, heads);
  }

  /** `signer` is not a writer anymore: their attestations are void. */
  revoked(signer: string): void {
    this.#bySigner.delete(signer);
    for (const [name, d] of this.#docs) {
      d.revoke(signer);
      if (!d.attested) {
        this.#docs.delete(name);
        this.#grown.delete(name);
        this.#counted.delete(name);
      }
    }
  }

  /** Forgets what is kept about `peer` (it disconnected: its sync state starts over). */
  forgetPeer(peer: string): void {
    this.#rewrittenCount -= this.#lastRewritten.get(peer)?.size ?? 0;
    this.#lastRewritten.delete(peer);
  }

  /** Remembers that `out` was the rewrite for (`peer`, `doc`); true if it was already the last one. */
  #repeated(peer: string, doc: string, out: Uint8Array): boolean {
    const fp = digest(out);
    let mine = this.#lastRewritten.get(peer);
    if (!mine) {
      mine = new Map();
      this.#lastRewritten.set(peer, mine);
    }
    if (mine.get(doc) === fp) return true;
    if (!mine.delete(doc)) this.#rewrittenCount++;
    mine.set(doc, fp);
    for (const old of mine.keys()) {
      if (mine.size <= MAX_REWRITTEN_PER_PEER) break;
      mine.delete(old);
      this.#rewrittenCount--;
    }
    for (const [other, m] of this.#lastRewritten) {
      if (this.#rewrittenCount <= MAX_REWRITTEN) break;
      if (other === peer) continue;
      this.#rewrittenCount -= m.size;
      this.#lastRewritten.delete(other);
    }
    return false;
  }

  /**
   * Returns the sync message `data`, which `from` sent about document `doc`, without
   * the changes that are not covered, or `undefined` when the message must not be
   * delivered: it does not decode, it holds a full-document fragment with an
   * uncovered change (a fragment cannot be split), or it is a message we already had
   * to cut, sent again. A message without uncovered changes is returned as is.
   *
   * The last rule ends a loop a forger could start: its heads name a change we
   * refuse, so each reply of ours would be answered by the same message, for ever.
   * Automerge never sends the same message twice in a row, so honest peers are not
   * affected.
   */
  filter(doc: string, from: string, data: Uint8Array): Uint8Array | undefined {
    let msg: ReturnType<typeof A.decodeSyncMessage>;
    try {
      msg = A.decodeSyncMessage(data);
    } catch {
      this.stats.dropped++;
      return undefined;
    }
    // A document nobody attested has no state of ours: its changes are all uncovered, and what a
    // stranger sends must not make us keep anything per document id.
    const tracked = this.#docs.get(doc);
    const docs = tracked ?? new DocCoverage();
    let chunks: Chunk[];
    try {
      chunks = msg.changes.flatMap(splitChunks).map(describeChunk);
    } catch {
      this.stats.dropped++;
      return undefined;
    }
    this.stats.fragments += chunks.filter((c) => c.isDocument).length;

    const accepted = docs.admit(chunks.flatMap((c) => c.changes));
    const covered = chunks.map((c) => c.changes.every((x) => accepted.has(x.hash)));
    this.stats.removed += docs.reject(
      chunks
        .filter((_, i) => !covered[i])
        .flatMap((c) => c.changes.filter((x) => !accepted.has(x.hash))),
    );
    if (chunks.some((c, i) => c.isDocument && !covered[i])) {
      this.stats.dropped++;
      return undefined;
    }
    // A peer cannot make us hold a text past the limit (ADR 0028): what the covered chunks add
    // to the text, counted over the session, stays under it; a deletion gives its room back.
    // A whole document is judged by its own length; a change counts once, whatever number of
    // times a peer sends it again.
    const seen = tracked ? (this.#counted.get(doc) ?? new Set<string>()) : new Set<string>();
    let grew = 0;
    let docChars = 0;
    const fresh: string[] = [];
    for (const [i, c] of chunks.entries()) {
      if (!covered[i]) continue;
      if (c.isDocument) {
        docChars = Math.max(docChars, c.chars);
        if (c.chars > this.#maxChars) grew = Number.POSITIVE_INFINITY;
        continue;
      }
      const hash = c.changes[0]?.hash ?? "";
      if (seen.has(hash)) continue;
      fresh.push(hash);
      grew += c.chars;
    }
    const total = Math.max(0, (this.#grown.get(doc) ?? 0) + grew);
    if (grew > 0 && total > this.#maxChars) {
      this.stats.dropped++;
      return undefined;
    }
    if (tracked) {
      for (const h of fresh) seen.add(h);
      this.#counted.set(doc, seen);
      // A document taken in is the text itself: what was counted before is part of it, not on top.
      this.#grown.set(doc, Math.max(total, docChars));
    }
    const heads = docs.withoutRejected(msg.heads);
    if (covered.every(Boolean) && heads === msg.heads) return data;
    const kept = chunks.filter((_, i) => covered[i]).map((c) => c.bytes);
    const out = A.encodeSyncMessage({ ...msg, heads, changes: kept });
    if (this.#repeated(from, doc, out)) {
      this.stats.repeated++;
      return undefined;
    }
    return out;
  }
}

/** One chunk of a sync message and the changes it holds. */
interface Chunk {
  bytes: Uint8Array;
  /** A whole-document chunk (several changes in one), which cannot be cut. */
  isDocument: boolean;
  changes: ChangeInfo[];
  /**
   * How many characters it adds to a text: inserts minus deletes of a change, or the length of
   * the text of a whole document. Read from what was decoded for the check anyway.
   */
  chars: number;
}

/** The first four bytes of every Automerge chunk. */
const MAGIC = [0x85, 0x6f, 0x4a, 0x83];

/**
 * Splits `item`, an entry of a sync message's `changes`, into the chunks it
 * concatenates: Automerge 3 packs several changes, or a document, into one entry.
 * A chunk is magic (4) | checksum (4) | type (1) | length (LEB128) | data. Throws if
 * the entry is not a sequence of whole chunks.
 */
function splitChunks(item: Uint8Array): Uint8Array[] {
  const out: Uint8Array[] = [];
  let at = 0;
  while (at < item.length) {
    if (!MAGIC.every((m, i) => item[at + i] === m)) throw new RangeError("not a chunk");
    let pos = at + CHUNK_TYPE + 1;
    let length = 0;
    for (let scale = 1; ; scale *= 128) {
      const byte = item[pos++];
      if (byte === undefined || scale > 128 ** 4) throw new RangeError("bad chunk length");
      length += (byte & 0x7f) * scale;
      if (byte < 0x80) break;
    }
    if (pos + length > item.length) throw new RangeError("truncated chunk");
    out.push(item.subarray(at, pos + length));
    at = pos + length;
  }
  return out;
}

function describeChunk(bytes: Uint8Array): Chunk {
  if (bytes[CHUNK_TYPE] === CHUNK_DOCUMENT) {
    const doc = A.load<unknown>(bytes);
    const changes = A.getChangesMetaSince(doc, []).map(({ hash, deps }) => ({ hash, deps }));
    const text = (doc as { text?: unknown }).text;
    return { bytes, isDocument: true, changes, chars: typeof text === "string" ? text.length : 0 };
  }
  const { hash, deps, ops } = A.decodeChange(bytes);
  return { bytes, isDocument: false, changes: [{ hash, deps }], chars: opsChars(ops) };
}

/**
 * How many UTF-16 units (what the editor counts) a decoded change adds to a text: its inserts at
 * the length of the value they carry (Automerge keeps one op per character, an emoji being one
 * op of two units), minus its deletes at one each (an op carries no value; a deleted emoji frees
 * two units but counts one, so the tally errs on the side of caution).
 */
function opsChars(ops: readonly { action: string; insert?: boolean; value?: unknown }[]): number {
  let chars = 0;
  for (const op of ops) {
    if (op.action === "del") chars--;
    else if (op.insert) chars += typeof op.value === "string" ? op.value.length : 1;
  }
  return chars;
}

/** The characters (UTF-16 units) one encoded change adds to a text; negative if it removes more. */
export function changeChars(change: Uint8Array): number {
  return opsChars(A.decodeChange(change).ops);
}
