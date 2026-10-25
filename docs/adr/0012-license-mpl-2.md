---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Licence : MPL-2.0

## Contexte et problème

Question ouverte du document ; elle conditionne la publication. Toutes les dépendances
retenues sont MIT, Apache-2.0 ou BSD.

## Options envisagées

- MIT / Apache-2.0 (permissives)
- **MPL-2.0** (copyleft au niveau du fichier)
- AGPL-3.0 (copyleft fort, y compris réseau)

## Décision

MPL-2.0 : les modifications des fichiers d'inkwell restent ouvertes, sans contaminer le
code qui l'intègre ni rebuter les équipes IT visées (l'AGPL est souvent interdite en
entreprise). L'application n'a pas de serveur, l'argument principal de l'AGPL tombe.
L'auteur unique garde la possibilité de double licence.

### Conséquences

- Bien : compatible avec toutes les dépendances ; adoption en entreprise possible.
- Moins bien : un fork peut garder ses **nouveaux** fichiers fermés.

### Confirmation

`LICENSE` à la racine ; en-tête SPDX `MPL-2.0` vérifiable par les outils de SBOM.
