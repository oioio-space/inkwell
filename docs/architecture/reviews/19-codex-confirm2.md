# Revue confirmatoire Codex — architecture 8.3 / design 7.2

2026-11-02. Revue documentaire du delta annoncé dans `docs/architecture/CHANGES.md`, section « 1.0.0-brouillon.12 », et `docs/design/CHANGES.md`, section « Addendum rév. 7.2 », confronté à `18-codex-confirm.md`. Les références ci-dessous sont des chemins depuis la racine du dépôt, suivis de lignes ou de sections. Aucune décision utilisatrice n'est rouverte. Aucun test d'implémentation exécuté ; les exemples sont des contre-exemples de contrat. Les portes et vecteurs annoncés restent des obligations futures, pas des résultats de tests.

## Vérification de C1–C7

| Item | Conclusion et preuve |
|---|---|
| C1 — preuve PD-8 | **Correction technique substantielle, fermeture inter-spécifications impossible (B1).** `docs/architecture/spec.md:3905–3917` définit deux opérations complètes, domaine exact lié au ProjectID, preuve propre, conservation, diffusion et gel. VD-2/VD-3 (§6.15) remplace le traitement des décisions successives : seule la première compte. Ce choix ne demande pas une nouvelle décision utilisatrice, mais le design n'applique pas cette règle. |
| C2 — retrait ∥ décision | **Course initiale corrigée ; pli global incomplet (B2).** `docs/architecture/spec.md:1955–1983` évalue les opérations dans leur passé déclaré et donne priorité à la décision sur le retrait concurrent, avec vecteurs de permutations. Le design §4.48 décrit les badges et l'intégration, mais ne reprend ni ne contredit explicitement cette priorité. L'agrégation de plusieurs demandes reste sous-définie. |
| C3 — plafonds et causalité | **Obstacle initial levé au niveau du contrat.** CM-6 et PD-7 (`docs/architecture/spec.md:1901–1921`, §15.19 PD-7) remplacent le refus global par des limites de modification, une histoire bornée sur chaque chaîne d'acteur, un quota par clé et un palier local de chargement. Les branches de plusieurs auteurs dépassant ensemble 16 Mio sont admises ; les boucles création/suppression consomment la part de leur auteur (CM-10). Le design §4.27 conserve fils, suppression et masquage personnel, sans réintroduire de plafond global. Norme locale : `docs/protocol/spec.md` §9.7 G3 et garde sur `before(c)`. Le quota de stockage N10 reste distinct de la validité CRDT, conformément au protocole §8 et §16. |
| C4 — première écriture concurrente | **Fermé pour le scénario signalé.** `docs/architecture/spec.md:1580–1591` et PD-6 (§15.19) fixent une même identité dérivée et un même état initial pour tous les auteurs ; aucun ancrage individuel ne recrée deux documents. Le design §4.22, paragraphe « Décidé », conserve fourche de l'index et fichiers fourchés à la première écriture. Norme locale : VA-3 ; le vecteur `variant-file-fork-concurrent-first-write` est prescrit par PD-6. |
| C5 — consentement inter-variante | **Fermé.** Architecture §6.11, paragraphe « Présence » (`docs/architecture/spec.md:1634–1638`), et RV-3 ; design §4.22, paragraphes « Suivre » et « Pendant le suivi » : accord avant changement, nouvelle question si la cible change de variante, suspension et arrêt en cas de refus. Norme : N46 et VA-1. La contradiction de présence antérieure à PD-5 est distincte (B3). |
| C6 — état contesté | **Rendu ajouté, sémantique encore incompatible (B1/B2).** Le design `docs/design/spec.md:3091–3116` définit forme, texte, nom accessible, accès clavier aux décisions et annonce polie ; architecture VD-5 émet `review.statusChanged`, interdit l'intégration des états contestés et conserve un audit tardif. Le contrat accessible existe désormais, mais les événements qui doivent le déclencher divergent. Normes locales : VD-3/VD-5 et design §4.48, exigences d'accessibilité des badges. |
| C7 — restes documentaires | **Corrections principales faites ; quelques mentions périmées subsistent.** Design §4.22 « Décidé », §4.27 (renvoi CM-1 à CM-10), §4.46 (`fonts/`) ; architecture FT-1 ; `docs/architecture/decisions.md` §AD-106/107 : décisions acquises. Le design `spec.md:1059–1060` dit encore que SY-7 vaut 100 ms et SY-4 « en option », et §4.22 dit encore l'architecture « à aligner » sur le consentement. Les règles effectives sont pourtant corrigées dans l'architecture. Mineur documentaire, sans obstacle autonome : retirer ces descriptions périmées, sans changer les décisions. |

## Vérifications ciblées sans obstacle supplémentaire

- **300 ms et activé par défaut : concordants.** Design §4.18 (`docs/design/spec.md:1055–1060`) ; architecture SY-4 et SY-7 (`docs/architecture/spec.md:3547–3549`, `3562–3567`). Le délai de calme est distinct du budget de latence après déclenchement ; le clic reste immédiat. La mention périmée du design relevée en C7 ne crée pas une deuxième valeur normative à adopter.
- **Polices : concordantes.** Design §4.46, « À famille et variante identiques » (`docs/design/spec.md:2998–3002`), et architecture FT-1/FT-2 (`docs/architecture/spec.md:1732–1745`) : projet `fonts/` → Inkwell → mes polices permanentes → ordinateur. L'ancien ordre du CHANGES design, addendum 7.1, est explicitement remplacé par son addendum 7.2, « Point 11 ». Aucun motif pour rouvrir cet ordre décidé.
- **Commenter ⇧⌘K : override documenté et acceptable comme contrat.** Design §8.2 (`docs/design/spec.md:3734`, `3776–3792`) impose `Prec.high`, nomme le masquage intentionnel de `deleteLine`, garde cette dernière dans la palette et étend K-6 aux trois claviers CodeMirror. Architecture CHANGES 8.3, « Pour le design », points 1 et 7, demande ce même traitement, y compris en grille. Preuve locale du conflit : `frontend/node_modules/@codemirror/commands/dist/index.js:1812` lie `Shift-Mod-k` à `deleteLine`. Preuve du mécanisme : `frontend/node_modules/@codemirror/state/dist/index.js:1830–1848` décrit le tri par précédence et `Prec.high` avant la précédence par défaut. La spécification doit être implémentée avec une commande qui consomme la touche dans son contexte ; cette revue ne prouve pas que le futur gestionnaire le fait. La norme locale K-6 autorise désormais explicitement cette collision résolue : elle n'est donc pas un obstacle en soi.
- **Intégration exacte : progrès conservé.** Architecture VD-5 et design §4.48 (`docs/design/spec.md:3131–3142`) font intégrer le manifeste validé, annoncent les changements plus récents exclus et gardent le contrôle comme règle des clients honnêtes avec audit. Les états qui ouvrent ce chemin restent affectés par B1/B2.

## Obstacles restants et régression

### B1 — Majeur : deux définitions incompatibles de « Validation contestée » (C1/C6)

**Norme :** architecture VD-2/VD-3 et PD-8, et design §4.48 doivent définir le même état observable ; VD-5 interdit l'intégration d'une validation contestée.

**Preuve :** `docs/architecture/spec.md:1965–1978` et `3912–3916` : une deuxième décision à une position différente est ignorée, même contraire ; seule une fourche de chaîne constitue la preuve d'équivoque. `docs/design/spec.md:3098–3105` : deux décisions contraires sur une demande déclenchent « Validation contestée », remplacent la coche et ne comptent plus comme validation.

**Contre-exemple :** la personne désignée signe approve à position 4 puis reject à position 5, sur une seule chaîne. Architecture : validée, deuxième décision sans effet. Design : contestée, intégration bloquée. Inversement, une fourche de la chaîne de l'auteur de la soumission, visée par VD-3, n'est pas couverte par la définition design limitée aux deux verdicts contraires de la personne désignée.

**Correction nécessaire :** aligner le déclencheur et le contenu accessible du design sur le gel de chaîne défini par PD-8, y compris lorsque les deux opérations probantes ne sont pas deux verdicts contraires. Ne pas modifier le choix acquis des opérations signées.

### B2 — Bloquant : agrégation de plusieurs soumissions non totale et reprise UI incompatible (C2/C6)

**Norme :** `docs/protocol/spec.md:426–428` (§4.9) exige une fonction de l'ensemble indépendante de l'arrivée ; architecture VD-3 annonce ce même déterminisme, et VD-5 conditionne l'intégration à son résultat.

**Preuve :** `docs/architecture/spec.md:1978–1983` : une version est validée si une demande est validée et aucune refusée ni contestée ; « avis contraires si l'on a des deux » ; sinon état de la demande « la plus récente sur la chaîne de son auteur ». Or VD-2 permet plusieurs auteurs qui écrivent, sans ordre commun entre leurs chaînes. Cas concret : deux auteurs soumettent la même VersionId ; une demande est retirée, l'autre encore ouverte, sans verdict. Aucun ordre entre leurs positions ne désigne la demande la plus récente. Autre lacune : une seule demande contestée et une demande validée ne satisfont pas la condition validée, sans définition explicite de l'agrégat contesté ou de « des deux ».

Le design `docs/design/spec.md:3115–3116` promet qu'une « nouvelle demande, validée sans avis contraire » permet de sortir du blocage. Pour la même VersionId ayant déjà une demande refusée, VD-3 conserve cette demande refusée et donc le blocage même après une nouvelle validation. La règle design « personnes différentes » (`3108–3109`) exclut aussi le cas de deux demandes distinctes confiées à la même personne avec des verdicts différents, que VD-3 n'exclut pas.

**Correction nécessaire :** définir une table totale des agrégats et un ordre déterministe inter-auteurs pour tout repli ; préciser quelles demandes restent prises en compte. Aligner le texte de reprise du design sur cette règle. Ajouter les cas sans verdict, contesté + validé, et deux demandes au même assignee aux vecteurs de permutations déjà annoncés. Aucun changement de rôle ni de modèle de stockage requis.

### B3 — Majeur, régression : fuite de présence inter-variante avant PD-5

**Norme :** N46, architecture VA-1/VA-3 et design §4.22 : les curseurs visibles appartiennent à la même variante ; le repli architecture §6.11 impose l'absence d'émission plutôt qu'un mélange.

**Preuve :** `docs/architecture/spec.md` §6.11 « Présence » (`1630–1634`) : sans PD-5, aucune présence émise sur un fichier non fourché d'une variante. `docs/design/spec.md` §4.22 « Présence » (`1263–1266`) : avant le canal `variant`, les curseurs du fichier s'affichent pour tous. Ouvrir le même fichier non fourché depuis l'Original et une variante produit donc soit aucun curseur de la variante, soit un curseur mélangé selon le document suivi.

**Correction nécessaire :** aligner le repli design sur l'absence d'émission prescrite par l'architecture ; ne pas affaiblir l'isolation décidée. Ce nouvel ajout du design empêche de confirmer l'absence de régression.

## Verdict

Les corrections C3, C4 et C5 répondent aux contre-exemples initiaux ; le délai, l'ordre des polices et le masquage explicite de ⇧⌘K sont acceptables. Les contre-exemples B1–B3 suffisent à empêcher une confirmation commune des deux textes et une affirmation d'absence de régression. Les portes P-COMMENT/P-VAR et la ratification des deltas de protocole restent les prérequis annoncés dans l'architecture (§0.5 et §15.19), sans constituer une preuve d'implémentation passée.

NOT ACCEPTED — blockers : B1 (déclencheur contesté incompatible, normes VD-2/VD-3/PD-8 et design §4.48) ; B2 (agrégat non total et reprise contradictoire, norme protocole §4.9 et VD-3/VD-5) ; B3 (repli de présence inter-variante divergent, norme N46 et architecture §6.11). Preuves et corrections nécessaires détaillées ci-dessus.
