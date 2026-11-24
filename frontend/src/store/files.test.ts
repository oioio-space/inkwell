import { describe, expect, it } from "vitest";
import vectors from "../../../internal/bridge/testdata/storestream.json";
import { decodeFileInfo, decodeFileList, Eol } from "./files";
import { decodeBlobInfo, decodeReferenced, decodeTree, decodeTreeResult } from "./tree";

const unhex = (s: string) => Uint8Array.from(s.match(/../g) ?? [], (b) => Number.parseInt(b, 16));
type Vector = { name: string; type: string; wireHex: string } & Record<string, unknown>;
const all = vectors as Vector[];
const of = (type: string) => all.filter((v) => v.type === type);

describe("file list v2", () => {
  it("EX0 R-9 the file list v2 decodes the shared vectors", () => {
    const lists = of("filelist");
    expect(lists.length).toBeGreaterThan(0);
    for (const v of lists) {
      const wire = unhex(v.wireHex);
      expect(wire[0]).toBe(2);
      const files = v.files as { path: string; size: number; main: boolean }[];
      for (const f of files) expect(Object.keys(f).sort()).toEqual(["main", "path", "size"]);
      expect(decodeFileList(wire)).toEqual(files);
    }
    // Version 1, with its kind byte per entry (`main.typ`, typst, 4 bytes, main): refused by its
    // version, not misread.
    const v1 = unhex("010000000101010000000000000004" + "00086d61696e2e747970");
    expect(() => decodeFileList(v1)).toThrow("bad file list header");
    // The other payloads keep version 1.
    for (const v of of("fileinfo")) {
      const wire = unhex(v.wireHex);
      expect(wire[0]).toBe(1);
      const i = v.info as { encoding: string; eol: keyof typeof Eol; bom: boolean; mixed: boolean };
      expect(decodeFileInfo(wire)).toEqual({ ...i, eol: Eol[i.eol] });
    }
    const trees = of("tree");
    expect(trees.length).toBeGreaterThan(0);
    for (const v of trees) {
      expect(unhex(v.wireHex)[0]).toBe(1);
      expect(decodeTree(unhex(v.wireHex)).length).toBe((v.entries as unknown[]).length);
    }
    for (const v of of("treeresult"))
      expect(() => decodeTreeResult(unhex(v.wireHex))).not.toThrow();
    for (const v of of("referenced"))
      expect(() => decodeReferenced(unhex(v.wireHex))).not.toThrow();
    for (const v of of("blobinfo")) expect(() => decodeBlobInfo(unhex(v.wireHex))).not.toThrow();
  });
});
