# Source canonique des icônes (exigence N56)

## 1. Source retenue et licence

La source canonique est le **GNOME Icon Development Kit** (`Teams/Design/icon-development-kit`).
L'application *Icon Library* (`World/design/icon-library`) extrait directement ses icônes de ce dépôt (`update-icons.py`).

- **Licence des illustrations :** Creative Commons Zero 1.0 Universal (CC0 1.0).
- **URL brute de la licence :** https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/COPYING.md
- **Extrait de `COPYING.md` :**
  > « Affirmer hereby overtly, fully, permanently, irrevocably and unconditionally waives, abandons, and surrenders all of Affirmer's Copyright and Related Rights [...] »
- **Compatibilité MIT :** intégration directe des tracés SVG autorisée sans restriction ni contamination copyleft.
- **Alternative écartée :** `GNOME/adwaita-icon-theme` est sous double licence LGPL-3.0 / CC-BY-SA-3.0 (https://gitlab.gnome.org/GNOME/adwaita-icon-theme/-/raw/master/COPYING : « GNU LGPL v3 or Creative Commons Attribution-Share Alike 3.0 »). La clause ShareAlike et les contraintes LGPL empêchent une incorporation permissive sous MIT.

## 2. Attribution recommandée (À propos / notices tierces)

La licence CC0 n'impose aucune attribution légale. Par courtoisie envers le projet GNOME, la mention suivante est retenue :

```text
Icônes : GNOME Icon Development Kit (Jakub Steiner et contributeurs GNOME), sous licence CC0 1.0 Universal.
https://gitlab.gnome.org/Teams/Design/icon-development-kit
```

## 3. Table de correspondance vérifiée

Les noms proposés dans l'audit provenaient d'Adwaita (avec suffixe `-symbolic`) ou étaient fictifs (`eye-open-negative-filled`). Le tableau ci-dessous indique les noms réels et vérifiés dans le dev-kit amont (alignés sur les mots-clés internes GNOME).

| Icône / Glyphe actuel | Nom proposé dans l'audit | Nom amont vérifié | URL brute SVG amont |
|---|---|---|---|
| `branch` (historique) | `preferences-system-time-symbolic` | `clock` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/clock.svg |
| `side` (barre latérale) | `sidebar-show-symbolic` | `sidebar-left` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/sidebar-left.svg |
| `eye` (aperçu) | `eye-open-negative-filled-symbolic` | `eye` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/eye.svg |
| `present` (diaporama) | `tv-symbolic` | `tv` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/tv.svg |
| `menu` (menu principal) | `open-menu-symbolic` | `open-menu` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/open-menu.svg |
| `close` / `×` | `window-close-symbolic` | `x` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/x.svg |
| `check` / `✓` | `object-select-symbolic` | `object-select` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/object-select.svg |
| `right` / `›` | `pan-end-symbolic` | `arrow-big-right` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/arrow-big-right.svg |
| `down` / `caretdown` | `pan-down-symbolic` | `arrow-big-down` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/arrow-big-down.svg |
| `plus` / `+` | `list-add-symbolic` | `plus` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/plus.svg |
| `minus` / `−` | `list-remove-symbolic` | `minus` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/minus.svg |
| `more` | `view-more-symbolic` | `view-more` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/view-more.svg |
| Triangle CSS | `go-next-symbolic` | `media-playback-start` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/media-playback-start.svg |
| `●` (radio/sync) | `radio-checked-symbolic` | `radio` | https://gitlab.gnome.org/Teams/Design/icon-development-kit/-/raw/main/icons/radio.svg |
