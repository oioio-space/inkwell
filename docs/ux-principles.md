# Principes UX/UI — la beauté comme exigence

Le critère de réussite n'est pas la liste des fonctions mais **le temps qu'il faut à
quelqu'un pour réussir cinq tâches** (créer, partager, relire, présenter, exporter),
mesuré en tests utilisateurs à chaque étape.

## Les dix règles (document de conception)

1. **Une fenêtre, trois zones** : Fichiers à gauche, éditeur au centre, aperçu à droite ;
   chaque zone se replie d'un geste et se souvient de son état par projet.
2. **Le texte d'abord** : barre d'en-tête Adwaita (titre, état de synchro, avatars), c'est
   tout. La barre de formatage apparaît sur la sélection ; les panneaux secondaires
   partagent un tiroir à onglets à droite.
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
9. **Clavier complet, lecteur d'écran, contraste** : focus visible, cibles de 44 px,
   `prefers-reduced-motion`, thème système.
10. **Mesurer sans télémétrie** : tests en personne, jamais de collecte.

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

## Références assumées

libadwaita / GNOME HIG (structure), typst.app (source ↔ aperçu), Figma (présence),
Google Docs (commentaires), Keynote (présentation).
