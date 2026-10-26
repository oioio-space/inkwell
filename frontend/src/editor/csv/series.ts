import { parseNumber } from "./dialect";

const DAYS = ["lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi", "dimanche"];
const MONTHS = [
  "janvier",
  "février",
  "mars",
  "avril",
  "mai",
  "juin",
  "juillet",
  "août",
  "septembre",
  "octobre",
  "novembre",
  "décembre",
];

function matchCase(sample: string, word: string): string {
  if (sample === sample.toUpperCase() && sample.length > 1) return word.toUpperCase();
  if (sample[0] === (sample[0] as string).toUpperCase())
    return word[0]?.toUpperCase() + word.slice(1);
  return word;
}

const pad = (n: number, w: number) => String(n).padStart(w, "0");

interface DateForm {
  parse: (s: string) => number | undefined;
  show: (days: number) => string;
}

const DAY = 86_400_000;
const iso: DateForm = {
  parse(s) {
    const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s.trim());
    return m
      ? Date.UTC(+(m[1] as string), +(m[2] as string) - 1, +(m[3] as string)) / DAY
      : undefined;
  },
  show(d) {
    const x = new Date(d * DAY);
    return `${x.getUTCFullYear()}-${pad(x.getUTCMonth() + 1, 2)}-${pad(x.getUTCDate(), 2)}`;
  },
};
const fr: DateForm = {
  parse(s) {
    const m = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(s.trim());
    return m
      ? Date.UTC(+(m[3] as string), +(m[2] as string) - 1, +(m[1] as string)) / DAY
      : undefined;
  },
  show(d) {
    const x = new Date(d * DAY);
    return `${pad(x.getUTCDate(), 2)}/${pad(x.getUTCMonth() + 1, 2)}/${x.getUTCFullYear()}`;
  },
};

/** Today in French form (Ctrl+;). */
export function today(now = new Date()): string {
  return `${pad(now.getDate(), 2)}/${pad(now.getMonth() + 1, 2)}/${now.getFullYear()}`;
}

/** Digits of a number as written, to keep its number of decimals. */
function decimalsOf(s: string): number {
  const m = /[.,](\d+)\s*$/.exec(s);
  return m ? (m[1] as string).length : 0;
}

/**
 * The values that continue `seed` for `count` more cells, as the fill handle does: a linear
 * series of numbers or dates, a cycle of day or month names, a number at the end of a text
 * ("Article 1" -> "Article 2"), otherwise the seed repeated. `dir` -1 continues backwards
 * (dragging up or left).
 */
export function continueSeries(
  seed: readonly string[],
  count: number,
  dir: 1 | -1 = 1,
  decimal: "," | "." = ",",
): string[] {
  const out: string[] = [];
  if (seed.length === 0 || count <= 0) return out;
  const repeat = () =>
    Array.from({ length: count }, (_, i) => seed[ordinal(i, seed.length, dir)] as string);

  // Numbers: two or more with a constant step continue it; one is copied.
  const nums = seed.map((s) => parseNumber(s, decimal));
  if (nums.every((n) => n !== undefined)) {
    if (seed.length < 2) return repeat();
    const ns = nums as number[];
    const step = (ns[1] as number) - (ns[0] as number);
    if (ns.every((n, i) => i === 0 || Math.abs(n - (ns[i - 1] as number) - step) < 1e-9)) {
      const dp = Math.max(...seed.map(decimalsOf));
      const start = dir === 1 ? (ns[ns.length - 1] as number) : (ns[0] as number);
      for (let i = 1; i <= count; i++) {
        const v = start + step * i * dir;
        const s = v.toFixed(dp);
        out.push(decimal === "," ? s.replace(".", ",") : s);
      }
      return out;
    }
    return repeat();
  }

  // Dates.
  for (const form of [iso, fr]) {
    const ds = seed.map((s) => form.parse(s));
    if (ds.every((d) => d !== undefined)) {
      const arr = ds as number[];
      const step = arr.length > 1 ? (arr[1] as number) - (arr[0] as number) : 1;
      const start = dir === 1 ? (arr[arr.length - 1] as number) : (arr[0] as number);
      for (let i = 1; i <= count; i++) out.push(form.show(start + step * i * dir));
      return out;
    }
  }

  // Day and month names.
  for (const list of [DAYS, MONTHS]) {
    const idx = seed.map((s) => list.indexOf(s.toLowerCase()));
    if (idx.every((i) => i >= 0)) {
      const n = list.length;
      const step =
        idx.length > 1 ? ((((idx[1] as number) - (idx[0] as number)) % n) + n) % n || 1 : 1;
      const start = dir === 1 ? (idx[idx.length - 1] as number) : (idx[0] as number);
      const sample = seed[0] as string;
      for (let i = 1; i <= count; i++) {
        out.push(matchCase(sample, list[(((start + step * i * dir) % n) + n) % n] as string));
      }
      return out;
    }
  }

  // Text ending with a number.
  const tails = seed.map((s) => /^(.*?)(\d+)$/s.exec(s));
  if (tails.every((m) => m) && new Set(tails.map((m) => (m as RegExpExecArray)[1])).size === 1) {
    const ms = tails as RegExpExecArray[];
    const prefix = ms[0]?.[1] ?? "";
    const vals = ms.map((m) => Number(m[2]));
    const step = vals.length > 1 ? (vals[1] as number) - (vals[0] as number) : 1;
    const digits0 = ms[0]?.[2] ?? "";
    const width = digits0.startsWith("0") ? digits0.length : 0;
    const start = dir === 1 ? (vals[vals.length - 1] as number) : (vals[0] as number);
    for (let i = 1; i <= count; i++) {
      const v = start + step * i * dir;
      if (v < 0) return repeat();
      out.push(prefix + (width ? pad(v, width) : String(v)));
    }
    return out;
  }
  return repeat();
}

/** Which seed value the i-th filled cell copies (the seed cycles in the direction of the drag). */
function ordinal(i: number, n: number, dir: 1 | -1): number {
  return dir === 1 ? i % n : (((n - 1 - i) % n) + n) % n;
}
