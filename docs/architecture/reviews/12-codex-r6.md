# Revue d’architecture — Codex, tour 6

2026-11-01 — `spec.md` **1.0.0-brouillon.7**, révision 6.

Revue documentaire de la spécification, de `CHANGES.md` et de ma revue
`10-codex-r5.md`. Aucune revue concurrente du tour 6 consultée. Seul ce fichier
est créé ; aucun commit. Les conclusions portent sur le contrat, sans présumer
la réussite des portes expérimentales (§0.5 E-1 à E-5, lignes 49–65).

Gravités : **bloquant** = garantie centrale non établie ; **majeur** = contrat
important contradictoire ou incomplet ; **mineur** = correction locale.
Les référentiels cités comprennent les exigences internes et les interfaces
officielles des systèmes ; ces dernières ne sont pas des résultats d’essai.

## 1. R5-B1 : fermeture de principe, oracle Linux encore insuffisant

**Correction acquise, sans anomalie sur le principe.** SR-5 (§6.7,
lignes 1092–1119) couvre création, affichage, saisie et transport ; reconnaît les
chaînes non effaçables et non verrouillables du moteur ; exige le contrôle du
swap **et** de l’hibernation avant toute cérémonie et refuse en cas d’échec ou
d’illisibilité. P-VAULT recherche les mots et les octets sur les supports après
pression mémoire, échec du verrouillage et plantage. Le retrait du DOM ne vaut
plus zéroïsation. Cela répond bien au changement annoncé dans `CHANGES.md:11`.

### R6-B1 — Bloquant : `resume = 0:0` ne prouve pas l’absence d’hibernation persistante

**Référentiel :** N37, SR-1, SR-5 (aucune représentation réversible du secret
persistante en clair), OV-1 ; [ABI Linux de gestion d’énergie](https://raw.githubusercontent.com/torvalds/linux/master/Documentation/ABI/testing/sysfs-power),
entrée `/sys/power/state` ; [interface officielle d’hibernation en espace utilisateur](https://docs.kernel.org/power/userland-swsusp.html),
`SNAPSHOT_CREATE_IMAGE`, `read()` et paragraphe sur les destinations de l’image.
Cette interface permet à un utilitaire de récupérer l’image et de la stocker
ailleurs que dans le swap, notamment dans une partition dédiée ou un fichier.

**Preuves :** §6.10 OV-1, lignes 1336–1341, accepte l’hibernation dès que
`/sys/power/resume` vaut `0:0`, sans vérifier que la création d’une image est
désactivée ni couvrir les utilitaires qui choisissent eux-mêmes sa destination.
SR-5, lignes 1103–1109, accorde pourtant sa garantie à partir de cet oracle.

**Contre-exemple de contrat, à tester :** swap absent ou zram sans repli,
`resume = 0:0`, utilitaire d’hibernation en espace utilisateur configuré pour
écrire une image sur une destination non chiffrée. Les conditions écrites
passent ; l’interface documentée permet néanmoins une copie persistante de la
mémoire de la webview. Il ne faut ni pagination effective vers le swap, ni
session compromise pour que cette configuration invalide la preuve proposée.
Ce scénario n’est pas une observation du code ni une affirmation de fréquence.

**Correction requise :** ne pas assimiler l’absence de périphérique de reprise
à une désactivation de l’hibernation. Définir les mécanismes pris en charge,
prouver leur désactivation effective ou contrôler toutes leurs destinations ;
configuration non déterminable = refus. Ajouter ce témoin à P-VAULT. La même
garantie doit rester vraie tant que des copies non effaçables peuvent subsister
dans le processus de rendu (SR-5, lignes 1113–1115).

**Par OS :** sous Linux, R5-B1 reste donc ouvert via R6-B1. Sous macOS, le swap
chiffré et l’image sur volume vérifié constituent une voie cohérente au contrat
(OV-2, lignes 1342–1346) ; Apple confirme le rôle du
[chiffrement de la mémoire virtuelle](https://support.apple.com/en-euro/guide/mac-help/mh11852/27/mac/27).
La branche `hibernatemode = 0` doit être validée sur les configurations réellement
admises : le [manuel Apple de pmset](https://raw.githubusercontent.com/apple-oss-distributions/PowerManagement/main/pmset/pmset.1)
distingue les profils batterie/secteur et les réglages courants (`-g`) de tous les
profils (`-g custom`). Le contrôle ponctuel ne constitue pas une preuve pour un
autre profil ultérieurement actif. **Gravité : majeur**, précision de portée
manquante dans OV-2 ; exiger tous les profils applicables ou le volume chiffré.
Sous Windows, OV-3 couvre chaque fichier d’échange et le fichier d’hibernation
sur le volume système, ou leur désactivation (lignes 1347–1350) : aucune nouvelle
faille identifiée dans cette règle, efficacité à prouver par P-VAULT. Microsoft
documente la protection des fichiers de pagination et d’hibernation dans les
[contre-mesures BitLocker](https://learn.microsoft.com/windows/security/operating-system-security/data-protection/bitlocker/countermeasures).

## 2. R5-B2 et R5-M1 : fermés au contrat

| Point | Référentiel et preuves | Conclusion / gravité |
|---|---|---|
| Volume effectif | N37, VK-1, EN-4 ; §6.10, lignes 1331–1356 ; `CHANGES.md:12` | **R5-B2 fermé**, aucune anomalie supplémentaire établie sur la liaison au volume. Chaque chemin sensible inclut désormais dossier, clé X25519 en mode disque, swap et image d’hibernation. macOS résout le volume APFS du chemin et rejette explicitement le seul statut global FileVault. Windows résout le volume du chemin avant interrogation BitLocker. OV-4 impose placement contrôlé et témoins externe en clair, montage modifié, état inconnu. |
| Linux et zram | OV-1, lignes 1336–1341 ; [documentation officielle zram, writeback](https://kernel.org/doc/html/v5.19/admin-guide/blockdev/zram.html) | **Correction acquise** : sysfs remplace `lsblk` ; zram est accepté sans repli ou avec repli `CRYPT-`, jamais avec repli en clair. La documentation confirme que zram peut écrire ses pages sur ce repli. La réserve R6-B1 concerne l’hibernation, distincte de ce contrôle. |
| Windows, état protégé | OV-3, lignes 1347–1350 ; [GetProtectionStatus](https://learn.microsoft.com/en-us/windows/win32/secprov/getprotectionstatus-win32-encryptablevolume) | **Cohérent** : la valeur 1 signifie volume complètement chiffré et clé non disponible en clair sur disque pour un disque standard ; le statut n’est plus emprunté à un autre volume. |
| Trousseau | SR-1/SR-2, VK-1/VK-2 ; §9.4, lignes 1641–1643 et 1651–1655 ; `CHANGES.md:13` | **R5-M1 fermé**, aucune anomalie identifiée : clé privée X25519 de l’appareil au trousseau ; jamais le secret de secours ; DEK dans l’objet age du projet. |

## 3. Non-régressions demandées

| Point | Référentiel et preuves | Conclusion / gravité |
|---|---|---|
| Préchauffage | N38, H-3/H-4, P-PLUG-BENCH ; §0.5 ligne 70, §9.10 H-4 ligne 1727, §12.5 ligne 1968 ; `CHANGES.md:14` | **Aucune régression identifiée.** Après première peinture et dernier projet modifiable, priorité basse, seulement si repli engagé, ≤ 6 hôtes ; PERF-START avec six greffons épinglés. Cache disque toujours soumis à ADR. |
| « Tout rechiffrer » | N37, EN-2, §9.4 ; §6.10 lignes 1290–1301 ; `CHANGES.md:16` | **Aucune régression identifiée.** La récupération ne prétend plus révoquer les copies de l’ancien appareil et propose nouvelle DEK, époque et rechiffrement des objets. Le vecteur doit porter sur les objets nouvellement rechiffrés : les copies anciennes déjà détenues restent lisibles (EN-7, ligne 1385). |

### R6-M1 — Majeur : la correction Windows reste contredite au §9.11

**Référentiel :** unicité du contrat (§0.3, INV-3), INV-13, P-SANDBOX ;
[Microsoft, Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
(limites de processus), et §9.5 qui distingue explicitement ces limites de la
taille des fichiers.

**Preuves :** §9.5, lignes 1669–1670, corrige bien le texte : quota sur le
volume ou refus, `MaxOutput` seulement après production. Mais §9.11,
lignes 1765–1769, affirme encore « sortie bornée par rlimit (Linux, macOS) ou
objet de tâche (Windows) ». `CHANGES.md:17` n’a donc pas une fermeture complète.

**Correction requise :** aligner ce résumé sur le quota prouvé ou le refus de
§9.5. La porte P-SANDBOX et le refus avant preuve restent présents ; ce défaut
de texte n’ajoute pas un second bloqueur de confidentialité.

## Verdict

La liaison FileVault au volume et le trousseau sont corrigés. SR-5 introduit le
bon refus, mais l’oracle Linux ne démontre pas encore sa prémisse ; préciser
aussi les profils d’énergie macOS. Préchauffage et récupération sont cohérents ;
le résumé Windows doit être aligné. Aucune porte expérimentale n’est présumée passée.

NOT ACCEPTED — bloqueur : R6-B1, absence de périphérique de reprise Linux assimilée à tort à l’absence d’hibernation persistante en clair.
