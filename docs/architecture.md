# Architecture

L'architecture d'inkwell est décrite dans [`docs/architecture/`](architecture/README.md) :

- [README.md](architecture/README.md) — une page, en mots simples ;
- [spec.md](architecture/spec.md) — la spécification normative (paquets, règles de dépendance,
  données, pont, réseau, sécurité, reprise, performance, tests, publication, extensibilité) ;
- [decisions.md](architecture/decisions.md) — chaque doute, ses options et la recommandation ;
- [traceability.md](architecture/traceability.md) — exigences → architecture → preuves ;
- [research/](architecture/research/) — études de fond (greffons).

Les règles de dépendance vérifiées par `scripts/arch-check.sh` sont au
[§4.5 de la spécification](architecture/spec.md#45-règles-de-dépendance).
