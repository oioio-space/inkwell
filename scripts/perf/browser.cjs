// Diagnostic only: attempt the real app; this does not certify a paint oracle.
const { chromium } = require("../../frontend/node_modules/@playwright/test");
const { spawn } = require("node:child_process");
const fs = require("node:fs");
(async () => {
  const out = "docs/perf/baseline/2026-11-02";
  const server = spawn("./bin/inkwell-server", [], {env:{...process.env,
    INKWELL_SERVER_PORT:"39127", INKWELL_RENDEZVOUS:"off",
    INKWELL_HOME:process.cwd()+"/.task/perf/home",
    INKWELL_DOCS:process.cwd()+"/testdata/perf/generated/project-1000"}});
  const log = fs.createWriteStream(out+"/server.log");
  server.stdout.pipe(log); server.stderr.pipe(log);
  let browser;
  try {
    for(let n=0;n<100;n++){
      try { if((await fetch("http://127.0.0.1:39127/health")).ok) break; } catch {}
      await new Promise(r=>setTimeout(r,200));
    }
    browser = await chromium.launch({headless:true});
    const page = await browser.newPage();
    await page.goto("http://127.0.0.1:39127");
    await page.locator(".cm-content").waitFor({timeout:30000});
    await page.locator(".cm-content").click();
    await page.evaluate(() => {
      window.perfEvents = [];
      window.perfLongTasks = [];
      new PerformanceObserver(list => window.perfEvents.push(...list.getEntries().map(e => e.toJSON())))
        .observe({type:"event", durationThreshold:16});
      new PerformanceObserver(list => window.perfLongTasks.push(...list.getEntries().map(e => e.toJSON())))
        .observe({type:"longtask"});
      window.slowLayout = false;
      document.addEventListener("keydown", () => {
        if (!window.slowLayout) return;
        const el = document.querySelector(".cm-content");
        const end = performance.now()+120;
        let n=0;
        while(performance.now()<end) {
          el.style.paddingLeft = (n++ % 2)+"px";
          el.getBoundingClientRect();
        }
      }, true);
    });
    const samples = [];
    for(let i=0;i<30;i++) {
      const before = await page.evaluate(()=>performance.now());
      await page.keyboard.type("x");
      await page.waitForTimeout(80);
      samples.push(await page.evaluate(start => ({
        start,
        events: window.perfEvents.filter(e=>e.startTime>=start),
        // Absence is censored by EventTiming's minimum threshold, never zero.
        textCharacters:document.querySelector(".cm-content").textContent.length
      }), before));
    }
    await page.evaluate(()=>{window.slowLayout=true; window.perfEvents=[];});
    await page.keyboard.type("z");
    await page.waitForTimeout(200);
    const control = await page.evaluate(()=>({
      events:window.perfEvents, longTasks:window.perfLongTasks,
      detected:window.perfEvents.some(e=>e.duration>=100)
    }));
    fs.writeFileSync(out+"/type-diagnostic.json", JSON.stringify({
      samples, slowLayoutControl:control,
      method:"30 warm Chromium inputs; EventTiming minimum threshold 16 ms, 8 ms granularity; not native WebKit, no certified exact p95"
    },null,2));
    if(!control.detected) throw new Error("Slow-layout control not detected");
    await page.screenshot({path:out+"/browser.png"});
    fs.writeFileSync(out+"/browser.json", JSON.stringify({engine:browser.version(),
      paint:await page.evaluate(()=>performance.getEntriesByType("paint").map(x=>x.toJSON())),
      warning:"Diagnostic only, not PM-2/PM-4 compliant"},null,2));
  } finally { if(browser) await browser.close(); server.kill(); log.end(); }
})().catch(e=>{ console.error(e); process.exitCode=1; });
