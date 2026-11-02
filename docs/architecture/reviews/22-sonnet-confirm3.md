# Confirmation 3 (Sonnet)

Portée : fermeture de B1 et B2 de 20-sonnet-confirm2 ; cohérence entre specs.

- **B1 (sémantique de « contestée »)** : fermé. Architecture VD-2/VD-3/PD-8 : décision définitive, seconde ignorée (« décision déjà rendue — soumettre à nouveau »), `contestée` = équivocation seule (fourche de chaîne, preuve `review_equivocation_v1`). Design §4.48 reprend les mêmes termes (« uniquement » l'équivocation ; seconde décision non contestée). Ordre du statut de version identique des deux côtés (contestée > avis contraires > refusée > validée > soumise > retirée).
- **B2 (précédence PD-8, décision après retrait vu)** : fermé. Table VD-3 à précédence explicite ; décision ∥ retrait = décision gagne ; décision dont le passé contient un retrait valide = invalide ; retrait après décision = sans effet ; PD-8 renvoie à VD-2/VD-3 et fixe le cas des positions différentes. Design §4.48 : mêmes trois cas, mêmes libellés.
- **CM-6** : quota N10 = plafond local, pas prédicat d'admission ; cohérent.
- **§4.18, §4.22** : repli de présence sans PD-5 aligné sur l'architecture §6.11 ; plus de phrase « à aligner » pour ces sections (la seule restante, §4.x point 8 sur HI-5, est hors périmètre).
- Entrées « Alignement final (confirmation 2) » des deux CHANGES.md : fidèles au contenu des specs.

Aucun bloquant restant.

ACCEPT
