import { describe, expect, it } from "vitest";
import vectors from "../../../internal/bridge/testdata/storestream.json";
import {
  decodeFileInfo,
  decodeFileList,
  decodeInvalidLine,
  Eol,
  encodeFileInfo,
  encodeFileList,
  FileKind,
} from "./files";
import {
  decodeRange,
  decodeRequest,
  decodeResponse,
  encodeRange,
  encodeRequest,
  encodeResponse,
  Op,
  ProtocolError,
  Status,
  validComponent,
} from "./protocol";
import {
  decodeBlobInfo,
  decodeReferenced,
  decodeTooLarge,
  decodeTree,
  decodeTreeResult,
  encodeBlobInfo,
  encodeReferenced,
  encodeTree,
  encodeTreeResult,
  NodeType,
  type TreeEntry,
} from "./tree";

const hex = (b: Uint8Array) => Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
const unhex = (s: string) => Uint8Array.from(s.match(/../g) ?? [], (h) => Number.parseInt(h, 16));

describe("store protocol golden vectors (shared with Go)", () => {
  for (const v of vectors) {
    it(v.name, () => {
      if (v.type === "request") {
        const req = {
          id: v.id as number,
          op: Op[v.op as keyof typeof Op],
          key: v.key as string[],
          data: unhex(v.dataHex as string),
        };
        expect(hex(encodeRequest(req))).toBe(v.wireHex);
        const back = decodeRequest(unhex(v.wireHex));
        expect(back).toEqual(req);
      } else if (v.type === "response") {
        const resp = {
          id: v.id as number,
          status: Status[v.status as keyof typeof Status],
          payload: unhex(v.payloadHex as string),
        };
        expect(hex(encodeResponse(resp))).toBe(v.wireHex);
        expect(decodeResponse(unhex(v.wireHex))).toEqual(resp);
      } else if (v.type === "filelist") {
        const files = (
          v.files as { path: string; kind: string; size: number; main: boolean }[]
        ).map((f) => ({ ...f, kind: FileKind[f.kind as keyof typeof FileKind] }));
        expect(hex(encodeFileList(files))).toBe(v.wireHex);
        expect(decodeFileList(unhex(v.wireHex))).toEqual(files);
      } else if (v.type === "fileinfo") {
        const i = v.info as { encoding: string; eol: string; bom: boolean; mixed: boolean };
        const info = { ...i, eol: Eol[i.eol as keyof typeof Eol] };
        expect(hex(encodeFileInfo(info))).toBe(v.wireHex);
        expect(decodeFileInfo(unhex(v.wireHex))).toEqual(info);
      } else if (v.type === "tree") {
        const entries: TreeEntry[] = (
          v.entries as {
            path: string;
            type: keyof typeof NodeType;
            url?: string;
            hash?: string;
            size?: number;
            main?: boolean;
          }[]
        ).map((e) => ({
          path: e.path,
          type: NodeType[e.type],
          ...(e.url === undefined ? {} : { url: e.url }),
          ...(e.hash === undefined ? {} : { hash: unhex(e.hash) }),
          size: e.size ?? 0,
          main: e.main ?? false,
        }));
        expect(hex(encodeTree(entries))).toBe(v.wireHex);
        expect(decodeTree(unhex(v.wireHex))).toEqual(entries);
      } else if (v.type === "treeresult") {
        expect(hex(encodeTreeResult(v.rejected as number))).toBe(v.wireHex);
        expect(decodeTreeResult(unhex(v.wireHex))).toBe(v.rejected);
      } else if (v.type === "referenced") {
        const hashes = (v.hashes as string[]).map(unhex);
        expect(hex(encodeReferenced(hashes))).toBe(v.wireHex);
        expect(decodeReferenced(unhex(v.wireHex))).toEqual(hashes);
      } else if (v.type === "blobinfo") {
        const info = { hash: unhex(v.hash as string), size: v.size as number };
        expect(hex(encodeBlobInfo(info))).toBe(v.wireHex);
        expect(decodeBlobInfo(unhex(v.wireHex))).toEqual(info);
      } else {
        const entries = (v.entries as { key: string[]; dataHex: string }[]).map((e) => ({
          key: e.key,
          data: unhex(e.dataHex),
        }));
        expect(hex(encodeRange(entries))).toBe(v.wireHex);
        expect(decodeRange(unhex(v.wireHex))).toEqual(entries);
      }
    });
  }
});

describe("encodeRequest rejects what Go would", () => {
  const req = (key: string[], id = 1) => ({ id, op: Op.Load, key, data: new Uint8Array() });
  it.each([
    ["a slash", ["a/b"]],
    ["an empty component", [""]],
    ["a dot dot", [".."]],
    ["a space", ["a b"]],
    ["a non-ASCII character", ["é"]],
    ["a component over 128 characters", ["a".repeat(129)]],
    ["five components", ["a", "b", "c", "d", "e"]],
  ])("%s", (_name, key) => {
    expect(() => encodeRequest(req(key))).toThrow(ProtocolError);
  });
  it("id 0 (reserved for pushes)", () => {
    expect(() => encodeRequest(req(["a"], 0))).toThrow(ProtocolError);
  });
  it("accepts the longest valid component", () => {
    expect(encodeRequest(req(["a".repeat(128)])).length).toBe(4 + 1 + 1 + 1 + 128);
  });
});

describe("decoders reject malformed frames", () => {
  it("short response", () => expect(() => decodeResponse(unhex("000000"))).toThrow(ProtocolError));
  it("unknown status", () => {
    expect(() => decodeResponse(unhex("0000000109"))).toThrow(ProtocolError);
  });
  it.each([
    ["empty", ""],
    ["count without data", "00000001"],
    ["truncated key", "000000010201 61".replace(" ", "")],
    ["data past the end", "0000000101016100000009 01".replace(" ", "")],
    ["trailing bytes", "0000000007"],
  ])("range: %s", (_name, wire) => {
    expect(() => decodeRange(unhex(wire))).toThrow(ProtocolError);
  });
});

describe("file payload decoders reject what Go would", () => {
  // One-entry list: version, count 1, then kind | flags | size u64 | path length u16 | path.
  const one = (kind: string, flags: string, size: string, len: string, path: string) =>
    `0100000001${kind}${flags}${size}${len}${path}`;
  const zero = "0000000000000000";
  it.each([
    ["empty", ""],
    ["bad version", "0200000000"],
    ["count without entry", "0100000001"],
    ["bad kind", one("63", "00", zero, "0001", "61")],
    ["bad flags", one("01", "02", zero, "0001", "61")],
    ["empty path", one("01", "00", zero, "0000", "")],
    ["path past the end", one("01", "00", zero, "0005", "61")],
    ["size over 2^53", one("01", "00", "0020000000000001", "0001", "61")],
    ["invalid UTF-8 path", one("01", "00", zero, "0001", "ff")],
    ["trailing bytes", "010000000009"],
  ])("file list: %s", (_name, wire) => {
    expect(() => decodeFileList(unhex(wire))).toThrow(ProtocolError);
  });
  it.each([
    ["empty", ""],
    ["bad version", "0200000161"],
    ["bad flags", "0104000161"],
    ["mixed as eol", "0100030161"],
    ["short name", "0100000561"],
    ["long name", "010000016162"],
  ])("file info: %s", (_name, wire) => {
    expect(() => decodeFileInfo(unhex(wire))).toThrow(ProtocolError);
  });
  it("invalid-text line", () => {
    expect(decodeInvalidLine(unhex("00000103"))).toBe(259);
    expect(() => decodeInvalidLine(unhex("0003"))).toThrow(ProtocolError);
  });
  it("decodes the new statuses", () => {
    expect(decodeResponse(unhex("0000000605")).status).toBe(Status.Binary);
    expect(decodeResponse(unhex("000000070600000003")).status).toBe(Status.InvalidText);
    expect(() => decodeResponse(unhex("0000000108"))).toThrow(ProtocolError);
  });
});

describe("validComponent", () => {
  it("follows ^[A-Za-z0-9._-]{1,128}$ without . and ..", () => {
    for (const ok of ["a", "A-z_0.9", "2akvofn6bxdksK3uVG6qU6m9fyEP", "main.typ", "..a", "a.."]) {
      expect(validComponent(ok), ok).toBe(true);
    }
    for (const bad of ["", ".", "..", "a/b", "a~", "a b", "é"]) {
      expect(validComponent(bad), bad).toBe(false);
    }
  });
});

describe("path keys and the new operations", () => {
  const req = (op: Op, key: string[]) => ({ id: 1, op, key, data: new Uint8Array() });
  it("a path op takes one component per folder, with any name Go will judge", () => {
    const wire = encodeRequest(req(Op.ReadProjection, ["notes", "r\u00e9sum\u00e9 final.md"]));
    expect(decodeRequest(wire).key).toEqual(["notes", "r\u00e9sum\u00e9 final.md"]);
    const deep = Array.from({ length: 32 }, () => "d");
    expect(decodeRequest(encodeRequest(req(Op.WriteProjection, deep))).key).toEqual(deep);
  });
  it.each([
    ["no component", []],
    ["33 components", Array.from({ length: 33 }, () => "d")],
    ["an empty component", ["a", ""]],
    ["a slash inside a component", ["a/b"]],
    ["a component over 255 bytes", ["\u00e9".repeat(128)]],
  ])("a path op refuses %s", (_name, key) => {
    expect(() => encodeRequest(req(Op.PutBlob, key))).toThrow(ProtocolError);
  });
  it("Tree and Referenced take no key; storage keys stay strict", () => {
    expect(() => encodeRequest(req(Op.Tree, ["a"]))).toThrow(ProtocolError);
    expect(() => encodeRequest(req(Op.Load, ["a b"]))).toThrow(ProtocolError);
    expect(encodeRequest({ id: 2, op: Op.Tree, key: [], data: Uint8Array.of(1) }).length).toBe(7);
  });
  it("decodes the TooLarge status and its limit", () => {
    const limit = unhex("0000000006400000");
    expect(decodeResponse(unhex("0000001007")).status).toBe(Status.TooLarge);
    expect(decodeTooLarge(limit)).toBe(100 * 1024 * 1024);
    expect(() => decodeTooLarge(unhex("0000"))).toThrow(ProtocolError);
    expect(() => decodeResponse(unhex("0000000108"))).toThrow(ProtocolError);
  });
});

describe("tree payload decoders reject what Go would", () => {
  // One entry: type, flags, size u64, path length u16, path, reference length, reference.
  const one = (
    type: string,
    flags: string,
    size: string,
    len: string,
    path: string,
    rlen: string,
    ref: string,
  ) => `0100000001${type}${flags}${size}${len}${path}${rlen}${ref}`;
  const zero = "0000000000000000";
  const url = "6175746f6d657267653a78"; // automerge:x
  const hash = "01".repeat(32);
  it.each([
    ["empty", ""],
    ["bad version", "0200000000"],
    ["too many entries", "010000c351"],
    ["count without entries", "0100000001"],
    ["bad type", one("03", "00", zero, "0001", "61", "00", "")],
    ["bad flags", one("01", "02", zero, "0001", "61", "0b", url)],
    ["size over 2^53", one("01", "00", "0020000000000001", "0001", "61", "0b", url)],
    ["empty path", one("01", "00", zero, "0000", "", "0b", url)],
    ["path over the limit", one("01", "00", zero, "0401", "61".repeat(1025), "0b", url)],
    ["invalid UTF-8 path", one("01", "00", zero, "0001", "ff", "0b", url)],
    ["text without a URL", one("01", "00", zero, "0001", "61", "00", "")],
    ["blob with a short hash", one("02", "00", zero, "0001", "61", "1f", hash.slice(0, 62))],
    ["folder with a reference", one("00", "00", zero, "0001", "61", "20", hash)],
    ["trailing bytes", `${one("00", "00", zero, "0001", "61", "00", "")}00`],
  ])("tree: %s", (_name, wire) => {
    expect(() => decodeTree(unhex(wire))).toThrow(ProtocolError);
  });
  it("tree: refuses what it cannot encode", () => {
    const bad = (e: Partial<TreeEntry>): TreeEntry => ({
      path: "a",
      type: NodeType.dir,
      size: 0,
      main: false,
      ...e,
    });
    expect(() => encodeTree([bad({ path: "" })])).toThrow(ProtocolError);
    expect(() => encodeTree([bad({ type: NodeType.text })])).toThrow(ProtocolError);
    expect(() => encodeTree([bad({ type: NodeType.blob, hash: new Uint8Array(5) })])).toThrow(
      ProtocolError,
    );
    expect(() => encodeTree([bad({ size: -1 })])).toThrow(ProtocolError);
  });
  it("referenced, blob info and tree result", () => {
    expect(() => decodeReferenced(unhex("0100000001"))).toThrow(ProtocolError);
    expect(() => decodeReferenced(unhex(`0100000001${"00".repeat(31)}`))).toThrow(ProtocolError);
    expect(() => encodeReferenced([new Uint8Array(5)])).toThrow(ProtocolError);
    expect(() => decodeBlobInfo(unhex("0102"))).toThrow(ProtocolError);
    expect(() => decodeTreeResult(unhex("01"))).toThrow(ProtocolError);
  });
});
