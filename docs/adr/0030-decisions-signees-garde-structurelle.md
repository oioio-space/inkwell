---
status: proposed
date: 2026-10-26
decision-makers: [Mathieu]
---

# Décisions d'administration hors du CRDT (journal signé) et garde structurelle de toute modification Automerge

## Contexte et problème

B4b (`origin/t11-b4b-files`) place l'état réservé aux admins dans l'index Automerge : règle de
suppression, décisions sur les demandes, purge, fichier principal, remplacement de nœuds.
`frontend/src/project/admit.ts` tente de juger chaque modification reçue. Quatre audits Opus ont
chacun trouvé de nouvelles brèches. Le dernier en relève quatre :

1. `op.pred` est choisi par l'auteur. « La clé existait » peut donc être forgé : un `makeMap`
   avec `pred: []` sur `settings` ou `nodes` vide l'arbre ou passe en suppression libre sans
   admin.
2. Une op qui vise un objet créé par une modification concurrente non ancêtre est acceptée chez
   certains pairs et refusée chez d'autres : la division est définitive.
3. Le budget de jugement est partagé : un écrivain affame les autres.
4. Déjà présent sur `main` : une modification brute fait paniquer Automerge
   (`Option::unwrap` sur `None`). N'importe quel écrivain fait tomber n'importe quel receveur.

Ce n'est pas une règle qui manque, c'est la mauvaise place. Juger qu'une modification Automerge
est *autorisée* oblige à interpréter ses op par rapport à un état. Or ces op portent des
métadonnées que l'auteur choisit (`pred`, `obj`, compteur, acteur). Un verdict qui lit l'état du
receveur dépend de l'ordre d'arrivée. Un verdict qui lit l'état aux dépendances
(`A.view`) coûte O(historique), d'où un budget, d'où la famine.

Sondes du 26/10/2026 sur Automerge 3.5.0. Chaque modification est construite par
`A.encodeChange` sur un vrai document et appliquée **sans** garde :

| Modification forgée | Effet |
|---|---|
| `pred` vers une op inexistante ; trou dans `seq` ; `elemId` inexistant (insertion ou suppression) | panique `Option::unwrap` |
| `elemId` pris dans un autre objet | panique `assertion failed` |
| op sur un objet créé par une modification concurrente (PoC `obj.mjs`) | appliquée chez qui l'a, panique ailleurs : divergence |
| clé de map sur un objet texte | appliquée, panique selon l'ordre, **save → load échoue** |
| `pred` vers une op d'une autre clé, d'un autre objet ou vers elle-même ; `obj` = op scalaire | appliquée, **save → load échoue** |
| `startOp` sous le compteur des dépendances ; acteur d'un autre pair réemployé | appliquée, **save → load échoue** (`max_op is lower than start_op`) |
| deux contenus pour le même `(acteur, seq)` | `RangeError` pour le second, selon l'ordre |
| `makeMap` à la racine avec `pred: []` (PoC `pred.mjs`) | conflit gagné par le forgeron |

Les lignes « save → load échoue » sont pires qu'une panique. La modification passe, puis le
document de **chaque** pair ne se recharge plus au prochain démarrage.

## Facteurs de décision

- **Déterminisme.** Le verdict sur une modification c est une fonction de c et de `before(c)`,
  l'ensemble de ses ancêtres. Il ne dépend jamais de l'état du receveur ni de l'ordre d'arrivée
  (Kleppmann 2022, §3.4).
- **Autorité convergente.** Révocation, suspension et antidatage suivent les règles éprouvées de
  l'ADR 0017 et de l'ADR 0021, pas des règles nouvelles.
- Go ne lit pas Automerge (ADR 0011).
- Aucun jugement en O(historique) sur le chemin de la frappe.
- La liste de membres garde son budget : 1024 op, Welcome ≤ 256 Kio.
- Une seule source de vérité par chose.

## Options envisagées

**Où vivent les décisions d'admin**

- **O1. Continuer à corriger `admit.ts`.** Quatre audits ont échoué, pour des causes de structure
  (voir plus haut).
- **O2. Document Automerge réservé aux admins**, filtré par la couverture T10 des seuls admins.
  L'admission dépend alors de la propagation. Une modification d'un admin rétrogradé, prise par un
  pair avant qu'il apprenne la rétrogradation, et par un autre après, fait diverger les deux pairs
  pour de bon. C'est le cas résiduel de l'ADR 0022.
- **O3. Nouveau kind dans la liste de membres** (l'hypothèse de départ, prise à la lettre).
  - Le scellement à 1024 op deviendrait commun : des décisions de fichiers figeraient la gestion
    des personnes.
  - Le pli porte des bitsets d'ancêtres en O(n²) : 36 Mo à 17 000 op.
  - Le Welcome est borné à 1024 op et 256 Kio.
  - Des op de membres auraient des op de décision dans `Prev` : un arrivant ne pourrait plus les
    intégrer.
- **O4. Journal de décisions signé, posé sur la liste de membres, avec des liens causaux dans les
  deux sens.** Chaque décision nomme la frontière de membres qu'elle a vue. Chaque op de membre
  nomme la frontière de décisions qu'elle a vue.
- **O5. Enregistrements signés par clé, sans DAG** (Willow/Meadowcap, Earthstar). Sans causalité
  avec les révocations, un admin révoqué antidate librement.

**Côté CRDT**

- **S1. Admission sémantique** : la voie de B4b.
- **S2. Garde structurelle seule, puis interprétation.** Toute valeur écrite par un écrivain est
  acceptable. L'effet visible est une fonction pure de l'index et des décisions.

## Décision

**O4 et S2.** Les décisions d'admin quittent le CRDT pour un journal signé, vérifié par Go, plié
avec les règles de l'ADR 0017. Le CRDT ne contient plus que de l'état d'écrivain, où toute valeur
est acceptable. Une garde structurelle, déterministe, protège Automerge sur **tous** les documents,
avant le `Repo`.

L'hypothèse est confirmée sur le fond. Elle est corrigée sur quatre points :
- pas dans la liste de membres elle-même (O3) ;
- « toute valeur acceptable » exige deux règles de schéma sans état, car masquer un nœud est déjà
  un effet d'admin ;
- la garde sert aussi au déterminisme (point 2) et à l'intégrité du stockage, pas seulement au
  point 4 ;
- l'équivoque `(acteur, seq)` reste, et ne peut pas être rendue déterministe (voir Conséquences).

### 1. Quel état vit où

| État | Où | Qui l'écrit |
|---|---|---|
| membres, rôles, suspensions | liste signée (ADR 0017, 0021) | admins |
| règle de suppression, fichier principal, corbeille sur accord, refus, restauration sur accord, purge | **journal de décisions** (`internal/policy`) | admins |
| arbre : `type`, `name`, `parent`, `url`, `hash`, `size`, `ftype`, `who`, `by`, `at` | index Automerge | écrivains |
| demande de suppression `deletion = {req, by, at}` (en mode libre : la corbeille) | index Automerge | écrivains |
| paquets `@preview` | index Automerge | écrivains |
| contenu des textes | un document Automerge par fichier | écrivains |
| octets des binaires | magasin BLAKE3 (ADR 0023) | — |

`settings`, `decisions`, `main` et `trashed` disparaissent de l'index. Retirer un paquet devient un
geste d'écrivain : c'est une dépendance du code, comme une ligne de texte. Point à confirmer par
l'utilisatrice (`docs/open-questions.md`).

### 2. Journal de décisions (`internal/policy`, Go)

**Op de décision** (encodage canonique binaire, domaine `policy-op`, signée sur le `ProjectID`,
au plus 8 Kio, `Decode` strict et fuzzé) :

| Champ | Contenu |
|---|---|
| `Prev` | frontière du journal vue par l'auteur ; ≤ 64, strictement croissante |
| `Members` | frontière de la liste de membres vue par l'auteur ; non vide, ≤ 64 |
| `Lamport` | 1 + le maximum de `Prev` (1 si `Prev` est vide) |
| `Author`, `Sig` | clé Ed25519 de l'appareil |
| `Entries` | ≤ 96 entrées triées, clés uniques : `mode` (approval, free) ; `main` (id de nœud, 16 octets) ; `node/<id>` (`kept`, `trashed` ou `purged`, plus le `req` de la demande tranchée, ou zéro) |

**Amendement de l'ADR 0017.** Les op de membres gagnent `Seen`, la frontière du journal vue par
l'auteur (≤ 64). Elle est encodée seulement en version 2. Une op sans `Seen` reste en version 1,
avec le même hachage. Un nouveau client émet donc la version 1 tant que le projet n'a aucune
décision. **Le pli des membres ignore `Seen`** : les règles 1 à 5 et S1 à S3 sont inchangées, de
même que le Welcome et le scellement à 1024.

`Seen` n'est pas une dépendance. Une op de membres s'intègre sans les décisions qu'elle nomme ; le
pli des décisions lit ce qui est stocké, et converge quand tout est arrivé.

**Admission** (fonctions de l'op et de son passé, comme `List.integrate`) :
- **A1.** L'op est canonique et signée pour le projet.
- **A2.** `Prev` et `Members` sont connus. Sinon l'op attend dans un tampon borné, réservé aux admins
  debout, comme pour `trust`.
- **A3.** Le Lamport est exact.
- **A4.** L'auteur a reçu le rôle admin dans `past(Members)`. C'est `everAdmin`, sinon
  `ErrUnknownAuthor`. Les lecteurs et les écrivains ne peuvent rien stocker.
- **Plafond.** 8192 op de décision, séparé des 1024 op de membres. Au-delà, le journal est
  **scellé** : les décisions se figent, mais l'édition et la gestion des personnes continuent.

**Pli** (pur, identique partout ; ≺ = Lamport, auteur, hachage) :
- **D1. Debout.** Une décision o par y tient si et seulement si :
  - (a) y est admin et non suspendu dans la liste pliée sur `past(Members(o))`. Ce sont les règles
    3(a) et S3(a), évaluées pour une op hypothétique à cette frontière ;
  - (b) aucune coupe debout sur y par une autre clé, aucune suspension debout de y par une autre
    clé, et aucune sortie de y n'est **concurrente** à o.
  - Une op de membres m est concurrente à o si `m ∉ past(Members(o))` et `o ∉ past(Seen(m))`.
  - Ainsi, un admin retiré qui antidate reste mort. Ce qu'un admin a décidé et que son
    révocateur avait vu tient.
- **D2. Registre.** Pour chaque clé, on prend la frontière : les décisions debout qui l'écrivent et
  qu'aucune autre ne voit. Puis :
  - `mode` vaut `approval` si l'une le dit ;
  - `main` prend la ≺-dernière ;
  - `node/<id>` prend la plus prudente, dans l'ordre `kept` > `trashed` > `purged`. Le refus gagne
    (FM-08). Une frontière à plus d'une valeur est marquée `contested`, pour l'interface.
- **D3. Défauts.** `mode` vaut `approval`. `main` est le nœud principal de la genèse de l'index.

**API exposée par `trust`** (pure, utilisée par `policy`) :
- `AdminAt(key, frontier)` ;
- `EverAdmin(key, frontier)` ;
- `CutsOn(key)` : les coupes, suspensions et sorties debout, avec leur `Seen` ;
- `InPast(hash, frontier)`.

`policy` dépend de `trust`, jamais l'inverse.

**Transport et pont.**
- La réplication suit le motif de `trust.Sync` (frontière, envoi dans l'ordre causal, budget par
  pair), sur le canal trust, avec un octet de type.
- Persistance : `.inkwell/policy/<hachage>.op` par le magasin.
- Deux kinds de pont, avec vecteurs dorés et fuzz :
  - `Decide` (webview → Go) : Go vérifie le rôle, signe, applique et diffuse ;
  - `Policy` (Go → webview) : l'état plié, au démarrage et à chaque changement.
- Go lit `main` (compilation Typst) et `purged` (ménage des blobs, ADR 0023) **directement**, sans
  lire Automerge.

### 3. Index : genèse déterministe, schéma minimal, interprétation

**Genèse.** Chaque pair calcule, sans réseau, la même modification G :
- acteur `a0 = BLAKE3("inkwell/index-actor/v1" ‖ ProjectID)[:16]`, avec `seq 1`, `startOp 1`,
  `time 0`, `deps []` ;
- op : `schema = 1`, `nodes = {}`, `packages = {}`, et le nœud `main.typ` qui pointe vers
  `RootURL` ;
- id du nœud principal : `BLAKE3("inkwell/main-node/v1" ‖ RootURL)[:16]` ;
- `docId = BLAKE3("inkwell/index/v1" ‖ hash(G))[:16]`, importé par `repo.import(…, {docId})`.

C'est le principe de l'`idFactory(heads)` de Keyhive.

Les octets sont identiques chez tous : c'est vérifié sur 3.5.0. Cela vaut pour les anciens projets
comme pour les nouveaux. Le document racine reste le texte de `main.typ`. **Il n'y a plus de
migration**, plus de course entre deux migrations, plus d'index virtuel pour les lecteurs, et le
format de la genèse `trust` ne change pas. Un vecteur doré fige le hachage de G : une montée de
version d'Automerge qui changerait l'encodage casse le test, pas les projets.

**Deux règles de schéma**, sans état : les identifiants de G sont des constantes.
- **R-root.** Une op dont `obj` est `_root` n'est admise que dans G. Les conteneurs racine sont
  donc figés.
- **R-nodes.** Sur la map `nodes`, une seule op est permise : `makeMap`, avec une clé de 32 hex et
  `pred: []`. Ni `del`, ni scalaire, ni remplacement. Masquer un nœud reste ainsi un effet de
  décision.

**Interprétation** (`materialize(index, policy)`, pure, TS ; le reste de l'ADR 0023 inchangé) :

| Situation | Effet |
|---|---|
| `node/<id> = purged`, sur le nœud ou sur un ancêtre | absent ; un déplacement concurrent hors du dossier y échappe |
| mode `approval` | en corbeille si et seulement si la décision (du nœud ou d'un ancêtre) est `trashed` ; une `deletion` dont le `req` n'est pas tranché est une demande ; tranché à `kept`, c'est un refus |
| mode `free` | en corbeille si et seulement si `deletion` est posé (sur le nœud ou un ancêtre) |
| fichier principal courant | jamais en corbeille ni purgé (FM-18) ; un `main` qui ne désigne pas un nœud texte visible retombe sur celui de la genèse |
| plusieurs maps pour `nodes[id]` | la première bien formée, dans l'ordre des opId |
| nœud mal formé (nom invalide, champ manquant) | **listé, marqué « endommagé »**, jamais projeté sur disque, réparable par un renommage. Seule une décision ou une corbeille cache un nœud |

**Changer de mode.** C'est un seul geste de l'admin, côté client honnête.
- Vers `approval` : un lot de décisions `mode = approval`, plus `node/<id> = trashed` pour chaque
  nœud alors en corbeille.
- Vers `free` : `deletion` est posé dans l'index pour chaque nœud `trashed`, puis la décision
  `mode = free`. Les demandes en attente deviennent effectives, et l'interface le dit.

### 4. Garde structurelle (TS, tous les documents, avant le `Repo`)

**Chaîne.** Message reçu → couverture T10 → garde → `Repo`.

La garde est une fonction de c et de `before(c)`. Elle est **sans attente** sauf G2. Le nom de la
règle est consigné au journal de sécurité.

- **G1. Bornes et format.**
  - c se décode et respecte la taille de trame ;
  - acteur de 1 à 32 octets ;
  - message ≤ 4 Kio ;
  - `deps` distincts ;
  - `1 ≤ startOp` et `startOp + |ops| ≤ 2⁵³` ;
  - action dans `set`, `del`, `inc`, `make{Map,List,Text,Table}`, `markBegin`, `markEnd` ;
  - type de donnée connu.
- **G2. Dépendances.**
  - une dépendance inconnue fait **attendre** ;
  - une dépendance refusée fait refuser c.
- **G3. Chaîne d'acteur.** `seq = 1` si l'acteur est absent de `before(c)`. Sinon,
  `(acteur, seq − 1)` doit être dans `before(c)`. `changeAt` crée un nouvel acteur, donc un client
  honnête respecte toujours cette règle (vérifié).
- **G4. Compteur de Lamport.** `startOp > maxOp(before(c))`. C'est la règle de Kleppmann (§3.4),
  en inégalité stricte, car `changeAt` saute des compteurs.
- **G5. Objet.**
  - `obj` vaut `_root`, ou désigne une op `make*` de `before(c)` ou une op antérieure de c ;
  - map ou table prennent une `key` ; liste ou texte prennent un `elemId`.
- **G6. Élément.** `elemId` vaut `_head`, ou désigne une op d'insertion **du même objet**, dans
  `before(c)` ou plus tôt dans c.
- **G7. `pred`.** Chaque entrée est distincte et désigne une op de `before(c)`, ou plus tôt dans c,
  sur le même objet et la même clé (ou le même élément), jamais c elle-même.
  - L'op désignée pose une valeur ou insère l'élément visé : `set`, `make*` ou insertion.
  - Elle n'est jamais un `del` ni un `inc`.
  - La liste exacte est verrouillée par T2 et T3.
- **G8. Unicité.** Un `(acteur, seq)` déjà admis avec un autre hachage est refusé, et la preuve
  d'équivoque est consignée. C'est la **seule** règle qui lit l'état local, et donc la seule qui
  dépend de l'ordre (voir Conséquences).
- **G9. Schéma** du document, s'il en déclare un : R-root et R-nodes pour l'index.
- **G10. Document complet.**
  - `A.load` dans un bac isolé ;
  - puis chaque modification, dans l'ordre topologique, passe G1 à G9 ;
  - un seul refus fait jeter le fragment entier, comme dans l'ADR 0018.

**Mise en œuvre** (`frontend/src/net/guard.ts`, `frontend/src/net/ancestry.ts`) :
- **Index par document**, tenu à jour par les modifications admises et locales :
  - `hash → {acteur, seq, startOp, n, maxOp}` ;
  - par acteur, les plages de `startOp` triées ;
  - des horloges vectorielles immuables à partage de structure (copie de chemin, points de contrôle
    tous les 64).
- **Ancêtres.** G3 rend les chaînes d'acteur causales, donc `(a, s) ∈ before(c)` si et seulement si
  `horloge(c)[a] ≥ s`.
- **Détail d'une op visée** : `getBackend(doc).getDecodedChangeByHash`, avec un cache LRU.
- **Coût** : O((|ops| + |deps|) · log A) par modification. Aucun parcours d'historique : `A.view`
  et `getChangesMetaSince` en sont exclus. Ce dernier coûte 200 ms pour une dépendance antidatée
  sur 20 000 modifications.
- **Défense en profondeur.** Si Automerge lève malgré tout une `PanicError`, le message est jeté,
  le document est rechargé depuis le magasin Go, et l'événement « trou de la garde » est consigné.

### 5. Équité (point 3)

- Plus aucun verdict ne lit l'historique, donc il n'y a plus de budget de jugement.
- Chaque pair émetteur a sa file, servie à tour de rôle.
- Un seau d'unités de travail (op + dépendances, et non des millisecondes) ralentit **sa** file
  seulement.
- Un manque de budget retarde, et ne change jamais un verdict.
- Les modifications en attente de G2 ont un quota par émetteur. On évince d'abord ses plus
  anciennes, jamais celles d'un autre.

### 6. Pourquoi les points 1 à 4 deviennent impossibles

| # | Brèche | Ce qui la ferme |
|---|---|---|
| 1 | `pred` forgé | Aucun verdict ne lit plus « la clé existait » : `settings`, `decisions` et `main` ne sont plus dans le CRDT. R-root refuse toute op racine hors de G, et R-nodes refuse tout remplacement ou retrait de nœud, **chez tous**. G7 vérifie `pred` seulement pour l'intégrité. |
| 2 | objet concurrent | G5, G6 et G7 exigent la cible dans `before(c)` : c'est refusé partout, quoi que le receveur détienne (Kleppmann §3.4). |
| 3 | famine | Pas de jugement en O(historique) ; files et seaux par pair (§5). |
| 4 | panique | G1 à G10 reprennent les invariants d'Automerge. L'oracle de test (plus bas) exige : aucune exception, aucune erreur dépendant de l'ordre, et save → load qui passe. La défense en profondeur protège le reste. |

### 7. Migration du code de B4b

| Fichier (B4b) | Sort |
|---|---|
| `project/admit.ts`, `admit.*.test.ts` | **supprimés** ; leurs PoC deviennent des tests de la garde |
| `net/coverage.ts` | garder T10 ; **supprimer** `DocValidator`, `adminCovers`, `promote`, `setAdmins` et les attentes « admin » et « budget » ; ne garder que l'attente de G2 |
| `WailsNetworkAdapter.ts`, `workspace.svelte.ts` | brancher la garde sur tous les documents ; retirer `IndexValidator` |
| `project/migrate.ts` | **supprimé** ; remplacé par `project/genesis.ts` |
| `project/model.ts` | retirer `settings`, `decisions`, `main` et `trashed` ; ajouter les types de `Policy` |
| `project/ops.ts` | garder `create`, `rename`, `move`, la demande et `replaceBlob` ; `trash`, `accept`, `refuse`, `restore` sur accord, `purge`, `setMain` et `setDeletionMode` rendent une intention `Decide` ; garder les vérifications de rôle pour l'interface |
| `project/materialize.ts` | garder ; prendre `policy` en paramètre ; corbeille et purge héritées ; marque « endommagé » au lieu de l'exclusion |
| `project/names.ts`, `import.ts`, `sync.ts`, `kinds.ts`, `filetypes/*`, `ui/files/*`, Go `addfile`, `reveal`, `opener` | gardés |
| `project/history.ts` | garder pour les gestes d'écrivain ; les décisions viennent des op signées, avec un auteur prouvé |
| `ProjectIndex.ts`, `runtime.svelte.ts` | adaptés : l'index, plus l'état `Policy` reçu de Go |

### Conséquences

**Bien**
- Les décisions d'admin ont la sémantique de l'ADR 0017 : convergence démontrée, antidatage
  impossible, révocation qui tue le concurrent.
- Leur auteur est **prouvé** dans l'historique, et non plus déclaré.
- Le CRDT n'a plus rien à autoriser. La garde est générique, en O(ops · log A), et protège aussi
  les textes et le tableur.
- L'intégrité du stockage est rétablie (save → load).
- Go connaît `main` et la purge sans lire Automerge.
- Il n'y a plus de migration en course.

**Moins bien, assumé**
- **Équivoque.** Un écrivain courant peut publier deux contenus pour un même `(acteur, seq)`,
  y compris l'acteur d'un autre pair, vers deux pairs différents. Automerge, qui identifie les op
  par `(compteur, acteur)`, refuse le second : le document se divise.
  - Aucune garde ne peut trancher sans consensus (Kleppmann §2.3.1 et §3.3 ; p2panda le qualifie
    de comportement byzantin).
  - G8 consigne la preuve, l'interface signale la personne, et l'escalade est la suspension.
  - Préexistant sur `main`. Suite proposée : une ADR « acteurs liés aux clés », avec un acteur
    dérivé de la clé, déclaré dans `Attest`.
- **Versions mêlées.** Dès la première décision, les op de version 2 et les messages du journal ne
  sont pas lus par un client antérieur, qui cesse d'intégrer la liste. Il faut alors une version
  commune, ce qui est acceptable avant 1.0. La compatibilité tient tant qu'aucune décision n'existe.
- **Deux écritures pour un geste.** Changer de mode ou restaurer en renommant écrit une décision
  et l'index. Chacune est idempotente, et l'écart reste transitoire et visible.
- **Scellement.** Un ancien admin peut sceller le journal en l'inondant d'op mortes : les décisions
  se figent, l'édition continue. C'est la même posture que l'ADR 0017.
- **Paquets.** Retirer un paquet devient un geste d'écrivain (à confirmer).
- **Données.** Les nœuds purgés restent dans le CRDT, cachés par la décision.
- **Coût.** Un nouveau paquet Go, deux kinds de pont et un champ `Seen`.

### Confirmation

**TypeScript (Vitest, fast-check) :**
- **T1. Vecteurs des audits.** `pred.mjs`, `obj.mjs` et chaque ligne du tableau des sondes.
  - Construction : `A.encodeChange` sur un vrai document, avec une **horloge réelle** :
    - `time: Date.now()` ;
    - `startOp`, `seq` et `deps` tirés du document ;
    - une seule règle violée par vecteur.
  - Attendu : refus par la règle nommée, chez trois répliques, dans tous les ordres de livraison.
  - Une assertion « sans garde » documente l'échec d'Automerge, pour voir un correctif amont.
- **T2. Complétude.** Des histoires honnêtes sont générées : `change`, `changeAt`, `merge`, texte,
  marques, blocs, compteurs, listes, 1 à 5 acteurs, fourches hors ligne. La garde doit les admettre
  à 100 %, par messages comme par document complet.
- **T3. Oracle de solidité.** Des mutations de modifications honnêtes sont générées : `obj`,
  `elemId`, `pred`, `key`, action, `insert`, `startOp`, `seq`, acteur, `deps`, type de donnée. Pour
  tout ce que la garde admet, livré à N répliques en ordre aléatoire :
  - aucune exception ;
  - têtes et `toJS` identiques ;
  - `save → load` et `saveIncremental → loadIncremental` passent ;
  - une frappe honnête suit sans erreur.

  Une passe longue tourne chaque nuit, avec sa graine consignée.
- **T4. Déterminisme.** Le même multiensemble est livré dans 1000 ordres : les mêmes ensembles
  admis partout. Seule exception : l'équivoque, dont la preuve est consignée chez tout pair qui
  voit les deux.
- **T5. Équité, horloge réelle** (`performance.now()`, aucune horloge injectée).
  - Un émetteur inonde le pair de modifications antidatées valides.
  - La latence p99 de la frappe d'un autre émetteur reste ≤ 2 × sa base.
  - Les verdicts ne dépendent pas du temps.
- **T6. Interprétation.** `materialize` est testée sur des index arbitraires, conflits compris :
  - aucune exception ;
  - aucun nœud caché sans décision ou corbeille ;
  - le fichier principal n'est jamais caché.
- **T7. Bancs.**
  - frappe sur un document de 100 000 modifications : p99 ≤ 50 µs ;
  - modification concurrente : p99 ≤ 1 ms ;
  - document complet de 20 000 modifications : ≤ 300 ms.

**Go :**
- **`policy`.** Les tests suivants sont prévus :
  - une référence naïve et un test de propriété, comme `trust/reference_test.go` ;
  - `FuzzDecode` ;
  - les attaques :
    - décision d'un écrivain ou d'un lecteur (`ErrUnknownAuthor`) ;
    - décision antidatée d'un admin retiré (morte) ;
    - décision concurrente à une coupe (morte) ou vue par la coupe (debout) ;
    - décision d'un suspendu (morte même après la reprise) ;
    - `trashed` contre `kept` concurrents (`kept`) ;
    - scellement à 8192 ;
    - tampon inondé.
- **`trust`.**
  - propriété : le pli est identique avec et sans `Seen` ;
  - décodage des versions 1 et 2 ;
  - les hachages des op de version 1 sont inchangés.
- **Pont et app.**
  - vecteurs dorés `Decide` et `Policy` ;
  - `Decide` est refusé à un non-admin ;
  - la persistance fait l'aller-retour.

**e2e et netsim :** FM-05 à FM-19 rejoués, avec des partitions pendant les décisions.

## Plus d'informations

**Amendements :**
- ADR 0017 : le champ `Seen`, ignoré par le pli ;
- ADR 0018 : la garde s'insère après la couverture ;
- ADR 0023 : le modèle d'index, l'admission et la migration.

**À réexaminer sous ces principes :** l'ADR 0022, dont les règles R1 et R2 reposent sur la
couverture admin et l'admission sémantique.

**Corrections de la matrice T11 :**
- **FM-14** : une suppression « directe » forgée par un écrivain n'existe plus. Sa marque se lit
  comme une demande.
- **FM-13** : les gestes d'un lecteur sont refusés par T10 et par A4.

**Sources :**
- Kleppmann, [Making CRDTs Byzantine Fault Tolerant](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf)
  (PaPoC 2022) : §2.3.1 équivoque, §2.4 et §3.4 validité décidée sur `before(u)` et Lamport,
  §3.3 identifiants uniques.
- Matrix, [vérifications à la réception d'un PDU](https://spec.matrix.org/v1.16/server-server-api/#checks-performed-on-receipt-of-a-pdu) :
  - le format invalide est jeté avant toute autorisation ;
  - l'autorisation porte sur les `auth_events` et l'état avant l'événement ;
  - l'échec contre l'état courant est un « soft fail », car un événement légitime retardé lui
    ressemble.

  Notre D1 en est l'analogue.
- p2panda, [notes sur un CRDT de contrôle d'accès convergent](https://p2panda.org/2025/08/27/notes-convergent-access-control-crdt.html)
  et [p2panda-auth](https://docs.rs/p2panda-auth) :
  - l'autorisation se décide sur le passé causal ;
  - la donnée applicative est séparée du CRDT d'accès ;
  - l'équivoque y est jugée byzantine.
- Ink & Switch, [Keyhive, note 01](https://www.inkandswitch.com/keyhive/notebook/01/) et le
  [guide ARK](https://automerge.org/docs/keyhive/ark-api-guide/) :
  - l'accès se gère par document entier, rien en dessous ;
  - `create2` avec `idFactory(heads)`.
- Loro, [discussion « Data Validation » n° 314](https://github.com/loro-dev/loro/discussions/314) :
  refuser une mise à jour casse la convergence, et la permission en pair-à-pair passe par des
  signatures.
- Automerge, correctifs de paniques sur entrées mal formées (#538, #541, #543, voir le
  [miroir](https://code.thetadev.de/ThetaDev/automerge/compare/patch4..main)) : l'amont ne valide
  pas tout. Les sondes de 3.5.0 le confirment.
