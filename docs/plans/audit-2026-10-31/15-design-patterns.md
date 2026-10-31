# Passe des motifs de conception — 31 octobre 2026

Périmètre : `internal/*`, `cmd/*`, `frontend/src`. Revue statique des frontières,
constructeurs, boucles et décisions, approfondie sur confiance, admission et effets.
Référentiel : [principes](../../engineering-principles.md), invariants de
`CLAUDE.md`, [barème](../2026-10-31-architecture-audit.md).
Les mesures ARC ci-dessous viennent du [rapport initial](01-architect-report.md) :
elles ne sont pas remesurées ici. Aucun test, benchmark ou mutant exécuté.
Les lignes citées sont celles du checkout lu ; les corrections concurrentes peuvent
les déplacer. « Aucun abus établi » signifie qu'aucun correctif n'est justifié.

Le Go idiomatique suffit : fonctions, structs, petits ports, `context`, canaux et
verrous explicites. Aucun framework GoF, acteurs ou statecharts n'est nécessaire.
**Les candidats de valeur sont le cœur fonctionnel, les transitions explicites,
la propriété de `Wiring.do`, le journal signé et ses projections, les ports,
la supervision au branchement du LSP et les frontières des effets frontend.**

Attention à l'état du checkout : `trust.Sync[P]` ne dépend plus de transport
(`internal/trust/sync.go:3`, `:29`), `pollCtx` est construit une seule fois
(`internal/app/wiring.go:162`, `:361`) et le watcher est extrait
(`internal/project/watch.go:34`, `:71`). Ne pas rouvrir ARC-C1, ARC-M2 ou le volet
watcher d'ARC-M3 comme s'ils étaient inchangés ; leur validation reste distincte.
L'extraction `internal/session` est **ARC-M5** ; la découpe de `bridge.Storage`
est **ARC-m4**. On s'y rattache, sans créer deux nouveaux constats.

## 1. Cœur fonctionnel / enveloppe impérative — forte valeur

- **Garder.** `trust.fold` calcule une vue déterministe, sans E/S
  (`internal/trust/fold.go:98`) ; `List.current` en gère le cache (`list.go:625`).
  `Sync` reçoit des événements et retourne des sorties sans goroutine
  (`internal/trust/sync.go:34`). Côté TS, `parsePeerMessage` produit un message
  normalisé (`frontend/src/net/admission.ts:51`) ; `statusOf` est une fonction
  simple (`frontend/src/workspace/workspace.svelte.ts:36`).
- **Corriger.** Le cœur existe déjà, mais ses décisions restent concentrées dans
  `sweep.decide` et `members` (`internal/trust/fold.go:270`, `:416`). ARC-M3 mesure
  respectivement 39 et 52 en complexité cognitive, au-delà de 30. Ce n'est pas
  une raison de rendre tout le DAG immuable ou de recopier chaque état.
- **Adopter localement.** Extraire des prédicats nommés selon les règles de
  suspension, révocation et ancienneté ; conserver le fold et ses caches locaux.
  Pour l'admission TS, séparer le calcul de budget de sa mise à jour
  (`frontend/src/net/admission.ts:165`) : entrée `{bucket, now}`, sortie
  `{bucket, allowed}`. Horloge déjà injectée (`:123`) ; pas de nouveau service.
  Problème nommé : complexité ARC-M3 et vérification avant usage (§4), notamment
  la conservation des budgets au reset (`:237`). Bénéfice attendu : tables de cas
  et mutants ciblés, indépendants du réseau. Coût moyen ; risque élevé si l'ordre
  causal du trust change. Preuve : conserver les oracles
  `internal/trust/reference_test.go:397`, `:418`, score de mutation sans baisse,
  complexité ≤ 30. Admission : mutants sur plafond, recharge et reset ; aucun gain
  de performance annoncé.

La séparation calcul/E/S suit [Bernhardt](https://www.destroyallsoftware.com/screencasts/catalog/functional-core-imperative-shell).
La mutation interne d'un résultat neuf ne contredit pas cette séparation.

## 2. Machines à états explicites — forte valeur, sans moteur générique

- **Garder.** `Sync` offre déjà des transitions explicites (`internal/trust/sync.go:57`,
  `:63`, `:89`). `watcher` possède son état et répartit les événements
  (`internal/project/watch.go:46`, `:71`). Le statut UI est dérivé, non recopié
  (`frontend/src/workspace/workspace.svelte.ts:71`).
- **Limite, pas bug prouvé.** Les états de lien sont encodés par trois ensembles
  `up`, `active`, `full` (`internal/app/wiring.go:139`) et les transitions par
  `activate`, `refresh`, `deactivate` (`:492`, `:512`, `:534`). La représentation
  permet des combinaisons que le code doit empêcher ; aucune transition illégale
  observée ne justifie de classer cela en majeur. `LinkKind`
  (`internal/transport/p2p/links.go:19`) décrit un chemin réseau, pas un cycle de vie :
  ne pas le détourner en état d'admission.
- **Adopter avec ARC-M5.** Dans la session extraite, remplacer `active/full` par
  un état par pair : hors ligne, connecté non admis, lié restreint, lié complet.
  Écrire une transition `(état, événement, décision) → (état, actions)` ; les rôles
  restent calculés depuis la liste signée. Problème nommé : prouver le contrat
  `Repo` (`internal/app/wiring.go:216`) et les invariants §4, en particulier
  suspendre sans couper la réplication de confiance. Pour TS, table explicite
  handshake/attente/cooldown autour d'`Admission` (`admission.ts:108`, `:214`,
  `:237`, `:246`) ; les budgets restent attachés à l'identité, pas au lien.
  Bénéfice : moins de combinaisons illégales et tests exhaustifs des transitions.
  Coût moyen ; risque élevé de couper un canal autorisé ou de réinitialiser une
  pénalité. Tester permutations PeerUp/Down, suspension locale/distante, changement
  de projet, attestation anticipée et reset en cooldown. Pas de réécriture de
  toutes les boucles `p2p` sans preuve supplémentaire.

Les [statecharts W3C](https://www.w3.org/TR/scxml/) précisent états et transitions ;
une enum Go ou une union discriminée TS suffit ici, sans SCXML ni dépendance.

## 3. Acteur / boîte aux lettres et propriété — forte valeur

- **Garder.** `Wiring.do` sérialise et attend (`internal/app/wiring.go:232`) ;
  `IssueWith`, `AdoptProject` et `Attestations` l'utilisent (`:256`, `:303`,
  `internal/app/attest.go:221`). `Close` attend la boucle (`wiring.go:225`). Les
  écritures d'`active/full` appartiennent à cette boucle, les lectures externes
  sont protégées (`:140`, `:547`). C'est un usage cohérent ; présence d'un mutex
  ne signifie pas violation du modèle.
- **Corriger.** Le problème établi de propriété est à l'entrée de l'application :
  `cmd/inkwell/main.go:129`, `:139` lancent les boucles, tandis que `:114` annule
  puis ferme leurs ressources sans les attendre. C'est ARC-m7, règle §3 sur le
  propriétaire et la sortie ; aucune course d'arrêt démontrée ici.
- **Adopter sans généraliser.** Avec ARC-M5, documenter les champs possédés par
  la boucle et les vues protégées ; garder les lectures synchrones nécessaires
  au transport. Pour ARC-m7, un propriétaire attend Watch/Router avant de fermer
  runtime et projet. Bénéfice : arrêt ordonné et contrat de mutation vérifiable.
  Coût faible à moyen ; risque de blocage si un callback de la boucle rappelle
  `do`, ou si une E/S ignore l'annulation. Tester arrêt sous trafic et goleak.
  Ne pas imposer des messages typés à chaque closure sans défaut identifié.

L'[acteur et sa mailbox](https://doc.akka.io/libraries/akka-core/current/typed/guide/actors-intro.html)
expliquent la propriété séquentielle ; les [durées de vie Go](https://google.github.io/styleguide/go/best-practices#goroutine-lifetimes)
demandent un arrêt attendu. Aucun runtime d'acteurs à introduire.

## 4. Journal de confiance + projections / CQRS local — forte valeur

- **Garder.** `opStore.load/buildList` rejoue les opérations signées
  (`internal/app/persist.go:162`, `:179`) ; `fold` produit la vue et `current` la
  met en cache. C'est un journal causal, pas un historique ordonné par heure de
  réception. Le KV garde des octets vérifiés (`internal/store/kv.go:1`) et les
  textes restent dans Automerge : respecter les sources de vérité de `CLAUDE.md`.
- **DP1 · mineur · fiabilité du journal.** `afterChange` avance `nOps` avant
  d'appeler `save` (`internal/app/wiring.go:430`, `:431`, `:437`) ; l'erreur ne
  fait que journaliser puis continuer (`:438`). À taille de liste inchangée, le
  prochain appel saute ce bloc ; le ticker l'appelle pourtant (`:350`). Preuve
  statique : une écriture échouée n'est pas réessayée tant qu'aucune nouvelle op
  n'est intégrée. Règle : fiabilité ISO/IEC 25010 du barème et source de vérité
  signée de `CLAUDE.md`. Pas de perte après crash reproduite, donc pas de critique.
- **Corriger concrètement.** Séparer « projection changée » et « opérations non
  persistées » ; reprendre les hashes absents de `saved` aux ticks, avec délai
  borné et erreur observable. Conserver l'écriture idempotente et la reprise de
  swap ; ne pas promettre « durable avant diffusion », car `IssueWith` diffuse
  déjà avant `afterChange` (`:260`). Une telle garantie demanderait une décision
  séparée. Bénéfice : panne disque transitoire récupérable sans nouvelle op.
  Coût faible à moyen ; risque de tempête d'E/S, d'où backoff. Test proposé :
  `Save` échoue une fois, réussit ensuite ; tick sans nouvelle op, puis relecture
  du magasin identique à la liste. Ajouter le cas échec permanent et arrêt.
- **Ne pas étendre.** Ni bus CQRS global, ni event store supplémentaire, ni
  remplacement d'Automerge par un journal Go. Aucun problème mesuré ne le demande.

[Event sourcing](https://martinfowler.com/eaaDev/EventSourcing.html) explique le
rejeu ; [CQRS](https://martinfowler.com/bliki/CQRS.html) distingue commandes et
lectures, tout en rappelant le coût d'une généralisation. Ici, cette séparation
reste locale au trust et aux projections existantes.

## 5. Ports et adaptateurs, injection — forte valeur, frontières existantes

- **Garder.** `Transport` fixe octets opaques, lecteur unique et fermeture
  (`internal/transport/transport.go:97`) ; memnet et la suite de contrat permettent
  de substituer l'implémentation. `invite.Lobby` n'a qu'une méthode
  (`internal/invite/flow.go:441`), `Dropper/Wanter` sont chez le consommateur
  (`internal/app/wiring.go:83`). `Compiler` est défini dans app avec assertion de
  conformité (`internal/app/preview.go:66`, `:73`). Côté TS, `workerAnalyzer`
  injecte `spawn` (`frontend/src/history/analysis.ts:143`).
- **Corriger par renvoi.** `bridge.Storage` reste large
  (`internal/bridge/storestream.go:310`) : appliquer **ARC-m4**, sans nouvelle
  variante du découpage. Le couplage des tests de session relève d'**ARC-M5**
  (mesures initiales : 610 dépendances et binaire de test 104 Mo).
- **Adopter au besoin réel.** Effectuer ces deux corrections ; garder un port
  sidecar au niveau de son consommateur lors du LSP, limité à ouvrir/fermer la
  session et ses flux, tandis que vérification et processus restent dans sidecar.
  `Compiler` couvre déjà la compilation : pas d'interface universelle Process.
  Bénéfice : faux stockage/transport/sidecar ciblés, tests sans embarquer toute
  l'application. Coût moyen à élevé pour M5, faible à moyen pour m4 ; risque de
  déplacer les dépendances sans les réduire. Preuves : gates de dépendance et
  objectifs de taille de test d'ARC-M5 ; contrats/fuzz du pont inchangés.

Les [ports et adaptateurs de Cockburn](https://alistair.cockburn.us/hexagonal-architecture)
isolent les E/S ; les [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments#interfaces)
placent les petits ports chez l'utilisateur. `Transport` reste la frontière
imposée par le projet : aucune découpe décorative en cinq interfaces.

## 6. Supervision et politique de redémarrage — valeur au LSP, pas avant

- **Garder le mécanisme.** `Supervisor` possède l'attente, l'arrêt du groupe et
  l'escalade TERM/KILL (`internal/sidecar/supervisor.go:215`, `:231`). Backoff,
  période stable et abandon existent (`:179`, `:183`, `:191`) ; `Command` doit
  vérifier le binaire à chaque lancement (`:41`).
- **Surapplication établie.** Le constructeur `NewSupervisor` (`:145`) n'a pas
  d'appelant de production : **ARC-m2**, API spéculative, et ARC-m3 pour tinymist
  embarqué sans usage. Aucun bénéfice utilisateur actuel à construire un arbre
  de supervision. La compilation ponctuelle n'a pas à être relancée éternellement.
- **Avec l'étape 3, LSP.** Brancher un seul propriétaire de tinymist ; préciser
  arrêts volontaires, crash récupérable et défaut permanent (hash/config).
  Conserver SHA-256 avant chaque démarrage et abandon observable. Choisir entre
  budget actuel d'échecs consécutifs et intensité sur fenêtre si les essais LSP
  montrent une boucle de crash ; pas d'ajout préventif de cette fenêtre.
  Bénéfice : reprise bornée du service réellement utilisé. Coût moyen ; risque
  de masquer une corruption ou rejouer des requêtes non idempotentes. Tests :
  crash, abandon, arrêt pendant backoff, aucun enfant survivant et hash altéré
  jamais lancé. Si le LSP est différé, suivre la décision de suppression/report
  d'ARC-m2 plutôt que développer le superviseur.

[Erlang/OTP](https://www.erlang.org/doc/system/sup_princ.html) distingue stratégie,
intensité et arrêt des enfants ; s'en inspirer sans transposer toute sa hiérarchie.

## 7. Backoff / circuit breaker

- **Garder.** `p2p.pause` double un délai plafonné, s'annule et ajoute du jitter
  (`internal/transport/p2p/dial.go:176`, `:192`). Admission conserve les cooldowns
  par identité (`frontend/src/net/admission.ts:120`, `:237`, `:262`).
- **Abus établi : aucun.** Le cooldown est une pénalité de pair, pas un circuit
  breaker de disponibilité. En ajouter un global au transport pourrait empêcher
  le rétablissement du réseau sans preuve d'une surcharge actuelle.
- **Usage nouveau limité à DP1.** Retenter la sauvegarde idempotente avec délai
  plafonné, réinitialisé après succès ; horloge injectée pour le test. Coût faible,
  bénéfice et risque décrits au §4. Ni relance aveugle d'une commande signée ni
  retries à chaque couche. Un circuit breaker supplémentaire n'a pas de problème
  mesuré à résoudre : écarté.

[AWS](https://aws.amazon.com/builders-library/timeouts-retries-and-backoff-with-jitter/)
traite délais, idempotence et amplification des retries ; le
[circuit breaker](https://martinfowler.com/bliki/CircuitBreaker.html) répond à un
autre besoin, l'interruption temporaire des appels à une dépendance défaillante.

## 8. Pipelines bornés et annulation

- **Garder.** `blob.Service` borne service et réponses Busy avant de lancer les
  goroutines (`internal/blob/service.go:225`, `:239`, `:248`). Le mux borne les
  files en nombre et octets (`internal/transport/mux/mux.go:58`). L'analyse
  d'historique garde un travail actif et un remplaçant
  (`frontend/src/history/analysis.ts:159`). Ce dernier modèle convient au calcul
  d'affichage périmable, pas aux événements signés qu'il faut conserver.
- **Corriger.** ARC-m7 couvre l'attente à l'arrêt ; ARC-m8 demande goleak pour blob.
  Les seules bornes par file ne prouvent pas un budget global ; ne pas annoncer
  que tous les scénarios hostiles sont couverts. Pas de nouvelle faille mémoire
  mesurée dans cette passe.
- **Adopter uniquement les preuves manquantes.** Attente des boucles et test de
  fuite ci-dessus ; garder les limites et tester annulation sous pression.
  Coût faible ; risque de deadlock si l'on attend avant d'annuler. Ni worker pool
  généralisé ni agrandissement des files pour masquer une saturation. Les
  allocations blob ARC-M1 se corrigent dans les tampons/lecteurs, pas par un
  pipeline supplémentaire ; tout gain doit passer benchstat selon §3.

Le [pipeline Go borné](https://go.dev/blog/pipelines) combine annulation, fermeture
et limite de parallélisme ; le nombre de canaux n'est pas un objectif.

## 9. Options fonctionnelles / configuration

- **Garder.** `typst.Option`, `WithPages`, `WithFormats` configurent un appel
  (`internal/typst/engine.go:143`, `:155`, `:160`). `PreviewOptions` injecte moteur
  et horloge (`internal/app/preview.go:80`, `:94`) ; `WiringOptions.Save` injecte
  l'E/S sans usine abstraite (`internal/app/wiring.go:92`).
- **Corriger.** Le global de test `isolationOverride` subsiste
  (`internal/sidecar/run.go:252`, `:259`) : règle §2, injection explicite et absence
  de global mutable ; rattachement à **ARC-m1**, sans nouvelle mesure de course.
  Plusieurs autres exemples d'ARC-m1 ont déjà changé : `DefaultTimestamp` est
  maintenant une fonction (`internal/typst/engine.go:65`).
- **Adopter la solution simple.** Injecter la sonde d'isolation via la configuration
  privée du lanceur ; défaut réel en production. Coût faible, tests isolables ;
  risque : permettre accidentellement de contourner l'isolation en production.
  Garder le crochet privé et tester le chemin réel sous race. Aucune conversion
  générale des structs Options en closures : elle ne résoudrait aucune règle.

Le [guide Google](https://google.github.io/styleguide/go/best-practices#variadic-options)
compare structs et options variadiques ; choisir selon les appels réels et le
minimum de mécanisme, pas selon un style uniforme.

## 10. Frontend : stores, effets et abonnements — forte valeur

- **Garder.** `$derived` calcule les compteurs
  (`frontend/src/preview/preview.svelte.ts:56`) ; `$state.raw` garde les objets
  externes (`frontend/src/ui/editorSession.svelte.ts:9`). Les effets de
  `App.svelte:109`, `:121` pilotent document/CodeMirror ; celui de `:162` retourne
  le désabonnement du handle (`:171`). Ne pas transformer ces E/S en `$derived`.
- **DP2 · mineur · durée de vie d'un abonnement.**
  `frontend/src/workspace/workspace.svelte.ts:508` ajoute un callback d'attente
  via `onProject` ; ni résolution ni timeout ne le retirent (`:509`, `:510`).
  `frontend/src/net/WailsNetworkAdapter.ts:152` pousse dans un tableau sans
  fournir de disposer ; `:437` rappelle les abonnés. Chaque attente passée par
  cette branche laisse donc une closure attachée à l'adaptateur. Règle : §3,
  ressources bornées/propriétaire ; fiabilité et maintenabilité du barème.
  Rétention visible statiquement, coût mémoire et effet utilisateur non mesurés.
- **Corriger à la frontière.** Faire retourner `off()` par `onProject`, l'appeler
  sur succès, timeout et arrêt du workspace. Donner à `Workspace` un `stop`
  symétrique de `start` (`workspace.svelte.ts:144`) et l'appeler au démontage :
  `App.svelte:184` ne ferme aujourd'hui que timer et preview. Nettoyer les
  abonnements, projection, synchro et adaptateur selon leur propriétaire.
  Bénéfice : durée de vie explicite et remontage testable ; coût moyen, risque
  de fermer un Repo encore utilisé ou de nettoyer trop tôt. Tests : succès,
  timeout, démontage pendant start et démarrages/arrêts répétés, aucun callback
  résiduel. Garder les calculs de domaine dans les modules TS purs et les effets
  dans cette enveloppe ; pas de migration générale de stores ni de nouveau
  framework d'état. Les imports domaine/UI écartés par le rapport initial
  restent écartés tant qu'aucune règle frontend n'est adoptée.

Les docs Svelte 5 réservent les [effets](https://svelte.dev/docs/svelte/$effect)
aux interactions externes et les [valeurs dérivées](https://svelte.dev/docs/svelte/$derived)
au calcul sans effet ; les abonnements doivent rendre leur nettoyage possible.

## Couverture des autres zones et décisions écartées

Les commandes de maintenance injectent déjà contexte et sorties
(`cmd/inkwell-relay/main.go:66`, `cmd/sidecar-fetch/main.go:46`) ; conserver cette
composition explicite. `identity.LoadOrCreate` reçoit un `os.Root`
(`internal/identity/device.go:43`), `version.resolve` sépare le calcul du stamp
(`internal/version/version.go:50`). Les modules CSV, recherche et présentation
ont déjà des points testables : `applySplices` (`frontend/src/editor/csv/model.ts:13`),
worker injecté (`frontend/src/editor/search/runner.ts:50`), environnement de pilotage
(`frontend/src/present/pilot.ts:18`). Aucun problème nommé ne demande d'y ajouter
Repository, Factory, Strategy, Observer générique ou bus d'événements.
Cela ne constitue pas une certification exhaustive de ces modules.

Sources primaires recherchées/consultées le 31 octobre 2026, liens auprès des
motifs concernés. Les textes fondateurs anciens sont utilisés pour leurs
principes ; aucune affirmation de nouveauté ni dépendance supplémentaire.
Toute modification future de frontière ou d'invariant demande une ADR comparant
au moins deux options, conformément au barème. Les corrections locales restent
soumises aux tests proposés ; les coûts ci-dessous sont qualitatifs, sans devis.

## Classement des actions

« maintenant » inclut les vérifications ciblées avant modification de sécurité ;
« avec étape 3 » vise le LSP restant, « avec ARC-M5 » le chantier déjà identifié.
« jamais » signifie écarté sur les preuves actuelles, à réexaminer seulement avec
un problème nouveau et mesuré. Le risque est explicité dans chaque section.

| Motif | Où | Problème résolu (preuve) | Changement | Bénéfice attendu | Coût | Priorité |
|---|---|---|---|---|---|---|
| Journal + projection | `app/wiring.go:430` | DP1 : échec Save non repris sans nouvelle op | Distinguer changements et persistance en attente ; retry borné | Reprise après panne transitoire | Faible–moyen | maintenant |
| Cœur fonctionnel | `trust/fold.go:270`, `:416` | ARC-M3 : 39/52, seuil 30 | Prédicats de règles, oracles et mutation testing | Décisions de sécurité vérifiables | Moyen | maintenant |
| Effets et abonnements | `workspace.svelte.ts:508`, `WailsNetworkAdapter.ts:152` | DP2 : callback retenu après attente | Disposer et arrêt symétrique | Ressources libérées et remontage sûr | Moyen | maintenant |
| Propriétaire des boucles | `cmd/inkwell/main.go:114`, `:129` | ARC-m7 : arrêt sans attente | Annuler, attendre, fermer | Arrêt ordonné | Faible–moyen | maintenant |
| Pipeline borné, preuve | `blob/service.go:239` | ARC-m8 : absence de vérification goleak | Ajouter preuve de sortie, sans nouveau pipeline | Non-régression des durées de vie | Faible | maintenant |
| Injection simple | `sidecar/run.go:259` | ARC-m1 : global mutable interdit §2 | Sonde privée injectée | Isolation des tests | Faible | maintenant |
| Ports stockage | `bridge/storestream.go:310` | ARC-m4 : interface large et complexité mesurée | Appliquer la découpe du rapport initial | Contrats ciblés | Moyen | maintenant |
| Ports session | `app/wiring.go:121` | ARC-M5 : 610 dépendances, test 104 Mo (rapport) | Appliquer l'extraction, sans nouvelle couche | Tests de session autonomes | Élevé | avec ARC-M5 |
| États explicites de lien | `app/wiring.go:139` | Contrat Repo et invariants §4 à prouver | Un état/pair, transitions et actions | Moins d'états illégaux représentables | Moyen | avec ARC-M5 |
| Admission fonctionnelle / états | `net/admission.ts:165`, `:237` | Vérification §4, budgets à préserver au reset | Calcul de budget pur et table de transitions | Mutants et séquences hostiles testables | Moyen | avec ARC-M5 |
| Mailbox cohérente | `app/wiring.go:232` | ARC-M5 et règle de propriété §3 | Conserver do ; expliciter propriété et vues | Mutation sérialisée préservée | Faible | avec ARC-M5 |
| Supervision + port LSP | `sidecar/supervisor.go:145` | ARC-m2/m3 : mécanisme sans consommateur | Brancher au LSP ou reporter/supprimer selon ARC-m2 | Reprise du service réellement lancé | Moyen | avec étape 3 — LSP |
| CQRS global / event bus | Trust, Automerge, pont | Aucun problème ; sources de vérité imposées | Ne pas ajouter | Évite double vérité et protocole | Élevé évité | jamais |
| Framework acteurs/statecharts | Session, p2p, frontend | Aucun besoin au-delà des idiomes existants | Enum/union et canaux existants | Moins de mécanisme | Élevé évité | jamais |
| Circuit breaker global | `p2p/dial.go:176` | Aucun défaut mesuré du backoff actuel | Garder backoff, jitter et annulation | Reconnexion conservée | Moyen évité | jamais |
| Options fonctionnelles partout / GoF | Constructeurs Go et modules TS | Aucun défaut des configs simples | Garder structs, fonctions et petits ports | API lisible | Moyen évité | jamais |

## Contre-vérification (Sonnet, 2026-10-31)

Chaque ligne « maintenant » a été revérifiée dans le code par un autre modèle.

- **DP1** (`internal/app/wiring.go:430`, échec de `save` non repris) : confirmé. Correctif simple : reprise
  bornée dans la boucle, sans nouveau motif.
- **DP2** (`workspace.svelte.ts:508`, `WailsNetworkAdapter.ts:152`, écouteur jamais retiré) : confirmé,
  mineur. `onProject` renvoie une fonction de désabonnement, appelée aussi au délai dépassé.
- **ARC-m7** (`cmd/inkwell/main.go:114,129`) : exagéré. Un `WaitGroup` attendu dans `OnShutdown` avant
  les fermetures suffit ; « propriétaire des boucles » est écarté comme motif de forme.
- **ARC-m8** (`internal/blob`) : exagéré. `Service.Run` attend déjà ses goroutines ; il manque seulement
  un `TestMain` goleak.
- **ARC-m1** (`internal/sidecar/run.go:252-259`, `isolationOverride`) : confirmé, faible valeur ;
  paramètre ou champ, Go ordinaire.
- **ARC-m4** (`bridge.Storage`, 13 méthodes) : confirmé ; interfaces côté consommateur.
- **ARC-M3** (« cœur fonctionnel » de `trust/fold.go`) : fond confirmé, mais c'est un effort de tests
  (oracles, mutation testing), pas un motif à introduire.

Lots retenus, dans l'ordre : DP1, DP2, ARC-m7 (`WaitGroup`), ARC-m4, ARC-m8 (+ ARC-m1).
