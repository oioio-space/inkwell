# Revue confirmatoire Codex — design 7.1 / architecture 8.2

2026-11-02. Revue documentaire, limitée aux ajouts indiqués dans les deux CHANGES : N43–N48, nouveaux raccourcis, variantes, exemption des auxiliaires WebKitGTK et PD-5–PD-9. Les bases acceptées (design 7, architecture 8.1) ne sont pas remises en revue. `17-fable-confirm.md` n'a pas été lu. Aucun test d'implémentation exécuté ; les scénarios ci-dessous sont des contre-exemples de spécification, pas des incidents observés.

AD-106 (un document Automerge de commentaires par fichier et index SQLite dérivé) et AD-107 (opérations signées, personne désignée sans rôle nouveau) sont des décisions acquises de l'utilisatrice. Les constats demandent de compléter leurs contrats, pas de changer ces décisions. Les renvois au protocole accepté servent de normes locales pour contrôler les seuls nouveaux usages.

Gravités : **bloquant** = garantie de sécurité/convergence non définie ; **majeur** = comportement requis incompatible ou incomplet ; **mineur** = défaut documentaire sans changement de décision. Les majeurs signalés comme obstacles ci-dessous empêchent aussi la confirmation du delta.

## Constats

### C1 — Bloquant : PD-8 ne peut pas réutiliser la preuve d'équivoque annoncée

**Preuve.** Architecture §6.15 VD-2, lignes 1928–1935, et §15.19 PD-8, lignes 3839–3844, renvoient à la preuve existante pour deux verdicts contraires. Or [protocole §4.8](../../protocol/spec.md) (lignes 415–423 ; lire le titre §4.8) encode `epoch`, `seq`, deux empreintes et deux signatures vérifiables selon §4.7, domaine `change`. PD-8 signe dans `review-op`, avec `position`/`prev`, sans `epoch`. De plus, deux verdicts contraires peuvent être signés à deux positions successives : ce n'est pas une collision `(acteur, seq)`.

**Norme.** Protocole accepté §3.3 (séparation des domaines), §4.7–4.8 (octets et vérification de la preuve), §9.7 G8. Une signature d'un autre domaine ne vérifie pas comme tuple d'auteur Automerge.

**Correction attendue.** Définir dans PD-8 sa preuve vérifiable, sa diffusion et son pli : deux opérations signées incompatibles sur la même soumission, y compris à des positions différentes ; préciser les octets signés et la liaison au projet. Ajouter les vecteurs correspondants. La boucle du protocole peut ratifier ce contrat ; le renvoi actuel ne le définit pas.

### C2 — Bloquant : retrait et décision concurrents n'ont pas de résultat déterministe

**Preuve.** Architecture VD-2 autorise `withdraw` « avant toute décision », VD-3 donne trois issues terminales, VD-9 ne teste que le retrait **après** décision. Après une soumission S, son auteur peut retirer hors ligne pendant que la personne désignée valide hors ligne. Les deux opérations voient S et aucune ne voit l'autre. Les frontières `members`/`decisions` de PD-8 portent sur l'autorité, pas sur un ordre total entre ces deux signataires. Le texte ne dit pas quel état produire en réunissant les deux opérations.

**Norme.** [Protocole accepté](../../protocol/spec.md) §4.9 : pli indépendant de l'ordre d'arrivée ; §9.7 : admission par le passé causal. C'est le contrat de déterminisme déjà accepté que VD-2 invoque.

**Correction attendue.** Définir validité causale et priorité déterministe pour retrait/décision concurrents, ainsi que plusieurs soumissions de la même VersionId. Tester toutes les permutations de réception et le résultat utilisé par VD-5. Ne pas décider selon la première opération reçue.

### C3 — Bloquant : les plafonds CM-6 ne sont pas compatibles, en l'état, avec la garde causale

**Preuve.** Architecture CM-6 (1891–1897) impose « document ≤ 16 Mio » avec refus par la garde ; PD-7 reprend ces bornes. Deux branches partent d'un document de 15 Mio et ajoutent chacune 0,75 Mio de messages de 4 Kio par des auteurs distincts. Chaque branche respecte le plafond dans son passé causal ; leur réunion dépasse 16 Mio. Refuser la dernière reçue diverge selon l'ordre d'arrivée ; admettre les deux viole le plafond annoncé. Même problème pour 2 000 objets d'un auteur utilisant plusieurs générations d'acteur. La borne sur les objets vivants ne borne pas non plus l'histoire conservée après suppressions.

**Norme.** [Protocole §9.7](../../protocol/spec.md), phrase normative `before(c)` ; §10.1, plafonds partagés avec parts réservées. CM-6 ajoute un invariant non stable par union sans mécanisme de réservation.

**Correction attendue.** Distinguer borne d'une modification, quota d'histoire/stockage et plafond de vue matérialisée ; définir allocation ou traitement déterministe des dépassements concurrents. Tester les deux branches valides réunies et la création/suppression répétée. Conserver le modèle de stockage décidé.

### C4 — Majeur, obstacle : première écriture concurrente dans une variante non définie

**Preuve.** Architecture §6.11, Identité (1578–1585), crée la fourche paresseuse à la première modification. Son DocId dépend de `author`, `slot` et `anchor` ; chaque créateur ajoute sa propre modification d'ancrage. Deux personnes ouvrant le même fichier non fourché de V peuvent donc créer deux fourches distinctes et modifier concurremment le pointeur de l'index. VA-3 promet pourtant la collaboration sur la **même** fourche. Aucun arbitrage ne précise comment conserver les deux éditions sans perdre la branche non retenue.

**Norme.** VA-3 et N46 ; [protocole §4.6](../../protocol/spec.md) (identité calculée) ; [modèle Automerge](https://automerge.org/docs/reference/documents/) : une map peut contenir des références mais la fusion de l'index ne fusionne pas à elle seule deux documents distincts.

**Correction attendue.** Définir une identité/initialisation commune ou une résolution conservant les éditions des deux fourches ; inclure ce scénario dans P-VAR. Cela ne rouvre ni le partage ni l'isolation des variantes.

### C5 — Majeur, obstacle : le suivi inter-variante perd le consentement explicite du design

**Preuve.** Design §4.22 (1251–1254) exige « Passer et suivre · Annuler » et « Rien ne change sans cet accord ». Architecture §6.11 (1628–1629) indique que le suivi « bascule la variante courante, avec le bandeau », sans cette étape. RV-3 ne la rétablit pas. Un bandeau après le changement ne satisfait pas cette condition.

**Norme.** Exigence N46, design §4.22 ; invariants de vue personnelle VA-1 et RV-2. La décision utilisateur est déjà explicite.

**Correction attendue.** Prescrire la confirmation avant tout changement, y compris si la personne suivie passe ensuite sur une autre variante ; suspension du suivi pendant ce choix. Ajouter acceptation/annulation au scénario RV-8.

### C6 — Majeur, obstacle : l'état « contesté » n'a aucun rendu accessible ni règle d'intégration explicite

**Preuve.** Architecture VD-2 exige l'état « contesté », les deux décisions montrées. Design §4.48 (3010–3048) ne définit que En attente, Validée, Refusée, Retirée ; VD-3 omet également contesté. Le badge accessible et le statut avant intégration ne précisent donc pas le rendu de la preuve hostile ni si une validation contestée satisfait VD-5.

**Norme.** [WCAG 2.2](https://www.w3.org/TR/WCAG22/) SC 1.3.1, 4.1.2 et 4.1.3 : état et relations perceptibles par les aides techniques, changements de statut annoncés ; exigence locale VD-2. Le constat porte sur un contrat UI manquant, pas sur une violation observée dans un produit exécuté.

**Correction attendue.** Définir badge, nom accessible et annonce polie de « Validation contestée », accès clavier aux deux décisions et traitement explicite dans VD-5. Tester l'arrivée tardive du second verdict : une ancienne coche ne doit pas rester la seule information exposée.

### C7 — Mineur : des restes documentaires contredisent les décisions et modèles désormais acquis

**Preuve.** Design §4.22 (1259–1262) parle encore de variantes par document « à confirmer », §4.27 (1371–1372) du modèle de commentaires « reste à écrire ». Architecture CHANGES 8.2 (40–42) et prose AD-106/107 disent encore « attendent »/« à confirmer », alors que la matrice de decisions.md (170–171) les donne décidés le 2026-11-02. Design §4.46 prescrit `polices/`, architecture FT-11/RA `fonts/`.

**Norme.** Décisions utilisatrice AD-106/107 ; VA-5 et PD-7 ; règle de cohérence design P2. Il s'agit de traçabilité et d'un chemin canonique divergent, pas de motifs pour redemander une décision.

**Correction attendue.** Aligner ces mentions sur les décisions acquises et choisir un seul chemin de stockage documenté.

## Contrôles sans obstacle supplémentaire

- **N43** : consentement unique, fichiers externes non cochés, priorité projet avant ordinateur avant ressources livrées, absence d'inventaire distant et absence de contrôle de licence concordent avec FT/RA. RA-2 conserve une vérification de portabilité ; P-FONT reste une porte, pas une preuve déjà passée. Le chemin divergent est C7.
- **N44/N45** : SY-3 segmente désormais les correspondances ; SY-5 évite les boucles et RV-1/2 préservent la vue sous modifications distantes. P-SYNC reste exigée pour la précision avancée ; aucun motif pour rouvrir le choix accepté du moteur ou du repli. C5 concerne exclusivement l'ajout des variantes.
- **Variantes et points d'étape** : VA-5–VA-9 conservent manifestes figés, restauration dans la variante, création depuis une version, épingles transitives et purge prioritaire. PD-6(a) garde les quotas existants ; l'option (b) reste explicitement proposée, non autorisée en production. C4 empêche de confirmer la collaboration dès la première édition.
- **WebKitGTK** : l'exception INV-14 est bornée aux auxiliaires des bibliothèques garanties lancés par celles-ci ; elle ne donne pas aux extensions un droit de lancer des programmes. DG-2 et AC-19 gardent le contrôle des dépendances. Le [guide officiel WebKitGTK 6.0](https://github.com/WebKit/WebKit/blob/main/Source/WebKit/gtk/migrating-to-webkitgtk-6.0.md) confirme le bac à sable obligatoire. P-DEPS sous Wails reste une vérification requise ; rien ici ne justifie de désactiver ce bac à sable.
- **Raccourcis nouveaux** : Ctrl+Maj+M / ⇧⌘K, Ctrl+Maj+N / ⇧⌘N, Maj+F2 et Ctrl+Entrée du composeur ne doublonnent aucune action de la table §8.2 dans leur contexte. Aucun n'appartient aux tables réservées Linux/macOS/Windows de cette section. Les raccourcis Finder propres à une autre application ne sont pas des réservations globales ([Apple](https://support.apple.com/en-us/102650)). Ctrl+Entrée manque toutefois au registre par système : à y inscrire avec son contexte et sa liaison macOS explicite, conformément à K-6. Aucun essai des moteurs, méthodes de saisie, lecteurs d'écran ou raccourcis personnels n'a été exécuté : absence de collision universelle non démontrée ; tests K-6 et e2e sur les trois systèmes restent nécessaires.
- **WCAG/ARIA des nouvelles surfaces** : les règles communes de §4 restent applicables (focus et retour, noms, clavier, états, repli étroit). Liste de polices en groupes nommés, échantillon décoratif, boutons 44px, bascule `aria-pressed`, fils nommés, motifs/formes en complément des couleurs et annonces polies sont cohérents avec WCAG 1.3.1, 1.4.1, 2.1.1, 2.4.3, 2.5.8, 4.1.2/3. Le fil CSV interactif doit appliquer le popover avec accès aux réponses, pas un `tooltip` : [APG Tooltip](https://www.w3.org/WAI/ARIA/apg/patterns/tooltip/) exclut les éléments focalisables dans une infobulle. `aria-describedby` du passage doit référencer un élément présent même quand le fil est replié ([WAI-ARIA 1.2](https://www.w3.org/TR/wai-aria-1.2/#aria-describedby)). Ces obligations d'implémentation ne démontrent pas une non-conformité actuelle ; C6 est la lacune documentaire certaine. Contrastes, reflow des marges et Orca/VoiceOver/NVDA restent à mesurer selon §10, dont les tests sont honnêtement « à écrire ».
- **Limite de l'approbation** : VD-5 dit correctement que la politique d'intégration est une garde des clients honnêtes et un audit, pas une restriction cryptographique des écritures ordinaires. Ce choix acquis n'est pas rouvert. La signature lie la version, elle ne transforme pas le PDF exporté en PDF signé.

## Verdict

Les acceptations antérieures restent acquises ; le delta introduit des contrats incomplets qu'une simple porte de performance ne résout pas. Les nouveaux choix ne sont pas à redécider. La boucle du protocole doit définir puis ratifier les nouveaux formats et règles avant leur activation.

NOT ACCEPTED — blockers : C1 (preuve d'équivoque PD-8), C2 (pli retrait/décision concurrent), C3 (bornes CM-6 en concurrence), C4 (première fourche concurrente), C5 (accord avant suivi inter-variante), C6 (état contesté accessible et traitement pour l'intégration).
