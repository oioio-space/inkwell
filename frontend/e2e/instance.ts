import type { ChildProcess } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { signalOwned, spawnOwned } from "./own";

const server = resolve(import.meta.dirname, "../../bin/inkwell-server");

/** The launch of an instance: its arguments, profile folder (INKWELL_HOME) and working directory. */
export interface Launch {
  args?: string[];
  /** A fresh temporary profile when unset: each instance is then its own person. */
  home?: string;
  cwd?: string;
  /**
   * A /bin/sh script that runs the server as "$0", for arguments a JavaScript string cannot hold
   * (bytes that are not UTF-8); args is then ignored.
   */
  shell?: string;
  /** Extra environment of the launch (its language, for instance). */
  env?: Record<string, string>;
}

const env = (port: number, docs: string, home: string) => ({
  ...process.env,
  INKWELL_SERVER_PORT: String(port),
  INKWELL_RENDEZVOUS: "off",
  INKWELL_HOME: home,
  INKWELL_DOCS: docs,
});

/** Starts another inkwell-server over the project folders in `docs` and waits until it answers. */
export async function startInstance(
  port: number,
  docs: string,
  launch: Launch = {},
): Promise<ChildProcess> {
  const home = launch.home ?? mkdtempSync(join(tmpdir(), "inkwell-e2e-home-"));
  const child = spawnOwned(server, launch.args ?? [], {
    env: env(port, docs, home),
    cwd: launch.cwd,
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

/**
 * Launches inkwell-server as the desktop would for « Ouvrir avec », on a profile where an instance
 * may already run, and returns its exit code and how long it ran; it is killed after `limitMs`.
 */
export async function launchAgain(
  port: number,
  docs: string,
  launch: Launch & { home: string },
  limitMs = 10_000,
): Promise<{ code: number | null; ms: number; stderr: string }> {
  const start = Date.now();
  const [file, args]: [string, string[]] = launch.shell
    ? ["/bin/sh", ["-c", launch.shell, server]]
    : [server, launch.args ?? []];
  const child = spawnOwned(file, args, {
    env: { ...env(port, docs, launch.home), ...launch.env },
    cwd: launch.cwd,
    stdio: ["ignore", "ignore", "pipe"],
  });
  let stderr = "";
  child.stderr?.on("data", (b: Buffer) => {
    stderr += b.toString();
  });
  const timer = setTimeout(() => signalOwned(child, "SIGKILL"), limitMs);
  const code = await new Promise<number | null>((r) => child.once("close", (c) => r(c)));
  clearTimeout(timer);
  return { code, ms: Date.now() - start, stderr };
}

/** Stops an instance and waits for it to be gone: its writes must not reach the next test. */
export async function stopInstance(child: ChildProcess): Promise<void> {
  const exited = new Promise((r) => child.once("exit", r));
  signalOwned(child, "SIGTERM");
  await exited;
}
