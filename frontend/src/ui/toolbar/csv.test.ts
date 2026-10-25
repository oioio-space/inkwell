import { describe, expect, it } from "vitest";
import { bibKeys, parseCsv, typstLabels } from "./csv";

describe("parseCsv", () => {
  it("reads rows and cells", () => {
    expect(parseCsv("a,b\n1,2\n")).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
    expect(parseCsv("a,b\r\n1,2")).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
  });

  it("reads quoted cells with commas, doubled quotes and line breaks", () => {
    expect(parseCsv('"x, y","il dit ""oui""","a\nb"')).toEqual([["x, y", 'il dit "oui"', "a\nb"]]);
  });

  it("keeps empty cells and ignores a missing final newline", () => {
    expect(parseCsv("a,,c")).toEqual([["a", "", "c"]]);
    expect(parseCsv("")).toEqual([]);
  });
});

describe("bibKeys", () => {
  it("lists the entry keys and skips comments and strings", () => {
    const bib = `@string{x = "y"}\n@article{dupont2020,\n title={A}\n}\n@Book{ martin-1999 ,\n}\n@comment{nope,}`;
    expect(bibKeys(bib)).toEqual(["dupont2020", "martin-1999"]);
  });
});

describe("typstLabels", () => {
  it("lists each label once", () => {
    expect(typstLabels("= A <intro>\n= B <bascule>\nvoir <intro>")).toEqual(["intro", "bascule"]);
  });
});
