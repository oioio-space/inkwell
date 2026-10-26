import type { ChildProcess } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { signalOwned, spawnOwned } from "./own";

/** Starts another inkwell-server over the project folders in `docs` and waits until it answers. */
export async function startInstance(port: number, docs: string): Promise<ChildProcess> {
  const child = spawnOwned(resolve(import.meta.dirname, "../../bin/inkwell-server"), [], {
    env: {
      ...process.env,
      INKWELL_SERVER_PORT: String(port),
      INKWELL_RENDEZVOUS: "off",
      INKWELL_HOME: mkdtempSync(join(tmpdir(), "inkwell-e2e-home-")),
      INKWELL_DOCS: docs,
    },
    stdio: "ignore",
  });
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(`http://127.0.0.1:${port}/health`)).ok) return child;
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  signalOwned(child, "SIGKILL");
  throw new Error(`instance on port ${port} did not start`);
}

/** Stops an instance and waits for it to be gone: its writes must not reach the next test. */
export async function stopInstance(child: ChildProcess): Promise<void> {
  const exited = new Promise((r) => child.once("exit", r));
  signalOwned(child, "SIGTERM");
  await exited;
}
