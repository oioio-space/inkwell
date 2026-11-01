# Glossaire

Règle du document : **« pair » dans le code, « personne » dans l'interface**. Aucun mot
technique dans l'interface par défaut ; les termes techniques vivent derrière
l'interrupteur « Détails techniques ».

| Interface (fr) | Interface (en) | Code / docs | Ne jamais dire dans l'UI |
|---|---|---|---|
| personne | person | `Peer` (une machine), `Person` (groupe de machines) | pair, nœud, peer ID |
| projet | project | `Project` (un dossier) | dossier, dépôt, repo |
| modification | change | `Change` (lot d'updates Automerge signé) | commit, opération, CRDT |
| version (nommée ou non) | version (named or not) | manifeste de têtes Automerge ; `Milestone` (projection go-git) | commit, tag, jalon, snapshot |
| divergence | divergence | `Divergence` (modifications concurrentes sur une même zone) | conflit, merge |
| variante | variant | variante Automerge (`history/graph.ts`) | branche, fork, brouillon |
| proposer d'intégrer, intégrer | propose to integrate, integrate | fusion Automerge d'une variante | fusionner, merge, pull request |
| protéger (le projet) | protect | chiffrement par objet (§6.10) | chiffrer, clé, DEK |
| phrase de secours, kit de secours | recovery phrase, recovery kit | secret de secours, fichier de reprise (ST-8) | seed, mnémonique, BIP-39 |
| sauvegarde, copie lisible, archive | backup, readable copy, archive | dorsal, export, projection | dump |
| ressources | resources | gestionnaire de tâches (§11.1) | processus, PID, CPU |
| non synchronisé | not synced | règle d'exclusion (`.inkwellignore`) | ignoré, gitignore, exclu |
| synchronisé | in sync | `Heads` identiques | heads, vecteur d'état |
| hors ligne | offline | — | partition |
| empreinte | fingerprint | 4 mots dérivés de la clé Ed25519 | clé publique, hash |
| relais | relay | relais libp2p circuit v2 | hole punching, NAT |
| écriture / lecture / admin | write / read / admin | `RoleWriter`, `RoleReader`, `RoleAdmin` | ACL, capacité |

Noms d'interface figés : panneaux **Fichiers**, **Sommaire** ; tiroir **Rechercher**,
**Relire** (Problèmes, Commentaires, Activité, Versions), **IA** ; dialogue **Personnes** ; vue
**Ressources** ; boutons **Garder** / **Annuler**, **Enregistrer une version**,
**Afficher les modifications**, **Me rendre invisible**.
