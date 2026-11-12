// CPU accounting of the paired startup driver (block PB, plan section 18.1): which CPU of a start's window was
// used by the measured set, and which by the rest of the machine (the background).
//   background = (busy CPU of the machine, /proc/stat) - (CPU of the run's cgroup, cpu.stat usage_usec)
// The measured set is a cgroup v2 subtree the driver creates under its own cgroup: `<run>/driver` holds the
// driver and the browser it launches, and every start runs in its own `<run>/start-<index>-<try>`, joined by the
// launcher before it executes the application (`launcher`). A process cannot leave its cgroup by forking,
// double-forking or calling setsid, so a detached helper's CPU is the application's, not background, and the
// cgroup keeps the CPU of children that exited or were reaped (the old process-tree accounting lost both).
// After a start the driver kills what is left in its cgroup (cgroup.kill) and removes it.
// A process can still be moved out by a cgroup writer: Chromium asks systemd over D-Bus to put its browser process in a
// scope of its own (app-org.chromium.Chromium-<pid>.scope), which would turn the browser's CPU into background. The
// driver keeps the invariant "every descendant of the driver is in the run's cgroup" with `adopt`, after the launch
// and before every start (outside the timed window), and records what it moved.
// /proc/stat is tick sampled (USER_HZ, 100 per second): a window of a few hundred milliseconds has a resolution
// of about one jiffy per busy CPU; usage_usec is exact. A negative difference is clamped to 0.
// No cgroup v2, or no writable cgroup: openRun throws an AccountingError and the driver writes the fault
// `accounting`, an invalid attempt (paired.classify), never a measured one.
const fs = require('node:fs');
const path = require('node:path');
const {execFileSync} = require('node:child_process');

class AccountingError extends Error {}

// user nice system irq softirq steal of the aggregate `cpu` line; idle and iowait are not busy, guest is
// already inside user.
function busyJiffies(statText) {
  const line = statText.split('\n').find((l) => l.startsWith('cpu '));
  if (!line) throw new Error('no aggregate cpu line in /proc/stat');
  const f = line.trim().split(/\s+/).slice(1).map(Number);
  return f[0] + f[1] + f[2] + f[5] + f[6] + (f[7] || 0);
}

// The cgroup v2 path of a process, from the text of /proc/<pid>/cgroup (the `0::` line).
function ownCgroup(text) {
  const line = text.split('\n').find((l) => l.startsWith('0::'));
  if (!line) throw new AccountingError('no cgroup v2 membership in /proc/self/cgroup');
  return line.slice(3);
}

// usage_usec of a cpu.stat text: the CPU time of every process that ever ran in the cgroup or below it.
function usageUsec(statText) {
  const m = /^usage_usec (\d+)$/m.exec(statText);
  if (!m) throw new AccountingError('no usage_usec in cpu.stat');
  return Number(m[1]);
}

const usage = (dir) => usageUsec(fs.readFileSync(path.join(dir, 'cpu.stat'), 'utf8'));

// Create the run's cgroup under this process's own cgroup and move this process (the driver, and so the
// browser it launches next) into `<run>/driver`. `name` keeps concurrent runs apart.
function openRun({fsRoot = '/sys/fs/cgroup', self = '/proc/self/cgroup', pid = process.pid, name = `pb-${process.pid}`} = {}) {
  try {
    const own = ownCgroup(fs.readFileSync(self, 'utf8'));
    const base = path.join(fsRoot, own);
    const dir = path.join(base, name);
    fs.mkdirSync(path.join(dir, 'driver'), {recursive: true});
    fs.writeFileSync(path.join(dir, 'driver', 'cgroup.procs'), String(pid));
    usage(dir);  // readable, or no accounting
    return {base, dir, pid, path: path.posix.join(own, name)};
  } catch (e) {
    throw e instanceof AccountingError ? e : new AccountingError(`no writable cgroup v2: ${e.message}`);
  }
}

// The live descendants of `root`, from the parent pids of /proc (pid -> {ppid, comm, cgroup}).
function descendants(root, proc = '/proc') {
  const table = new Map();
  for (const name of fs.readdirSync(proc)) {
    if (!/^\d+$/.test(name)) continue;
    try {
      const stat = fs.readFileSync(path.join(proc, name, 'stat'), 'utf8');
      const close = stat.lastIndexOf(')');
      table.set(Number(name), {ppid: Number(stat.slice(close + 2).split(' ')[1]), comm: stat.slice(stat.indexOf('(') + 1, close)});
    } catch { /* exited */ }
  }
  const out = [];
  const todo = [root];
  while (todo.length) {
    const parent = todo.pop();
    for (const [pid, p] of table) if (p.ppid === parent) { out.push({pid, comm: p.comm}); todo.push(pid); }
  }
  return out;
}

// Move back into <run>/driver every descendant of the driver that left the run's cgroup. Returns [{pid, comm, from}];
// a process that cannot be moved back is an AccountingError (its CPU would be counted as background).
function adopt(run, {proc = '/proc'} = {}) {
  const moved = [];
  for (const {pid, comm} of descendants(run.pid, proc)) {
    let where;
    try { where = ownCgroup(fs.readFileSync(path.join(proc, String(pid), 'cgroup'), 'utf8')); } catch { continue; /* exited */ }
    if (where === run.path || where.startsWith(run.path + '/')) continue;
    try {
      fs.writeFileSync(path.join(run.dir, 'driver', 'cgroup.procs'), String(pid));
    } catch (e) {
      if (fs.existsSync(path.join(proc, String(pid)))) throw new AccountingError(`cannot bring ${comm} (${pid}) back from ${where}: ${e.message}`);
      continue;
    }
    moved.push({pid, comm, from: where});
  }
  return moved;
}

// Move the driver back to its own cgroup and remove the run's (best effort: an empty cgroup left behind goes
// with its scope).
function closeRun(run) {
  try {
    fs.writeFileSync(path.join(run.base, 'cgroup.procs'), String(run.pid));
    fs.rmdirSync(path.join(run.dir, 'driver'));
    fs.rmdirSync(run.dir);
  } catch { /* left to the scope */ }
}

// A fresh cgroup for one start. The name has a fixed width, so that it does not vary with the index.
function startCgroup(run, index, attempt) {
  const dir = path.join(run.dir, `start-${String(index).padStart(4, '0')}-${attempt}`);
  try {
    fs.mkdirSync(dir);
    return dir;
  } catch (e) {
    throw new AccountingError(`cannot create ${dir}: ${e.message}`);
  }
}

// Exit status of the launcher when it could not join the start's cgroup (the application never ran).
const LAUNCH_FAILED = 125;

// [command, args] that run `bin` inside cgroup `dir`: the shell joins the cgroup, then executes the binary in
// the same process, so the application and everything it spawns are in the cgroup from their first instruction.
// The application's own argv and environment are unchanged.
function launcher(dir, bin) {
  return ['/bin/sh', ['-c', `echo $$ > "$0/cgroup.procs" || exit ${LAUNCH_FAILED}; exec "$1"`, dir, bin]];
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// After the application exited: kill whatever it left in its cgroup, wait until the cgroup is empty, read its
// CPU and remove it. Returns {app_cpu_ms, leftover} (processes that outlived the application).
async function closeStart(dir, {timeoutMs = 2000} = {}) {
  const procs = () => fs.readFileSync(path.join(dir, 'cgroup.procs'), 'utf8').split('\n').filter(Boolean);
  const leftover = procs().length;
  if (leftover) fs.writeFileSync(path.join(dir, 'cgroup.kill'), '1');
  const deadline = performance.now() + timeoutMs;
  while (/populated 1/.test(fs.readFileSync(path.join(dir, 'cgroup.events'), 'utf8'))) {
    if (performance.now() > deadline) throw new AccountingError(`${dir} still populated after cgroup.kill`);
    await sleep(10);
  }
  const appCpuMs = usage(dir) / 1000;
  fs.rmdirSync(dir);
  return {app_cpu_ms: appCpuMs, leftover};
}

// A snapshot of the machine and of the run: busy jiffies and the run cgroup's usage. The machine counter is read
// last when opening a window and first when closing it, so that the reads stay outside the window.
function snapshot(run, {proc = '/proc', now = () => performance.now(), opening = true} = {}) {
  const busy = () => busyJiffies(fs.readFileSync(path.join(proc, 'stat'), 'utf8'));
  if (opening) {
    const measured = usage(run.dir);
    return {at: now(), busy: busy(), measured};
  }
  const b = busy(), at = now();
  return {at, busy: b, measured: usage(run.dir)};
}

// The background CPU between two snapshots: {window_ms, bg_cpu_ms, bg_share}. bg_share is in CPUs (4.0 = four
// CPUs fully busy for the whole window).
function background(open, close, hz) {
  const machineMs = (close.busy - open.busy) * 1000 / hz;
  const measuredMs = (close.measured - open.measured) / 1000;
  const bgMs = Math.max(0, machineMs - measuredMs);
  const windowMs = close.at - open.at;
  return {window_ms: windowMs, bg_cpu_ms: bgMs, bg_share: windowMs > 0 ? bgMs / windowMs : 0};
}

// Diagnostics of a slot (never a verdict input): the 1-minute load and the `some total` stall of a PSI
// resource (microseconds), null where unavailable.
function load1(proc = '/proc') {
  try { return Number(fs.readFileSync(path.join(proc, 'loadavg'), 'utf8').split(' ')[0]); } catch { return null; }
}
function pressure(name, proc = '/proc') {
  try { return Number(/total=(\d+)/.exec(fs.readFileSync(path.join(proc, 'pressure', name), 'utf8'))[1]); } catch { return null; }
}
function clockTicks() {
  try { return Number(execFileSync('getconf', ['CLK_TCK'], {encoding: 'utf8'})) || 100; } catch { return 100; }
}

module.exports = {AccountingError, LAUNCH_FAILED, busyJiffies, ownCgroup, usageUsec, openRun, closeRun, startCgroup, adopt,
  launcher, closeStart, snapshot, background, load1, pressure, clockTicks};
