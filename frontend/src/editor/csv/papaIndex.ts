import Papa from "papaparse";

/** What the first parse of a file gives the grid: where records start and how wide they are. */
export interface ParsedIndex {
  starts: Int32Array;
  cols: Uint16Array;
}

/**
 * Reads the whole text with Papa Parse (5.7.0) in streaming mode and keeps only the offsets
 * of the records: cells are parsed on demand later, so memory stays bounded. `step` gives
 * the cursor after each record, which is where the next one starts. Runs inside a Worker.
 */
export function papaIndex(
  text: string,
  delimiter: string,
  progress: (done: number) => void = () => {},
  first?: (partial: { starts: Int32Array; cols: Uint16Array }) => void,
): ParsedIndex {
  let cap = 1 << 16;
  let starts = new Int32Array(cap);
  let cols = new Uint16Array(cap);
  let n = 0;
  let prev = 0;
  let lastReport = 0;
  let announced = false;
  Papa.parse<string[]>(text, {
    delimiter,
    newline: "\n",
    quoteChar: '"',
    skipEmptyLines: false,
    step(res) {
      if (n === cap) {
        cap *= 2;
        const s = new Int32Array(cap);
        s.set(starts);
        const c = new Uint16Array(cap);
        c.set(cols);
        starts = s;
        cols = c;
      }
      starts[n] = prev;
      cols[n] = Math.min(res.data.length, 0xffff);
      n++;
      prev = res.meta.cursor;
      if (!announced && n >= 60) {
        announced = true;
        first?.({ starts: starts.slice(0, n), cols: cols.slice(0, n) });
      }
      if (prev - lastReport > 2_000_000) {
        lastReport = prev;
        progress(prev / Math.max(1, text.length));
      }
    },
  });
  // Papa reports a cursor past a final record that has no terminator; a text that ends with a
  // newline has no phantom record.
  while (n > 0 && (starts[n - 1] as number) >= text.length) n--;
  progress(1);
  return { starts: starts.slice(0, n), cols: cols.slice(0, n) };
}
