# Revue Codex n°8 (tour 8) de la spécification, révision 8 (2026-11-01) — verdict : revise

- CLOSED — ancien Majeur, index : `max = r` imposé en §9.6 et CDDL ; recettes régénérées, SHA-256 et pages de 4 194 304 / 4 194 305 octets conformes ([spec §8.1](../spec.md:1022)).
- PARTIAL — ancien Majeur, staging global : plafond incluant les absents et ordre d’éviction définis, mais garantie d’admission incohérente (§8.1, §14.7, §16.1).
- Majeur — N7/N12 : contre-exemple permis par §8.1 : 63 pairs servent chacun leur racine ; un 64e sert deux racines imputées à deux annonceurs déconnectés. Cela fait 65 entrées actives, potentiellement > 1 Gio, sans éviction autorisée ([preuve](../spec.md:1030)).
- Correctif — §8.1 : attribuer chaque entrée active à un emplacement connecté unique, gérer les collisions lors des reprises/réattributions et réserver atomiquement sa taille ; tester ce scénario, au-delà des seuls « 65 membres successifs ».
- CLOSED — ancien Mineur, rejeu : `indivisible-tuple-false-root` régénéré identiquement ; modification décodable, empreinte reçue différente, imputation attendue à l’annonceur ([générateur](../vectors/generate-repo.cjs.txt:325)).
- CLOSED — vecteur pont manquant : `bridge-advice` vérifié, kind 19, session 7, génération 2, taille 200 Mio, longueur `doc` en `str16` conforme à l’annexe A.1.
- Mineur — §8.1 : duplication après 30 s compatible avec l’appariement par pair ; toutefois **PEUT** ne garantit pas l’absence de retard annoncée. Staging par racine et détenteurs explicites, sous réserve du défaut d’attribution ci-dessus.
- Aucune nouvelle Critique relevée ; portes conservées : ADR 0034 « Confirmation/Migration », activation conjointe L/H, P-COMPAT et vérifications avant bascule ; limites TLA explicites (§7.3, §21).
revise

