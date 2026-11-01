# Revue Fable n°1 du système de design (2026-11-01) — verdict : revise

Texte transmis par la coordination (résumé des constats), archivé tel quel.

1. **[Majeur] §2.2, anneau de focus.** `2px var(--accent)` sur les surfaces sombres du thème
   clair échoue à 1.4.11 :
   - `--toast-bg` 2,39:1 ;
   - `--tooltip-bg` 2,07:1 ;
   - `--stage-bg` 3,46:1.

   → Ajouter `--focus-ring-on-dark` (blanc ou `#81d0ff`) pour le toast, l'infobulle et la scène,
   avec une paire de contraste dédiée.
2. **[Majeur] §3.1 / L-2 contre le plan T11 B10**, validé le 2026-10-26. Ce plan place à gauche
   « Fichiers | Sommaire | Tâches », en `AdwViewSwitcher`. La spec, elle :
   - déplace « Tâches » dans le tiroir de droite ;
   - ne mentionne jamais « Sommaire » ni le panneau « Liens » de B7.

   Deux autres écarts avec A6 :
   - la vue présentateur et le panneau de recherche sont redimensionnables (RSZ-07, RSZ-08), mais
     absents de §3.2 ;
   - A6 superpose l'aperçu sous 900px, la spec non.

   → Arbitrer explicitement.
3. **[Majeur] L-8, « un seul bandeau ».** La priorité ignore les **questions** :
   - modification extérieure en mode « Demander » (§5.12) ;
   - demande d'accès (§5.3, 4 bis) ;
   - passage de main (§5.9).

   Un état durable pourrait masquer une question en attente. → Une question passe avant un état,
   ou devient un dialogue.
4. **[Majeur] Commentaires.** On les trouve :
   - comme onglet du tiroir ;
   - dans la tâche « relire » ;
   - dans la référence à Google Docs.

   Mais il n'y a ni composant (ancre, fil, résolution, @mention) ni parcours. Le code n'a qu'un
   texte vide (`HistoryDrawer.svelte:207`). → Les définir, ou les déclarer hors du périmètre de
   N17.
5. **[Mineur] §2.5, cibles.** La règle 9 de `ux-principles.md` dit 44px. La spec fixe 24, 28, 34
   et 44px, alors que le README dit que les principes l'emportent. → Rendre l'écart explicite.
6. **[Mineur] §1.** P5 (zéro jargon) est classé dernier, alors que c'est un invariant de
   `CLAUDE.md`. → Le justifier ou le remonter.
7. **[Mineur] Jetons.**
   - `--disabled-opacity` vaut 40 % en contraste élevé dans libadwaita.
   - La règle T-11 « durées à 0 sauf fondus à `--duration-fast` » est inapplicable tant que
     `--duration-fast` sert aussi au survol. → Ajouter `--duration-fade`.
8. **[Mineur] Vocabulaire.** `fr.json:549` (« Avant fusion de… ») et `:656` (« Fusionner »)
   contredisent le vocabulaire figé (« Intégrer », jamais « fusionner »).
9. **[Mineur] §4.12 / A-13.** « Annuler » disparaît avec le toast (10s). → Dire que Ctrl+Z et
   l'Historique le rattrapent (2.2.1).
10. **Inventaire.** Il est exact. → Passer G-18 (infobulles) en vague 2, ou justifier son rang ;
    ajouter le point 1 à la vague 1.
11. **[Mineur] §10, preuves manquantes :**
    - un test unitaire de la file `BannerHost` / `ToastHost` (L-8, L-9) ;
    - un e2e `light-dark()` sous WebKitGTK (T-2) ;
    - un test 2.4.11 : l'infobulle ne couvre pas le focus.

**Avis sur les questions ouvertes,** convergent avec Codex :

| Question | Avis |
|---|---|
| Q1 | Ctrl+K reste le lien. Palette sur Ctrl+Maj+P, avec l'entrée de menu « Rechercher une commande… » ; amender `ux-principles` explicitement. |
| Q2 | Le nom du projet. |
| Q3 | Un toast, avec une annulation durable et « Laisser revenir » sur la ligne. |
| Q4 | Le choix à l'accueil, et un créneau libre par défaut. Le créneau du projet fait foi ; la préférence locale n'est qu'un souhait. |
| Q5 | Le tiroir, de façon cohérente avec B10. |
| Q6 | Oui, avec la variante pour fond sombre. |
| Q7 | fr et en, avec des propriétés logiques ; pas de RTL pour l'instant. |

revise
