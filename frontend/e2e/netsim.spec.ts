import { execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { docText, open, pair, typeAtEnd } from "./helpers";

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
const note = (text: string) => writeFileSync(join(out, `${scenario}.txt`), text);

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
