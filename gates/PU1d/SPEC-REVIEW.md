# PU1d cross-family spec review (DeepSeek V4 Pro via OpenRouter, one-shot curated bundle, 2026-11-24; Gemini quota exhausted). Real cost 0.054 USD (26,062 prompt / 6,183 completion tokens). A first tool-using run (claude-or.sh, 0.04 USD) read 22 turns and returned no text.

Verdict : accepter avec corrections

1. [majeur, confiance haute] La règle P-16 exige que la connexion à fermer soit identifiée même en l’absence de flux. La spécification (spec.md: « qu’il nomme avant l’ouverture ») impose un choix préalable, mais le code actuel (stream.go: `on` peut être nil) n’y satisfait pas.  
   Fix : l’étape 2 doit systématiquement sélectionner une connexion (via `isBetterConn`) et la conserver comme cible de `close_conn` ; le modèle doit refléter cette obligation (par ex. un invariant `on ≠ NoConn` sur l’état d’ouverture).

2. [mineur, confiance moyenne] Le modèle omet la réinitialisation du flux par le répondeur sur une connexion qu’il ferme (§8.7.3). La trace d’une réinitialisation arrivant avant que l’ouvreur n’apprenne la fermeture pourrait être interprétée à tort comme un refus – sans conséquence grave sous P-16, mais non couverte par la preuve S6.  
   Fix : ajouter un commentaire dans CloseScope.tla signalant cette simplification, et s’assurer que le test nommé `TestFailedOpeningClosesOnlyItsConnection` couvre ce scénario.

3. [mineur, confiance haute] Un pair byzantin peut provoquer une boucle d’ouvertures en maintenant une connexion vivante, mais le nombre de connexions est borné à 2 en régime (spec.md §16.1), donc la boucle se résout en au plus 30 s. Le comportement n’est pas pire qu’avec `ClosePeer`.  
   Fix : aucun, la borne est acceptable.

4. [mineur, confiance haute] La spécification est globalement cohérente, mais l’expression « connexion restante » (P-17) gagnerait à être explicitée comme « toute autre connexion encore ouverte après la fermeture de celle de l’ouverture » dans la table de l’ouvreur (spec.md, ligne Fermé/pair connecté).

5. [mineur, confiance haute] Le modèle et ses mutants prouvent les règles sans vacuité (MC1 et MC2 sont capturés correctement, témoins violés). L’espace d’état (2 connexions, profondeur 34) est adapté à la limite de connexions du système.  
   Fix : aucun.