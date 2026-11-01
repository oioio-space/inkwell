# Revue d’architecture — Codex, tour 5

2026-11-01 — architecture **1.0.0-brouillon.6**, révision 5.

Revue documentaire de `spec.md`, `CHANGES.md`, `decisions.md` et de ma revue
`08-codex-r4.md`, centrée sur les fermetures et régressions demandées. Aucune revue
concurrente du tour 5 consultée. Aucun essai de sécurité ou de performance exécuté :
une fermeture au contrat ne signifie pas que les portes expérimentales sont passées.
Seul ce rapport est créé, sans commit ; les modifications préexistantes du design
ne font pas partie de cette revue.

Gravités : **bloquant**, garantie centrale non tenue ; **majeur**, contrat important
incomplet ou contradictoire ; **mineur**, correction locale.

## 1. R4-B1 : réparation réelle, fermeture encore incomplète

**Référentiel :** N37, modèle « Voleur » (§9.2), SR-1/SR-2 et VK-1 à VK-3.
**Preuves :** `spec.md` §6.1, ligne « Secret de secours » (ligne 777), §6.7
SR-1 à SR-4 (1073–1091), §6.10 EN-3 (1270–1288) ; `decisions.md` AD-67 et
AD-91 révision 5 ; `CHANGES.md`, entrée brouillon.6, Codex R4-B1.

La sauvegarde volontaire en fichier secret nu est interdite. L’appareil conserve
la clé publique de secours et les sous-clés de sauvegarde enveloppées ; la clé
privée X25519 dépend du trousseau, du mot de passe Argon2id aux paramètres fixés,
ou du disque chiffré vérifié. Aucune voie automatique `0600` n’est autorisée pour
un projet protégé. **L’ancien défaut de stockage explicite est corrigé dans SR/VK**.
Deux limites empêchent cependant de fermer la garantie globale : R5-B1 et R5-B2.

### R5-B1 — Bloquant : la phrase peut encore laisser une copie persistante par pagination

**Référentiel :** N37 ; SR-1 (« jamais sur le disque », y compris une forme dont
le secret se dérive), VK-2 ; [Linux man-pages, mlock(2), DESCRIPTION, ERRORS et
NOTES](https://man7.org/linux/man-pages/man2/mlock.2.html) : le verrouillage protège
les pages concernées contre le swap, peut échouer et ne supprime pas tous les
risques de persistance, notamment l’hibernation.

**Preuves :** `spec.md` §6.7 ST-3 définit les 24 mots comme encodage du secret ;
SR-1 (1077–1078) transmet ces mots à la webview et les retire du DOM ; SR-3
(1083–1089) ne verrouille les pages que « quand le système l’accorde » et exclut
expressément la mémoire de la webview de la garantie. EN-4 (§6.10) vérifie le swap
pour le dossier de travail, sans conditionner toute création ou saisie de phrase
à cette vérification. L’édition reste possible sans matérialisation.

**Contre-exemple de contrat, non observation du code :** création ou restauration
sous Linux avec swap en clair ; verrouillage refusé, ou pages de la phrase dans
le processus web non verrouillées ; pagination de ces pages pendant le dialogue.
L’effacement du tampon Go et le retrait du DOM ne suppriment pas la copie déjà
écrite sur disque. Après vol hors ligne, retrouver les mots suffit pour dériver
le destinataire de secours et contourner le mot de passe local. La survenue
effective de cette pagination est une **hypothèse à provoquer en essai**, mais
aucune règle ne refuse ce parcours. Une compromission de session ouverte n’est
pas nécessaire : le problème est la copie restante après son extinction.

**Correction requise :** couvrir toute représentation réversible de la phrase,
y compris saisie, affichage et transport. Avant de la créer ou saisir, imposer
une voie mémoire maîtrisée avec verrouillage réussi, ou un environnement de
pagination et de suspension vérifié qui ne conserve pas de clair ; sinon refuser
ce parcours. Ne pas présenter le retrait du DOM comme une zéroïsation des copies
du moteur. Ajouter à P-VAULT pression mémoire, échec du verrouillage, fermeture,
plantage et inspection hors ligne du swap avec recherche des mots, pas seulement
des octets du secret. L’exception EN-7 pour la webview ne suffit pas à prouver SR-1.

L’exclusion des captures est bien demandée dans SR-3 lorsque l’OS le permet,
avec limite Linux affichée. **Gravité de ce sous-point : aucune anomalie de contrat
supplémentaire identifiée** ; efficacité à valider par version d’OS, sans assimiler
une API nommée à un résultat d’essai.

### R5-B2 — Bloquant : l’oracle FileVault n’est pas lié au volume qui contient le clair ou la clé

**Référentiel :** N37 ; EN-4 et VK-1 exigent le chiffrement du support effectif ;
[Apple, protection des données par chiffrement](https://support.apple.com/en-ca/guide/mac-help/mh40593/27/mac/27)
et [Apple, chiffrement d’un support dans Utilitaire de disque](https://support.apple.com/en-asia/guide/disk-utility/dskutl35612/mac)
distinguent la protection des supports, notamment externes.

**Preuves :** `spec.md` §6.10 EN-4, tableau macOS (1296) et oracle (1303–1304),
accepte `fdesetup status` = « FileVault is On », sans résolution du volume du
dossier. VK-1 (1273–1278) réutilise cet oracle pour autoriser le stockage de la
clé privée sans enveloppe par mot de passe. AD-91 révision 5 reproduit cet oracle.
À l’inverse, les clauses Linux et Windows désignent le support du dossier.

**Contre-exemple hypothétique à tester :** FileVault actif sur le volume de
démarrage, dossier de travail sur un volume externe non chiffré. Le statut global
proposé ne prouve pas le chiffrement de cet autre volume ; pourtant il pourrait
autoriser sa matérialisation. Le même défaut toucherait une clé VK-1 placée sur
un autre volume. Ce n’est pas l’affirmation que le code choisit actuellement
ce placement : le contrat n’établit pas la liaison nécessaire.

**Correction requise :** résoudre chaque chemin sensible vers son volume réel,
vérifier le chiffrement complet et la protection de ses clés sur ce volume, ou
restreindre explicitement le placement au volume effectivement contrôlé. Tester
le témoin « démarrage chiffré, destination externe en clair », les changements
de montage et les états inconnus ; résultat attendu : refus. Vérifier aussi le
support de la clé privée, pas seulement celui du dossier de travail.

## 2. Majeur et mineur du tour 4

| Constat | Référentiel et preuves | Verdict / gravité |
|---|---|---|
| R4-M1, chronomètre PM-4 | N38, principes d’ingénierie §3 ; `spec.md` §12.5 PM-4 (1942–1951), AD-93 révision 5 : gestionnaire, attente du rappel et affichage séparés ; oracle par moteur ou externe, témoin de mise en page lente, échec sans oracle validé | **Majeur fermé au contrat.** Le simple ou double rAF ne vaut plus preuve. La validation doit porter sur l’affichage de la modification concernée, comme le demande le texte. |
| R4-m1, résumés explicitement signalés | N18/N36, ordre des sources §0.3 ; `spec.md` §3.2 (267), annexe Décision 6 (3236–3238) ; `decisions.md` AD-83/91/95/96 marquent les recommandations initiales remplacées | **Mineur fermé sur ces occurrences.** Un autre résumé reste contradictoire : R5-M1. |

### R5-M1 — Majeur : §9.4 ordonne encore de conserver le secret de secours

**Référentiel :** N18/N36, unicité du contrat (§0.3), SR-1/SR-2, VK-2.
**Preuve :** `spec.md` §9.4 (1599–1601) indique toujours que le trousseau garde
« le secret de secours et l’enveloppe locale de la DEK », contrairement à §6.1,
SR-1 et VK-2 ; `CHANGES.md` annonce pourtant le secret absent de l’appareil.
Le trousseau protégé ne constitue pas à lui seul l’ancien contournement par
fichier nu : ce constat n’ajoute donc pas un bloqueur de déchiffrement hors ligne.
Il permet néanmoins une implémentation incompatible avec l’exigence explicite
d’absence du secret persistant. Remplacer cette phrase par le stockage de la
clé privée d’appareil selon VK-1 et des seules sous-clés enveloppées selon SR-2.

## 3. Autres changements : contrôle de non-régression

| Point | Référentiel ; preuves | Conclusion / gravité |
|---|---|---|
| Préchauffage des greffons | N7/N38, H-3/H-4, E-1 à E-4 ; `spec.md` §0.5 P-PLUG-BENCH, §9.10, §15.4, annexe P-PLUG-BENCH ; `CHANGES.md` Fable R4-M1 | **Aucune nouvelle anomalie identifiée.** Compilation en mémoire dans l’hôte, après première peinture, ≤ 6 actifs ; cache disque seulement après ADR. Le message d’attente ne prouve pas le budget froid : le repli doit être mesuré après relance et éviction ; si insuffisant, revue exigée, budgets inchangés. |
| Récupération sans ancien appareil | N37, ADR 0029/non-perte, ST-7 ; `spec.md` §6.10 EN-2 (1257–1269), AD-91 révision 5 | **Correction acceptée au contrat.** Exception au contrôle `peer` limitée au destinataire de secours, projet toujours vérifié, ancien appareil affiché, ré-enveloppement époque + 1 pour le nouveau destinataire et le secours. Ce changement ne restitue pas les droits d’admin (ST-8), ni ne révoque cryptographiquement les copies déjà détenues par l’ancien appareil (EN-7). Le vecteur « ancien appareil refusé » porte sur l’admission locale de l’enveloppe restaurée. |
| Oracles disque/swap | N37, EN-4, E-2/E-3 ; `spec.md` §6.10 (1299–1308) ; AD-91 révision 5 | **Partiellement accepté**, réserve bloquante R5-B2 et portée insuffisante pour la phrase R5-B1. Linux nomme les périphériques swap et l’ascendance chiffrée ; Windows vise le volume du dossier ; oracle illisible = refus ; macOS exige aussi le swap chiffré pour son disque mémoire. Les montages composites restent à éprouver dans P-VAULT, sans crédit anticipé. |
| Plafonds de sortie des sidecars | N7/N32/N38, ADR 0033, INV-13 ; `spec.md` §0.5 P-SANDBOX et §9.5 (1616–1617), AD-95 révision 5 | **Aucune nouvelle anomalie identifiée au contrat.** 256 Mio par compilation, 512 Mio simultanés, admission par le parent ; la ligne suivante exige toujours le confinement des écritures par le noyau, sinon lancement borné refusé. Une limite de taille par fichier ne remplace pas le plafond cumulé : le corpus hostile de la porte doit exercer plusieurs fichiers et compilations. Les cibles non prouvées restent refusées. |

## Verdict

PM-4 et les occurrences du mineur R4 sont corrigés. L’interdiction du fichier
secret nu et l’enveloppement des sous-clés sont acquis au contrat ; la garantie
« secret seulement en mémoire, verrouillé et effacé » ne l’est pas pour toutes
ses représentations. Le nouvel oracle macOS ne prouve pas le support effectif.
Les portes restent expérimentales sur les six cibles, sans résultat présumé.

**NOT ACCEPTED — bloqueurs : R5-B1 (persistance possible de la phrase par pagination) ; R5-B2 (oracle FileVault non lié au volume sensible).**
