import { next as A } from "@automerge/automerge";

/**
 * The start of the reason a validator gives for a change that must wait, not be refused: it needs
 * an admin's attestation, or a change it builds on has not arrived or has not been admitted yet.
 * Such a change is kept aside and judged again when roles or attestations change.
 */
export const HOLD = "hold: ";

/** Held messages kept in all, and per sender: the oldest of the same sender go first. */
const MAX_HELD = 128;
const MAX_HELD_PER_SENDER = 16;

/** What was cut out of a message for now: the chunks that wait, and the message they came in. */
interface Held {
  doc: string;
  from: string;
  msg: ReturnType<typeof A.decodeSyncMessage>;
  chunks: Chunk[];
}

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
  #waiting = new Set<string>(); // held changes: kept aside, not asked for again
  // What admins vouch for (ADR 0022 R1): the same graph, from the heads of admins only. What an
  // admin vouched for stays so when they are demoted, suspended or removed: nothing admitted is
  // ever taken back after the fact.
  #adminCovered = new Set<string>();
  #adminOpen = new Set<string>();

  attest(signer: string, heads: string[], admin = false): void {
    const kept = this.#heads.get(signer) ?? [];
    for (const h of heads) {
      if (admin) this.#cover(h, this.#adminCovered, this.#adminOpen);
      if (kept.includes(h)) continue;
      kept.push(h);
      this.#cover(h);
    }
    if (kept.length > MAX_KEPT_HEADS) kept.splice(0, kept.length - MAX_KEPT_HEADS);
    this.#heads.set(signer, kept);
  }

  /** The heads already received from people who are admins now count as admin heads. */
  promote(admins: ReadonlySet<string>): void {
    for (const [signer, heads] of this.#heads) {
      if (!admins.has(signer)) continue;
      for (const h of heads) this.#cover(h, this.#adminCovered, this.#adminOpen);
    }
  }

  /** True when the change is an attested head of an admin, or an ancestor of one. */
  adminCovers(hash: string): boolean {
    return this.#adminCovered.has(hash);
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
      if (this.#open.delete(c.hash)) for (const d of c.deps) this.#cover(d);
      if (this.#adminOpen.delete(c.hash)) {
        for (const d of c.deps) this.#cover(d, this.#adminCovered, this.#adminOpen);
      }
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
    const out = (h: string) => this.#rejected.has(h) || this.#waiting.has(h);
    if (!heads.some(out)) return heads;
    return heads.filter((h) => !out(h));
  }

  /**
   * Marks changes as held: we have them aside and will judge them again, so a peer whose heads
   * name them is not asked for them over and over.
   */
  wait(hashes: string[]): void {
    for (const h of hashes) this.#waiting.add(h);
    for (const h of this.#waiting) {
      if (this.#waiting.size <= MAX_REJECTED) break;
      this.#waiting.delete(h);
    }
  }

  /** The changes are not held any more (delivered, or refused for good). */
  unwait(hashes: string[]): void {
    for (const h of hashes) this.#waiting.delete(h);
  }

  #cover(root: string, covered = this.#covered, open = this.#open): void {
    const todo = [root];
    for (let h = todo.pop(); h !== undefined; h = todo.pop()) {
      if (covered.has(h)) continue;
      covered.add(h);
      if (covered === this.#covered) this.#rejected.delete(h); // a writer took it in after all
      const deps = this.#deps.get(h);
      if (deps) todo.push(...deps);
      else open.add(h);
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
/**
 * A check on top of coverage, for one document: the admission rules of a document whose changes
 * mean something (the project index). It sees each covered change in order and refuses it with
 * a reason, or lets it through; a document that arrives whole is judged as a whole.
 */
export interface DocValidator {
  /** `adminCovered` says whether an admin vouches for the change with that hash. */
  check(bytes: Uint8Array, adminCovered: (hash: string) => boolean): string | undefined;
  /** A whole document: judged change by change, in causal order, as if they came one by one. */
  checkDocument(doc: A.Doc<never>, adminCovered: (hash: string) => boolean): string | undefined;
}

export class Coverage {
  readonly stats: CoverageStats = {
    removed: 0,
    dropped: 0,
    fragments: 0,
    repeated: 0,
  };
  #docs = new Map<string, DocCoverage>();
  #validators = new Map<string, DocValidator>();
  #admins: ReadonlySet<string> = new Set();
  #lastRewritten = new Map<string, Uint8Array>(); // peer + document → last message we had to cut
  #held: Held[] = [];
  #heldDirty = false;

  /** `signer`, a current writer, attested `heads` of document `doc`. */
  attested(signer: string, doc: string, heads: string[]): void {
    this.#doc(doc).attest(signer, heads, this.#admins.has(signer));
    this.#heldDirty = true;
  }

  /**
   * The devices that are admins now (from the `Roles` notice). Their heads, received before or
   * after, count as admin heads for the documents that have a validator.
   */
  setAdmins(admins: Iterable<string>): void {
    this.#admins = new Set(admins);
    for (const d of this.#docs.values()) d.promote(this.#admins);
    this.#heldDirty = true;
  }

  /** Adds the admission rules of document `doc`, applied to what coverage lets through. */
  setValidator(doc: string, validator: DocValidator): void {
    this.#validators.set(doc, validator);
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
    const waiting = new Set<Chunk>();
    const vetoed = this.#validate(doc, docs, chunks, covered, waiting);
    if (waiting.size > 0) {
      this.#hold({ doc, from, msg, chunks: [...waiting] });
      docs.wait([...waiting].flatMap((c) => c.changes.map((x) => x.hash)));
    }
    this.stats.removed += docs.reject(
      chunks
        .filter((_, i) => !covered[i])
        .flatMap((c) => c.changes.filter((x) => !accepted.has(x.hash) || vetoed.has(c))),
    );
    if (chunks.some((c, i) => c.isDocument && !covered[i])) {
      this.stats.dropped++;
      return undefined;
    }
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

  /**
   * Runs the validator of `doc`, if it has one, on the chunks coverage let through, in order.
   * Returns the chunks it refused, and marks them uncovered.
   */
  #validate(
    doc: string,
    docs: DocCoverage,
    chunks: Chunk[],
    covered: boolean[],
    waiting: Set<Chunk>,
  ): Set<Chunk> {
    const vetoed = new Set<Chunk>();
    const validator = this.#validators.get(doc);
    if (!validator) return vetoed;
    chunks.forEach((chunk, i) => {
      if (!covered[i]) return;
      const admin = (h: string) => docs.adminCovers(h);
      const refusal = chunk.loaded
        ? validator.checkDocument(chunk.loaded as A.Doc<never>, admin)
        : validator.check(chunk.bytes, admin);
      if (refusal === undefined) return;
      covered[i] = false;
      if (refusal.startsWith(HOLD)) {
        waiting.add(chunk); // kept aside, not refused: judged again when something changes
        return;
      }
      console.warn(`index change refused: ${refusal}`);
      vetoed.add(chunk);
    });
    return vetoed;
  }

  #hold(entry: Held): void {
    this.#held.push(entry);
    const mine = this.#held.filter((h) => h.from === entry.from);
    if (mine.length > MAX_HELD_PER_SENDER) {
      this.#forget(this.#held.splice(this.#held.indexOf(mine[0] as Held), 1)[0]);
    }
    if (this.#held.length > MAX_HELD) this.#forget(this.#held.shift());
  }

  /** A held message that is dropped: its changes can be asked for again. */
  #forget(h: Held | undefined): void {
    if (h) this.#doc(h.doc).unwait(h.chunks.flatMap((c) => c.changes.map((x) => x.hash)));
  }

  /**
   * Judges again what was held, now that roles or attestations may have changed, and returns the
   * messages that can be delivered at last (only the chunks that now pass). What still waits
   * stays; what is refused for good is forgotten. Call after {@link attested} and
   * {@link setAdmins}.
   */
  releaseHeld(): { doc: string; from: string; data: Uint8Array }[] {
    if (!this.#heldDirty || this.#held.length === 0) {
      this.#heldDirty = false;
      return [];
    }
    this.#heldDirty = false;
    const out: { doc: string; from: string; data: Uint8Array }[] = [];
    // In the order they were held (what builds on a change comes after it), again until nothing
    // moves: a change that was admitted lets its dependents through.
    let queue = this.#held;
    for (let moved = true; moved && queue.length > 0; ) {
      moved = false;
      const next: Held[] = [];
      for (const h of queue) {
        const docs = this.#doc(h.doc);
        const validator = this.#validators.get(h.doc);
        const accepted = docs.admit(h.chunks.flatMap((c) => c.changes));
        const admin = (hash: string) => docs.adminCovers(hash);
        const ready: Chunk[] = [];
        const waiting: Chunk[] = [];
        for (const chunk of h.chunks) {
          const hashes = chunk.changes.map((x) => x.hash);
          if (!validator || !chunk.changes.every((x) => accepted.has(x.hash))) {
            docs.unwait(hashes); // not covered any more
            continue;
          }
          const refusal = chunk.loaded
            ? validator.checkDocument(chunk.loaded as A.Doc<never>, admin)
            : validator.check(chunk.bytes, admin);
          if (refusal === undefined) {
            ready.push(chunk);
            docs.unwait(hashes);
          } else if (refusal.startsWith(HOLD)) waiting.push(chunk);
          else {
            docs.unwait(hashes);
            console.warn(`index change refused: ${refusal}`);
            this.stats.removed += docs.reject(chunk.changes);
          }
        }
        if (ready.length > 0) {
          moved = true;
          const heads = docs.withoutRejected(h.msg.heads);
          const changes = ready.map((c) => c.bytes);
          out.push({
            doc: h.doc,
            from: h.from,
            data: A.encodeSyncMessage({ ...h.msg, heads, changes }),
          });
        }
        if (waiting.length > 0) next.push({ ...h, chunks: waiting });
      }
      queue = next;
    }
    this.#held = queue;
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
  /** The document of a whole-document chunk, loaded to read its changes. */
  loaded?: A.Doc<unknown>;
  changes: ChangeInfo[];
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
    return { bytes, isDocument: true, loaded: doc, changes };
  }
  const { hash, deps } = A.decodeChange(bytes);
  return { bytes, isDocument: false, changes: [{ hash, deps }] };
}
