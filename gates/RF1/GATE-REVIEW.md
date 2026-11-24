# RF1 cross-family review, round 2 (DeepSeek V4 Pro via OpenRouter, one-shot curated bundle, 2026-11-24). Real cost 0.011 USD (26,615 prompt / 4,754 completion tokens); a first call at reasoning effort high spent all 32,000 output tokens on reasoning and returned no text (0.116 USD).

Verdict : accepter

1. **[mineur, confiance élevée] Correcte application des constats du round 1.**
   - Constat 1 (modèle token bucket) : les dettes et le remboursement exact dans `RelayFanout.tla` (l. 124-137) modélisent le seau ; le mutant MF13 (fenêtre fixe) est rattrapé par `RateBound` (l. 272).  
   - Constats 2‑3 (taux et quota) : les scripts `gates/RF1/rf1_wave.py` et `rf1_recover.py` calculent les temps de reprise (plan §4) ; l’origine relais est relevée à 16 (plan §3), les IP restent à 8.  
   - Constat 4 (régression) : la commande `regression` de `gates/RF1.toml` (l. 271) exécute tout le module `protocol` (`go test ./...`).  
   Aucune correction supplémentaire nécessaire.

2. **[mineur, confiance élevée] Nouvelle surface d’attaque (part réservée de 2 par membre, capacité 4).**
   - L’invariant `SharesKept` (RelayFanout.tla l. 278) garantit qu’un membre byzantin ne consomme jamais la part d’un autre.  
   - Les appels sortants (`XDial`) ne sont pas limités par le quota d’origine, donc un étranger ou un byzantin ne peut empêcher un membre honnête d’être joint (`OutNeverRefused` l. 285, `MembersEventuallyUp` l. 289).  
   - Les 184 MiO QUIC tiennent dans le budget de 256 MiO (plan §3).

3. **[mineur, confiance élevée] Robustesse de la porte.**
   - Les mutants Go MG1‑MG6 et le test `TestLedgerModel` (modèle de référence `rapid` d’`admit`) détecteraient une implémentation rompue.  
   - Aucune commande vide ; la somme des caps (3630 s) est inférieure à `max_minutes 61` (3660 s).  
   - Le critère `regression` est actuellement rouge à cause d’une dérive PU1d extérieure (WITNESSES.md l. 40‑46), documentée et à corriger hors RF1.

**Points ouverts**  
- Résiduel [RF1] (inondation via un relais public) : acceptable ; le `Allow` ferme la brèche et les chemins directs restent opérationnels.  
- Excédent de 2 connexions à 64 membres (attente ≤ 5 s pour fermer la connexion morte) : acceptable (plan §8).