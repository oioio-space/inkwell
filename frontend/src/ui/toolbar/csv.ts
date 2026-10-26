/** Parses CSV text (RFC 4180: quoted cells, doubled quotes, newlines inside quotes), `,` or another delimiter. */
export function parseCsv(text: string, delimiter = ","): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = "";
  let quoted = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (quoted) {
      if (ch === '"' && text[i + 1] === '"') {
        cell += '"';
        i++;
      } else if (ch === '"') {
        quoted = false;
      } else {
        cell += ch;
      }
    } else if (ch === '"' && cell === "") {
      quoted = true;
    } else if (ch === delimiter) {
      row.push(cell);
      cell = "";
    } else if (ch === "\n" || ch === "\r") {
      if (ch === "\r" && text[i + 1] === "\n") i++;
      row.push(cell);
      rows.push(row);
      row = [];
      cell = "";
    } else {
      cell += ch;
    }
  }
  if (cell !== "" || row.length > 0) {
    row.push(cell);
    rows.push(row);
  }
  return rows;
}

/** The citation keys of a BibTeX/BibLaTeX file (`@article{key,`), in file order. */
export function bibKeys(text: string): string[] {
  const keys: string[] = [];
  for (const m of text.matchAll(/^\s*@(\w+)\s*[{(]\s*([^,\s{}()]+)\s*,/gm)) {
    const kind = (m[1] ?? "").toLowerCase();
    if (kind !== "comment" && kind !== "string" && kind !== "preamble" && m[2]) keys.push(m[2]);
  }
  return keys;
}

/** The labels (`<label>`) a Typst document defines, to offer them as cross-reference targets. */
export function typstLabels(doc: string): string[] {
  return [...new Set([...doc.matchAll(/<([\p{L}\p{N}_:.-]+)>/gu)].map((m) => m[1] ?? ""))].filter(
    Boolean,
  );
}
