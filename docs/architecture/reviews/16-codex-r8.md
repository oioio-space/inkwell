# Revue d’architecture — Codex, tour 8

2026-11-01 — `spec.md` **1.0.0-brouillon.10**, révision **8.1**.

Revue documentaire de `spec.md`, `CHANGES.md`, `decisions.md`, `coherence.md`,
des exigences N42–N45 et du précédent avis `14-codex-r7.md`. Vérifications
ponctuelles du code et du CDDL de présence. Aucune revue concurrente du tour 8
consultée. Seul ce compte rendu est créé ; aucun commit ni essai du produit.

Gravités : **bloquant** = décision d’architecture inacceptable ; **majeur** =
contrat fonctionnel à corriger avant réalisation ; **mineur** = précision locale.
L’acceptation reste celle du §0.5 : elle ne vaut pas passage des portes expérimentales.

## 1. R7-B1 : fermé par correction et acceptation explicite du résiduel

**Gravité résiduelle : aucune anomalie bloquante.**

**Référentiel :** décision utilisatrice **D-17**, `AD-91` révision 8 ; OV-1,
SR-5 et §9.9. L’[interface officielle Linux d’hibernation en espace utilisateur](https://docs.kernel.org/power/userland-swsusp.html)
permet toujours de transférer l’image obtenue depuis `/dev/snapshot` vers un
support choisi : l’application ne prouve donc pas le chiffrement de toute
destination imaginable.

**Preuves :** `spec.md:1365–1398` remplace l’exemption erronée des partitions
brutes par un contrôle des mécanismes connus : chaque swap actif, périphérique
de reprise, configurations uswsusp et absence de TuxOnIce. Le verrou
`Inhibit("sleep", …, "block")` et la fin du processus de rendu dédié sont
explicitement requis pendant la cérémonie ; leur réalisation reste à prouver
par P-VAULT. OV-4 ajoute les témoins uswsusp en clair, reprise en clair et
hibernation pendant la cérémonie (`:1412–1419`).

Le §9.9 (`:1957–1965`) décrit bien la possibilité restante : outil privilégié
hors des mécanismes contrôlés, contournement de logind, image brute en clair
pendant la cérémonie. Ce risque n’est ni déclaré impossible ni présenté comme
cryptographiquement empêché. Le qualifier de hors du modèle du Voleur signifie
une exclusion de périmètre, pas une protection contre les conséquences d’un
outil déjà installé. D-17 accepte précisément cette exclusion ; je ne rouvre
pas le choix strict rejeté par l’utilisatrice. **R7-B1 est clos.**

## 2. Vérifications des nouveaux contrats

Les constats suivants ont une **gravité : aucune**, sous les réserves de la section 3.

| Point | Référentiel | Preuves et conclusion |
|---|---|---|
| N42 : programmes externes | N42, INV-14, AC-18/19 | `CHANGES.md`, section « chaque appel remplacé » ; spec §4.2 et §9.5 : espaces de noms, montages, limites et Landlock passent par notre lanceur ; oracles par API ; intégration au bureau par API ; Git et age par bibliothèques. `tar` et `git check-ignore` restent limités à la construction et aux tests. §16 reconnaît les écarts actuels : aucune conformité du code n’est inventée. |
| N42 : dépendances garanties | §14.1 DG-1 à DG-5, P-DEPS | Table des composants, plancher glibc, dépendances des paquets, liste des cibles et contrôles de liaison ; vérification avant fenêtre et lanceur statique avant le chargeur dynamique. Les bibliothèques Linux nécessaires à l’archive sont explicitement des prérequis détectés, pas des composants prétendument embarqués. Une cible non prouvée est retirée de la promesse. |
| WebView2 hors ligne | DG-5 ; [Microsoft, distribution de WebView2](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution#offline-deployment) | §14.1 prévoit l’installateur autonome Evergreen embarqué, exécuté à l’installation, et le test Windows sans runtime et sans réseau. DG-1 détecte ensuite l’absence sans téléchargement réparateur. Le remplacement de l’amorce actuelle est bien déclaré restant à faire. |
| Protection facultative | Décision utilisatrice ; EN-0, AD-91 | `spec.md:1274–1278`, matrice des décisions et `coherence.md` §6, point 9 : jamais cochée par défaut, même partagé ; activation volontaire. Aucun retour implicite à une protection obligatoire. |
| N43 : priorité des polices | N43, FT-1/2, P-FONT | §6.12 impose projet → inkwell → personnel → ordinateur. Le [code officiel de Typst 0.15.1](https://raw.githubusercontent.com/typst/typst/v0.15.1/crates/typst-cli/src/fonts.rs) confirme l’ajout système, embarqué, puis chemins : les deux drapeaux de désactivation sont nécessaires. La même liste est exigée pour tinymist 0.15.10, avec test d’égalité et de collision. Les mesures déclarées sur l’appareil de développement ne sont pas des résultats P-FONT reproduits par cette revue ; la source de `getDocumentMetrics` n’a pas pu être récupérée indépendamment. La porte exige justement cette preuve, replis de glyphes compris. |
| N43 : partage et export | §6.13 RA-1 à RA-4, AD-101/102 | Une seule étape aux deux points d’entrée ; ressources externes non cochées, chemin montré, types et tailles bornés ; contrôle hors réseau sans sources locales. RA-T1 exige export clair et chiffré, import sur image propre et SHA-256 du PDF identique, dates fixées. Polices et greffons disponibles dans le projet, installation personnelle volontaire ; aucune gestion de licences ajoutée. |
| N44 : faisabilité et repli | N44, P-SYNC, SY-1 à SY-8 | §15.17 distingue API interne Typst, exposition manquante par stdio et aperçu web écarté. LSP amont ou notre sidecar résident sont des options à prouver ; les ancres au paragraphe constituent le repli déclaré. Révisions des réponses, arbitrage des entrées humaines et budgets sont spécifiés. Le repli n’est pas présenté comme un saut exact au caractère. |
| N45 : vue stable et suivi | N45, RV-1 à RV-8 | Ancrage Automerge du premier caractère visible, sélection appliquée à travers les changements, absence d’annulation distante et arrêt du suivi à l’entrée locale. Présence hostile bornée et nom issu de la liste signée. L’aperçu conserve également son bloc visible. RV-2 doit se lire pour le mode ordinaire ; RV-3 est l’exception volontaire du suivi. |
| Œil du suiveur | AD-104, PD-4 ; protocole `cddl/repo.cddl`, `channel-pair` et `snapshot-state` | Le CDDL actuel ferme les canaux et l’état du snapshot. Ajouter `follow` seulement en update évite de perdre aussi nom et curseur sur les anciens clients. PD-4 reste proposé à la boucle du protocole ; RV-4 ne prétend ni que l’œil existe déjà ni qu’il empêche l’observation par un client modifié. |

## 3. Constats à traiter, non bloquants pour l’acceptation documentaire

### R8-M1 — La table de synchronisation suppose un ordre global qui n’existe pas toujours

**Sévérité : majeur, avant passage de P-SYNC.**

**Référentiel :** N44 ; MF-10, SY-3, SY-6 et SY-8 ;
[Typst 0.15.1, API de saut](https://raw.githubusercontent.com/typst/typst/v0.15.1/crates/typst-ide/src/jump.rs).

**Preuves :** `spec.md:3211–3216` réduit la table à `(ligne source, page, y)`
et propose la recherche inverse ; `:3235–3237` demande la monotonie. Or SY-6
admet plusieurs fichiers et plusieurs occurrences. Deux fichiers ont chacun
une ligne 10 ; deux colonnes peuvent partager le même y ; un contenu stocké
puis rendu ailleurs inverse l’ordre source/rendu. Une seule liste globalement
monotone ne représente pas ces cas. L’API amont conserve l’identité du fichier
et des positions physiques plus riches.

**Correction :** préciser nœud, révision, occurrence et coordonnées utiles ;
index source et rendu séparés, interpolation seulement entre segments
compatibles. Ajouter à P-SYNC les colonnes, le contenu réordonné et les
inclusions répétées. Ce défaut ne condamne ni les API choisies ni le repli au
paragraphe ; il empêche de considérer le contrat SY-3 comme déjà complet.

### R8-m1 — AC-18 interdit le lancement que DG-2 exige

**Sévérité : mineur. Référentiel :** N42, INV-14, AC-18, DG-2.

**Preuves :** `spec.md:677` réserve `unix.Exec` à `internal/sandbox`, alors que
`:570–573` et `:2374–2382` le demandent dans `cmd/inkwell-launch`, qui n’importe
que la bibliothèque standard et `x/sys`. INV-14 autorise déjà cette exception.

**Correction :** reporter dans AC-18 l’exception étroite du lanceur, limitée
à l’exécutable installé d’inkwell ; conserver l’interdiction générale des outils
externes. Le contrôle architectural ne doit pas rejeter son propre mécanisme.

### R8-m2 — PD-4 doit définir le rafraîchissement de l’annonce

**Sévérité : mineur, à résoudre dans la boucle du protocole. Référentiel :**
N45, RV-4/5, PD-4 ; présence éphémère du protocole §9.5.

**Preuves :** `spec.md:3509–3518` exclut volontairement `follow` du snapshot et
fixe un maximum d’émission, sans préciser renouvellement, réémission à la
reconnexion ni expiration propre. Une annonce perdue n’est donc pas réparée
par un snapshot ; un arrêt perdu peut laisser un œil périmé tant que le pair
reste présent.

**Correction :** définir émission initiale, renouvellement ou durée de validité,
arrêt et reconnexion, sans réintroduire le champ dans le snapshot 2.0 ; ajouter
les vecteurs perte/reconnexion. Le suivi lui-même ne dépend pas de cet indicateur.

### R8-m3 — Deux formulations héritées restent à aligner

**Sévérité : mineur. Référentiel :** N36, EN-4, D-17 et OV-1.

**Preuves :** EN-9 (`spec.md:1444`) et C03 de `coherence.md` gardent le disque
en mémoire macOS désormais supprimé par EN-4. SR-1/SR-5 (`spec.md:1090`,
`:1119`) expriment encore la garantie sans renvoi immédiat au résiduel D-17.
Enfin §14.1 rend logind facultatif alors qu’OV-1 exige son verrou pendant la
cérémonie.

**Correction :** retirer le support macOS obsolète ; ajouter à SR-5 un renvoi
au périmètre D-17 ; préciser le refus de la cérémonie lorsque le verrou requis
ne peut être obtenu, ou expliciter l’exception retenue. Ces ambiguïtés locales
ne rendent pas mensongère la description explicite du résiduel au §9.9.

## 4. Non-régression et verdict

Les oracles restent liés aux volumes, avec échec fermé ; les fichiers d’échange
et tous les périphériques btrfs restent couverts. Les profils d’énergie macOS,
les vidages et le placement des rapports WebView2 sont conservés. §9.5 et §9.11
gardent le quota disque ou le refus sous Windows, sans attribuer cette garantie
aux objets de tâche. Les nouveaux dossiers de polices sont ouverts en lecture
seule ; l’accès aux fichiers externes exige toujours un geste. Aucun nouveau
port ni changement implicite du protocole accepté n’est autorisé.

Le bloqueur du tour 7 est fermé, le résiduel accepté est honnêtement décrit,
et aucune régression bloquante n’est établie. Les constats ci-dessus doivent
alimenter les corrections et les portes correspondantes ; aucune porte n’est
déclarée passée par cette revue.

ACCEPT
