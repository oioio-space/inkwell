---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Versions SemVer par tags, paquets .deb/.rpm natifs par architecture, installés avant publication

## Contexte et problème

La finalité de la vague D est une application autonome, livrée en exécutable seul, `.deb` et
`.rpm` pour amd64 et arm64, versionnée en SemVer et testée telle qu'elle sera installée (plan
t11, « Objectif final — livraison », cas PKG-01 à PKG-12). Il faut trancher comment la version
naît (release-please ou tags), comment on construit du cgo (GTK4 + WebKitGTK 6.0) pour deux
architectures, ce qu'on signe, et comment on prouve que les paquets s'installent et se
désinstallent proprement.

## Facteurs de décision

- cgo : pas de compilation croisée (ADR 0003) ; chaque architecture se construit chez elle.
- Un seul responsable : la sortie d'une version doit rester un geste humain explicite.
- Une seule source de vérité par chose : la version vient du tag, le contenu des paquets d'un
  seul fichier, le CHANGELOG des commits.
- Rien ne se publie sans avoir été installé sur une machine propre de chaque famille.
- Pas de clé à garder pour signer les exécutables ; clé GPG facultative pour `.deb`/`.rpm`.

## Options envisagées

**Version.**
- release-please : ouvre une PR de version depuis les Conventional Commits, crée le tag et le
  CHANGELOG à la fusion.
- Tag posé à la main ; goreleaser construit sur le tag ; CHANGELOG généré par git-cliff.

**Construction.**
- goreleaser `--split` puis `continue --merge`.
- Une exécution `goreleaser release --skip=publish` par runner natif, puis un job d'assemblage.
- Compilation croisée (zig/xx) : exclue par l'ADR 0003.

**AppImage.** Dans cette vague, ou plus tard.

## Décision

**Version : tag + goreleaser + git-cliff, pas de release-please.**
- Le tag `vMAJEUR.MINEUR.CORRECTIF` ou `…-rc.N` (aussi `-alpha.N`, `-beta.N`) est la seule source
  de la version. goreleaser la lit (`{{ .Version }}`) et l'injecte dans l'exécutable
  (`-ldflags -X …/internal/version.Version/Commit/Date`), dans les noms et métadonnées des
  paquets, dans AppStream. `inkwell --version`, « À propos » (`Session.AppVersion`) et le nom du
  paquet disent donc la même chose (PKG-07, testé dans les conteneurs).
- `mise run changelog:next` propose la prochaine version depuis les commits (git-cliff `--bump`,
  `0.x` : une fonctionnalité monte le mineur) ; `mise run changelog -- --tag vX.Y.Z` régénère
  `CHANGELOG.md`, qu'on commite **avant** de poser le tag. Le workflow refuse un tag dont la
  section manque dans le CHANGELOG et dont le commit n'a pas de CI verte.
- Raisons contre release-please : (1) tout ce qu'il crée avec `GITHUB_TOKEN` (tag, PR) ne
  déclenche aucun autre workflow, il faudrait un PAT ou une GitHub App à garder
  ([README de release-please-action](https://github.com/googleapis/release-please-action#other-actions-on-release-please-prs)) ;
  (2) les préversions `-rc.N` et la politique `0.x` demandent une configuration par manifeste
  plus lourde que le gain pour un seul dépôt ; (3) la PR de version ajoute une étape
  d'approbation que le tag explicite fait déjà. Réexamen : si l'équipe grossit ou si les
  versions deviennent fréquentes.
- Préversions : `-rc.N` (publiées en **brouillon**, marquées préversion). Dans les paquets, la
  version prend la forme Debian/RPM `0.1.0~rc.1` (le tilde trie avant `0.1.0` : l'`rc` se met à
  jour vers la finale) ; les **noms de fichiers** gardent le `-` (`inkwell_0.1.0-rc.1_amd64.deb`)
  car GitHub renomme les `~` des assets. Une préversion installe l'icône « nightly » rayée
  (règle GNOME des préversions, [HIG](https://developer.gnome.org/hig/)), une version finale
  l'icône pleine.
- `0.x` tant que le format des projets peut changer ; `1.0.0` le jour où il est figé (ADR 0023).

**Construction : une exécution native par architecture, assemblage ensuite.**
- Le découpage `--split`/`continue --merge` est **réservé à GoReleaser Pro**
  ([Split & Merge](https://goreleaser.com/customization/partial/)). En OSS : le job `build` du
  workflow `release.yml` tourne en matrice sur `ubuntu-24.04` (amd64) et `ubuntu-24.04-arm`
  (arm64), chacun lance `goreleaser release --skip=publish` (deux entrées `builds`, dont une seule
  est active grâce à `skip: '{{ ne .Runtime.Goarch … }}'`) et produit tar.gz, `.deb`, `.rpm` et
  un SBOM syft par fichier. Un job `publish` rassemble les deux, calcule `checksums.txt`, signe
  et publie : goreleaser n'est pas utilisé pour publier (`release.disable: true`).
- Les runners arm64 hébergés par GitHub sont **gratuits uniquement pour les dépôts publics**
  ([annonce du 7 août 2025](https://github.blog/changelog/2025-08-07-arm64-hosted-runners-for-public-repositories-are-now-generally-available/)).
  Le dépôt va devenir public : `ubuntu-24.04-arm` est le défaut ; tant qu'il est privé, la
  variable de dépôt `ARM64_RUNNER` doit désigner un runner arm64 payant (« larger runner »).
- Les paquets sont décrits à un seul endroit, `.goreleaser.yaml` (nFPM intégré), avec leurs
  fichiers sources dans `build/linux/` : entrée de bureau
  `io.github.oioio_space.Inkwell.desktop`, AppStream (notes de version tirées du CHANGELOG par
  `scripts/pkg/stage.sh`), type MIME `text/x-typst`, page de manuel, icônes hicolor (PNG 16 à
  512, SVG, symbolique, jeu « nightly »). Les anciennes tâches Wails `package` (nfpm.yaml,
  AppImage, Docker) sont supprimées : une seule source.
- Installés dans `/usr/bin/inkwell` ; dépendances `libgtk-4-1`, `libwebkitgtk-6.0-4` (deb) et
  `gtk4`, `webkitgtk6.0` (rpm) ; licence MPL-2.0 ; mainteneur
  `oioio-space <64966845+oioio-space@users.noreply.github.com>` (adresse noreply GitHub du
  mainteneur : aucune adresse personnelle dans le dépôt).
- Hook sidecars : `scripts/pkg/prebuild.sh` lance `mise run sidecar:fetch` s'il existe. La tâche
  doit récupérer les binaires de l'architecture de la machine, vérifier leur SHA-256 (ADR 0008)
  et les poser où `go:embed` les prend.

**Signatures et traçabilité.**
- `checksums.txt` est signé par **cosign sans clé** (identité = le workflow `release.yml` sur le
  tag, jeton OIDC de GitHub, bundle `checksums.txt.sigstore.json`), conformément au
  [modèle de goreleaser](https://goreleaser.com/customization/sign/sign/) et à
  [cosign `sign-blob`](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/).
  Attention : en dépôt privé, la signature sans clé écrit le nom du dépôt dans le journal public
  Rekor ; à accepter ou à éviter en attendant que le dépôt soit public.
- `.deb` et `.rpm` sont signés en OpenPGP par nFPM **si** le secret `GPG_PRIVATE_KEY` (et
  `GPG_PASSPHRASE`) existe ; sinon le workflow le dit et construit sans signature. La clé publique
  est publiée avec la version (`inkwell-signing-key.asc`).
- Un SBOM SPDX (syft) accompagne chaque archive et chaque paquet.
- Taille : `sizes.txt` est publié avec chaque version ; la suivante avertit au-delà de +10 %
  (`scripts/pkg/sizes.sh`, PKG-12).

**Tests d'installation avant publication** (`scripts/pkgtest/`, `mise run pkg:test`, podman) :
chaque paquet est installé sur des conteneurs neufs (`debian:13`, `ubuntu:24.04`, `fedora:44`),
puis : dépendances résolues et aucune bibliothèque manquante ; `--version` = version du paquet ;
`desktop-file-validate` et `appstreamcli validate` ; icônes, type MIME et cache d'associations ;
démarrage **sans réseau** (conteneur `--network none`) sous Xvfb avec WebKitGTK réel, fenêtre
affichée, capture d'écran non vide ; désinstallation (tous les fichiers du paquet et les
dépendances devenues inutiles disparaissent, rien ne reste nommé « inkwell » hors du dossier
personnel, un projet de l'utilisateur est intact) ; mise à niveau depuis un paquet antérieur ;
signatures OpenPGP quand elles existent. Le job `test` de `release.yml` les rejoue sur chaque
architecture, sur son runner natif ; la publication en dépend. Un déclenchement manuel du
workflow est un essai à blanc : tout sauf la signature cosign et la publication.

**AppImage : reporté.** Un AppImage utile devrait embarquer WebKitGTK 6.0 (et ses processus
auxiliaires, bubblewrap, GStreamer) ; un AppImage qui s'appuie sur le WebKitGTK de l'hôte n'est
pas plus autonome que le `.tar.gz`. Un prototype dédié décidera ; l'exécutable seul est livré en
`.tar.gz`.

### Conséquences

- Bien : une version = un tag ; les deux architectures sont construites et testées chez elles ;
  rien n'est publié sans installation réussie ; pas de clé cosign à garder ; un seul fichier
  décrit les paquets.
- Moins bien : deux jobs de construction (pas de `--split`) ; l'arm64 impose un dépôt public ou
  un runner payant ; les tests de mise à niveau comparent à un paquet jetable (version
  `0.0.0-a.1`) tant qu'aucune version publiée n'existe ; AppStream n'a pas encore de captures
  d'écran (à ajouter avec les vraies, et une URL publique) ; le sandbox de WebKit est désactivé dans le
  test de fumée en conteneur (`WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS`, pas dans l'application).

### Confirmation

`mise run pkg:lint` (dans `mise run ci`) : entrée de bureau, AppStream généré, type MIME accepté
par `update-mime-database`, page de manuel, jeux d'icônes, `goreleaser check` et auto-tests des
scripts de version (notes, assemblage, tailles, vérification d'un bundle cosign). `mise run
pkg:test` et le job `test` de `release.yml` : cas PKG-01 à PKG-11 sur trois distributions.
`actionlint` et `yamlfmt` couvrent le workflow.

## Plus d'informations

Procédure et secrets : [docs/releasing.md](../releasing.md). Sources : [GoReleaser nFPM](https://goreleaser.com/customization/package/nfpm/),
[Split & Merge (Pro)](https://goreleaser.com/customization/partial/),
[cosign et goreleaser](https://goreleaser.com/customization/sign/sign/),
[runners arm64 gratuits pour les dépôts publics](https://github.blog/changelog/2025-08-07-arm64-hosted-runners-for-public-repositories-are-now-generally-available/),
[release-please-action](https://github.com/googleapis/release-please-action),
[git-cliff](https://git-cliff.org/docs/),
[AppStream](https://www.freedesktop.org/software/appstream/docs/),
[SemVer](https://semver.org/lang/fr/), [Conventional Commits](https://www.conventionalcommits.org/fr/).
Réexamen : dépôt public (runners arm64, Rekor), première version publiée (test de mise à niveau
depuis la vraie version précédente), arrivée des sidecars (taille, `sidecar:fetch`), prototype
AppImage.
