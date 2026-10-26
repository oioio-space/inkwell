import { type ChildProcess, spawn } from "node:child_process";

// A run signals only the processes it started itself, by handle, never by name or by port: other
// checkouts run the same binaries at the same time. spawnOwned is the only way specs start one,
// and signalOwned refuses anything else.
const owned = new WeakSet<ChildProcess>();

/** Starts a process that this run owns. */
export function spawnOwned(...args: Parameters<typeof spawn>): ChildProcess {
  const child = spawn(...args);
  owned.add(child);
  return child;
}

/** Signals a process this run started; throws for any other. */
export function signalOwned(child: ChildProcess, signal: NodeJS.Signals): void {
  if (!owned.has(child)) throw new Error("refusing to signal a process this run did not start");
  if (child.pid !== undefined) child.kill(signal);
}
