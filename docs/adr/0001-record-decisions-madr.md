---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Consigner les décisions d'architecture en MADR

## Contexte et problème

Le projet est développé avec des agents IA sur plusieurs sessions. Sans trace écrite des
raisons, un agent (ou un humain) défait une décision qu'il ne comprend pas.

## Options envisagées

- Format Nygard (Titre, Statut, Contexte, Décision, Conséquences)
- MADR 4 (ajoute facteurs, options, confirmation ; en-tête YAML)
- Rien de formel : tout dans le document de conception

## Décision

MADR 4, en français, dans `docs/adr/`, parce que la section « Options envisagées » répond
d'avance à la question qui revient toujours (« pourquoi pas X ? ») et que l'en-tête YAML
est lisible par les outils.

### Conséquences

- Bien : `CLAUDE.md` reste court et pointe ici ; chaque ADR est citable.
- Moins bien : une discipline de plus ; atténuée en n'écrivant d'ADR que pour les
  décisions coûteuses à défaire.

### Confirmation

Toute PR qui change une frontière de [`architecture.md`](../architecture.md) référence une ADR.
