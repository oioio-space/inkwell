# Objectif en cours

Registre lu par les outils (`scripts/gate.py`, `scripts/block_done.py`) : noms de blocs, lots et reports de critères.
Plus les décisions encore en vigueur. L'état d'avancement, l'ordre et le reste à faire vivent dans `PROGRESS.md`.
Récit des lots terminés et journal de pilotage : [`docs/archive/2026-11-objectif-journal.md`](archive/2026-11-objectif-journal.md).

## Registre des blocs

Une ligne par bloc (porte `gates/<bloc>.toml` quand elle existe) ; l'état et l'ordre vivent dans `PROGRESS.md`,
le récit de chaque bloc dans l'archive.

| Bloc | Rôle |
|---|---|
| L0, L1, L2, L3 | lots MVP : références et portes de performance, bac à sable, éditeur multi-fichiers (G-0a), tinymist (L3) |
| P2, P3 | protocole, étapes 2 et 3 d'ADR 0034 (signature N-1 ; profil CBOR et schémas) |
| RL1 | règle L de la spec sur sync/1, perte silencieuse corrigée |
| NI0 | réseau perturbé (netsim) et mesures de lien v1 |
| WB, SP1, SP1b | budget d'écriture puis chemin d'envoi de sync/1 ; WB remplacé par SP1, SP1 par SP1b |
| CI1, CI1b | CI plus rapide (netsim dégradé seulement si touché) ; CI1 remplacé par CI1b |
| C5S | mesure de perte par la sonde de lien (ADR 0036) et clause C5 sûre |
| PB, RG | mesures de performance appariées et entrelacées ; garde de benchmark robuste à la mise en page du code |
| GR | reprise d'une porte critère par critère sur le même candidat |
| RC1 | reconnexion après redémarrage sans relais |
| SB1, SB1b | rapports de plantage et mémoire du bac à sable typst ; SB1 remplacé par SB1b |
| SI1, SI1b | indicateur de synchronisation honnête ; SI1 remplacé par SI1b |
| OI | une seule instance par profil et ouverture avec (OI1, OI2 : W41) |
| LF0, LF0a, LF0a2, LF0a3, LF0b | coloriage incrémental (W42) ; chaque variante remplace la précédente, LF0a3 livré, LF0b = analyseur Typst à état explicite |
| PR1 | présence correcte (W46 : départ, présence figée, couleurs ; R1) |
| SI1c | en-tête stable et lisible, amende DS-3 de SI1b (R1) |
| DU | mise à jour ciblée des dépendances pour R1 (`uuid` ≥ 11.1.1) |
| PF1 | light, no gate: every frontend build keeps the tracked `frontend/dist/.gitkeep` (Vite plugin `distPlaceholder`), so the PERF-START reference build no longer dirties its worktree; decision in `docs/autonomous-decisions.md` (2026-11-24) |
| DEP1 | mises à jour des dépendances et audit des versions (W22, W48) |
| OBS1 | observabilité locale : point `metrics` du registre, mesures et état de débogage gardés sur l'appareil (W86) |
| Sans porte encore | AW1, PA, AA (audits), ST1, DG1, DG2, CP0 à CP5, LF0c, LF1, IM1, G-0b, G-0c, S13, N35, Fondations-extensibles |

## Lots suivants

L'ordre des blocs a une seule place : `PROGRESS.md`, « Cap à long terme » (la liste qui figurait ici, datée du
2026-11-08, est retirée le 2026-11-24 : elle contredisait cet ordre, G-0b compris).

### Lots connus et critères reçus en report

Chaque report (`status = "deferred"`) d'une porte vise un lot de cette liste ou un `gates/<lot>.toml`
existant, qui doit lister le critère à nouveau ; sinon `scripts/gate.py` le compte en échec. Un
critère reporté plusieurs fois figure dans chaque lot de la chaîne (PERF-MEM : L0 → L2 → G-0b).

```gate-lots
L2: PERF-TYPE-native PERF-MEM
L3: PERF-TYPE-native
G-0b: PERF-MEM
N35: native-cgo-darwin-windows seccomp-trace-arm64
Fondations-extensibles: PERF-START-plugins
```

## Décisions en vigueur

Déplacées le 2026-11-24 (une seule place par décision) : les décisions de l'utilisatrice dans
[`design/decisions-utilisatrice.md`](design/decisions-utilisatrice.md) (« Décisions de pilotage »), les décisions
techniques dans [`autonomous-decisions.md`](autonomous-decisions.md) (« Carried over from objectif-en-cours.md »).
Ce fichier ne garde que le registre lu par les outils.
