# Revue Codex n°10 (tour 10, confirmation), révision 10 (2026-11-01) — verdict : accept

Majeur du tour 9 fermé : [§8.1](../spec.md:1036) applique D2 partout, connexion/redémarrage compris : FIFO avant annonces, rang persisté, sans préemption ; hypothèse et borne d’attente explicites.
Éviction sûre sous cette hypothèse : [§8.1](../spec.md:1058) prescrit la précédence demandée, conserve le rang et réserve atomiquement ; actives ≤ 64 × 16 Mio, total ≤ 1 Gio.
Mineur : [§16.1](../spec.md:1547) résume encore toutes les inactives par « LRU », contrairement au détail du §8.1.
Aucune nouvelle Critique/Majeur détectée ; [P-BYZ](../../../docs/adr/0034-protocole-unifie.md:178) couvre R et l’éviction ; gates, P-COMPAT et activation conjointe L/H conservés, sans préjuger leur validation.
accept
