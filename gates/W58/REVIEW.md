# W58 — cross-family review before freeze, round 1 (Codex, 2026-11-14)

**Verdict : REJECT avant gel.** Le mode restreint est pertinent, mais la classification proposée laisse un dossier fabriqué obtenir le fonctionnement normal. Revue statique uniquement : aucun fichier modifié, aucun test lancé.

1. **Critical — Une liste signée ne prouve pas le consentement local.** L’attaquant peut créer sa genèse puis ajouter la clé publique de la victime : nul besoin de copier un projet authentique. Ce dossier devient *member* selon [ADR:81](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:81), puis reçoit admission, réseau et stockage selon [node.go:177](/home/mathieu/GolandProjects/inkwell/internal/app/node.go:177). Le protocole permet à l’admin d’ajouter une clé sans signature du sujet ([flow.go:411](/home/mathieu/GolandProjects/inkwell/internal/invite/flow.go:411)). **Correction :** exiger aussi une provenance enregistrée dans le profil après création ou invitation explicite ; ajouter `TestCraftedGenesisAddsSelfStillRestricted`.

2. **Critical — Le résidu R-3 n’est pas borné par la convergence.** Une copie peut rejouer un état antérieur à une révocation et fournir des chunks arbitraires. Les attestations sont signées sous le projet courant dès que le pli donne le rôle writer ([attest.go:124](/home/mathieu/GolandProjects/inkwell/internal/app/attest.go:124), [:157](/home/mathieu/GolandProjects/inkwell/internal/app/attest.go:157)), avant toute convergence garantie. L’affirmation contraire figure dans [ADR:137](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:137). **Correction :** restreindre les copies non enregistrées, conserver un état de confiance hors dossier et tester le rejeu d’une liste ancienne. Les signatures par changement ne prouvent pas, seules, le consentement à ouvrir un dossier.

3. **Major — Les 32 candidats rendent « aucun membre existe » indémontrable.** Déposer 32 dossiers étrangers avec un mtime futur masque le dossier membre classé 33e ; FP-7 ouvre alors l’un des étrangers ([plan:19](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:19)). Le classement actuel dépend effectivement du mtime contrôlable ([project.go:259](/home/mathieu/GolandProjects/inkwell/internal/project/project.go:259)). **Correction :** sélectionner d’abord parmi les dossiers enregistrés dans le profil ; à défaut, afficher les étrangers sans prétendre avoir exclu tout membre. Ajouter le cas « membre 33e ».

4. **Major — `fresh` permet encore une appropriation automatique.** « Aucun document » ne signifie pas « création locale interrompue » : un attaquant peut fournir un dossier vide avec des métadonnées préparées. Pourtant D6 autorise sa signature ([plan:35](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:35)). **Correction :** réserver la reprise *fresh* à une transaction de création attestée dans le profil ; tout autre dossier sans genèse reste *unclaimed*. Tester métadonnées, caches et magasins préparés sans fichier visible.

5. **Major — La classification puis le chargement n’ont pas de contrat contre les substitutions.** Le plan sépare choix, `peekTrust`, ouverture et `Start` ([plan:47](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:47)); aujourd’hui `Start` recharge la liste ([node.go:164](/home/mathieu/GolandProjects/inkwell/internal/app/node.go:164)). **Hypothèse à fermer :** remplacement du dossier ou de la liste entre vérification et utilisation. **Correction :** conserver les handles et l’identité vérifiés, vérifier la liaison profil/dossier/ProjectID sur l’objet réellement utilisé, refuser toute divergence ; tests avec substitutions déterministes entre étapes.

6. **Major — « Lecture bornée » ne borne pas encore les entrées hostiles.** FP-1 borne nombre et taille, mais ne précise ni types de fichiers ni délai ([plan:13](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:13)). Le lecteur cité ouvre avant de limiter la lecture et liste tout le répertoire ([persist.go:230](/home/mathieu/GolandProjects/inkwell/internal/app/persist.go:230), [:238](/home/mathieu/GolandProjects/inkwell/internal/app/persist.go:238)). Un FIFO peut bloquer ; des liens peuvent rediriger la lecture. **Correction :** lecture incrémentale bornée, fichiers réguliers seulement, politique explicite sur liens et substitutions, budget global ; tests FIFO, liens, énorme répertoire et DAG incomplet. Exiger aussi zéro pending et une liste entièrement validée.

7. **Major — Le join du même projet réactive les internes précisément exclus auparavant.** FP-10 ajoute les ops puis rouvre le dossier comme membre ([plan:22](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:22)), alors que l’ADR reconnaît que ses chunks contournent l’admission ([ADR:43](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:43)). **Correction :** rejoindre dans un dossier neuf, même pour le même ProjectID, ou reconstruire un stockage propre sans importer les anciens internes. Tester une invitation valide avec un magasin local empoisonné.

8. **Major — D13 contredit l’identité immuable du dossier.** L’ADR promet un projet fixé pour la vie du dossier ([ADR:53](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:53)), mais conserve le remplacement depuis un dossier membre ([plan:42](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:42)). Le chemin existe : `store.replace`, puis changement du wiring ([invites.go:357](/home/mathieu/GolandProjects/inkwell/internal/app/invites.go:357)). **Correction :** appliquer « chaque join crée un dossier » à tous les standings et tester membre→autre projet, échec et crash.

9. **Major — La genèse unique n’est garantie que par le nouveau créateur Inkwell.** `trust.Genesis` reste déterministe pour les mêmes arguments ([op.go:404](/home/mathieu/GolandProjects/inkwell/protocol/trust/op.go:404)); la spécification publique autorise toujours une racine vide et ne prescrit aucune fraîcheur ([spec.md:271](/home/mathieu/GolandProjects/inkwell/docs/protocol/spec.md:271)). **Correction :** définir normativement l’unicité à la création, avec vecteurs et compatibilité. Documenter que les anciens ProjectID ne peuvent être corrigés en place sans invalider leurs descendants signés.

10. **Major — Le réensemencement peut transformer une projection obsolète en contenu faisant autorité.** FP-8 prescrit de recréer toute racine manquante depuis le fichier principal ([plan:20](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:20)). Le code actuel remplace déjà une ouverture ratée par un document vide ([workspace:379](/home/mathieu/GolandProjects/inkwell/frontend/src/workspace/workspace.svelte.ts:379)). **Correction :** autoriser le seeding uniquement pour une création/copie locale avec manifeste de seed vérifié ; sur join ou projet existant, attendre la racine distante ou signaler une récupération nécessaire. Tester absence, corruption et indisponibilité réseau séparément.

11. **Major — La publication et la récupération après crash se contredisent.** FP-9 publie au rename puis ensemence après réouverture ([plan:21](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:21)); D9 exige pourtant aucun nouveau projet visible après un crash « à toute étape » ([plan:38](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:38)). **Correction :** distinguer avant/après commit durable ; après commit, reprise idempotente obligatoire. Définir quand réseau et invitation deviennent possibles, et tester les crashes pendant chaque seed et avant sa suppression.

12. **Major — Le modèle proposé ne représente pas les attaques décisives.** Deux clés et des standings déjà abstraits ([plan:59](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:59)) ne prouvent ni provenance, ni rejeu, ni substitution, ni admission des anciens chunks. **Correction :** modéliser séparément consentement du profil, liste fournie, état connu et durable, substitution, join et publication ; vérifier absence d’actes avant consentement et absence de réactivation des internes. Pour `RenameBeforeSync`, modéliser visibilité **et** durabilité. TLA+ suffit ; les 36 lignes Go prouvent uniquement la table choisie.

13. **Major — La matrice tests/mutants ne couvre pas toutes les règles.** Les noms imposés omettent notamment refus direct d’écriture du store, seed idempotent et multifichier, chemin argument D12, fin/échec du réseau de join ([gate:37](/home/mathieu/GolandProjects/inkwell/gates/W58.toml:37), [:44](/home/mathieu/GolandProjects/inkwell/gates/W58.toml:44), [:56](/home/mathieu/GolandProjects/inkwell/gates/W58.toml:56)). Les mutants de `standing.ts` prétendent couvrir un renew dans le composant ([plan:88](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:88)). **Correction :** ajouter une ligne règle→entrée→oracle indépendant→test→mutant ; muter le point d’appel réel pour renew et host, et vérifier la panne attendue, pas seulement le nom d’un mutant dans un rapport ([gate:82](/home/mathieu/GolandProjects/inkwell/gates/W58.toml:82)).

14. **Major — Le gate n’est pas encore démontré satisfaisable en 45 minutes ni rouge pour la bonne raison.** Aucun gate imbriqué explicite, mais les budgets annoncés totalisent déjà environ 44 minutes sans marge ([gate:19](/home/mathieu/GolandProjects/inkwell/gates/W58.toml:19), [:73](/home/mathieu/GolandProjects/inkwell/gates/W58.toml:73)). Mutation n’a pas de timeout global ([gate:50](/home/mathieu/GolandProjects/inkwell/gates/W58.toml:50)). Les critères manquants échoueraient aujourd’hui sur fichiers/tests absents, sans démontrer leur oracle. **Correction :** mesurer avant gel, plafonner chaque commande, découper si nécessaire ; fournir un témoin fautif compilable par critère. Les mutants TLC doivent échouer sur la propriété désignée, alors que le runner accepte actuellement toute violation ([tla-check.sh:89](/home/mathieu/GolandProjects/inkwell/scripts/tla-check.sh:89)).

**(a)** Ajouter le registre du profil maintenant. Il doit lier emplacement canonique, identité du dossier et ProjectID ; `project.json` reste copiable et insuffisant seul. Migration : classer les dossiers existants comme hérités restreints, sans inscription automatique fondée sur leur liste ; invitation explicite vers un dossier neuf, ou copie personnelle. ADR 0034 étape 8 ne résout pas la provenance locale.

**(b)** Recommandation W58 : **racine aléatoire créée en Go**, puisque la genèse possède déjà ce champ et que sa signification est correcte. Porter l’obligation de fraîcheur dans le protocole public et ses tests, sans imposer Go aux autres implémentations. Un nonce versionné est préférable si le protocole doit permettre une création sans racine ; il implique codec, signatures, vecteurs et négociation de version. Dans les deux cas, conserver les anciennes genèses ; traiter leur collision comme dette héritée explicite.

**(c)** Inclure dans W58 **chaque join vers un dossier neuf**, même pour le même projet. Cela ferme simultanément le remplacement depuis un dossier membre et la réactivation d’internes étrangers après une invitation valide. Le conserver séparément empêcherait d’affirmer l’invariant annoncé par cette ADR.

Codex session ID: 01a1d936-433d-78c3-8e47-25c049956c51
Resume in Codex: codex resume 01a1d936-433d-78c3-8e47-25c049956c51

## Review round 1 — dispositions (Claude Opus, 2026-11-14)

Every citation was re-read in the code and the drafts before answering. All hold; two are a few lines off and are
corrected here: the unsigned Add is `internal/invite/flow.go:416-424` (`:411` is the pending ask), the empty-document
fallback is `frontend/src/workspace/workspace.svelte.ts:382-389`. No finding is refuted. Revision 2 replaces the single
block with W58a (registry and restricted standing; alone unblocks R1), W58b (genesis freshness), W58c (fresh folders);
the draft `gates/W58.toml` is replaced by `gates/W58a.toml`, `gates/W58b.toml`, `gates/W58c.toml`.

| # | Finding | Disposition | Where fixed |
|---|---|---|---|
| 1 | A signed list does not prove local consent | accepted | ADR facts 1, option C rejected, rules 1-2 (profile registry); plan A-1, `TestCraftedGenesisAddsSelfStillRestricted`, model mutant MemberIsEnough |
| 2 | R-3 not bounded by convergence | accepted | ADR fact 3, rule 1 (trust frontier in the record), standing *rolled back*; old R-3 removed; plan A-1 `TestOldListReplayRestricted`, mutant NoFrontier |
| 3 | 32 candidates cannot prove « no member » | accepted | ADR rule 5 (registered first, profile clock); plan A-6 with 33 future-mtime folders, `TestSelectRegisteredFirst`, mutant LatestByMtime |
| 4 | `fresh` still claims automatically | accepted | `fresh` removed; ADR rule 8 (only a `creating` record of this profile allows a genesis); plan A-8, C-1 |
| 5 | No contract against substitution between check and use | accepted | ADR rules 2-3 (handles held, `Start` takes the verified project, never re-reads); plan A-3 `TestSubstitutionAfterCheck`, mutant RecheckByPath |
| 6 | Bounded read does not bound hostile input | accepted | ADR rule 2; plan A-2 (chunked `ReadDir`, `O_NOFOLLOW`/`O_NONBLOCK`, `fstat` regular, byte and 2 s budget, zero pending, one genesis); FIFO, links, huge directory, incomplete DAG tests |
| 7 | Joining the same project reactivates excluded internals | accepted | ADR rule 11 (every join into a fresh folder, same `ProjectID` included, received list only); plan C-5 `TestJoinPoisonedStoreIgnored` |
| 8 | D13 contradicts the folder's fixed identity | accepted | D13 removed; ADR rule 11 for all standings; plan C-5 member to other, failure, crash tests; mutant JoinInPlace |
| 9 | Genesis uniqueness only by Inkwell's creator | accepted | ADR rule 7 and W58b: normative obligation in `docs/protocol/spec.md` §4.1, vectors, legacy geneses kept as inherited debt (R-7); plan B-1..B-3 |
| 10 | Re-seeding can make a stale projection authoritative | accepted | ADR rule 9 (seed only from this profile's verified `seed.json`; otherwise recovery state, never an empty or local text); plan B-6 (absent, corrupt, offline), C-3 |
| 11 | Publication and crash recovery contradict | accepted | ADR rules 8-9 (before commit nothing visible; after commit resumed, no network or invitation before `ready`); plan C-1, C-2 |
| 12 | The model misses the decisive attacks | accepted | two models: `ForeignStanding.tla` (consent, supplied list, substitution, selection, migration) and `FreshFolder.tla` (visibility and durability separate, crash, seeding, joins); plan parts A and C |
| 13 | Test and mutant matrix incomplete | accepted | plan part A rule matrix (input, oracle, test, mutant); store write refusal, multi-file and idempotent seed, argument path, join window end and failure added; renew and host mutated on the mounted `ShareDialog.svelte` copy, the named assertion must fail |
| 14 | Gate not shown satisfiable in 45 min nor red for the right reason | accepted | split in three gates (about 40, 25, 40 min, measured at freeze); every command has a timeout, mutation included; each criterion names its faulty witness; `tla-check.sh` to check the property a mutant names (it accepts any violation today, `scripts/tla-check.sh:89`) |

Answers (a) to (c), as decided by the coordinator: (a) registry now (ADR rule 1); migration designed so that an honest
person is not locked out (ADR rule 6): once, only when the registry is absent and the device key existed before this
launch, only the folder the replaced version would open at this very launch, only if its list names the device; the
person sees their current project unchanged, other old folders are inherited (*copied here*); residual R-3 states the
remaining trust-on-upgrade exactly. (b) Go-minted fresh root, obligation in the public spec with vectors, legacy debt
(W58b). (c) every join into a fresh folder, inside W58 (W58c).

Points for round 2: whether the one-folder migration of ADR rule 6 is acceptable as the only trust-on-upgrade; whether
W58a alone is enough for R1 (case 1 reads but cannot edit or share until W58c).

# Round 2 (Codex, 2026-11-14)

**Verdict global : REJECT avant gel.** La révision ferme plusieurs défauts de conception, mais le registre, les états intermédiaires et les gates gardent des lacunes majeures. Revue statique uniquement : aucun fichier modifié, aucun commit, aucun test exécuté.

Les statuts « résolu » ci-dessous concernent le **contrat documentaire**, pas une implémentation déjà validée.

## Round-1 resolution table

Liens abrégés : **ADR** = `docs/adr/0043-foreign-project.md`, **plan** = `docs/plans/2026-11-14-w58-foreign-project.md`.

| # | Statut | Preuve et limite |
|---|---|---|
| 1 | **Résolu** | Le consentement vient désormais du profil ; une liste nommant la victime ne suffit plus. [ADR:84–98](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:84), test hostile [plan:42](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:42). Exception de migration explicitement assumée. |
| 2 | **Partiellement** | Frontière enregistrée et standing *rolled back* ajoutés. [ADR:88–98](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:88). La limite de 64 hashes et la restauration du profil laissent des trous ; voir F2–F3. |
| 3 | **Résolu** | Sélection prioritaire parmi les dossiers liés au registre, horloge du profil ; cas des 33 dossiers hostiles nommé. [ADR:107](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:107), [plan:48](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:48). |
| 4 | **Résolu** | `fresh` supprimé ; création autorisée uniquement par une transaction du profil ; `load` ne signe plus. [ADR:122–126](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:122), [plan:33](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:33). |
| 5 | **Partiellement** | Handles conservés et liste en mémoire imposés à `Start`. [ADR:89–103](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:89). Cela ferme la relecture par chemin, mais la liaison sans identité stable reste insuffisante ; F1. |
| 6 | **Résolu** | Lecture incrémentale, non bloquante, sans liens, fichiers réguliers, budgets et validation complète explicites. [plan:27](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:27), cas adverses [plan:44](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:44). |
| 7 | **Partiellement** | Join neuf avec liste reçue seulement dans W58c. [ADR:136–138](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:136). W58a ne définit pas le comportement transitoire du join actuel ; F4. |
| 8 | **Partiellement** | D13 supprimé ; changement de projet dans un dossier interdit par W58c. [plan:96](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:96). Même lacune entre a et c ; F4. |
| 9 | **Résolu** | Obligation normative, vecteurs, racine fraîche et dette des anciennes genèses explicités. [ADR:116–121](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:116), [plan:81–85](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:81). |
| 10 | **Partiellement** | Suppression du remplacement vide dans B-6 ; seed réservé aux transactions locales. [plan:86](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:86), [ADR:127](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:127). Provenance du manifeste encore incomplète ; F5. |
| 11 | **Partiellement** | Commit et reprise après commit distingués. [ADR:122–131](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:122). La suppression du manifeste précède encore la transition durable vers `ready` ; F6. |
| 12 | **Partiellement** | Les deux modèles couvrent désormais consentement, substitution, migration et durabilité. [plan:54–59](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:54), [plan:100–104](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:100). Ils doivent intégrer F1–F6 ; les propriétés des mutants ne sont pas toutes associées explicitement ; F9. |
| 13 | **Partiellement** | Matrices et mutants des composants réels ajoutés. [plan:37–52](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:37). Les gates peuvent encore masquer des échecs et accepter de simples mentions de mutants ; F8–F9. |
| 14 | **Non résolu** | Découpage utile, mais budgets toujours estimés et témoins seulement décrits. [plan:116–118](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:116). Deux commandes sont statiquement insatisfaisables ; F7 et F10. |

## New defects

**Registre et substitution.** Avec une identité de dossier stable, remplacer le dossier par une copie à un autre inode doit être refusé ; un renommage sur le même système conserve cette identité. C’est une amélioration réelle. Mais le repli « emplacement seulement » de [l’ADR:159](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:159) autorise une substitution au même emplacement si les identifiants et la liste sont copiés. Les déplacements hors du même système doivent donc rester restreints, sans rattachement automatique.

**Sauvegarde du profil.** Restaurer seulement la clé, sans registre, satisfait de nouveau les conditions de migration. Restaurer un ancien registre restaure aussi une ancienne frontière de confiance. Ces deux cas ne sont pas couverts par les résiduels actuels ; F2.

**Migration initiale.** Oui, l’attaquant peut gagner l’unique slot : dossier valide, genèse hostile avec `Add(self)`, mtime maximal. Le code actuel sélectionne bien sur ce mtime ([project.go:259–264](/home/mathieu/GolandProjects/inkwell/internal/project/project.go:259)) et l’ajout ne demande pas le consentement du sujet ([flow.go:416](/home/mathieu/GolandProjects/inkwell/internal/invite/flow.go:416)).

Pour une alpha entre personnes de confiance, **ce résidu initial peut être acceptable comme décision explicite du propriétaire** : il conserve exactement une exposition déjà présente dans l’ancien binaire. Il ne constitue toutefois pas un consentement historique et ne permet pas d’affirmer « un dossier fabriqué ne peut jamais agir » comme le fait [l’ADR:145–146](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:145). Il faut aussi tester le gagnant hostile, aujourd’hui absent de la matrice de migration ([plan:49](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:49)).

**W58a seul.** La lecture sans modification ni partage est un état produit cohérent pour une personne réinstallée, à condition d’accepter explicitement cette portée réduite de R1 ([plan:15–17](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:15)). En revanche, **W58a n’est pas encore une frontière livrable sûre** : le devenir du join en place et le remplacement d’une racine manquante restent hors de son contrat ; F4.

## Gates assessment

| Gate | Satisfaisable ? | ≤45 min ? | Rouge aujourd’hui pour la bonne raison ? |
|---|---|---|---|
| **W58a** | **Non en l’état** : analyseur e2e invalide, F7. | Non démontré : commentaires ≈44 min, presque sans marge ; build e2e non plafonné. [W58a:61–77](/home/mathieu/GolandProjects/inkwell/gates/W58a.toml:61). | Non établi. Plusieurs commandes échoueraient d’abord sur packages, modèles ou tests absents ; cela ne démontre pas leur oracle. |
| **W58b** | Pas d’impossibilité syntaxique relevée ; acceptation frontend trop permissive, F8. | Non démontré : ≈22 min annoncées avant la régression, qui n’a pas d’estimation. [W58b:38–48](/home/mathieu/GolandProjects/inkwell/gates/W58b.toml:38). | Non établi ; les témoins ne sont pas fournis avec leurs résultats. |
| **W58c** | **Non en l’état** : même analyseur e2e invalide, F7. | Non démontré : ≈36 min avant la régression ; build non plafonné. [W58c:16–49](/home/mathieu/GolandProjects/inkwell/gates/W58c.toml:16). | Non établi ; même distinction entre absence d’artefact et détection du défaut. |

**Aucun gate imbriqué repéré** dans les critères ni dans les tâches `e2e:build` et `mutate:go` consultées ([mise.toml:345](/home/mathieu/GolandProjects/inkwell/mise.toml:345), [mise.toml:396](/home/mathieu/GolandProjects/inkwell/mise.toml:396)).

Par famille de critères :

- **Spec** : échec attendu sur éléments documentaires/i18n manquants ; les témoins adverses restent à démontrer.
- **Go, fuzz, frontend, TLA** : packages, tests ou modèles nouveaux encore absents ; leur absence n’est pas un témoin fautif compilable.
- **Mutation** : les seuils sont vérifiés, mais pas toutes les fautes manuelles ni leurs assertions attendues.
- **E2E a/c** : rouge intrinsèque du wrapper Python, indépendamment du produit.
- **Review et simplify** : présence de fichiers ou mots-clés ; cela vérifie une livraison documentaire, pas la justesse d’un oracle.
- **Régression** : statut actuel inconnu, aucune exécution autorisée pendant cette revue.

Le runner TLC actuel accepte encore une violation quelconque ([tla-check.sh:89](/home/mathieu/GolandProjects/inkwell/scripts/tla-check.sh:89)). La modification prévue est pertinente, mais ne constitue pas encore la preuve que chaque mutant tue sa propriété nommée.

## Verdicts per sub-block

| Sous-bloc | Verdict |
|---|---|
| **W58a — registre et restriction** | **REJECT** : liaison de remplacement, restauration, frontière bornée et contrat transitoire à décider ; gate à corriger. |
| **W58b — fraîcheur de genèse** | **ACCEPT WITH FIXES** : conception retenue ; corriger la validation frontend et fournir la qualification du gate. |
| **W58c — dossiers neufs** | **REJECT** : provenance du seed et reprise après suppression du manifeste insuffisamment définies ; gate e2e insatisfaisable. |

## Ranked findings — 11 maximum

Aucun nouveau **Critical** inconditionnel établi. Les attaques conditionnelles sont signalées comme telles.

1. **Major — Le repli par emplacement réautorise une copie substituée.**  
   [ADR:92–94](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:92), [ADR:159–160](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:159). **Hypothèse :** système sans identité stable, remplacement au chemin enregistré avec les mêmes `project.json`, genèse et frontière. Toutes les preuves restantes sont copiables ; la substitution obtient `registered`.  
   **Correction :** échouer fermé lorsque l’identité ne peut être établie ; copie ou join neuf pour réautoriser. Un déplacement sur le même système suit l’identité, une copie ne la suit jamais. **Décision de conception à escalader.**

2. **Major — Une restauration du profil annule les garanties « une fois » et anti-rejeu.**  
   [ADR:88](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:88), [ADR:110–115](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:110), [plan:32](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:32). Clé restaurée sans registre : nouvelle migration possible. Ancien registre restauré : ancienne frontière acceptée ; **hypothèse** de rejeu si le dossier lié retrouve cet état.  
   **Correction :** définir un état de récupération ; ne pas assimiler toute absence de registre à une mise à niveau. Soit obtenir un ancrage de version non restauré, soit documenter honnêtement que la garantie est relative au profil restauré et restreindre sa réactivation. Ajouter les deux scénarios de sauvegarde. **Décision de conception à escalader.**

3. **Major — Les 64 hashes ne représentent pas nécessairement toute la frontière connue.**  
   [ADR:88](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:88). Le code prévoit explicitement une frontière dépassant `MaxPrev` ([list.go:447–450](/home/mathieu/GolandProjects/inkwell/protocol/trust/list.go:447)). La borne de parents n’est donc pas une borne de têtes. **Hypothèse :** une troncature omet une branche contenant une révocation ; retirer cette branche passe le contrôle des hashes retenus.  
   **Correction :** conserver une couverture complète, bornée par les 1024 ops, ou refuser d’agir si elle ne tient pas ; aucune troncature silencieuse. Tester plus de 64 branches avec révocation sur la branche omise.

4. **Major — La frontière W58a laisse le join actuel changer de projet hors du registre.**  
   Le découpage remet chaque join neuf à c ([plan:11–17](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:11)). Aujourd’hui `adopt` remplace la liste puis change le wiring ([invites.go:357–379](/home/mathieu/GolandProjects/inkwell/internal/app/invites.go:357)), sans repasser par `Start`. Un contrôle au seul démarrage ne ferme pas cette transition. La racine vide reste aussi possible jusqu’à b ([workspace.svelte.ts:382–389](/home/mathieu/GolandProjects/inkwell/frontend/src/workspace/workspace.svelte.ts:382)).  
   **Correction :** définir le produit intermédiaire : désactiver le changement de projet avant c, ou avancer son chemin sûr dans a ; avancer également le refus du remplacement vide, ou imposer b avant R1. Tester W58a indépendamment. **Décision de découpage à escalader.**

5. **Major — Le manifeste de seed ne possède pas encore de preuve de provenance côté profil.**  
   [ADR:95](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:95), [ADR:127–130](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:127), [plan:94](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:94). Des hashes stockés dans le même manifeste que les fichiers prouvent leur cohérence, pas leur origine. **Hypothèse :** après publication et avant reprise, substitution du manifeste et des fichiers dans le dossier `seeding`.  
   **Correction :** enregistrer dans le profil l’empreinte du manifeste et l’identité de la transaction ; vérifier la liaison du dossier aussi pour `creating/seeding`, puis vérifier les fichiers depuis les objets effectivement utilisés. Tester une substitution cohérente manifeste + fichiers.

6. **Major — Crash entre suppression de `seed.json` et `ready` non défini.**  
   L’ordre prescrit est suppression du manifeste, puis `ready` ([ADR:129–130](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:129)). Les injections nommées s’arrêtent notamment *avant* sa suppression ([plan:93](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:93)). Après sa suppression durable, le profil peut rester `seeding` sans source de reprise.  
   **Correction :** rendre la complétion et `ready` durables avant le nettoyage du manifeste, ou ajouter un état durable permettant cette reprise ; injecter après suppression et avant/après sauvegarde du registre.

7. **Major — Les commandes e2e a/c passent des `\n` littéraux à Python.**  
   [W58a:65](/home/mathieu/GolandProjects/inkwell/gates/W58a.toml:65), [W58c:41](/home/mathieu/GolandProjects/inkwell/gates/W58c.toml:41). Après décodage TOML, le programme de `python3 -c` contient les caractères `\` et `n` ; les doubles quotes de `/bin/sh` ne les transforment pas en sauts de ligne. Le parseur Python échouera avant l’analyse du rapport.  
   **Correction :** placer l’analyseur dans un fichier Python, ou transmettre de vrais sauts de ligne. Le rouge actuel du wrapper ne compte pas comme preuve adverse.

8. **Major — Les rapports frontend peuvent masquer une campagne en échec.**  
   [W58a:58](/home/mathieu/GolandProjects/inkwell/gates/W58a.toml:58), [W58b:35](/home/mathieu/GolandProjects/inkwell/gates/W58b.toml:35). Après `vitest`, `; cd .. && python…` abandonne son code de sortie ; seuls quelques titres passés sont requis. Une autre assertion peut échouer et le critère passer. Les e2e a/c abandonnent aussi le code de sortie et ne contrôlent pas les erreurs globales du rapport.  
   **Correction :** préserver le code de sortie, imposer zéro erreur/échec global et zéro skip selon le contrat, puis vérifier les titres requis.

9. **Major — Les mutants manuels et leurs propriétés restent insuffisamment contractualisés.**  
   [W58a:80–82](/home/mathieu/GolandProjects/inkwell/gates/W58a.toml:80) vérifie seulement la présence de noms ; [W58c:52–53](/home/mathieu/GolandProjects/inkwell/gates/W58c.toml:52) exige seulement un fichier non vide. Les listes TLA nomment propriétés et mutants séparément ([plan:55–59](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:55), [plan:101–104](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:101)). De plus, le témoin FIFO est déclaré tué par timeout ([W58a:36–37](/home/mathieu/GolandProjects/inkwell/gates/W58a.toml:36)), contrairement à [méthode v2:63–65](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-12-method-v2.md:63).  
   **Correction :** une ligne explicite mutant → propriété/assertion → résultat attendu, avec preuve vérifiable sur copie compilable ; distinguer assertion de délai dépassé et timeout global du processus.

10. **Major — Le plafond et la qualification rouge restent promis, non établis.**  
    [plan:116–118](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:116). Les builds a/c n’ont pas de timeout externe ; nombre de timeouts commencent après l’attente de `heavy.sh`. Le runner n’ajoute pas de plafond ([gate.py:427–429](/home/mathieu/GolandProjects/inkwell/scripts/gate.py:427)).  
    **Correction :** plafonner les critères complets, mesurer chaque gate corrigé sur machine inactive et conserver, pour chaque oracle, le résultat d’un témoin fautif exécutable. Cette preuve manque ; je ne l’ai pas exécutée conformément à la consigne.

11. **Minor — L’état « rôles inconnus » contredit la règle d’invitation.**  
    L’ADR réserve l’invitation à l’admin non suspendu ([ADR:139–140](/home/mathieu/GolandProjects/inkwell/docs/adr/0043-foreign-project.md:139)), tandis qu’A-10 autorise les rôles inconnus ([plan:35](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-14-w58-foreign-project.md:35)). Cela conserve brièvement le faux bouton que W58 doit retirer.  
    **Correction :** bouton désactivé pendant le chargement, puis activé sur capacité Go connue ; tester admin, non-admin et self absent.

## Escalation note

**Des Major restent.** Les choix à arbitrer sont surtout : identité indisponible, restauration du profil, acceptation du trust-on-upgrade et frontière livrable de W58a. Le résidu initial de migration est défendable pour R1 entre personnes de confiance, mais doit être accepté comme exception explicite, avec des garanties limitées.

Cette revue est le dernier round demandé. La méthode v2 limite les rounds et impose une décision de conception pour un Major après le smoke ([méthode v2:17–21](/home/mathieu/GolandProjects/inkwell/docs/plans/2026-11-12-method-v2.md:17)). **Si un Major subsiste au troisième point de revue, il faut escalader la décision ; pas lancer une nouvelle série de correctifs ponctuels.**

Codex session ID: 01a1d93f-da02-7bc3-a9b7-d43d714faa43
Resume in Codex: codex resume 01a1d93f-da02-7bc3-a9b7-d43d714faa43

## Round 2 — decisions and dispositions (coordinator decisions D-1..D-7; applied by Claude Opus, 2026-11-14)

Method v2: no third patch round; the remaining majors are settled by the design decisions below, applied in ADR 0043
revision 3, the plan revision 3 and the three draft gates. Citations re-read: all hold except F7 (see its row).

| # | Finding | Decision | Disposition | Where |
|---|---|---|---|---|
| F1 | Location-only binding re-authorises a substitute | D-1 | accepted | ADR option I rejected, rule 2 (no stable identity: inherited; a move on one file system follows, a copy never; re-authorisation only by copy or fresh join); plan A-3 `TestCopyAtSamePathInherited`, `TestNoStableIdentityInherited`, `TestMoveSameFSFollows`; mutant location-fallback; model LocationOnlyBinding -> NoActOnSubstitute |
| F2 | Profile restore breaks « once » and anti-replay | D-2 | accepted | ADR rule 6: no registry + key = recovery state; E-1 an explicit, dated, owner-accepted R1-only exception, removed by the first release after R1; restored registry heads are a lower bound (descendant or rolled back); « Consequences » now says exactly what holds; plan A-7 `TestMigrateHostileWinner`, `TestRestoreKeyWithoutRegistry`, `TestRestoreOldRegistry`; witness WitnessHostileWinnerEnrolled |
| F3 | 64 hashes are not the whole frontier | D-3 | accepted | ADR rule 1 (every head, bounded by `MaxMemberOps`; otherwise the folder does not act); plan A-4 `TestRecordAllHeads` (65 and 1024 heads, revocation on the head a cap would drop); mutant capped-heads |
| F4 | W58a leaves in-place project change and the empty root | D-4 | accepted | order W58-T, W58b, W58a, W58c; W58b takes the missing-root rule (B-6); W58a A-11 refuses a join from a folder that holds a project (`join.refusedInPlace`), keeping in-place adoption only from an untouched first launch so that two people can still start; R1 needs all lots (plan header, proposed PROGRESS row) |
| F5 | Seed manifest has no profile-side provenance | D-5 | accepted | ADR rules 1, 8, 9 (transaction id and manifest SHA-256 in the record; binding checked in `creating`/`seeding`; files hashed on the handle read); plan C-3 `TestSeedSubstitutionRefused`; model SubstitutedManifest -> SeedFromRecordedDigest |
| F6 | Crash between manifest deletion and `ready` | D-5 | accepted | ADR rule 9 (complete -> `ready` durable -> manifest deleted and synced; a `ready` record only deletes a leftover); plan C-2 crashes after deletion and around each registry save; model DeleteManifestBeforeReady -> SeedResumable |
| F7 | e2e analysers receive literal `\n` | D-6 | **refuted as stated, fixed anyway** | TOML basic strings decode `\n` into a newline (TOML 1.0, « Basic strings » escapes): `tomllib` on the round-2 `gates/W58a.toml` gave a real newline in the e2e command (`repr` shows `\n`, not `\\n`), and round 1 parsed the same analyser with `ast.parse` after decoding; `gates/OI.toml` uses the same form. Analysers now live in `scripts/gate_check.py` (lot W58-T), so the point is moot |
| F8 | Reports can hide a failing campaign | D-6 | accepted | `gate_check.py` runs the command itself, keeps its exit code, requires zero failures and zero unlisted skips before named tests (plan part T, with self-tests) |
| F9 | Manual mutants and properties under-contracted; FIFO killed by timeout | D-6 | accepted | `gates/W58{a,b,c}/mutants.toml`: one entry per mutant (fault -> test -> rule id -> fail), applied to a compilable copy; gate `tla` criteria pass `--mutant NAME=PROPERTY` and `tla-check.sh` checks the named property; the FIFO witness is the A-2 deadline assertion, a timeout is no kill |
| F10 | Caps and red qualification only promised | D-6 | accepted | every criterion has an outer `timeout -k 10` covering the `heavy.sh` wait; each gate header makes freeze conditional on a measured run ≤ 45 min on an idle machine and on each faulty witness run red, recorded in `gates/<lot>/WITNESSES.md` (checked by `review`) |
| F11 | Unknown roles still offer « Inviter » | D-7 | accepted | ADR rule 12; plan A-10 (`Session.Capabilities()`, disabled while loading); Vitest loading / admin / non-admin / self absent; mutant renew-unguarded |

Not reconciled within the decisions: D-4's refusal, applied literally, would refuse every join (a first launch always
creates a project), break `join.spec.ts` and `collab.spec.ts`, and stop two people from starting. The plan narrows it to
« a folder that holds a project the person has written in »; the untouched first-launch folder keeps today's in-place
adoption until W58c (ADR rule 11, residual R-8). To confirm.

Text for the owner (E-1 and R1 scope), to be shown verbatim:

> Pour R1, je propose une exception volontaire et limitée. Au premier lancement de la nouvelle version, Inkwell ne
> possède pas encore la liste des documents qui sont à vous sur cet ordinateur. Pour que vous retrouviez votre document
> comme la veille, sans question, il reprend une seule fois, et un seul document : celui que l'ancienne version aurait
> ouvert ce jour-là, à condition que ce document vous compte parmi ses membres. Le prix est le suivant : si quelqu'un
> avait glissé avant la mise à jour un faux document plus récent dans votre dossier Inkwell, c'est lui qui serait repris,
> exactement comme l'ancienne version l'aurait ouvert ; cela n'ajoute aucun risque nouveau, mais ce n'est pas une
> preuve que le document est à vous. La même chose arrive si vous restaurez votre identité sans cette liste. Tous les
> autres documents s'ouvrent en lecture seule, avec « En faire une copie à moi » ou « Demander une invitation ».
> L'exception est datée du 14 novembre 2026 et disparaît dans la première version après R1. R1 ne sortira qu'avec les
> quatre parties : l'outil de vérification, l'identifiant unique de chaque nouveau document, les documents en lecture
> seule, et la copie ou l'invitation dans un dossier neuf. Entre la troisième et la quatrième, rejoindre un autre
> document depuis un document où vous avez déjà écrit sera refusé, sans rien modifier.

## Round 3 — scope reset (owner's rule of 2026-11-14, W64; applied by the Claude session)

Shown exception E-1, the owner answered: « on ne doit pas tout protéger, on reste un éditeur de texte, on délègue les
risques d'accès aux fichiers à l'utilisateur et son système d'exploitation ». The rule is written in
`docs/engineering-principles.md` §4 (« Frontière de confiance »): the adversary is a peer; files already on the machine
are the person's. ADR 0043, the plan and the gates are rewritten as revision 4: two standard blocks, W58b (genesis
freshness) then W58 (standing, read-only, invite capability, copy, ask for an invitation).

Dropped: lot W58-T (`scripts/gate_check.py`, named-property `tla-check.sh`), the ownership registry and its
standings, handle binding, bounded reads of a hostile folder, exception E-1 and the migration, rollback detection, the
restricted runtime (the node keeps running: joining needs it), fresh-folder transactions and seeding manifests for
joins, the models `ForeignStanding.tla` and `FreshFolder.tla`, the gates `W58a.toml` and `W58c.toml`.

| Earlier finding | Status under revision 4 |
|---|---|
| Round 1 critical « a crafted folder is admitted as a member », « an older copy is replayed » | out of scope (prior local write access, W64) |
| F1 location-only binding, F2 profile restore and replay, F3 64 heads, F5 seed manifest provenance, F6 crash between manifest deletion and `ready` | out of scope: the mechanisms they concern no longer exist |
| F4 in-place project change and the empty root | joining in place is kept as today by decision; the empty-root fallback is residual R-1 of the plan |
| F7 literal `\n` in e2e analysers | refuted in round 2; unchanged |
| F8 a report can hide a failing campaign, F9 mutants under-contracted, F10 caps and red runs only promised | still true of the gate conventions in general, not specific to W58: the revision-4 gates use the forms of RC1 and PR1 (named tests read back, `scripts/netsim/mutants.toml`, JSON reports with `numFailedTests`); a tooling block is opened only on a measured false verdict (method v2) |
| F11 unknown roles still offer « Inviter » | holds: rule N-4 |
| Deterministic genesis (ADR fact 4) | holds, and the copy makes it reachable: block W58b |

Before the freeze: one second-family review of the revision-4 plan and gates (standard rigour), limited to the trust
boundary above; the four « to check before the freeze » points of the plan are answered in code first.

## Round 4 — Codex on revision 4, dispositions (2026-11-14)

Verdict: ACCEPT WITH CHANGES. Integrator's decision: majors 1 to 5 all come from the copy, so the copy leaves R1
(golden rule; W64) and the second action becomes a take-over in place. Revision 5 of the ADR, the plan and the gate.

| # | Finding (Codex) | Disposition | Where |
|---|---|---|---|
| 1 | The original is not untouched: opening deletes caches (`docs.go:103`) and the projection writes at start (`projection.ts:92`) | moot (copy deferred); attached to residual R-4 | ADR options |
| 2 | The copy has no complete failure contract (`project.go:127-143`) | moot; R-4 | ADR options |
| 3 | A staging folder can be picked by `Latest` (`project.go:255-264`) | moot; R-4 | ADR options |
| 4 | Seeding is not idempotent (`Repo.import` returns before persisting; `workspace.svelte.ts:381-389`) | moot; R-4 | ADR options |
| 5 | No project switch exists (`main.go:221-229`, `openRequests.ts:51`) | moot; R-4 | ADR options |
| 6 | A typed Go error carries no key across the bridge | accepted | rule N-2: `Invitation.Refused`, tested on the real bridge (e2e) |
| 7 | The e2e criterion loses Playwright's exit code, lacks `e2e:build`, must require zero global errors and the titles | accepted | `gates/W58.toml` criterion `e2e` |
| 8 | Mutant entries missing; `mutants.py:74-80` accepts any `--- FAIL` | accepted | plan « Mutants »: entries MW1..MW6 written with the gate, optional `expect` field in `mutants.py` (smallest change, in this block, with its test) |
| m1 | Weak document checks | accepted | criterion `docs` checks the rule ids, the catalogue keys and the ADR status |
| m2 | `timeout` must wrap `heavy.sh` | accepted | every criterion: `timeout -k 10 N ./scripts/heavy.sh …` |
| m3 | Blob index not rebuilt in a copied folder | moot; R-4 | ADR options |
| + | `List.Since(nil)` and `Op.Subject` give « an op about this device » | used | rule N-1 `removed` |
| + | `Repo.import(binary, { docId })` accepts a chosen id; an empty root stays wire-compatible | recorded for residual R-2 | ADR consequences |

W58b (genesis freshness at every creation) is no longer a prerequisite: the take-over's genesis carries the folder's
recorded root URL, so its `ProjectID` differs from that of any other project of this device. Residual R-2, after R1.

## Round 5 — Codex on revision 5, dispositions (2026-11-14)

Verdict: ACCEPT WITH CHANGES. This is the second and last review round of the revision-4 line (method v2: two rounds at
most); revision 6 applies it and is ready to freeze. A further major finding would call for a design decision, not a
patch.

| # | Finding (Codex) | Disposition | Where |
|---|---|---|---|
| 1 | (major) The webview keeps `#attested`, `#lastHeads`, `Coverage` across a `ProjectID` change; a head asked before is never sent again | accepted: new rule N-11, one mechanism for the take-over and for a join that keeps the same root (same gap) | ADR rule 6; plan N-11, N-10 scenario « a third profile joins after the take-over and receives the earlier text » |
| 1b | Re-attestation and ADR 0031 | stated: valid for the boundary as executed today, not the stricter planned rule | ADR « Consequences », residual R-5 (with ADR 0034 step 8) |
| 2 | (major) An archive under `.inkwell/backups/` is rotated with the text backups | accepted: dedicated `.inkwell/trust-archive/<time>/`, never rotated; archive by copy, active ops untouched before `replace` | ADR rule 3; plan N-5, N-7, mutant MW6 |
| 3 | « AdoptProject resets everything » is too broad | accepted: Codex's inventory replaces it | plan, paragraph after the rules |
| 4 | MW4 too weak; `expect` must bind text to the named test | accepted: `TestTakeOverWiringFollows` (wiring `ProjectID`, a third device admitted); `go test -json`, same-test fail and output events; collision case in `mutants_test.py` | plan N-5 and « Mutants »; gate `mutant-tool`, `mutants` |
| 5 | Uncapped `e2e:build` and regression tasks | accepted: each inside a `timeout -k 10` | gate `e2e`, `regression` |
| 6 | A2 (joiner and the genesis root), A4 (roles event after adopt) | settled with Codex's evidence; removed from the open checks | plan « Settled, and still to check » |

