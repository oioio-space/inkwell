# Mesures de la conception d'admission (2026-10-31)

Scripts exécutés avec Node 22.23.1, automerge 3.5.0 et automerge-repo 2.5.6 tels qu'installés dans `frontend/node_modules`. Ils sont reproduits ici tels quels, avec leur sortie, comme preuve de [06-admission-design.md](06-admission-design.md). Les copier dans un dossier vide, hors du dépôt, pour les relancer.

Variables : `AM=frontend/node_modules/@automerge/automerge/dist/mjs/entrypoints/fullfat_node.js`, `AR=frontend/node_modules/@automerge/automerge-repo/dist/entrypoints/fullfat.js`, `CBOR=frontend/node_modules/.pnpm/cbor-x@*/node_modules/cbor-x/index.js`.

Limites connues (revue Codex n°2, point 11) : un seul acteur qui ajoute en fin de texte ; les temps incluent des `clone` ; une seule exécution. Les chiffres valent pour cette charge seulement ; le banc répété sur un vrai `Repo` est prévu au plan (§8 de 06).

## reconnect.mjs

```js
// Reconnect cost: persisted sync state (sharedHeads only) vs fresh state (isEphemeral peers).
const A = (await import(process.argv[2])).next;
function build(n) {
  let d = A.from({ text: "" });
  for (let i = 0; i < n; i++) d = A.change(d, x => A.splice(x, ["text"], x.text.length, 0, "a"));
  return d;
}
function run(a, b, sa, sb) {
  let bytes = 0, msgs = 0, rounds = 0;
  for (;;) {
    let progressed = false;
    let m; [sa, m] = A.generateSyncMessage(a, sa);
    if (m) { bytes += m.byteLength; msgs++; progressed = true; [b, sb] = A.receiveSyncMessage(b, sb, m); }
    [sb, m] = A.generateSyncMessage(b, sb);
    if (m) { bytes += m.byteLength; msgs++; progressed = true; [a, sa] = A.receiveSyncMessage(a, sa, m); }
    if (!progressed) break;
    rounds++;
  }
  return { a, b, sa, sb, bytes, msgs, rounds };
}
for (const n of [1000, 10000, 100000]) {
  const base = build(n);
  let a = base, b = A.clone(base);
  // establish a synced pair
  let r = run(a, b, A.initSyncState(), A.initSyncState());
  const persistedA = A.encodeSyncState(r.sa), persistedB = A.encodeSyncState(r.sb);
  for (const extra of [0, 10]) {
    let a2 = A.clone(r.a); for (let i = 0; i < extra; i++) a2 = A.change(a2, x => A.splice(x, ["text"], 0, 0, "b"));
    const t0 = performance.now();
    const p = run(A.clone(a2), A.clone(r.b), A.decodeSyncState(persistedA), A.decodeSyncState(persistedB));
    const t1 = performance.now();
    const f = run(A.clone(a2), A.clone(r.b), A.initSyncState(), A.initSyncState());
    const t2 = performance.now();
    console.log(`changes=${n} new=${extra} persisted: ${p.bytes} B/${p.msgs} msgs/${(t1-t0).toFixed(1)} ms  fresh: ${f.bytes} B/${f.msgs} msgs/${(t2-t1).toFixed(1)} ms`);
  }
}
```

## sizes.mjs

```js
const A = (await import(process.argv[2])).next;
const { encode } = await import(process.argv[3]);
let d = A.from({ text: "" });
for (let i = 0; i < 2000; i++) d = A.change(d, x => A.splice(x, ["text"], x.text.length, 0, "abcde "));
const cur = A.getCursor(d, ["text"], 5000);
const name = "N".repeat(128);
const snap = { __presence: { type: "snapshot", state: { name, anchor: cur, head: cur } } };
const upd = { __presence: { type: "update", channel: "anchor", value: cur } };
console.log("cursor len", cur.length, "snapshot B", encode(snap).byteLength, "update B", encode(upd).byteLength);
// resident memory per loaded doc (10k-char text, 2000 changes)
const bytes = A.save(d);
global.gc(); const m0 = process.memoryUsage();
const docs = []; for (let i = 0; i < 100; i++) docs.push(A.load(bytes));
global.gc(); const m1 = process.memoryUsage();
console.log("saved KiB", (bytes.byteLength/1024).toFixed(1), "per loaded doc KiB (rss)", ((m1.rss - m0.rss)/100/1024).toFixed(0), "(heap+ext)", (((m1.heapUsed+m1.external+m1.arrayBuffers)-(m0.heapUsed+m0.external+m0.arrayBuffers))/100/1024).toFixed(0));
```

## mem2.mjs

```js
const A = (await import(process.argv[2])).next;
for (const [n, k] of [[1, 1000], [2000, 500]]) {
  let d = A.from({ text: "" });
  for (let i = 0; i < n; i++) d = A.change(d, x => A.splice(x, ["text"], x.text.length, 0, String.fromCharCode(97 + (i*7)%26)));
  const bytes = A.save(d);
  global.gc(); const r0 = process.memoryUsage().rss;
  const docs = []; for (let i = 0; i < k; i++) docs.push(A.load(bytes));
  global.gc(); const r1 = process.memoryUsage().rss;
  console.log(`changes=${n} saved=${bytes.byteLength}B docs=${k} rss/doc=${((r1-r0)/k/1024).toFixed(0)}KiB`);
}
```

## handles.mjs

```js
const R = await import(process.argv[2]); await import(process.argv[3]);
const repo = new R.Repo({ network: [] });
const N = 5000;
global.gc(); const m0 = process.memoryUsage();
const ps = [];
for (let i = 0; i < N; i++) {
  const url = R.generateAutomergeUrl();
  ps.push(repo.find(url, { allowableStates: ["unavailable", "ready", "requesting"] }).catch(e => e));
}
const res = await Promise.all(ps); console.log("first result:", String(res[0]).slice(0,120), res[0]?.state);
await new Promise(r => setTimeout(r, 200));
global.gc(); const m1 = process.memoryUsage();
const states = {}; for (const h of Object.values(repo.handles)) states[h.state] = (states[h.state]||0)+1;
console.log("handles", Object.keys(repo.handles).length, JSON.stringify(states), "heap/handle B", Math.round((m1.heapUsed-m0.heapUsed)/N), "rss/handle B", Math.round((m1.rss-m0.rss)/N));
const one = Object.keys(repo.handles)[0];
try { await repo.removeFromCache(one); } catch (e) { console.log("removeFromCache threw:", e.message); }
console.log("after removeFromCache of an unavailable handle, still cached:", one in repo.handles);
process.exit(0);
```

## Sorties

`node reconnect.mjs $AM` — coût d'une reconnexion, état de synchro gardé contre état neuf :

```
changes=1000 new=0 persisted: 148 B/2 msgs/0.8 ms  fresh: 1373 B/2 msgs/1.4 ms
changes=1000 new=10 persisted: 1552 B/4 msgs/7.6 ms  fresh: 3998 B/4 msgs/2.4 ms
changes=10000 new=0 persisted: 148 B/2 msgs/1.5 ms  fresh: 12623 B/2 msgs/7.6 ms
changes=10000 new=10 persisted: 1552 B/4 msgs/5.7 ms  fresh: 26498 B/4 msgs/11.7 ms
changes=100000 new=0 persisted: 148 B/2 msgs/38.3 ms  fresh: 125125 B/2 msgs/220.4 ms
changes=100000 new=10 persisted: 1562 B/4 msgs/56.8 ms  fresh: 251512 B/4 msgs/224.1 ms
```

`node --expose-gc sizes.mjs $AM $CBOR` — tailles de présence :

```
cursor len 37 snapshot B 265 update B 89
```

`node --expose-gc mem2.mjs $AM` — RSS par valeur Automerge chargée (pas un `DocHandle` complet) :

```
changes=1 saved=157B docs=1000 rss/doc=56KiB
changes=2000 saved=241B docs=500 rss/doc=309KiB
```

`node --expose-gc handles.mjs $AR $AR` — `DocHandle` « unavailable » d'un vrai `Repo` sans réseau, et `removeFromCache` :

```
first result: [object Object] unavailable
handles 5000 {"unavailable":5000} heap/handle B 28340 rss/handle B 66087
removeFromCache threw: DocHandle is not ready
after removeFromCache of an unavailable handle, still cached: true
```
