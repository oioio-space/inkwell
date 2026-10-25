---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Contrôle d'écriture par têtes signées

## Contexte et problème

Seule la webview sait lire Automerge (ADR 0002) ; seule Go garde la clé privée et connaît la
liste de membres (ADR 0005, 0017). Une modification forgée par un lecteur — ou par un ancien
écrivain — ne doit jamais atteindre le `Repo`, et Go ne peut pas la reconnaître : pour lui, un
message de synchro est une suite d'octets opaques.

## Options envisagées

- Go déchiffre Automerge : exclu (ADR 0011, aucun automerge en Go).
- Signer chaque modification : le format Automerge n'a pas de place pour une signature, et le
  coût de signature serait celui de la frappe.
- **Signer les têtes** : un écrivain atteste « voici les têtes de mon document », la webview
  vérifie que toute modification reçue descend d'une tête attestée.

## Décision

- Trois kinds du pont (`bridge.Kind`, sur-ensemble de `transport.Kind`, version 1 inchangée) :
  `Attest` (webview → Go : `doc | têtes`), `Attested` (Go → webview : `peer` = signataire),
  `Revoked` (Go → webview). Un quatrième, `Sealed`, signale la liste saturée.
- **Émission.** Avant un message de synchro dont les têtes diffèrent de la dernière attestation du
  document, l'adaptateur envoie `Attest` **sur le même Stream**. Go signe avec la clé de
  l'appareil (qui ne sort jamais), diffuse sur le canal 3, puis relaie le message repo : l'ordre
  des canaux d'un même flux garantit qu'une attestation précède les modifications qu'elle couvre.
  Go refuse de signer pour un lecteur, un ancien membre, et quand la liste est scellée.
- **Réception** (`frontend/src/net/coverage.ts`, avant `emit("message")`). Une modification est
  couverte si elle est une tête attestée par un écrivain courant ou l'un de ses ancêtres (parcours
  des `deps`). Les modifications non couvertes sont retirées et le message est réencodé. Une
  entrée de synchro peut concaténer plusieurs modifications, ou un document complet : un document
  complet dont une modification n'est pas couverte fait jeter le message entier. Les messages
  éphémères (curseurs, présence) ne sont pas des modifications : les lecteurs peuvent en envoyer.
- **Révocation.** Les attestations du révoqué sont oubliées (`Revoked`) ; chaque écrivain
  rattestera ses têtes, ce qui couvre l'historique déjà accepté. Les modifications hors ligne
  d'un révoqué que personne n'a acceptées restent non couvertes, donc refusées. Cela tranche la
  question ouverte « updates hors ligne d'un auteur révoqué ».
- **Liste scellée.** Go cesse de signer (`ErrSealed`) et envoie `Sealed` : l'interface dit
  « liste de membres saturée — créez un nouveau projet ».
- **Boucle de synchro.** Une tête que l'on refuse est retirée des têtes du message réencodé, et
  un message déjà coupé qui se répète à l'identique n'est pas livré : sans cela, le pair honnête
  répond à chaque message d'un forgeron par une demande de la modification refusée, à l'infini.

### Conséquences

- Bien : la clé reste en Go ; le `Repo` ne voit que des modifications couvertes ; le coût est
  une signature par changement de têtes, pas par frappe.
- Moins bien, assumé :
  - la couverture est celle des têtes **attestées** : un écrivain atteste tout ce que contient son
    document, y compris ce qu'il a accepté d'un autre — on fait confiance aux écrivains courants ;
  - une attestation de plus de 64 têtes n'est pas possible : ces modifications ne sont pas
    couvertes ;
  - le graphe de couverture est gardé en mémoire (une entrée par modification couverte) ; la
    vérification d'un document complet coûte O(document) à la première synchro ;
  - **transitoire (tâche 8)** : tant que le ticket ne porte pas les opérations du projet, un invité
    adopte le `ProjectID` écrit dans le texte d'invitation (`Wiring.Adopt`) et, jusqu'à la
    première opération reçue, tient l'hôte pour l'admin (`List.VerifyTrusting`). Un appareil qui
    partage déjà son projet refuse d'en adopter un autre.

### Confirmation

Go : vecteurs dorés des kinds (`internal/bridge/testdata/envelopes.json`, partagés avec TypeScript),
`FuzzDecode`, `FuzzDecodeHeads`, `TestReaderCannotAttest`, `TestSealedStopsSigning`,
`TestAdoptJoinsHostsProject`. TypeScript : scénarios nommés de simulation à 10 graines (lecteur
qui forge, fragment de document complet couvert ou non, écrivain révoqué hors ligne, nouveau pair
après révocation) et e2e `collab.spec.ts`.

## Plus d'informations

[Plan de l'étape 2, tâche 10](../plans/2026-10-25-etape-2-reseau.md) · `internal/trust/attest.go`.
