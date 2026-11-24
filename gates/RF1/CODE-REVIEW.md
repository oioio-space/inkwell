# RF1 cross-family code review (DeepSeek V4 Pro via OpenRouter, one-shot curated bundle, 2026-11-24, reasoning effort medium). Real cost 0.021 USD (34,208 prompt / 11,865 completion tokens).

**Verdict : refuser** (bloquant RF-6, part réservée contournable)

### **Constat 1** — [bloquant, confiance élevée] Un membre byzantin peut priver un membre honnête de sa part réservée (RF‑6).  
La fonction `room()` (`admit/ledger.go:231`) autorise une part réservée (`n < Share`) uniquement si `l.nMember < l.cfg.Members`. Un byzantin (jusqu’à 4 connexions par pair) peut remplir le plafond global de 128 avec d’autres byzantins, rendant impossible pour un membre légitime d’obtenir ses 2 connexions garanties.  
**Trace** : 63 byzantins ouvrent chacun 3 connexions → 128 saturé → un 64ᵉ membre honnête échoue avec `ReasonMembers`.  
**Correction** : isoler la part réservée de chaque membre dans le ledger (compteur individuel non soumis au total global).

### **Constat 2** — [majeur, confiance élevée] Épuisement des 64 connexions/IP sur le relais (RF‑7).  
`relayAdmitConfig()` (`admission.go:80`) n’impose ni deadline ni hiérarchisation : un étranger peut occuper indéfiniment les 64 places par IP, bloquant tout nœud légitime derrière le même NAT.  
**Correction** : ajouter une deadline ou un rate‑limit plus strict pour les connexions non identifiées du relais.

### **Constat 3** — [mineur, confiance moyenne] Mutant du balayage de buckets non vraiment équivalent.  
Le mutant de `now-l.swept >= l.cfg.Refill` (`ledger.go`sweep) peut empêcher le vidage des buckets pleins → croissance illimitée du map `buckets` (fuite mémoire/DoS). Les tests ne mesurent pas la mémoire sur la durée.  
**Correction** : ajouter un test de bornitude mémoire sur une charge longue.

### **Réponses aux questions d’attaque**

**A. Admission** — **oui**, le byzantin peut priver un membre (cf. Constat 1). Aucun trou pour les bornes par origine/taux (RFC‑1‑5) ; l’IPv6 et les circuits relayés sont correctement séparés.

**B. Comptabilité** — **pas de fuite**. Chaque sortie (fermeture, expiration, échec) libère la place ; pas de double remboursement (token payé par période). Les entiers `admit.Nanos` n’overflowent pas en pratique. Le wrapper sérialise bien les accès.

**C. Intégration libp2p** — les limiteurs par IP et taux de libp2p sont désactivés (`libp2pManager`, `node.go`), l’admission prenant le relais (`VerifySourceAddress` correct). Les limites transitoires et mémoire sont adaptées sans casser l’existant.

**D. Relais** — amplification/exhaustion pour les connexions/IP (Constat 2). Pas d’amplification au‑delà de 64 circuits/peer (RF‑7).

**E. Tests et mutation** — le mutant de balayage peut cacher une fuite mémoire (Constat 3). Les deux tests sur le resource manager réel (`TestNodeConnsBound`, `TestMemberShareKept`) sont acceptables et nécessaires.
## Round 2 (DeepSeek V4 Pro, one-shot curated bundle, reasoning capped at 14,000 tokens, 2026-11-24). Real cost 0.009 USD (27,128 prompt / 3,318 completion tokens); a first call at effort medium spent all 32,000 output tokens on reasoning and returned no text (0.117 USD).

Verdict : **accepter**

1. **Constat 1** — Bloquant, confiance élevée. La trace originale (63 byzantins hors roster volant les parts) est réfutée par `TestByzantinesCannotTakeShares` (ledger.go: `room` protège). Le vrai trou (membre ajouté après saturation) est corrigé par `SetRoster` + `shed` (ledger.go:124-156), validé par `TestRosterGrowthShedsSurplus` et les tests de choix. Pas de régression sur les parts réservées.

2. **Constat 2** — Majeur, confiance élevée. L’étranger non Allow-listé est maintenant refusé à la poignée de main (relay.go: `admit` dans le `gate`). `TestRelayUnlistedHoldsNoPlace` confirme. Le cas relais ouvert reste non protégé (RLY1 documenté, pas bloquant).

3. **Constat 3** — Mineur, confiance élevée. `TestBucketsBoundedUnderChurn` tue les deux mutants non équivalents (borne de map vérifiée). Plus de fuite mémoire.

4. **Question B (shed)** — Aucun trou nouveau. `shed` retire les connexions au-dessus de la part propre, jamais une connexion unique (`shed` >= share). Un changement de roster hostile n’est pas déclenchable depuis le réseau (Want piloté par l’application). Pas de thrash ni de perte de session unique (ledger.go:150-155, admission.go:201-213).

5. **Question C (handshake relais)** — Aucun bypass. Le `ConnectionGater` ferme la connexion avant réservation/circuit pour tout pair absent de l’Allow list, quel que soit le chemin (direct ou circuit). Le code `originOf` distingue bien circuit et direct (admission.go:217-233).