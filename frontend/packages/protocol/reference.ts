// SPDX-License-Identifier: MPL-2.0
// Test-side reference for the extracted formats. Node crypto verifies signatures;
// the exported runner accepts any implementation, including browser implementations.
import { createHash, createPublicKey, verify } from "node:crypto";
import type { Vector } from "./src/conformance";
import { decodeEnvelope, encodeEnvelope } from "./src/envelope";

const hex = (s = "") => Buffer.from(s, "hex");
const hash = (b: Uint8Array) => createHash("sha256").update(b).digest("hex");
function assert(ok: unknown, why = "invalid encoding"): asserts ok {
  if (!ok) throw new Error(why);
}
const equal = (a: Uint8Array, b: Uint8Array) => Buffer.from(a).equals(Buffer.from(b));
const p = (1n << 255n) - 19n;
const mod = (x: bigint) => ((x % p) + p) % p;
function pow(x: bigint, n: bigint): bigint {
  let r = 1n;
  for (; n; n >>= 1n, x = mod(x * x)) if (n & 1n) r = mod(r * x);
  return r;
}
const d = mod(-121665n * pow(121666n, p - 2n));
// Extended Edwards coordinates avoid modular inversions in the subgroup check.
type Point = [bigint, bigint, bigint, bigint];
function add([x, y, z, t]: Point, [u, v, w, q]: Point): Point {
  const a = mod((y - x) * (v - u)),
    b = mod((y + x) * (v + u));
  const c = mod(2n * d * t * q),
    e = b - a;
  const f = mod(2n * z * w - c),
    g = mod(2n * z * w + c),
    h = b + a;
  return [mod(e * f), mod(g * h), mod(f * g), mod(e * h)];
}
const validKeys = new Set<string>();
function keyValid(b: Buffer) {
  if (validKeys.has(b.toString("hex"))) return;
  assert(b.length === 32);
  const sign = b.readUInt8(31) >> 7;
  const y = BigInt(`0x${Buffer.from(b).reverse().toString("hex")}`) & ((1n << 255n) - 1n);
  assert(y < p);
  const xx = mod((y * y - 1n) * pow(d * y * y + 1n, p - 2n));
  let x = pow(xx, (p + 3n) / 8n);
  if (mod(x * x) !== xx) x = mod(x * pow(2n, (p - 1n) / 4n));
  assert(mod(x * x) === xx);
  assert(x !== 0n || sign === 0);
  if (Number(x & 1n) !== sign) x = p - x;
  assert(x !== 0n || y !== 1n);
  let a: Point = [x, y, 1n, mod(x * y)],
    r: Point = [0n, 1n, 1n, 0n];
  for (let n = (1n << 252n) + 27742317777372353535851937790883648493n; n; n >>= 1n, a = add(a, a))
    if (n & 1n) r = add(r, a);
  assert(r[2] !== 0n && r[0] === 0n && r[1] === r[2]);
  validKeys.add(b.toString("hex"));
}
function signature(pub: Buffer, msg: Buffer, sig: Buffer) {
  assert(pub.length === 32);
  assert(
    verify(
      null,
      msg,
      createPublicKey({
        key: Buffer.concat([hex("302a300506032b6570032100"), pub]),
        format: "der",
        type: "spki",
      }),
      sig,
    ),
    "signature",
  );
}
class Reader {
  i = 0;
  constructor(readonly b: Buffer) {}
  take(n: number) {
    assert(n >= 0 && this.i + n <= this.b.length);
    const b = this.b.subarray(this.i, this.i + n);
    this.i += n;
    return b;
  }
  u8() {
    return this.take(1).readUInt8(0);
  }
  u16() {
    return this.take(2).readUInt16BE();
  }
  u32() {
    return this.take(4).readUInt32BE();
  }
  u64() {
    return this.take(8).readBigUInt64BE();
  }
  str(max: number) {
    const b = this.take(this.u16());
    assert(b.length <= max);
    return new TextDecoder("utf-8", { fatal: true }).decode(b);
  }
  done() {
    assert(this.i === this.b.length);
  }
}
function ordered(xs: string[]) {
  assert(xs.every((x, i) => i === 0 || (xs[i - 1] ?? "") < x));
}
interface Op {
  b: Buffer;
  h: string;
  kind: number;
  role: number;
  lamport: bigint;
  author: string;
  subject: string;
  prev: string[];
  body: Buffer;
  sig: Buffer;
  name: string;
  root: string;
  person: string;
}
function op(b: Buffer): Op {
  assert(b.length <= 4096);
  const r = new Reader(b);
  assert(r.u8() === 1);
  const kind = r.u8(),
    role = r.u8(),
    lamport = r.u64(),
    author = r.take(32),
    subject = r.take(32),
    person = r.take(32).toString("hex");
  keyValid(author);
  keyValid(subject);
  const name = r.str(128),
    root = r.str(512),
    n = r.u8();
  assert(n <= 64);
  const prev = Array.from({ length: n }, () => r.take(32).toString("hex"));
  ordered(prev);
  const body = b.subarray(0, r.i),
    sig = r.take(64);
  r.done();
  assert(kind >= 1 && kind <= 6);
  if (kind === 1)
    assert(equal(author, subject) && role === 3 && lamport === 0n && !n && /^0+$/.test(person));
  else {
    assert(root === "" && n > 0 && lamport > 0n);
    if (kind === 2 || kind === 3) assert(role >= 1 && role <= 3);
    if (kind !== 2) assert(/^0+$/.test(person));
    if (kind >= 4) assert(role === 0 && name === "");
    if (kind >= 5) assert(!equal(author, subject));
  }
  return {
    b,
    h: hash(b),
    kind,
    role,
    lamport,
    author: author.toString("hex"),
    subject: subject.toString("hex"),
    prev,
    body,
    sig,
    name,
    root,
    person,
  };
}
const domain = (d: string, b: Buffer) => Buffer.concat([Buffer.from(`inkwell/${d}/v1\0`), b]);
function verifyOp(o: Op, id: Buffer) {
  signature(
    hex(o.author),
    domain("trust-op", Buffer.concat([o.kind === 1 ? Buffer.alloc(32) : id, o.body])),
    o.sig,
  );
}
function bulk(b: Buffer) {
  const r = new Reader(b),
    kind = r.u8();
  r.take(32);
  if (kind === 1 || kind === 2) {
    const offset = r.u64(),
      n = r.u32();
    assert(n > 0 && n <= 1 << 20 && offset <= 1n << 40n && offset % 16384n === 0n);
    if (kind === 1) assert(n % 16384 === 0);
    else {
      const bao = r.take(b.length - r.i);
      assert(bao.length >= 8 + n && bao.length <= (1 << 20) + (16 << 10));
    }
  } else if (kind === 4) assert(r.u32() <= 60000);
  else assert(kind === 3);
  r.done();
}
function lobby(b: Buffer, reply: boolean) {
  const r = new Reader(b),
    kind = r.u8();
  const str8 = () => new TextDecoder("utf-8", { fatal: true }).decode(r.take(r.u8()));
  if (!reply) {
    assert(b.length <= 4096 && kind >= 1 && kind <= 3);
    if (kind === 2) str8();
    if (kind === 3) {
      r.take(32);
      r.take(16);
      str8();
    }
  } else {
    assert(b.length <= 262144);
    if (kind === 1) {
      str8();
      str8();
    } else if (kind === 2) {
      r.str(512);
      const n = r.u16();
      assert(n <= 1024);
      for (let i = 0; i < n; i++) op(r.take(r.u32()));
    } else if (kind === 3) {
      const reason = r.u8();
      assert(reason >= 1 && reason <= 4);
    } else assert(false);
  }
  r.done();
}
function fold(inputs: string[], id: Buffer) {
  const ops = inputs
    .map((x) => op(hex(x)))
    .sort((a, b) =>
      a.lamport < b.lamport ? -1 : a.lamport > b.lamport ? 1 : a.h.localeCompare(b.h),
    );
  for (const o of ops) verifyOp(o, id);
  const byHash = new Map(ops.map((o) => [o.h, o]));
  const ancestors = (o: Op): Set<string> => {
    const s = new Set<string>();
    const visit = (h: string) => {
      if (s.has(h)) return;
      s.add(h);
      for (const p of byHash.get(h)?.prev ?? []) visit(p);
    };
    for (const p of o.prev) visit(p);
    return s;
  };
  const past = new Map(ops.map((o) => [o.h, ancestors(o)]));
  const ranks = new Map<string, number>();
  for (const o of ops)
    if ((o.kind === 1 || o.kind === 2) && !ranks.has(o.subject)) ranks.set(o.subject, ranks.size);
  let live = new Set(ops.map((o) => o.h));
  for (let iteration = 0; iteration < ops.length; iteration++) {
    const members = new Map<string, { public_hex: string; role: number; suspended: boolean }>();
    const cuts: Op[] = [];
    for (const o of ops) {
      if (!live.has(o.h)) continue;
      const author = members.get(o.author);
      if (o.kind !== 1 && author?.role !== 3) continue;
      const target = members.get(o.subject);
      if (
        o.kind >= 3 &&
        target?.role === 3 &&
        (ranks.get(o.subject) ?? 0) < (ranks.get(o.author) ?? 0) &&
        o.subject !== o.author
      )
        continue;
      if (
        (o.kind === 4 || (o.kind === 3 && o.role !== 3)) &&
        o.author === o.subject &&
        [...members.values()].filter((m) => m.role === 3).length === 1
      )
        continue;
      if (o.kind === 1 || o.kind === 2)
        members.set(o.subject, { public_hex: o.subject, role: o.role, suspended: false });
      if (o.kind === 3 && target) target.role = o.role;
      if (o.kind === 4) {
        members.delete(o.subject);
        cuts.push(o);
      }
      if (o.kind === 5 && target) {
        target.suspended = true;
        cuts.push(o);
      }
      if (o.kind === 6 && target) target.suspended = false;
    }
    const next = new Set(
      ops
        .filter(
          (o) =>
            !cuts.some(
              (c) =>
                c.author !== c.subject &&
                c.subject === o.author &&
                o.kind !== 1 &&
                !past.get(c.h)?.has(o.h) &&
                !past.get(o.h)?.has(c.h),
            ),
        )
        .map((o) => o.h),
    );
    if ([...next].every((h) => live.has(h)) && next.size === live.size)
      return [...members.values()];
    live = next;
  }
  throw new Error("fold did not stabilize");
}
export function reference(v: Vector, corpus: Vector[]) {
  const b = hex(v.input_hex),
    e = v.expect ?? {};
  const genesis = corpus.find((v) => v.id === "op-genesis");
  assert(genesis, "missing genesis fixture");
  const id = hex(hash(hex(genesis.input_hex)));
  let accepted = true;
  try {
    switch (v.decoder) {
      case "bridge_envelope":
        assert(equal(encodeEnvelope(decodeEnvelope(b)), b));
        break;
      case "bulk":
        bulk(b);
        break;
      case "trust_op":
      case "trust_op+apply": {
        const o = op(b);
        if (e.hash_hex) assert(o.h === e.hash_hex);
        if (v.decoder.endsWith("apply")) verifyOp(o, id);
        break;
      }
      case "trust_sync": {
        const r = new Reader(b);
        const kind = r.u8(),
          n = r.u16();
        if (kind === 1) {
          assert(n <= 256);
          r.take(n * 32);
        } else {
          assert(kind === 2 && b.length <= 65536);
          for (let i = 0; i < n; i++) op(r.take(r.u16()));
        }
        r.done();
        break;
      }
      case "attestation":
      case "attestation+verify": {
        assert(b.length <= 4096);
        const r = new Reader(b);
        assert(r.u8() === 1);
        const project = r.take(32),
          pub = r.take(32);
        keyValid(pub);
        r.u64();
        assert(r.str(256) !== "");
        const n = r.u16();
        assert(n <= 64);
        ordered(Array.from({ length: n }, () => r.take(32).toString("hex")));
        const body = b.subarray(0, r.i),
          sig = r.take(64);
        r.done();
        if (v.decoder.endsWith("verify")) {
          assert(equal(project, id));
          signature(pub, domain("attest", body), sig);
        }
        break;
      }
      case "lobby_request":
        lobby(b, false);
        break;
      case "lobby_reply":
        lobby(b, true);
        break;
      case "ticket_text": {
        const text = String(e.text);
        assert(text.startsWith("inkwell1:") && text.length <= 1024);
        let bits = 0,
          n = 0;
        const raw: number[] = [];
        for (const c of text.slice(9)) {
          const x = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567".indexOf(c);
          assert(x >= 0);
          n = (n << 5) | x;
          bits += 5;
          if (bits >= 8) {
            bits -= 8;
            raw.push((n >> bits) & 255);
          }
        }
        const r = new Reader(Buffer.from(raw));
        assert(r.u8() === 1);
        const flags = r.u8();
        assert(flags <= 1);
        assert([1, 2, 3].includes(r.u8()));
        r.take(32);
        if (flags) r.take(32);
        assert(r.take(16).some((x) => x));
        r.u64();
        const host = r.take(r.u8());
        assert(host.length > 0 && host.length <= 64);
        const count = r.u8();
        assert(count <= 16);
        for (let i = 0; i < count; i++) assert(r.take(r.u8())[0] === 47);
        r.done();
        break;
      }
      case "public_key": {
        for (const k of (e.keys_hex as string[] | undefined) ?? [v.input_hex ?? ""]) {
          let valid = true;
          try {
            keyValid(hex(k));
          } catch {
            valid = false;
          }
          assert(valid === (v.verdict === "accept"), "public key verdict");
        }
        return;
      }
      case "peer_id": {
        let n = 0n;
        const s = String(e.peer_id);
        for (const c of s) {
          const digit = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz".indexOf(c);
          assert(digit >= 0);
          n = n * 58n + BigInt(digit);
        }
        let raw = n.toString(16);
        if (raw.length % 2) raw = `0${raw}`;
        const decoded = Buffer.concat([Buffer.alloc(s.match(/^1*/)?.[0].length ?? 0), hex(raw)]);
        assert(equal(decoded.subarray(0, 6), hex("002408011220")));
        keyValid(decoded.subarray(6));
        break;
      }
      case "signature":
      case "signature+key":
      case undefined: {
        const pub = hex(String(e.public_hex));
        if (v.decoder === "signature+key") keyValid(pub);
        signature(
          pub,
          v.decoder === undefined
            ? domain(String(e.domain), hex(String(e.message_hex)))
            : hex(String(e.signed_bytes_hex)),
          hex(String(e.signature_hex)),
        );
        break;
      }
      case "trust_fold": {
        const got = fold(v.inputs_hex ?? [], id);
        assert(
          JSON.stringify(got) === JSON.stringify(e.members),
          `fold mismatch ${JSON.stringify(got)}`,
        );
        assert(
          JSON.stringify(fold([...(v.inputs_hex ?? [])].reverse(), id)) ===
            JSON.stringify(e.members),
        );
        break;
      }
      default:
        throw new Error(`unsupported: ${v.decoder}`);
    }
  } catch (error) {
    if (String(error).includes("unsupported:")) throw error;
    accepted = false;
    if (v.verdict === "accept") throw error;
  }
  assert(accepted === (v.verdict === "accept"), "verdict mismatch");
}
