# Revue confirmatoire n°2 (Sonnet) — architecture 8.3 (brouillon.12) / design 7.2

2026-11-02. Lecture seule. Périmètre : fermeture de R9-M1 à M5 (`17-fable-confirm.md`), de C1 à C7
(`18-codex-confirm.md`) et accord des deux spécifications. Aucune décision de l'utilisatrice rouverte.
Références : `A` = `docs/architecture/spec.md`, `D` = `docs/design/spec.md`, `AC`/`DC` = leurs CHANGES.
Gravités : bloquant (contrat contradictoire ou indéterminé), majeur, mineur (texte seul).

## 1. Fermeture, constat par constat

| Constat | État | Preuve |
|---|---|---|
| R9-M1 ⇧⌘K contre `deleteLine` | **Fermé** | D:3734 (⇧⌘K), D:3778–3786 (registre en `Prec.high`, quatrième table réservée `defaultKeymap`/`historyKeymap`/`searchKeymap`, ⇧⌘K listé « masquée sciemment »), D:3768–3782 (K-6 échoue sur une collision absente du tableau). Norme : CodeMirror 6, *Extension precedence*. |
| R9-M2 intégration avec validation obligatoire | **Fermé** | A:1989–1996 (VD-5 : « cette version validée exactement », VA-7), D:3133–3141 (relecture : « Vous intégrez la version … 2 modifications plus récentes ne sont pas intégrées »). Norme : ISO/IEC 25010:2023 exactitude fonctionnelle, N48. |
| R9-M3 gestes absents de la conception | **Fermé** | D:1432 (Ré-ancrer), 1421 (Masquer), 1445 (réglage lecteurs), 1449/3339–3340 (export), 1284–1286 (mise à jour, Intégrer depuis cette version), 1254/3360 (Garder toutes les variantes), 3120 (Soumettre inactif). Norme : D K-1, K-2. |
| R9-M4 borne globale de CM-6 | **Fermé** (réserve m3) | A:1901–1921 : bornes par modification, par acteur `(clé, epoch)` sur sa chaîne (≤ 8 192 modifications, ≤ 8 Mio), palier de chargement sans refus, plus de borne d'objets vivants ; A:3898–3903 (vecteurs `#comments-author-history`, `#comments-two-branches-over-16MiB`). Norme : protocole §9.7, §9.2. |
| R9-M5 pli de validation | **Fermé en partie** (voir B2) | A:1955–1982 : validité fonction de l'op et de ses frontières, 1.ʳᵉ décision sur la chaîne, « la décision l'emporte » sur un retrait concurrent, « ouvertes » à la frontière `reviews`, `assignee` membre, preuve propre A:3911–3915. |
| R9-m1 300 ms, activé par défaut | **Fermé** | A:3547–3562 (SY-4 « activé par défaut », SY-7 300 ms) ; D:1056–1060, 1080. |
| R9-m2 / C5 suivre dans une autre variante | **Fermé** | A:1634–1640 (§6.11), A:3604–3610 (RV-3), A:3639–3642 (RV-8) ; D:1267–1272. Accord avant tout changement, suivi suspendu, question redemandée. Norme : WCAG 2.2 SC 3.2.2. |
| R9-m3, C7 restes et chemin des polices | **Fermé** (réserve m1) | `fonts/` partout (A:1733, D:2928, 2999). Plus de « à confirmer » ni de « reste à écrire » sur variantes/commentaires (grep D). |
| R9-m4, m5 | **Fermé** | A:3895–3903 (étiquette `inkwell/doc-comments/v1`, clause d'ensemble, G11), A:1901 (couverture pour l'`état` d'autrui), A:1890–1893 (CM-4 têtes dans `meta.variant.depart`), A:1951 (CBOR déterministe), A:3907 (domaine exact). |
| R9-m6 ⇧F2 contre K-7 | **Fermé** | D:3734–3736 : cellule et texte partagent ⇧⌘K ; Maj+F2 sous Linux/Windows seulement. |
| R9-m7 ARIA | **Fermé** | D:1475–1478 (`EditorView.announce`), D:956–958/1240 (`aria-current`), D:3656 (A-18 avec la barre de suivi), D:650/1219/1237 (« Variante : … » unique). |
| C1 preuve d'équivoque PD-8 | **Fermé** | A:3911–3915 : `review_equivocation_v1`, deux ops complètes, même auteur et position, octets signés avec `ProjectID`, diffusion, gel ; deux positions différentes ne sont pas une équivoque. Norme : protocole §3.3, §4.7–4.8, §4.10. |
| C2 retrait contre décision | **Fermé** (réserve B2) | A:1970–1982 (VD-3) ; vecteurs `#withdraw-concurrent-decide`, `#permutations`. |
| C3 plafonds CM-6 | **Fermé** (réserve m3) | A:1901–1921, A:3903. |
| C4 première écriture concurrente | **Fermé** | A:1584–1595 et PD-6 A:3885–3893 : `DocId` dérivé `inkwell/doc-variant-file/v1`, état initial aux têtes du manifeste, même fourche pour tous ; vecteur `#variant-file-fork-concurrent-first-write`. |
| C6 « contesté » accessible | **Fermé pour le rendu** (voir B1) | D:3098–3113 : forme (losange), mot, nom accessible, deux décisions au clavier, annonce polie, ancienne coche remplacée ; A:1984–1996 (VD-5 : ne compte pas). Norme : WCAG 2.2 SC 1.3.1, 4.1.2, 4.1.3. |

## 2. Vérifications demandées

1. **Pli PD-8 déterministe** : presque. Validité fonction de l'op et du passé déclaré, une décision par soumission (plus petite `position`), égalité de position = équivoque donc gel. Il manque un ordre de précédence total entre états (B2).
2. **Limites de commentaires par auteur** : oui. Par acteur `(clé, epoch)` sur sa chaîne, plus quota N10 par clé (64 Mio pour un lecteur) (A:1906–1916). Réserve m3.
3. **Première édition concurrente en variante** : oui, une seule fourche (A:1584–1595, PD-6).
4. **Validation obligatoire intègre la version exacte** : oui (A:1989, D:3133).
5. **Suivi vers une autre variante avec accord** : oui, des deux côtés.
6. **« Contesté » accessible** : rendu oui ; déclencheur incohérent entre les deux specs (B1).
7. **300 ms, activé par défaut** : oui des deux côtés.
8. **Ordre des polices projet, inkwell, personnel, ordinateur** : oui (A:1732–1736 FT-1 ; D:2999–3001).
9. **⇧⌘K, `Prec.high`, documenté en K-6** : oui (D:3734, 3778–3786). Réserve m2.

## 3. Constats ouverts

### B1 — Bloquant : le déclencheur de « Validation contestée » diffère entre les deux specs

- **Preuve.** D:3098–3102 : contestée « quand la personne désignée a signé **deux décisions contraires** », nom accessible « validée le 3, refusée le 4 ». A:1972–1974 (VD-2) : une seule décision compte, « la première sur la chaîne » ; la suivante est « décision ultérieure sans effet » ; A:3914 : deux décisions contraires à des positions différentes ne sont **pas** une équivoque. A:1976–1980 : `contestée` = **fourche de chaîne** (deux ops à la même `position`).
- **Norme.** Cohérence entre specs (D P2, A:`coherence`) ; protocole §4.10 ; WCAG 2.2 SC 4.1.2 (le nom accessible doit dire l'état réel).
- **Effet.** Selon la conception, le cas « validée le 3, refusée le 4 » donne « contestée » ; selon l'architecture, il donne « validée » avec une note « sans effet ». Le test accessible et le garde VD-5 divergent.
- **Correction (texte).** Dans D:3098–3113, définir « contestée » comme une chaîne du signataire fourchée (deux appareils ont signé des décisions différentes à la même étape), pas comme deux décisions successives. Ajouter à D un rendu pour « décision ultérieure sans effet » et « retrait concurrent, sans effet » (A:1973, 1978). Reprendre les noms accessibles sur ces cas.

### B2 — Bloquant : VD-3 n'ordonne pas totalement les états

- **Preuve.** A:1976–1982 : l'état d'une soumission est tantôt « validée ou refusée si une décision valide existe », tantôt « contestée » (fourche), tantôt « retirée » ; la précédence entre `contestée` et `validée`/`refusée` n'est pas écrite. Une décision dont le passé déclaré contient déjà un retrait valide (A:1971 : `decide` valide si assignee et membre ; aucune clause sur un retrait vu) est aussi laissée à l'implémentation : validée, ou sans effet ?
- **Norme.** Protocole §4.9 (pli indépendant de l'ordre d'arrivée), §9.7 ; la correction attendue de C2 (« priorité déterministe »).
- **Effet.** Le résultat reste fonction de l'ensemble, mais deux implémentations honnêtes peuvent écrire deux ordres différents ; les vecteurs `#permutations` ne les distinguent pas.
- **Correction (texte).** Écrire la précédence en une ligne (par exemple `contestée` > `validée`/`refusée` > `retirée` > `soumise`, avec « contestée » limitée aux ops au-delà du point de gel), et trancher `decide` après un `withdraw` vu : sans effet, ou l'emporte (la règle « la décision l'emporte » devient alors générale). Ajouter les deux cas aux vecteurs.

### m1 — Mineur : restes périmés dans les deux dossiers

- D:1059–1060 (« L'architecture (SY-7 : 100ms ; SY-4 : « en option ») est à aligner ») et D:1272–1273 (« L'architecture §6.11 et RV-3 sont à aligner ») : l'architecture 8.3 est alignée (A:3547, 3562, 3604–3610). Retirer ces phrases.
- `design/coverage.md:188` : « Commenter ⌃⌘M » contredit D:3734 (⇧⌘K). `design/inventory.md:217` (G-102) : « dossier `polices/` ». DC:504–508, 514 : lignes ⌃⌘M et 100 ms marquées remplacées plus bas (DC:538–548) : acceptable, mais à épurer.

### m2 — Mineur : attribution erronée

- D:3777 écrit « Codex R9-M1 » ; R9-M1 est une constatation de Fable (`17-fable-confirm.md:40`). Même erreur dans `design/inventory.md` G-107.

### m3 — Mineur : le quota N10 n'est pas un prédicat d'admission

- A:1906 annonce des bornes « toutes stables par réunion », or A:1914 y range le quota N10 par clé, « cumulé, crédité après vérification » (protocole §8, §10.1). Un quota cumulé sur plusieurs `epoch` n'est pas une fonction de `before(c)` d'un seul acteur. Écrire que N10 est un plafond de ressource local (jamais un verdict d'admission), ou citer la part réservée qui le rend stable. Norme : protocole §9.7, §10.1.

## 4. Non-régression

Aucune modification de `docs/protocol/` ni de `docs/design/` hors des fichiers annoncés ; aucune décision utilisatrice rouverte (AD-100, AD-105 à 107 respectées). Les contrôles du tour précédent sans obstacle (N43 à N45, WebKitGTK, raccourcis) restent valables.

## Verdict

NOT ACCEPTED — blockers : B1 (déclencheur de « Validation contestée » contradictoire entre design 7.2 et architecture 8.3), B2 (précédence des états de VD-3 et `decide` après `withdraw` non écrites). Les deux sont des changements de texte ; tous les autres R9-M* et C* sont fermés.
