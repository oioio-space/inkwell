---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Routage des modèles : économie de tokens sans perte de qualité

## Contexte et problème

Règle du projet : économiser les tokens sans perte de qualité, en confiant les tâches
adaptées à des modèles open-weight **hébergés** (pas de modèle local : la machine n'a pas la
capacité). Il faut savoir quelles tâches peuvent sortir de Claude sans baisse de qualité.

## Facteurs de décision

- Qualité au moins égale au tier Claude remplacé, prouvée par une source indépendante
- Rétention zéro des données (ZDR) imposée à chaque requête
- Pas de proxy devant la session Claude Code (authentification par abonnement, chaîne
  d'approvisionnement)

## Options envisagées

- claude-code-router / LiteLLM devant Claude Code (`ANTHROPIC_BASE_URL`)
- **Délégation explicite** par script (`scripts/llm.sh`, OpenRouter, ZDR)
- Tout garder sur Claude

## Décision

Délégation explicite via `scripts/llm.sh`, uniquement pour du texte mécanique :

| Usage | Modèle | Preuve (25/10/2026) |
|---|---|---|
| Brouillon de commit, tri de logs, résumé de long contexte | `deepseek/deepseek-v4.1-flash-1017` | Index Artificial Analysis 45 vs Haiku 5.5 : 43 ; 1M de contexte ; ZDR |
| Traduction i18n, textes FR (pilote) | `mistralai/mistral-large-4-0` | Meilleur a priori pour le français, parité non prouvée → relu par Claude |
| Code, revue, sécurité, conception | **Claude** (Haiku / Sonnet / Opus) | GLM-5.3, meilleur open-weight pour coder : AA 45 vs Sonnet 5.5 : 56 |

Paramètres imposés : `provider: {zdr: true, quantizations: ["fp8","bf16"],
require_parameters: true}` ; pas de variantes `:free` (non ZDR) ni `:batch`.
Sans `OPENROUTER_API_KEY`, le script sort en code 3 et l'appelant revient à Haiku.

### Conséquences

- Bien : aucun changement d'infrastructure de la session ; qualité inchangée là où elle compte.
- Moins bien : l'économie porte sur le texte mécanique, pas sur le code — c'est le prix de
  la règle « sans perte de qualité ».

### Confirmation

Réexaminer à chaque trimestre ou quand un modèle open-weight atteint l'index de Sonnet sur
une source indépendante ; A/B à l'aveugle sur 50 chaînes d'interface avant de généraliser
Mistral.

## Plus d'informations

[Artificial Analysis — leaderboard](https://artificialanalysis.ai/leaderboards/models) ·
[OpenRouter ZDR](https://openrouter.ai/docs/features/zdr) ·
[Claude Code LLM gateway](https://code.claude.com/docs/en/llm-gateway.md) ·
[GLM-5.3 model card](https://huggingface.co/zai-org/glm-5.3).
