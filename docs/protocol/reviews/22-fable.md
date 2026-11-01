# Revue Claude Fable n°10 (tour 10, confirmation), révision 10 (2026-11-01) — verdict : accept

1. D2 appliqué exactement (§8.1, §16.1, P-BYZ) : réservation atomique pour l'inactive éligible de plus petit rang à chaque
   libération ou disponibilité (fin, abandon, connexion, redémarrage), avant toute nouvelle annonce ; rang de première
   annonce persisté, conservé aux réannonces et redémarrages ; aucune préemption ; hypothèse de vivacité et borne
   (rangs plus petits × transfert de 16 Mio) écrites.
2. Règle unique au redémarrage acceptée : les octets vérifiés sont persistés (reprise), donc « plus récemment avancée
   d'abord » ne gagnerait que de la latence au prix d'une seconde règle et d'un risque de famine — abandon confirmé.
3. Précédence d'éviction saine : les actives tiennent dans 64 × 16 Mio = 1 Gio, donc tous les octets inactifs sont
   retirables et une réservation dans l'emplacement d'un pair connecté réussit toujours ; finies → inactives sans
   détenteur connecté (LRU) → éligibles du plus grand rang au plus petit ; octets retirés, rang gardé ; borne d'attente
   inchangée.
4. Remarques 4 et 5 appliquées. Mineur non bloquant : la cellule §16.1 résumait l'éviction en « inactives en LRU » sans
   le troisième palier — alignée sur §8.1 après cette revue.
5. Rien de nouveau Critique ou Majeur : le diff de la révision 10 est confiné à §8.1, §16.1, CHANGES, P-BYZ et aux revues.

accept
