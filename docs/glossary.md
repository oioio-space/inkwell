# Glossaire

Règle du document : **« pair » dans le code, « personne » dans l'interface**. Aucun mot
technique dans l'interface par défaut ; les termes techniques vivent derrière
l'interrupteur « Détails techniques ».

| Interface (fr) | Interface (en) | Code / docs | Ne jamais dire dans l'UI |
|---|---|---|---|
| personne | person | `Peer` (une machine), `Person` (groupe de machines) | pair, nœud, peer ID |
| projet | project | `Project` (un dossier) | dossier, dépôt, repo |
| modification | change | `Change` (lot d'updates Automerge signé) | commit, opération, CRDT |
| version nommée | named version | `Milestone` (jalon go-git) | commit, tag |
| jalon | milestone | `Milestone` | commit |
| divergence | divergence | `Divergence` (modifications concurrentes sur une même zone) | conflit, merge |
| brouillon (v2) | draft | `Branch` (clone Automerge) | branche, fork |
| synchronisé | in sync | `Heads` identiques | heads, vecteur d'état |
| hors ligne | offline | — | partition |
| empreinte | fingerprint | 4 mots dérivés de la clé Ed25519 | clé publique, hash |
| relais | relay | relais libp2p circuit v2 | hole punching, NAT |
| écriture / lecture / admin | write / read / admin | `RoleWriter`, `RoleReader`, `RoleAdmin` | ACL, capacité |

Noms d'interface figés : panneaux **Fichiers**, **Activité**, **Historique**,
**Personnes** ; boutons **Garder** / **Annuler**, **Enregistrer une version**,
**Afficher les modifications**, **Me rendre invisible**.
