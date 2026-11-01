# Revue Fable n°7 de l'architecture (spec 1.0.0-brouillon.8, révision 7, 2026-11-01) — verdict : **ACCEPT** (0 bloquant, 1 majeur de texte, 4 mineurs)

Portée : `spec.md` (brouillon.8), `CHANGES.md`, `decisions.md`, `traceability.md`, `coherence.md`,
tels que commis en `4d2baf2` (diff complet `b939e24..4d2baf2` relu ligne à ligne ; `docs/protocol/`,
`docs/design/`, `docs/adr/` intouchés). Non lu : la revue Codex du tour 7. Lu : `12-codex-r6.md`,
pour rejouer son bloquant et ses deux majeurs sur le texte nouveau. Référentiels : règles E-1 à E-5
du §0.5, ADR 0033 (échec fermé), N37, modèle « Voleur » (§9.2), `proc_pid_mountinfo(5)`, `stat(2)`,
`core(5)`, `systemd-coredump(8)`, noyau `Documentation/power/userland-swsusp` (docs.kernel.org),
`uswsusp.conf(5)` (manpages.debian.org), Microsoft Learn (`CoreWebView2Environment.FailureReportFolderPath`),
`pmset(1)` (cité par Codex). Mesures faites sur cet hôte (Fedora 44, UEFI, `/sys/power`, `/sys/block`,
`/proc/self/mountinfo`, `/run/udev/data`, `core_pattern`) pour les oracles Linux.
Gravités : **bloquant** = garantie centrale contredite ou porte qui ne peut ni passer ni échouer ;
**majeur** = texte qui doit changer avant qu'un lot s'en serve ; **mineur** = précision.

---

## 1. Mes constats du tour 6 : tous fermés

| # | Constat | Où c'est fermé (`spec.md`) | Vérifié |
|---|---|---|---|
| R6-M1 | OV-1 ne passe ni sur btrfs ni sur un fichier d'échange | `:1343–1348` : volume par `/proc/self/mountinfo` (point de montage le plus long → source du montage), partition remontée au disque parent avant `slaves/`, btrfs : **chaque** périphérique de `/sys/fs/btrfs/<uuid>/devices/` ; `:1349–1351` : entrée `Type: file` de `/proc/swaps` résolue comme un chemin ; témoins OV-4 `:1377–1379` (« btrfs sur LUKS → accepté », « fichier d'échange sur LUKS → accepté », « btrfs en clair → refus ») | oui — rejoué ici : `mountinfo` donne `/home` → `btrfs /dev/nvme0n1p3`, `/sys/fs/btrfs/8eb9d031-…/devices/` = `nvme0n1p3`, sans `slaves/` vers un `CRYPT-` → « btrfs en clair → refus », conforme |
| R6-M2 | vidage sur plantage du processus de rendu | **OV-5** `:1380–1388` : `RLIMIT_CORE = 0` **avant la création de la webview**, `PR_SET_DUMPABLE = 0` pour Go pendant la cérémonie, volume de destination du gestionnaire en tube vérifié sinon refus, WebView2 purgé, `DiagnosticReports` sur volume vérifié ; SR-5 `:1096–1098` renvoie à OV-5 et la garantie minimale nomme « pagination **et** vidage » (`:1105–1106`) ; P-VAULT `:1118–1119` tue le processus de rendu pendant la cérémonie et fouille les magasins d'OV-5 | oui |
| R6-m1 | « Protéger ce projet » dépend aussi de SR-5 ; hibernation au déverrouillage | VK-1 `:1309–1312` ; §0.5 P-VAULT `:72` (repli) ; §9.2 Voleur `:1656` (« dit à l'écran », non bloquant pour l'édition, bloquant pour cérémonie et matérialisation) | oui |
| R6-m2 | oracles macOS et Windows non nommés | OV-2 `:1360–1361` (plist analysée structurellement, clés à confirmer) ; OV-3 `:1369–1372` (`NtfsEncryptPagingFile`, `HibernateEnabled`) ; OV-1 `:1353` (`/sys/power/disk` = `[disabled]`) | oui — un nom de porte reste à définir (R7-m3) |
| R6-m3 | dossier de données de la webview hors d'OV-4 | OV-4 `:1373–1375` | oui |
| R6-m4 | `MaxOutput` sous Linux | §9.5 `:1703` (dimensionne la borne noyau, `engine.go:484`, `mount_linux.go:95` ; ailleurs *a posteriori*, `dir_other.go`) ; §16 `:3232` | oui |
| R6-m5 | `coherence.md` daté | `coherence.md:5` → révision 7, brouillon.8 | oui |

---

## 2. Les constats Codex du tour 6, rejoués sur le texte

**R6-B1 (bloquant : `resume = 0:0` ne prouve pas l'absence d'hibernation).** Contre-exemple de
Codex rejoué : swap absent ou zram sans repli, `resume = 0:0`, outil d'hibernation en espace
utilisateur écrivant l'image sur une destination en clair. Révision 7 (`:1351–1359`) : l'hibernation
n'est plus déduite de `resume` ; elle est acceptée seulement si (a) **impossible** (`/sys/power/state`
sans `disk`, ou `/sys/power/disk` = `[disabled]`) ou (b) **toute destination locale** d'une image est
chiffrée ; l'interface `/dev/snapshot` est nommée ; « tout autre cas → refus » ; témoin P-VAULT
« outil d'hibernation en espace utilisateur vers une destination en clair → refus » (`:1378–1379`).
Dans le contre-exemple, la destination en clair est un périphérique bloc persistant qui n'est pas
`CRYPT-` : (b) échoue, la cérémonie est refusée. J'ai vérifié la prémisse de Codex dans la source
qu'il cite : `userland-swsusp` dit que les utilitaires utilisent « *a swap partition … or a swap
file* », mais que ce n'est « *not really required, as they can use, for example, a special (blank)
suspend partition* » — la portée large de (b) est donc justifiée. Le contre-exemple ne tient plus.
**Fermé sur le fond.** Reste que la branche (b), telle qu'écrite, ne peut ni être lue ni passer sur
la cible courante (R7-M1) : c'est un défaut de passabilité, pas de sûreté — le refus est du bon côté.

**R6-M (majeur : profils d'énergie macOS).** OV-2 `:1364–1366` : `hibernatemode` = 0 sur **tous** les
profils (`pmset -g custom` : batterie, secteur, onduleur) **ou** volume de `hibernatefile` vérifié ;
profil illisible → refus. C'est ce que Codex demandait (« tous les profils applicables ou le volume
chiffré »). **Fermé.**

**R6-M1 (majeur : §9.11 contredit §9.5 sur Windows).** §9.11 `:1801–1804` dit désormais « sortie
bornée par le noyau sous Linux …, par `RLIMIT_FSIZE` sous macOS ; sous Windows, un objet de tâche ne
borne **pas** la taille des fichiers écrits : volume à quota, ou lancement borné refusé (§9.5) ». Grep
de « bornée par rlimit (Linux, macOS) ou objet » : aucune occurrence. Les douze autres mentions
d'« objet de tâche » (`:567`, `:1705–1706`, `:1793–1796`, `:1903`, `:1922`, `:3356`) portent sur les
processus, la mémoire, le CPU et la mort avec le parent, jamais sur la taille des fichiers. **Fermé.**

---

## 3. Régression : aucune

Greps sur les formulations retirées ou interdites : `st_dev` (hors `st_rdev`, voir R7-m1), `lsblk`,
`fdesetup` (ne reste que la négation d'OV-2, `:1363`), « vaut `0:0`, ou » (ancienne règle
d'hibernation), « pendant la cérémonie (`RLIMIT », « jamais au démarrage », « *a posteriori* de la
taille », « bornée par rlimit (Linux, macOS) ou objet » : **aucune** occurrence. Cohérence vérifiée :
SR-5 ↔ OV-5 ↔ P-VAULT (`:1096–1098`, `:1105–1106`, `:1118–1119`, `:1380–1388`) ; VK-1 ↔ §0.5 ↔ §9.2
pour l'indisponibilité de « Protéger ce projet » ; OV-4 ↔ OV-5 pour le dossier de données de la
webview ; §9.5 ↔ §9.11 ↔ §16 pour `MaxOutput` ; `AD-91` rév. 7 (`decisions.md:1336`) résume
fidèlement les cinq changements. Versions alignées sur brouillon.8 : `spec.md:5`, `decisions.md:5`,
`traceability.md:5`, `coherence.md:5`, `CHANGES.md:3`. Le témoin « démarrage chiffré, destination
externe en clair » et les témoins de montage changé et d'état inconnu sont conservés. Rien de ce que
j'avais noté au tour 6 comme « à ne pas toucher » (forme de SR-5, résolution par chemin, zram avec
`backing_dev`, EN-2, H-4) n'a bougé.

---

## 4. Constats nouveaux de la révision 7

### R7-M1 — Majeur (texte) : la branche (b) de l'oracle d'hibernation ne peut ni être lue ni passer sur une installation UEFI

**Preuve (mesurée sur cet hôte).** OV-1 (b) (`:1353–1357`) exige que « chaque périphérique bloc
persistant de `/sys/block/` … ne porte que des volumes `CRYPT-` ou aucun système de fichiers ni
swap ». Trois faits :
(1) **Impassable.** Toute installation UEFI porte une partition système EFI en **vfat en clair**
(obligation du firmware, elle ne peut pas être `CRYPT-`) et, chez Fedora et Ubuntu, un `/boot` ext4
en clair. Ici : `nvme0n1p1` vfat (`/boot/efi`), `nvme0n1p2` ext4 (`/boot`), `nvme0n1p3` btrfs — deux
systèmes de fichiers en clair sur le seul disque persistant. Dès que l'hibernation est **possible**
(pas de verrouillage du noyau, pas de `nohibernate`), (b) échoue sur **100 %** des cibles Linux à
chiffrement du disque : la voie « LUKS + swap chiffré + hibernation » que la révision 6 acceptait est
refusée sans issue autre que désactiver l'hibernation.
(2) **Illisible par la règle d'OV-1 elle-même** (« lecture de `/proc` et de sysfs, jamais de la
sortie d'un programme externe »). sysfs n'expose **aucune signature de système de fichiers** pour une
partition non montée (pas de `fstype` sous `/sys/class/block/<part>/`) ; le périphérique brut n'est
pas lisible par la personne (`dd if=/dev/nvme0n1p1` → *Permission non accordée*, mesuré) ; seule la
base udev (`/run/udev/data/b259:1` → `E:ID_FS_TYPE=vfat`, lisible) le dit, et c'est la sortie d'un
programme. « Aucun système de fichiers » est donc indécidable depuis l'application : par la règle
« oracle illisible → refus », (b) ne peut que refuser, même sur un disque entièrement chiffré.
(3) **`removable` ne vaut pas preuve.** « Aucun périphérique amovible branché » lit
`/sys/block/*/removable` ; un disque USB externe y vaut souvent `0` (fait connu du noyau, le drapeau
vient du périphérique). Si (b) survit, ce témoin doit lire le bus (`/sys/block/<dev>/device/..` sous
`usb`) ou être retiré.
Sur cet hôte, (a) passe (`/sys/power/state` = `freeze mem`, `/sys/power/disk` = `[disabled]`, noyau
verrouillé par Secure Boot) ; c'est le seul chemin qui passe aujourd'hui.
**Norme.** E-2/E-3 (une porte doit pouvoir **passer** sur les six cibles) ; OV-1 (lecture sysfs
seulement) ; ADR 0033 (échec fermé, pas échec permanent). Pas une faille : le refus est sûr ; c'est le
même défaut que R6-M1, déplacé d'un cran. Majeur et non bloquant : (a) passe sur les noyaux
verrouillés, le message de SR-5 (« désactivez … la mise en veille prolongée ») reste juste, aucun lot
ne s'en sert.
**Correction (recommandée).** Remplacer (b) par (b′), lisible et honnête : hibernation **possible**
acceptée si — chaque swap actif passe l'oracle de swap (déjà exigé) ; `/sys/power/resume` vaut `0:0`
ou désigne un `CRYPT-` (le noyau n'écrit que dans un swap actif, `swsusp` ; `resume` désigne lequel) ;
et aucun outil d'hibernation en espace utilisateur n'est configuré vers une destination en clair :
`/etc/uswsusp.conf` et `/etc/suspend.conf` absents, ou leur `resume device` (+ `resume offset`,
`uswsusp.conf(5)` : « *a swap partition or the partition that contains the swap file* ») résolu
`CRYPT-` par l'oracle de volume ; `/sys/power/tuxonice` absent. Résiduel à écrire au §9.2 (Voleur) :
un outil privilégié écrivant une image vers une partition vierge en clair n'est pas détectable depuis
l'application (il faut la racine, hors périmètre du modèle) ; P-VAULT garde le témoin « outil en
espace utilisateur vers une destination en clair → refus » (uswsusp configuré vers un swap en clair).
Variante plus simple si l'on préfère : hibernation possible → refus, sauf (a) ; honnête, mais elle
refuse la configuration « LUKS + swap chiffré » que la révision 6 acceptait.

### R7-m1 — Mineur : « `stat` de la source » doit lire `st_rdev`

OV-1 (`:1344–1345`) : « `stat` de la source → `/sys/dev/block/<maj>:<min>` ». Pour un nœud de
périphérique, `stat(2)` met le numéro du périphérique dans **`st_rdev`** ; `st_dev` est celui du
système de fichiers qui porte le nœud (ici `devtmpfs`, `st_dev = 7`, mesuré sur `/dev/mapper/*`).
Un mot ; mon R6-M1 avait la même imprécision. **Norme.** `stat(2)`.

### R7-m2 — Mineur : `FailureReportFolderPath` n'est pas un réglage

OV-5 (`:1386–1387`) : « `FailureReportFolderPath` de WebView2 **placé** sous le dossier de données de
la webview ». Microsoft Learn : `CoreWebView2Environment.FailureReportFolderPath` est une propriété
**en lecture seule** (« *Gets the failure report folder that all CoreWebView2s created from this
environment are using* », `{ get; }`) ; le dossier est sous le dossier de données **par construction**,
et c'est le dossier de données qui est placé par OV-4. Écrire : « se trouve (propriété en lecture
seule) sous le dossier de données de la webview, placé par OV-4 ; purgé à la fin de la cérémonie ».
**Norme.** E-2 (un oracle qui nomme ce qu'il lit, pas ce qu'il croit régler).

### R7-m3 — Mineur : `P-VAULT-MAC` n'existe pas

OV-2 (`:1362–1363`) renvoie à « P-VAULT-MAC » ; le nom vient de mon R6-m2, mais §0.5 ne définit que
**P-VAULT** (`:72`, `:1423`) et **P-SANDBOX-MAC** (`:1786`). Soit renvoyer à P-VAULT (ligne macOS de
« six cibles »), soit définir la porte. **Norme.** §0.3 (une source par chose), E-1.

### R7-m4 — Mineur : OV-5 peut dire que `systemd-coredump` honore la limite

OV-5 (`:1383–1385`) : un gestionnaire en tube « peut ignorer cette limite (`core(5)`) ». Exact pour le
noyau (`core(5)` : la limite est ignorée quand le vidage est envoyé à un tube) et pour Apport ; mais
`systemd-coredump(8)` dit que « *the core dump will only be processed when the related process
resource limits (`RLIMIT_CORE`) are sufficient* » — avec `%c = 0`, rien n'est écrit dans
`/var/lib/systemd/coredump/`. La vérification du volume de destination reste bonne (défense en
profondeur, Apport, gestionnaires inconnus) ; une demi-ligne évite qu'un lecteur croie `RLIMIT_CORE`
inutile sous systemd. **Norme.** N37 (exactitude).

---

## 5. Ce qui est bien et qu'il ne faut pas toucher

OV-1 « volume d'un chemin » par `mountinfo`, avec le parent d'une partition et **tous** les
périphériques d'un btrfs : c'est la résolution juste, et elle refuse correctement l'hôte de
développement d'aujourd'hui (btrfs en clair). OV-5 : la limite posée avant la webview est le seul
moment où elle est héritée ; le témoin « processus de rendu tué pendant la cérémonie » rend la porte
capable d'échouer. OV-2 sur tous les profils, OV-3 avec ses deux clés de registre. VK-1 et §0.5 qui
disent que « Protéger ce projet » peut être indisponible, mot de passe ou non. §9.5/§9.11/§16 enfin
d'accord sur `MaxOutput`.

---

## Verdict : **ACCEPT**

Mes 2 majeurs et 5 mineurs du tour 6 sont fermés dans le texte ; le bloquant Codex R6-B1 est fermé
sur le fond (hibernation définie par mécanisme, `/dev/snapshot` couvert, contre-exemple rejoué : il
ne tient plus), ses deux majeurs (profils d'énergie macOS, §9.11) le sont aussi ; aucune régression.
Un majeur **de texte** pour la révision 8, au fil de l'eau, sans nouveau tour bloquant : **R7-M1**
(la branche (b) de l'oracle d'hibernation exige une lecture que sysfs ne fournit pas et un état
qu'aucune installation UEFI ne présente ; la remplacer par (b′) swap actif + `resume` + configuration
uswsusp, résiduel écrit au §9.2). Mineurs R7-m1…m4 au fil de l'eau, dont deux imprécisions que
j'avais moi-même introduites (`stat`, `P-VAULT-MAC`). Acceptation conditionnée à rien d'autre.
