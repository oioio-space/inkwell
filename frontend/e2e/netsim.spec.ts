import { execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { docText, joinAs, open, pair, pasteAtEnd, typeAtEnd } from "./helpers";

// Driven by scripts/netsim/run.sh, which builds the network (two homes behind NATs and a
// public relay, in network namespaces), starts the instances inside it and runs the
// scenario named by NETSIM_SCENARIO. The browser reaches the instances only through unix
// sockets bridged to the loopback; the instances reach each other only through the NATs.

const scenario = process.env.NETSIM_SCENARIO ?? "0";
const topology = process.env.NETSIM_TOPOLOGY ?? "";
const out = process.env.NETSIM_OUT ?? ".";

const topo = (...args: string[]) => execFileSync(topology, args, { encoding: "utf8" }).trim();
const packets = (nat: string, counter: "direct" | "egress") =>
  Number(topo("counter", nat, counter));
/** Packets that crossed a hole punched between two NATs, all NATs together. */
const directPackets = () =>
  ["nat-a", "nat-b", "nat-c"].reduce((n, nat) => n + packets(nat, "direct"), 0);
// The impaired scenarios (5-8) run once per seed (and scenario 8 once per variant): `run` names the
// run's files, `seed` drives netem, the typing schedule and the payloads.
const run = process.env.NETSIM_RUN ?? scenario;
const seed = Number(process.env.NETSIM_SEED ?? "0") || 0;
const variant = process.env.NETSIM_VARIANT ?? "";
const strict = process.env.NETSIM_EXPECT === "strict";
const note = (text: string) => writeFileSync(join(out, `${run}.txt`), text);

async function twoPeople(browser: Browser) {
  const alice = await open(browser, "http://127.0.0.1:18081/");
  const bob = await open(browser, "http://127.0.0.1:18082/", true);
  await pair(alice, bob);
  return { alice, bob };
}

/** Both documents contain every given text and are identical. */
async function converge(a: Page, b: Page, texts: string[], timeout: number) {
  for (const p of [a, b]) {
    for (const text of texts) {
      await expect(p.locator(".cm-content")).toContainText(text, { timeout });
    }
  }
  await expect.poll(() => docText(a), { timeout }).toBe(await docText(b));
}

test("scenario 1: cone NAT, direct link after hole punching", async ({ browser }) => {
  const { alice, bob } = await twoPeople(browser);
  const t0 = Date.now();
  await expect
    .poll(
      async () => {
        await typeAtEnd(alice, "a");
        return directPackets();
      },
      { timeout: 90_000, intervals: [1_000], message: "no direct traffic between the two NATs" },
    )
    .toBeGreaterThan(0);
  const secs = Math.round((Date.now() - t0) / 1000);
  await typeAtEnd(bob, " bob");
  await converge(alice, bob, ["bob"], 30_000);
  note(`direct ${secs} s after both synced, ${directPackets()} packets across the NATs`);
});

test("scenario 2: symmetric NAT, the link stays relayed", async ({ browser }) => {
  const { alice, bob } = await twoPeople(browser);
  // Long enough for hole punching to try, fail and retry.
  await alice.waitForTimeout(45_000);
  await typeAtEnd(alice, "via le relais ");
  await typeAtEnd(bob, "bob aussi");
  await converge(alice, bob, ["via le relais", "bob aussi"], 30_000);
  expect(directPackets(), "packets crossed between the NATs").toBe(0);
  note("relayed only, 0 packets across the NATs, text converged");
});

test("scenario 3: alice offline for 60 s while both type", async ({ browser }) => {
  const { alice, bob } = await twoPeople(browser);
  await typeAtEnd(alice, "avant ");
  await expect(bob.locator(".cm-content")).toContainText("avant");

  topo("alice-detach");
  await typeAtEnd(alice, "alice-seule ");
  await typeAtEnd(bob, "bob-ici ");
  await alice.waitForTimeout(60_000);
  expect(await docText(alice)).not.toContain("bob-ici");
  topo("alice-attach", "nat-a");

  const t0 = Date.now();
  await converge(alice, bob, ["alice-seule", "bob-ici"], 60_000);
  note(`converged ${Math.round((Date.now() - t0) / 1000)} s after alice came back`);
});

test("scenario 4: alice moves from nat-a to nat-c", async ({ browser }) => {
  const { alice, bob } = await twoPeople(browser);
  await typeAtEnd(alice, "avant ");
  await expect(bob.locator(".cm-content")).toContainText("avant");
  const viaC = packets("nat-c", "egress");

  topo("alice-detach");
  await typeAtEnd(alice, "alice-en-route ");
  await typeAtEnd(bob, "bob-ici ");
  await alice.waitForTimeout(5_000);
  topo("alice-attach", "nat-c");

  const t0 = Date.now();
  await converge(alice, bob, ["alice-en-route", "bob-ici"], 90_000);
  expect(packets("nat-c", "egress"), "alice's traffic through nat-c").toBeGreaterThan(viaC);
  note(
    `new public address 11.0.0.4, converged ${Math.round((Date.now() - t0) / 1000)} s after the move`,
  );
});

// ─── Impaired network (docs/plans/2026-11-08-netsim-impaired-plan.md, tests NI-1..NI-4) ───

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/** Mulberry32: a small seeded generator, so the typing schedule and the payloads replay. */
function rng(salt: number) {
  let a = (seed * 7919 + salt * 104729 + 1) >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** `n` seeded characters from U+4E00..U+5DFF: 12 bits each, so the text does not compress away. */
function cjk(n: number, random: () => number) {
  let text = "";
  for (let i = 0; i < n; i += 4096) {
    const chunk = Math.min(4096, n - i);
    text += String.fromCharCode(
      ...Array.from({ length: chunk }, () => 0x4e00 + Math.floor(random() * 4096)),
    );
  }
  return text;
}

// Clauses. A scenario records one verdict per clause of the plan (C1, C2, ...), keeps going when
// one is red, and writes them all to results/<run>.json before failing the test: a red clause
// never hides the others. Scenario 7 carries declared known failures (decisions Q2 and
// 2026-11-09, until ADR 0034 step 6): they are executed and reported on every run, but do not
// block, unless NETSIM_EXPECT=strict. Only a measured miss is downgraded: a harness error stays
// red. A known failure that passes is reported as XPASS, the signal to flip it to strict.
// Scenario 6 C2 was flipped to strict on 2026-11-09: rule L (block RL1) closed the silent loss and
// it passed on every run since (seeds 1-3 and a random seed).
const KNOWN_FAILURES: Record<string, Record<string, string>> = {
  "7": {
    C1: "N-15 (total write budget 20 s + size / 16 KiB/s on sync/1), closed by ADR 0034 step 6",
    C2: "head-of-line behind a paste larger than the write budget on a link under 16 KiB/s: each failed write resyncs (rule L) and resends it; worst 44-49 s measured vs 19-21 s before RL1 (2026-11-09); closed by ADR 0034 step 6 (separate lane for large objects), mitigated by block WB (write budget follows measured throughput)",
  },
};

type Verdict = "pass" | "fail" | "known-failure" | "blocked";
interface ClauseResult {
  id: string;
  title: string;
  bound: string;
  status: Verdict;
  measured: string;
  known?: string;
  xpass?: boolean;
}
const clauses: ClauseResult[] = [];

/** The link metrics (plan section 7, `window.__inkwellLinks`) are not exposed by this build. */
class MetricsMissing extends Error {}

async function clause(
  id: string,
  title: string,
  bound: string,
  check: () => Promise<{ ok: boolean; measured: string }>,
) {
  let status: Verdict;
  let measured: string;
  let missed = false; // a measured miss, as opposed to a harness error or a blocked clause
  try {
    const verdict = await check();
    status = verdict.ok ? "pass" : "fail";
    missed = !verdict.ok;
    measured = verdict.measured;
  } catch (e) {
    status = e instanceof MetricsMissing ? "blocked" : "fail";
    measured = e instanceof MetricsMissing ? e.message : `error: ${String(e).slice(0, 300)}`;
  }
  const known = KNOWN_FAILURES[scenario]?.[id];
  if (known && !strict && missed) status = "known-failure";
  const xpass = Boolean(known) && status === "pass";
  clauses.push({
    id,
    title,
    bound,
    status,
    measured,
    ...(known ? { known } : {}),
    ...(xpass ? { xpass } : {}),
  });
  console.log(`netsim ${run} ${id} ${status}: ${title} [${bound}] measured ${measured}`);
  if (xpass) console.log(`netsim ${run} ${id} XPASS: known failure passes, flip it to strict`);
}

/** Writes results/<run>.json and fails the test when a clause is red (a known failure is not). */
function finish(extra: Record<string, unknown> = {}) {
  writeFileSync(
    join(out, `${run}.json`),
    JSON.stringify(
      {
        scenario,
        run,
        variant,
        seed,
        expect: strict ? "strict" : "characterize",
        ...extra,
        clauses,
      },
      null,
      2,
    ),
  );
  note(clauses.map((c) => `${c.id} ${c.status}${c.xpass ? " (XPASS)" : ""}`).join(", "));
  const red = clauses.filter((c) => c.status === "fail" || c.status === "blocked");
  expect(
    red.map((c) => `${c.id} ${c.status}: ${c.title} (${c.measured})`),
    "red clauses",
  ).toEqual([]);
}

interface Link {
  peer?: string;
  path?: string;
  rttMs?: number | null;
  lossPct?: number | null;
  downsTotal?: number;
  syncSent?: number;
  [key: string]: unknown;
}

/** The link list the people popover reads: every person this instance is connected to. */
async function links(p: Page): Promise<Link[]> {
  const value = await p.evaluate(() => {
    const f = (globalThis as { __inkwellLinks?: () => unknown }).__inkwellLinks;
    return typeof f === "function" ? { value: f() } : undefined;
  });
  if (value === undefined)
    throw new MetricsMissing(
      "window.__inkwellLinks is not exposed (link metrics contract, plan section 7)",
    );
  const v = value.value as unknown;
  if (Array.isArray(v)) return v as Link[];
  const inner = (v as { links?: unknown } | null)?.links ?? v;
  if (Array.isArray(inner)) return inner as Link[];
  return Object.values((inner ?? {}) as Record<string, Link>);
}

const only = async (p: Page): Promise<Link> => {
  const ls = await links(p);
  if (ls.length !== 1) throw new Error(`expected 1 link, got ${ls.length}: ${JSON.stringify(ls)}`);
  return ls[0];
};
/**
 * Polls the link list until `ok` holds for exactly one link, for at most `timeoutMs` (the bound of
 * the clause that waits, never longer). A miss comes back as data (`count`, `link` of the last
 * read); a harness error (metrics missing, page gone) propagates.
 */
async function awaitOnly(
  p: Page,
  timeoutMs: number,
  ok: (l: Link) => boolean = () => true,
  everyMs = 1000,
): Promise<{ ms?: number; link?: Link; count: number }> {
  const t0 = Date.now();
  let last: { link?: Link; count: number } = { count: -1 };
  for (;;) {
    const ls = await links(p);
    last = { link: ls.length === 1 ? ls[0] : undefined, count: ls.length };
    if (last.link && ok(last.link)) return { ...last, ms: Date.now() - t0 };
    if (Date.now() - t0 + everyMs > timeoutMs) return last;
    await sleep(everyMs);
  }
}
const field = (l: Link, key: string): number | undefined =>
  typeof l[key] === "number" ? (l[key] as number) : undefined;
const isDirect = (l: Link) => l.path === "direct" || l.path === "lan";

/** Qdisc statistics of one profile, as {ns, dev, bytes, sent, dropped, overlimits} rows. */
function qstats(profile: string) {
  return topo("qstats", profile)
    .split("\n")
    .filter(Boolean)
    .map((line) => {
      const [ns, dev, ...kv] = line.split(" ");
      const stat = Object.fromEntries(kv.map((x) => x.split("=")).map(([k, v]) => [k, Number(v)]));
      return {
        ns,
        dev,
        bytes: stat.bytes ?? 0,
        sent: stat.sent ?? 0,
        dropped: stat.dropped ?? 0,
        overlimits: stat.overlimits ?? 0,
      };
    });
}

/**
 * Types seeded unique tokens (`a001x `) at the end of a document, one per `periodMs` (+-20 %),
 * until `count` are written or `stop()`. The history (tokens and when each was written) is what
 * the checker reads afterwards, as in Jepsen: nothing is judged from what the screen happens to
 * show. A token is one atomic edit: two people typing keystrokes at the same place interleave
 * their characters (that is how a text CRDT merges), which would split the tokens.
 */
function typist(
  page: Page,
  prefix: string,
  periodMs: number,
  salt: number,
  opts: { count?: number } = {},
) {
  const random = rng(salt); // one generator per typist: the seed replays each schedule
  const tokens: string[] = [];
  const times: number[] = [];
  let stopped = false;
  const done = (async () => {
    while (!stopped && tokens.length < (opts.count ?? Number.POSITIVE_INFINITY)) {
      const token = `${prefix}${String(tokens.length + 1).padStart(3, "0")}x `;
      await pasteAtEnd(page, token);
      tokens.push(token);
      times.push(Date.now());
      await sleep(periodMs * (0.8 + 0.4 * random()));
    }
  })();
  return {
    tokens,
    times,
    done,
    stop: async () => {
      stopped = true;
      await done;
    },
  };
}

/** Every token of `all` is in `text` exactly once; returns the missing and the duplicated. */
function check(text: string, all: string[]) {
  const missing: string[] = [];
  const duplicated: string[] = [];
  for (const token of all) {
    const n = text.split(token).length - 1;
    if (n === 0) missing.push(token);
    else if (n > 1) duplicated.push(token);
  }
  return { missing, duplicated };
}

/** Polls `step` every `everyMs` until it returns true; the time it took, or undefined at the deadline. */
async function until(step: () => Promise<boolean>, timeoutMs: number, everyMs = 1000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    if (
      await step().catch((e) => {
        if (e instanceof MetricsMissing) throw e;
        return false;
      })
    )
      return Date.now() - t0;
    await sleep(everyMs);
  }
  return undefined;
}

/**
 * Page-side checks. A poll that shipped a 100-300 KB document over CDP every second would be slow
 * and would load the machine under test: the needles are stored in the page once (`stash`) and
 * `probe` returns the length, a hash and, per needle, whether the document contains it.
 */
const stash = (p: Page, key: string, needles: string[]) =>
  p.evaluate(
    ([k, n]) => {
      (globalThis as Record<string, unknown>)[`__ns_${k}`] = n;
    },
    [key, needles] as const,
  );
const probe = (p: Page, key: string) =>
  p.evaluate((k) => {
    const g = globalThis as { __inkwellDocText?: () => string | undefined } & Record<
      string,
      unknown
    >;
    const raw = g.__inkwellDocText?.();
    if (raw === undefined)
      throw new Error("probe: no __inkwellDocText (build with VITE_E2E_OBSERVE=1)");
    const text = raw.replace(/[\u200b\u2060\n]/g, "");
    let hash = 2166136261; // FNV-1a
    for (let i = 0; i < text.length; i++)
      hash = Math.imul(hash ^ text.charCodeAt(i), 16777619) >>> 0;
    const needles = (g[`__ns_${k}`] as string[] | undefined) ?? [];
    return { len: text.length, hash, has: needles.map((n) => text.includes(n)) };
  }, key);

/** What the two outage clauses compare: the counters of every page's link before and after. */
type Snapshot = {
  rows?: { downs?: number; resyncs?: number }[];
  /** A measured miss: the peer is not an active member (one link) within the wait. */
  missing?: string;
  /** A harness error. */
  error?: unknown;
};
/** Reads each page's link counters, waiting up to `waitMs` (the clause's own bound) for the link to exist. */
async function snapshot(pages: Page[], waitMs = 0): Promise<Snapshot> {
  try {
    const reads = await Promise.all(pages.map((p) => awaitOnly(p, waitMs)));
    const gone = reads.find((r) => !r.link);
    if (gone) return { missing: `expected 1 link, got ${gone.count} (waited ${secs(waitMs)})` };
    return {
      rows: reads.map(({ link: l }) => ({
        downs: field(l as Link, "downsTotal"),
        resyncs: field(l as Link, "resyncs"),
      })),
    };
  } catch (error) {
    return { error };
  }
}
/**
 * Rule L: the transport connection is not torn down. `downsTotal` counts real connection losses
 * only, so it must not rise; `resyncs` (a failed send re-synchronised on a surviving connection)
 * is expected to rise and is only reported.
 */
function notTornDown(before: Snapshot, after: Snapshot) {
  for (const s of [before, after]) if (s.error) throw s.error;
  const gone = [before, after].find((s) => s.missing);
  if (gone) return { ok: false, measured: gone.missing as string };
  const b = before.rows ?? [];
  const a = after.rows ?? [];
  const newDowns = a.map((r, i) => (r.downs ?? Number.NaN) - (b[i]?.downs ?? Number.NaN));
  const resyncs = a.map((r, i) =>
    r.resyncs === undefined || b[i]?.resyncs === undefined
      ? "n/a"
      : String(r.resyncs - (b[i].resyncs ?? 0)),
  );
  return {
    ok: newDowns.every((d) => d === 0),
    measured: `new real outages (downsTotal) ${newDowns.join(", ")}; resyncs ${resyncs.join(", ")} (expected to rise)`,
  };
}

/** Both pages hold the same text, and every token exactly once. */
async function consistent(pages: Page[], all: string[]) {
  const texts = await Promise.all(pages.map(docText));
  if (texts.some((t) => t !== texts[0])) return false;
  const { missing, duplicated } = check(texts[0], all);
  return missing.length === 0 && duplicated.length === 0;
}

const secs = (ms: number | undefined) =>
  ms === undefined ? "never" : `${(ms / 1000).toFixed(1)} s`;
const range = (v: number | null | undefined, lo: number, hi: number) =>
  typeof v === "number" && v >= lo && v <= hi;

test("scenario 5: NI-1 bad Wi-Fi that changes (converge, stay up, report the truth)", async ({
  browser,
}) => {
  test.setTimeout(480_000);
  const PHASE = 40_000;
  // The profile is on before the pair meets, so hole punching itself runs under jitter (R6).
  topo("profile", "wifi");
  const { alice, bob } = await twoPeople(browser);
  const tPair = Date.now();

  const a = typist(alice, "a", 500, 51);
  const b = typist(bob, "b", 500, 52);

  // C1: direct within 90 s of pairing, seen by the NAT counter and, when exposed, by the metrics.
  const c1Run = (async () => {
    const t = await until(async () => directPackets() > 0, 90_000);
    let path = "metrics not read";
    if (t !== undefined) {
      try {
        // The link may not exist yet (the peer is not an active member): poll within what is
        // left of the 90 s bound, a miss is reported as measured.
        const r = await awaitOnly(alice, Math.max(0, 90_000 - t), isDirect);
        path = r.link ? `path ${r.link.path}` : `${r.count} links`;
        if (!r.link || !isDirect(r.link))
          return { ok: false, measured: `${secs(t)}, but ${path} within 90 s` };
      } catch (e) {
        if (!(e instanceof MetricsMissing)) throw e;
        return { ok: true, measured: `${secs(t)} (counter only: ${e.message})` };
      }
    }
    return {
      ok: t !== undefined,
      measured: `${secs(t)} after pairing (${secs(Date.now() - tPair)} elapsed), ${path}`,
    };
  })();
  const c1 = c1Run.catch((e) => ({ ok: false, measured: `error: ${String(e).slice(0, 300)}` }));
  // The P1 timer starts once C1 has seen the direct path (or given up at 90 s): a relayed link
  // reports a relayed RTT, which is not what C4 judges.
  await c1;

  // Phases: wifi, wifi-far, wifi-clean. Metrics are sampled 20 s and 40 s into each; C4 and C5 judge
  // the end of each phase (the last sample), once the estimators have had time to follow the change.
  const truth = { P1: 60, P2: 300, P3: 60 };
  const samples: Record<
    string,
    { rtt: (number | null | undefined)[]; loss: (number | null | undefined)[] }
  > = {};
  let metricsMissing: MetricsMissing | undefined;
  let p1Stats: ReturnType<typeof qstats> = [];
  for (const [phase, profile] of [
    ["P1", "wifi"],
    ["P2", "wifi-far"],
    ["P3", "wifi-clean"],
  ] as const) {
    if (phase !== "P1") topo("profile", profile);
    samples[phase] = { rtt: [], loss: [] };
    for (let half = 0; half < 2; half++) {
      await sleep(PHASE / 2);
      try {
        for (const page of [alice, bob]) {
          const l = await only(page);
          samples[phase].rtt.push(field(l, "rttMs"));
          samples[phase].loss.push(field(l, "lossPct"));
        }
      } catch (e) {
        if (!(e instanceof MetricsMissing)) throw e;
        metricsMissing = e;
      }
    }
    if (phase === "P1") p1Stats = qstats("wifi");
  }
  await Promise.all([a.stop(), b.stop()]);
  const all = [...a.tokens, ...b.tokens];

  await clause("C1", "path direct within 90 s of pairing", "direct <= 90 s", () => c1);
  await clause(
    "C2",
    "texts equal, every token exactly once, within 15 s of the last keystroke",
    "<= 15 s",
    async () => {
      const t = await until(() => consistent([alice, bob], all), 15_000, 500);
      return { ok: t !== undefined, measured: `${secs(t)} for ${all.length} tokens` };
    },
  );
  await clause("C3", "zero PeerDown on either side", "downsTotal = 0", async () => {
    const downs = await Promise.all(
      [alice, bob].map(async (p) => field(await only(p), "downsTotal")),
    );
    return { ok: downs.every((d) => d === 0), measured: `downsTotal ${downs.join(", ")}` };
  });
  await clause(
    "C4",
    "reported RTT at the end of each phase (40 s after its change) follows the injected delay",
    "[0.8 R, 1.5 R + 20 ms], R = 60 / 300 / 60",
    async () => {
      if (metricsMissing) throw metricsMissing;
      const bad: string[] = [];
      for (const [phase, r] of Object.entries(truth))
        for (const v of samples[phase].rtt.slice(-2))
          if (!range(v, 0.8 * r, 1.5 * r + 20)) bad.push(`${phase} ${v} (R=${r})`);
      return {
        ok: bad.length === 0,
        measured:
          Object.entries(samples)
            .map(([k, v]) => `${k}: ${v.rtt.join("/")}`)
            .join("; ") + (bad.length ? ` OUT ${bad.join(", ")}` : ""),
      };
    },
  );
  await clause(
    "C5",
    "reported loss follows the injected loss (30 s window)",
    "P1, P2 in [1 %, 10 %]; P3 <= 0.5 %",
    async () => {
      if (metricsMissing) throw metricsMissing;
      const bad: string[] = [];
      for (const phase of ["P1", "P2"])
        for (const v of samples[phase].loss.slice(-2))
          if (!range(v, 1, 10)) bad.push(`${phase} ${v}`);
      for (const v of samples.P3.loss.slice(-2)) if (!range(v, 0, 0.5)) bad.push(`P3 ${v}`);
      return {
        ok: bad.length === 0,
        measured:
          Object.entries(samples)
            .map(([k, v]) => `${k}: ${v.loss.join("/")}`)
            .join("; ") + (bad.length ? ` OUT ${bad.join(", ")}` : ""),
      };
    },
  );
  await clause(
    "C6",
    "anti-vacuity: every netem qdisc dropped packets in P1",
    "dropped > 0 on 4 qdiscs",
    async () => {
      const quiet = p1Stats.filter((q) => q.dropped === 0);
      return {
        ok: p1Stats.length === 4 && quiet.length === 0,
        measured: p1Stats
          .map((q) => `${q.ns}/${q.dev} sent ${q.sent} dropped ${q.dropped}`)
          .join("; "),
      };
    },
  );
  finish({ tokens: all.length });
});

test("scenario 6: NI-2 25 s blackout during a large paste (characterization)", async ({
  browser,
}) => {
  test.setTimeout(420_000);
  const { alice, bob } = await twoPeople(browser);
  const paste = cjk(100_000, rng(6));
  const b = typist(bob, "b", 1000, 61);
  await sleep(5000);
  const before = await snapshot([alice, bob]);

  topo("profile", "blackout");
  const t0 = Date.now();
  await sleep(1000);
  await pasteAtEnd(alice, paste);
  await sleep(Math.max(0, 25_000 - (Date.now() - t0)));
  const dropped = qstats("blackout"); // read before the qdiscs are removed
  topo("unprofile", "blackout");
  const restored = Date.now();
  await b.stop();
  const during = b.times.filter((t) => t > t0 && t < restored).length;
  const after = await snapshot([alice, bob]);

  // One loop finds when each condition first holds: the bound is 35 s, but it watches 60 s so a
  // failure says "late" or "never". The checks run in the pages (length, hash, needles): a probe
  // returns a few bytes, not the 100 KB document.
  const first: Record<string, number | undefined> = {};
  const all = b.tokens;
  await Promise.all([alice, bob].map((p) => stash(p, "s6", [paste, ...all])));
  const deadline = restored + 60_000;
  while (Date.now() < deadline && Object.keys(first).length < 2) {
    const [ta, tb] = await Promise.all([probe(alice, "s6"), probe(bob, "s6")]);
    const now = Date.now() - restored;
    if (first.C2 === undefined && ta.len === tb.len && ta.hash === tb.hash && tb.has[0])
      first.C2 = now;
    if (first.C3 === undefined && ta.has.slice(1).every(Boolean)) first.C3 = now;
    await sleep(1000);
  }

  await clause(
    "C1",
    "the transport connection is not torn down (no new real outage on either side)",
    "downsTotal unchanged; resyncs may rise",
    async () => notTornDown(before, after),
  );
  await clause(
    "C2",
    "Bob has the whole paste and both texts are equal",
    "<= 35 s after restore",
    async () => ({
      ok: first.C2 !== undefined && first.C2 <= 35_000,
      measured: `${secs(first.C2)} after restore (watched 60 s)`,
    }),
  );
  await clause(
    "C3",
    `every token Bob typed during the blackout reaches Alice (${during} typed during it)`,
    "<= 35 s after restore",
    async () => ({
      ok: first.C3 !== undefined && first.C3 <= 35_000 && during > 0,
      measured: `${secs(first.C3)} after restore (watched 60 s), ${all.length} tokens`,
    }),
  );
  await clause(
    "C4",
    "anti-vacuity: the blackout qdiscs dropped packets",
    "dropped > 0 on both",
    async () => ({
      ok: dropped.length === 2 && dropped.every((q) => q.dropped > 0),
      measured: dropped
        .map((q) => `${q.ns}/${q.dev} sent ${q.sent} dropped ${q.dropped}`)
        .join("; "),
    }),
  );
  finish();
});

test("scenario 7: NI-3 8 KiB/s relay (the transfer finishes, typing is not held back, the link stays up)", async ({
  browser,
}) => {
  test.setTimeout(480_000);
  const { alice, bob } = await twoPeople(browser);
  // Plan: Alice types in a second file. Files other than main.typ are read-only in this build, so
  // Bob types in the same document instead: his tokens do not depend on Alice's paste (changes of
  // different actors), so they can overtake it, which is what the clause measures.
  const paste = cjk(300_000, rng(7));
  await stash(bob, "paste", [paste]);
  // The profile starts at once; the baseline is read as soon as both links exist, within the
  // scenario's 175 s window (a person who is not an active member yet has no link).
  const beforeRead = snapshot([alice, bob], 175_000);
  topo("profile", "slowrelay");
  const t0 = Date.now();
  await pasteAtEnd(alice, paste);
  const b = typist(bob, "b", 2000, 71);

  // C2: latency of each of Bob's tokens at Alice. C1: time for Bob to hold Alice's whole paste.
  const seen = new Map<string, number>();
  let c1: number | undefined;
  const deadline = t0 + 175_000;
  while (Date.now() < deadline && (c1 === undefined || seen.size < b.tokens.length)) {
    // In the pages, on the editor state: only a few booleans cross CDP.
    const tokens = [...b.tokens];
    const times = [...b.times];
    await stash(alice, "tokens", tokens);
    const mine = await probe(alice, "tokens");
    const now = Date.now();
    for (const [i, token] of tokens.entries())
      if (!seen.has(token) && mine.has[i]) seen.set(token, now - times[i]);
    if (c1 === undefined && (await probe(bob, "paste")).has[0]) {
      c1 = Date.now() - t0;
      void b.stop();
    }
    await sleep(1000);
  }
  await b.stop();
  const transferMs = c1 ?? Date.now() - t0;
  const heldBack: string[] = [];
  for (const [i, token] of b.tokens.entries())
    if (!seen.has(token)) heldBack.push(`${token.trim()} (${secs(Date.now() - b.times[i])})`);
  const before = await beforeRead;
  const after = await snapshot([alice, bob], Math.max(0, deadline - Date.now()));
  const stats = qstats("slowrelay");
  const worst = Math.max(0, ...seen.values());

  await clause(
    "C1",
    "Bob's document holds Alice's whole 300 000-character paste",
    "<= 175 s",
    async () => ({
      ok: c1 !== undefined && c1 <= 175_000,
      measured: c1 === undefined ? "never within 175 s" : secs(c1),
    }),
  );
  await clause(
    "C2",
    "each token typed during the transfer is visible at the other side",
    "p100 <= 10 s",
    async () => ({
      ok: b.tokens.length > 0 && heldBack.length === 0 && worst <= 10_000,
      measured: `${seen.size}/${b.tokens.length} seen, worst ${secs(worst)}${heldBack.length ? `, held back ${heldBack.slice(0, 3).join(" ")}` : ""}`,
    }),
  );
  await clause(
    "C3",
    "the transport connection is not torn down (no new real outage), and the path stays relayed",
    "downsTotal unchanged, direct packets = 0; resyncs may rise",
    async () => {
      const direct = directPackets();
      const v = notTornDown(before, after);
      return {
        ok: direct === 0 && v.ok,
        measured: `${v.measured}, ${direct} packets across the NATs`,
      };
    },
  );
  await clause(
    "C4",
    "anti-vacuity: the relay's qdisc carried >= 0.6 x 8 KiB/s and queued to its limit",
    "bytes >= 0.6 x 8192 x T; dropped or overlimits > 0",
    async () => {
      const q = stats[0];
      const need = 0.6 * 8192 * (transferMs / 1000);
      return {
        ok: q !== undefined && q.bytes >= need && q.dropped + q.overlimits > 0,
        measured: `${q?.bytes} bytes over ${secs(transferMs)} (need ${Math.round(need)}), dropped ${q?.dropped}, overlimits ${q?.overlimits}`,
      };
    },
  );
  finish();
});

/** `admin` makes `name` a reader. Whoever hosts keeps the Admin role (it cannot be handed back to a guest as a reader), so the reader pass has Alice host and invite Carol. */
async function makeReader(admin: Page, host: Page, name: string) {
  const panel = admin.getByRole("dialog", { name: "Personnes" });
  await admin.getByRole("button", { name: "Voir les personnes" }).click();
  await admin.getByLabel(`Rôle de ${name}`).selectOption("reader");
  await expect(admin.getByLabel(`Rôle de ${name}`)).toHaveValue("reader");
  await panel.getByRole("button", { name: "Fermer" }).click();
  await expect(host.getByTestId("access-banner")).toContainText("sans pouvoir le modifier", {
    timeout: 30_000,
  });
}

test("scenario 8: NI-4 three devices, alice-bob down (converge through carol, no amplification)", async ({
  browser,
}) => {
  test.setTimeout(900_000);
  const carolWrites = variant !== "reader";
  const alice = await open(browser, "http://127.0.0.1:18081/");
  const bob = await open(browser, "http://127.0.0.1:18082/", true);
  const carol = await open(browser, "http://127.0.0.1:18083/");
  // Editor pass: Carol hosts the project, so both tickets carry her public address (11.0.0.20, no
  // NAT). Reader pass: Alice hosts (her tickets go through the relay; Carol is reachable directly
  // all the same) and makes Carol a reader.
  const host = carolWrites ? carol : alice;
  await joinAs(
    host,
    carolWrites ? alice : carol,
    carolWrites ? "Alice" : "Carol",
    carolWrites ? "Carol" : "Alice",
  );
  await joinAs(host, bob, "Bob");
  for (const p of [alice, bob, carol])
    await expect(p.getByRole("contentinfo", { name: "Barre d'état" })).toContainText(
      "3 personnes",
      {
        timeout: 60_000,
      },
    );
  if (!carolWrites) await makeReader(alice, carol, "Carol");
  const pages = [alice, bob, carol];

  // Wait for the expected links: A-C and B-C direct, A-B relayed (metrics `path`).
  let metricsMissing: MetricsMissing | undefined;
  const paths = async (p: Page) =>
    (await links(p))
      .map((l) => (isDirect(l) ? "direct" : (l.path ?? "?")))
      .sort()
      .join("+");
  const expectedPhase1 = ["direct+relayed", "direct+relayed", "direct+direct"].join(" | ");
  let seen1 = "";
  try {
    await until(async () => {
      seen1 = (await Promise.all(pages.map(paths))).join(" | ");
      return seen1 === expectedPhase1;
    }, 120_000);
  } catch (e) {
    if (!(e instanceof MetricsMissing)) throw e;
    metricsMissing = e;
  }

  const sync = async () => {
    const m = new Map<string, number>();
    for (const [i, p] of pages.entries())
      for (const l of await links(p)) m.set(`${i}>${l.peer}`, field(l, "syncSent") ?? Number.NaN);
    return m;
  };
  const baseline = metricsMissing ? undefined : await sync().catch(() => undefined);

  // Phase 1: full mesh. Each device writes 30 tokens as single edits, interleaved, then 30 s idle.
  const writers = [
    typist(alice, "a", 1000, 81, { count: 30 }),
    typist(bob, "b", 1000, 82, { count: 30 }),
    ...(carolWrites ? [typist(carol, "c", 1000, 83, { count: 30 })] : []),
  ];
  await Promise.all(writers.map((w) => w.done));
  const tokens1 = writers.flatMap((w) => w.tokens);
  const t1 = await until(() => consistent(pages, tokens1), 15_000, 500);
  const afterPhase1 = metricsMissing ? undefined : await sync().catch(() => undefined);
  // The quiescence baseline is taken after a short settle: the tail of the last exchanges is not idle traffic.
  await sleep(3_000);
  const settled = metricsMissing ? undefined : await sync().catch(() => undefined);
  await sleep(30_000);
  const afterIdle = metricsMissing ? undefined : await sync().catch(() => undefined);

  // Phase 2: the relay's card goes down: the A-B circuit dies, A-C and B-C do not use it.
  topo("relay-down");
  const tDown = Date.now();
  let goneAt: number | undefined;
  let phase2Paths = "";
  if (metricsMissing) {
    await sleep(75_000);
  } else {
    goneAt = await until(async () => {
      phase2Paths = (await Promise.all(pages.map(paths))).join(" | ");
      return phase2Paths === ["direct", "direct", "direct+direct"].join(" | ");
    }, 90_000);
  }
  const w2 = [
    typist(alice, "x", 1000, 84, { count: 20 }),
    typist(bob, "y", 1000, 85, { count: 20 }),
  ];
  await Promise.all(w2.map((w) => w.done));
  const all = [...tokens1, ...w2.flatMap((w) => w.tokens)];
  const t2 = await until(() => consistent(pages, all), 15_000, 500);
  topo("relay-up");

  await clause(
    "C1",
    "phase 1 links: A-C and B-C direct, A-B relayed",
    "paths direct+relayed / direct+relayed / direct+direct",
    async () => {
      if (metricsMissing) throw metricsMissing;
      return { ok: seen1 === expectedPhase1, measured: seen1 };
    },
  );
  await clause(
    "C2",
    "anti-vacuity: in phase 2 A has no link to B and B none to A, both keep C",
    "<= 60 s after relay-down",
    async () => {
      if (metricsMissing) throw metricsMissing;
      return {
        ok: goneAt !== undefined && goneAt <= 60_000,
        measured: `${secs(goneAt)} after relay-down; ${phase2Paths}`,
      };
    },
  );
  await clause(
    "C3",
    "all three texts equal, every token exactly once, within 15 s of the last keystroke (both phases)",
    "<= 15 s",
    async () => ({
      ok: t1 !== undefined && t2 !== undefined,
      measured: `phase 1 ${secs(t1)} (${tokens1.length} tokens), phase 2 ${secs(t2)} (${all.length - tokens1.length} tokens, A-B down for ${secs(Date.now() - tDown)})`,
    }),
  );
  await clause(
    "C4",
    "no amplification: sync messages sent in phase 1",
    "<= 2 n (n - 1) = 12 per local change",
    async () => {
      if (metricsMissing) throw metricsMissing;
      if (!baseline || !afterPhase1 || [...afterPhase1.values()].some(Number.isNaN))
        throw new MetricsMissing("syncSent is not exposed per link (plan section 7)");
      let sent = 0;
      for (const [k, v] of afterPhase1) sent += v - (baseline.get(k) ?? 0);
      const bound = 12 * tokens1.length;
      return {
        ok: sent <= bound,
        measured: `${sent} sync messages for ${tokens1.length} changes (bound ${bound})`,
      };
    },
  );
  await clause(
    "C5",
    "quiescence: no sync traffic once converged",
    "<= 2 per ordered pair in 30 s",
    async () => {
      if (metricsMissing) throw metricsMissing;
      if (!settled || !afterIdle)
        throw new MetricsMissing("syncSent is not exposed per link (plan section 7)");
      const rows = [...afterIdle].map(([k, v]) => [k, v - (settled.get(k) ?? 0)] as const);
      return {
        ok: rows.every(([, d]) => d <= 2),
        measured: rows.map(([k, d]) => `${k} ${d}`).join(", "),
      };
    },
  );
  finish({ carolWrites });
});
