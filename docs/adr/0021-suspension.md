---
status: proposed
date: 2026-10-25
decision-makers: [Mathieu]
---

# Suspension : déconnecter et laisser revenir (opérations signées `suspend` / `resume`)

## Contexte et problème

B3 demande de couper une personne sans la retirer :
- les pairs refusent ses modifications ;
- « laisser revenir » accepte ce qu'elle a écrit entre-temps, avec le même rôle et le même rang.

Le retrait (ADR 0017) est définitif, et l'attestation (ADR 0018) ne connaît qu'« écrivain courant ou
non ». Il faut un état réversible qui remplisse trois conditions :
- toutes les répliques le décident de la même façon ;
- sans horloge de confiance ;
- sans que Go lise Automerge (ADR 0011).

## Facteurs de décision

- Convergence : l'état est une fonction de l'ensemble des opérations, jamais de l'ordre d'arrivée.
- Un suspendu ne peut ni antidater, ni se rétablir, ni rétablir un complice.
- Aucune perte : ce qu'il écrit reste chez lui et entre à la reprise.
- Pas de nouveau mécanisme d'écriture : celui de l'ADR 0018 suffit.
- Budget de 1024 opérations avant scellement (ADR 0017).

## Options envisagées

1. **Retirer puis réinviter.** La personne perd son rang, et il faut un nouveau code (contraire à
   ACL-03).
2. **Rétrograder en lecture puis rétablir.** Le lien n'est pas coupé, et cela mêle deux intentions.
3. **Registre de suspension hors de la liste.** Sans causalité avec la liste, un admin suspendu
   antidaterait librement.
4. **Deux nouvelles opérations de la liste signée, avec un état orthogonal au rôle.**
5. **Inscrire dans l'op les têtes du document au moment de la suspension**, et refuser ce qui n'en
   descend pas (rejet expliqué plus bas).

## Décision

Option 4.

**Format.**
- `OpSuspend = 5`, `OpResume = 6` : nouveaux kinds, `opVersion` inchangé.
- `Subject` ≠ `Author`, sinon `ErrMalformed` dès le décodage. Se suspendre ou se rétablir soi-même
  est donc impossible par construction (ACL-05).
- `Role`, `Person` et `Name` sont vides.
- Comme toute op, elles sont réservées aux admins (`everAdmin` à l'admission). Un écrivain qui forge
  une op reçoit `ErrUnknownAuthor` (ACL-04).

**Pli** (règles ajoutées à la doc du paquet `trust`).
- **S1 — portée.** Une suspension ou une reprise de y sur x compte si x remplit l'une de ces
  conditions :
  - x est sans rang ;
  - x est plus jeune que y ;
  - x est plus ancien mais n'était pas admin à cette op (même exception que `Remove`, règle 2).

  La créatrice admin et les admins plus anciens sont hors d'atteinte : l'op est stockée, sans effet.
- **S2 — état.**
  - On retient les suspensions et reprises *debout* sur x qui **voient** chaque `Remove` survivant
    de x posé par une clé plus ancienne (et non pas seulement celles qui le suivent dans ≺) : une
    suspension concurrente d'un retrait ne survit pas à la réadmission. S3(a) utilise le même
    ensemble de `Remove`.
  - Une reprise concurrente d'une suspension est écartée : une coupe bat une remise concurrente
    (règle 4).
  - x est suspendu si la ≺-dernière op restante est une suspension.
  - Un retrait suivi d'une réadmission repart non suspendu (ACL-07).
  - Une suspension d'un junior sur un senior (exception de S1) **tombe** dès qu'une attribution
    admin survivante, qu'elle ne précède pas, remet x admin : l'état et l'effet concordent (les op
    de x tiennent alors, et x n'est plus affiché suspendu).
- **S3 — effet sur la liste.**
  - La règle 3(a) exige en plus « y non suspendu à o », calculé sur ce que o voit.
  - La règle 3(b) traite une suspension debout de y, posée par une autre clé, comme une coupe : tout
    ce que y signe sans l'avoir vue est mort, même après la reprise. C'est la parade à l'antidatage.
- **Pas des op de rôle.** Rangs, rôles et `survivors` sont inchangés. Le balayage reste acyclique,
  car les op sur y viennent de clés décidées avant y. Les suspensions d'un junior sur un senior
  n'entrent que dans S2, jamais dans S3 : elles ne changent rien de ce que le senior décide.

**API.**
- Ajouts : `Member.Suspended`, `Member.SuspendedBy`, `Member.SuspendedOp` (hachage de la suspension
  qui tient), `Member.ResumedBy` (auteur de la reprise qui l'a levée), `List.Suspended(pub)`.
- `List.Role` rend le rôle conservé (ACL-03).
- `IssueWith` refuse :
  - un senior admin (`ErrOutranked`) ;
  - l'ajout d'un appareil à une personne dont un appareil est suspendu (`ErrSuspended`) ;
  - une attribution du rôle admin à un membre suspendu (`ErrSuspended`) : on le reprend d'abord ;
  - un auteur suspendu (`ErrSuspended`) : ce qu'il signerait serait mort ;
  - un départ (retrait ou rétrogradation de soi) s'il ne reste aucun autre admin **non suspendu**
    (`ErrLastAdmin`) : sinon le dernier admin actif partirait et la liste resterait figée.
    Contrôle local, au moment de l'émission. Le pli ne change pas (`verdict` compte toujours les
    admins suspendus) : un départ concurrent d'une suspension peut encore laisser seulement des
    admins suspendus ; la liste est alors figée, comme pour des départs concurrents (ADR 0017), et
    l'édition continue. Un changement du pli n'est pas nécessaire tant que la règle reste locale.
  - une op qui ne tiendrait pas (`ErrNotStanding`) : `IssueWith` et `IssueBatch` replient la liste
    avec l'op avant de la stocker, ce qui attrape un préfixe plafonné qui manque une op à voir.
- `capPrev` couvre toutes les suspensions **et reprises** debout de l'émetteur, et les op debout de
  **tous** les sujets d'un lot. Le tampon d'attente (`standingAdmin`) exclut les suspendus.

**Transport (`internal/app`).**
- Un suspendu reste admis à la connexion, mais en **lien restreint** :
  - seul le canal trust (0x02) passe ;
  - le dépôt (0x01), les attestations (0x03) et les blobs (ADR 0023) sont refusés dans les deux
    sens ;
  - le pont reçoit `PeerDown`.
- Le lien restreint suffit pour qu'il apprenne sa suspension (ACL-01) et sa reprise (ACL-03). Le
  budget par pair de `Sync` borne ses envois.
- Sur l'appareil suspendu :
  - un nouveau kind de pont, `Suspended` (Go → webview, avec l'auteur de l'op) ;
  - `attestHeads` garde les têtes dans `pending` et renvoie `ErrSuspended` ;
  - à la reprise, `flushPending` signe ces têtes et les diffuse.
- L'heure affichée (« à 10:02 ») est l'heure locale de réception de l'op : les op ne portent pas
  d'horloge (X-20). Elle est notée explicitement à la persistance, dans un fichier `<hachage>.rcv`
  à côté de `<hachage>.op` (jamais l'heure de modification du fichier), une seule fois, et
  retrouvée par `Member.SuspendedOp`. Rejoindre un projet note l'arrivée des op à ce moment-là.
- `Session.Suspend` et `Session.Resume` refusent une personne dont un appareil est celui-ci : un
  appareil frère qui a échappé à la suspension ne peut pas la lever.

**Contrôle d'écriture.**
- `List.Verify` renvoie `ErrSuspended`. `afterChange` élague alors les attestations du suspendu et
  émet `Revoked` (mécanisme existant).
- Règle qui en résulte : **une modification du suspendu est gardée si et seulement si l'attestation
  d'un écrivain courant non suspendu la couvre.**
  - « Avant » et « après » se lisent à la propagation. Ce qu'un écrivain avait accepté avant
    d'apprendre la suspension figure dans ses têtes : il l'atteste, et tous l'acceptent.
  - Le reste attend la reprise.
  - La couverture ne fait que croître, donc les répliques convergent (X-14).
- **Option 5 rejetée.** Une modification acceptée entre-temps par C a des descendants signés par C,
  qu'Automerge ne peut pas appliquer sans elle. La refuser bloquerait le travail honnête de C.
  Trancher une frontière stricte exige un consensus, que Keyhive juge aussi contraire au local-first.
- À la reprise, les modifications hors ligne deviennent couvertes et fusionnent (ACL-03). Celles qui
  touchent une zone verrouillée suivent l'ADR 0022 (X-13).

**Plusieurs appareils.**
- Une op par clé : « Déconnecter Bob » émet un lot (`IssueBatch`), refusé en entier si le lot
  scellerait la liste.
- Un appareil ajouté concurremment par un autre admin échappe à la suspension. L'interface le montre
  (« 1 appareil de Bob reste connecté »).

### Conséquences

**Bien :**
- réversible ; rang et personne sont conservés ;
- un admin suspendu ne peut rien signer d'utile, même en antidatant ;
- rien n'est perdu ;
- aucune nouvelle voie d'écriture.

**Moins bien, assumé :**
- **Liste figée avec des admins suspendus.** Si deux admins partent en même temps pendant que
  l'un d'eux suspend le dernier admin restant, il ne reste que des admins suspendus : la liste se
  fige (comme deux départs concurrents, ADR 0017) et ce dernier admin **reste suspendu pour
  toujours**, faute de quelqu'un pour le rétablir. L'écriture continue.
- **Fenêtre de propagation.** Un suspendu qui cache l'op à un écrivain isolé lui fait accepter ses
  modifications, donc les fait accepter par tous. Elles restent attribuées et réversibles
  (« Revenir ») ; l'escalade est le retrait.
- **Budget.** Chaque bascule consomme le budget de 1024 op : alerte au-delà de 900, confirmation
  demandée.
- **Scellement.** Un admin suspendu peut encore sceller la liste en l'inondant d'op mortes, comme un
  ancien admin (ADR 0017).
- **Visibilité.** Le lien restreint montre encore au suspendu les changements de la liste (noms,
  rôles).

### Confirmation

**Go, `internal/trust` :**
- référence naïve étendue (`reference_test.go`) et test de propriété ;
- `TestSuspend` couvre :
  - une reprise concurrente écartée (ACL-06) ;
  - l'op antidatée d'un admin suspendu, morte même après la reprise (ACL-12) ;
  - un senior ou la créatrice, sans effet (ACL-05) ;
  - se suspendre soi-même (`ErrMalformed`) ;
  - un écrivain qui forge une op (`ErrUnknownAuthor`, ACL-04) ;
  - une réadmission après retrait, non suspendue (ACL-07) ;
- `FuzzDecode` couvre les nouveaux kinds.

**Go, `internal/app` (memnet + synctest) :**
- lien restreint : aucune trame dépôt, attestation ou blob dans un sens comme dans l'autre ; trust
  passe (ACL-13) ;
- `Revoked` est émis à la suspension ;
- les têtes en attente sont signées à la reprise ;
- deux appareils (ACL-11) ;
- un lot qui scellerait la liste est refusé (ACL-14).

**TypeScript et e2e :** ACL-01, -02, -03, -05, -06, X-12, X-13, X-14.

**Netsim :** les 4 scénarios rejoués avec une suspension active.

**Attaques couvertes :**
- op forgée par un écrivain ;
- suspension d'un senior ;
- suspension de soi ;
- antidatage par un admin suspendu ;
- reprise par un autre appareil suspendu de la même personne ;
- rejeu d'une vieille reprise (≺ et S2) ;
- rejeu d'une attestation d'avant la suspension (`Verify`) ;
- inondation par le lien restreint (budget de `Sync`, tampon réservé aux admins debout).

## Plus d'informations

Amende les ADR 0017 (règles S1–S3) et 0018 (`ErrSuspended`).

Corrections de la matrice T11 :
- **ACL-06** : la suspension l'emporte sur une reprise concurrente.
- **X-14** : la frontière est la propagation, pas l'heure.
- **ACL-01** : le lien est restreint plutôt que coupé ; l'heure affichée est l'heure de réception
  locale.
- **ACL-05** : une op sur un senior est stockée sans effet ; seule l'op sur soi est refusée.

Sources :
- [Keyhive, note 01](https://www.inkandswitch.com/keyhive/notebook/01/) (antidatage ; consensus
  jugé « dernier recours ») ;
- [Kleppmann, Making CRDTs Byzantine Fault Tolerant](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf).
