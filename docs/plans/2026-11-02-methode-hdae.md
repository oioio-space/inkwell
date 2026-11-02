# Méthode de pilotage : développement agentique dirigé par l'humain (HDAE)

Adoptée le 2026-11-02, d'après G. M. Kurtzer, *Ginux: Building an Independent Kernel That Runs Linux
Userspace through Human-Directed Agentic Engineering*, rapport technique, octobre 2026,
DOI 10.5281/zenodo.23682548.

## Deux boucles

- **Boucle extérieure (humaine)** : vision → architecture et contraintes → porte d'acceptation →
  revue du comportement observé → correction, nouvelle priorité ou acceptation.
- **Boucle intérieure (agent)** : implémenter → construire et lancer → mesurer et analyser les
  échecs → corriger → test de non-régression. Elle tourne seule jusqu'à ce que la porte passe ou
  qu'une décision humaine soit nécessaire.

## Règles reprises

1. **Documents durables hors conversation** : spécifications acceptées, décisions, registre des
   exigences, `docs/objectif-en-cours.md` (lot actif, porte, condition d'arrêt, journal de pilotage).
2. **Chaque lot se termine par une porte exécutable** (`mise run gate -- <lot>`), qui écrit un
   manifeste lisible par machine (`gates/<lot>.json`) : réussi / échoué par critère, versions des
   outils, empreintes des résultats bruts. Un message de réussite ne clôt jamais un lot ; le lot
   suivant réutilise le précédent dans un cadre plus réel.
3. **Boucle intérieure exécutable par l'agent** : l'agent lance lui-même compilation, tests, e2e et
   mesures (bac à sable Codex avec accès réseau local pour les e2e).
4. **Une longue tâche d'implémentation par lot** (Codex), revue par Claude à la porte ; Claude
   garde le jugement (revue, sécurité, conception).
5. **Performance** : entrées fixées, une modification à la fois, échantillons répétés, garder ou
   annuler (benchstat), résultats bruts archivés avec empreintes.
6. **Honnêteté** : chaque rapport de lot liste ses échecs connus et ses limites (menaces sur la
   validité) ; aucun échec masqué.

## Ce que l'on ne reprend pas

- Un seul agent sans revue indépendante : chez nous, toute fusion passe une revue (Claude, et
  Fable + Codex pour les spécifications).
- Les spécifications ne bougent plus pendant la construction, sauf décision consignée.
