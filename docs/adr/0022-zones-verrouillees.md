---
status: proposed
date: 2026-10-25
decision-makers: [Mathieu]
---

# Zones verrouillées : verrous dans le document, admission déterministe, compensation des concurrents

## Contexte et problème

Seul un admin pose ou lève un verrou sur une plage de texte. Un écrivain ne peut rien y changer : il
est bloqué chez lui, et une modification forgée est refusée par les pairs. Il peut proposer un
changement.

Trois contraintes compliquent cela :
- le texte est un DAG Automerge : on ne retire pas une modification sur laquelle d'autres ont bâti ;
- Go ne lit pas Automerge (ADR 0011) ;
- T10 (ADR 0018) ne lie pas l'acteur Automerge à une clé.

## Facteurs de décision

- Verdict identique partout : une fonction pure de la modification et de son passé causal
  (Kleppmann 2022), jamais de l'ordre d'arrivée.
- Ne jamais refuser ce qu'un pair honnête a pu prendre et étendre : sinon les répliques divergent.
- Coût par frappe négligeable ; aucun coût O(document) en régime normal.
- Ne pas peser sur la liste signée (1024 op, scellement).

## Options envisagées

**Où vit le verrou :**
- **A. Op `lock` / `unlock` dans la liste signée.** Elles ne sont pas causalement reliées au texte :
  impossible de dire si une modification « a vu » le verrou. Elles consomment aussi le budget (X-08
  seul : plus de 20 op), et un scellement gèlerait les verrous.
- **B. Entrées du document, protégées par l'attestation d'un admin.** La pose est une modification
  Automerge, donc « vu ou pas vu » est exact.

**Application chez les pairs :**
- **C1. Rejet** de toute modification qui touche la zone. Les répliques divergent : celui qui
  ignorait le verrou a accepté et bâti dessus.
- **C2. Compensation seule**, après coup. Les violations délibérées passent.
- **C3. Mixte** : rejet déterministe de ce qui descend du verrou ; ce qui lui est concurrent est admis
  puis compensé.

## Décision

B et C3.

**Données** (par document texte : un verrou vit dans le fichier qu'il protège).

| Entrée | Contenu | Écrite par |
|---|---|---|
| `meta.locks[id]` | `{start, end, by, at, note?}` ; `start` et `end` = `getCursor(text, i, "after")` sur le premier et le dernier caractère | admin |
| `meta.unlocks[id]` | `{by, at}` | admin |
| `meta.suggestions[sid]` | `{lock, start, end, text, by, at}` | écrivain |
| `meta.decisions[sid]` | `{verdict, by, at}` | admin |
| `meta.compensations[hash]` | `{by, at}` | admin |

- **Entrées immuables.** On crée, on ne modifie ni ne supprime :
  - reverrouiller crée un nouvel `id` ;
  - réancrer, c'est lever et reposer dans une même modification. Un admin qui réécrit une zone
    (revenir, X-06) réancre ainsi.
- **Verrou actif pour c** : sa création est dans le passé de c, et sa levée n'y est pas. Le test se
  fait avec `getChangesMetaSince(doc, c.deps)`.
- **Zone à l'état X.**
  - Bornes : `s = pos(start)` et `e = pos(end) + (vivant(end) ? 1 : 0)`. Un curseur k est vivant si
    `getCursor(pos(k), "after") === k`.
  - La zone ne grandit d'aucun côté (LCK-03).
  - Supprimer une ancre la rétrécit.
  - Tout supprimer donne `s = e`, un « verrou vide » signalé à l'admin (X-07).
  - Comportement vérifié sur Automerge 3.5.0.

**Admission** (webview, après la couverture T10 ; ce sont des fonctions pures de c et de son passé).
- **R1.** Toute création sous `meta.locks`, `unlocks`, `decisions` ou `compensations` exige que c soit
  couverte par l'attestation d'un **admin**. Sont refusés pour tous :
  - modifier ou supprimer une entrée existante ;
  - remplacer `meta` ou `text`.
- **R2.** Si un verrou est actif pour c, c exige la couverture d'un admin dès qu'elle, à l'état
  `c.deps` :
  - insère strictement dans `(s, e)` ;
  - supprime dans `[s, e)` ;
  - ou pose une marque qui chevauche la zone.
- **Calcul.**
  - Les effets de c : `diff(doc, c.deps, [c.hash])`, mesuré à environ 1 µs.
  - La zone à `c.deps` est la zone aux têtes admises H (curseurs courants, environ 60 µs),
    transportée par `diff(doc, H, c.deps)`. Ce dernier est vide pour une frappe à jour.
  - Le transport est exact si les ancres sont vivantes en H : elles le sont alors à `c.deps`, et un
    élément vivant se transporte sans ambiguïté.
  - Si une ancre est morte en H : repli exact par `getCursorPosition(obj, cursor, c.deps)` du backend.
    Ce repli coûte O(document) (environ 8 ms à 50 000 caractères, 150 ms à 1 M). Il est rare, et
    budgété par pair.
  - Document d'essai : `clone(doc)` (0,3 ms), sur lequel on applique les candidats, puis on le jette.
- **Couverture admin.** C'est le graphe de T10 restreint aux signataires admins, connus par le nouveau
  kind de pont `Roles`.
  - Les têtes déjà reçues d'un admin rétrogradé ou suspendu sont **gardées localement** : on ne
    retire jamais après coup ce qui a été admis.
  - Un arrivant compte sur la réattestation des admins courants.
  - Cas résiduel : la modification de zone d'un admin rétrogradé, avant qu'aucun autre admin ne l'ait
    reçue. Un admin courant la ratifie alors en un clic.
- **Fragment de document complet.** On ne contrôle que les modifications non couvertes par un admin.
  Une seule modification refusée fait refuser le fragment entier, comme avec T10.

**Modifications concurrentes au verrou** (LCK-10, X-13, antidatage, fusion d'une variante née avant
le verrou).
- Elles sont **admises partout**. Les refuser ferait diverger les répliques : on ignore qui les a
  déjà prises, et l'acteur d'un écrivain chaîne toutes ses modifications suivantes à celle-là.
- Elles sont signalées (« modifiée pendant la pose du verrou »).
- **L'appareil qui a posé le verrou les compense.** C'est le seul compensateur automatique, pour
  éviter les réinsertions en double. Une seule modification contient :
  - un inverse ciblé, restreint à la zone (undo par opérations inverses, ADR 0002) ;
  - une suggestion contenant le texte écarté ;
  - l'entrée `compensations[hash]`.
- Si cet appareil n'est plus admin, tout admin peut cliquer « Rétablir la zone ».

**Client honnête.**
- On utilise `EditorState.transactionFilter`, et non `changeFilter`, qui découperait une
  suppression englobante.
  - Toute spécification de changement qui touche une zone est retirée **entière** (LCK-04). Les autres
    passent (LCK-05).
  - Sont exemptées les transactions distantes et celles des admins.
- Le filtre lit `handle.doc()` dans le même tour synchrone que `handle.change`. Un verrou distant ne
  peut donc pas s'intercaler. Sinon l'écrivain serait coincé, puisque toutes ses modifications
  suivantes dépendraient de celle-là.
- Trois chemins produisent des modifications « dans le passé » :
  - le `changeAt` des intégrations disque (B1, X-15) ;
  - revenir à une version (X-06) ;
  - la fusion d'une variante (X-11).

  Ils filtrent selon les zones actives **aux têtes courantes**. Ce qui touche une zone part en
  suggestion.
- **Fusion d'une variante.**
  - Par un admin : `handle.merge`.
  - Par un écrivain, si la variante touche une zone active : fusion textuelle hors zone, plus une
    suggestion.
- **Chevauchement (X-10).**
  - À la pose, l'admin fusionne en un seul verrou (l'enveloppe) et lève les anciens dans la même
    modification.
  - Deux poses concurrentes coexistent : on applique l'union, et lever l'un laisse l'autre.
  - Tout admin lève tout verrou. L'ancienneté ne protège rien ici, puisque tout admin peut déjà écrire
    dans la zone.
- **Suggestion (LCK-09).**
  - Accepter : l'admin applique le texte et écrit `decisions[sid] = accepted`.
  - Refuser : `refused`, et la suggestion est masquée.

### Conséquences

**Bien :**
- verdict déterministe ;
- aucune divergence par dépendance ;
- quelques µs par frappe ;
- la liste signée reste intacte ;
- un retour en arrière du texte ne touche pas les verrous.

**Moins bien, assumé :**
- Une modification concurrente ou antidatée reste visible jusqu'à sa compensation, qui attend
  l'appareil propriétaire ou un admin. Toute garantie plus forte exigerait un consensus, que Keyhive
  écarte aussi.
- La fusion textuelle d'un écrivain perd le lien d'historique avec la variante.
- `by` est une déclaration d'un écrivain courant, pas une preuve d'auteur.
- Le repli coûte O(document) quand une ancre est morte.
- Une ratification manuelle reste nécessaire dans le cas résiduel de l'admin rétrogradé.

### Confirmation

**Cas de la matrice T11 :** LCK-01 à LCK-10, X-06 à X-11, X-13, X-15, X-19 (verrous via relais).

**Nouveaux cas :**

| Cas | Ce qui est vérifié |
|---|---|
| LCK-11 | un verrou distant arrive pendant la frappe ; jamais de modification refusée chez l'écrivain honnête |
| LCK-12 | levée forgée par un écrivain |
| LCK-13 | modification antidatée compensée une seule fois ; `compensations` est idempotente |
| LCK-14 | fast-check sur 1 000 ordres de livraison : verdicts et états identiques chez tous |
| LCK-15 | bench : contrôle d'une frappe sur 1 Mo avec 10 verrous, p99 < 1 ms |
| LCK-16 | modification de zone d'un admin rétrogradé, puis réattestation ou ratification |

**Attaques :**
- Écriture dans la zone par une modification qui descend du verrou : refusée par tous, inscrite au
  journal de sécurité.
- Antidatage, ou variante née avant le verrou : admis, puis compensé et journalisé. L'escalade est
  la suspension (ADR 0021).
- Pose ou levée forgée.
- Modification d'une entrée existante.
- Remplacement de `meta` ou de `text`.
- Marque posée sur la zone.
- Inondation forçant le repli lent : budget CPU par pair, puis `drop`.

## Plus d'informations

**Corrections de la matrice T11 :**
- X-06 : le verrou reste parce qu'il vit dans `meta`, pas dans la liste.
- X-09 : union des entrées immuables ; l'interface montre les deux actions.
- LCK-06 : refus si la modification descend du verrou, compensation sinon.
- LCK-10, X-13 : modifications admises, puis compensées et proposées en suggestion.
- X-11 : fusion textuelle pour un écrivain.
- B2 : `versions.by` n'est pas vérifié par T10. Piste : déclarer les acteurs dans l'attestation.

**Sources :**
- [`getCursor`](https://automerge.org/automerge/api-docs/js/functions/getCursor.html)
- [`diff`](https://automerge.org/automerge/api-docs/js/functions/diff.html)
- [Keyhive, note 01](https://www.inkandswitch.com/keyhive/notebook/01/)
- [ARK, sans verrou par zone](https://automerge.org/docs/keyhive/ark-api-guide/)
- [BFT CRDT](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf)
- [CodeMirror](https://codemirror.net/docs/ref/)
- [plages en lecture seule](https://discuss.codemirror.net/t/how-to-make-certain-ranges-readonly-in-codemirror6/3400)
