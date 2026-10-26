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

const SPLIT_AT = 512;

/**
 * The pieces of the text, in order, kept in blocks with the length of text each holds. Finding a
 * position walks the blocks, then one block: a few hundred steps whatever the number of pieces,
 * where a flat list costs a walk of all of them for every edit (and so, for the thirty thousand
 * edits of a rewrite, a thousand times more).
 */
export class PieceList {
  #blocks: Piece[][] = [[]];
  #lens: number[] = [0];

  clone(): PieceList {
    const out = new PieceList();
    out.#blocks = this.#blocks.map((b) => b.slice());
    out.#lens = this.#lens.slice();
    return out;
  }

  clear(): void {
    this.#blocks = [[]];
    this.#lens = [0];
  }

  /** All the pieces, in order. */
  toArray(): Piece[] {
    return this.#blocks.flat();
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
    this.#lens[at.b] = (this.#lens[at.b] as number) + sum(pieces);
    if (block.length > 2 * SPLIT_AT) {
      const tail = block.splice(SPLIT_AT);
      this.#blocks.splice(at.b + 1, 0, tail);
      this.#lens[at.b] = sum(block);
      this.#lens.splice(at.b + 1, 0, sum(tail));
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
      this.#lens[from.b] = sum(first);
      return removed;
    }
    const last = this.#blocks[to.b] as Piece[];
    removed = first.splice(from.i, first.length - from.i, ...markers);
    for (let b = from.b + 1; b < to.b; b++) removed.push(...(this.#blocks[b] as Piece[]));
    removed.push(...last.splice(0, to.i));
    this.#blocks.splice(from.b + 1, to.b - from.b - 1);
    this.#lens.splice(from.b + 1, to.b - from.b - 1);
    this.#lens[from.b] = sum(first);
    this.#lens[from.b + 1] = sum(last);
    if (last.length === 0 && this.#blocks.length > 1) {
      this.#blocks.splice(from.b + 1, 1);
      this.#lens.splice(from.b + 1, 1);
    }
    return removed;
  }
}

const sum = (pieces: readonly Piece[]): number => {
  let n = 0;
  for (const p of pieces) n += p.len;
  return n;
};
