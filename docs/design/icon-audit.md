# Audit des icônes d'Inkwell

## 1. Inventaire des artefacts visuels (frontend/src)

| Emplacement (fichier:ligne) | Rôle visuel | Source / Type | Style (stroke / fill) | Grille / Taille | Épaisseur |
|---|---|---|---|---|---|
| `ui/icons/paths.ts:5-73` (53 icônes) | Commandes, navigation, état | `ICONS` (`Icon.svelte`) | stroke `currentColor` (sauf `more`/`ul` cercles fill) | 16×16 (rendu 12–16px) | 1.6 (exceptions 1.3 à 2.2) |
| `ui/icons/paths.ts:80-91` (10 types) | Types de fichiers | `FILE_MARKS` (`FileIcon.svelte`) | outline neutre + marque colorée | 16×16 (rendu 16px) | 1.6 (marque 1.1 à 1.7) |
| `ui/history/HistoryRow.svelte:88` | Graphe DAG de l'historique | SVG `<svg class="graph">` | stroke & fill dynamiques | variable × 28px | 2px |
| `ui/search/SearchPanel.svelte:412` | Bouton fermer la recherche | Glyphe Unicode `×` (U+00D7) | texte (`currentColor`) | sans (13px) | sans |
| `ui/search/ResultsTree:111`, `SearchPanel:499` | Chevrons de section | Glyphe Unicode `›` (U+203A) | texte (`currentColor`) | sans (12px) | sans |
| `ui/sheet/SheetStatus:96,128,137` | Options cochées | Glyphe Unicode `✓` (U+2713) | texte (`currentColor`) | sans (12px) | sans |
| `ui/sheet/SheetStatus:117,148` | Sélection d'option radio | Glyphe Unicode `●` (U+25CF) | texte (`currentColor`) | sans (10px) | sans |
| `ui/sheet/SheetStatus:164,166` | Zoom moins / zoom plus | Glyphes Unicode `−` (U+2212), `+` | texte (`currentColor`) | sans (12px) | sans |
| `ui/toolbar/spec.ts:124`, `BarButton:47` | Bouton formule mathématique | Glyphe Unicode `∑` (U+2211) | texte Serif (`currentColor`) | sans (16px) | sans |
| `ui/sheet/Sheet.svelte:525` | Préfixe barre d'expression | Glyphes Unicode `ƒx` (U+0192) | texte Serif italique | sans (13px) | sans |
| `editor/changeGutter.ts:124-135` | Marqueur ligne supprimée | Triangle CSS (`border-left`) | border `var(--change-color)` | 7×10px | sans (remplissage bord) |
| `HeaderBar:262,278`, `StatusBar:205`, `SheetStatus:204` | Non enregistré / synchronisation | Puces CSS (`border-radius: 50%`) | fill `var(--fg)`, `var(--success)` | 8×8px | sans |
| `ui/people/PeopleDialog.svelte:302` | Statut connecté | Puce CSS (`::before`) | fill `var(--success)` | 8×8px | sans |
| `theme/adwaita.css:97` | Flèche de popover | Carré CSS pivoté 45° | fill `var(--popover-bg)` | 14×14px | sans |
*(Aucune image d'arrière-plan via `url()` n'est présente dans le frontend).*

## 2. Origine des tracés ICONS et conformité des licences

- **Origine :** Faits à la main pour le mockup Adwaita (`paths.ts:2`, git `dabca15`), complétés par commits (`98c3b4b`, `86af030`).
- **Jeu d'appartenance :** Dessins maison (hand-made) imitant Adwaita, sans dépendance externe. Licence Inkwell : MPL-2.0 (compatible).
- **Ensembles de référence et licences :**
  - *GNOME Icon Development Kit :* Licence CC0 1.0 (domaine public). Aucun droit restrictif, compatibilité parfaite avec MPL-2.0.
  - *adwaita-icon-theme :* Double licence LGPL-3.0-or-later / CC-BY-SA-3.0 (ou 4.0). Exige attribution et copyleft sur modifications.
  - *libadwaita (icônes embarquées) :* Licence LGPL-2.1-or-later.
  - *Lucide (ISC) / Material (Apache-2.0) :* Permissives, mais dessin étranger à GNOME.

## 3. Sources officielles GNOME et réutilisation webview

- **GNOME HIG - UI Icons :** https://developer.gnome.org/hig/guidelines/ui-icons.html. Exige une grille de 16×16 px, un trait nominal de 2 px (pas de trait de 1 px) et un rendu monochrome recolorable via `currentColor`.
- **Application Icon Library :** https://apps.gnome.org/IconLibrary/ (dépôt : https://gitlab.gnome.org/World/design/icon-library).
- **GNOME Icon Development Kit :** https://gitlab.gnome.org/Teams/Design/icon-development-kit.
- **Réutilisation webview non-GTK :** Le jeu CC0 (Icon Development Kit) est le choix optimal pour une webview. Il évite la contamination copyleft (CC-BY-SA / LGPL) et s'intègre sous forme de sprite ou map TypeScript.

## 4. Incohérences relevées

1. **Jeu hétérogène :** Tracés SVG artisanaux côtoyant des glyphes Unicode (`×`, `›`, `✓`, `●`, `−`, `+`, `∑`, `ƒx`) et des formes CSS.
2. **Contour vs remplissage :** Tracés SVG en filaire (`fill: none; stroke: currentColor`), sauf `more` et `ul` contenant des disques pleins.
3. **Épaisseurs de trait :** Valeur par défaut à 1.6 px au lieu des 2 px de la HIG. Écarts internes : 1.3 px (`footnote`), 1.8 px (`caretdown`), 2.2 px (`freeze`), et 1.1 à 1.7 px dans `FILE_MARKS`.
4. **Tailles disparates :** Affichage arbitraire à 12, 13, 14 ou 16 px sans standard unifié.
5. **Métaphores et doublons :** `branch` utilisé pour l'historique (violation HIG / `spec.md` §8.3 / G-101 qui impose `clock`). Doublon `down`/`caretdown`. `zoomin`/`zoomout` orphelins.

## 5. Recommandations et correspondances

Adopter une source unique : **GNOME Icon Development Kit (CC0)** sur grille 16×16 px, trait 2 px, `currentColor`.

| Icône / Glyphe actuel | Emplacement représentatif | Équivalent GNOME standard recommandé |
|---|---|---|
| `branch` | `HeaderBar.svelte:126` | `preferences-system-time-symbolic` (`clock`) |
| `side` | `HeaderBar.svelte:80` | `sidebar-show-symbolic` |
| `eye` | `HeaderBar.svelte:100` | `eye-open-negative-filled-symbolic` |
| `present` | `HeaderBar.svelte:111` | `tv-symbolic` |
| `menu` | `HeaderBar.svelte:129` | `open-menu-symbolic` |
| `close` / `×` | Dialogues, `SearchPanel:412` | `window-close-symbolic` |
| `check` / `✓` | Menus, `SheetStatus:96` | `object-select-symbolic` |
| `right` / `›` | `Sidebar:39`, `SearchPanel:499`| `pan-end-symbolic` |
| `down` / `caretdown` | Menus, grille | `pan-down-symbolic` |
| `plus` / `+` | Menus, `SheetStatus:166` | `list-add-symbolic` |
| `minus` / `−` | `Preview:109`, `SheetStatus:164`| `list-remove-symbolic` |
| `more` | `FormatBar.svelte:86` | `view-more-symbolic` |
| Triangle CSS | `changeGutter.ts:124` | `go-next-symbolic` |
| `●` (radio/sync) | `SheetStatus.svelte:117` | `radio-checked-symbolic` |

## 6. Règle de test / lint proposée

Test Vitest (`frontend/src/ui/icons/icons.test.ts`) interdisant SVG inline et glyphes hors de `Icon.svelte` :

```ts
import { readFileSync, readdirSync, statSync } from "node:fs";
import { describe, expect, it } from "vitest";

function getSvelteFiles(dir: string, files: string[] = []): string[] {
  for (const f of readdirSync(dir)) {
    const p = `${dir}/${f}`;
    if (statSync(p).isDirectory()) getSvelteFiles(p, files);
    else if (p.endsWith(".svelte") && !p.endsWith("Icon.svelte") && !p.endsWith("HistoryRow.svelte")) files.push(p);
  }
  return files;
}

describe("Règle d'intégrité des icônes", () => {
  it("interdit les balises <svg> et glyphes d'icônes hors de Icon.svelte", () => {
    const FORBIDDEN = [/<svg\b/, /[×✓›●]/];
    for (const f of getSvelteFiles("frontend/src")) {
      const src = readFileSync(f, "utf8").replace(/<script[\s\S]*?<\/script>/g, "").replace(/<style[\s\S]*?<\/style>/g, "");
      for (const rx of FORBIDDEN) {
        expect(rx.test(src), `${f} utilise une icône brute non gérée par Icon.svelte: ${rx}`).toBe(false);
      }
    }
  });
});
```
