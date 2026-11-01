# Revue Codex n°3 (tour 3) de la spécification, révision 3 (2026-11-01) — verdict : revise

- C3 PARTIAL · Critique · [spec §7.2, §10.6](../spec.md:777) : 63 pairs peuvent renouveler leurs objets de 4 Mio sous leur part, à ≥16 Kio/s, sans laisser 8 Mio libres ; la FIFO ne garantit donc pas l’admission de l’honnête. Réserver progressivement la capacité au premier attendant.
- C4 CLOSED · [ADR Migration, étape 6](../../../docs/adr/0034-protocole-unifie.md) : retrait du Reset et installation de L/H réunis derrière `sync2`.
- C5 PARTIAL · Critique · [spec §5.1, §10.7](../spec.md:545) : LARGE_PULL corrige le LARGE non réservé, mais `CancelRead` ne restitue pas immédiatement le crédit : [quic-go](module cache: github.com/quic-go/quic-go@v0.64.1/receive_stream.go:137) attend FIN/RESET et sa taille finale ; plusieurs générations peuvent retenir du crédit.
- C5, suite · Mineur · §5.1 : [libp2p](module cache: github.com/libp2p/go-libp2p@v0.51.0/p2p/transport/quicreuse/config.go:12) fixe des **maximums** 10/15 Mio ; les fenêtres [initiales](module cache: github.com/quic-go/quic-go@v0.64.1/internal/protocol/params.go:19) sont 512/768 Kio. Préciser quelle fenêtre P-QUIC vérifie.
- C7 CLOSED · [spec §8.1](../spec.md:889) : exception explicite pour le dernier SLICE raccourci.
- C9 CLOSED · [spec §9.2.1 et générateur](../vectors/generate-repo.cjs.txt:73) : doublons comparés après décodage ; les 46 vecteurs REPO sont régénérés identiquement, sans écriture.
- C13 CLOSED · [spec §6.1](../spec.md:704) : un nouveau digest/have conserve le curseur actif.
- Handover PARTIAL · Mineur · [spec §5.1, A.1](../spec.md:1689) : limites ×2 et époque des Message corrigées ; les crédits restent indexés par génération seulement, et PeerDown n’a pas de session. Spécifier le rejet des Credit/PeerDown tardifs de l’ancienne connexion.
- Nouveau · Critique · [spec §5.5, §9.6](../spec.md:1084) : H ne répare pas une branche concurrente manquante si seules les modifications « qui descendent de have » sont retournées ; demander tout changement hors du passé causal de `have`. Prévoir aussi une réponse terminale vide.
- Nouveau · Mineur · [spec §7.4, §16.2](../spec.md:872) : le remplacement par document ne définit ni la régénération du message après effacement du sync-state, ni l’admission d’un seul LARGE >16 Mio dans la file plafonnée à 16 Mio ; ajouter ces transitions.
- Nouveau · Mineur · [spec §6.2, §17](../spec.md:731) : conservation de plusieurs attestations pour 256 têtes contredit « ne compte que si elle est la plus récente » ; définir sélection, éviction et acceptation des attestations anciennes nécessaires à H.
- Nouveau · Mineur · §9.2–§21 : le [parseur des vecteurs](../vectors/generate-repo.cjs.txt:166) accepte des clés supplémentaires autour de `__presence` et ne cherche pas les clés critiques dans son CBOR interne, contrairement au [CDDL fermé](../cddl/repo.cddl:78) ; ajouter les négatifs correspondants.
- Portes · Mineur · [ADR Confirmation](../../../docs/adr/0034-protocole-unifie.md) : conserver toutes les portes, dont P-COMPAT ; étendre [ConnCredit](../tla/ConnCredit.tla) au délai FIN/RESET, car `Switch` libère artificiellement `old` immédiatement. RepoGen reste mono-session ; objets proposés sans codec et admission déclarative restent à valider avant bascule.
revise

