import { type ChildProcess, spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { expect, test } from "./fixtures";
import { docText, open, pair, typeAtEnd } from "./helpers";

// A relay is killed and restarted (same key, same port) while two people, both in
// relay-only mode, type. No root and no NAT: everything is on the loopback, so this
// covers "offline, then back" and leaves the NAT part to `mise run e2e:netsim`.
// The test starts its own relay and instances (the relay's address, which holds its
// identity, is only known once it runs).

const bin = (name: string) => resolve(import.meta.dirname, "../../bin", name);
const RELAY_PORT = 17419;
const ports = { alice: 18091, bob: 18092 };

const children: ChildProcess[] = [];
let dir = "";

function start(path: string, args: string[], env: Record<string, string> = {}): ChildProcess {
  const child = spawn(path, args, {
    env: { ...process.env, ...env },
    stdio: ["ignore", "pipe", "pipe"],
  });
  children.push(child);
  return child;
}

/** Starts the relay and resolves with its UDP address once it listens. */
async function startRelay(): Promise<{ proc: ChildProcess; addr: string }> {
  const proc = start(bin("inkwell-relay"), [
    "--key",
    join(dir, "relay.key"),
    "--listen",
    `/ip4/127.0.0.1/udp/${RELAY_PORT}/quic-v1`,
    "--listen",
    `/ip4/127.0.0.1/tcp/${RELAY_PORT}`,
    "--print-addr",
  ]);
  let out = "";
  const addr = await new Promise<string>((resolveAddr, reject) => {
    const timer = setTimeout(() => reject(new Error(`relay did not start: ${out}`)), 15_000);
    proc.once("exit", (code) => reject(new Error(`relay exited with ${code}: ${out}`)));
    proc.stdout?.on("data", (chunk: Buffer) => {
      out += chunk.toString();
      const m = out.match(/^\/ip4\/127\.0\.0\.1\/udp\/\d+\/quic-v1\/p2p\/\S+$/m);
      if (m) {
        clearTimeout(timer);
        resolveAddr(m[0]);
      }
    });
  });
  return { proc, addr };
}

async function waitHealthy(port: number) {
  await expect(async () => {
    const res = await fetch(`http://127.0.0.1:${port}/health`);
    expect(res.ok).toBe(true);
  }).toPass({ timeout: 30_000 });
}

/** Stops a process the polite way and waits for it, so its port is free again. */
async function stop(proc: ChildProcess) {
  if (proc.exitCode !== null || proc.signalCode !== null) return;
  const exited = new Promise((r) => proc.once("exit", r));
  proc.kill("SIGTERM");
  await exited;
}

test.beforeAll(() => {
  dir = mkdtempSync(join(tmpdir(), "inkwell-offline-"));
});

test.afterAll(() => {
  for (const c of children) c.kill("SIGKILL");
  rmSync(dir, { recursive: true, force: true });
});

test("a relay killed and restarted: both sides keep typing offline and converge", async ({
  browser,
}) => {
  test.setTimeout(180_000);
  let relay = await startRelay();
  for (const [who, port] of Object.entries(ports)) {
    start(bin("inkwell-server"), [], {
      INKWELL_SERVER_PORT: String(port),
      INKWELL_HOME: join(dir, who),
      INKWELL_DOCS: join(dir, `${who}-docs`),
      INKWELL_RELAYS: relay.addr,
      INKWELL_NET: "relay-only",
    });
  }
  await waitHealthy(ports.alice);
  await waitHealthy(ports.bob);

  const alice = await open(browser, `http://127.0.0.1:${ports.alice}/`);
  const bob = await open(browser, `http://127.0.0.1:${ports.bob}/`, true);
  await pair(alice, bob);

  await typeAtEnd(alice, "avant ");
  // Bob's page opens Alice's document once it has arrived through the relay (`repo.find`): the
  // "Synchronisé" status only says the link is up. That first transfer is a network round trip,
  // like the 30 s waits below, not an editor refresh.
  await expect(bob.locator(".cm-content")).toContainText("avant", { timeout: 30_000 });

  // The relay goes away: the link between them cannot survive without it.
  await stop(relay.proc);
  await expect(alice.getByRole("status")).toHaveText(/Seul/, { timeout: 30_000 });
  await expect(bob.getByRole("status")).toHaveText(/Seul/, { timeout: 30_000 });
  await typeAtEnd(alice, "alice-hors-ligne ");
  await typeAtEnd(bob, "bob-hors-ligne ");
  // Nothing crossed while the relay was down.
  expect(await docText(alice)).not.toContain("bob-hors-ligne");
  expect(await docText(bob)).not.toContain("alice-hors-ligne");

  // Same key, same port: the relay keeps its identity, so the people find it again.
  const restarted = Date.now();
  relay = await startRelay();
  for (const p of [alice, bob]) {
    await expect(p.locator(".cm-content")).toContainText("alice-hors-ligne", { timeout: 30_000 });
    await expect(p.locator(".cm-content")).toContainText("bob-hors-ligne", { timeout: 30_000 });
  }
  expect(Date.now() - restarted).toBeLessThan(30_000);
  await expect.poll(() => docText(alice)).toBe(await docText(bob));
  await expect(alice.getByRole("status")).toHaveText(/Synchronisé/);
});
