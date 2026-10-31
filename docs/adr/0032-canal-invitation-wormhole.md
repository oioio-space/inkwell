---
status: proposée
date: 2026-10-31
decision-makers: [Mathieu]
---

# Canal d’invitation Wormhole séparé de Transport

## Contexte et problème

L’ADR 0004 (`docs/adr/0004-network-stack-libp2p.md:23-25`, avant amendement)
et `CLAUDE.md:45` plaçaient tout le réseau derrière `Transport`. Pourtant,
`internal/invite/wormhole.go:49,78` appelle directement `SendText` et `Receive`.
`docs/architecture.md:76` (au moment de l’audit) et `scripts/arch-check.sh`
(règle d’import wormhole-william) autorisent ce choix. C’est ARC-C2.

Wormhole transmet un texte pour trouver l’hôte, pas les modifications du projet.
Le consommateur le voit par `carrier` (`internal/app/invites.go:39-43`).
`invite.Lobby` (`internal/invite/flow.go:439-443`) est une autre frontière :
elle sert à l’admission sur libp2p après réception du texte.

## Facteurs de décision

- Garder une frontière simple entre rendez-vous, admission et synchronisation.
- Décrire les garanties réelles sans changer le code.
- Borner l’exposition d’un canal accessible avant de devenir membre.

## Options envisagées

- **A. Garder Wormhole séparé dans `internal/invite`.** Conserve le code court et
  l’échange unique ; exige un modèle de menace distinct, décrit ci-dessous.
- **B. Porter les invitations sur `Transport`/libp2p.** Unifie le transport et ses
  tests, mais impose de trouver et joindre l’hôte avant de recevoir son adresse.
  Reproduire le code court demanderait un mécanisme de rendez-vous supplémentaire.
- **C. Déplacer Wormhole dans `internal/transport/wormhole`.** Rend le réseau plus
  visible dans l’arbre, mais ne lui donne ni flux continu ni identité `PeerID`.
  Le risque reste le même et une interface distincte reste nécessaire.

## Décision

**Option A recommandée, en attente d’acceptation.** Cette ADR amenderait l’ADR 0004
sur le canal d’invitation seulement. Aucun changement de code n’est proposé.

### Invariant exact proposé

La synchronisation entre pairs passe par `internal/transport.Transport`. Le rendez-vous d'invitation Wormhole est un canal séparé, limité à un texte d'invitation et à `internal/invite` ; l'admission passe ensuite par `invite.Lobby` sur libp2p. Seul `internal/transport/p2p` importe libp2p ; seul `internal/invite` importe wormhole-william.

### Modèle de menace du rendez-vous

- **Code court et PAKE.** Le code partagé permet d’établir une clé par SPAKE2,
  puis de chiffrer de bout en bout. Le protocole limite une attaque active à une
  tentative par code à usage unique ; ce n’est pas une preuve d’identité civile.
  Un code divulgué donne accès à son détenteur : le transmettre en privé et en
  créer un nouveau après un échec suspect ([documentation Magic-Wormhole](https://magic-wormhole.readthedocs.io/en/latest/welcome.html#safely)).
- **Usage unique.** Distinguer le code Wormhole à usage unique du ticket Inkwell :
  l’hôte retire ce dernier à sa consommation et refuse les tickets expirés
  (`internal/invite/flow.go:317-350`). Recevoir le texte ne rend pas membre.
- **Délais.** Le ticket expire après 15 minutes (`internal/invite/ticket.go:36-37`).
  L’offre prend cette échéance ; obtenir le code est limité à 8 secondes et le
  récupérer à 30 secondes (`internal/app/invites.go:25-30,242-280`). `Wormhole`
  dépend du contexte fourni ; `Fetch` ne vérifie pas lui-même l’expiration.
- **Tailles.** Code limité à 64 octets, texte à 1024 octets ; lecture limitée à
  `MaxTextLen+1`, puis analyse stricte (`internal/invite/wormhole.go:18-30,86-90` ;
  `internal/invite/ticket.go:38-39,134`). Ces bornes applicatives ne prouvent pas
  une borne sur toutes les allocations internes de la bibliothèque.
- **Serveur de rendez-vous.** Il reste nécessaire et peut bloquer l’échange ou
  observer les connexions. PAKE ne garantit pas la disponibilité. Le README de
  [wormhole-william](https://github.com/psanford/wormhole-william#readme) distingue
  le texte sur Wormhole des fichiers sur le protocole transit. Inkwell n’utilise
  que le texte (`wormhole.go:35-38,82-85`), sans relais de fichiers transit.

### Conséquences

- Bien : la documentation décrit le code existant ; la synchronisation garde son
  interface et ses tests, le rendez-vous garde ses limites propres.
- Moins bien : deux canaux réseau à examiner et une dépendance au serveur de
  rendez-vous. L’isolation des imports ne vaut pas audit de la bibliothèque.
- Les formulations de `CLAUDE.md` et d’`architecture.md` restent marquées comme
  propositions jusqu’à l’acceptation. ARC-m11 est corrigé pour les seuls écarts
  documentaires ; aucun contrôle supplémentaire n’est ajouté au script.

### Confirmation

Revue de l’invariant identique dans les trois documents et de ses frontières.
Le contrôle existant `scripts/arch-check.sh` limite les imports libp2p et
wormhole-william ; les tests existants d’invitation couvrent consommation,
expiration et refus. Réexaminer cette décision si le canal transporte autre
chose qu’un texte d’invitation ou si ses limites changent.

## Plus d’informations

Sources amont consultées le 31 octobre 2026 : documentation Magic-Wormhole et
README wormhole-william, liés ci-dessus. Constats : ARC-C2 et ARC-m11 dans
`docs/plans/audit-2026-10-31/01-architect-report.md`.
