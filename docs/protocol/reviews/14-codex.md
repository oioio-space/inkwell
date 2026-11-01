# Revue Codex n°7 (tour 7) de la spécification, révision 7 (2026-11-01) — verdict : revise

- PARTIAL — Majeur, index persistant des racines (§8.1, §16.1 ; N12) : validation à l'insertion, suppression et imputation ajoutées, mais la borne est fausse : `max` peut valoir 64 Kio et `page-route-envelope-overflow` indexe 70 112 octets, pas > 4 Mio. Corriger la borne et la tester (spec.md:1020).
- CLOSED — ancien Majeur, tuple/relais (§4.7, §8.1) : toute annonce non vérifiée occupe l'emplacement du pair qui la livre ; le quota N10 de l'auteur n'est crédité qu'après vérification.
- CLOSED — ancien PARTIAL, principal provisoire (§8.1) : retiré ; la borne « 1 tirage ≤ 16 Mio par pair annonceur » est énoncée sans prétendre borner par auteur réel.
- NOUVEAU — Majeur, disque (§8.1, §14.7, §16.1 ; N7) : 64 pairs actifs ne bornent pas les annonceurs persistés après leur déconnexion ; 65 membres successifs à près de 16 Mio dépassent 1 Gio. Définir et tester un plafond global incluant les pairs absents (spec.md:1025).
- OK — §8.1, §9.6 : H reste disponible auprès des autres détenteurs ; même racine = détenteur ajouté, pas de second transfert ; fermeture causale des pages explicite pour deux en vol.
- OK — §7.2(5) : 60 s sans progrès libère aussi la réservation d'un LARGE d'ancienne génération, sans règle L.
- Mineur — §21, rejeu : `indivisible-tuple-false-root` tronque la modification ; Automerge renvoie « not enough data » au lieu de l'empreinte différente attendue. Utiliser une autre modification valide et vérifier l'imputation (generate-repo.cjs.txt:313).
- OK — §7.2, annexe A.1 : Advice (kind 19) défini, les trois vecteurs réseau concordent ; le vecteur pont du kind 19 manque, codecs `sync2` différés.
- OK — §7.3, §21, ADR (Confirmation, Migration) : bornes TLA explicites, portes intactes (P-COMPAT, activation conjointe L/H, vérifications avant bascule) ; aucune nouvelle Critique.

revise
