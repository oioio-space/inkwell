# Revue Codex n°9 (tour 9) de la spécification, révision 9 (2026-11-01) — verdict : revise

CLOSED — Majeur tour 8 : [§8.1](../spec.md:1031), §14(7), §16.1 ferment le scénario 63 + 2 : une seule des deux racines occupe l’emplacement du dernier pair.
Preuve — Majeur fermé, §8.1 : annonce, doublon, réattribution et redémarrage utilisent exclusivement un emplacement libre avec réservation atomique ; le choix « plus récemment avancée » ne change pas la borne : octets actifs ≤ connectés × 16 Mio ≤ 1 Gio.
Majeur — Vivacité, [§8.1](../spec.md:1036), §10.4, N12 : une ancienne entrée R peut rester inactive indéfiniment si, à chaque libération, une nouvelle annonce prend l’emplacement de son détenteur avant R ; aucune priorité anti-famine n’est prescrite, et DRR arbitre les pairs.
Majeur — §5.5/H, §17 : redemander ne garantit pas l’admission ; aucune borne d’attente n’est établie. L’absence de délai maximal respecte la lenteur, mais ne justifie pas une famine malgré un détenteur honnête disponible.
Correctif — Majeur, §8.1 : imposer une promotion équitable des entrées inactives à chaque libération, sans préempter les transferts actifs ; tester R face à des annonces renouvelées et préciser l’hypothèse d’un détenteur honnête.
Conservé — aucune nouvelle Critique ; [ADR 0034, Confirmation/Migration](../../../docs/adr/0034-protocole-unifie.md:170) maintient P-BYZ, les portes avant bascule, P-COMPAT et l’activation conjointe L/H ; leur présence ne constitue pas leur validation.
revise

