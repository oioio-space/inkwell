# Étude — l'IA « de premier rang » dans inkwell (N31)

> Statut : étude de recherche, **pas une décision** (deux ADR seront nécessaires : fournisseurs
> et politique IA ; exposition MCP). Date : 2026-11-01.
> Convention (comme `plugins.md`) : **[vérifié]** = lu ce jour dans la source citée ;
> **[mémoire]** = connaissance générale non revérifiée (à confirmer avant ADR) ;
> **[avis]** = jugement de l'auteur.
> Grille fixe appliquée à chaque décision : **Sé**curité · **Pr**ivacité · **Va**leur pour des
> écrivaines · **Co**ût/complexité · **In**variants (CLAUDE.md, spec §15). Notes : ++ / + / 0 / − / −−.

## 1. Résumé et recommandations

1. **Une seule API, quatre surfaces.** Le registre de commandes (§15.4, point `commands`) est
   la source unique : palette, greffons, **outils de l'agent IA** et **outils MCP** en sont
   quatre projections. L'IA n'a aucun droit qu'un greffon ne puisse pas demander (dogfooding,
   §15.1 point 2). Elle est une **extension intégrée** `ext/ai`.
2. **Une IA propose, une personne dispose.** Toute sortie qui touche un document est une
   **proposition** (`document.propose`, déjà au §15.5) : diff relisible, acceptation **par
   morceau**, appliquée en **une** modification Automerge annulable, signée par la personne qui
   accepte, étiquetée « via IA ». Aucun outil d'IA n'écrit directement.
3. **Fournisseurs : socle OpenAI-compatible** (`/v1/chat/completions`, flux SSE, appels
   d'outils) + adaptateurs natifs minces (Anthropic Messages, Responses). Les appels sortent
   **de Go** (la webview n'a pas de réseau), avec URL de base libre, CA interne, mTLS,
   clé ou jeton OIDC, proxy, zéro télémétrie. C'est ce qui sert le cas « DGX au travail »
   (NIM, vLLM, SGLang, Ollama, llama.cpp parlent tous ce dialecte).
4. **Politique IA par projet, signée** (journal de décisions, `protocol/policy`) : `off`,
   `local` (boucle locale), `org` (profils approuvés par l'admin), `libre`. Par défaut, dans un
   projet à plusieurs : **rien ne part vers un hôte distant sans décision d'admin**. Honnête sur
   la limite : c'est un contrôle de **conformité des clients honnêtes**, pas de
   confidentialité cryptographique (quiconque lit peut copier).
5. **Injection de prompt : on ne filtre pas, on borne.** Tout texte écrit par un pair est
   *non fiable* (Byzantin). Règle des deux + étiquetage de provenance : dès qu'un contexte
   contient du texte non fiable, l'agent **perd** les outils hors « proposer » (pas de réglages,
   pas de réseau, pas d'autre projet). Les détecteurs ne sont qu'un signal.
6. **« Discuter pour modifier l'appli »** = produire des **artefacts déclaratifs** validés
   (patch de réglages, jetons de thème, modèles de projet, invocation de commande du registre),
   avec consentement et annulation. Jamais d'exécution de code ; la génération de greffons
   (code) est différée.
7. **Serveur MCP** : par défaut un **shim stdio** (`inkwell mcp`) qui parle à l'instance
   en cours par socket local `0600` ; **Streamable HTTP** sur `127.0.0.1`, port éphémère,
   jeton en temps constant, validation d'`Origin`, en option. Portées : `lecture`, `proposition`
   (jamais d'écriture directe), `compilation`. Politique du projet appliquée avant tout.
8. **Dépendance** : SDK officiel `modelcontextprotocol/go-sdk` (épinglé, ADR 0008) pour MCP ;
   client de modèles **écrit en interne** (≈ `net/http` + analyseur SSE fuzzé), sans SDK
   fournisseur (poids, télémétrie, surface).
9. **Protocoles (§9–§11)** : socle OpenAI-compatible + serveur MCP restent le cœur ; **client ACP**
   (agents externes comme Claude Code/Codex/Gemini CLI, à la Zed) en **option P3**, sans `fs`
   ni `terminal`, l'agent recevant **notre serveur MCP** pour voir et proposer ; A2A, AG-UI,
   WebMCP, lsp-ai, MCP Apps : **ne pas adopter** (inspiration ou veille seulement) ; **UTCP** : pas
   de client ; au plus un **manuel exporté** (P3, §11.6), MCP restant l'interface de référence.
10. **Feuille de route** : P0 fournisseurs + propositions + commandes (reformuler, corriger) →
   P1 politique + marques d'autorat + panneau de discussion → P2 MCP stdio lecture/proposition →
   P3 MCP HTTP → P3 aussi : client ACP (option, politique admin) → P4 discussion « appli » + client MCP + génération de greffons + dictée locale.

## 2. Spécification MCP (état au 2026-11-01)

### 2.1 Révision courante

- Dernière révision : **2026-07-28** [vérifié, modelcontextprotocol.io/specification/latest].
  Précédentes : 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05.
- **Protocole sans état** : plus de poignée de main `initialize` ni de sessions ; chaque requête
  porte version et capacités du client dans `_meta.io.modelcontextprotocol/*` [vérifié,
  transports et streamable-http].
- Fonctions serveur : **ressources**, **prompts**, **outils**. Côté client : **élicitation**.
  Utilitaires : progression, annulation, erreurs. **Extensions optionnelles** (négociées) :
  *Tasks* (opérations longues), *Skills over MCP*, *MCP Apps* (UI interactive en ligne)
  [vérifié, specification/latest].
- **Échantillonnage (sampling), racines (roots) et journalisation sont dépréciés** à partir de
  2026-07-28 (le SDK Go les garde au moins 12 mois) [vérifié, README go-sdk]. Les échanges
  serveur → client (élicitation…) passent par des **allers-retours multiples** (MRTR,
  `InputRequiredResult`) et non plus par des requêtes serveur ; plus de flux GET [vérifié].
  Conséquence [avis] : ne **pas** bâtir sur sampling/roots ; l'élicitation sert, avec parcimonie,
  à demander une confirmation humaine côté client.
- Transports [vérifié] : **stdio** (sous-processus lancé par le client, JSON-RPC délimité par
  lignes) et **Streamable HTTP** (un POST par message, réponse JSON ou SSE limitée à la
  requête ; annulation = fermeture du flux ; `subscriptions/listen` pour les notifications
  longues). HTTP+SSE 2024-11-05 déprécié. Transports sur mesure permis (réutiliser le cadrage
  stdio sur socket Unix : exactement notre shim).
- **Sécurité du transport HTTP** [vérifié] : le serveur **DOIT** valider `Origin` (403 si
  invalide, contre le DNS rebinding), **DEVRAIT** n'écouter que sur `127.0.0.1`, **DEVRAIT**
  authentifier. Meilleures pratiques locales : privilégier stdio, sinon jeton ou socket Unix à
  droits restreints.

### 2.2 Autorisation

- Optionnelle ; pour HTTP : OAuth 2.1 (brouillon 13), **Protected Resource Metadata RFC 9728
  obligatoire**, indicateurs de ressource **RFC 8707** (`resource` dans autorisation et jeton),
  PKCE, validation de l'émetteur **RFC 9207** (anti « mix-up »), identification du client par
  **Client ID Metadata Documents** (DCR déprécié), élévation progressive de portée par
  `WWW-Authenticate scope=` [vérifié, basic/authorization].
- **stdio : la spec dit de NE PAS suivre ce flux** et de prendre les identifiants dans
  l'environnement [vérifié]. Notre jeton local (§5.3) n'est donc pas de l'OAuth : c'est permis
  (autorisation facultative) mais certains clients ne savent faire qu'OAuth [avis : hors v1 ; un
  mini serveur d'autorisation local serait un nouveau risque].
- Bonnes pratiques de sécurité [vérifié, basic/security_best_practices] : *confused deputy*
  (proxy OAuth à identifiant statique + DCR + cookie de consentement : consentement **par client**
  obligatoire), **passage de jeton interdit** (« MUST NOT accept or transit any other tokens »),
  SSRF (HTTPS, blocage des plages privées, redirections), détournement de poignée d'état,
  compromission d'un serveur local (afficher la commande exacte avant lancement, bac à sable),
  validation des URL d'autorisation (`javascript:` etc.), portées minimales.
- **Empoisonnement d'outils** : les descriptions d'outils et annotations sont **non fiables**
  sauf serveur de confiance [vérifié, specification/latest]. Pour inkwell : (a) comme **serveur**,
  nos descriptions sont statiques et compilées ; (b) comme **client** (P4), toute description
  d'un serveur externe est traitée comme du texte non fiable (§6).

### 2.3 SDK Go

`github.com/modelcontextprotocol/go-sdk` : « SDK Go officiel », maintenu avec Google, ≈ 5,3 k
étoiles ; **v1.7.0+ prend en charge la révision 2026-07-28** et les quatre précédentes ; paquets
`auth` et `oauthex` (primitives OAuth, RFC 9728) ; OAuth côté client *expérimental* ; licence
Apache-2.0 pour les nouvelles contributions, MIT pour l'existant [vérifié, README]. Transports
`StdioTransport`, `CommandTransport`, paquet `jsonrpc` pour les transports sur mesure [vérifié] ;
le gestionnaire Streamable HTTP n'a pas été confirmé dans la page lue [mémoire : existe,
`StreamableHTTPHandler` ; **à confirmer avant ADR**].
**Reco [avis]** : l'adopter (la spec bouge vite, la réécrire coûte plus que l'épingler) ; fuzzer
notre couche de décodage d'arguments d'outils (« tout octet extérieur est vérifié ») ; garder le
SDK derrière `internal/mcpserver` pour pouvoir le remplacer.

## 3. Points d'accès aux modèles

### 3.1 Le paysage

| Famille | Dialecte | Notes |
|---|---|---|
| NVIDIA NIM (DGX, auto-hébergé) | OpenAI Chat Completions ; URL de base à remplacer, ex. `http://hôte:8000/v1` [vérifié, docs Docker/NVIDIA] | Backends SafeTensors, vLLM, SGLang, TensorRT ; tous les modèles ne savent pas appeler des outils [vérifié] |
| vLLM | `/v1/chat/completions` (modèle de conversation requis), `/v1/responses`, `/v1/completions`, `/v1/embeddings`, `/v1/models` ; `parallel_tool_calls` [vérifié, docs vLLM] | Appel d'outils à activer côté serveur (options d'analyseur) [mémoire] ; clé et TLS en options du serveur [mémoire] |
| SGLang, TGI, llama.cpp `llama-server`, Ollama | OpenAI-compatible [mémoire] | Qualité d'appel d'outils très variable ; Ollama sert aussi un dialecte natif |
| Anthropic | Messages API (+ outils, flux SSE, mise en cache de prompt) [mémoire] | adaptateur natif utile (cache, blocs de réflexion) |
| OpenAI | Chat Completions et **Responses** [mémoire] | Responses est le dialecte en croissance |
| Mistral | compatible OpenAI [mémoire] | souveraineté européenne : argument pour les entreprises |

### 3.2 Abstraction recommandée

```
type Provider interface {                 // internal/ai/provider (Go, aucun import Automerge)
    Models(ctx) ([]Model, error)          // /v1/models ; sonde de capacités
    Chat(ctx, Request) (<-chan Event, error)   // texte, appel d'outil, usage, fin ; annulable
}
```
- **Socle** : `openai-chat` (flux + `tools`). Adaptateurs : `anthropic-messages`,
  `openai-responses`. Un « profil » est de la **donnée** (pas du code, donc pas un greffon) :
  `{nom, dialecte, base_url, ca_pem, mtls, auth, en-têtes, modèle(s), classe}` avec `classe` ∈
  `local | org | externe` (§5.1).
- **Sonde de capacités** à l'ajout du profil : appel d'outils ? sortie structurée ? fenêtre ?
  Si pas d'outils → mode **« texte seul »** (le modèle rend une proposition structurée en JSON
  validé) : la fonction reste utilisable sur un petit modèle local [avis].
- **Besoins d'entreprise** : URL libre ; **CA interne** ajoutée au `x509.CertPool` du profil
  (jamais `InsecureSkipVerify`) ; mTLS ; clé API **ou** jeton porteur obtenu par OIDC
  (flux navigateur système + PKCE, ou jeton fourni par la passerelle) ; secrets dans le
  trousseau de l'OS, jamais dans les réglages ni les journaux ; proxy (`HTTPS_PROXY` ou
  explicite, PAC hors v1) ; **zéro télémétrie** (le client interne n'émet rien d'autre que
  l'appel demandé) ; liste d'hôtes **fermée au profil** : le SSRF se règle par liste d'autorisation
  (une DGX est en adresse privée, donc le blocage des plages privées de la spec MCP ne s'applique
  pas au fournisseur choisi, mais s'applique à tout le reste).
- Où ? **Go**, pas la webview (CSP sans réseau, secrets hors DOM). Flux vers la webview par un
  Stream Wails dédié `ai` (aucun port, ADR 0009), borné et à contre-pression (§7).
- **Pas de SDK fournisseur** [avis] : poids, télémétrie par défaut possible, surface d'attaque ;
  l'analyseur SSE et la validation des appels d'outils sont courts et fuzzables.

### 3.3 Encaisser l'existant de la spec (§15.5)

Règle d'exclusion actuelle : un greffon avec `net.hosts` n'a aucune capacité de lecture en
clair. L'IA exige les deux (lire du texte, parler à un hôte). Solution : **le réseau reste en
Go, derrière un point d'extension `ai-providers`** (verbe *remplacer*, un actif par projet et par
appareil, comme `storage-backend`), dont les adaptateurs sont **compilés dans le cœur** ; un greffon
tiers n'obtient que la capacité `ai.complete` (envoyer **ce qu'il a déjà le droit de lire**,
via la politique et le consentement ; il ne voit ni clé ni hôte et n'a toujours pas `net.hosts`).
Ajouter `ai.complete` à l'analyse de la règle d'exclusion (**AD à ouvrir**).

## 4. État de l'art dans les éditeurs

| Produit | Ce qu'il fait de bien | Limite / leçon pour inkwell | Source |
|---|---|---|---|
| **Zed** (détail au §9) | Panneau d'agent ; agents externes via **ACP** (Claude, Gemini CLI, Codex…) ; *edit prediction* par **Zeta**, modèle ouvert, avec suppressions ; MCP et fournisseurs configurables dans le panneau | Orienté développeurs ; revue par morceau non vérifiée dans mes sources | zed.dev/docs/ai/* [vérifié] |
| **VS Code Copilot (mode agent) + MCP** | Confirmation **avant chaque appel d'outil** par défaut, désactivable par serveur ; « toujours » par conversation (autre client) ; 1.130 : approbations « assistées » où le modèle évalue le risque | Le modèle juge ses propres appels : pas une frontière de sécurité ; fatigue de confirmation | code.visualstudio.com/updates, blogues [vérifié, secondaire] |
| **Cursor, Continue.dev, JetBrains AI** | Diff en ligne accepter/refuser, fichiers d'exclusion (`.cursorignore`), modèles locaux/URL libre (Continue, JetBrains avec Ollama) | [mémoire] à revérifier avant citation |  |
| **Obsidian (greffons IA)** | Écosystème de greffons (Copilot, Smart Connections) ; modèles locaux possibles | Chacun fait son consentement ; aucun autorat | [mémoire] |
| **Notion AI, Google Docs (Gemini)** | Intégration native, « aide à écrire » | Données chez le fournisseur, politique d'organisation côté admin ; confidentialité mal lisible pour les collaboratrices | [mémoire] ; pages officielles non obtenues |
| **Overleaf AI Assist (Writefull)** | Aide LaTeX payante ; données non utilisées pour l'entraînement ; **indisponible pour les comptes gérés par une organisation ou si l'institution s'est retirée** | Preuve qu'un choix **institutionnel** est attendu | docs.overleaf.com/integrations-and-add-ons/ai-features [vérifié] |
| **iA Writer – Authorship** | Texte IA marqué (grisé, puis dégradé coloré en 7.3), auteurs humains en couleurs ; « Coller comme IA » manuel ; données d'autorat **écrites en fin de fichier** (visibles dans d'autres éditeurs) ; détection partielle | Autorat déclaratif, pollue le fichier, non signé | ia.net/writer/support/editor/authorship [vérifié] |
| **LanguageTool** | Correction **déterministe** (règles), serveur auto-hébergeable, soulignement + remplacement | [mémoire] ; à confirmer. Leçon : la correction n'a pas besoin d'un LLM | |

**Constats [avis]** : (1) personne ne relie IA, **autorat signé** et **politique de projet** ;
(2) la revue par morceau est la norme chez les développeurs, mais presque inexistante pour des
auteures ; (3) la correction « souligner → proposer → accepter » (modèle LSP/LanguageTool) est le
geste le plus familier pour des écrivaines : à privilégier sur le « réécris-moi tout ».

## 5. Conception pour inkwell

### 5.1 Politique et gouvernance des données

Problème : envoyer un document partagé à un fournisseur **partage le texte des autres**.
Qui consent ? Le texte appartient au projet, pas à la personne qui clique.

| Niveau | Qui décide | Défaut |
|---|---|---|
| Texte **privé** (projet solo, brouillons locaux) | la personne | libre, avec aperçu de ce qui part |
| Projet **partagé** | **admin**, par décision signée au journal (`protocol/policy`, comme le mode de suppression) | `local` seulement |
| **Fichier** | étiquette `ai: never` (liste `.inkwell/ai-ignore`), honorée par les clients honnêtes | néant |

Modes de politique : `off` · `local` (profils de classe `local`, boucle locale) · `org` (profils
**nommés** par l'admin : URL, empreinte de CA, classe `org`) · `libre` (chaque personne choisit).
Une décision est un événement signé avec effet **prospectif** ; les clients affichent « IA :
autorisée, fournisseur X (interne) » dans l'en-tête du projet et dans « Qui voit quoi » (aucune
fonction ne devine qui a utilisé l'IA : un **journal local** par appareil, et un compteur
facultatif partagé).
- **Aperçu de charge utile** : avant le premier envoi d'une session, « voici exactement le
  texte qui part, et vers où » (plage, nom du fournisseur, classe). Le glossaire dit
  « personne », pas « pair » : « l'IA de votre entreprise », pas « endpoint org ».
- **Limite assumée [avis]** : en menace Byzantine, un membre lisant le document peut le copier
  ailleurs ; la politique ne protège pas contre lui. Elle (a) empêche l'accident côté clients
  honnêtes, (b) donne une base de responsabilité (décision signée, journal), (c) rend le
  hors-politique visible (documents modifiés « via IA non autorisée » → marqueur). À écrire
  tel quel dans la doc (zéro surpromesse).
- **Sur site** : profil `local`/`org` pointant sur la DGX ; aucune sortie hors du réseau ; la
  liste d'hôtes du profil est fermée ; mode « tout est local » = politique `local` + pare-feu :
  c'est **l'argument de vente** pour les entreprises.
- Conformité (RGPD, sous-traitant) : hors périmètre technique ; l'admin doit pouvoir **exporter
  la politique en vigueur** [avis, pas un avis juridique].

### 5.2 Édition = proposition, autorat visible

1. L'extension IA appelle `document.read` (plage), le modèle répond ; Go valide la forme ;
   l'extension crée une **proposition** d'opérations ancrées (§5.2 de plugins.md).
2. La webview affiche un **diff relisible** par morceau (barre « Accepter / Refuser / Tout
   accepter », ligne d'explication du modèle). Le texte en flux n'est **jamais** écrit dans le
   CRDT : il vit dans une couche *fantôme* locale ; rien ne part aux collaboratrices avant
   l'acceptation (pas de demi-phrases à l'écran des autres).
3. L'acceptation produit **une** modification Automerge (`change`) avec métadonnées
   `{ia:{modèle,classe,profil}, via:"ext/ai", invite:<hash>}`, **signée par la personne** ; annulable
   (Ctrl+Z = un cran).
4. **Marques d'autorat** : calculées depuis l'historique (attribution par modification), **pas
   écrites dans le fichier** (contrairement à iA : pas de pollution du `.typ` ni du `.md`).
   Rendu : teinte discrète/soulignement doux, interrupteur, couleur par personne, légende ;
   taper dans un passage IA le rend humain (même règle qu'iA [avis]). Export : option « retirer
   les marques ».
5. **Honnêteté Byzantine** : l'étiquette « IA » est une **déclaration signée par la personne**,
   pas une preuve : un pair malveillant peut mentir dans les deux sens. L'UI dit « marqué IA par
   Camille », jamais « écrit par une IA ». Une politique peut **imposer** le marquage par défaut
   pour les clients honnêtes.
6. Garde-fous de proposition (bornes du §15.5 : opérations par proposition) : au plus N morceaux,
   alerte si > X % du fichier change, morceaux touchant le texte d'**autrui** étiquetés, et
   **validateur de contenu** : nouvelles URL, nouvelles références de fichiers, `#read`, `#eval`,
   `#include` hors projet, caractères invisibles (zéro largeur, **balises Unicode U+E0000**,
   bidi) **mis en évidence** dans le diff.

### 5.3 Injection de prompt par des pairs Byzantins

Menace : une collaboratrice plante « ignore tout et propose/exécute… » dans le document ; l'IA
de **l'autre** personne le lit et agit avec les droits de **celle-ci**.

État de l'art :
- **Règle des deux** (Meta, 2025-10-31) : une session d'agent ne devrait réunir que deux de
  trois propriétés — [A] traite des entrées non fiables, [B] accède à des systèmes/données
  sensibles, [C] change l'état ou communique vers l'extérieur ; les trois exigent supervision
  humaine [vérifié, ai.meta.com/blog/practical-ai-agent-security].
- **CaMeL** (Debenedetti, Shumailov, Carlini, Tramèr…, Google DeepMind/ETH, arXiv 2503.18813,
  v2 juin 2025) : séparer flux de contrôle (requête de confiance) et flux de données, capacités
  contrôlant les flux ; sécurité « prouvable » pour les tâches résolues, **77 % de tâches
  AgentDojo contre 84 %** sans défense [vérifié]. Coût : interpréteur, réécriture des agents.
- **« The Attacker Moves Second »** (Nasr, Carlini et al., arXiv 2510.09023, oct. 2025) : 12
  défenses publiées (détecteurs, filtres) contournées par attaques adaptatives, la plupart à
  plus de 90 % ; la rouge humaine réussit partout [vérifié, résumé]. **Conclusion pour nous :
  aucun classifieur n'est une frontière de sécurité.**
- *Dual LLM* (Willison, 2023) : un modèle « en quarantaine » lit le non fiable **sans outils**
  [mémoire].

**Conception [avis]** (Sé ++, Co 0) :
1. **Provenance par intervalle** : chaque intervalle de texte porte l'étiquette `soi` (écrit par
   cette personne, signature locale) ou `autrui` (tout autre acteur, y compris éditeurs : les pairs
   sont Byzantins), `externe` (ressource MCP, fichier importé, collage). Calcul depuis les
   acteurs Automerge, côté webview (Go reçoit des octets déjà étiquetés par intervalles).
2. **Réduction dynamique des droits (CaMeL allégé)** : tant que le contexte contient du `autrui`
   ou `externe` → catégories d'outils *lecture du fichier courant* + *proposer* seulement ;
   **retirés** : autres projets/fichiers hors contexte, réglages, greffons, commandes à effet,
   réseau, outils MCP clients. Elle est **appliquée par Go et la webview**, pas par le prompt.
3. **Mode sans outil** pour « reformuler/résumer/traduire » du texte d'autrui : fonction pure
   texte → proposition (quarantaine) ; c'est 80 % de la valeur pour des écrivaines.
4. **Classes d'outils** : `lire` (auto, dans la portée) · `proposer` (auto, mise en file, revue
   humaine = frontière) · `réglages/greffons/thèmes` (**confirmation modale** avec l'effet
   exact) · **destructifs ou sociaux** (supprimer, inviter, changer la politique, partager, clés)
   : **non exposés à l'IA du tout**. Aucune exécution automatique d'un outil qui écrit hors
   proposition.
5. **Canaux d'exfiltration** : pas d'image/lien distant rendu dans la discussion ; liens
   nouveaux signalés (§5.2 point 6) ; pas de `net.*` pour les outils.
6. **Descriptions d'outils MCP externes** (P4) : texte `externe`, épinglées par empreinte,
   changement → reconfirmation ; serveur nommé et lancé avec la commande exacte affichée.
7. **Fatigue de confirmation** : peu de dialogues, mais au bon endroit (réglages, pas
   proposition) ; le diff est la confirmation.
8. Tests : corpus d'injections en `fuzz/` + propriétés « aucune séquence d'outils depuis un
   contexte `autrui` n'atteint une classe retirée » (test de propriété, pas de théâtre).

### 5.4 Discuter pour modifier l'application

Portée (tout via l'API de greffons, avec consentement) :

| Demande | Artefact produit | Contrôle |
|---|---|---|
| « Passe en sombre, police plus grande » | **patch de réglages** validé par les schémas `settings` | aperçu ; un bouton Annuler ; historique de réglages |
| « Thème plus chaleureux » | **jetons de thème** (contrat §15.8) | validation des contrastes (≤ 20 ms), aperçu |
| « Crée un modèle de rapport trimestriel » | **modèle de projet** (N27) : fichiers Typst + métadonnées | revue du diff, création dans un dossier |
| « Ouvre le plan / lance la compilation » | **invocation d'une commande du registre** (nom + arguments typés) | classe d'outil, §5.3 |
| « Fais un greffon qui… » | greffon **déclaratif** (données seulement, sans code : snippets, grammaire, thème) ; greffon **code** différé | installation comme un greffon : capacités en phrases claires, **non signé, local, d'origine IA**, sans `net.hosts` |

Jamais : shell, `eval`, écriture hors projet, modification du noyau fermé (§15.3), changement de
politique. Génération de greffons en code : exige une chaîne de compilation fournie (aucun
téléchargement à l'exécution) → **différé** [avis].

### 5.5 Serveur MCP d'inkwell

**Contrainte clé** : Go n'importe pas Automerge ; le texte vit dans la webview. Le serveur MCP
(Go) est donc un **relais** : chaque outil devient un appel au registre de commandes via le pont ;
la webview applique portées et politique. Sans fenêtre ouverte : lecture seule de la projection
go-git (plus ancienne, signalée comme telle) ; pas de proposition possible.

| Option | Sé | Pr | Va | Co | In | Reco |
|---|---|---|---|---|---|---|
| A. `inkwell mcp` (stdio) → socket Unix `0600` / canal nommé de l'utilisateur → instance en cours | ++ | ++ | + | 0 | ++ (pas de port) | **défaut** : les clients (Claude Desktop, IDE) lancent un sous-processus |
| B. Streamable HTTP `127.0.0.1`, port éphémère, jeton comparé en temps constant, jamais journalisé, **`Origin` validé** | + | + | ++ (clients distants locaux, conteneurs) | − | + (conforme à l'invariant) | optionnel, **désactivé par défaut**, jeton affiché/copié depuis l'UI et écrit dans un fichier `0600` |
| C. HTTP + OAuth 2.1 complet | − | 0 | + | −− | − (serveur d'autorisation local) | hors v1 |

Outils et ressources (noms de commandes du registre, **schémas JSON stricts**, bornes de taille) :

| Portée | Ressources | Outils |
|---|---|---|
| `lecture` | `inkwell://projet/fichiers`, `…/document/{id}` (plage), plan, commentaires, diagnostics, état de compilation | `plan`, `rechercher`, `lire_plage`, `diagnostics`, `commentaires` |
| `compilation` | aperçu (PDF/SVG), `typst query` | `compiler` |
| `proposition` | file de propositions | `proposer_modification` (anchorée) → **file de revue humaine**, étiquetée « via <nom du client MCP> » |

- **Jamais** : écriture directe, suppression, partage, invitation, réglages, politique, clés,
  accès hors du projet ouvert. `prompts` : modèles (« relire le chapitre », « résumer »).
- **Politique** : `ia.mcp: off | lecture | proposition` décidée par l'admin pour les projets
  partagés, par la personne pour les siens ; **nom du client affiché** et consentement **par
  client** (leçon *confused deputy*) à la première connexion ; jeton **lié au serveur**, aucun
  jeton tiers n'est accepté ni relayé (*token passthrough* interdit).
- Les sorties MCP contiennent du texte `autrui` : l'IA **cliente** est hors de notre contrôle ;
  nous n'exposons donc que ce qu'un greffon avec les mêmes capacités verrait, et nous **étiquetons**
  la provenance dans les métadonnées de ressource (`_meta`) pour les clients qui la respectent
  [avis].
- Élicitation : pour confirmer côté client une opération sensible (rare) ; pas de sampling.
- Limites et budget : requêtes/s, taille de plage, 1 proposition en attente par client ; `PERF-*`
  mesuré ; fuzz des arguments.

### 5.6 Performance et expérience

- **Flux** : Go lit le SSE et pousse des fragments par Stream Wails ; la webview les regroupe par
  image d'animation dans la couche fantôme ; **annulation** par `context` (fermer la requête HTTP
  coupe le calcul chez vLLM/NIM [mémoire]) ; aucun appel sur le fil d'affichage ; 1 requête
  active par document ; temps maximal, taille de contexte plafonnée.
- **Affichage** : premier mot (TTFT), durée, jetons, **coût estimé** (table de prix fournie par
  l'utilisateur ou l'admin ; « gratuit — votre machine/entreprise » pour `local`/`org`), jauge de
  contexte (« l'IA verra 3 pages »).
- **Hors ligne** : fonction grisée avec la raison (« le serveur de votre entreprise ne répond
  pas »), jamais d'erreur technique ; la correction déterministe (§5.7) reste disponible.
- **Complétion en ligne** (type Zeta) : désactivée par défaut, phase ultérieure, modèle local.

### 5.7 Correction à la LanguageTool, via `diagnostics`

La correction (orthographe, grammaire, style) est un **fournisseur de diagnostics**
(`diagnostics` + action rapide), alimenté (a) par règles déterministes (dictionnaires, serveur
LanguageTool auto-hébergé) et (b) par une « relecture IA » qui rend des diagnostics **ancrés** et
non un texte libre. Même modèle mental que les erreurs de compilation (N28) : souligner → expliquer
→ proposer → accepter. Le chemin déterministe fonctionne sans IA ni réseau.

## 6. Grille de décision

| Décision | Sé | Pr | Va | Co | In | Verdict |
|---|---|---|---|---|---|---|
| IA écrit directement dans le CRDT | −− | − | + | + | −− | **rejeté** |
| IA propose, revue par morceau, change signé | ++ | + | ++ | 0 | ++ | **retenu** |
| Socle OpenAI-compatible + adaptateurs natifs | + | ++ | ++ | + | ++ | **retenu** |
| Natif uniquement (un fournisseur) | + | − | 0 | ++ | + | rejeté (pas de DGX) |
| Politique par projet signée + classes de profil | ++ | ++ | + | 0 | ++ | **retenu** |
| Consentement individuel seul | − | −− | ++ | ++ | + | rejeté (partage le texte des autres) |
| Filtre/détecteur d'injection comme défense | −− | 0 | 0 | 0 | 0 | **rejeté** comme frontière ; signal seulement |
| Provenance + réduction dynamique des droits | ++ | + | + | 0 | ++ | **retenu** |
| MCP stdio via socket local | ++ | ++ | + | 0 | ++ | **retenu** (défaut) |
| MCP HTTP loopback + jeton | + | + | ++ | − | + | **optionnel** |
| Autorat dans le fichier (iA) | 0 | − | + | + | −− | rejeté ; dérivé de l'historique |
| Discussion « appli » par artefacts déclaratifs | + | + | ++ | − | + | **retenu** ; code généré différé |

## 7. Intégration à la spec et ADR à prévoir

- §15.4 : ajouter les points `ai-providers` (remplacer, Go, intégré), `ai-tools` (contribuer :
  un greffon peut apporter des outils ; ce sont des commandes du registre étiquetées
  `agent:true`), `ai-surfaces` (panneau de discussion, actions de sélection).
- §15.5 : capacités `ai.complete`, `ai.tools` (**classe de l'outil** déclarée, montrée en
  phrase : « laisser l'IA lancer cette action ») ; mise à jour de la règle d'exclusion.
- §7.1 : un `Stream` `ai` (codec borné, vecteurs, fuzz) ; §6 : décision `ai.policy` au journal,
  borne de taille, effet prospectif ; protocole : `kind` de décision versionné.
- ADR 0040 (fournisseurs, politique, provenance) ; ADR 0041 (exposition MCP) ; ligne N31 de
  `exigences.md` : 🔨 → liens vers ce document ; **AD à ouvrir** : (1) `ai.complete` × règle
  d'exclusion ; (2) calcul de l'autorat depuis Automerge sur gros historiques (compaction) ;
  (3) client MCP sortant (P4) ; (4) OAuth local pour clients qui n'ont que cela ;
  (5) confirmation de l'API `StreamableHTTPHandler` du SDK ; (6) obligation légale d'information
  des collaboratrices (hors technique) ; (7) génération de greffons en code.
- Tests : propriété « proposition acceptée = une seule modification annulable » ; convergence
  TypeScript avec modifications « via IA » ; fuzz décodeur SSE et arguments d'outils ; corpus
  d'injections ; `ux-walkthrough` sur le diff par morceau et l'aperçu de charge utile.

## 8. Extension (2026-11-01, à la demande de N31)

Les §9 à §11 étendent l’étude : Zed et ACP, catalogue exhaustif des protocoles, combinaison retenue.

## 9. Zed en profondeur (ce que Zed a fait, et ce qu'on en retient)

### 9.1 Agent Client Protocol (ACP)

- **Objet** : standardiser éditeur ↔ agent de code, « comme LSP l'a fait pour les langages » ;
  réutilise les représentations JSON de MCP, ajoute des types propres à l'édition (diffs) et du
  Markdown pour le texte [vérifié, agentclientprotocol.com/overview/introduction].
- **Transport** : JSON-RPC 2.0 sur **stdio** (l'agent est un sous-processus de l'éditeur) ;
  agents distants en HTTP/WebSocket : *« travail en cours »* ; un RFD « Streamable HTTP &
  WebSocket » et un groupe de travail Transports existent [vérifié, llms.txt].
- **Cycle** [vérifié, protocol/overview] : `initialize` (capacités) → `authenticate` éventuel →
  `session/new` (ou `session/load`) → `session/prompt` ; l'agent diffuse des `session/update`
  (fragments de message/pensée, `tool_call` et `tool_call_update`, plans, commandes
  disponibles, modes, compactage) et finit par une raison d'arrêt ; `session/cancel` interrompt.
  Méthodes **côté client** : `session/request_permission` ; optionnelles `fs/read_text_file`,
  `fs/write_text_file` (capacités `fs.*`), `terminal/*` (capacité `terminal`),
  `elicitation/create`. Chemins absolus, lignes à partir de 1, extensions par `_meta` et
  méthodes préfixées `_`.
- **Appels d'outils** [vérifié, protocol/tool-calls] : `kind` ∈ `read, edit, delete, move,
  search, execute, think, fetch, switch_mode, other` ; états `pending → in_progress →
  completed | failed` ; contenu = texte/image/ressource, **diff** (`path`, `oldText`, `newText`)
  ou **terminal** ; `locations` = affichage seulement, **pas un droit d'accès**.
- **Permissions** [vérifié] : l'agent envoie des options `allow_once | allow_always |
  reject_once | reject_always` (indications d'interface ; **l'agent possède** la portée et la durée
  de ce qu'il retient) ; le client répond `selected` ou `cancelled`. Consigne de sécurité : l'agent
  n'agit que sur ce qui est approuvé et redemande si la cible change.
- **Maturité** : protocole **v1 stable**, **v2 en brouillon** ; SDK Kotlin, Java, Python, Rust,
  TypeScript (Rust et TypeScript en 1.0.0) ; **pas de SDK Go dans la liste lue** ; Registre ACP
  « stabilisé » ; RFD sur les chaînes de proxys ; gouvernance communautaire avec mainteneur en
  chef, groupes de travail [vérifié, llms.txt]. Licence : non lue ce jour [mémoire : Apache-2.0].
- **Adoption** : créé par Zed (août 2025), rejoint par JetBrains, registre commun annoncé en
  janvier 2026 ; Neovim (CodeCompanion, avante), Emacs (agent-shell), VS Code en greffons
  communautaires ; agents : Claude (via l'adaptateur `claude-agent-acp`), Codex, Gemini CLI,
  OpenCode, Copilot, Cursor, Goose, OpenHands, Poolside… [vérifié pour la liste de Zed
  (docs/ai/external-agents) ; **secondaire et contradictoire** pour le reste : à revérifier].
- **Ce que Zed ne fait pas pour vous** : chaque agent gère sa connexion, sa facturation, ses
  données ; « Zed ne facture rien » et renvoie aux conditions du fournisseur ; profils et
  permissions de l'agent natif de Zed **ne s'appliquent pas** aux agents externes ; Zed relaie ses
  serveurs MCP à l'agent par ACP, mais l'agent peut lire sa propre configuration MCP [vérifié].
  Zed installe les agents depuis le registre (paquets npm/PyPI/GitHub) : **téléchargement à
  l'exécution, interdit chez nous**.

### 9.2 Le reste du panneau d'IA de Zed

| Élément | Constat [vérifié, zed.dev/docs/ai/*] | À reprendre |
|---|---|---|
| Revue des changements | accordéon « fichiers modifiés » → multi-tampon où l'on accepte/refuse **par morceau** ou tout ; option de revue en ligne ; **« Restaurer le point de contrôle »** avant chaque message | notre proposition par morceau ; Ctrl+Z = un cran ; « annuler cette discussion » = défaire les changes « via IA » de la session |
| Suivre l'agent | icône qui fait suivre les fichiers touchés | « suivre » = défilement vers le morceau en cours (couche fantôme) |
| Permissions d'outils | `always_allow / always_confirm / always_deny` par outil, **regex**, précédence : **règles intégrées > deny > confirm > allow > défaut outil > défaut global** ; protections codées en dur non contournables ; outils MCP adressés `mcp:<serveur>:<outil>` ; défaut `confirm` | même précédence ; « règles intégrées » = nos **classes non exposables** (§5.3) ; adresses `mcp:` pour le client MCP (P4) |
| Profils | un profil restreint les outils (natifs + MCP) d'un fil | nos **modes** : Corriger (sans outil), Discuter (lecture + proposer), Réglages (artefacts) |
| `@`-mentions | fichiers, symboles, diagnostics, URL, autres fils | `@` = sélection, plan, diagnostics, commentaires ; provenance affichée |
| Serveurs de contexte (MCP) | bloc `context_servers` : commande locale (`command/args/env`) ou **URL + en-têtes**, OAuth MCP si pas d'en-tête ; seuls **outils et prompts** sont pris en charge ; recharge à `tools/list_changed` | confirmer la commande exacte (§2.2) ; pas de sampling |
| Fournisseurs | Zed, clés d'API, abonnements, **passerelles**, modèles locaux ; `openai_compatible` : `api_url`, `available_models` (`max_tokens`), **`capabilities`** `tools` (oui), `images`, `parallel_tool_calls`, `prompt_cache_key`, `chat_completions` (non pour modèles « Responses seulement »), `reasoning_effort` ; `anthropic_compatible` (+ `prompt_caching`, `custom_headers`) ; clés dans le **trousseau**, variable d'environnement prioritaire | notre profil reprend ces drapeaux, **sondés** plutôt que saisis |
| Ollama, LM Studio | découverte automatique des modèles ; Ollama : `num_ctx` **4096 par défaut** | piège à signaler : contexte tronqué en silence ; régler `num_ctx` ou utiliser l'API native |
| Désactiver l'IA, confidentialité | réglage global ; page de confidentialité et « données d'entraînement » en opt-in | interrupteur général **et** politique de projet (§5.1) |

### 9.3 Prédictions d'édition (Zeta)

- Pas de protocole exotique : Zed envoie à un serveur **OpenAI-compatible `/v1/completions`**
  un prompt dont le format dépend de `prompt_format` (variantes Zeta, CodeLlama, StarCoder,
  DeepSeek, Qwen, Codestral, GLM, Sweep ; `infer` déduit du nom du modèle) ; `max_tokens` 256,
  température 0,2, séquences d'arrêt ; fournisseurs : **Zeta (défaut)**, Copilot (jeton
  d'appareil, `enterprise_uri`), Codestral, Mercury, Ollama, serveurs personnalisés (vLLM,
  llama.cpp, LocalAI) ; modes **subtil** (touche maintenue) ou **empressé** ; `disabled_globs`
  par chemin [vérifié, docs/ai/edit-prediction].
- **Zeta2** : réentraîné, +30 % d'acceptation, ≈ 100 000 exemples **opt-in** de dépôts sous
  licence ouverte ; poids ouverts sur Hugging Face, mais **jeu de données non publié** à cette
  échelle (Zeta1 : jeu et poids ouverts, Apache-2.0) ; base Seed-Coder-8B [vérifié, blog Zed ;
  licence des poids Zeta2 : source tierce, **à vérifier**].
- **Pour inkwell** [avis] : un petit modèle local de continuation de phrase est plausible (8 B
  tient sur un portable), mais pour de la prose il faut un autre jeu de données ; l'**interface**
  (fantôme grisé, `Tab` pour accepter, mode subtil, exclusion par motifs) est à copier, le
  modèle non. Désactivé par défaut ; chemin `ai: never` → jamais envoyé.

## 10. Catalogue des protocoles et interfaces

| Interface | Objet | Transport | Maturité / gouvernance (source) | Pertinence pour inkwell |
|---|---|---|---|---|
| **MCP** 2026-07-28 | agent ↔ outils, données, prompts | stdio ; Streamable HTTP (POST) | spécification datée stable ; confiée à l'**Agentic AI Foundation** (Linux Foundation) en déc. 2025 [secondaire] | **Retenu** : serveur (§5.5) ; client en P4 |
| **MCP Apps** (SEP-1865, extension `io.modelcontextprotocol/ui`) | UI interactive dans la conversation : ressources `ui://` HTML rendues dans un **iframe sandbox**, sans réseau direct | ressources MCP | SEP **finale** (dépôt `ext-apps`) [secondaire ; spécification non lue] | **Ne pas exposer** (v1). Plus tard : afficher une App MCP dans `ui.canvas` (iframe `sandbox`, sans `allow-same-origin`, CSP sans réseau : déjà notre modèle) |
| **UTCP** (Universal Tool Calling Protocol) | alternative à MCP : un fournisseur publie un **manuel JSON** (`utcp_version`, `tools[]` avec schéma d'entrées et `tool_call_template`) ; l'agent appelle l'outil **directement** par son transport natif, sans serveur-enveloppe | modèles d'appel : `http`, `cli`, `sse`, `http_stream`, `websocket`, `grpc`/gNMI, `graphql`, `tcp`, `udp`, `webrtc`, `mcp`, `text` (liste du SDK Go) ; découverte typique `GET /utcp` ; conversion depuis OpenAPI [vérifié, utcp.io, go-utcp] | docs v1.1 (manuel 1.0.1) ; **organisation GitHub communautaire**, aucune fondation ni document de gouvernance, processus RFC ; SDK Python (654 étoiles), TypeScript (122), **Go (127, Go ≥ 1.25)**, pas de Java listé ; code **MPL-2.0**, spécification Apache-2.0 ; aucun hôte majeur nommé dans mes sources [vérifié] | **Pas de client ; manuel exporté facultatif** (§11.6) |
| **ACP** (Zed) | éditeur ↔ agent de code | JSON-RPC stdio ; HTTP/WS en RFD | v1 stable, v2 brouillon, registre, SDK 1.0 Rust/TS [vérifié] | **Option P3** : client (§11) |
| **A2A** (Agent2Agent) | agent ↔ agent opaques, délégation de tâches (Agent Card, tâches) | JSON-RPC, gRPC, REST [mémoire] | donné à la Linux Foundation (juin 2025), **Apache-2.0**, v1.0, SDK Python/JS/Java/.NET/Go/Rust, TSC AWS/Cisco/Google/IBM/Microsoft/Salesforce/SAP/ServiceNow, passé dans l'AAIF (août 2026, sources secondaires) [vérifié : a2a-protocol.org] | **Non** : inkwell n'est pas un agent ; aucun besoin inter-organisations. Réévaluer si un agent d'entreprise ne parle qu'A2A |
| **ACP d'IBM** (Agent Communication Protocol) | REST entre agents (BeeAI) | HTTP/REST | **fusionné dans A2A** (annonce fin août 2025), dépôt archivé [secondaire] ; ne pas confondre avec ACP de Zed | Sans objet |
| **AG-UI** | agent ↔ interface : flux d'événements (jetons, outils, **état partagé** par diffs, **interruptions** humaines) | HTTP, WebSocket [vérifié, vague] | ouvert ; adopté par LangChain, CrewAI, Microsoft Agent Framework, Google ADK, AWS Strands, Mastra, Pydantic AI… ; licence non lue | **Inspiration** : notre Stream `ai` Go → webview a le même rôle ; en reprendre le **vocabulaire** (delta de texte, début/fin d'outil, delta d'état, interruption). Pas de dépendance |
| **OpenAI Chat Completions** | génération + outils + flux SSE | HTTPS `/v1/chat/completions` | **norme de fait** (vLLM, NIM, Ollama, LM Studio, llama.cpp, Gemini en bêta partielle) [vérifié partiel] | **Socle** |
| **OpenAI Responses** | éléments typés, outils intégrés, état possible | HTTPS `/v1/responses` | pris en charge par vLLM [vérifié] ; certains modèles ne marchent qu'ainsi (Zed : `chat_completions:false`) | adaptateur P2 |
| **Open Responses** | spécification **neutre** dérivée de Responses, schéma OpenAPI + tests d'acceptation, sans état par défaut | HTTPS | **très récent**, projet ouvert à charte ; Open WebUI expérimental, Portkey [secondaire] | **Veille** ; si vLLM/NIM l'adoptent, remplace l'adaptateur Responses |
| **OpenAI Realtime** | voix en direct | WebRTC, WebSocket, SIP | GA ; modèles `gpt-realtime-*` (2026) [secondaire, docs Azure] | **Non** pour v1 : audio vers un nuage = vie privée. Dictée locale/entreprise : `/v1/audio/transcriptions` (vLLM, modèles ASR [vérifié]) en P4 |
| **Anthropic Messages** | blocs, outils, cache de prompt, flux | HTTPS | stable [mémoire] ; « compatibles Anthropic » chez des passerelles (Zed : `anthropic_compatible`) [vérifié] | adaptateur natif P0/P1 |
| **Gemini API** | `generateContent`, Live API ; couche OpenAI **bêta, partielle** (chat et embeddings) [secondaire] | HTTPS | stable [mémoire] | via couche OpenAI si besoin ; pas d'adaptateur natif en v1 |
| **Ollama, LM Studio, llama.cpp, vLLM, NIM, SGLang** | exécution locale ou sur site | HTTP (OpenAI-compatible ; Ollama aussi natif) | courant [vérifié : Zed, vLLM, Docker/NVIDIA] | **cible principale** « DGX » ; sonde de capacités, `num_ctx` |
| **LSP-based AI** (`lsp-ai`) | serveur LSP qui appelle des LLM (complétion, actions) pour tout éditeur LSP | LSP stdio | ≈ 3 100 étoiles, **projet inactif** d'après SourcePulse [secondaire] | **Non** ; l'idée (actions de code = diagnostics + correctifs) est déjà dans notre `diagnostics` (§5.7) |
| **WebMCP** (W3C) | des **pages web** exposent des outils à des agents du navigateur (`modelContext`) | API navigateur | brouillon de groupe communautaire (au 2026-10-20), essai d'origine Chrome seulement, nom de l'API instable (navigator → document) [secondaire, contradictoire] | **Non** : webview WebKitGTK/WKWebView/WebView2, pas Chrome ; ne pas exposer d'outils depuis la page (surface d'injection). L'équivalent chez nous est le registre de commandes |
| **Agent Skills / `AGENTS.md`/règles** | consignes persistantes pour agents | fichiers | `AGENTS.md` hébergé par l'AAIF [secondaire] ; Zed : règles, skills | **Règles de projet** pour écrivaines : charte de style, glossaire (§11.4) |

## 11. Combinaison recommandée pour inkwell

### 11.1 Évaluation selon la grille

| Option | Sé (pairs Byzantins) | Pr | Va (écrivaines) | Co | In (serveurs locaux, sans téléchargement) | Verdict |
|---|---|---|---|---|---|---|
| Socle OpenAI-compatible + natifs Anthropic/Responses (Go) | + | ++ | ++ | + | ++ | **P0** |
| Serveur MCP (stdio shim, HTTP loopback option) | ++ / + | ++ | + | 0 | ++ | **P2/P3** |
| **Manuel UTCP exporté** (sans client) | + (le serveur applique les droits) | ++ | 0 | ++ (≈ 100 lignes, généré du registre) | + | **P3, option** (§11.6) |
| Client UTCP (appels directs) | −− (modèles d'appel = ordres exécutables ; `cli`, `tcp`, `udp`) | − | 0 | − | − | **écarté** |
| **Client ACP** (agents externes) | − (sans bac à sable) / 0 (avec) | − : l'agent a son propre fournisseur | 0 : agents faits pour le code | − : v2 en brouillon, pas de SDK Go, sous-processus tiers | − : le registre télécharge ; on exige une **commande fournie par la personne** | **P3, option**, politique admin, désactivée par défaut |
| Client MCP sortant (outils d'entreprise) | − : descriptions non fiables | − | + (dépôt documentaire de l'entreprise) | − | + | **P4**, réduit par la règle des deux |
| Voix : dictée locale ou d'entreprise | + | + si locale | + (accessibilité) | 0 | ++ | **P4** |
| AG-UI | 0 | 0 | 0 | − | 0 | **vocabulaire seulement** |
| A2A, ACP d'IBM, WebMCP, `lsp-ai`, Realtime nuage | — | — | — | — | — | **écartés** (raisons §10) |
| MCP Apps | − (iframe, réseau via l'hôte) | 0 | 0 | − | + | **différé** |

### 11.2 Pourquoi un client ACP malgré tout

Il permet à une personne ou à une équipe de **brancher son propre agent** (Claude Code, Codex,
Gemini CLI…) comme dans Zed, sans que nous écrivions de pilote par modèle ; utile aux auteures de
modèles Typst, de greffons et de documentation technique (N27, N29), moins à une romancière. Mais
l'agent est un **processus tiers à pleins droits** (réseau, disque, propre fournisseur) : il sort
de notre bac à sable, de nos classes de profil et de la politique IA (§5.1). Donc : **option,
décision d'admin par empreinte de la commande, jamais installé par nous**, jamais en tâche de fond.

### 11.3 Comment les demandes de permission ACP se traduisent chez nous

Principe : inkwell ne **déclare ni `fs` ni `terminal`** dans ses capacités de client. L'agent
reçoit, dans `session/new`, **notre serveur MCP** (stdio de session ou HTTP loopback à jeton
**propre à la session**, révoqué à la fin) : il lit et propose par nos outils (§5.5), pas par le
disque. L'espace de travail de l'agent est un **dossier vide** (ou un miroir en lecture seule),
dans un bac à sable de l'OS quand c'est possible (bubblewrap/Landlock, Seatbelt [mémoire] ; **non
garanti partout : résiduel assumé**, signalé dans l'interface).

| Message ACP | Traduction inkwell |
|---|---|
| `initialize` | capacités client minimales ; nom et version de l'agent (info d'implémentation) → étiquette « via <agent> » |
| `session/new` | crée une **session IA** : portée = projet ouvert, provenance initiale `soi`, jeton MCP de session, mode (Corriger/Discuter) |
| `request_permission`, `kind: read/search` | accordé automatiquement **dans la portée MCP** de la session ; sinon refus ; texte `autrui` lu ⇒ session **marquée contaminée** |
| `kind: edit/delete/move` ou `fs/write_text_file` (non annoncé) | **jamais d'écriture directe** : l'édition arrive par `proposer_modification` ; si l'agent émet un **diff** (`path`, `oldText`, `newText`), on le convertit en **opérations ancrées** ; si `oldText` ne correspond plus (texte modifié entre-temps) → proposition périmée, redemande ; suppression/déplacement : **refusés** (classe non exposable) |
| `kind: execute`, `fetch`, `other` | **refusé** par défaut (pas de terminal, pas de réseau pour les outils) ; en session contaminée : refus imposé par Go |
| `kind: switch_mode` | ignoré ou limité aux modes que nous définissons |
| `allow_always` / `reject_always` | `allow_always` devient **« pour cette session »** seulement et **n'existe jamais** pour une classe réglages/greffons ; `reject_always` honoré. L'agent « possède » la portée en ACP, nous **ne lui déléguons pas** notre décision [avis] |
| `session/update` `tool_call` avec `diff` | alimente la **couche fantôme** (aperçu) ; **rien** n'entre dans le CRDT avant « Accepter » |
| `session/cancel` / fermeture | coupe le sous-processus (`context`), proposition en cours abandonnée |
| `elicitation/create` | boîte de dialogue Adwaita habituelle, texte `externe` (jamais interprété) |

**Lien avec nos capacités** : une session ACP est une **extension intégrée virtuelle** `ext/acp:<agent>`
dont le manifeste est fixé par nous : `document.read` (plage), `document.propose`,
`compile.read` ; **aucun** `net.hosts`, `export.write`, `import.read`. Les droits sont montrés en
phrases (« ce programme pourra lire vos documents et proposer des modifications »), accordés **par
projet et par appareil**, et la règle des deux s'applique à la session. Les propositions
**sont les mêmes** que celles des greffons : même file, même diff, même change signé « via
<agent> » ; l'autorat montre la personne qui a accepté.

### 11.4 Compléments tirés de Zed

- **Règles de projet** (`.inkwell/ai-rules.md` : ton, glossaire, interdits). Fichier éditable par
  les éditeurs ⇒ **vecteur d'injection** : il n'a force de consigne que s'il est **signé par un
  admin** (décision au journal) ; sinon c'est du texte `autrui`.
- **Permissions** : même précédence que Zed, avec en tête nos classes intégrées non
  contournables ; adresses `mcp:<serveur>:<outil>` et `acp:<agent>:<kind>`.
- **Journal des échanges** (comme « ACP logs » de Zed) : fenêtre de diagnostic locale des messages
  agent ↔ inkwell, sans secrets ni texte de documents si la personne le demande.
- **Complétion en ligne** : fournisseur distinct (`/v1/completions`), `ai: never` par motifs.

### 11.5 Points à ouvrir (AD)

(8) SDK Go pour ACP ou client maison + vecteurs du schéma v1 ; (9) ACP v2 : attendre la
stabilité avant P3 ; (10) bac à sable de sous-processus multiplateforme (deb/rpm, macOS) ;
(11) empreinte de commande et mise à jour d'un agent installé par la personne ; (12)
conversion `oldText/newText` → opérations ancrées sur texte concurrent ; (13) licence de
l'ACP et des poids Zeta2 ; (14) statut MCP Apps/ext-apps ; (15) Open Responses vs Responses.
(16) UTCP : attendre un hôte majeur ou une fondation avant tout client ; licence MPL-2.0
du SDK Go (ADR 0008) ; confirmer le modèle d'appel `mcp` sur stdio dans la spécification.

### 11.6 UTCP face à MCP

**Vérifié** [utcp.io, utcp.io/utcp-vs-mcp, dépôts de l'organisation, lus le 2026-11-01] : UTCP décrit
comment appeler des outils **existants** par leur transport natif, sans serveur intermédiaire ;
discovery par un manuel JSON ; auth déclarée dans le manuel (clé d'API, OAuth, variables
`${ENV}` résolues par le client) et **réutilisant l'auth du fournisseur** ; le modèle d'appel `mcp`
permet d'appeler aussi des serveurs MCP, et `utcp-mcp` est un serveur MCP qui expose des
points UTCP (**les deux mondes s'interconnectent**). Les comparaisons de utcp.io sont de la
documentation partisane (« surface de sécurité minimale », « pas de point unique de défaillance »)
et ne traitent ni le consentement ni les grandes listes d'outils. Le SDK Go dit lui-même :
« traitez les fournisseurs CLI comme des intégrations privilégiées » (commandes fixes, arguments
validés, environnement restreint, isolation OS). Une critique indépendante note que la sécurité
repose sur la configuration de chaque outil [secondaire, feedbagel.com].
Le souvenir de la coordination est donc **confirmé**, avec deux nuances : (i) l'écosystème est
petit et sans gouvernance formelle ; (ii) le catalogue de transports est plus large que MCP
(`tcp`, `udp`, `webrtc`, `grpc`), ce qui élargit la surface.

**(a) Exposer les outils d'inkwell à des IA externes**

| Critère | MCP | UTCP | Pour inkwell |
|---|---|---|---|
| Point d'accès | serveur MCP (stdio via socket local, HTTP loopback) | il faudrait **un point d'accès natif** à décrire : or inkwell n'a **aucun** serveur d'API (le pont est un Stream Wails, sans port, ADR 0009) | l'avantage « sans enveloppe » **disparaît** : il faudrait écrire le même serveur local |
| Droits et consentement | côté serveur : jeton, portées, file de propositions, politique | **identique** si le point d'accès applique tout ; le protocole n'a pas de crochet de consentement | MCP non plus n'en impose (« ne peut pas être imposé au niveau du protocole », §2) ; **la médiation reste chez nous** dans les deux cas |
| Jeton local | `Authorization` en-tête ou stdio sans OAuth | `auth` du manuel avec `${INKWELL_TOKEN}` lu dans l'environnement du client | pas de conflit **si** le manuel ne contient jamais le jeton (référence seulement) ; l'environnement est hérité par les processus enfants (fuite), moins sûr que le fichier `0600` ou le socket |
| Port éphémère | oui | le manuel doit être **régénéré** à chaque lancement (port, jeton) → fichier `0600` | gérable |
| Sémantique | ressources, prompts, élicitation, progression, annulation, abonnements | outils seulement (appel + flux SSE) | perte des ressources (plan, commentaires) et de l'élicitation |
| Portée | Claude Desktop, IDE, agents : SDK officiels (Go v1.7+ pour 2026-07-28) [vérifié] | clients UTCP rares dans mes sources | **MCP gagne** |
| Simplicité | un SDK, un protocole | un manuel JSON simple, mais N transports à sécuriser | égalité côté lecture, MCP côté sûreté |

**(b) L'IA d'inkwell qui appelle des outils**

- Les outils **internes** sont des commandes du registre appelées **en processus** : ni MCP ni UTCP
  (aucun transport) ; la médiation (classes d'outils, provenance, règle des deux) est dans l'hôte.
- Outils **externes** (entreprise) : avec UTCP, **l'hôte est le client** : il médie donc chaque appel ;
  le contournement de la médiation n'existe que pour un agent **externe** appelant inkwell (cas a).
  Mais le risque change de nature : un **manuel** n'est pas seulement une description (comme une
  fiche d'outil MCP, déjà non fiable) : ses `tool_call_template` sont des **ordres exécutables**
  (URL, méthode, corps, **commande `cli`**, `tcp`, `udp`). Un manuel empoisonné ou modifié par un pair
  peut donc diriger l'hôte vers n'importe quel hôte ou programme, là où MCP restreint à des
  appels `tools/call` vers un serveur déjà approuvé.
- Traduction en capacités (§15.5) : `http|sse|websocket|grpc|graphql|tcp|udp|webrtc` → réseau
  (`net.hosts` : **hôtes pris dans une liste signée par l'admin, jamais dans le manuel**) ;
  `cli` → **processus : aucune capacité n'existe** (« ni processus », §15.5) ⇒ refusé ; `text` →
  lecture de fichier ⇒ refusé. Avec contexte `autrui`, la règle des deux retire de toute façon
  tout réseau (§5.3). L'interface de consentement devrait en outre montrer **ce que fait
  l'appel** (méthode, hôte, corps) pour des gabarits hétérogènes : plus difficile qu'avec un
  nom d'outil et un schéma.
- Conclusion : si un besoin d'entreprise exige UTCP, l'ajouter **comme une source d'outils de plus
  derrière le client MCP/registre de P4**, avec manuels signés par l'admin, hôtes figés, classes
  de risque déclarées par nous, sans `cli`/`tcp`/`udp`/`webrtc`.

**Un manuel UTCP à côté du serveur MCP, à bas coût ?** Oui, **en export seulement** : une commande
`inkwell utcp-manual` génère un JSON **depuis le registre de commandes** (mêmes schémas JSON que
les outils MCP) et l'écrit dans un fichier `0600`. Deux variantes, aucune ne crée de nouveau
serveur : (1) modèle `mcp` pointant sur la commande stdio `inkwell mcp` (nécessite de confirmer
dans la spécification que ce modèle sait lancer un MCP stdio [mémoire]) ; (2) modèles `http`
vers le point HTTP loopback **déjà optionnel** (§5.5, option B), avec `${INKWELL_TOKEN}`
en référence. Mêmes portées, même politique, mêmes propositions : l'IA externe n'obtient rien de
plus. Coût estimé : ≈ 100 lignes Go + test de non-divergence avec le registre ; **risque** :
dérive de sémantique entre deux descriptions (d'où le test) et une promesse de support de plus.

**Recommandation** [avis] : **MCP reste l'interface unique** de référence (P2/P3). **Pas de client
UTCP.** **Manuel exporté : P3, facultatif, désactivé par défaut**, seulement si une personne ou
un client réel le demande ; à réévaluer si un hôte majeur ou une fondation adopte UTCP. Raisons :
portée (MCP), absence de serveur à « ne pas écrire » chez nous, sécurité (manuels exécutables,
transports larges), gouvernance (communautaire, sans fondation), licence (MPL-2.0 du SDK Go).

## 12. Sources

- MCP, spécification 2026-07-28 : https://modelcontextprotocol.io/specification/latest
  (transports : …/basic/transports ; Streamable HTTP : …/2026-07-28/basic/transports/streamable-http ;
  autorisation : …/basic/authorization ; sécurité : …/basic/security_best_practices) — lu le 2026-11-01.
- go-sdk : https://github.com/modelcontextprotocol/go-sdk — lu le 2026-11-01.
- Meta, *Agents Rule of Two* (2025-10-31) : https://ai.meta.com/blog/practical-ai-agent-security/
- CaMeL : https://arxiv.org/abs/2503.18813 ; *The Attacker Moves Second* :
  https://arxiv.org/abs/2510.09023
- vLLM, serveur compatible OpenAI : https://docs.vllm.ai/en/latest/serving/online_serving/
- NVIDIA NIM (URL de base remplaçable, dialecte OpenAI) : https://docs.docker.com/ai/docker-agent/providers/nvidia/
- Zed : https://zed.dev/docs/ai/edit-prediction.md , https://zed.dev/docs/ai/agent-settings.html
- iA Writer, Authorship : https://ia.net/writer/support/editor/authorship , https://ia.net/topics/see-what-ai-wrote
- Overleaf, fonctions IA : https://docs.overleaf.com/integrations-and-add-ons/ai-features
- VS Code, mode agent : https://code.visualstudio.com/updates/v1_99 (le reste de la ligne VS Code : sources secondaires, à revérifier).
- **Non vérifiés ce jour [mémoire]** : API Anthropic/OpenAI/Mistral, SGLang/TGI/Ollama/llama.cpp,
  Cursor, Continue, JetBrains, Obsidian, Notion, Gemini, LanguageTool, *dual LLM*. À sourcer
  avant toute citation dans une ADR.

### Sources ajoutées (§9–§11), lues le 2026-11-01 sauf mention
- ACP : https://agentclientprotocol.com/overview/introduction ,
  https://agentclientprotocol.com/protocol/overview ,
  https://agentclientprotocol.com/protocol/tool-calls , https://agentclientprotocol.com/llms.txt
- Zed : https://zed.dev/docs/ai/overview , …/external-agents , …/agent-panel , …/tool-permissions ,
  …/mcp , …/use-api-access , …/use-a-local-model , …/edit-prediction ; Zeta2 :
  https://zed.dev/blog/zeta2 ; Claude via ACP : https://zed.dev/blog/claude-code-via-acp
- A2A : https://a2a-protocol.org/latest/ ; AG-UI : https://docs.ag-ui.com/introduction
- MCP Apps (SEP-1865) : https://modelcontextprotocol.io/seps/1865-mcp-apps-interactive-user-interfaces-for-mcp
- Open Responses : https://www.openresponses.org/ ; WebMCP : https://web-platform-dx.github.io/web-features-explorer/features/navigator-modelcontext
- `lsp-ai` : https://github.com/SilasMarvin/lsp-ai (état : SourcePulse, secondaire)
- Fusion IBM ACP → A2A, AAIF : sources secondaires (itsfoss.com, flowhunt.io, softwareseni.com) ;
  Realtime : docs Azure (learn.microsoft.com/azure/foundry/openai/how-to/realtime-audio-websockets).
- **Secondaire ou contradictoire, à revérifier avant ADR** : dates d'adoption d'ACP par JetBrains,
  statut des adaptateurs Claude/Codex, dates de la fusion IBM/A2A et de l'entrée dans l'AAIF,
  nom et statut de l'API WebMCP, licence d'ACP et des poids de Zeta2.
- UTCP : https://www.utcp.io/ , https://utcp.io/utcp-vs-mcp , https://github.com/universal-tool-calling-protocol (dépôts `utcp-specification`, `go-utcp`, `python-utcp`) ; critique indépendante : https://feedbagel.com/post/utcp-a-scalable-secure-alternative-to-mcp-for-ai-tool-calling (secondaire). **Non lu** : sections de sécurité de la spécification UTCP, numéros de version des SDK, adoption réelle (aucun chiffre d'usage trouvé).
