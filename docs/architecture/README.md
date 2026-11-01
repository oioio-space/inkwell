# L'architecture d'inkwell, simplement

> Une page pour comprendre. Le détail normatif est dans [spec.md](spec.md), les doutes et
> leurs réponses dans [decisions.md](decisions.md), le lien exigence → code → test dans
> [traceability.md](traceability.md). La finalité reste le [document de conception](../conception.pdf).

## Ce que fait inkwell

Plusieurs personnes écrivent en même temps le même projet Typst, Markdown ou texte, chacune
sur son ordinateur, **sans serveur** : pas de compte, pas de cloud. Chaque frappe est gardée
sur le disque de chacune ; l'édition continue hors ligne et tout se rejoint au retour. On sait
qui a écrit quoi, on nomme des versions, on revient en arrière, on exporte un PDF identique
chez tout le monde.

## Les quatre pièces

```
  ┌──────────────── fenêtre inkwell ────────────────┐
  │  Webview : l'éditeur, l'aperçu, les panneaux    │  ← le texte vit ici (Automerge)
  └───────────────▲─────────────────────────────────┘
                  │ Streams Wails : octets opaques, aucun port local
  ┌───────────────┴─────────────────────────────────┐
  │  Cœur Go : réseau, confiance, disque, Typst     │  ← ne décode jamais Automerge (il lit la projection)
  └───▲───────────────▲───────────────────▲─────────┘
      │ libp2p chiffré │ os.Root            │ processus bornés
   autres personnes   dossier du projet    typst (et tinymist)
```

1. **La webview** tient le document. Automerge (un CRDT) fusionne toujours les frappes de
   tout le monde ; CodeMirror les affiche. C'est là qu'on vérifie qu'une modification reçue a
   le droit d'entrer (signée par une personne qui écrit, bien formée, bornée).
2. **Le pont** fait passer des octets entre la webview et Go par des Streams Wails. Il ne
   les comprend pas : il les compte et les borne.
3. **Le cœur Go** fait tout ce qui touche au monde : le réseau (libp2p, chiffré de bout en
   bout), la liste signée des personnes et de leurs droits, le disque (écritures atomiques et
   vérifiées), les fichiers binaires (adressés par leur empreinte BLAKE3), et Typst.
4. **Typst** compile dans un processus à part, enfermé : pas de réseau, pas d'accès hors du
   projet, mémoire, temps et sortie bornés. Il est embarqué dans l'exécutable et revérifié
   avant chaque lancement.

Et, autour de ces quatre pièces, des **extensions** : presque tout ce que l'on voit (commandes,
menus, panneaux, types de fichiers, coloration, diagrammes, exportateurs, thèmes, stockage
distant) passe par un seul registre ; les fonctions d'inkwell s'y inscrivent comme celles d'un
greffon. Les greffons tiers sont des modules WebAssembly (plusieurs langages) qui tournent dans un
processus enfermé, sans fichier, sans réseau, sans clé ; ils ne demandent que des droits nommés.
Un petit **noyau fermé** (identité, liste des membres, signatures, réseau, règles du CRDT,
intégrité du stockage) ne s'étend pas.

## Une seule vérité par chose

| Chose | Vérité | Ce qu'on en tire |
|---|---|---|
| Le texte, l'arbre du projet, les commentaires, les versions nommées | documents Automerge | les fichiers `.typ` lisibles sur le disque, l'aperçu, la recherche |
| Les images, polices, paquets Typst | magasin BLAKE3 | une copie sous leur nom, dans le dossier |
| Les personnes, rôles, décisions d'admin | listes signées, pliées pareil partout | qui peut se connecter, écrire, décider |
| L'historique Git (optionnel) | rien : c'est une projection | un dépôt Git reconstruit à partir des versions nommées |

Aucun panneau n'invente sa propre vérité. Si deux vérités semblent nécessaires, c'est qu'une
des deux est une vue.

## Comment l'architecture sert la finalité

| La conception demande | L'architecture répond par |
|---|---|
| sans serveur, LAN ou code court | libp2p (QUIC, Noise, mDNS, relais optionnel), code Wormhole pour l'invitation |
| hors ligne d'abord | chaque frappe persistée localement en moins de 200 ms ; fusion par Automerge |
| qui a écrit quoi | chaque modification signée par son auteur ; historique Automerge complet |
| confiance explicite | liste de membres signée, rôles, suspension, retrait ; modèle byzantin : une personne malveillante ne gêne jamais les autres |
| export PDF fiable, hors ligne | `typst` épinglé, embarqué, paquets fournis, PDF identique à l'octet |
| un logiciel auquel on se fie | invariants de non-perte testés par pannes injectées ; état affiché seulement s'il est prouvé |
| rapide, même par Internet et sur de gros fichiers | Automerge hors du fil d'affichage, paliers de taille, flux séparés par classe sur le réseau |
| un seul exécutable, paquets `.deb` et `.rpm` | sidecars et interface embarqués ; construction native par architecture, signée |
| zéro jargon, Adwaita | « pair » dans le code, « personne » à l'écran ; composants Adwaita réutilisés |
| un projet de plusieurs fichiers | un document par fichier texte, un index qui les relie, chaque fichier désigné par un identifiant stable (on peut le renommer pendant qu'une autre personne écrit) |
| hautement extensible (N20–N26) | registre unique, extensions intégrées sur la même API, greffons WebAssembly à droits nommés, greffons de projet bornés, interface jamais dénaturée, budgets mesurés |
| une IA de premier rang (N31) | une extension intégrée comme les autres : fournisseurs appelés par Go (y compris le serveur d'IA de l'entreprise), politique décidée pour le projet, propositions relues avant d'entrer, protection contre les consignes cachées dans le texte des autres, serveur MCP local sans port par défaut |
| écrire confortablement (N27–N30) | modèles créés par les personnes, erreurs cliquables comme dans un IDE, aide à la saisie Typst (tinymist), outils d'écriture |

## Les règles qui ne bougent pas

- Go ne lit jamais Automerge ; la convergence se teste en TypeScript.
- Tout le réseau passe par une seule frontière (`internal/transport`) ; libp2p n'est importé
  qu'à deux endroits.
- Tout octet venu d'un pair est hostile : borné, décodé strictement, vérifié, fuzzé.
- Aucun téléchargement à l'exécution ; aucun port local pour le pont.
- Un paquet = un rôle, créé quand on en a besoin.
- Ouvert par défaut ; fermé seulement là où le modèle de menace l'exige ; une extension ajoute,
  décore ou enveloppe, et « Redémarrer sans extensions » rend toujours l'application d'origine.

Elles sont vérifiées par `scripts/arch-check.sh` ; les changer exige une ADR.

## Où en est-on

Le cœur tourne (étapes 0 à 3 en grande partie, T11 en cours), avec un seul document par projet
pour l'instant. Le protocole réseau public (ADR 0034) est accepté et en migration. Les greffons
(ADR 0039, en annexe de la spec, étude dans [research/plugins.md](research/plugins.md)) et la
spécification du protocole des greffons (`docs/plugin-api/`, à écrire) sont proposés. Les écarts entre ce document et le code sont listés
dans [spec.md §16](spec.md#16-écarts-avec-le-code-au-2026-11-01), chacun avec sa décision.
