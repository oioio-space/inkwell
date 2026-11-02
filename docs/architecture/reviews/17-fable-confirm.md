# Revue Fable n°9 — confirmation des deltas (conception rév. 7.1, architecture 1.0.0-brouillon.11 / rév. 8.2, 2026-11-02) — verdict : **ACCEPT** (0 bloquant, 5 majeurs de texte, 7 mineurs)

Portée : **confirmatoire**. Les deux spécifications sont acceptées (conception rév. 7, architecture
rév. 8.1) ; je ne relis que ce qui a été ajouté depuis, et je ne rouvre aucune décision de
l'utilisatrice. Lu : `docs/design/spec.md` §4.18, §4.22, §4.27, §4.46, §4.47, §4.48, §5.3, §5.12,
§4.41, §4.1, §3.5, §7, §8.2 ; `docs/design/CHANGES.md` (addendum rév. 7.1) ; `coverage.md`,
`decisions-utilisatrice.md` (n° 21) ; `docs/architecture/spec.md` §1.4 INV-14, §4.5 AC-18/AC-19,
§6.1, §6.11 (« Variantes isolées », VA-1 à VA-10), §6.13, §6.14 (CM-1 à CM-10), §6.15 (VD-1 à
VD-9), §14.1, §15.17 (SY-*, RV-*), §15.19 (PD-4 à PD-9), §16 ; `CHANGES.md` brouillon.11 ;
`decisions.md` `AD-105` à `AD-107` ; `traceability.md` N46–N48 ; `docs/protocol/spec.md` §3.3,
§3.4, §4.6–§4.10, §9.4, §9.7 ; `cddl/repo.cddl:78–86` ; ADR 0022, ADR 0031. Code lu :
`frontend/src/editor/setup.ts`, `frontend/node_modules/@codemirror/commands/dist/index.js`.
Vérifié sur cet hôte (Fedora 44) : `webkitgtk6.0-2.54.1-1.fc44` — `/usr/libexec/webkitgtk-6.0/`
contient `WebKitWebProcess`, `WebKitNetworkProcess`, `WebKitGPUProcess` ; `rpm -qR` exige
`bubblewrap` et `xdg-dbus-proxy`.

Gravités : **bloquant** = garantie centrale contredite ou porte qui ne peut ni passer ni échouer ;
**majeur** = texte qui doit changer avant qu'un lot (ou la boucle du protocole) s'en serve ;
**mineur** = précision. Chaque constat cite une norme ou une règle du projet, une preuve
(fichier:ligne) et une correction.

---

## 1. Non-régression de ce qui est accepté

| Point accepté | Vérifié | Résultat |
|---|---|---|
| INV-14 (autoporteur) et l'exception « processus auxiliaires des bibliothèques garanties » (R8-M1) | `spec.md:153` ; AC-18 `:680` intact (seul `internal/sandbox` importe `os/exec`, exception `cmd/inkwell-launch`) ; AC-19 `:681` : liste exacte des programmes **absents**, `bwrap` et `xdg-dbus-proxy` **présents** ; §14.1 ligne `bubblewrap` | **conforme**. Les trois noms de processus sont exacts (`WebKitGPUProcess` existe bien dans WebKitGTK 2.54, vérifié ci-dessus) ; les deux dépendances sont des *Requires* du paquet. L'exception est bornée (« lancés par la bibliothèque, jamais par notre code ») et testable par l'e2e d'AC-19. Aucune garantie affaiblie. |
| PD-4 (`follow` en `update` seul) et RV-1 à RV-8 | `:3519–3582`, `:3787–3807` | inchangés |
| SY-1 à SY-8 (révision 8.1) | `:3450–3517` | inchangés |
| §6.1 « une source de vérité par chose » | `:777–803` : la ligne « Texte » porte désormais le document `comments` frère dérivé (PD-7) | cohérent avec `AD-106` ; **manque** une ligne pour les opérations de validation (R9-m5) |
| §16 écarts | `:3985–3993` : variantes par document, `A.merge`, lecteurs sans commentaires, N47/N48 absents | à jour |
| Registre des raccourcis §8.2 (table accepté) | balayage automatique des trois colonnes : **aucun doublon** par système dans le registre lui-même | conforme (mais voir R9-M1 : collision avec l'éditeur, hors registre) |
| Décisions de l'utilisatrice | `AD-106` (C : document dérivé + index SQLite dérivé), `AD-107` (personne désignée, pas de rôle nouveau), polices (aucune licence, aucune annonce entre appareils, projet d'abord, fichiers hors dossier non cochés), variantes partagées, décision n° 21 | **toutes reprises telles quelles** dans les deux spécifications ; rien n'est rouvert ici |

---

## 2. Cohérence entre les deux spécifications

### R9-M1 — Raccourci « Commenter » sur macOS : ⇧⌘K est `deleteLine` dans l'éditeur (majeur)

- **Preuve.** Conception §8.2 `:3628` et §4.27 `:1376` : Commenter = Ctrl+Maj+M / **⇧⌘K** /
  Ctrl+Maj+M. `@codemirror/commands/dist/index.js:1812` : `{ key: "Shift-Mod-k", run: deleteLine }`
  dans `defaultKeymap` ; `frontend/src/editor/setup.ts:127` charge
  `keymap.of([...defaultKeymap, ...historyKeymap])` (et `:172` pour la vue en lecture seule).
  Sur macOS, `Mod` = ⌘ : ⇧⌘K **supprime la ligne** dès que le texte a le focus, soit exactement
  le contexte où l'on commente une sélection. Le test K-6 (`:3661–3666`) ne regarde que le
  registre d'Inkwell et les trois tables réservées des systèmes : il ne peut pas voir cette
  collision.
- **Norme.** K-5 (« un raccourci = une action dans un même contexte », `:3569`) ; CodeMirror 6,
  *Extension precedence* (`Prec`) : à précédence égale, la première liaison enregistrée gagne.
- **Correction (texte).** Dans §8.2, une règle : « le registre est enregistré à `Prec.high`,
  au-dessus des liaisons par défaut de CodeMirror ; K-6 teste aussi, dans le contexte « texte »,
  les tables `defaultKeymap`, `historyKeymap` et `searchKeymap` comme **quatrième table réservée**,
  et liste les liaisons **masquées sciemment** ». Pour ⇧⌘K, deux issues honnêtes : garder ⇧⌘K
  (convention de Pages, `deleteLine` reste atteignable par ⌘⌫ + ligne) **en l'écrivant** comme
  masquage, ou choisir ⇧⌘M… non : ⇧⌘M est déjà Formule (`:3606`). Je recommande de garder ⇧⌘K et
  d'écrire le masquage. Note : Ctrl+I (`Mod-i`, `selectParentSyntax`) et Ctrl+M (`Ctrl-m`,
  `toggleTabFocusMode`) sont déjà masqués de la même façon par les liaisons « faites » ; la règle
  les couvre rétroactivement.

### R9-M2 — Point d'intégration quand la validation est obligatoire (majeur)

- **Preuve.** Conception §4.48 `:3037–3040` : avant d'intégrer dans l'Original, la relecture
  affiche « le statut de validation de la **dernière version** de la variante » ; un admin peut
  rendre la validation obligatoire. Architecture VD-5 `:1942–1946` : avec cette politique,
  « Intégrer » exige que le **point d'intégration** soit une version validée ; HI-6 `:1515–1518` :
  « Intégrer » applique la différence jusqu'à l'**état actuel** de la variante ; VD-1 `:1925–1927` :
  toute modification ultérieure est une autre version, non couverte.
- **Écart.** Si le texte a changé depuis la version validée (cas que la conception nomme :
  « Validée · le texte a changé depuis », `:3035`), la conception laisse intégrer l'état actuel
  avec un statut « Validée » en tête, alors que l'architecture exige que le contenu intégré **soit**
  la version validée. Avec la politique active, la garde VD-5 serait vide de sens si l'on intègre
  autre chose que ce qui a été validé.
- **Norme.** ISO/IEC 25010:2023, exactitude fonctionnelle (§2 de l'architecture) ; N48 (« la
  validation reste liée à la version exacte »).
- **Correction (texte, conception §4.48 et §4.22).** « Quand la validation est obligatoire,
  « Intégrer » intègre **cette version validée exactement** (« Intégrer depuis cette version »,
  VA-7), même si la variante a continué ; la relecture le dit (« Vous intégrez la version « Envoyée
  au comité », validée par Anne ; 2 modifications plus récentes ne sont pas intégrées »). Sans
  politique, c'est l'état actuel qui est intégré et le statut dit « Validée · le texte a changé
  depuis ». » Architecture VD-5 : renvoyer à VA-7 pour le mécanisme.

### R9-M3 — Éléments d'interface introduits par l'architecture, absents de la conception (majeur)

La conception est l'autorité de l'interface (K-1 : « avant de créer un composant, chercher en
§4 ; si rien ne convient, compléter la spec d'abord », `:3562` ; K-2 : « un réglage entre dans un
groupe de §5.13 », `:3564`). La révision 8.2 nomme des gestes et des réglages que la rév. 7.1 ne
connaît pas :

| Élément (architecture) | Où | Dans la conception |
|---|---|---|
| Réactions (8 emojis fixes) et Markdown minimal dans un commentaire | CM-1 `:1866–1872` | absent de §4.27 |
| « Ré-ancrer » un fil orphelin | CM-2 `:1876` | §4.27 n'offre que « Voir dans les versions » `:1399` |
| Réglage d'admin « Les lecteurs peuvent commenter » | CM-5 `:1888` | absent de §5.13 |
| « Masquer les commentaires de X » | CM-6 `:1897` | absent |
| « Exporter les commentaires » (rapport Markdown/CSV) et case « Sans les commentaires » | CM-8 `:1905–1908` | absent de §5.12 |
| « Mettre à jour depuis l'original » (intégrer l'original dans V) | VA-4 `:1591–1593` | absent de la table des gestes §4.22 `:1213–1222` |
| Option « Garder toutes les variantes » ; mention « seulement sur l'ordinateur de Paul » | `:1608`, `:1620` | absent de §5.13 et de §4.22 |
| Liste des personnes qui peuvent valider (`review/policy.approvers`) ; soumission réservée à qui écrit | VD-2 `:1928`, VD-4 `:1938–1941`, PD-9 `:3846` | §4.48 ne montre que « rendre obligatoire » `:3040` ; rien sur l'état inactif de « Soumettre » pour un lecteur (le motif existe pour les lecteurs en §4.27 `:1420–1421`) |

- **Correction (texte).** Pour chaque ligne, **soit** la conception (rév. 7.2) ajoute le geste à
  l'endroit existant (menu Projet, §5.12, §5.13 › Projet, en-tête de la sous-vue Commentaires),
  **soit** l'architecture marque « sans interface en v1 ». Les réactions et le Markdown des corps
  sont les deux cas où je recommande la seconde voie (calme de §4.27 ; coût nul à retirer).

### R9-m1 — Délais et défaut de la synchronisation texte ↔ aperçu (mineur)

Conception §4.18 `:1056–1057` : l'aperçu suit le curseur « après **300 ms** de calme » et
l'interrupteur « Suivre le texte » est **activé par défaut** (`:1080`). Architecture SY-7
`:3507` : « suivre le curseur » attend **100 ms** ; SY-4 `:3492` : « en option ». Une seule
valeur (300 ms, celle des autres temporisations de la conception : §4.21, §4.22 point 3) et un seul
défaut (activé). Norme : K-5 ; PF-12.

### R9-m2 — Suivre une personne dans une autre variante (mineur)

Conception §4.22 `:1253–1255` : on **demande d'abord** (« Passer et suivre · Annuler ») ; rien ne
change sans accord. Architecture §6.11 `:1628–1629` : le suivi « **bascule** la variante
courante, avec le bandeau ». Aligner l'architecture sur la conception (confirmation, puis
bascule), et dire ce qui se passe si la personne suivie change de variante **pendant** le suivi
(même question, ou arrêt du suivi avec « Marie est passée sur « Version courte » »). Norme :
HIG GNOME, *ne jamais changer le contexte sans action de la personne* ; WCAG 2.2 SC 3.2.2.

### R9-m3 — Ce qui dépend de PD-5, et deux notes périmées (mineur)

- §4.22 `:1229–1230` (« qui est sur chacune ») et `:1251–1252` (curseurs de la même variante
  seulement) supposent le canal `variant` (PD-5 `:3809–3821`) pour un fichier **non fourché** ;
  l'architecture écrit la limite (`:1627`), la conception non. Écrire « d'ici là, … » comme RV-4
  le fait pour l'œil (`:3556`).
- §4.22 `:1261–1263` (« variantes par document … à confirmer dans HI-6 ») et §4.27 `:1370–1372`
  (« le modèle de données reste à écrire dans l'architecture ») sont périmés : la révision 8.2
  tranche les deux (`:1564–1689`, `:1848–1918`). Remplacer par le renvoi.

---

## 3. Raccourcis, par système

Nouveaux : Commenter (Ctrl+Maj+M / ⇧⌘K / Ctrl+Maj+M), Commentaire suivant (Ctrl+Maj+N / ⇧⌘N /
Ctrl+Maj+N), Commentaire d'une cellule (Maj+F2 / ⇧F2 / Maj+F2) ; Ctrl+Maj+J (⇧⌘J) existait.

| Vérification | Résultat |
|---|---|
| Doublons dans le registre, par colonne | aucun (balayage des 55 lignes de `:3597–3651`) |
| Tables réservées (`:3656–3659`) | aucune des six liaisons nouvelles n'y figure. Alt+F2 (GNOME, *Run a command*) et ⌥⌘M (macOS, *Réduire toutes les fenêtres*) sont évités à raison |
| K-7 : pas de Ctrl+Alt, pas d'Alt+lettre sous Windows, pas d'Option+lettre seule sous macOS | conforme |
| Collision avec l'éditeur | **⇧⌘K = `deleteLine`** (R9-M1). Ctrl+Maj+M, Ctrl+Maj+N, ⇧⌘N, Maj+F2 : absents de `defaultKeymap` et `historyKeymap` |
| Pages / Numbers | ⇧⌘K = « Ajouter un commentaire » (Apple, *Raccourcis clavier de Pages*) : convention bien choisie |

### R9-m6 — ⇧F2 sur macOS contredit K-7 (mineur)

K-7 `:3592–3593` : « chaque commande à touche de fonction a une liaison macOS à lettre, **sauf
Zones** ». La ligne `:3630` donne ⇧F2 seul (Fn requis sur un portable). Numbers emploie ⇧⌘K pour
commenter une cellule : donner **⇧⌘K dans le contexte de la grille** (K-5 : un raccourci, une action
par contexte), et garder Maj+F2 sous Linux et Windows (convention Excel). Ou écrire l'exception
comme pour Zones.

---

## 4. WCAG 2.2 AA et ARIA pour l'interface nouvelle

Ce qui est bien : badges de validation en forme + icône + mot (SC 1.4.1, A-3) ; badge dans le nom
de l'option de la frise (SC 4.1.2) ; « Suivre le texte » en bouton bascule `aria-pressed` (APG
*Button*) ; aucune annonce pour la synchronisation automatique, annonce après un saut demandé
(A-10, SC 4.1.3) ; surlignage en fondu de 1 s, instantané sous mouvement réduit (SC 2.3.1, 2.3.3) ;
barre de suivi à la couleur **et** au nom de la personne (SC 1.4.1) ; fil de commentaire en
région nommée (SC 1.3.1) ; annonce d'une mention une seule fois et poliment (A-10) ; repère de
cellule CSV en forme (SC 1.4.1) ; échantillon de police `aria-hidden`, nom porteur (SC 1.1.1).

### R9-m7 — Quatre précisions ARIA (mineur)

1. **`aria-describedby` sur le texte commenté** (§4.27 `:1417`). Le texte vit dans le
   `contenteditable` de CodeMirror ; une description posée sur un `<span>` de décoration n'est pas
   lue quand le curseur y entre (les lecteurs d'écran lisent les descriptions sur l'élément
   **focalisé**, ici l'éditeur entier ; WAI-ARIA 1.2 §6.7, *aria-describedby* ; voir aussi la
   recommandation d'A-19 : l'éditeur est nommé, les curseurs ne sont pas annoncés). Correction :
   annoncer « Commentaire de Marie, 2 réponses » par `EditorView.announce` (API CodeMirror 6 pour
   cela) **à l'entrée du curseur** dans un passage commenté, une fois par fil, au rythme d'A-10 ;
   garder `aria-describedby` sur le **repère de marge** (un bouton, focalisable par Ctrl+Maj+N).
2. **Fil ouvert au survol ou au focus d'une cellule** (§4.27 `:1405`) : SC 1.4.13 exige
   fermable (Échap), survolable et persistant ; dire que c'est le popover de §4.7 (A-9).
3. **Barre « Vous suivez Marie · Arrêter »** (§4.47 `:2979–2981`) : elle porte un bouton, donc
   elle est dans l'ordre de tabulation ; A-18 `:3550` (panneau → onglets → barre de mise en forme
   → texte…) ne la place pas. L'ajouter après « onglets » (SC 2.4.3).
4. **« Vous êtes ici »** (§4.22 `:1235`) : la frise est une *Listbox* ; mettre ce mot dans le nom
   accessible de l'option ou `aria-current="true"` (SC 4.1.2). Et un seul libellé pour le sélecteur :
   §4.1 `:650` et la table §4.22 `:1216` disent « Original ⌄ », N46 `:1234` dit « Variante :
   Original » (H-4 : noms fixes).

---

## 5. Sûreté byzantine des opérations nouvelles

Référentiels : modèle de menace §9.2 `:2095–2117` (lecteur malveillant : « présence bornée »,
jamais « occuper des ressources au-delà de sa part » ; écrivain malveillant : jamais « bloquer,
faire diverger sans arbitre ») ; protocole §9.7 `:1278–1280` : « la garde décide en fonction de `c`
et de `before(c)` seulement, même verdict partout » ; ADR 0022 `:104` : refuser selon l'ordre
d'arrivée « ferait diverger les répliques » ; protocole §4.9 `:425–427` : une liste est « une
fonction de l'ensemble des ops, indépendante de l'ordre d'arrivée ».

### PD-5 (présence `variant`) — sûr

Même classe que PD-4 : éphémère, non autoritaire, `update` seul, expiration 45 s, budgets de
RV-5/RV-6. Un pair qui ment sur sa variante n'obtient qu'un curseur affiché au mauvais endroit ;
le nom vient de la liste signée. Rien à ajouter.

### PD-7 (document de commentaires) — R9-M4 et R9-m4

**R9-M4 — Une borne globale d'admission fige le document pour tous (majeur).** CM-6 `:1895–1896`
fait refuser par la garde « document ≤ 16 Mio ». C'est bien une fonction de `before(c)` (même
verdict partout, §9.7), mais c'est un **gel définitif** : une fois le passé causal au-delà de
16 Mio, **aucune** modification n'est plus admise, ni d'un admin, ni pour résoudre ou supprimer.
Et la borne par auteur « ≤ 2 000 objets **vivants** » ne borne pas l'**histoire** : un lecteur
(couvert par PD-7) modifie son propre message ≤ 4 Kio en boucle — 4 100 modifications licites,
chacune sur ses propres objets, et le document de commentaires du fichier est mort pour tout le
monde. Un seul lecteur suffit, sans équivoque ni sybil, donc sans arbitrage. C'est exactement ce que
§9.2 interdit au lecteur (« au-delà de sa part ») et à l'écrivain (« bloquer »). Correction
(texte, CM-6 et PD-7) : remplacer la borne globale par une borne **par auteur sur l'histoire**,
fonction de `before(c)` restreinte à la chaîne de l'auteur (G3) — par exemple ≤ 8 192
modifications et ≤ 8 Mio compressés par (auteur, document de commentaires) ; au-delà, refus des
**seules** modifications de cet auteur ; la taille totale devient un **palier de chargement local**
(ADR 0028, comme un document « énorme »), jamais un refus. Vecteur `repo.json#comments-author-history`.

**R9-m4 — Identité, admission, rôle et fourche (mineur).**
- `DocId = BLAKE3("inkwell/doc/v1" ‖ ProjectID ‖ "comments" ‖ DocId du texte)[:16]` (`:3834`)
  réutilise l'étiquette de §4.6 `:380` avec une **autre structure** d'entrée. Pas de collision
  (longueurs 68 et 111 octets), mais la séparation de domaines du protocole (§3.3 `:243–247` : un
  domaine par usage ; §3.4 `:257–266`) demande une étiquette propre, `inkwell/doc-comments/v1`, et
  une ligne au tableau §3.4.
- L'ensemble admis (§4.10 `:483–518`) ne contient le document dérivé par aucune clause : écrire
  `∪ { comments(d) : d ∈ admis } \ { comments(d) : doc/d = purged }` et dire que G11 (ancrage) ne
  s'applique pas (pas d'annonce, donc pas d'ancre), ou qu'une modification d'ancrage vide en tient
  lieu.
- « Changer `état` … si l'on écrit » (`:1893–1894`) dépend du **rôle**, que la garde structurelle ne
  connaît pas (§9.7 : `c` et `before(c)` seulement). Le mécanisme existant est la **couverture**
  (§9.4 étape 3 ; ADR 0022 : « on ne contrôle que les modifications non couvertes par un admin ») :
  dire que résoudre le fil d'autrui exige une attestation d'un appareil qui écrit, et que la garde
  ne vérifie que « auteur = clé qui signe » et « objets propres ».
- CM-4 `:1882–1886` : la fourche du document de commentaires se fait « avec » le fichier, mais à
  **quelles têtes** ? Le manifeste de départ (VA-8 `:1646`) n'enregistre que les documents de texte.
  Deux appareils qui fourchent à des moments différents emportent des fils de l'original postérieurs
  au départ, et leur union (même `DocId`) les fait entrer dans V : VA-4 (« rien ne fuit sinon »)
  n'est plus tenu pour les commentaires. Enregistrer les têtes du document de commentaires dans
  `meta.variant.depart`.

### PD-8 / PD-9 (validation) — R9-M5 et R9-m5

**R9-M5 — Le pli des opérations de validation n'est pas encore une fonction de l'ensemble
(majeur).** VD-2 `:1928–1935` et PD-8 `:3839–3844` :
1. `withdraw` « avant toute décision » et `decide` par la personne désignée peuvent être
   **concurrents** (aucun ne voit l'autre). Aucune règle ne dit l'état résultant : deux appareils
   peuvent afficher « Retirée » et « Validée ». Il faut une règle déterministe ; je recommande
   **la décision l'emporte** (c'est la déclaration signée de la personne désignée sur un contenu
   exact ; le retrait concurrent est montré « retrait arrivé après la décision »), l'inverse est
   acceptable aussi, pourvu qu'il soit écrit et vectorisé (`review.json#withdraw-concurrent-decide`).
2. « ≤ 64 soumissions **ouvertes** par auteur » dépend des décisions d'autres clés, donc de ce que
   chaque appareil a reçu. Évaluer « ouvertes » à la **frontière déclarée** par l'op (`decisions`
   `[32] × n`, déjà dans PD-8) : la règle redevient une fonction de l'op et de son passé, comme L5
   de l'ADR 0031.
3. « Deux décisions contraires … = équivoque (preuve existante, §4.8) » : §4.8 `:415–424` est la
   preuve pour les **tuples de modification Automerge** `(key, epoch, seq)`, pas pour une op de
   chaîne. Deux décisions à des `position` distinctes ne sont pas une équivoque : la première selon
   la chaîne gagne (VD-3, « définitive ») ; deux ops à la **même** `position` sont la fourche de
   chaîne que L3 et le **point de gel** de §4.10 `:514–516` traitent déjà. Citer cela, et réserver
   « contesté » au gel.
4. `assignee` : exiger qu'il soit **membre** (tout rôle) dans le pli de `members` de l'op, sinon la
   soumission est invalide (comme L4 pour l'auteur).

**R9-m5 — Encodage, domaine, vérité (mineur).**
- VD-1 `:1925` : « manifeste canonique » sans encodage nommé ; dire CBOR déterministe
  (RFC 8949 §4.2.1), comme PD-1 point 1.
- PD-8 : « domaine « review-op » » ; la forme exacte du protocole est
  `"inkwell/review-op/v1" ‖ 0x00 ‖ message` (§3.3), à écrire, et une ligne au registre §20.4.
- §6.1 `:777–803` : les soumissions et décisions sont une **vérité nouvelle** (ensemble signé) ;
  la règle du tableau (« toute nouvelle ligne exige une ADR », `:802`) est satisfaite par `AD-107` ;
  ajouter la ligne (vérité : ops `review_op_v1` ; ledger ; `.inkwell/sets/review/` ; vue dérivée :
  états VD-3, étiquettes Git VD-6).

VD-8 (ce que la signature prouve, eIDAS art. 3(10), 25, 41) est juste et prudent ; je n'ai rien à y
redire.

---

## 6. Ce que je confirme

- Les deltas de la conception (N43 à N48) et de l'architecture (N46 à N48) sont **cohérents sur le
  fond** : fourche par variante ↔ variante du projet entier, versions par variante ↔ VA-5/VA-6,
  commentaires par variante ↔ CM-4, statut de validation ↔ VD-3, ressources au partage et à
  l'export ↔ §6.13 (polices cochées, fichiers hors du dossier **non** cochés, rien à Rejoindre),
  ordre des polices projet > ordinateur > Inkwell ↔ FT-1/P-FONT.
- Les décisions de l'utilisatrice sont appliquées sans écart et ne sont pas rouvertes.
- Les cinq majeurs sont des **changements de texte** ; trois (R9-M4, R9-M5, R9-m4) doivent être
  faits **avant** que PD-7 et PD-8 entrent dans la boucle du protocole, car ils touchent les règles
  d'admission ; R9-M1 et R9-M2 avant le lot qui code les raccourcis et l'intégration.

**Verdict : ACCEPT** — 0 bloquant ; majeurs R9-M1 (⇧⌘K masque `deleteLine`), R9-M2 (point
d'intégration validé), R9-M3 (gestes de l'architecture absents de la conception), R9-M4 (borne
globale = gel du document de commentaires), R9-M5 (pli des ops de validation : retrait ∥ décision,
« ouvertes », équivoque, destinataire membre) ; mineurs R9-m1 à R9-m7.
