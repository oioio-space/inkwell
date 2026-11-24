# SB2b-1g cross-family code review (DeepSeek V4 Pro via OpenRouter, scripts/review-or.py, 2026-11-24). Real cost 0.042 USD (12,565 prompt / 10,179 completion tokens).

Verdict : accepter

1. [Aucun, confiance élevée] S-11a est exacte. `internal/sidecar/memory_linux.go:160-166` ne lit que `file`, `shmem`, `unevictable`; `file_dirty`/`file_writeback` sont donc laissés dans `clean`. Formule : `kept=shmem+unevictable`, `clean=file-min(kept,file)`, `return current-min(clean,current)`. Traces : `current=200M, file=190M, dirty=100M, writeback=100M` → mesure `10M`; `shmem=190M` → `200M`; clé manquante → erreur, donc aucune décision de kill (test “no stat”). Pas de sous-dépassement grâce aux deux `min`. Aucun autre chemin de calcul de la mesure dans le diff.

2. [Aucun, confiance élevée] Le skip `-race` est acceptable. `internal/sidecar/bound_linux_test.go:198-205` ne saute que `held_heap_slow_storage` quand `wbps>0 && raceEnabled`; `local_storage` reste exécuté. La porte et MD2 exécutent sans `-race` et interdisent le skip; l’ancienne règle y tue le témoin 5/5, donc une implémentation cassée est toujours prise. MD1/MD3 attrapent aussi dirty/writeback unitairement. L’ancien test dirty 512 MiB n’est pas affaibli : il devient `local_storage`, non skippé.

3. [Mineur, confiance moyenne] Couverture mutante suffisante : dirty/writeback MD1/MD3, règle réelle MD2, shmem MD4, unevictable MD5, compteur manquant MD6, file entier MD7. Le `min(kept,file)` n’est pas muté et le cas `kept>file` n’a plus de ligne unitaire (l’ancien « more kept than cache » a été retiré). Non bloquant : le code sature correctement sans sous-dépassement. Correctif recommandé : ajouter une ligne `shmem/unevictable > file` dans `TestUnreclaimableLeavesOutOnlyFileCache`. Collision MD1-MD3 avec PU1d : aucun risque, la sélection est par bloc.

4. [Aucun] Rien ne bloque le merge.

Integrator: finding 3 applied — the removed « more kept than cache » row is restored, with unevictable pages (the only kept pages above the cache under S-11a); it passes, and fails when its expectation is altered.
