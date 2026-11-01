# Revue d’architecture — Codex, tour 7

2026-11-01 — `spec.md` **1.0.0-brouillon.8**, révision 7.

Revue documentaire ciblée de `spec.md`, de `CHANGES.md`, de
`12-codex-r6.md` et de `11-fable-r6.md`. Aucune revue du tour 7 consultée.
Seul ce fichier est créé ; aucun commit. Les références système ci-dessous
ont été consultées ; aucun résultat d’essai de P-VAULT ou P-SANDBOX n’est présumé.

Gravités : **bloquant** = garantie centrale non établie ; **majeur** = contrat
important contradictoire ou incomplet ; **mineur** = précision locale.

## 1. R6-B1 reste ouvert : l’exemption des destinations brutes invalide la branche Linux (b)

**R7-B1 — Bloquant.**

**Référentiel :** N37 (`docs/exigences.md:82`), SR-1/SR-5 et OV-1
(aucune représentation réversible de la phrase persistante en clair) ;
[interface officielle Linux d’hibernation en espace utilisateur](https://docs.kernel.org/power/userland-swsusp.html),
`SNAPSHOT_CREATE_IMAGE`, `read()` et paragraphe sur les supports de stockage.
Le noyau documente expressément une partition d’hibernation vierge comme
destination possible, sans obligation d’utiliser un swap.

**Preuves :** `spec.md:1352–1359` corrige bien la branche (a) : absence de
`disk` dans `/sys/power/state` ou lecture `[disabled]` dans `/sys/power/disk`.
L’absence de périphérique de reprise n’est plus présentée comme une preuve
de désactivation. En revanche, la branche (b), après avoir exigé le chiffrement
de toute destination locale, accepte explicitement un périphérique qui ne porte
« aucun système de fichiers ni swap » (`:1355–1356`). Cette exemption ne prouve
pas qu’il est impossible d’y stocker une image brute. `CHANGES.md:11` annonce
donc une fermeture plus forte que celle permise par le test écrit.

**Contre-exemple documentaire :** racine, dossier de la personne et magasins
de vidage sur LUKS ; swap absent ; hibernation possible ; `resume = 0:0` ;
un second disque interne vierge, sans système de fichiers ni swap, non amovible.
Un utilitaire système d’hibernation est configuré pour transférer l’image lue
depuis `/dev/snapshot` vers ce disque en clair. Le second disque passe
l’exemption explicite, mais peut recevoir les pages du processus de rendu.
Il ne s’agit ni d’un essai exécuté ni d’une session compromise : c’est le
mécanisme système que R6-B1 demandait précisément de couvrir.

**Correction requise :** supprimer l’assimilation « sans système de fichiers
ni swap = sans destination persistante ». Pour la branche (b), établir que
chaque destination effectivement utilisable par les mécanismes pris en charge,
y compris les partitions brutes, est protégée par LUKS ; configuration inconnue
ou destination brute en clair → refus. À défaut de preuve, n’accepter que la
branche (a). Décliner le témoin P-VAULT générique (`:1378–1379`) en témoin
explicite « disque interne vierge, destination brute en clair → refus ».

## 2. Fermetures et non-régressions vérifiées

Les lignes ci-dessous sont des constats de fermeture au contrat, sans anomalie
supplémentaire établie ; leur gravité résiduelle est **aucune**.

| Point | Référentiel cité | Preuves et conclusion |
|---|---|---|
| Profils d’énergie macOS — majeur du tour 6 | SR-5, OV-2 ; [manuel Apple de pmset](https://raw.githubusercontent.com/apple-oss-distributions/PowerManagement/main/pmset/pmset.1), distinction entre réglages courants et `-g custom` | `spec.md:1360–1366`, `CHANGES.md:12` : tous les profils batterie, secteur et onduleur sont vérifiés ; mode 0 ou volume de l’image vérifié ; profil illisible → refus. **Fermé.** |
| Windows §9.11 contre §9.5 — R6-M1 | INV-13, P-SANDBOX ; [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects), limites de processus | `spec.md:1703` et `:1799–1807`, `CHANGES.md:13` : les deux sections disent qu’un objet de tâche ne borne pas la taille des fichiers ; volume à quota ou refus. La porte expérimentale reste préalable. **Fermé.** |
| Résolution Linux, btrfs et fichiers d’échange — Fable R6-M1 | OV-1/OV-4 ; [proc_pid_mountinfo(5)](https://man7.org/linux/man-pages/man5/proc_pid_mountinfo.5.html), source et point de montage | `spec.md:1343–1351` : résolution par mountinfo au lieu du seul `st_dev` du chemin ; traitement des partitions ; chaque périphérique btrfs contrôlé ; swap de type fichier résolu comme chemin. `:1377–1378` conserve les trois témoins LUKS/clair. **Correction présente, aucune régression identifiée dans ce périmètre.** |
| Vidages Linux — Fable R6-M2 | SR-5, OV-5 ; [core(5)](https://man7.org/linux/man-pages/man5/core.5.html), exception des gestionnaires en tube à `RLIMIT_CORE` | `spec.md:1380–1385` pose la limite avant création de la webview et exige le volume vérifié du gestionnaire en tube, sinon refus. SR-5 (`:1097`, `:1105–1106`, `:1117–1121`) relie garantie, oracle et inspection des magasins après plantage du rendu. **Correction présente.** |
| Rapports WebView2 — Fable R6-M2 | SR-5, OV-4/OV-5 ; [Microsoft, gestion des échecs de processus WebView2](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/process-related-events), rapports de plantage | `spec.md:1373–1375`, `:1386–1388` : dossier de données sur volume vérifié, `FailureReportFolderPath` sous ce dossier, purge après cérémonie. La protection du volume couvre la persistance avant purge ; l’exclusion WER n’est plus l’unique réponse. **Correction présente.** |

## Verdict

Les deux majeurs Codex du tour 6 sont fermés. Les corrections demandées par
l’autre revue sont conservées. Le bloqueur Linux est réduit à la branche
« hibernation possible », mais reste ouvert : l’absence de système de fichiers
ou de swap ne protège pas une destination brute. Aucun nouveau majeur ni
mineur n’est établi dans le périmètre demandé.

NOT ACCEPTED — bloqueur : R7-B1 (R6-B1 non fermé), destination brute d’hibernation en clair admise par OV-1(b).
