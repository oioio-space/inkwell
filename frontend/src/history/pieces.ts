/** A stretch of text written by one session, or (length 0) a marker where text was removed. */
export interface Piece {
  len: number;
  s: number;
  text: string;
  replace: boolean;
  /** A marker: text of session `owner` removed by session `by` at this point. */
  del?: { by: number; owner: number };
}

/** Where a position falls in the list: a block, and an index in it. */
export interface Loc {
  b: number;
  i: number;
}

/** A block is cut in two halves of this many pieces once it holds more than twice as many. */
export const SPLIT_AT = 512;

/**
 * The work of a piece list and of its copies: `finds`, the places looked up
 * ({@link PieceList.locate}); `steps`, the blocks and pieces walked over, copied or shifted. A count
 * is the same on every machine and under any load, so the tests that bound what following a
 * document costs read it, not the clock (FE1). The application never reads it.
 */
export interface PieceWork {
  finds: number;
  steps: number;
}

/**
 * The pieces of the text, in order, kept in blocks with the length of text each holds. Finding a
 * position walks the blocks, then one block: a few hundred steps whatever the number of pieces,
 * where a flat list costs a walk of all of them for every edit (and so, for the thirty thousand
 * edits of a rewrite, a thousand times more).
 */
export class PieceList {
  #blocks: Piece[][] = [[]];
  #lens: number[] = [0];

  constructor(readonly work: PieceWork = { finds: 0, steps: 0 }) {}

  clone(): PieceList {
    const out = new PieceList(this.work);
    out.#blocks = this.#blocks.map((b) => b.slice());
    out.#lens = this.#lens.slice();
    this.work.steps += this.#blocks.length + this.#pieceCount();
    return out;
  }

  #pieceCount(): number {
    let n = 0;
    for (const b of this.#blocks) n += b.length;
    return n;
  }

  clear(): void {
    this.#blocks = [[]];
    this.#lens = [0];
  }

  /** All the pieces, in order. */
  toArray(): Piece[] {
    const all = this.#blocks.flat();
    this.work.steps += this.#blocks.length + all.length;
    return all;
  }

  /**
   * The first place where the text before is `pos` characters; `afterMarkers`: past the markers
   * that stand there. A piece that spans `pos` is cut in two.
   */
  locate(pos: number, afterMarkers: boolean): Loc {
    let acc = 0;
    let b = 0;
    while (b < this.#blocks.length - 1 && pos > acc + (this.#lens[b] as number)) {
      acc += this.#lens[b] as number;
      b++;
    }
    const block = this.#blocks[b] as Piece[];
    this.work.finds++;
    // The blocks walked, and the whole block: what this costs at most, cut or not.
    this.work.steps += b + block.length;
    let at = acc;
    for (let i = 0; i < block.length; i++) {
      const p = block[i] as Piece;
      if (at === pos && (p.len > 0 || !afterMarkers)) return { b, i };
      if (at + p.len > pos) {
        const k = pos - at;
        block.splice(
          i,
          1,
          { ...p, len: k, text: p.text.slice(0, k) },
          { ...p, len: p.len - k, text: p.text.slice(k) },
        );
        return { b, i: i + 1 };
      }
      at += p.len;
    }
    return { b, i: block.length };
  }

  /** Puts pieces at a place found by {@link locate}. */
  insert(at: Loc, ...pieces: Piece[]): void {
    const block = this.#blocks[at.b] as Piece[];
    block.splice(at.i, 0, ...pieces);
    this.work.steps += block.length;
    this.#lens[at.b] = (this.#lens[at.b] as number) + this.#sum(pieces);
    if (block.length > 2 * SPLIT_AT) {
      this.work.steps += this.#blocks.length;
      const tail = block.splice(SPLIT_AT);
      this.#blocks.splice(at.b + 1, 0, tail);
      this.#lens[at.b] = this.#sum(block);
      this.#lens.splice(at.b + 1, 0, this.#sum(tail));
    }
  }

  /**
   * Takes out what lies between two places (`from` before `to`) and puts `markers` there. Returns
   * the pieces taken out.
   */
  replace(from: Loc, to: Loc, markers: Piece[]): Piece[] {
    const first = this.#blocks[from.b] as Piece[];
    let removed: Piece[];
    if (from.b === to.b) {
      removed = first.splice(from.i, to.i - from.i, ...markers);
      this.#lens[from.b] = this.#sum(first);
      return removed;
    }
    const last = this.#blocks[to.b] as Piece[];
    this.work.steps += this.#blocks.length;
    removed = first.splice(from.i, first.length - from.i, ...markers);
    for (let b = from.b + 1; b < to.b; b++) removed.push(...(this.#blocks[b] as Piece[]));
    removed.push(...last.splice(0, to.i));
    this.#blocks.splice(from.b + 1, to.b - from.b - 1);
    this.#lens.splice(from.b + 1, to.b - from.b - 1);
    this.#lens[from.b] = this.#sum(first);
    this.#lens[from.b + 1] = this.#sum(last);
    if (last.length === 0 && this.#blocks.length > 1) {
      this.#blocks.splice(from.b + 1, 1);
      this.#lens.splice(from.b + 1, 1);
    }
    this.work.steps += removed.length;
    return removed;
  }

  /** The text length of `pieces`, counting the walk. */
  #sum(pieces: readonly Piece[]): number {
    this.work.steps += pieces.length;
    return sum(pieces);
  }
}

const sum = (pieces: readonly Piece[]): number => {
  let n = 0;
  for (const p of pieces) n += p.len;
  return n;
};
