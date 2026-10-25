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
- L'absence d'un admin ne bloque jamais l'édition (elle ne dépend pas de la liste en ligne) ;
  seule la gestion des personnes demande un admin présent.
- Une coupe (retrait, rétrogradation) tue toute opération concurrente de sa cible, même après
  réadmission ; une révocation l'emporte sur une promotion concurrente.
- Seuls des admins (dans le passé causal de l'op) peuvent écrire dans la liste.
- 1024 opérations : liste **scellée** ; seules les modifications de la liste de membres sont
  refusées, la lecture des rôles et la vérification des attestations continuent (l'édition n'est
  jamais bloquée).
- Tout départ volontaire (retrait ou rétrogradation de soi) exige un autre admin dans le passé
  causal de l'opération, et neutralise les opérations antidatées du partant.
- Synchronisation : jamais d'annonce en réponse à une annonce, budget par pair.

### Conséquences

- Bien : convergence démontrable, coût en millisecondes, attaques de l'audit impossibles.
- Moins bien, assumé :
  - **personne ne peut révoquer le créateur contre son gré** ; en revanche il peut **passer la
    main** (se retirer ou se rétrograder) dès qu'un autre admin existe — indispensable en
    entreprise, où les gens partent. Perte de son appareil sans autre admin = nouveau projet,
    d'où l'invitation insistante à nommer un second admin dès le premier partage. Après la passation,
    une fuite ultérieure de la clé de l'ancien créateur ne permet ni de prendre le contrôle ni
    d'antidater : ses opérations concurrentes à son départ sont mortes ;
  - un ancien admin compromis peut **sceller** la liste par antidatage : la gestion des membres
    est alors gelée (jamais une prise de contrôle), l'édition continue, et les répliques peuvent
    diverger sur la liste selon l'ordre d'arrivée — il faut recréer un projet pour gérer à
    nouveau les membres ; l'interface doit le dire — réévaluer avec Keyhive ;
  - l'ancienneté survit à la rétrogradation, mais un senior qui n'est plus admin (rétrogradé,
    ou ancien créateur après passation) peut être retiré ou rétrogradé par un plus jeune admin ;
    on ne peut jamais promouvoir ni renommer un senior ;
  - des départs simultanés (le créateur et le dernier autre admin) peuvent ne laisser aucun
    admin : la gestion des membres est gelée, l'édition non.

### Confirmation

Tests de non-régression issus des preuves de concept, test de propriété contre une
implémentation de référence naïve, nouvel audit Opus.
