# Revue Codex n°6 (tour 6) de la spécification, révision 6 (2026-11-01) — verdict : revise

CLOSED — Enveloppe (ancien Majeur) : [spec §7.2, §9.6](../spec.md:1180), routage selon la page singleton encodée ; frontières vérifiées.
CLOSED — Staging/24 h (ancien Majeur) : [§14.7](../spec.md:1438) prime explicitement le nettoyage des caches.
CLOSED — Reliquat ADR (ancien Mineur) : [Conséquences](../../../docs/adr/0034-protocole-unifie.md:159), parts fixes et absence de récupération pour faire place.
OK — [§8.1, §14.7, §16.1](../spec.md:1008) : staging ≤16 Mio/principal ; suppression si obtenu ailleurs, document désadmis ou principal retiré ; fermeture/redémarrage conserve la reprise.
Majeur — [§8.1 « qui sert », §17](../spec.md:1002) : index persistant sans borne, imputation ni suppression définies ; service autorisé, mais « ~70 octets chacune » ne borne pas la carte ; préciser quota et insertion après validation.
Majeur — [§4.7, §8.1](../spec.md:994) : le tuple signe `change`, pas `root/size/doc` ; un relais peut associer un tuple honnête à une fausse racine et immobiliser son unique staging ; imputer l’annonce non vérifiée au livreur, tester ce rejeu.
PARTIAL — Principal provisoire ([§8.1](../spec.md:994)) : annonceur authentifié, donc pas d’usurpation directe ; plusieurs relais contournent néanmoins une limite par auteur réel, tout en restant bornés par pair.
OK — [§5.1, §7.2–7.3, §9.6, §10.7](../spec.md:804) : plafonds LARGE/history-request de 4 Mio, événements anciens ignorés ; deux pages permises, réservations LARGE toujours sérialisées.
OK — [Vecteurs/générateur](../vectors/generate-repo.cjs.txt:296) : régénération identique ; Automerge 70 112 octets, page 70 318, seuils 70 318/70 317 corrects ; hash/taille acceptés/refusés conformes ; codecs de production encore différés (§21).
OK — [ADR, Confirmation/Migration](../../../docs/adr/0034-protocole-unifie.md:169) : portes, P-COMPAT et activation conjointe L/H intactes ; limites TLA explicites ; aucune nouvelle Critique établie.
revise

