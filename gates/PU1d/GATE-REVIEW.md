# PU1d cross-family gate review, round 2 (DeepSeek V4 Pro via OpenRouter, one-shot curated bundle, 2026-11-24). Real cost 0.059 USD (24,580 prompt / 6,565 completion tokens).

Verdict : accepter

1. [mineur, confiance haute] Attaque A : aucune implémentation défectueuse plausible ne passe tous les critères. Les tests Go couvrent exhaustivement les règles P‑16/P‑17 (nommage, fermeture ciblée, réouverture sans événement) et les mutants MD1‑MD3 sanctionnent les retours en arrière. Aucune correction.

2. [mineur, confiance haute] Attaque B : chaque commande est saine – elle vérifie les codes de sortie et ne peut réussir à vide. Exemples : `go` liste les 7 tests par nom, `tla` dénombre exactement 10 ✓, `vectors` contrôle les digests et l’absence de `close_peer`. Aucune correction.

3. [mineur, confiance haute] Attaque C : le scénario netsim (crash) n’exerce pas le défaut (`ClosePeer` ne s’y déclenche pas) mais c’est acceptable pour un bloc « complete » car les tests Go et les mutants MD1‑MD3 le capturent ; un mutant e2e n’est pas requis. Aucune correction.

4. [mineur, confiance haute] Attaque D : les corrections de la revue initiale sont appliquées. La spec inclut la règle P‑16 avec `S7 OpeningNamed`, le modèle définit l’invariant correspondant, et le mutant MC3 (`NamesAtReturn`) est présent dans `CloseScopeMutants.tla`. Aucune correction.

5. [mineur, confiance haute] Attaque E : les timeouts tiennent dans `max_minutes` 45 (somme 2670 s). Les durées mesurées (TLA 139 s, Go 6 s, netsim 18 s, etc.) sont très inférieures aux caps ; aucun cap n’est trop étroit pour une implémentation correcte sous charge normale. Aucune correction.
Integrator check (Opus, same day): every criterion checks its exit code and names what it expects (exact ✓ count and lines for the model, exact vector digests and ids, named tests); the round reads as shallow, so the gate was also read in full before the freeze.
