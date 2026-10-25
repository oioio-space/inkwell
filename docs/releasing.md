# Publier une version

Décision et raisons : [ADR 0025](adr/0025-versions-et-paquets.md). Cas de test : PKG-01 à PKG-12 du
[plan t11](plans/2026-10-25-t11-interface.md).

## Couper une version

```bash
mise run ci                                   # tout est vert, y compris pkg:lint
mise run pkg:build && mise run pkg:test       # facultatif : les paquets de cette machine, installés en conteneurs
mise run changelog:next                       # la version que les commits appellent, ex. v0.1.0
mise run changelog -- --tag v0.1.0-rc.1       # CHANGELOG.md tel qu'il sera ; relire, commiter
git tag -s v0.1.0-rc.1 && git push origin v0.1.0-rc.1
```

Le push du tag lance `.github/workflows/release.yml` : version (SemVer, section du CHANGELOG,
CI verte du commit) → construction native amd64 et arm64 → tests d'installation sur Debian 13,
Ubuntu 24.04 et Fedora 44 pour chaque architecture → assemblage, `checksums.txt`, signature cosign,
publication. Une version `-rc.N` est publiée en **brouillon** et marquée préversion : la relire sur
GitHub, puis la publier.

Essai à blanc : *Actions → release → Run workflow* construit et teste tout, n'envoie rien ; les
fichiers restent dans l'artefact `release-assets`.

## Réglages du dépôt

| Réglage | Rôle | Si absent |
|---|---|---|
| variable `ARM64_RUNNER` | label d'un runner arm64 (dépôt **privé** : « larger runner » payant) | `ubuntu-24.04-arm`, gratuit seulement si le dépôt est public |
| secret `GPG_PRIVATE_KEY` (armée) et `GPG_PASSPHRASE` | signature OpenPGP des `.deb` et `.rpm` ; la clé publique part avec la version | paquets non signés, avertissement dans le journal |
| permissions Actions : lecture/écriture | `gh release create` | la publication échoue |

Créer la clé : `gpg --quick-generate-key "Inkwell releases <adresse>" rsa4096 sign 2y`, puis
`gpg --armor --export-secret-keys <id>` dans le secret.

## Vérifier ce qu'on a téléchargé

```bash
mise run pkg:verify ./dossier v0.1.0-rc.1     # sommes de contrôle, signature cosign (identité = ce dépôt), SBOM
sudo apt install ./inkwell_0.1.0-rc.1_amd64.deb   # ou : sudo dnf install ./inkwell-0.1.0-rc.1.x86_64.rpm
inkwell --version
```

Signature OpenPGP : importer `inkwell-signing-key.asc` (`gpg --import`, et `rpm --import` pour un
`.rpm`), puis `rpm -K fichier.rpm`. Un `.deb` est signé à la manière de `dpkg-sig` (membre
`_gpgorigin` de l'archive `ar`) : la vérification est écrite dans `scripts/pkgtest/inside.sh`
(`verify_signature`).

## Construire et tester en local

| Commande | Effet |
|---|---|
| `mise run pkg:build` | exécutable, tar.gz, `.deb`, `.rpm` de **cette** architecture dans `dist/` (version `PKG_VERSION`, défaut `0.0.0-dev.<commit>`) |
| `mise run pkg:lint` | valide entrée de bureau, AppStream, MIME, man, icônes, goreleaser, scripts |
| `mise run pkg:test` | installation en conteneurs podman ; `PKG_DISTROS=fedora:44` limite ; `PKG_SIGN=1` signe avec une clé jetable et vérifie ; `PKG_KEEP=1` garde les images |
| arm64 sur une machine amd64 | `PKG_ARCH=arm64` : ignoré avec la raison tant que `qemu-user-static` n'est pas enregistré, et il faut des paquets arm64 (cgo : construits sur une machine arm64) |

## Ce qui n'est pas encore là

- Captures d'écran AppStream (il faut les vraies et une URL publique).
- Sidecars Typst : `scripts/pkg/prebuild.sh` lance `mise run sidecar:fetch` dès que la tâche existe.
- Test de mise à niveau depuis la **vraie** version précédente (aujourd'hui un paquet jetable `0.0.0-a.1`).
- AppImage (ADR 0025 : reporté).
