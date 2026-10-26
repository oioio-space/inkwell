import { detectDialect } from "./dialect";
import { CsvModel } from "./model";
import { rawCell } from "./records";

/** A rectangular range of cells, inclusive, in record/cell coordinates. */
export interface Range {
  r0: number;
  c0: number;
  r1: number;
  c1: number;
}

const FORMULA_START = /^[=+\-@\t\r]/;

/** A cell that a spreadsheet would run as a formula (a plain number such as -5 is not one). */
export function looksLikeFormula(value: string): boolean {
  if (!FORMULA_START.test(value)) return false;
  return !/^[-+]?\d+([.,]\d+)?$/.test(value.trim());
}

const escapeHtml = (s: string) =>
  s
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");

/** The cells of a range as rows of text. */
export function cellsOf(model: CsvModel, range: Range): string[][] {
  const rows: string[][] = [];
  for (let r = range.r0; r <= range.r1; r++) {
    const cells = model.record(r);
    const row: string[] = [];
    for (let c = range.c0; c <= range.c1; c++) row.push(cells[c]?.value ?? "");
    rows.push(row);
  }
  return rows;
}

/** TSV as spreadsheets write it: a cell with a tab, quote or newline is quoted. */
export function toTsv(rows: readonly (readonly string[])[]): string {
  return rows.map((r) => r.map((v) => rawCell(v, "\t")).join("\t")).join("\n");
}

/** A plain `<table>`: no styles, so pasting elsewhere takes the cells and nothing else. */
export function toHtml(rows: readonly (readonly string[])[]): string {
  const body = rows
    .map(
      (r) =>
        `<tr>${r.map((v) => `<td>${escapeHtml(v).replaceAll("\n", "<br>")}</td>`).join("")}</tr>`,
    )
    .join("");
  return `<table><tbody>${body}</tbody></table>`;
}

/** Prefixes cells that would run as formulas with an apostrophe; counts them. */
export function neutralise(rows: readonly (readonly string[])[]): {
  rows: string[][];
  count: number;
} {
  let count = 0;
  const out = rows.map((r) =>
    r.map((v) => {
      if (!looksLikeFormula(v)) return v;
      count++;
      return `'${v}`;
    }),
  );
  return { rows: out, count };
}

/** Reads the table of an HTML fragment: the text of each cell, nothing of its markup. */
export function htmlTable(html: string): string[][] | undefined {
  if (!/<table[\s>]/i.test(html) || typeof DOMParser === "undefined") return undefined;
  // DOMParser builds an inert document: no script runs and no resource loads.
  const doc = new DOMParser().parseFromString(html, "text/html");
  const table = doc.querySelector("table");
  if (!table) return undefined;
  const rows: string[][] = [];
  for (const dead of Array.from(table.querySelectorAll("script, style, template"))) dead.remove();
  for (const tr of Array.from(table.querySelectorAll("tr"))) {
    if (tr.closest("table") !== table) continue;
    const row: string[] = [];
    for (const cell of Array.from(tr.children)) {
      if (cell.tagName !== "TD" && cell.tagName !== "TH") continue;
      for (const br of Array.from(cell.querySelectorAll("br"))) br.replaceWith("\n");
      const text = (cell.textContent ?? "")
        .split("")
        .filter((ch) => {
          const n = ch.charCodeAt(0);
          return n > 31 || n === 9 || n === 10 || n === 13;
        })
        .join("")
        .replace(/\r\n?/g, "\n");
      const span = Number.parseInt(cell.getAttribute("colspan") ?? "1", 10);
      row.push(text.trim());
      for (let i = 1; i < Math.min(span || 1, 50); i++) row.push("");
    }
    rows.push(row);
  }
  return rows.length ? rows : undefined;
}

/** Reads delimited text (TSV from a spreadsheet, or CSV) into rows of cells. */
export function delimitedText(text: string): string[][] {
  const clean = text.replace(/\r\n?/g, "\n").replace(/\n$/, "");
  if (clean === "") return [[""]];
  const delimiter = clean.includes("\t") ? "\t" : detectDialect(clean).delimiter;
  const model = new CsvModel(`${clean}\n`, { delimiter, header: false, decimal: "." });
  return Array.from({ length: model.records }, (_, r) => model.record(r).map((c) => c.value));
}

/** What a paste carries, as rows: an HTML table wins over text; text is split on tabs/delimiters. */
export function fromClipboard(data: { text?: string; html?: string }): string[][] | undefined {
  if (data.html) {
    const t = htmlTable(data.html);
    if (t) return t;
  }
  if (data.text !== undefined && data.text !== "") return delimitedText(data.text);
  return undefined;
}

/** Size of a block of rows: [rows, columns]. */
export function sizeOf(rows: readonly (readonly string[])[]): [number, number] {
  return [rows.length, Math.max(0, ...rows.map((r) => r.length))];
}
