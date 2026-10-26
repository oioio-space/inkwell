import { typstString as escapeTypst } from "../format/typst";
import type { Dialect } from "./dialect";

/**
 * The plotting package of the chart snippet. Pinned in wave C (`.typstdeps`); the snippet is
 * generated now and unit-tested, the preview waits for wave C.
 */
export const CHART_PACKAGE = "@preview/lilaq:0.5.0";

const typstString = (s: string): string => `"${escapeTypst(s)}"`;

const delimiterArg = (d: Dialect["delimiter"]) =>
  d === "," ? "" : `, delimiter: ${typstString(d)}`;

/** "Tableau depuis un CSV": a table reading the file, which stays linked to it. */
export function csvTableCode(path: string, dialect: Dialect, columns: number): string {
  const read = `csv(${typstString(path)}${delimiterArg(dialect.delimiter)})`;
  return `#table(columns: ${Math.max(1, columns)}, ..${read}.flatten())`;
}

/** "Graphique depuis un CSV": a line plot of column `x` against column `y` (0-based). */
export function csvChartCode(
  path: string,
  dialect: Dialect,
  x: number,
  y: number,
  header = dialect.header,
): string {
  const num = (i: number) =>
    dialect.decimal === "," ? `float(r.at(${i}).replace(",", "."))` : `float(r.at(${i}))`;
  const rows = `csv(${typstString(path)}${delimiterArg(dialect.delimiter)})${header ? ".slice(1)" : ""}`;
  return [
    `#import ${typstString(CHART_PACKAGE)} as lq`,
    `#let rows = ${rows}`,
    "#lq.diagram(",
    `  lq.plot(rows.map(r => ${num(x)}), rows.map(r => ${num(y)})),`,
    ")",
  ].join("\n");
}
