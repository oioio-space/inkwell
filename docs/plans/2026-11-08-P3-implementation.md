# Plan — ADR 0034, étape 3 : modèle d'abord (TLC), profil CBOR et schémas §9

Date : 2026-11-08. Porte figée : `gates/P3.toml`. Source : `docs/adr/0034-protocole-unifie.md`,
tableau « Migration », ligne 3 ; spec du protocole §9 (profil de validité CBOR, schémas) et §7.3
(modèle) ; `docs/protocol/tla/README.md`. Clôt **N-12**.

## Déjà là

- `docs/protocol/tla/RepoGen.tla`, `ConnCredit.tla` et leurs `.cfg`, vérifiés à la main avec TLC 1.8.0
  (`tla2tools.jar`, SHA-256 `011267fc…7e10cd`) ; résultats et mutations décrits dans le README.
- 45 vecteurs `step-3` (`repo.json`, décodeur `repo_message`).
- Java 25 sur le poste de développement.

## À faire

1. **`mise run tla`** : récupère `tla2tools.jar` à la version épinglée, refuse tout fichier dont le
   SHA-256 diffère (outil de développement, jamais embarqué), lance TLC sur `RepoGenSmall.cfg`
   (exhaustif, vivacité) et sur `ConnCredit` (exhaustif), puis chaque mutation du README : la tâche
   échoue si un modèle sain viole une propriété **ou** si un mutant passe. Mémoire bornée (`-Xmx`)
   et passage par `scripts/heavy.sh`.
2. **Profil CBOR (§9.2.1)** dans `@inkwell/protocol` : décodage strict (types et longueurs canoniques,
   pas de flottant, pas d'étiquette inconnue, clés de table ordonnées, tailles bornées par §16), et
   `encode(decode(b)) = b`. Propriété fast-check P1.
3. **Schémas §9** : un validateur par message ; vecteurs `repo.json` branchés dans les deux exécuteurs
   (Go `conformance`, TS `runConformance`) à `INKWELL_PROTOCOL_STEP=3`.
4. **Adaptateur réseau** de la webview branché sur le profil : `targetId` partout, encodeur conforme,
   pointeur en dix-millièmes entiers (aucun flottant sur le fil).

## Porte (rappel)

`P-TLA`, `P-VEC-3-go`, `P-VEC-3-ts`, `adapter-profile`, régression `P2` (qui inclut L3).

## Suivi (2026-11-08)

Le décodeur CBOR (`protocol/repo/cbor.go`, `@inkwell/protocol` `cbor.ts`) lit des octets de pairs :
il doit recevoir sa cible de fuzz (`FuzzRepoMessage`, corpus = vecteurs `repo.json`) au plus tard à
P-FUZZ (étape 9, 1 h sans plantage). Implémenté par un modèle gratuit (Nemotron Ultra via opencode) ;
diff relu (un test d'interface affaibli `innerText` → `textContent` rétabli) ; porte P3 verte.
