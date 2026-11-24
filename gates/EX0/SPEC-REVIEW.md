# EX0 cross-family review of the ADR, plan and gate (DeepSeek V4 Pro via OpenRouter, scripts/review-or.py, 2026-11-24). Real cost 0.024 USD (14,114 prompt / 8,445 completion tokens).

**Verdict : accepter avec corrections**

**Constatations numérotées (sévérité, confiance, preuve, correction)**

1. **Sévérité élevée – Confiance moyenne** : Pollution prototype possible via les clés `__proto__`, `constructor`, `prototype` dans les manifestes JSON. La validation « stricte, clés inconnues refusées » (ADR 0027 l. 112) n’interdit pas ces clés dangereuses si elles étaient acceptées comme « connues ».  
   **Correction** : Dans `manifest.ts`, rejeter explicitement tout objet contenant ces clés, même pour les nœuds imbriqués.

2. **Sévérité élevée – Confiance faible** : Un manifeste de plugin pourrait nommer un chemin d’entrée (`entry`) pointant vers un module interne (ex. `../../../internal/bridge`). Le mécanisme de résolution pour les plug-ins futurs n’est pas confiné par l’ADR, seule l’origine `builtin` est protégée (l. 106‑107).  
   **Correction** : Spécifier que le *loader* refuse toute `entry` extérieure au répertoire de l’extension pour les origines `plugin`/`project` ; l’écrire dans l’ADR et le test R‑5.

3. **Sévérité moyenne – Confiance élevée** : Le critère de performance `PERF-EXT-VIEWER-CSV` (200 ms p95) est ambigu. Le plan mentionne « fresh window each » (plan l. 60), ce qui pourrait inclure le démarrage complet de l’application et rendrait le budget irréaliste.  
   **Correction** : Clarifier que la mesure porte uniquement sur l’activation + premier rendu dans une application déjà chargée ; détailler le scénario dans `ex0.spec.ts` (ouverture d’un fichier CSV depuis le panneau de fichiers).

4. **Sévérité faible – Confiance élevée** : La détection statique `no-lists` (gate l. 53‑56) ne repère pas les listes d’extensions utilisant des constantes ou des expressions calculées.  
   **Correction** : Ajouter une règle de style (Biome) interdisant les *maps* associant des extensions à des actions en dehors de `src/extensions`. Le risque résiduel est couvert par les tests d’intégration.

5. **Sévérité faible – Confiance élevée** : Le passage du flux Go en v2 sans octet `kind` est sûr car le flux est local au couple Go/webview ; le risque inter‑pairs est inexistant, Go ne portant que des octets opaques (ADR 0027 l. 165). Si le flux était persisté dans le document Automerge, une rétrocompatibilité serait nécessaire.  
   **Correction** : Vérifier que le flux n’est pas inclus dans le CRDT ; sinon, prévoir une migration.

6. **Sévérité faible – Confiance moyenne** : La porte valide uniquement la présence et le succès de tests nommés (R‑1…R‑9), pas la pertinence de leurs assertions. Une implémentation vide pourrait passer.  
   **Correction** : La revue *simplify* (EX0/SIMPLIFY.md) doit inclure une vérification manuelle des corps de test avant le gel.

**Questions ouvertes – une ligne par question**

- **Validateur** : Garder le validateur TypeScript dédié (pas ajv, qui exigerait `new Function` interdit par la CSP) ; le générer éventuellement depuis un schéma hors-ligne pour la maintenabilité.
- **Changement Go** : Le garder dans EX0, car la suppression du `Kind` est une conséquence directe du registre ; un bloc séparé ne se justifie pas.
- **Exception `native/` à FE‑3** : Acceptable temporairement pour EX0, à condition de limiter cette exception aux extensions `essential` et de planifier son remplacement par des capacités explicites (§15.20).
- **Règle du visualiseur par défaut** : L’ADR est clair (visualiseur de l’extension propriétaire du type) ; ajouter un test pour le cas où l’extension ne fournit pas de visualiseur (fallback vers le visualiseur essentiel du contenu).
- **Mesure 200 ms en e2e** : Pertinent, à condition d’utiliser l’API `performance.getEntriesByName` dans Playwright et de réserver un créneau exclusif (`heavy.sh --exclusive`).
