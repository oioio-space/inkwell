# Parcours UX : S11, présenter (2026-10-26)

Branche testée : `origin/t11-c4-present` (c8d4e4e). Corrections : branche `t11-c4-ux`.
Méthode : skill `ux-walkthrough`, parcours d'abord à l'écran (deux ou trois instances réelles,
Chromium, `bin/inkwell-server`), code lu ensuite. Captures regardées une à une.

## Bloquants (sévérité 3 et 4) : tous corrigés sur `t11-c4-ux`

| Id | Sév. | Défaut | Test e2e |
|----|------|--------|----------|
| UX-S11-1 | 4 | Pendant une présentation lancée avec F5 depuis l'éditeur, le curseur reste dans le texte caché derrière : toute touche sans sens pour les diapositives (x, y, Suppr, Ctrl+A puis Suppr…) modifie le document partagé sans rien montrer, pour toutes les personnes. | `frontend/e2e/ux/s11.spec.ts` |
| UX-S11-2 | 3 | Tab sort de la présentation et atterrit sur les boutons de l'éditeur cachés derrière (Partager, Gras…), sans aucun repère visible ; aucun piège de focus, axe signale aussi l'éditeur caché (contraste des numéros de ligne en sombre). | idem |
| UX-S11-3 | 3 | À 200 % de zoom (550 px) et à 360 px, l'en-tête du présentateur et du suiveur se chevauche : « Vous présentez » / « Alice présente » passe sur le chronomètre, « Quitter la présentation » passe sur 2 lignes. Lectrice à faible vision (Léa) touchée. | idem |

Correctif : `frontend/src/App.svelte` (`inert` sur la fenêtre pendant une présentation, donc ni
touches ni focus), `frontend/src/present/PresenterView.svelte` (le focus entre dans la vue ; en
dessous de 760 px le titre se coupe avec « … », l'heure disparaît, « Quitter » ne garde que la
flèche, son nom reste pour les lecteurs d'écran). Rejoué : les 3 tests passent, `present.spec.ts`
(4 tests) passe, le focus revient dans l'éditeur à la sortie.

## Parcours

### Alice présente sur un PC (vue public)
1. F5 (ou bouton « Présenter ») : vue présentateur en 1,5 s ; diapositive, suivante, notes,
   chronomètre, heure, pied avec les raccourcis. Clair.
2. Bouton « Ouvrir la vue public » : une seconde fenêtre s'ouvre, la diapositive seule, noir
   autour, curseur masqué. Bouton « plein écran » visible seulement quand la souris bouge :
   F11 et bouton passent bien en plein écran (vérifié avec `document.fullscreenElement`).
3. Échap dans la vue public : ferme la fenêtre et quitte la présentation. Le premier Échap sorti
   du plein écran ne quitte pas (garde de 400 ms). Non vérifiable en navigateur sans fenêtre native :
   le vrai Échap qui quitte le plein écran, et le plein écran dans WebKitGTK (voir « Non vérifié »).
4. Touches B (écran noir), L (laser), flèches, chiffres puis Entrée : répondent en moins de 100 ms.

### Paul pilote depuis une seconde instance
1. Bandeau « Alice présente. [Suivre] » sous la barre : trouvé tout de suite.
2. « Suivre » : vue suiveur, titre « Alice présente », bandeau « Prendre la main », choix
   « Suivre / Parcourir librement ». Les diapositives avancent avec Alice.
3. « Parcourir librement » : Paul avance seul ; « Suivre » devient « Revenir au présentateur ».
4. « Prendre la main » : « Demande envoyée… » chez Paul, « Bob demande la main [Refuser]
   [Accorder] » chez Alice. Accorder : Paul pilote (« Vous présentez »), Alice devient suiveuse
   (« Bob présente »), la vue public d'Alice suit Paul. Le flux consentement est clair.
5. Alice fermant la vue public avec Échap pendant que Paul pilote : elle sort de la présentation
   et retrouve le bandeau « Bob présente. [Suivre] ». Cohérent.

### Léa, lectrice, suit
Invitée en lecteur (appel direct de `Session.Invite("reader")`, l'interface n'offre pas ce choix à
cette étape). Bandeau « Alice présente. Suivre », puis « Vous pouvez suivre la présentation, mais
pas la piloter. », pas de bouton « Prendre la main », laser grisé. Notes visibles. Pas de
dépassement horizontal à 200 % ni à 360 px (hors en-tête, corrigé).

### Clair / sombre, accessibilité
Sombre : présentateur, suiveur, demande de main et vue public lisibles, mêmes composants que le
reste (bandeaux, boutons segmentés, cartes Adwaita). axe (WCAG A/AA) : vue public sans
violation ; présentateur et suiveur : seulement l'éditeur caché (corrigé par UX-S11-2).
`forced-colors` : tout reste lisible, le laser devient vert.

### Cohérence avec les écrans voisins (U31)
Mêmes bandeaux (`Banner`), boutons `fb`/`btn`/`sug`, icônes et jetons ; « Échap », « F5 »
comme ailleurs. Écarts : icône cible du laser = icône « Aller à l'endroit du curseur » de
l'aperçu (UX-S11-9) ; libellé du bouton segmenté qui change (UX-S11-6) ; « vue public » (UX-S11-4).

## Défauts restants (sévérité ≤ 2, à corriger dans la vague)

| Id | Sév. | Défaut et attendu |
|----|------|-------------------|
| UX-S11-4 | 2 | « vue public » est fautif : « vue publique » (bouton, infobulle, fr.json `present.showPublic`, `present.public.*`). |
| UX-S11-5 | 2 | Pied de page « → ← espace : avancer et reculer · B · W · L » affiché aussi à une personne qui suit ou lit : ses flèches ne font rien, sans message. Attendu : pied adapté (« Parcourir librement pour avancer seul ») ou flèches qui proposent de passer en libre. |
| UX-S11-6 | 2 | Le bouton « Suivre » devient « Revenir au présentateur » : même bouton, deux noms ; et en mode libre rien n'indique où est le présentateur (« Alice est à la diapositive 2 »). |
| UX-S11-7 | 2 | Écran noir (B) : le présentateur voit un grand carré noir sans mot ; attendu « Écran noir : B pour reprendre ». Idem écran blanc. |
| UX-S11-8 | 2 | À 200 % (550 x 320) et 360 px la diapositive courante se réduit à une bande et les notes sont coupées. Attendu : zone qui défile, ou diapositive prioritaire. |
| UX-S11-9 | 1 | Laser : même icône que « aller à l'endroit du curseur » ; l'icône et le raccourci L ne sont dits qu'en infobulle. |
| UX-S11-10 | 1 | Chronomètre : après « Accorder », Alice suiveuse voit encore le sien tourner, Paul démarre à 0:00 ; le chronomètre appartient au pilote mais ne le suit pas. Deux boutons du chronomètre et le laser (grisé) restent affichés au suiveur et au lecteur. |
| UX-S11-11 | 1 | Vue public : aucune aide visible pour F11 hors de l'infobulle du bouton de la fenêtre principale ; au repos la fenêtre ne dit rien d'autre que la diapositive (voulu, PRS-40). Une indication brève à l'ouverture (« F11 : plein écran · Échap : quitter ») s'effacerait avant la projection. |
| UX-S11-12 | 1 | Le texte « Aucune note pour cette diapositive » en grande taille occupe la carte entière ; discret, à réduire. |

## Non vérifié (honnêteté)

- Plein écran réel dans la fenêtre native Wails (WebKitGTK), placement sur un second écran, et
  vrai Échap du navigateur qui sort du plein écran : à rejouer avec `mise run app:duo` par une
  personne.
- Une troisième instance (Léa en plus de Paul) ne reçoit pas le texte d'Alice dans ce banc
  (2 personnes affichées, document vide, avatar « ? » chez Alice) : constaté deux fois, rôle
  lecteur et écrivain ; c'est un défaut d'invitation à trois (S2/S7) ou du banc, hors S11, à
  ouvrir séparément. Léa a donc été jouée avec la seconde instance, en lecteur.
- Dernier coup d'oeil sur l'invitation : la boîte « Écrire à plusieurs » reste ouverte avec
  le texte collé et « Rejoindre » grisé après une jonction réussie (S2).
