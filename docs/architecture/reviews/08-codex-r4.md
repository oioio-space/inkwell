# Revue d’architecture — Codex, tour 4

2026-11-01 — architecture **1.0.0-brouillon.5**, protocole **2.0.0-brouillon.10**.

Revue statique des contrats : `spec.md`, `CHANGES.md`, `decisions.md`, `coherence.md`,
`traceability.md`, `06-codex-r3.md`, `05-fable-r3.md`, exigences N1–N41, `CLAUDE.md`,
principes d’ingénierie et protocole ; contrôle de l’ADR 0031 pour l’intégration.
Aucune revue concurrente du tour 4 consultée. Aucun prototype, confinement ou banc réputé passé.
Seul ce rapport est créé ; aucun commit.

Gravités : **bloquant** = garantie centrale contredite ; **majeur** = contrat important
incomplet ; **mineur** = correction locale. Une fermeture **au contrat** ne vaut ni livraison
ni réussite de la porte expérimentale.

## 1. Les quatre blocages précédents

| Constat | Référentiel | Preuve dans la révision 4 | Conclusion et gravité |
|---|---|---|---|
| R3-B1 : dossier privé persistant en clair | N37 ; principes §4 ; EN-7 ; [documentation tmpfs](https://docs.kernel.org/filesystems/tmpfs.html) : pages susceptibles de partir au swap | §6.10 EN-4 impose mémoire avec swap vérifié, ou support chiffré vérifié, sinon refus des fonctions qui matérialisent. EN-7 conditionne la garantie ; P-VAULT impose l’inspection hors ligne avant relance. | **Ancien défaut bloquant fermé au contrat pour le dossier de travail.** Le disque mémoire macOS reste à prouver. La garantie globale du coffre reste bloquée par R4-B1 ci-dessous, qui concerne une autre copie sensible : le secret de secours. |
| R3-B2 : DEK de 32 octets confondue avec la clé de fichier age | N15/N37 ; [C2SP age, File key / Header / Payload](https://c2sp.org/age@v1.1.0) | EN-2 : fichier age complet produit par `filippo.io/age`, clé de fichier de 16 octets, CBOR `{v, project, peer, epoch, dek}` comme charge utile ; stanzas inchangées ; refus des substitutions de projet/appareil. AD-91 rév. 4 concorde. | **Bloquant fermé au contrat.** Le format age et les destinataires ne remplacent pas la protection de leurs secrets locaux (R4-B1). |
| R3-B3 : versions restaurables contre ménage des blobs | N39 ; INV-3 ; ADR 0029 ; purge/finalité du protocole §4.5/4.10 et ADR 0031 | §6.5 ajoute versions nommées et variantes non abandonnées aux racines du ménage ; HI-9 pose l’épingle avant publication, prévoit V1/A→B/31 jours/redémarrage/reconstruction Git et fait primer la purge, avec version « incomplète ». | **Bloquant fermé au contrat.** HI-2 se lit avec l’exception explicite HI-9 : un moment quelconque ne garantit pas les anciens blobs ; une purge ne revient jamais. |
| R3-B4 : portes contradictoires ou non discriminantes | N18/N38 ; principes introduction/§3 ; §0.5 E-1…E-5 | Entrées et seuils figés avant essai, résultats bruts, échec si non mesurable, matrice, replis nommés ; 16 installés/6 actifs/10 en attente ; sidecars inclus ; inspection du coffre avant relance ; seuils français ; trois moteurs pour P-FRAME ; ACP sorti des expériences. | **Bloquant fermé comme méthode d’acceptation expérimentale.** Les jeux d’essai précis et machines doivent être archivés avant exécution ; ils ne sont pas déjà produits par ce document. R4-M1 empêche de déclarer PERF-TYPE prouvé avec le chronomètre proposé. |

### Contrôle porte par porte

Référentiel commun : N18/N38, principes « chaque principe a sa preuve », E-1…E-5.
La gravité des défauts de porte était **bloquante** au tour 3 ; les remèdes ci-dessous sont
acceptés comme contrats d’expérimentation, sans acceptation anticipée des technologies.

| Porte | Entrées / oracle et seuil préétabli | Repli et jugement |
|---|---|---|
| P-PLUG-PROTO | Rust/Go, imports réellement liés, ABI-8/9, paquet signé, erreurs mémoire, timer positif, annulation, mandat révoqué, proposition/Annuler ; tous les cas passent | Extensions intégrées TS seules : **fermé**. |
| P-PLUG-BENCH | Témoin en processus, trajet complet, tailles/froid/chaud, deux architectures, 16/6/10 ; annexe : appel 64 Kio p95 ≤ 2 ms, activation ≤ 150/500 ms, repos ≤ 40 Mio | Précompilation/conservation en mémoire pour activation lente ; autre dépassement → nouvelle revue, jamais budget relevé : **fermé**. Le repli doit lui-même être mesuré après redémarrage et perte de l’hôte ; aucune exemption de froid n’est acquise. |
| P-SANDBOX-* / PLUG-COMPROMISED-* | Corpus natif hostile, canaris normalement accessibles, descripteurs, réseau, enfants, deux hôtes, quotas/arrêt ; sidecars : lectures autorisées seulement, sorties cumulées bornées, aucun réseau, mort avec parent ; zéro réussite interdite ; matrice OS/architectures/paquet/archive | Capacités refusées sur cible ; Linux seul = alpha : **fermé**. « Tout refusé » vise les attaques, pas la lecture légitime du dossier par Typst. |
| P-VAULT | Coupures à chaque étape, inspection hors ligne avant relance, migration/récupération sans ancien appareil, erreurs disque/objets, index/temporaires ; I1–I8 et budgets §12.5 | Coffre non livré, G13 reporté en v1.x : **méthode fermée**, mais scénario des secrets à ajouter pour fermer R4-B1. |
| P-IDX | Corpus français versionné, 200 requêtes antérieures à l’essai, jugements à trois niveaux ; rappel@10 ≥ 0,8, MRR ≥ 0,6, retard ≤ 5 s ; reconstruction et budgets ; R1/R2 seulement à +0,1 de rappel | Sans index, lectures explicitement désignées sous RG-6/IA-2 : **fermé**. E-1 exige de figer aussi la règle de pertinence des trois niveaux et l’agrégation des métriques avant le banc. |
| P-GIT-BENCH | 1 000 fichiers, 2 Mo texte, 50 blobs de 1 Mo, 500 versions avec 2 % de changement ; ≤ 1 s/version, reconstruction ≤ 60 s | Écrivain minimal : **fermé**. |
| P-MERMAID | Module épinglé, imports permis, corpus supporté/refusé, polices, SVG assaini, annulation ; ≤ 8 Mio, p95 ≤ 300 ms/200 nœuds, six cibles | Mermaid absent, Graphviz/D2 restent : **fermé**. |
| P-FRAME | Processus dédié, canaris réseau/navigation/source, boucle infinie, frappe préservée ; chaque moteur ; budgets de frappe du §2 | Mermaid complet absent : **fermé**. |
| PD-* | Boucle du protocole et vecteurs, selon son §21 ; aucun octet nouveau approuvé ici | Fonctions dépendantes bloquées : **fermé par séparation des acceptations**. |

Les autres portes de conformité (AC/FE, P-EXT, fuzz, revue de sécurité) restent obligatoires
dans leurs sections ; §0.5 n’est pas une dispense. L’acceptation de l’architecture ne transforme
pas un nom de test en résultat.

## 2. Réponses à Fable et jugement sur « Intégrer »

| Constat du tour 3 | Référentiel et preuve | Verdict / gravité d’origine |
|---|---|---|
| P-IDX mesurable | N33/N38 ; §0.5, AD-89 rév. 4, traçabilité N33 : corpus français, rappel@10 ≥ 0,8 et MRR ≥ 0,6 | **Majeur fermé au contrat**, pas résultat obtenu. |
| ACP = report | N7/N35 ; §0.5, ACP-1, AD-82 rév. 4, coherence §5 : après v1, décision séparée avec profil propre | **Mineur fermé**. Le reste d’ACP-1 décrit une intention future, pas une capacité v1 autorisée. |
| Repli avant mesure | N38 ; colonne de §0.5 et E-4 | **Majeur fermé**. Un report du coffre n’est pas une clôture de N37. |
| Mesure de référence au premier lot | N38, CLAUDE « prove perf changes » ; PM-5 et première ligne du MVP dans coherence §5 | **Majeur de phasage fermé** ; oracle d’affichage encore R4-M1. |
| Plafond du dossier privé | N32/N37/N38 ; EN-9 : dépendances réelles, ≤ 512 Mio, tmpfs/disque mémoire bornés, budget global, refus visible et aucun débordement sur disque | **Majeur fermé au contrat**. Le comptage doit inclure l’index qui partage ce dossier (EN-5/RG-3), pas lui ajouter 200 Mio hors plafond. |
| Sens d’« Intégrer » | N7/N39/N41 ; HI-6, AD-96 rév. 4 ; protocole §4.6/4.7/9.4/9.7 | **Majeur fermé pour le choix architectural** : différence appliquée, nouvelles modifications autorisées de l’intégrateur, groupe annulable, attribution fine conservée dans la variante. |

**J’approuve l’intégration par différence appliquée et l’absence d’amendement d’ADR 0031 pour
cette opération.** Aucune modification étrangère n’est importée dans le graphe de l’original :
la fabrique locale produit des changements ordinaires, signés et couverts pour le document
cible, soumis à la garde et aux droits. La [fusion native Automerge](https://automerge.org/automerge/api-docs/js/functions/merge.html)
combine au contraire les changements des documents. HI-6 ne prétend plus réécrire leurs auteurs.
Il traite variante modifiée pendant revue, répétition, divergence, auteur retiré, verrou et
annulation après frappe ; §15.5 apporte la proposition ancrée. §16 reconnaît que le code actuel
`A.merge` n’est pas la preuve du contrat. Les tests devront aussi conserver les morceaux refusés
ou non acceptés lors d’une intégration partielle, sans avancer leur point de reprise : c’est
l’application de HI-6 et de « Accepter / Refuser » (§15.5), pas une nouvelle règle d’admission.

## 3. D-10…D-16 : accord avec l’issue enregistrée

| Débat | Référentiel / preuve | Position et gravité de l’enjeu |
|---|---|---|
| D-10 | Moindre privilège, principes §4 ; IPC-3, H-4, AD-57 rév. 4 | **Accord** : un hôte par greffon actif, paresseux, six au plus, aucune éviction pendant mandat/appel ; coût à prouver. Ancien enjeu **bloquant fermé**. |
| D-11 | N7 ; H-0/H-5 : Landlock avant ré-exécution, contrôle EACCES avant seccomp, puis EPERM et TSYNC | **Accord**, sous P-SANDBOX ; un statut `ready` déclaré ne remplace pas l’essai natif hostile. Ancien enjeu **bloquant sous porte**. |
| D-12 | N25/N26 ; ABI-9 et P-PLUG-PROTO | **Accord** Rust/Go 1.0, TinyGo 1.1, attente positive/annulation et coûts consignés. Enjeu **majeur fermé au contrat**. |
| D-13 | Principes §2, N31 ; §15.12/15.18 | **Accord**, UTCP retiré ; pas de second catalogue. Enjeu **mineur fermé**. |
| D-14 | Principes introduction/§3 ; E-1…E-5, AD-95 | **Accord** sur expériences sans dépendance de production et replis préétablis ; aucune preuve acquise par rédaction. Enjeu **bloquant de méthode fermé**. |
| D-15 | N7/N24/N38 ; §0.5 et AD-70 rév. 4 | **Accord** : candidat beautiful-mermaid 1.1 derrière P-MERMAID, sans P-FRAME pour cette voie WASM ; Mermaid complet derrière P-FRAME par moteur. Enjeu **majeur fermé au choix**. |
| D-16 | Protocole §3.3/20.4 ; ABI-7, ST-5, EN-6, §9.4 | **Accord**, préfixes `inkwell-ext:` disjoints, PD-2 retiré, liste fermée incluant `enc-key`. Ancien enjeu **bloquant fermé au choix** ; reliquat rédactionnel R4-m1. |

## 4. Défaut bloquant du coffre révélé par la relecture

### R4-B1 — Bloquant : le secret de secours peut rester lisible sur le disque volé

**Référentiel.** N37 ; modèle « Voleur » §9.2 ; EN-3/EN-7 ; principes §4, sécurité par
défaut ; [C2SP age, destinataires X25519](https://c2sp.org/age@v1.1.0#x25519-recipient-stanza).
Un destinataire de secours est une voie complète de déchiffrement, pas une seconde étape
qui exigerait aussi le mot de passe local.

**Preuves.** `spec.md:777` (§6.1, ligne « Secret de secours ») permet le trousseau **ou**
`<UserConfigDir>/inkwell/` en `0600`. ST-3 (`:1063–1068`) conserve cette règle et dérive du
secret le destinataire de secours. EN-2 (`:1230–1238`) chiffre la DEK pour ce destinataire ;
EN-3 (`:1239–1242`) annonce un mot de passe Argon2id en l’absence de trousseau, sans imposer
que **le secret de secours** soit lui aussi enveloppé. EN-4 autorise légitimement Linux avec
tmpfs sans swap et sans chiffrement intégral du disque : le dossier de configuration peut
donc rester sur un disque ordinaire.

**Contre-exemple permis par le contrat.** Linux, aucun trousseau, aucun swap, projet protégé
et dossier privé en tmpfs, configuration sur disque non chiffré. L’appareil éteint est volé.
L’attaquant lit le fichier de secours hors ligne (`0600` n’oppose aucune barrière à cette
lecture), dérive la clé de récupération pour le ProjectID, ouvre `keyring/<époque>.age`,
obtient la DEK et déchiffre le projet. Le mot de passe Argon2id n’est jamais nécessaire.
Les sorties en mémoire et le format age peuvent être parfaitement corrects : l’attaque tient.

**Nature.** Défaut hérité, non une régression que j’attribue à la révision 4 ; il n’avait pas
été relevé au tour 3. La réparation du dossier privé est réelle mais ne suffit pas à vérifier
la promesse du coffre de bout en bout. Ce n’est pas une vulnérabilité affirmée du code livré.

**Fermeture requise.** Toutes les voies locales qui peuvent restituer la DEK doivent dépendre
d’un secret absent du disque lisible hors ligne : secret de secours et clé privée X25519
dans un trousseau convenablement protégé, ou enveloppés sous le mot de passe avec paramètres
Argon2id fixés, ou sur un support chiffré vérifié. Aucun repli automatique en fichier secret
nu `0600` pour un projet protégé. Une phrase gardée hors appareil reste une voie de secours.
Aligner §6.1, ST-3, EN-3 et §9.4 ; P-VAULT doit inclure **configuration et trousseau absents /
indisponibles**, verrouillage puis vol hors ligne, récupération avec phrase, et vérification
que le mot de passe ne peut être contourné par une autre copie locale des clés.

## 5. Régressions et restes non bloquants

### R4-M1 — Majeur : PM-4 arrête le chronomètre avant la peinture

**Référentiel.** N38 ; principes §3 ; PM-2, budget frappe→affichage du §2 ;
[HTML Standard, mise à jour du rendu](https://html.spec.whatwg.org/multipage/webappapis.html#update-the-rendering).
Les rappels d’animation précèdent recalcul des styles, mise en page et mise à jour visuelle.

**Preuve.** `spec.md:1880–1882` propose événement→`requestAnimationFrame` suivant pour prouver
événement→peinture. Une frame dont le rappel arrive vite mais dont la mise en page est lente
peut ainsi réussir le premier chronomètre et rater le budget visible. Le remplacement de la
preuve Long Tasks est bienvenu, mais ce nouvel oracle reste insuffisant : **nouveau défaut
de mesure**, donc je ne certifie pas « aucune régression » sans réserve.

**Correction.** Séparer durée instrumentée du gestionnaire (≤ 4 ms), attente du rappel et
latence jusqu’au rendu de la modification concernée ; utiliser une mesure de rendu vérifiée
par moteur ou un instrument externe, avec cas témoin de mise en page lente. Ne pas présenter
un simple rAF, ni un double rAF, comme une preuve universelle de présentation. Tant que le
banc ne mesure pas sa grandeur, PM-4/E-2 le font échouer. Ce reste maintient R3-M4 partiellement
ouvert ; il ne remet pas en cause le choix des expériences ni les replis de R3-B4.

### R4-m1 — Mineur : résumés anciens encore contradictoires

**Référentiel.** N18/N36, unicité du contrat §0.3 ; protocole §3.3/20.4 et ABI-7.
**Preuves.** §3.2 (`spec.md:267`) parle encore d’un hôte global et d’un hôte par projet ;
l’annexe, Décision 6 (`:3169`), donne encore `inkwell/plugin/v1`, contre le message exact
`inkwell-ext:plugin-package:1\0` d’ABI-7 (`:3006`). `decisions.md`, AD-83/91/95/96,
conserve des recommandations initiales remplacées par les paragraphes « Révision 3/4 ».
Les corrections datées et les contrats détaillés rendent le choix identifiable ; ces restes
hérités ne constituent pas un désaccord technique rouvert. Remplacer les résumés de spec et
marquer les recommandations remplacées comme historiques avant d’en dériver des implémentations.

Les autres réponses Codex R3 sont présentes : découverte et namespace de sauvegarde (§6.7),
transitions d’exclusion (IG-5), intersection d’autorité et invalidation RAG (RG-6/7), surveillance
permanente et verbes par classe (RS-8). Référentiels respectifs : N18/ADR 0029, N5/N34,
N7/N33 et N7/N32 ; anciens enjeux **majeurs**, fermés au contrat, sauf précision de mesure
ci-dessus. Aucune autre régression de frontière CRDT/Go, admission, protocole, formats N40
ou parité annoncée n’a été identifiée dans ce périmètre. Les limites des six cibles restent
exposées, sans les déclarer satisfaites ; N22/N35/N37/N41 ne sont pas réputées livrées.

## Verdict

Les quatre remèdes demandés au tour 3 sont apportés au contrat ; les majeurs Fable et les
sept arbitrages ont des issues acceptables. L’acceptation globale reste impossible tant que
la voie de secours autorise le déchiffrement hors ligne sans le secret de déverrouillage.

**NOT ACCEPTED — bloqueur : R4-B1 (secret de secours local potentiellement en clair, contournant la protection du coffre).**
