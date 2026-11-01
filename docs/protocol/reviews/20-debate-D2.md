# Débat D2 entre les relecteurs (tour 9, 2026-11-01) — accord

**Question.** Équité de l'attribution des emplacements de mise en attente (§8.1) : une entrée inactive ancienne peut-elle
attendre indéfiniment ?

**Position initiale de Fable (tour 9 : accept).** Un détenteur honnête connecté ne garde dans son emplacement qu'une
entrée qu'il sert lui-même, qui finit en temps borné ; suggestion : à la libération, réattribuer l'entrée inactive la
plus récemment avancée.

**Position initiale de Codex (tour 9 : revise).** Une entrée ancienne R peut rester inactive indéfiniment si, à chaque
libération, une nouvelle annonce prend l'emplacement avant elle ; aucune priorité anti-famine n'est prescrite.

**Après échange.** Fable : le scénario de Codex est réel, même avec un détenteur honnête qui produit sans cesse de
nouvelles modifications ; « la plus récemment avancée d'abord » affame les plus anciennes sous un flux d'annonces.
Codex : la promotion FIFO soutient le mieux l'objectif de complétude d'ADR 0028 tout en préservant l'isolement
byzantin, le DRR, la réservation atomique et la non-préemption.

**Accord.** À chaque libération (ou disponibilité à la connexion ou au redémarrage) d'un emplacement, le réserver
atomiquement pour l'entrée inactive éligible qui attend depuis le plus longtemps, avant toute nouvelle annonce ; l'ordre
d'attente (première annonce) est conservé à travers les réannonces et les redémarrages ; aucun transfert actif n'est
préempté ; plafonds, éviction des inactives et règle H inchangés. Vivacité garantie pour une entrée ayant au moins un
détenteur honnête, connecté et disponible ; attente bornée par le nombre d'entrées devant elle dans la file de ce
détenteur × un transfert de 16 Mio.

**Nuance arbitrée.** Fable proposait « la plus récemment avancée d'abord » au seul redémarrage ; Codex conserve l'ordre
d'attente aussi au redémarrage. Retenu : une règle unique (ordre d'attente partout), plus simple et sans famine — à
confirmer par les deux relecteurs au tour 10.
