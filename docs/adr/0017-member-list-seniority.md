---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Liste de membres : DAG signé, ancienneté stricte (amende 0005)

## Contexte et problème

La première implémentation (« l'admin le plus ancien gagne », annulations collantes) a échoué
à l'audit de sécurité : un admin révoqué pouvait annuler sa révocation en antidatant, un
lecteur pouvait geler la liste, deux répliques pouvaient diverger, la synchronisation pouvait
s'emballer (preuves de concept exécutables).

## Options envisagées

- Corriger règle par règle l'algorithme à point fixe
- Quorum de délégués (Radicle) — impraticable hors ligne
- Capacités convergentes (Keyhive) — non livré, laisse coexister les révocations mutuelles
- **Ancienneté stricte en un seul balayage** (Matrix v12, Kleppmann, p2panda « strong removal »)

## Décision

Spécification complète : [plan trust-fold-v2](../plans/2026-10-25-trust-fold-v2.md).

- Rangs statiques : créateur = 0, puis ordre d'admission ; on n'agit que sur plus jeune que soi.
- Une coupe (retrait, rétrogradation) tue toute opération concurrente de sa cible, même après
  réadmission ; une révocation l'emporte sur une promotion concurrente.
- Seuls des admins (dans le passé causal de l'op) peuvent écrire dans la liste.
- 1024 opérations : liste **scellée**, refus par défaut.
- Synchronisation : jamais d'annonce en réponse à une annonce, budget par pair.

### Conséquences

- Bien : convergence démontrable, coût en millisecondes, attaques de l'audit impossibles.
- Moins bien, assumé :
  - **le créateur ne peut pas être révoqué** — perte de son appareil = nouveau projet ;
  - un ancien admin compromis peut **sceller** la liste par antidatage (déni de service qui
    échoue fermé, jamais une prise de contrôle) — réévaluer avec Keyhive ;
  - l'ancienneté survit à la rétrogradation.

### Confirmation

Tests de non-régression issus des preuves de concept, test de propriété contre une
implémentation de référence naïve, nouvel audit Opus.
