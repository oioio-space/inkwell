import { describe, expect, it } from "vitest";
import { looksTabular, startDialect } from "./dialect";

const COMMA = "nom,qte\nA,1\nB,2\nC,3\n";
const SEMICOLON = "nom;prix\nA;1,5\nB;2,5\nC;3,5\n";
const TAB = "nom\tqte\nA\t1\nB\t2\nC\t3\n";
const PIPE = "nom|qte\nA|1\nB|2\nC|3\n";
const SINGLE = "seul\n1\n2\n3\n";

describe("the extension chooses the starting dialect, the content confirms it (CSV-DIALECT)", () => {
  it("each extension: .csv comma, .tsv and .tab tab without quoting, .psv pipe", () => {
    expect(startDialect("a.csv", COMMA)).toEqual({
      dialect: { delimiter: ",", header: true, decimal: "." },
    });
    for (const path of ["a.tsv", "a.TAB"]) {
      const { dialect, contradiction } = startDialect(path, TAB);
      expect(dialect.delimiter).toBe("\t");
      expect(dialect.quote).toBe(false);
      expect(contradiction).toBeUndefined();
    }
    const psv = startDialect("a.psv", PIPE);
    expect(psv.dialect.delimiter).toBe("|");
    expect(psv.contradiction).toBeUndefined();
  });

  it("a .csv with semicolons (decimal comma) is read as such, with nothing to report", () => {
    const { dialect, contradiction } = startDialect("prix.csv", SEMICOLON);
    expect(dialect.delimiter).toBe(";");
    expect(dialect.decimal).toBe(",");
    expect(contradiction).toBeUndefined();
  });

  it("a .csv that is really tab-separated is read as TSV and the contradiction is reported", () => {
    const { dialect, contradiction } = startDialect("a.csv", TAB);
    expect(dialect.delimiter).toBe("\t");
    expect(contradiction).toEqual({ found: "\t", expected: "," });
  });

  it("a .tsv that is really commas, and a .csv that is really pipes, are reported too", () => {
    expect(startDialect("a.tsv", COMMA).contradiction).toEqual({ found: ",", expected: "\t" });
    expect(startDialect("a.csv", PIPE).contradiction).toEqual({ found: "|", expected: "," });
  });

  it("an ambiguous single column keeps what the extension says and reports nothing", () => {
    expect(startDialect("a.csv", SINGLE).dialect.delimiter).toBe(",");
    expect(startDialect("a.tsv", SINGLE).dialect.delimiter).toBe("\t");
    expect(startDialect("a.psv", SINGLE).dialect.delimiter).toBe("|");
    expect(startDialect("a.csv", SINGLE).contradiction).toBeUndefined();
    expect(startDialect("a.csv", "").contradiction).toBeUndefined();
  });

  it("the person's earlier choice for the file wins, without a new question", () => {
    const { dialect, contradiction } = startDialect("a.csv", TAB, ",");
    expect(dialect.delimiter).toBe(",");
    expect(contradiction).toBeUndefined();
  });

  it("a .txt that looks like a table is offered, read by detection, never reported", () => {
    expect(looksTabular(TAB)).toBe(true);
    expect(looksTabular(SEMICOLON)).toBe(true);
    expect(looksTabular("Bonjour,\nceci est un texte\nsans tableau.\n")).toBe(false);
    expect(looksTabular(SINGLE)).toBe(false);
    const { dialect, contradiction } = startDialect("notes.txt", TAB);
    expect(dialect.delimiter).toBe("\t");
    expect(contradiction).toBeUndefined();
  });
});
