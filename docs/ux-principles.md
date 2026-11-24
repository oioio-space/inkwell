# Principes UX/UI — la beauté comme exigence

Le critère de réussite n'est pas la liste des fonctions mais **le temps qu'il faut à
quelqu'un pour réussir cinq tâches** (créer, partager, relire, présenter, exporter),
mesuré en tests utilisateurs à chaque étape.

## Les dix règles (document de conception)

1. **Une fenêtre, trois zones** : Fichiers à gauche, éditeur au centre, aperçu à droite ;
   chaque zone se replie d'un geste et se souvient de son état par projet.
2. **Le texte d'abord** : barre d'en-tête Adwaita (titre, état de synchro, avatars) ; les barres
   présentes et la barre de mise en forme (fixe, repliable, choix de l'utilisatrice) sont fixées par
   la [spec de design](design/spec.md), principe P1 et §3 (« La barre de mise en forme suit le choix de la
   personne ») ; les panneaux secondaires partagent un
   tiroir à onglets à droite.
3. **Trois accès à tout** : clic, raccourci, palette (Ctrl/Cmd+Maj+P, et menu › « Rechercher une commande… ») qui affiche le
   raccourci ; Ctrl/Cmd+K reste « lien » (décision de cohérence C14, 2026-11-01).
4. **Zéro jargon** : voir [glossaire](glossary.md).
5. **Chaque message dit quoi faire** : une phrase + une action, jamais un code seul ; toute
   attente a une durée ou une progression visible.
6. **Réversible plutôt que confirmé** : toast « Annuler » quelques secondes au lieu d'une
   boîte de confirmation ; corbeille et historique rattrapent le reste.
7. **Même langage visuel que le résultat** : police de code, couleurs des blocs et encadrés
   reprises de la charte du PDF produit.
8. **Première minute soignée** : nom, couleur, projet d'exemple ouvert, aperçu qui se met à
   jour dès la première frappe ; aucune question réseau avant le premier partage.
9. **Clavier complet, lecteur d'écran, contraste** : focus visible, tailles de cible de la spec de design (A-7),
   `prefers-reduced-motion`, thème système.
10. **Rien ne quitte l'appareil sans un geste explicite** : aucune télémétrie, aucun envoi automatique ;
    les statistiques locales sont permises, sous le contrôle de la personne (voir, exporter, effacer,
    couper : `docs/plans/2026-11-24-obs1-observability.md`) ; l'interface se mesure par des tests en
    personne. Seule place de cette règle : les autres textes y renvoient.

## Système visuel

- **Adwaita en CSS** : reprendre la palette et les variables de libadwaita (≥ 1.8) plutôt
  que dépendre d'une feuille entière ; polices **Adwaita Sans** / **Adwaita Mono**
  (libres), JetBrains Mono en option (ligatures désactivables).
- **Jetons de design** en trois niveaux — primitifs → sémantiques → composants — en
  propriétés CSS. Les composants ne référencent que des jetons sémantiques, jamais une
  valeur brute.
- **Thème** : `color-scheme: light dark` + `light-dark()` ; l'OS décide par défaut,
  `data-theme` sur la racine permet de forcer. Haut contraste via media query.
- **Mouvement** : toute animation non essentielle est coupée sous
  `prefers-reduced-motion: reduce` ; les jetons de durée ont leur variante réduite.
- **Couleurs des personnes** : 8 teintes Okabe-Ito, lisibles par les daltoniens ; le nom
  est toujours visible au survol — **jamais la couleur seule comme information**.
- **Lecture longue** : style « document » (police document, interligne augmenté) pour
  l'aperçu et les panneaux de texte, comme `.document` de libadwaita 1.8.

## Preuves

Ce qui, dans ces règles, se vérifie par une machine a sa preuve dans
[engineering-principles.md](engineering-principles.md) §5 (plan : `docs/plans/2026-11-09-ui-formal.md`).

## Références assumées

libadwaita / GNOME HIG (structure), typst.app (source ↔ aperçu), Figma (présence),
Google Docs (commentaires), Keynote (présentation).
