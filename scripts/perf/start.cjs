// Paired startup driver (block PB, plan section 8): cold application process to usable editor, one start per
// slot of a plan written by scripts/perf/paired.py, the two sides interleaved in the plan's order.
//   node scripts/perf/start.cjs --plan <plan.json>
// For each slot: nothing may listen on the port; open a page, launch the side's binary in a fresh cgroup with its
// home and documents directory, poll /health every 20 ms, load the page, wait for `.cm-content`; record the
// duration, the exit status, the tail of stderr, the load average and the CPU and IO pressure over the slot
// (diagnostics, never a verdict input); kill, await exit, empty and remove the cgroup, close the page, idle for
// cool_ms.
// Per-sample interference (plan section 18): each start also records the CPU time that the rest of the machine
// used during its window, outside the run's cgroup, which holds this driver, the browser and the start's own
// cgroup (cpu.cjs: window_ms, bg_cpu_ms, bg_share in CPUs; app_cpu_ms and leftover from the start's cgroup).
// The per-start decision is slotStep, the twin of paired.slot_step (testdata/slot-rule.json): with
// plan.bg_share_max set, a start above it is invalid, kept in `discarded` and re-measured in its own slot (same
// side, same place in the pair order) once the machine is quiet again, at most plan.max_tries times; more than
// plan.invalid_max of the plan's slots invalid, or a slot invalid max_tries times, is the fault 'interference'.
// The bound reads background CPU, never the duration, so it cannot hide a slow application.
// No writable cgroup v2, or a descendant that left the run's cgroup and cannot be brought back (cpu.adopt, run
// after the browser launch and before every start): the fault 'accounting' (an invalid attempt).
// The browser is prelaunched and OS caches are not evicted: not a native window oracle.
// Output (plan.out): {seed, port, planned, bg_share_max, invalid_max, fault, slots, discarded, adopted}. fault is null, or
// {kind: 'port-busy' | 'not-ready' | 'interference' | 'accounting', side, index, detail}: a fault stops the
// attempt there; the verdict is paired.py's.
const {spawn} = require('node:child_process');
const {performance} = require('node:perf_hooks');
const net = require('node:net');
const fs = require('node:fs');
const cpu = require('./cpu.cjs');

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const STDERR_TAIL = 2048;

const listening = (port) => new Promise((resolve) => {
  const socket = net.connect({host: '127.0.0.1', port});
  const done = (v) => { socket.destroy(); resolve(v); };
  socket.setTimeout(500, () => done(false));
  socket.once('connect', () => done(true));
  socket.once('error', () => done(false));
});

const delta = (a, b) => (a === null || b === null ? null : b - a);

// The decision after one measured start of a slot: 'keep', 'retry', 'stop-cap' or 'stop-tries'. The twin of
// paired.slot_step; both are checked against testdata/slot-rule.json. `tries` counts this start, `discarded` the
// invalid starts of the run before it; `rule` is the plan (bg_share_max, invalid_max, max_tries).
function slotStep(bgShare, tries, discarded, planned, rule) {
  if (rule.bg_share_max == null || !(bgShare > rule.bg_share_max)) return 'keep';
  if (discarded + 1 > rule.invalid_max * planned) return 'stop-cap';
  return tries >= rule.max_tries ? 'stop-tries' : 'retry';
}

async function slotRun(browser, plan, slot, order, hz, run, attempt) {
  const record = {index: slot.index, pair: slot.pair, side: slot.side, warmup: slot.warmup, order, ms: null, ready: false,
    exit: null, stderr: '', load: cpu.load1(), cpu_pressure_us: null, io_pressure_us: null,
    window_ms: null, bg_cpu_ms: null, bg_share: null, app_cpu_ms: null, leftover: null, valid: true};
  const cpu0 = cpu.pressure('cpu'), io0 = cpu.pressure('io');
  fs.mkdirSync(slot.home, {recursive: true});
  fs.mkdirSync(slot.docs, {recursive: true});
  const cgroup = cpu.startCgroup(run, slot.index, attempt);
  const [command, args] = cpu.launcher(cgroup, slot.bin);
  const page = await browser.newPage();
  const opened = cpu.snapshot(run);
  const start = performance.now();
  const child = spawn(command, args, {
    env: {...process.env, INKWELL_SERVER_PORT: String(plan.port), INKWELL_RENDEZVOUS: 'off',
      INKWELL_HOME: slot.home, INKWELL_DOCS: slot.docs},
    stdio: ['ignore', 'ignore', 'pipe']});
  let exited = false, status = null;
  const exit = new Promise((r) => child.once('close', (code, signal) => { exited = true; status = {code, signal}; r(); }));
  child.once('error', (e) => { record.stderr += String(e); exited = true; status = {code: null, signal: null}; });
  child.stderr.on('data', (d) => { record.stderr = (record.stderr + d).slice(-STDERR_TAIL); });
  let fault = null;
  try {
    const deadline = start + plan.ready_timeout_ms;
    while (!record.ready && !exited && performance.now() < deadline) {
      try { record.ready = (await fetch(`http://127.0.0.1:${plan.port}/health`)).ok; } catch { /* not listening yet */ }
      if (!record.ready) await sleep(20);
    }
    if (!record.ready) {
      const busy = /address already in use/i.test(record.stderr);
      const joined = !(exited && status?.code === cpu.LAUNCH_FAILED);
      fault = {kind: !joined ? 'accounting' : busy ? 'port-busy' : 'not-ready', side: slot.side, index: slot.index,
        detail: !joined ? `the launcher could not join ${cgroup}` :
          exited ? `server exited before ready (${JSON.stringify(status)})` : 'server not ready in time'};
    } else {
      await page.goto(`http://127.0.0.1:${plan.port}`);
      await page.locator('.cm-content').waitFor();
      record.ms = performance.now() - start;
      Object.assign(record, cpu.background(opened, cpu.snapshot(run, {opening: false}), hz));
    }
  } catch (e) {
    fault = {kind: 'not-ready', side: slot.side, index: slot.index, detail: String(e)};
  } finally {
    child.kill();
    await exit;
    record.exit = status;
    await page.close();
    record.cpu_pressure_us = delta(cpu0, cpu.pressure('cpu'));
    record.io_pressure_us = delta(io0, cpu.pressure('io'));
  }
  try {
    Object.assign(record, await cpu.closeStart(cgroup));
  } catch (e) {
    fault = fault || {kind: 'accounting', side: slot.side, index: slot.index, detail: String(e)};
  }
  return {record, fault};
}

// Wait, at most plan.quiet_wait_ms, until two consecutive 100 ms windows hold no more background CPU than
// the bound: the re-measurement of an invalid start does not start inside the burst that spoiled it.
async function quiet(plan, hz, run) {
  const deadline = performance.now() + plan.quiet_wait_ms;
  let calm = 0;
  while (calm < 2 && performance.now() < deadline) {
    const open = cpu.snapshot(run);
    await sleep(100);
    calm = slotStep(cpu.background(open, cpu.snapshot(run, {opening: false}), hz).bg_share, 1, 0, Infinity, plan) === 'keep' ? calm + 1 : 0;
  }
}

const STOP = {
  'stop-cap': (plan, n) => `${n} starts above ${plan.bg_share_max} background CPUs, more than ${plan.invalid_max} of the plan`,
  'stop-tries': (plan, n, tries) => `${tries} tries above ${plan.bg_share_max} background CPUs`,
};

async function main() {
  const {chromium} = require('../../frontend/node_modules/@playwright/test');
  const planPath = process.argv[process.argv.indexOf('--plan') + 1];
  const plan = JSON.parse(fs.readFileSync(planPath, 'utf8'));
  const result = {seed: plan.seed, port: plan.port, planned: plan.slots.length, bg_share_max: plan.bg_share_max,
    invalid_max: plan.invalid_max, fault: null, slots: [], discarded: [], adopted: []};
  const write = () => fs.writeFileSync(plan.out, JSON.stringify(result, null, 1));
  let run;
  try {
    run = cpu.openRun();
  } catch (e) {
    result.fault = {kind: 'accounting', side: null, index: null, detail: String(e.message)};
    write();
    return;
  }
  const seen = new Set();
  const hz = cpu.clockTicks();
  const browser = await chromium.launch({headless: true});
  try {
    for (const slot of plan.slots) {
      try {
        result.adopted.push(...cpu.adopt(run).map((m) => ({...m, before: slot.index})));
      } catch (e) {
        result.fault = {kind: 'accounting', side: slot.side, index: slot.index, detail: String(e.message)};
        break;
      }
      if (await listening(plan.port)) {
        result.fault = {kind: 'port-busy', side: slot.side, index: slot.index, detail: 'something listens on the port'};
        break;
      }
      const order = seen.has(slot.pair) ? 'second' : 'first';
      seen.add(slot.pair);
      let tries = 0, last = null;
      for (;;) {
        tries += 1;
        last = await slotRun(browser, plan, slot, order, hz, run, tries);
        last.record.try = tries;
        if (last.fault) break;
        const step = slotStep(last.record.bg_share, tries, result.discarded.length, plan.slots.length, plan);
        last.record.valid = step === 'keep';
        if (step === 'keep') break;
        result.discarded.push(last.record);
        if (step !== 'retry') {
          last.fault = {kind: 'interference', side: slot.side, index: slot.index,
            detail: STOP[step](plan, result.discarded.length, tries)};
          break;
        }
        await sleep(plan.cool_ms);
        await quiet(plan, hz, run);
      }
      if (last.fault?.kind !== 'interference') result.slots.push(last.record);  // an interference stop keeps it in `discarded`
      if (last.fault) { result.fault = last.fault; break; }
      await sleep(plan.cool_ms);
    }
  } finally {
    await browser.close();
    cpu.closeRun(run);
  }
  write();
}

module.exports = {slotStep};

if (require.main === module) main().catch((e) => { console.error(e); process.exitCode = 1; });
