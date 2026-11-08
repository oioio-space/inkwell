// SPDX-License-Identifier: MPL-2.0
/**
 * The CBOR validity profile of spec §9.2 and its deterministic encoder
 * (RFC 8949 §4.2.1, core deterministic). The receiver accepts every encoding the
 * profile admits, including non-preferred length widths (§9.2.1); the encoder
 * always emits the canonical form.
 */

export type CBORMap = { [key: string]: CBORValue };
export type CBORValue = null | boolean | number | string | Uint8Array | CBORValue[] | CBORMap;

export const CBOR_MAX_DEPTH = 8;
export const CBOR_MAX_ITEMS = 4096;
export const CBOR_MAX_MAP = 64;
export const CBOR_MAX_ARRAY = 256;
export const CBOR_MAX_INT = Number.MAX_SAFE_INTEGER;

const utf8 = new TextEncoder();
const strictUtf8 = new TextDecoder("utf-8", { fatal: true });

export class InvalidCBOR extends Error {
  override name = "InvalidCBOR";
}

function truncated(): never {
  throw new InvalidCBOR("truncated");
}

class Reader {
  #i = 0;
  #items = 0;
  constructor(private readonly b: Uint8Array) {}

  #arg(ai: number): number {
    if (ai < 24) return ai;
    const width = ai === 24 ? 1 : ai === 25 ? 2 : ai === 26 ? 4 : ai === 27 ? 8 : 0;
    if (width === 0) {
      throw new InvalidCBOR(ai === 31 ? "indefinite length" : "reserved additional info");
    }
    if (this.#i + width > this.b.length) truncated();
    let value = 0;
    for (let k = 0; k < width; k++) value = value * 256 + (this.b[this.#i + k] as number);
    this.#i += width;
    if (value > CBOR_MAX_INT) throw new InvalidCBOR("integer above 2^53-1");
    return value;
  }

  #item(depth: number): CBORValue {
    if (depth > CBOR_MAX_DEPTH) throw new InvalidCBOR("depth > 8");
    if (++this.#items > CBOR_MAX_ITEMS) throw new InvalidCBOR("more than 4096 items");
    if (this.#i >= this.b.length) truncated();
    const initial = this.b[this.#i++] as number;
    const major = initial >> 5;
    const ai = initial & 31;
    switch (major) {
      case 0:
        return this.#arg(ai);
      case 1:
        throw new InvalidCBOR("negative integer");
      case 2:
      case 3: {
        const n = this.#arg(ai);
        if (this.#i + n > this.b.length) truncated();
        const bytes = this.b.subarray(this.#i, this.#i + n);
        this.#i += n;
        if (major === 2) return bytes;
        let text: string;
        try {
          text = strictUtf8.decode(bytes);
        } catch {
          throw new InvalidCBOR("The encoded data was not valid for encoding utf-8");
        }
        return text;
      }
      case 4: {
        const n = this.#arg(ai);
        if (n > CBOR_MAX_ARRAY) throw new InvalidCBOR("array > 256");
        const out: CBORValue[] = [];
        for (let k = 0; k < n; k++) out.push(this.#item(depth + 1));
        return out;
      }
      case 5: {
        const n = this.#arg(ai);
        if (n > CBOR_MAX_MAP) throw new InvalidCBOR("map > 64 pairs");
        const out: CBORMap = {};
        const seen = new Set<string>();
        for (let k = 0; k < n; k++) {
          if (this.#i >= this.b.length) truncated();
          const byte = this.b[this.#i];
          if (byte === undefined) truncated();
          if (byte >> 5 !== 3) throw new InvalidCBOR("map key not a text string");
          const key = this.#item(depth + 1);
          if (typeof key !== "string") throw new InvalidCBOR("map key not a text string");
          if (seen.has(key)) throw new InvalidCBOR("duplicate map key");
          seen.add(key);
          const value = this.#item(depth + 1);
          Object.defineProperty(out, key, {
            value,
            enumerable: true,
            configurable: true,
            writable: true,
          });
        }
        return out;
      }
      case 6:
        throw new InvalidCBOR("tag");
      default:
        if (initial === 0xf4) return false;
        if (initial === 0xf5) return true;
        if (initial === 0xf6) return null;
        throw new InvalidCBOR(
          ai >= 25 && ai <= 27 ? "float" : "simple value other than false/true/null",
        );
    }
  }

  read(): CBORValue {
    const value = this.#item(1);
    if (this.#i !== this.b.length) throw new InvalidCBOR("trailing bytes");
    return value;
  }
}

/** Decodes one item under the validity profile (§9.2.1). */
export function decodeCBOR(b: Uint8Array): CBORValue {
  return new Reader(b).read();
}

function head(major: number, n: number): number[] {
  if (n < 24) return [(major << 5) | n];
  if (n < 0x100) return [(major << 5) | 24, n];
  if (n < 0x10000) return [(major << 5) | 25, n >> 8, n & 0xff];
  if (n < 0x100000000) {
    return [(major << 5) | 26, (n >>> 24) & 0xff, (n >>> 16) & 0xff, (n >>> 8) & 0xff, n & 0xff];
  }
  const out = [(major << 5) | 27];
  const x = BigInt(n);
  for (let k = 7; k >= 0; k--) out.push(Number((x >> BigInt(8 * k)) & 255n));
  return out;
}

function compareBytes(a: Uint8Array, b: Uint8Array): number {
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) {
    const d = (a[i] as number) - (b[i] as number);
    if (d !== 0) return d;
  }
  return a.length - b.length;
}

function encodeInto(value: CBORValue, out: number[]): void {
  if (value === null) {
    out.push(0xf6);
    return;
  }
  if (value === false) {
    out.push(0xf4);
    return;
  }
  if (value === true) {
    out.push(0xf5);
    return;
  }
  if (typeof value === "number") {
    if (!Number.isSafeInteger(value) || value < 0) throw new InvalidCBOR("unsigned integer only");
    out.push(...head(0, value));
    return;
  }
  if (typeof value === "string") {
    const bytes = utf8.encode(value);
    out.push(...head(3, bytes.length), ...bytes);
    return;
  }
  if (value instanceof Uint8Array) {
    out.push(...head(2, value.length), ...value);
    return;
  }
  if (Array.isArray(value)) {
    out.push(...head(4, value.length));
    for (const item of value) encodeInto(item, out);
    return;
  }
  const pairs = Object.keys(value)
    .map((key) => ({ key, encoded: encodeCBOR(key), value: encodeCBOR(value[key] as CBORValue) }))
    .sort((a, b) => compareBytes(a.encoded, b.encoded));
  out.push(...head(5, pairs.length));
  for (const pair of pairs) {
    out.push(...pair.encoded, ...pair.value);
  }
}

/** Encodes one value in the core deterministic form (§4.2.1); no floats, tags or negatives. */
export function encodeCBOR(value: CBORValue): Uint8Array {
  const out: number[] = [];
  encodeInto(value, out);
  return Uint8Array.from(out);
}
