// SPDX-License-Identifier: MPL-2.0
/**
 * The CBOR validity profile of spec §9.2 and its deterministic encoder
 * (RFC 8949 §4.2.1, core deterministic). The receiver accepts every encoding the
 * profile admits, including non-preferred length widths (§9.2.1); the encoder
 * always emits the canonical form.
 */

export type CBORMap = { [key: string]: CBORValue };
export type CBORValue = null | boolean | number | string | Uint8Array | CBORValue[] | CBORMap;

/** Input type for encodeCBOR: accepts CBORValue and message-like objects with compatible fields. */
export type EncodableValue = CBORValue | { [key: string]: unknown } | unknown[];

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
        for (let k = 0; k < n; k++) {
          if (this.#i >= this.b.length) truncated();
          const byte = this.b[this.#i];
          if (byte === undefined) truncated();
          if (byte >> 5 !== 3) throw new InvalidCBOR("map key not a text string");
          const key = this.#item(depth + 1);
          if (typeof key !== "string") throw new InvalidCBOR("map key not a text string");
          if (Object.hasOwn(out, key)) throw new InvalidCBOR("duplicate map key");
          const value = this.#item(depth + 1);
          if (key === "__proto__") {
            Object.defineProperty(out, key, {
              value,
              enumerable: true,
              configurable: true,
              writable: true,
            });
          } else {
            out[key] = value;
          }
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

class Writer {
  #buf = new Uint8Array(256);
  #len = 0;

  get buffer(): Uint8Array {
    return this.#buf.subarray(0, this.#len);
  }

  #ensure(cap: number): void {
    if (this.#len + cap <= this.#buf.length) return;
    let newCap = this.#buf.length * 2;
    while (this.#len + cap > newCap) newCap *= 2;
    const newBuf = new Uint8Array(newCap);
    newBuf.set(this.#buf);
    this.#buf = newBuf;
  }

  #writeByte(byte: number): void {
    this.#ensure(1);
    this.#buf[this.#len++] = byte;
  }

  #writeBytes(bytes: Uint8Array): void {
    this.#ensure(bytes.length);
    this.#buf.set(bytes, this.#len);
    this.#len += bytes.length;
  }

  #writeHead(major: number, n: number): void {
    if (n < 24) {
      this.#writeByte((major << 5) | n);
      return;
    }
    if (n < 0x100) {
      this.#ensure(2);
      this.#buf[this.#len++] = (major << 5) | 24;
      this.#buf[this.#len++] = n;
      return;
    }
    if (n < 0x10000) {
      this.#ensure(3);
      this.#buf[this.#len++] = (major << 5) | 25;
      this.#buf[this.#len++] = (n >> 8) & 0xff;
      this.#buf[this.#len++] = n & 0xff;
      return;
    }
    if (n < 0x100000000) {
      this.#ensure(5);
      this.#buf[this.#len++] = (major << 5) | 26;
      this.#buf[this.#len++] = (n >>> 24) & 0xff;
      this.#buf[this.#len++] = (n >>> 16) & 0xff;
      this.#buf[this.#len++] = (n >>> 8) & 0xff;
      this.#buf[this.#len++] = n & 0xff;
      return;
    }
    this.#ensure(9);
    this.#buf[this.#len++] = (major << 5) | 27;
    const hi = Math.floor(n / 2 ** 32);
    const lo = n >>> 0;
    this.#buf[this.#len++] = (hi >>> 24) & 0xff;
    this.#buf[this.#len++] = (hi >>> 16) & 0xff;
    this.#buf[this.#len++] = (hi >>> 8) & 0xff;
    this.#buf[this.#len++] = hi & 0xff;
    this.#buf[this.#len++] = (lo >>> 24) & 0xff;
    this.#buf[this.#len++] = (lo >>> 16) & 0xff;
    this.#buf[this.#len++] = (lo >>> 8) & 0xff;
    this.#buf[this.#len++] = lo & 0xff;
  }

  writeNull(): void {
    this.#writeByte(0xf6);
  }

  writeBool(value: boolean): void {
    this.#writeByte(value ? 0xf5 : 0xf4);
  }

  writeInt(value: number): void {
    if (!Number.isSafeInteger(value) || value < 0) throw new InvalidCBOR("unsigned integer only");
    this.#writeHead(0, value);
  }

  writeString(value: string): void {
    const bytes = utf8.encode(value);
    this.#writeHead(3, bytes.length);
    this.#writeBytes(bytes);
  }

  writeBytes(value: Uint8Array): void {
    this.#writeHead(2, value.length);
    this.#writeBytes(value);
  }

  writeArray(value: unknown[]): void {
    this.#writeHead(4, value.length);
    for (const item of value) encodeInto(item, this);
  }

  writeMap(value: { [key: string]: unknown }): void {
    const keys = Object.keys(value);
    const encodedPairs: { key: Uint8Array; value: Uint8Array }[] = [];
    for (const key of keys) {
      const kw = new Writer();
      kw.writeString(key);
      const vw = new Writer();
      const val = value[key];
      if (val !== undefined) encodeInto(val, vw);
      encodedPairs.push({ key: kw.buffer, value: vw.buffer });
    }
    encodedPairs.sort((a, b) => compareBytes(a.key, b.key));
    this.#writeHead(5, encodedPairs.length);
    for (const pair of encodedPairs) {
      this.#writeBytes(pair.key);
      this.#writeBytes(pair.value);
    }
  }
}

function compareBytes(a: Uint8Array, b: Uint8Array): number {
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) {
    const d = (a[i] as number) - (b[i] as number);
    if (d !== 0) return d;
  }
  return a.length - b.length;
}

function encodeInto(value: unknown, out: Writer): void {
  if (value === null) {
    out.writeNull();
    return;
  }
  if (value === false) {
    out.writeBool(false);
    return;
  }
  if (value === true) {
    out.writeBool(true);
    return;
  }
  if (typeof value === "number") {
    out.writeInt(value);
    return;
  }
  if (typeof value === "string") {
    out.writeString(value);
    return;
  }
  if (value instanceof Uint8Array) {
    out.writeBytes(value);
    return;
  }
  if (Array.isArray(value)) {
    out.writeArray(value);
    return;
  }
  if (value !== null && typeof value === "object") {
    out.writeMap(value as { [key: string]: unknown });
    return;
  }
  throw new InvalidCBOR("unsupported value type");
}

/** Encodes one value in the core deterministic form (§4.2.1); no floats, tags or negatives. */
export function encodeCBOR(value: EncodableValue): Uint8Array {
  const w = new Writer();
  encodeInto(value, w);
  return w.buffer;
}
