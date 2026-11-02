# Outils externes à suivre : revue et verdicts (2026-11-02)

Méthode : README de chaque dépôt lu le 2026-11-02 (`raw.githubusercontent.com`, API GitHub pour licence et activité). Les fichiers
`SKILL.md` n'ont pas été relus un par un : les verdicts reposent sur les README, ce qui suffit pour décider « idée / installer /
ignorer ». Rien n'est installé. Règle : on **copie la pratique** (avec attribution et licence), on n'installe pas du code tiers
qui s'exécute dans les sessions (hooks, `npx`, `setup`).

## Synthèse

| # | Dépôt | Licence | Ce que ça fait vraiment | Verdict |
|---|-------|---------|-------------------------|---------|
| 1 | obra/superpowers | MIT | Chaîne de skills (brainstorming, plans, TDD, revue, débogage) | Déjà installé ; garder |
| 2 | multica-ai/andrej-karpathy-skills | non déclarée dans l'API (README : MIT) | Un `CLAUDE.md` de 4 principes contre les défauts des LLM | ADOPTER COMME IDÉE |
| 3 | firecrawl/firecrawl | AGPL-3.0 | API de scraping/recherche web vers Markdown (service hébergé ou auto-hébergé) | IGNORER |
| 4 | anthropics/skills | non déclarée (Apache 2.0 pour une partie ; docx/pdf/pptx/xlsx sous licence propre) | Exemples de skills + spécification Agent Skills | ADOPTER COMME IDÉE (référence de format) |
| 5 | garrytan/gstack | MIT | ~40 skills « rôles » (CEO, Eng, Design, QA, CSO, ship) + navigateur compilé (Bun) | ADOPTER COMME IDÉE ; ne pas installer |
| 6 | thedotmack/claude-mem | Apache-2.0 | Capture auto des appels d'outils, résumés, SQLite+Chroma, 4 outils MCP | IGNORER (idées seulement) |
| 7 | Leonxlnx/taste-skill | MIT | Skill de design web (20 directions, build 30-45 min, contrôle navigateur) | IGNORER (une idée) |
| 8 | heygen-com/hyperframes | Apache-2.0 | HTML/CSS vers vidéo MP4 déterministe | IGNORER (hors sujet), à revoir pour démos |
| 9 | blader/humanizer | MIT | Réécrit le texte pour retirer 26 tics d'IA (base : Wikipédia « Signs of AI writing ») | ADOPTER COMME IDÉE (version française) |
| 10 | coreyhaines31/marketingskills | MIT | Skills marketing (SEO, CRO, copywriting) ; partenaires commerciaux | IGNORER |

## Détail par dépôt

### 1. superpowers (déjà installé)
Rien à ajouter. Vérifier seulement que nos règles locales (porte `/simplify`, TDD, revue) ne divergent pas de ses skills.

### 2. andrej-karpathy-skills — ADOPTER COMME IDÉE
Quatre principes issus du message de Karpathy (source : README) : **Réfléchir avant de coder** (énoncer ses hypothèses, présenter
les interprétations, s'arrêter quand on est perdu), **Simplicité d'abord**, **Changements chirurgicaux** (« chaque ligne modifiée
se rattache à la demande »), **Exécution pilotée par l'objectif** (critères vérifiables, boucle jusqu'à vérification).
Recouvrement : fort. Notre mémoire et nos règles couvrent déjà `feedback-no-empiricism` (preuve mesurée), `feedback-simplify-gate`,
`feedback-feasible-first`, la porte exécutable HDAE (`mise run gate`), le skill `simplify` et la règle « un paquet = un rôle ».
Manque réel, à copier dans `CLAUDE.md` en 4 lignes :
1. **Hypothèses énoncées avant d'agir** quand la demande est ambiguë (nous n'avons pas de règle écrite).
2. **Changements chirurgicaux** : ne pas reformater ni « améliorer » le voisinage ; signaler le code mort préexistant sans le
   supprimer ; retirer seulement les orphelins créés par soi. Le gate `/simplify` pousse plutôt à nettoyer : à arbitrer
   explicitement (le nettoyage reste limité au diff).
3. **Test de lecture du diff** : chaque ligne se rattache à l'exigence (`docs/exigences.md`).
4. Critère de bon fonctionnement du README : « moins de lignes inutiles dans les diffs » ; mesurable via la revue de lot.
Risque d'installation : faible (un fichier Markdown), mais inutile : on copie, avec attribution (README MIT).

### 3. firecrawl — IGNORER
AGPL-3.0 (contaminant si on l'intègre ou l'auto-héberge en service) ; le service hébergé envoie nos requêtes à un tiers, contraire
à l'esprit « sans serveur » et à la règle « Typst : jamais de téléchargement à l'exécution » (qui ne vise pas l'outillage, mais le
principe de moindre dépendance). Recouvrement : `WebFetch`, `WebSearch`, Playwright MCP déjà disponibles, `scripts/ghx.sh` pour
GitHub. Pas de besoin produit (aucun scraping dans inkwell). Risque : clé d'API, flux de données sortant, AGPL.

### 4. anthropics/skills — ADOPTER COMME IDÉE
Utile comme **référence de format** (spec Agent Skills, `template/`) quand nous écrivons ou revoyons nos skills : description
courte et déclencheuse, chargement progressif (SKILL.md court, références à part), scripts embarqués. Nos 12 skills de
`.claude/skills` suivent déjà ce format ; le plugin `skill-creator` est disponible pour les valider. Ne pas installer les skills
documents (docx/pdf/pptx/xlsx) : licences propres, inutiles (la sortie PDF passe par Typst). Le README précise « démonstration et
éducation ». Risque : faible, mais aucun gain.

### 5. gstack — ADOPTER COMME IDÉE, ne pas installer
Ce que c'est : un « studio » de rôles (CEO, Eng Manager, Designer, Staff Engineer, QA, CSO, Release Engineer, Technical Writer)
invoqués par commandes, plus un navigateur piloté (`/browse`) et un installeur `./setup` (Bun, mises à jour automatiques, mode
« équipe » qui commit un hook obligatoire).
Rôles de gstack contre notre routage (`CLAUDE.md`, agents) :
| gstack | Chez nous |
|--------|-----------|
| `/office-hours`, `/plan-ceo-review`, `/spec` | `superpowers:brainstorming` + `docs/plans/` + revue double Fable/Codex ; manque : questions « forçantes » avant spec |
| `/plan-eng-review`, `/autoplan` | `architect` (Opus) + boucle de revue double ; équivalent |
| `/review` (« bugs qui passent la CI ») | `go-reviewer` + `/simplify` + Codex adversarial ; équivalent |
| `/qa`, `/design-review` | skill `ux-walkthrough` + agent `ux-tester` ; équivalent, plus rigoureux (personas, Nielsen) |
| `/cso` | `security-auditor` + `vuln-guard` ; équivalent |
| `/ship`, `/land-and-deploy` | `scripts/qpush.sh`, `mise run ci`, porte HDAE |
| `/retro`, `/learn` | mémoire + `PROGRESS.md` ; `/learn` (apprentissages triés/élagués) à envisager |
| `/investigate` | `superpowers:systematic-debugging` (même « loi d'airain », arrêt après 3 corrections ratées) |
Idées à reprendre (pratiques, pas le code) :
- **`/test-audit`** : balayer périodiquement les tests qui coûtent plus qu'ils ne protègent (greps de source, doublons, sondes sans
  assertion, exports réservés aux tests). Cohérent avec notre règle « pas d'inflation de tests ». À écrire comme tâche `mise` +
  passage de revue, en s'appuyant sur nos tests de propriétés.
- **Garde d'intégrité de la porte** : `/review` signale tout test, délai ou instantané *affaibli* dans le diff. À ajouter à la
  porte HDAE (`mise run gate`) : échec si un seuil, un timeout ou une assertion est relâché sans entrée dans `docs/exigences.md`.
- **`/write-skill`** : « corriger l'outil d'abord, n'ajouter que ce que le modèle ne sait pas déjà, éditer la ligne plutôt
  qu'ajouter un paragraphe » : bonne règle d'hygiène pour nos skills, qui grossissent.
- **Notation 0-10 par dimension de design, avec « à quoi ressemble 10 »** (`/plan-design-review`) : à intégrer dans le rubric
  fixe de `ux-walkthrough` (notre règle « standard + preuve mesurée + rubric fixe »).
- **`/ubicloud`** : exécuter une suite lourde sur une VM éphémère avec TTL : piste pour les mutations/e2e si le plafond de
  mémoire local (cgroup 8 Go) devient bloquant ; hors périmètre aujourd'hui.
Risques si installé (rédhibitoires) : `git clone` dans `~/.claude/skills` + `./setup` exécute un script arbitraire ; Bun requis ;
**mise à jour automatique à chaque session** (code non épinglé, contraire à notre épinglage SHA-256) ; hooks « safety » bloquants
qui entrent en conflit avec nos hooks PreToolUse ; mode équipe qui commit à notre place ; chevauchement massif avec notre routage.
On perdrait aussi le contrôle du coût (modèle et effort choisis explicitement à chaque lancement).

### 6. claude-mem vs notre plugin `remember` — IGNORER
Ce que c'est : hooks qui capturent chaque appel d'outil, un « observateur » LLM qui résume, stockage SQLite + Chroma, 4 outils MCP
avec un schéma de recherche en 3 couches (index compact → chronologie → détail à la demande), balises `<private>` pour exclure du
texte. L'installateur propose une connexion à un compte (« claude-mem observer », essai de 30 jours, fournisseur hébergé par
défaut), un worker local et plusieurs intégrations d'IDE.
Notre `remember` (+ mémoire fichier `MEMORY.md` avec règles typées) : écrit à la demande, texte lisible, versionnable, sans
service résident. Différence : claude-mem est exhaustif et automatique, nous sommes curatés. Pour ce projet, la curation gagne :
les règles utiles sont des décisions (« pas d'empirisme », « modèle + effort à chaque agent »), pas des journaux d'outils ; un
observateur LLM supplémentaire coûte des tokens (contre notre règle d'économie), et un fournisseur hébergé recevrait du code et du
contenu d'architecture non publiés. Idées à retenir :
- **Divulgation progressive** : index court d'abord, détail à la demande. Nous l'appliquons déjà (`MEMORY.md` en une ligne par entrée).
  Pareil pour `docs/` : garder `CLAUDE.md` court et pointer.
- **Balise `<private>`** : notre `secret-guard` couvre les secrets ; rien à ajouter.
- Si un jour la mémoire devient trop volumineuse pour l'index : recherche FTS5 locale seule, sans embeddings ni compte.
Risque si installé : élevé (hooks sur tous les outils, worker résident, `npx` non épinglé, compte distant, journalisation de
contenu potentiellement sensible).

### 7. taste-skill — IGNORER (une idée)
Skill de **design web** (pages marketing, awwwards ; v2.1 ajoute tableaux de bord). Il tire une « direction » parmi 20, construit
pendant 30 à 45 minutes (« quelques centaines de milliers de tokens »), télécharge des images libres, vérifie dans Chrome à trois
largeurs et élimine le « slop ». Notre système Adwaita est accepté, contraint (GTK/libadwaita, `docs/ux-principles.md`,
règle de cohérence) : un générateur de directions originales irait contre « jamais de verrue ». Coût en tokens élevé. Il exige
Node 22 et Chrome et exécute des scripts. À retenir comme idée seulement : **la liste de contrôle « slop » exécutée dans un vrai
navigateur à trois largeurs, avec auto-correction des trois pires défauts** (jusqu'à trois tours). Cela s'ajoute à
`ux-walkthrough` (captures clair/sombre ; ajouter trois largeurs de fenêtre) ; `frontend-design` est déjà installé pour le reste.
Pour le site vitrine éventuel, la question se reposera.

### 8. hyperframes — IGNORER
Vidéo déterministe à partir de HTML/CSS (rendu local via CLI). Pas de besoin produit. À réexaminer si l'on veut des démos
reproductibles (capture automatisée des parcours `ux-walkthrough`). Idée à noter : « rendu déterministe et vérifiable par
script » rejoint notre porte exécutable. Installation : `npx` et plugin à mise à jour automatique, donc à éviter.

### 9. humanizer — ADOPTER COMME IDÉE (français, pas d'installation)
Le skill est conçu pour l'anglais (base : guide Wikipédia anglais). Les patrons structurels se transposent : « ce n'est pas X mais
Y », fermeture en une ligne, formules profondes creuses, mise en scène (« Voici le point »), inflation et autorité empruntée,
mise en forme systématique (listes, gras, tirets longs), restes de conversation, mauvais lecteur visé. Les listes de mots (« delve »,
« tapestry ») ne se transposent pas. Pour nos textes (interface, docs en français) : écrire une **liste de contrôle française
d'une page** dans `docs/` ou dans `readme-author` : pas de « non seulement… mais aussi », pas de formule de clôture, pas de
tirets cadratins en rafale, pas de gras décoratif, « personne » et zéro jargon (glossaire), voix active, une idée par phrase.
Elle sert de relecture, pas de réécriture automatique : le skill dit lui-même qu'il vise le lecteur et non les détecteurs.
Risque d'installation : faible (texte), mais l'anglais seul le rend peu utile ; copier la structure A-F avec attribution (MIT).
Précision : ne pas passer les spécifications normatives (`docs/protocol/`) à un réécrivain automatique.

### 10. marketingskills — IGNORER
Skills de marketing (SEO, CRO, copywriting) ; inkwell est un éditeur local sans acquisition en ligne. README sponsorisé par
des « partenaires vérifiés » : biais commercial, intégrations de comptes. Seule idée possible pour la future page de présentation :
le skill de base `product-marketing` (positionnement lu par tous les autres) rappelle notre `docs/glossary.md` et
`docs/conception.pdf` comme contexte unique ; déjà acquis.

## Actions proposées (aucune faite)

1. `CLAUDE.md` : ajouter une section « Comportements » (4 lignes tirées de karpathy, avec attribution MIT).
2. Porte HDAE : contrôle de relâchement (test, seuil, timeout) ; tâche `mise run test:audit` inspirée de `/test-audit`.
3. `ux-walkthrough` : notation 0-10 avec « ce que vaut 10 » ; trois largeurs de fenêtre ; limite de trois tours de correction.
4. `readme-author` (ou `docs/`) : liste de contrôle française anti-tics, relecture et non réécriture automatique.
5. Revue de nos skills avec `skill-creator` et la règle `/write-skill` de gstack (court, pas de redite du modèle).
6. Ne rien installer. Si l'un de ces dépôts est un jour retenu : épingler un commit, lire le code d'installation, désactiver la
   mise à jour automatique, exécuter dans un bac à sable, puis passer par `supply-chain-risk-auditor`.

## Sources

- https://github.com/obra/superpowers
- https://github.com/multica-ai/andrej-karpathy-skills (principes de Karpathy : https://x.com/karpathy/status/2015883857489522876)
- https://github.com/firecrawl/firecrawl
- https://github.com/anthropics/skills ; spécification : https://agentskills.io
- https://github.com/garrytan/gstack
- https://github.com/thedotmack/claude-mem
- https://github.com/Leonxlnx/taste-skill
- https://github.com/heygen-com/hyperframes
- https://github.com/blader/humanizer ; base : https://en.wikipedia.org/wiki/Wikipedia:Signs_of_AI_writing
- https://github.com/coreyhaines31/marketingskills
