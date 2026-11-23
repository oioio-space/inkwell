# PC1b2 cross-family code review (DeepSeek V4 Pro via OpenRouter, 2026-11-23; Gemini quota exhausted). Real cost 0.46 USD.

## Verdict : accepter

Aucun défaut bloquant ni majeur. Une seule remarque mineure.

**1. `internal/transport/p2p/dial.go` — `errConnectInFlight` (mineur, confiance moyenne)**
Le mécanisme est correct : `Connect` incrémente `connecting[id]` sous `n.mu` avant `h.Connect` (node.go:863) et `connectDone` décrémente toujours (defer, node.go:865) ; `dial` refuse et saute `ClearAddrs` (dial.go:358-365) — l'ordre incrément/clear est sûr sous `n.mu`. Pas de famine : backoff borné à 1 min, `kickAll` sur changement d'adresses. Pas de fuite : timers `Stop`és, goroutines liées à `n.ctx`.
Nit : `errConnectInFlight` tombe dans la branche d'échec (dial.go:229-236), donc double le backoff comme une vraie panne ; et un `Connect` qui *réussit* ne réveille pas la boucle en pause (`onConnected` ferme `chg`, pas `kick` — stream.go:65). La boucle peut donc attendre jusqu'à `maxBackoff` avant de voir la connexion. Auto-correctif, sans conséquence fonctionnelle.

**2. J-7 / `approveLocked` — aucun défaut**
La ligne `msg=restarting change=… folder=…` est émise une seule fois, avant `d.restart` (startup.go:183), après `finish` (donc `joined` déjà posé). `finishHeld` consomme `confirm` au drain irréversible (change.go:147) ; sur refus : `held=nil`, `requested=false` (change.go:149-150) — pas de redémarrage sans approbation, la session ré-accepte. Drain réussi puis échec → `requested` reste vrai : correct (flux fermés, N-16). `hold`/`cancelHeld`/`finish` bien sérialisés par `c.mu`.

**3. J-8 — aucun défaut**
`state()` = expiré → outstanding → utilisé (invites.go:280-286) ; `Outstanding` compare en temps constant après `purgeLocked` (flow.go:197-208). Le dialogue n'affiche/copie qu'après réponse Go (`checked`, ShareDialog.svelte:83-102) et seulement si `invitationState === "open"` ; une réponse obsolète est ignorée par identité de texte (workspace.svelte.ts:1054). Un ticket encore détenu répond toujours « open », un ticket consommé « used » pour de bon.

**4. J-9 — aucun défaut**
`CompleteJoin` réécrit RootURL+Previous+PendingJoin=nil en un seul `saveLocked` atomique (project/join.go:93-107). Le retour revalide via `Prefer`→`IsProject` ; dossier disparu → `errNoPrevious` ou échec d'écriture → repli sur le dossier qui attend (`setJoined("")`). Pas de boucle : `Previous` n'existe que pour un dossier né d'une jointure.

**5. Octets pairs / fuites — rien**
Les ops du Welcome passent par `trust.List.ApplyAll` (flow.go:559) puis `Pending()==0` ; la liste stockée est la liste vérifiée. Aucune fuite goroutine/timer repérée.

The minor remark (errConnectInFlight backoff) is filed → TX1.
