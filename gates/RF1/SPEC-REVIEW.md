# RF1 cross-family spec and gate review, round 1 (DeepSeek V4 Pro via OpenRouter, one-shot curated bundle, 2026-11-24; Gemini quota exhausted). Real cost 0.016 USD (18,728 prompt / 10,090 completion tokens).

Verdict : accepter avec corrections

1. [majeur, confiance élevée] Le modèle TLA+ de l’admission utilise une fenêtre fixe (Tick recharge tous les jetons) avec B=2, alors que l’implémentation réelle utilise un *token bucket* (`golang.org/x/time/rate`) à burst 16 et recharge 1/5 s. La preuve de vivacité `MembersEventuallyUp` peut ne pas tenir : un attaquant pourrait maintenir le bucket vide en boucle, empêchant tout membre d’obtenir un jeton, malgré les propriétés vérifiées sur le modèle abstrait.
   - Évidence : plan §5 (« Rate: a fixed window abstracts the token bucket »), `RelayFanout.tla` lignes 30-31, `InOK` (l.99) et `Tick` (l.194). Le modèle ne distingue pas les effets de rafale longue.
   - Correction : étendre le modèle avec un *token bucket* (émission continue, burst max) ou prouver que l’abstraction est conservative ; ajuster les valeurs réelles si nécessaire.

2. [majeur, confiance moyenne] Le burst de 16 ouvertures par origine et le remplissage à 1/5 s (RF-5) permettent à un adversaire de saturer rapidement le quota par origine (8) et le quota global (32), puis de maintenir le déni de service très longtemps. Une fois l’attaque finie, les membres derrière cette origine ne récupèrent qu’une tentative toutes les 5 secondes, ce qui peut retarder la reconnexion au-delà des délais acceptables pour 64 membres.
   - Évidence : plan §3 RF-5, spécification §16.1, scénario RF1 résiduel accepté, mais la lenteur de reprise n’est pas quantifiée.
   - Correction : réduire le burst (ex. 8) ou augmenter le taux de recharge (ex. 1 / 1 s) pour limiter l’impact d’une inondation tout en permettant un rétablissement rapide.

3. [mineur, confiance moyenne] Le quota de connexions non vérifiées par origine est fixé à 8 (RF-3). Si 64 membres se connectent simultanément via le même relais, seules 8 tentatives entrantes peuvent coexister ; les autres sont refusées et doivent réessayer. Cela allonge l’établissement de la session complète, bien que tempéré par les reconnexions rapides.
   - Évidence : plan §3 RF-3, modèle `Flood` avec P=2, non représentatif à pleine échelle.
   - Correction : documenter que 8 est suffisant grâce au délai de 15 s et au parallélisme des reconnexions, ou bien l’augmenter à 16 pour tolérer une vague simultanée.

4. [majeur, confiance moyenne] La porte de régression (`regression`) n’exécute que `internal/transport/p2p` et `internal/app`. Le bloc modifie `node.go`, `relay.go` et le tableau du §16.1 ; un test instantané de la spécification (§16) situé hors de ces paquets (leçon du bloc PU1d) pourrait ne pas être couvert, laissant passer une régression.
   - Évidence : `gates/RF1.toml` critère `regression`, commande limitée à deux paquets.
   - Correction : étendre la couverture à tous les tests du module (`go test ./...`) ou au minimum inclure le paquet contenant le test de conformité de la spécification.

---

**Réponses aux questions ouvertes**

- **E (architecture)** : Placer le noyau de comptabilité dans `protocol/admit` est le bon choix (pureté, pas d’E/S, facile à fuzzer). Oui, il faut explicitement neutraliser le limiteur de débit de libp2p (`rate.Limiter` vide, `WithNetworkPrefixLimit` illimité) car les adresses sans IP partagent le même bucket, comme découvert par l’auteur.

- **Justification des nombres (B)** :  
  • 8/32 (quotas d’origine et global) sont justifiés et cohérents avec les menaces.  
  • 15 s (délai) est correct, aligné sur QUIC.  
  • **Burst 16 + 1/5 s** : le burst est trop élevé et le taux trop faible, comme relevé ci-dessus (nos constats 1 et 2).  
  • 168 (capacité totale) est justifié (somme des quotas).  
  • 64 par IP sur un relais public : justifié, correspond au nombre maximal de membres derrière un NAT.  
  • 4 relais au plus : choix de conception raisonnable (limitation des ressources relais).  
  • 168 Mio de fenêtre QUIC : compatible, reste en dessous de la limite de 256 Mio.