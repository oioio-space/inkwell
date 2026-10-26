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

  /** `maxChars`: the most characters peers may add to a text (default: what a document grows to). */
  constructor(maxChars: number = MAX_GROWN_CHARS) {
    this.#maxChars = maxChars;
  }
  #lastRewritten = new Map<string, Uint8Array>(); // peer + document → last message we had to cut

  /** `signer`, a current writer, attested `heads` of document `doc`. */
  attested(signer: string, doc: string, heads: string[]): void {
    this.#doc(doc).attest(signer, heads);
  }

  /** `signer` is not a writer anymore: their attestations are void. */
  revoked(signer: string): void {
    for (const d of this.#docs.values()) d.revoke(signer);
  }

  /** Forgets what is kept about `peer` (it disconnected: its sync state starts over). */
  forgetPeer(peer: string): void {
    for (const key of this.#lastRewritten.keys()) {
      if (key.startsWith(`${peer}\0`)) this.#lastRewritten.delete(key);
    }
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
    const docs = this.#doc(doc);
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
    const seen = this.#counted.get(doc) ?? new Set<string>();
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
    for (const h of fresh) seen.add(h);
    this.#counted.set(doc, seen);
    // A document taken in is the text itself: what was counted before is part of it, not on top.
    this.#grown.set(doc, Math.max(total, docChars));
    const heads = docs.withoutRejected(msg.heads);
    if (covered.every(Boolean) && heads === msg.heads) return data;
    const kept = chunks.filter((_, i) => covered[i]).map((c) => c.bytes);
    const out = A.encodeSyncMessage({ ...msg, heads, changes: kept });
    const key = `${from}\0${doc}`;
    const last = this.#lastRewritten.get(key);
    if (last && last.length === out.length && last.every((b, i) => b === out[i])) {
      this.stats.repeated++;
      return undefined;
    }
    this.#lastRewritten.set(key, out);
    return out;
  }

  #doc(doc: string): DocCoverage {
    let d = this.#docs.get(doc);
    if (!d) {
      d = new DocCoverage();
      this.#docs.set(doc, d);
    }
    return d;
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
