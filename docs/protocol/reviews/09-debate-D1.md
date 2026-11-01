# Débat D1 entre les relecteurs (tour 4, 2026-11-01) — accord

**Question.** Admission d'une modification indivisible plus grande que la part d'un pair (D1, `CHANGES.md` « Tour 3 »).

**Position de Codex.** Garder les pages d'historique dont le receveur fixe la taille (≤ part) ; pour une modification
indivisible plus grande, des tranches Bao reprenables et vérifiées (§8.1), liées au document, à l'auteur, à
l'empreinte et à la taille attendues ; réservation disque protégée imputée au quota de l'auteur (N10) ; service
équitable ; ni voie minutée ni réinitialisation sur la lenteur. Objection à la voie : toute préemption par quantum
fait perdre l'objet entier à un transfert honnête plus lent que le quantum ; sous contention durable il ne finit
jamais (§10.2, ADR 0028).

**Position de Fable, puis ralliement.** Fable défendait la voie séparée (bit de voie, verdict « en file », quantum,
préemption en refus). Après lecture de la position de Codex : « Codex's objection is correct and decisive ».
Les tranches suppriment le bit de voie, le verdict 2, le quantum et la réservation FIFO ; l'annonce LARGE porte
`{id, size, documentId, indivisible, hash}` ; le receveur tire par empreinte chez l'annonceur ou tout détenteur
(Bao vérifie contre la racine) ; l'enveloppe CBOR est faite localement, ce qui supprime le dépassement de 16 Mio ;
le reste en attente est persisté et repris comme les têtes d'historique ; trois échecs → autre détenteur, puis règle H.

**Accord.** « C (pages ≤ r) + tranches Bao reprenables via §8.1 pour les modifications indivisibles > r, tirées par
empreinte depuis l'annonce LARGE, mises en attente sur disque par auteur sous le quota N10, ordonnancées en DRR,
sans quantum, sans voie, sans réinitialisation sur la lenteur. »
