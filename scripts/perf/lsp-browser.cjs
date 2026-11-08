// Actual browser -> Wails Stream -> Go -> pinned, confined tinymist latency.
// Run after the e2e-observe build, under heavy.sh --exclusive. No request retries.
const { chromium } = require('../../frontend/node_modules/@playwright/test');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '../..');
const out = path.resolve(process.env.INKWELL_PERF_OUT || path.join(root, 'gates/L3'));
const reportPath = path.join(out, 'PERF-LSP-browser.json');
const url = 'http://127.0.0.1:39129';
const fixtureRoot = path.join(root, 'testdata/perf/lsp');
const fixtureNames = ['main.typ', 'charter.typ', 'chapter-one.typ', 'chapter-two.typ'];
const sources = Object.fromEntries(fixtureNames.map(name => [name, fs.readFileSync(path.join(fixtureRoot, name), 'utf8')]));
const fixture = sources['main.typ'];
const fixtureLines = Object.values(sources).reduce((count, text) => count + text.split('\n').length - 1, 0);
if (fixtureLines < 3000) throw new Error(`Performance fixture shrank to ${fixtureLines} lines`);
const requests = [
  { name: 'completion', method: 'textDocument/completion', offset: fixture.indexOf('#tex') + 4 },
  { name: 'signature', method: 'textDocument/signatureHelp', offset: fixture.indexOf('#l3add(1, 2)') + '#l3add(1, '.length },
  { name: 'hover', method: 'textDocument/hover', offset: fixture.indexOf('#l3add(1, 2)') + 3 },
];
const pause = (ms) => new Promise(resolve => setTimeout(resolve, ms));
function useful(name, result) {
  if (!result || typeof result !== 'object') return false;
  if (name === 'completion') {
    const items = Array.isArray(result) ? result : result.items;
    return Array.isArray(items) && items.some(item => typeof item.label === 'string' && item.label.includes('text'));
  }
  if (name === 'signature') {
    return Array.isArray(result.signatures) && result.signatures.some(signature => typeof signature.label === 'string' && signature.label.includes('l3add'));
  }
  return Boolean(result.contents) && JSON.stringify(result.contents).includes('Adds two numbers');
}
async function main() {
  fs.mkdirSync(out, { recursive: true });
  fs.rmSync(reportPath, { force: true });
  // Never mistake another checkout's already-running service for this process.
  try {
    await fetch(url + '/health', { signal: AbortSignal.timeout(500) });
    throw new Error('PERF-LSP browser port39129 already has an HTTP listener');
  } catch (error) {
    if (!['TypeError', 'TimeoutError'].includes(error.name)) throw error;
  }
  const stateParent = path.join(root, '.task/perf');
  fs.mkdirSync(stateParent, { recursive: true });
  const state = fs.mkdtempSync(path.join(stateParent, 'lsp-browser-'));
  const log = fs.createWriteStream(path.join(out, 'lsp-browser-server.log'));
  const server = spawn(path.join(root, 'bin/inkwell-server'), [], {
    cwd: root,
    env: { ...process.env, INKWELL_SERVER_PORT: '39129', INKWELL_RENDEZVOUS: 'off',
      INKWELL_HOME: path.join(state, 'home'), INKWELL_DOCS: path.join(state, 'docs') },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  server.stdout.pipe(log);
  server.stderr.pipe(log);
  let launchError;
  server.on('error', error => { launchError = error; });
  const exited = new Promise(resolve => server.once('exit', resolve));
  let browser;
  try {
    let ready = false;
    for (let n = 0; n < 300; n++) {
      if (launchError) throw launchError;
      if (server.exitCode !== null) throw new Error(`Language server application exited ${server.exitCode}`);
      try {
        if ((await fetch(url + '/health', { signal: AbortSignal.timeout(500) })).ok) { ready = true; break; }
      } catch {}
      await pause(100);
    }
    if (!ready) throw new Error('Language server application did not become ready');
    const projects = fs.readdirSync(path.join(state, 'docs'), { withFileTypes: true }).filter(entry => entry.isDirectory());
    if (projects.length !== 1) throw new Error('Expected exactly one private performance project');
    const project = path.join(state, 'docs', projects[0].name);
    for (const name of fixtureNames.filter(name => name !== 'main.typ')) {
      fs.writeFileSync(path.join(project, name), sources[name], { mode: 0o600 });
    }
    browser = await chromium.launch({ headless: true });
    const page = await browser.newPage({ locale: 'fr-FR' });
    await page.goto(url);
    await page.locator('.cm-content').waitFor();
    await page.waitForFunction(() => globalThis.__inkwellLanguageReady?.() === true, undefined, { timeout: 30000 });
    await page.locator('.cm-content').fill(fixture);
    await page.waitForFunction(() => globalThis.__inkwellLanguageReady?.() === true, undefined, { timeout: 30000 });
    const report = { id: 'PERF-LSP-browser', threshold_ms: 100, engine: browser.version(),
      scope: 'Browser LanguageClient through actual Wails Stream, Go language service, pinned sandboxed tinymist; page-side request-to-useful-response; 40 expression edits at the imported charter call; varying completion-prefix, argument and hover positions in a multi-file manuscript; initialization excluded; no measured request retries',
      fixture_files: fixtureNames.length, fixture_lines: fixtureLines, edit_cycles: 40, edits: [], status: 'fail' };
    for (const request of requests) report[request.name] = { samples_ms: [], revisions: [], offsets: [] };
    let currentText = fixture;
    try {
      for (let cycle = 0; cycle < 40; cycle++) {
        const previous = await page.evaluate(() => globalThis.__inkwellLanguageVersion?.());
        if (!Number.isInteger(previous)) throw new Error('Missing e2e-only document revision observer');
        const edit = `#l3add(1, ${cycle + 3})`;
        const content = page.locator('.cm-content');
        await content.press('ControlOrMeta+Home');
        for (let n = 0; n < 3; n++) await content.press('ArrowDown');
        await content.press('Home');
        await content.press('Shift+End');
        await page.keyboard.insertText(edit);
        await page.waitForFunction(version => globalThis.__inkwellLanguageVersion?.() > version, previous);
        const revision = await page.evaluate(() => globalThis.__inkwellLanguageVersion());
        const visibleLines = await content.locator('.cm-line').allTextContents();
        if (!visibleLines.includes(edit)) throw new Error('The actual editor did not apply the expected imported-function call edit');
        const lines = currentText.split('\n');
        lines[3] = edit;
        currentText = lines.join('\n');
        report.edits.push({ cycle: cycle + 1, previous_revision: previous, revision, line: 3, text: edit });
        for (const template of requests) {
          const request = { ...template };
          if (request.name === 'completion') request.offset = currentText.indexOf('#tex') + 3 + cycle % 2;
          if (request.name === 'signature') request.offset = currentText.indexOf('#l3add(') + (cycle % 2 ? 10 : 7);
          if (request.name === 'hover') request.offset = currentText.indexOf('#l3add(') + 2 + cycle % 4;
          const sample = await page.evaluate(async ({ method, offset }) => {
            if (typeof globalThis.__inkwellLanguageRequest !== 'function') throw new Error('Missing e2e-only language observer');
            const start = performance.now();
            const result = await globalThis.__inkwellLanguageRequest(method, offset);
            return { elapsed_ms: performance.now() - start, result };
          }, request);
          if (!Number.isFinite(sample.elapsed_ms) || sample.elapsed_ms < 0) throw new Error('Invalid language timing sample');
          report[request.name].samples_ms.push(sample.elapsed_ms);
          report[request.name].revisions.push(revision);
          report[request.name].offsets.push(request.offset);
          if (!useful(request.name, sample.result)) {
            report.failed_request = { name: request.name, cycle: cycle + 1, revision, ...sample };
            throw new Error(`${request.name} cycle${cycle + 1} returned no useful result (stale/missing responses fail)`);
          }
        }
      }
      report.status = 'pass';
      for (const request of requests) {
        const result = report[request.name];
        result.p95_ms = [...result.samples_ms].sort((a, b) => a - b)[Math.ceil(result.samples_ms.length * .95) - 1];
        if (result.p95_ms > 100) report.status = 'fail';
      }
    } finally {
      fs.writeFileSync(reportPath, JSON.stringify(report, null, 2) + '\n');
    }
    if (report.status !== 'pass') throw new Error('Browser PERF-LSP exceeds100ms p95');
    console.log('Browser PERF-LSP:40 edits and40 useful responses each completion/signature/hover; p95<=100ms');
  } finally {
    if (browser) await browser.close();
    if (server.exitCode === null && !launchError) {
      server.kill();
      await Promise.race([exited, pause(5000)]);
      if (server.exitCode === null) { server.kill('SIGKILL'); await exited; }
    }
    await new Promise(resolve => log.end(resolve));
    fs.rmSync(state, { recursive: true, force: true });
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
