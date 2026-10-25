---
status: accepted
date: 2026-10-24
decision-makers: [Mathieu]
---

# Modèle de confiance v1 : liste de membres signée

## Contexte et problème

Sans serveur, le contrôle d'accès doit voyager avec les données. C'est le point le moins
mûr de l'écosystème local-first.

## Options envisagées

- Code de jonction = accès complet (Teamtype)
- Identité = clé, références signées, délégués (Radicle)
- Capacités convergentes (Keyhive) — pas encore livré
- UCAN (`go-ucan`) — révocation à notre charge

## Décision

1. Identité = paire Ed25519 par machine, empreinte prononçable en 4 mots.
2. Invitation = code Wormhole ou acceptation explicite d'un pair LAN.
3. Liste de membres signée, rôles `admin` / `écriture` / `lecture` ; petit CRDT à ordre
   total déterministe (Lamport, puis clé publique) ; une révocation l'emporte sur une
   invitation concurrente.
4. Chaque lot d'updates est signé par son auteur ; les pairs refusent un auteur absent ou
   en lecture seule (commentaires et curseurs restent acceptés en lecture).
5. Révocation = retrait + rotation de la clé de session.
6. Pas de chiffrement au repos en v1. Réévaluer Keyhive dès une version stable avec
   liaison JS (remplacerait 3 à 5).

### Conséquences

- Bien : simple, explicite, non répudiable.
- Moins bien : ce qu'un membre retiré a déjà reçu reste chez lui (inévitable).

### Confirmation

Scénarios nommés de la simulation : révocation concurrente, deux admins, antidatage.

## Plus d'informations

Questions ouvertes : [open-questions.md](../open-questions.md).
