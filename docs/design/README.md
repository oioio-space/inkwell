# Le design d'Inkwell, en clair

**Statut : acceptée (rév. 7)** par les deux relecteurs, le 2026-11-01. Les choix qui restent à
l'utilisatrice sont dans [`decisions-utilisatrice.md`](decisions-utilisatrice.md).

Ce dossier décrit à quoi ressemble Inkwell et comment il se comporte. Il s'adresse à toute
personne qui touche à l'interface : développement, revue, tests.

| Fichier | Contenu |
|---|---|
| `README.md` | Ce texte : l'intention, en mots simples. |
| [`spec.md`](spec.md) | La règle : jetons, composants, écrans, accessibilité, cohérence. Normatif (DOIT, DEVRAIT, PEUT). |
| [`inventory.md`](inventory.md) | L'interface actuelle, relevée dans le code, et ses écarts avec la règle, dans l'ordre où les corriger. |
| [`tokens.json`](tokens.json) | Les jetons de design, lisibles par une machine. |
| [`coverage.md`](coverage.md) | La grille de contrôle : Nielsen, WCAG 2.2 A et AA, motifs APG, GNOME HIG et libadwaita, et tous les domaines de l'expérience. |
| [`decisions-utilisatrice.md`](decisions-utilisatrice.md) | Les décisions qui restent à l'utilisatrice, en clair. |
| [`CHANGES.md`](CHANGES.md) | Ce que la révision 2 a changé, constat par constat. |
| [`reviews/`](reviews/) | Les revues archivées. |

Ce dossier précise [`../ux-principles.md`](../ux-principles.md) et le [glossaire](../glossary.md),
il ne les remplace pas. En cas de désaccord, ce sont eux qui gagnent, jusqu'à ce qu'une ADR
tranche.

## Pour qui

Pour des personnes qui écrivent ensemble et ne sont pas informaticiennes : une équipe qui
rédige une procédure, une association qui prépare un rapport, une enseignante et ses élèves.
Elles connaissent un traitement de texte et un navigateur. Elles ne connaissent pas Git, ni
Typst, ni le mot « pair ».

## Ce que la personne doit ressentir

- **« Je sais où je suis. »** Une fenêtre, trois zones : les fichiers, le texte, l'aperçu. Les
  mêmes boutons au même endroit, partout.
- **« Rien ne se perd. »** Tout s'enregistre, tout se rattrape. On annule plutôt que de
  demander « Êtes-vous sûr ? ». Hors ligne, le texte reste là, et le dit.
- **« Je vois qui est là. »** Une pastille, un nom et une couleur par personne. Le nom est
  toujours écrit : la couleur seule ne dit jamais rien.
- **« On me dit quoi faire. »** Chaque message tient en une phrase et propose une action. Jamais
  un code d'erreur seul.
- **« C'est calme. »** Peu de chrome, pas d'animation gratuite, pas de couleur criarde. Le
  texte d'abord.

## Personnalité

Inkwell est un **carnet partagé posé dans une pièce calme**. Il est sobre, précis et
bienveillant. Il parle comme une collègue attentive : il vouvoie, va droit au but, ne
s'excuse pas à chaque ligne, ne crie pas et ne fait pas de blagues.

Visuellement, il est **natif de GNOME** : l'apparence Adwaita (libadwaita 1.8), ses
couleurs, ses barres d'en-tête, ses bandeaux, ses toasts et ses popovers. Ce n'est pas un
effet de mode. C'est la promesse qu'une personne habituée à son bureau Linux s'y retrouve
sans apprendre, et que l'application suit le thème clair ou sombre du système.

## Les sept principes, en une ligne chacun

1. **Le texte d'abord** : le chrome s'efface ; une seule barre d'en-tête, des panneaux qui se
   replient.
2. **Un seul langage** : un même composant pour un même besoin (un seul dialogue, un seul
   toast, un seul bandeau), avec les mêmes mots et les mêmes raccourcis.
3. **Réversible plutôt que confirmé** : un toast « Annuler » plutôt qu'une boîte « Êtes-vous
   sûr ? ».
4. **Chaque message dit quoi faire** : ce qui se passe, ce qui est préservé, ce qu'on peut
   faire.
5. **Zéro jargon** : on dit « personne », « version », « divergence », jamais « pair »,
   « commit » ou « conflit ».
6. **Pour tout le monde** : clavier complet, lecteur d'écran, contraste AA, mouvement réduit,
   grandes polices.
7. **Mesuré, pas supposé** : chaque règle a sa preuve (un test, une capture, une revue).

## Révision 6, en bref

- **Raccourcis écrits pour chaque système** :
  - jamais Ctrl+Alt (c'est AltGr sous Windows) ;
  - rien de ce que macOS, Windows ou GNOME se réservent ;
  - Cmd sur Mac, avec ses conventions (⌥⌘F pour remplacer, ⌘P pour imprimer).
- **Le secours est simple** : une phrase par personne, une fois ; le kit est mis à jour en silence ;
  on peut récupérer avec la phrase seule.
- **Relire une proposition** : on choisit morceau par morceau, puis on applique **en une fois**.
- **L'historique** se parcourt au clavier, et aucun mot de Git n'apparaît. Une variante couvre
  toujours tout le projet.
- **161 paires de contraste**, dont le texte secondaire sur les survols, qui a reçu son propre
  jeton.

## Les grandes décisions (révision 5)

- **La carte calme** (passe de cohérence, `architecture/coherence.md`). À l'ouverture : Fichiers,
  le texte, l'aperçu, aucun tiroir.
  - **À gauche**, deux onglets : Fichiers et Sommaire.
  - **À droite**, trois destinations : Rechercher ; **Relire** (Problèmes, Commentaires,
    Activité, Versions) ; IA, si elle est activée.
  - La **barre d'état** a quatre éléments, avec **un seul compteur** de problèmes.
  - **Ressources** est à part.
- **Revenir en arrière sans Git :**
  - une frise en mots ;
  - le passé se regarde en lecture seule ;
  - **restaurer crée une nouvelle version**, et rien n'est perdu ;
  - comparer par mots ;
  - **variantes** et « Proposer d'intégrer », relues par morceau, avec les passages des deux
    côtés côte à côte.
- **Protéger, sauvegarder, transmettre**, quatre mots distincts (version, sauvegarde, archive,
  modèle) :
  - protection cochée par défaut pour un projet partagé ;
  - phrase et kit de secours vérifiés ;
  - restauration à côté, sans rien écraser.
- **Ne pas synchroniser** : un geste et un nuage barré ; la personne, ou l'admin pour tout le
  projet ; l'aperçu des fichiers concernés.
- **Une seule façon de relire une proposition** (extension, IA, application, variante) : par
  morceau.
- **États honnêtes** : on dit « enregistré », « envoyé » ou « reçu », jamais une promesse de plus.
- **Partout le même Adwaita**, avec les gestes de chaque système (Cmd sur macOS). Aucun format
  Word ni ODT.
- **Jetons :** 131 paires vérifiées, états interactifs et frise compris ; contrat de thème.
- Les décisions non tranchées et les désaccords sont dans `spec.md` §11 et `CHANGES.md`.

## Comment s'en servir

- **Avant de créer un écran :** lire `spec.md` §4 (composants) et §5 (écrans). Réutiliser ce
  qui existe. Si rien ne convient, compléter la spec d'abord.
- **Avant une revue d'interface :** appliquer la liste de `spec.md` §8 (cohérence) et §9
  (anti-motifs), puis le skill `ux-walkthrough`.
- **Pour changer un jeton :** modifier `tokens.json` et `frontend/src/theme/tokens.css`
  ensemble. Le test de parité qui le vérifiera est à écrire (`spec.md` T-5, inventaire G-1b).
