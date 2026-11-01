# Revue Fable n°8 de l'architecture (spec 1.0.0-brouillon.10, révision 8.1, 2026-11-01) — verdict : **ACCEPT** (0 bloquant, 1 majeur de texte, 7 mineurs)

Portée : `spec.md` (brouillon.10), `CHANGES.md` (révisions 8 et 8.1), `decisions.md` (`AD-91` rév. 8,
`AD-99` à `AD-104`), `coherence.md`, `docs/exigences.md` N42–N45. Lu : `14-codex-r7.md`, pour rejouer
R7-B1 sur (b′). Non lu : la revue Codex du tour 8. Code lu : `internal/sidecar/{limit,netns,dir,mount}_linux.go`,
`fetch/fetch.go`, `internal/typst/engine.go`, `build/windows/nsis/{wails_tools.nsh,project.nsi}`,
`internal/sidecar/manifest.json`, `docs/protocol/cddl/repo.cddl`, `docs/protocol/spec.md` (tableau des refus).
Référentiels : règles E-1 à E-5 du §0.5, INV-14, AC-18/AC-19, ADR 0009, ADR 0024, ADR 0033, N38 ;
sources externes vérifiées **aujourd'hui** : `crates/typst-cli/src/fonts.rs` et `crates/typst-kit/src/fonts.rs`
à l'étiquette `v0.15.1` ; `kernel/power/swap.c` (Linux v6.12) ; dépôt tinymist (`gh search code`) ;
README de `cmarker` ; `go list -m -versions github.com/landlock-lsm/go-landlock`. Mesures faites sur cet hôte
(Fedora 44, i7-1370P, 432 fichiers de polices) avec le **binaire typst 0.15.1 épinglé** du dépôt
(`internal/sidecar/bin/linux_amd64/typst.gz`, SHA-256 `29273eaa…` = manifeste), et sur le paquet
`webkitgtk6.0` installé.
Gravités : **bloquant** = garantie centrale contredite ou porte qui ne peut ni passer ni échouer ;
**majeur** = texte qui doit changer avant qu'un lot s'en serve ; **mineur** = précision.

---

## 1. Mes constats du tour 7 : tous fermés

| # | Constat | Où c'est fermé (`spec.md`) | Vérifié |
|---|---|---|---|
| R7-M1 | (b) illisible (aucune signature de partition non montée dans sysfs) et infranchissable sous UEFI | OV-1 **(b′)** `:1353–1360` : chaque swap actif passe l'oracle ; `/sys/power/resume` = `0:0` ou `CRYPT-` ; `/etc/uswsusp.conf`, `/etc/suspend.conf` absents ou `resume device` en `CRYPT-` ; `/sys/power/tuxonice` absent ; l'ancienne règle est dite **abandonnée** avec la raison ; défense en plus `:1361–1365` (verrou `Inhibit("sleep", …, "block")`, processus de rendu dédié terminé) ; témoins OV-4 `:1379–1381` | oui — c'est la (b′) que je proposais, durcie. Sur cet hôte, (a) passe toujours ; (b′) passerait aussi (swap unique `zram0` sans `backing_dev`, `resume` = `0:0`, aucun uswsusp) |
| R7-m1 | `st_dev` | OV-1 `:1346` : **`st_rdev`** « (le numéro du périphérique, pas `st_dev`) » | oui |
| R7-m2 | `FailureReportFolderPath` n'est pas réglable | OV-5 `:1389–1391` : propriété **en lecture seule**, sous le dossier de données de la webview **par construction** (placé par OV-4), purgée | oui |
| R7-m3 | « P-VAULT-MAC » indéfini | OV-2 `:1368` : « à confirmer par P-VAULT sur macOS » | oui |
| R7-m4 | `systemd-coredump` honore `RLIMIT_CORE` | OV-5 `:1385–1388` : dit, avec `systemd-coredump(8)` ; contrôle du volume gardé pour Apport et les gestionnaires inconnus | oui |

**D-17 — le résiduel est-il honnête ?** Oui. Ce que (b′) couvre est prouvé mécanisme par mécanisme ; ce
qu'elle ne couvre pas est nommé deux fois, au même mot (§6.10 `:1366–1371` et §9.9 `:1960–1964`) :
un outil installé et configuré **avec les droits d'administration**, hors des mécanismes connus, qui
contourne `logind` et écrit une image brute vers une partition en clair **pendant** la cérémonie. J'ai
vérifié les deux prémisses : (1) `/dev/snapshot` et `/sys/power/state` exigent la racine, donc le
résiduel est bien un acteur administrateur, hors du modèle du Voleur (§9.2) ; (2) l'image du noyau ne
contient pas les pages **libres** (le processus de rendu dédié, terminé, rend ses pages anonymes au
tampon libre), donc « terminé à la fin » réduit réellement l'exposition à la durée de la cérémonie.
Deux précisions de formulation seulement (R8-m1). Rien n'est présenté comme garanti qui ne le soit.

---

## 2. Codex R7-B1, rejoué sur (b′)

Codex : la branche (b) admettait « aucun système de fichiers ni swap », donc une partition vierge
d'hibernation en clair (`userland-swsusp`). Sur le texte nouveau, la partition vierge n'est plus une
destination admise par aucune des trois voies : `swsusp` n'écrit que dans un swap actif (voir R8-m1
pour le détail du noyau) et tous les swaps actifs doivent passer l'oracle ; uswsusp n'est accepté que
si son `resume device` résout en `CRYPT-` ; TuxOnIce doit être absent. L'écriture d'une image brute
vers une partition vierge par un outil **tiers** reste possible — et c'est exactement le résiduel
écrit, tranché par l'utilisatrice (D-17). Le contre-exemple de Codex est donc soit fermé (outils
connus), soit explicitement accepté et documenté (outil arbitraire à droits d'administration).
**Fermé sur le texte**, sans surclamer.

---

## 3. Le contenu nouveau

### 3.1 N42 — autoporteur

**Liste de remplacement de `CHANGES.md` contre le code.** Chaque entrée Linux existe où elle est
dite : `limit_linux.go:9,29,32` (`/bin/sh`, `ulimit -v`, `ulimit -f` en blocs de 512 octets — d'où
« octets exacts, sans arrondi de shell » au §9.5, juste) ; `netns_linux.go:28`
(`exec.Command("/bin/sh", "-c", "exit 0")` comme sonde) ; `dir_linux.go:16–17,40,74`
(`/usr/bin/setpriv --no-new-privs --landlock-access … /bin/true`, `/usr/bin/prlimit --fsize=`) ;
`mount_linux.go:17,31,101–102,121–127` (`/usr/bin/unshare --user --map-root-user --mount /bin/sh -c
'… /usr/bin/mount -t tmpfs …'`, puis `setpriv`, `prlimit`) ; `fetch/fetch.go:271` (`tar -xOf`, importé
par le seul `cmd/sidecar-fetch` : construction, conforme à AC-6). Aucun autre `exec` en production
(grep de `exec.Command`, des littéraux `/bin/`, `/usr/bin/`, `xdg-open`, `fc-list`, `hdiutil`,
`pmset`, `diskutil` : rien d'autre) ; `run.go:136` lance le chemin résolu par `Store.Resolve`, c'est
le sidecar vérifié (AC-19). Les entrées macOS et Windows de la table (`hdiutil`, `diskutil`,
`fdesetup`, `pmset`, `sysctl`, PowerShell, `manage-bde`) étaient des mots de la spécification, pas du
code : la table le dit (« spec … »). **Juste.** Les remplacements tiennent : `SysProcAttr.Cloneflags`
avec `CLONE_NEWUSER` et `UidMappings` est pris en charge par `os/exec` sans `newuidmap` pour une
correspondance soi → 0 ; `unix.Mount("tmpfs", …)` est permis dans l'espace utilisateur nouveau ;
go-landlock existe en **v0.11.0** (dernière version publiée, vérifié par le proxy Go) ; `filippo.io/age`
est une bibliothèque Go.

**§14.1, table des dépendances garanties.** Lue ligne à ligne : plancher glibc = Ubuntu 24.04 (2.39),
juste ; `DT_NEEDED`/`LC_LOAD_DYLIB` par `debug/elf`/`debug/macho`, faisables ; Go ≥ 1.24 fournit
`elf.File.DynamicVersionNeeds` pour la plus haute `GLIBC_*` importée. WebView2 : l'amorce de Wails est
bien **en ligne** (`wails_tools.nsh:170–200` : `MicrosoftEdgeWebview2Setup.exe`, commentaire
« online-only deployment ») et le §16 `:3662` la liste ; le remplacement par le programme
d'installation **autonome** Evergreen est juste, à une précision d'architecture près (R8-m4).
Contrôle au démarrage DG-1 et lanceur statique DG-2 : cohérents avec `ld.so(8)` (un `DT_NEEDED`
manquant tue le processus avant `main`, donc seul un lanceur statique peut parler) ; une contradiction
interne sur les imports du lanceur (R8-m3). **Un manque** : les processus auxiliaires que WebKitGTK
lance lui-même ne sont ni dans la table ni dans l'exception d'INV-14 (R8-M1).

**Protection facultative (EN-0).** `:1291–1294` : jamais cochée par défaut, même partagé, aucune
incitation ; `AD-91` rév. 8 (`decisions.md:160` et le paragraphe « Révision 8 ») ; `coherence.md:163`
élément 9 du design. Cohérent avec VK-1 (« indisponible » quand SR-5 échoue, sans mot de passe).

### 3.2 N43 — polices

**Ordre de chargement de typst 0.15.1 — vérifié à la source.** `crates/typst-cli/src/fonts.rs`
(`discover_fonts`, étiquette `v0.15.1`) : `fonts::system()` si `!ignore_system_fonts`, puis
`fonts::embedded()` si `!ignore_embedded_fonts`, puis `fonts::scan(path)` pour chaque `--font-path`,
dans cet ordre ; `typst-kit/src/fonts.rs:151` : `db.load_system_fonts()` (fontdb, aucun programme
externe). L'« état vérifié » du §6.12 est **exact**, et la conséquence aussi : sans les deux drapeaux
`--ignore-*`, le système et `typst-assets` passeraient avant le projet. FT-2 (une liste `--font-path`
ordonnée, les deux drapeaux, la même liste pour tinymist) est le seul moyen d'obtenir
projet → inkwell → mes polices → ordinateur avec ce binaire ; les quatre drapeaux et `--variants`
existent (`typst compile --help`, `typst fonts --help`, mesuré). Une petite parité manque (R8-m6).

**« Polices utilisées » par tinymist.** `tinymist.getDocumentMetrics` est bien une commande du serveur
(`crates/tinymist/src/server.rs`, utilisée par `editors/vscode/src/tools/summary.ts`) : accessible sur
stdio, sans aperçu web. FT-8 a son repli nommé (compilation de contrôle) si P-FONT l'invalide. Juste.

**Rassemblement (§6.13).** Mesuré ici avec le binaire épinglé : `#image("../x.png")` hors de `--root`
donne `error: path "../x.png" would escape the project root` avec position ; une famille inconnue
donne `warning: unknown font family: …` avec position ; `--deps - --deps-format json` existe et rend
`inputs`/`outputs`. Les fichiers hors racine **non cochés** avec le chemin complet affiché et la borne
« fichiers ordinaires d'un type connu » : la bonne décision (un document reçu d'un pair est hostile,
§9.2) et conforme à `AD-101`. RA-T1 (même SHA-256 du PDF sur image vierge, `--creation-timestamp`,
`SOURCE_DATE_EPOCH`) est une preuve d'autosuffisance, pas une déclaration.

**Budget PF-11 (≈ 80 ms).** Mesuré ici, 10 exécutions triées, cache chaud : par défaut
77–84 ms ; `--ignore-system-fonts` 7–10 ms ; les deux `--ignore-*` sans `--font-path` 4–6 ms ; les deux
`--ignore-*` avec `--font-path /usr/share/fonts` (320 fichiers) 44–50 ms. La découverte du système
coûte donc **≈ 72 ms pour 432 fichiers** : le « ≈ 80 ms » de la spécification est confirmé. Mais la
conséquence sur le budget n'est pas tirée jusqu'au bout (R8-m2).

### 3.3 N44 — synchronisation texte ↔ aperçu

**État vérifié.** `jump_from_cursor` n'est appelé dans tinymist que par
`crates/tinymist/src/tool/preview/compile.rs` (aperçu web) et `crates/tinymist-query/src/hover.rs`
(lien de survol vers l'aperçu) : aucune requête LSP ne rend les positions au client. L'écart est donc
réel et la voie (iv) (aperçu web sur `127.0.0.1`) est à bon droit écartée (ADR 0009, espace réseau
privé du §9.5). **P-SYNC** est une porte franche : (i) requête amont publiée **ou** (ii) sidecar
`inkwell-typst` construit et mesuré, avec (iii) comme repli nommé et précis « au paragraphe » — ni
l'une ni l'autre n'est promise sans preuve (E-3). SY-3 (table une fois par compilation, interpolation
O(log n), aucun appel à Go par image) et SY-5 (le volet touché mène 300 ms, les défilements programmés
portent un jeton et n'émettent jamais) sont le modèle classique anti-ping-pong et SY-8 le teste par
modèle. SY-2 : `cmarker` a bien `heading-labels` (`"github"`, `"jupyter"`) et `label-prefix` (README,
§ options). Une observation sur (ii) : elle supprime aussi le coût de FT-6 (dit à SY-1) — c'est
l'argument qui devrait peser dans P-SYNC et P-FONT ensemble (R8-m2).

### 3.4 N45 — vue stable et mode « suivre »

**RV-1.** Annotation `remote`, aucun `scrollIntoView`, positions appliquées à travers les changements,
ancrage par curseur Automerge du premier caractère visible : juste, et `EditorView.scrollIntoView(pos,
{y: "start", yMargin})` existe. Note sans gravité : `@codemirror/view` fait déjà un ancrage de
défilement interne (contenu au-dessus de la fenêtre qui change → position tenue) ; RV-1 le renforce
d'un ancrage sémantique (curseur Automerge), ce qui est mieux sous suppression ; le dire évite qu'une
implémentation se batte contre le sien.

**RV-3 à RV-7.** La présence existante suffit (`repo.cddl:63–86` : `anchor`/`head` en curseurs
Automerge, `sessionId`, émetteur authentifié) ; « fichier actif = document du dernier `update` » tient
parce que Go garde la dernière par (pair, `sessionId`) ; le nom vient de la liste signée, jamais du
`name` déclaré ; la présence hostile est à perte, bornée, et la moindre entrée locale rend la main.
RV-5 reste sous la borne dure du protocole (64 s⁻¹). RV-7 recommande « Ne pas partager ma position » :
cohérent avec RV-4 (l'œil est une courtoisie, pas une garantie — dit honnêtement).

**PD-4.** `follow` en `update` seul, jamais en `snapshot` : la raison est juste
(`snapshot-state` est **fermée**, `repo.cddl:82`, un appareil 2.0 jetterait tout le `snapshot`).
Valeur `tstr .size (0..64)` : un PeerID Ed25519 en base58 tient (52 caractères). Proposé, pas écrit
dans `docs/protocol/` — conforme à la boucle du protocole. Une précision sur la règle 2.0 citée (R8-m5).

---

## 4. Régression : aucune

Greps sur les formulations retirées : « tout périphérique bloc chiffré » ne subsiste que comme règle
**abandonnée** nommée (OV-1) ; `hdiutil`, `diskutil`, `fdesetup`, `pmset`, `manage-bde`, PowerShell,
`lsblk`, `fc-list` n'apparaissent plus que dans des négations (INV-14, OV-2, OV-3, FT-4) ou dans
`AD-91` comme historique. Cohérences vérifiées : INV-14 ↔ AC-18/AC-19 ↔ §9.5 ↔ §14.1 ↔ §16
(`:3660–3663`) ; EN-4 macOS (« aucun support mémoire ») ↔ OV-2 ↔ `AD-99` (`hdiutil`) ; FT-1 ↔ FT-2 ↔
§9.5 (dossiers de polices en lecture seule) ↔ §6.13 RA-2 (sources (1) et (2) seules) ↔ §14.1
(polices garanties) ; SY-5 ↔ RV-2 ↔ RV-3 (« la dernière action de la personne gagne », un seul
principe) ; PF-11/12/13 ↔ FT-6/SY-7/RV-5 ; `AD-91` rév. 8 résume fidèlement les sept changements.
Versions alignées sur brouillon.10 : `spec.md:5`, `coherence.md:5`, `CHANGES.md:3`. SR-5, OV-4,
OV-5, VK-1, EN-2, H-4, `MaxOutput` (§9.5 ↔ §9.11 ↔ §16) : intouchés.

---

## 5. Constats nouveaux

### R8-M1 — Majeur (texte) : INV-14 et l'e2e d'AC-19 ignorent les processus auxiliaires de WebKitGTK

**Preuve (mesurée sur cet hôte).** Le paquet garanti `webkitgtk6.0` **exige** `bubblewrap` et
`xdg-dbus-proxy` (`rpm -q --requires`) ; `libwebkitgtk-6.0.so.4` contient les chemins **fixes**
`/usr/bin/bwrap` et `/usr/bin/xdg-dbus-proxy` ; `/usr/libexec/webkitgtk-6.0/` porte
`WebKitWebProcess`, `WebKitNetworkProcess`, `WebKitGPUProcess`, que la bibliothèque lance à **chaque**
création de webview. Sur Linux, inkwell lance donc à l'exécution au moins trois programmes qui ne sont
ni « nos binaires » ni le lanceur DG-2 — et, sandbox WebKit activée, `bwrap` et `xdg-dbus-proxy`.
**Norme.** INV-14 (`:151`) : « aucun programme externe n'est lancé ; seules exceptions : nos
binaires …, le lanceur Linux …, la commande d'agent ACP » — littéralement faux à chaque démarrage
Linux. AC-19 (`:678`) : e2e « dans un conteneur sans shell ni util-linux (`/bin/sh`, `/usr/bin/*`
absents) : aperçu, export, greffon, sauvegarde fonctionnent » — avec `/usr/bin/*` absent, WebKitGTK ne
peut pas lancer `bwrap` : la porte ne peut pas passer (E-2/E-3) ; même défaut de passabilité que
R7-M1. §14.1 ne nomme ni `bubblewrap` ni `xdg-dbus-proxy` ; DG-4 (`:2389–2391`) ne mentionne les
processus auxiliaires que pour écarter AppImage. Pas une faille : ce sont des composants du paquet
garanti. Majeur parce que l'invariant est l'armature de N42 et que la porte, telle qu'écrite, ne
passe pas.
**Correction (recommandée).** (1) INV-14 : ajouter l'exception « les processus auxiliaires des
bibliothèques garanties du §14.1 (WebKitGTK : `WebKitWebProcess`, `WebKitNetworkProcess`,
`WebKitGPUProcess`, `bwrap`, `xdg-dbus-proxy` ; WebView2 : ses processus de navigateur et de rendu) »,
lancés par la bibliothèque, jamais par notre code (AC-18 reste intact). (2) §14.1 : une ligne
`bubblewrap`, `xdg-dbus-proxy` (Linux ; dépendances obligatoires du paquet WebKitGTK ; `tar.gz` : le
lanceur DG-2 les cherche aussi). (3) AC-19 : remplacer « `/usr/bin/*` absents » par la liste exacte des
programmes que le code actuel lançait (`/bin/sh`, `/usr/bin/{sh,setpriv,prlimit,unshare,mount,tar,git,
fc-list,fc-match,xdg-open}`) absents, le reste du système présent. (4) Dire si la sandbox WebKit est
activée (Wails) : si oui, `bwrap` devient une dépendance de l'aperçu lui-même et DG-2 doit le vérifier.

### R8-m1 — Mineur : (b′) est juste, sa justification du noyau est incomplète ; le résiduel manque un cas

`kernel/power/swap.c` (v6.12 `:353–361`, `swsusp_swap_check`) : si `/sys/power/resume` est **nul**, le
noyau choisit le **premier swap actif** (`find_first_swap`) et y écrit l'image ; s'il désigne un
périphérique qui n'est pas un swap actif, `swap_type_of` échoue et l'hibernation **échoue** (sûr). Le
texte (`:1354–1355` : « n'écrit l'image que dans un swap actif, désigné par `/sys/power/resume` »)
laisse croire que `0:0` signifie « nulle part » ; la règle, elle, est juste parce qu'elle exige que
**chaque** swap actif passe. Écrire la phrase du noyau et citer `swap.c`. Résiduel (§9.9 `:1960`) :
ajouter « ou un mécanisme connu configuré hors de ses fichiers standard (`s2disk -f <autre.conf>`,
`/sys/power/resume` réécrit par la racine après la vérification) » — même classe d'acteur, même
conclusion, mais « hors des mécanismes connus » ne le couvre pas à la lettre.

### R8-m2 — Mineur : PF-11 fixe un budget que FT-2 ne peut pas tenir ; le « repli » de FT-6 est la voie normale

Mesuré ici : ≈ 0,14 ms par fichier de police par `--font-path` (44–50 ms pour 320 fichiers, cache
chaud) ; 1 000 fichiers ≈ 140 ms, soit ~3× le budget « ≤ 50 ms p95 » (`:2285`, FT-6 `:1619`).
Avec FT-2, les dossiers du système sont des `--font-path` : le coût est le même qu'aujourd'hui. Donc
sur toute machine ordinaire, la **vue réduite** de FT-6 est la voie principale, pas un repli. Le texte
le laisse deviner (« d'où la vue réduite ») sans le dire. Norme : N38 et E-3 (un budget doit pouvoir
passer par le mécanisme nominal). Correction : faire de la vue réduite la règle (dossier de session
par familles utilisées ou nommées, recalculé à l'apparition d'une famille) et de la liste complète
l'exception (gestionnaire, index FT-5 en arrière-plan) ; ou lier PF-11 à la voie (ii) de SY-1
(sidecar résident : découverte payée une fois), qui est déjà nommée pour cette raison — alors P-FONT et
P-SYNC se décident ensemble.

### R8-m3 — Mineur : le lanceur DG-2 contredit ses propres imports

§4.2 `:570–573` : `cmd/inkwell-launch` « n'importe que la bibliothèque standard et `golang.org/x/sys` » ;
DG-2 `:2377–2383` : message « par une notification (`org.freedesktop.Notifications`, D-Bus) quand un
bus existe ». Un client D-Bus ne vient ni de la bibliothèque standard ni de `x/sys`. Choisir : sortie
d'erreur + `$XDG_STATE_HOME/inkwell/demarrage.txt` seulement (le bureau ne lit pas la sortie d'erreur
d'un lanceur `.desktop`, mais le fichier reste), ou admettre `godbus` (pur Go) dans les imports du
lanceur et le dire au §4.2 et à PF-11 (≤ 5 ms).

### R8-m4 — Mineur : le programme d'installation autonome de WebView2 est par architecture

Le « Evergreen Standalone Installer » existe en x64, x86 et **ARM64**, chacun de l'ordre de
150–200 Mio ; DG-3 promet Windows amd64 **et** arm64. §14.1 (`:2358`) doit dire « un installateur par
architecture, chacun embarquant l'autonome de son architecture », et le contrôle DG-1 par
`GetAvailableCoreWebView2BrowserVersionString` reste commun. Sinon l'image « Windows arm64 sans
WebView2 » de P-DEPS ne peut pas passer.

### R8-m5 — Mineur : PD-4 cite une règle 2.0 qui ne vise pas la map `update`

PD-4 `:3511–3514` : un appareil 2.0 « jette ce seul message » au titre de « clé ou valeur d'une map
CBOR **fermée** (présence) → message jeté » (`docs/protocol/spec.md:71`). Or `update` est une map
**ouverte** (`repo.cddl:80` : `* tstr => any`) ; ce qui échoue est la **valeur** de `channel` hors de
`channel-pair` (`:83–84`). Le comportement est vraisemblablement le même (validité CDDL en échec →
classe à perte), mais il faut nommer la règle exacte et le pinner par un vecteur **côté 2.0**
(`#presence-update-unknown-channel` → jeté seul, sans sanction) ; `docs/protocol/vectors/` ne contient
aujourd'hui qu'un `README.md`, les quatre vecteurs de PD-4 sont donc tous à écrire.

### R8-m6 — Mineur : FT-4 ne reproduit pas tout ce que typst voit par défaut sur macOS

`typst-kit/src/fonts.rs:204` (`load_adobe_fonts`) ajoute le dossier des polices Adobe (CoreSync) à la
liste du système ; FT-4 (`:1596–1600`) liste `/System/Library/Fonts`, `/Library/Fonts`,
`~/Library/Fonts` et les URL de CoreText. Une personne ayant une police Adobe la verrait avec `typst`
nu et pas avec inkwell. Sans gravité (déterminisme préservé, FT-9 avertit) ; soit ajouter le dossier,
soit écrire la différence dans FT-4.

### R8-m7 — Mineur : « processus de rendu dédié » n'est pas acquis sur WebView2 même avec une fenêtre à part

WebView2 attribue les processus de rendu **par site** : deux fenêtres de la même application et de la
même origine (`wails://…`) partagent en général un même processus de rendu. Pour que la cérémonie ait
son processus (OV-1 `:1362–1364`, « à prouver par P-VAULT : processus dédié sur chaque moteur »), la
fenêtre de cérémonie doit venir d'un **`CoreWebView2Environment` distinct** (dossier de données
distinct, placé par OV-4 et purgé) — un processus de navigateur et de rendu à elle. WebKitGTK 6.0 :
une `WebKitWebView` créée sans `related-view` obtient son propre processus web ; vérifier que Wails
v3 ne lie pas les fenêtres entre elles. Écrire ces deux conditions à côté du « à prouver ».

---

## Verdict

**ACCEPT.** Aucun bloquant. R7-M1 et les quatre mineurs sont fermés ; la décision D-17 est appliquée
avec un résiduel honnête et vérifiable (R8-m1 n'en change pas le sens). N42 est fondé sur le code tel
qu'il est, avec des remplacements qui existent (un manque de texte sur les processus de WebKitGTK,
R8-M1, à corriger avant que le lot « sans shell » ne se serve de la porte). N43 repose sur un ordre de
chargement **vérifié à la source** et sur des mesures que je reproduis ; N44 et N45 ont des portes
franches, un modèle unique (« la dernière action de la personne gagne ») et un delta de protocole
proposé dans les règles. À traiter dans la révision 9 : R8-M1 ; les mineurs au fil de l'eau.
