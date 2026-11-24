# SB2b-1g cross-family spec and gate review (DeepSeek V4 Pro via OpenRouter, scripts/review-or.py one-shot curated bundle, 2026-11-24; Gemini quota exhausted). Real cost 0.041 USD (18,776 prompt / 5,016 completion tokens).

Verdict : accepter

1. [Mineur] S-11a n’ouvre pas de brèche en production (tier N). Le document ne peut écrire que sur les slots de sortie du tmpfs privé (ADR 0044 « Files », Landlock bloque toute autre écriture). tinymist n’a pas de slot d’écriture (`internal/language/language.go:137`). Les pages tmpfs sont comptées comme `shmem` (`mm/shmem.c:5214`, pas de `file_dirty`) et restent dans la mesure. Les menaces T1‑T5 sont écartées. (Confiance élevée, plan §2).

2. [Majeur] Les sources du noyau dans le plan §3 sont exactes : domaine writeback par memcg (`mm/memcontrol.c:3386‑3394`), `balance_dirty_pages` traite le seuil comme une limite douce (`mm/page-writeback.c:873‑880`), les pages en writeback sont libérées par les flushers (`mm/vmscan.c:1180‑1240`). Aucun sous‑comptage dangereux : en cas de device bloqué les pages dirty restent dans `memory.current` et déclenchent `memory.max` (R‑7). En tier N aucune page disque n’est modifiée, donc le scénario n’est pas atteignable. (Confiance élevée).

3. [Mineur] Le témoin `held_heap_slow_storage` (WITNESSES.md) et les mutants MD1‑MD7 prouvent la règle. Une implémentation cassée serait prise par MD4/MD5 (shmem/unevictable). Le saut du témoin lent sur GitHub est compensé par `local_storage`, le cas exact qui a tué la CI (gate `witness` et `github`). (Confiance élevée).

4. [Mineur] Le critère `github` exige 3 exécutions CI sur la branche candidate, première tentative. Avec une probabilité d’échec antérieur de ~1/2, 3 succès consécutifs sur première tentative laissent un risque de faux positif de ~12,5 %. Pour une garantie plus forte, 5 tentatives seraient préférables ; en l’état le correctif corrige la cause racine et la vérification est suffisante. (Confiance moyenne).

(1) Aucune garde supplémentaire au‑delà de `memory.max` n’est requise aujourd’hui ; le futur emplacement disque d’un plugin (AD‑57) est couvert par le résidu R‑7.
(2) Voir le constat 4 ci‑dessus : le risque de faux succès est faible mais non nul ; le correctif supprimant la cause, 3 exécutions sont raisonnables.
(3) L’absence de modèle formel est acceptable car la règle est une fonction pure d’un instantané ; modéliser la dynamique de reclaim aurait reproduit l’hypothèse fausse.
(4) Le saut du témoin `held_heap_slow_storage` sur GitHub (pas de délégation io) est voulu ; `local_storage` en est l’équivalent, documenté dans la porte.

Integrator: finding 4 applied before the freeze (five first-attempt GitHub runs).
