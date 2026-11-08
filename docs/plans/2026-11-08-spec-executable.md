# Méthode : tout spécifier en exécutable, puis implémenter avec des modèles légers

Date : 2026-11-08. Décision de l'utilisatrice : « définir tous les trucs comme des vecteurs ou du TLA
pour que même des modèles simples y arrivent ; bien spécifier, être sûr que ça va fonctionner ; ensuite
l'implémentation peut être faite par des modèles plus légers ». Crédit OpenRouter : ~9,40 $, sans
recharge ; à n'utiliser qu'à très bon escient.

## Constat qui fonde la méthode

Les étapes 2 et 3 d'ADR 0034 ont été implémentées par des modèles gratuits (DeepSeek V4.1 Flash,
Nemotron Ultra) et validées, parce que tout y était **exécutable** : vecteurs étagés, modèle TLA+, porte
figée. Les erreurs vues ailleurs (agent qui n'écrit rien, test affaibli `innerText` → `textContent`,
retouches hors périmètre, affirmations non vérifiées) ont toutes été arrêtées par une machine ou par la
relecture du diff. La concurrence (6 courses trouvées dans `amSync` par Opus alors que tous les tests
passaient) est ce qu'un modèle léger ne voit pas : elle doit devenir exécutable avant le code.

## Les quatre formes de spécification exécutable

1. **Vecteurs** (`protocol/vectors/*.json`, étagés par `enforce`) : entrée → verdict, lus par les deux
   exécuteurs (Go `protocol/conformance`, TS `@inkwell/protocol`).
2. **Modèles TLA+** (`docs/protocol/tla/`) vérifiés par `mise run tla`, avec des **mutants** qui doivent
   échouer (preuve que le modèle mord).
3. **Traces de modèle → vecteurs** : TLC produit des comportements (suites d'états et d'actions) ; un
   générateur les écrit en vecteurs, rejoués sur l'automate Go pur. Le code est ainsi vérifié contre le
   modèle formel lui-même, pas seulement contre des exemples choisis.
4. **Tests à modèle** (`rapid` `StateMachine`, fast-check `commands`) : un modèle de référence pur et
   minuscule ; l'implémentation doit produire les mêmes observations sur des suites d'opérations
   aléatoires, avec rétrécissement des contre-exemples.

## Trous à combler, étape par étape (avant leur implémentation)

| Étape | Déjà exécutable | À écrire d'abord |
|---|---|---|
| 4 `sync2`, règles I/B/A/R/T/W | 47 vecteurs de codec (`compat.json`, `session.json`) ; TLA+ `RepoGen` | générateur de traces `RepoGen` → vecteurs `rule_trace` ; automate Go pur des règles qui les rejoue |
| 5 boucles non bloquantes, vrac | 2 vecteurs (`bulk+expect`) | tests à modèle : P3 non-blocage (un pair lent n'arrête personne), persistance avant diffusion, P8 parts réservées ; vecteurs de tailles adaptatives |
| 6 pont, crédits, règles L/H, LARGE | 5 vecteurs (`bridge_envelope`) ; TLA+ `ConnCredit` | modèle du crédit A.1 (TLA+ ou `rapid`) et ses traces ; règles L/H ; objet LARGE §11.2 (P6 : exactement `size` octets, réservés avant lecture) |
| 8 SET et objets ADR 0031 | 25 vecteurs d'objets | modèle du curseur SET (convergence, bornes) ; propriété P7 (pli identique dans tout ordre) |

## Répartition des modèles

| Travail | Qui | Pourquoi |
|---|---|---|
| Écrire les spécifications exécutables (vecteurs, TLA+, modèles de référence, générateurs de traces) | Claude Opus (session principale) | jugement, sécurité, concurrence |
| Relecture adversariale d'une **spécification** de sécurité ou de concurrence | Opus 5.5 via OpenRouter, un seul appel ciblé (< 1 $) | seul contrôle non mécanisable ; budget compté |
| Implémentation jusqu'à ce que la porte passe | modèles gratuits via `scripts/opencode-free.sh` (Nemotron Ultra, North Mini Code ; DeepSeek Zen si quota) | tout est tranché par la porte |
| Travail mécanique (format, docs, recherche dans le code) | sous-agents Haiku, ou modèle gratuit | coût minimal |
| Vérification de chaque diff | Claude (session principale) | périmètre, tests intacts, affirmations vraies |

## Règles de vérification d'un diff produit par un modèle léger

1. Périmètre : aucun fichier hors de ceux que nomme le plan de l'étape (sinon justifié ou retiré).
2. Tests : aucune assertion existante affaiblie (`innerText` → `textContent`, seuil élargi, `skip`,
   tolérance ajoutée) ; les nouveaux tests échouent sur l'ancien code.
3. Affirmations du résumé : chacune revérifiée par une commande (« test instable », « déjà fait »…).
4. Porte complète relancée par Claude avec la commande officielle, régressions comprises.

## Budget OpenRouter (~9,40 $)

Uniquement des relectures en un appel d'Opus 5.5 sur une spécification (pas sur du code généré, que
les portes jugent) : sécurité (étapes 4 et 6 : préambules, crédits, règles L/H), et la spec du SET.
Estimation : 4 à 6 relectures, ~1 $ chacune ; le reste en réserve. Jamais d'agent payant en boucle.
