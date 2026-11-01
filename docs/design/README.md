# Le design d'Inkwell, en clair

Ce dossier décrit à quoi ressemble Inkwell et comment il se comporte. Il s'adresse à toute
personne qui touche à l'interface : développement, revue, tests.

| Fichier | Contenu |
|---|---|
| `README.md` | Ce texte : l'intention, en mots simples. |
| [`spec.md`](spec.md) | La règle : jetons, composants, écrans, accessibilité, cohérence. Normatif (DOIT, DEVRAIT, PEUT). |
| [`inventory.md`](inventory.md) | L'interface actuelle, relevée dans le code, et ses écarts avec la règle, dans l'ordre où les corriger. |
| [`tokens.json`](tokens.json) | Les jetons de design, lisibles par une machine. |

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

## Les grandes décisions de cette version

- **Les jetons libadwaita 1.8 restent la base.** On corrige quatre valeurs qui ne passent pas
  WCAG 2.2 AA : l'anneau de focus, la couleur « succès », deux marques de fichiers. On ajoute
  une variante « trait » des couleurs de personnes pour les curseurs sur fond clair.
- **Une échelle typographique de six tailles** remplace les treize tailles relevées dans le
  code.
- **Un seul composant `Dialog`** remplace sept dialogues recopiés.
- **Au plus un bandeau visible à la fois**, choisi par priorité, comme `AdwBanner`. Un toast
  ne pose jamais de question.
- **Une palette de commandes (Ctrl+K)** est prévue par les principes, mais Ctrl+K sert
  aujourd'hui aux liens : c'est une question ouverte (voir `spec.md` §11).

## Comment s'en servir

- **Avant de créer un écran :** lire `spec.md` §4 (composants) et §5 (écrans). Réutiliser ce
  qui existe. Si rien ne convient, compléter la spec d'abord.
- **Avant une revue d'interface :** appliquer la liste de `spec.md` §8 (cohérence) et §9
  (anti-motifs), puis le skill `ux-walkthrough`.
- **Pour changer un jeton :** modifier `tokens.json` et `frontend/src/theme/tokens.css`
  ensemble ; le test de parité le vérifie (voir `spec.md` §2.1).
