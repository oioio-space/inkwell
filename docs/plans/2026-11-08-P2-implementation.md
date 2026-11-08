# Plan — ADR 0034, étape 2 : N-1, signatures et clés strictes (`protocol/sig`)

Date : 2026-11-08. Porte figée : `gates/P2.toml` (écrite avant l'implémentation).
Source : `docs/adr/0034-protocole-unifie.md`, tableau « Migration », ligne 2 ; spec du protocole
§3.2–§3.3 (`docs/protocol/`).

## But

Un seul point de vérité pour accepter une clé et une signature Ed25519, appelé partout où un octet
venu d'un pair est vérifié. Clôt **N-1**.

## Déjà fait (vérifié le 2026-11-08)

- Module `protocol/` (étape 1, `b4b1f91`).
- `protocol/sig.ValidatePublicKey` : encodage canonique, point sur la courbe, ni petit ordre ni
  ordre mixte.
- `conformance.Run(step, check)` sait étager les vecteurs par `enforce` ; le test actuel tourne à
  l'étape 1 **sans décodeur** (`check` nul) : il ne vérifie que la forme des fichiers.

## Reste à faire

1. **Vérification stricte** dans `protocol/sig` : `Verify(key, msg, sig)` qui refuse un `S` non
   réduit (`S ≥ L`), un `R` ou une clé non canoniques ou de petit ordre, et applique l'équation
   cofactorisée choisie par la spec §3.2 (à relire avant d'écrire). Vecteurs : `crypto.json`,
   `speccheck-0` à `speccheck-11` (« Taming the many EdDSAs »), `noncanonical-public-keys`,
   `off-curve-public-key`.
2. **Conformité Go** : `TestCorpus` lit `INKWELL_PROTOCOL_STEP` (1 par défaut) et passe un
   `check` qui route chaque `decoder` (`public_key`, `signature+key`, …) vers `protocol/sig`.
3. **Conformité TypeScript** : `runConformance` de `@inkwell/protocol` fait de même, avec une
   vérification stricte équivalente (bibliothèque auditée, à choisir et justifier, par exemple
   `@noble/curves` `ed25519` en mode `zip215: false`).
4. **Appelants N-1** : filtre de connexion (`p2p`), `KeyFromID`, ops, attestations, décisions,
   ticket, rechargement et import passent par `protocol/sig`. Le travail de la branche `n1`
   (`fix/n1-small-order-keys`) s'y range, plus la règle d'ordre premier.
5. **Fuzz** : `FuzzVerify` dans `protocol/sig`, corpus = vecteurs.

## Porte (rappel)

`P-VEC-2-go`, `P-VEC-2-ts`, `N-1-callers`, `P-FUZZ-sig` (120 s ici ; 1 h à l'étape 9), `P-ARCH`,
régression `L3`. Revue indépendante (Codex ou Opus) avant fusion.
