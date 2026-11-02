// Cold application process to usable editor, current server build, no plugins.
// Browser is prelaunched; OS caches are not evicted. Not a native window oracle.
const {chromium}=require('../../frontend/node_modules/@playwright/test');
const {spawn}=require('node:child_process');
const {performance}=require('node:perf_hooks');
const fs=require('node:fs');
(async()=>{
 const browser=await chromium.launch({headless:true}); const samples=[];
 try { for(let i=0;i<30;i++) {
  const page=await browser.newPage(); const start=performance.now();
  const child=spawn('./bin/inkwell-server',[],{env:{...process.env,INKWELL_SERVER_PORT:'39128',INKWELL_RENDEZVOUS:'off',INKWELL_HOME:process.cwd()+'/.task/perf/start-home',INKWELL_DOCS:process.cwd()+'/.task/perf/start-docs'},stdio:'ignore'});
  try {
   let ready=false;
   for(let j=0;j<300;j++){try{if((await fetch('http://127.0.0.1:39128/health')).ok){ready=true;break;}}catch{} await new Promise(r=>setTimeout(r,20));}
   if(!ready) throw Error('cold process did not become ready');
   await page.goto('http://127.0.0.1:39128'); await page.locator('.cm-content').waitFor();
   samples.push(performance.now()-start);
  } finally { const exited=new Promise(r=>child.once('exit',r)); child.kill(); await exited; await page.close(); }
 }
 fs.writeFileSync('docs/perf/baseline/2026-11-02/start.json',JSON.stringify({samples_ms:samples,p95_ms:[...samples].sort((a,b)=>a-b)[Math.ceil(samples.length*.95)-1],plugins:0,scope:'Current app server build: cold process to usable Chromium editor; browser prelaunched; warm OS caches; not native WebKitGTK window timing'},null,2));
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1});
