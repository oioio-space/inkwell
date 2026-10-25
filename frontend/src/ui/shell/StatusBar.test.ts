import { afterEach, describe, expect, it } from "vitest";
import { Eol, type FileInfo } from "../../store/files";
import { encodingLabel, needsIndicator } from "../popover/encoding";
import { cleanup, click, fakeEditor, render } from "../testkit.svelte";
import StatusBar from "./StatusBar.svelte";

afterEach(cleanup);

const utf8: FileInfo = { encoding: "utf-8", eol: Eol.LF, bom: false, mixed: false };
const idle = () => new Promise((r) => setTimeout(r, 120));

const mountBar = (
  marked: string,
  info: FileInfo | undefined,
  lang: "typst" | "markdown" | "plain" = "typst",
) => {
  const editor = fakeEditor(marked);
  const host = render(StatusBar, {
    lang,
    status: "alone",
    peerCount: 0,
    get editorState() {
      return editor.state;
    },
    info,
  });
  return { editor, host };
};

describe("status bar", () => {
  it("UI-07 hides the encoding for UTF-8 with LF", () => {
    expect(needsIndicator(utf8)).toBe(false);
    const { host } = mountBar("a‸", utf8);
    expect(host.querySelector('button[aria-label^="Encodage"]')).toBeNull();
    expect(host.textContent).not.toContain("UTF-8");
    expect(host.textContent).not.toContain("LF");
  });

  it("UI-07 shows Windows-1252 or CRLF, with a menu that explains it", () => {
    const { host } = mountBar("a‸", {
      encoding: "windows-1252",
      eol: Eol.CRLF,
      bom: false,
      mixed: false,
    });
    const indicator = host.querySelector<HTMLButtonElement>(
      'button[aria-haspopup="menu"][aria-expanded]',
    );
    const encoding = [...host.querySelectorAll<HTMLButtonElement>("button")].find((b) =>
      b.textContent?.includes("Windows-1252"),
    ) as HTMLButtonElement;
    expect(encoding.textContent).toContain("Windows-1252 · CRLF");
    expect(indicator).not.toBeNull();

    click(encoding);
    const menu = document.querySelector('[role="menu"]') as HTMLElement;
    expect(menu.textContent).toContain("Ce fichier est enregistré en Windows-1252.");
    // The conversion is not wired yet: the button is there, disabled, with its "bientôt" tooltip.
    const convert = [...menu.querySelectorAll("button")].find((b) =>
      b.textContent?.includes("Convertir en UTF-8"),
    ) as HTMLButtonElement;
    expect(convert.getAttribute("aria-disabled")).toBe("true");
    expect(
      document.querySelector(`#${convert.getAttribute("aria-describedby")}`)?.textContent,
    ).toBe("Bientôt disponible");
    const radios = [...menu.querySelectorAll('[role="menuitemradio"]')];
    expect(radios.map((r) => r.getAttribute("aria-checked"))).toEqual(["false", "true", "false"]);
  });

  it("UI-07 a UTF-8 file with CRLF still shows an indicator, and other encodings are named", () => {
    const crlf: FileInfo = { ...utf8, eol: Eol.CRLF };
    expect(needsIndicator(crlf)).toBe(true);
    expect(needsIndicator({ ...utf8, encoding: "utf-16le" })).toBe(true);
    expect(encodingLabel("utf-16le")).toBe("UTF-16 LE");
    const { host } = mountBar("a‸", crlf);
    expect(host.textContent).toContain("UTF-8 · CRLF");
  });

  it("UI-07 shows nothing while the file's storage is not known yet", () => {
    const { host } = mountBar("a‸", undefined);
    expect(host.querySelector('button[aria-label^="Encodage"]')).toBeNull();
  });

  it("shows the caret as line and column, in graphemes", () => {
    const { host } = mountBar("première\nlig🙂ne‸", utf8);
    expect(host.querySelector('[data-testid="position"]')?.textContent).toBe("Ligne 2, colonne 7");
  });

  it("counts words when the browser is idle and opens the statistics", async () => {
    const { host } = mountBar("un deux ⟨trois quatre⟩", utf8);
    await idle();
    const words = host.querySelector('[data-testid="words"]');
    expect(words?.textContent).toBe("4 mots");

    click(words?.closest("button") as HTMLElement);
    const popup = document.querySelector('[role="dialog"]') as HTMLElement;
    expect(popup.getAttribute("aria-label")).toBe("Statistiques");
    const rows = [...popup.querySelectorAll("tbody tr")].map((r) =>
      [...r.children].map((c) => c.textContent?.trim()),
    );
    expect(rows[0]).toEqual(["Mots", "4", "2"]);
  });

  it("names the language of the file", () => {
    expect(
      mountBar("a‸", utf8, "markdown").host.querySelector('[data-testid="language"]')?.textContent,
    ).toBe("Markdown");
    cleanup();
    expect(
      mountBar("a‸", utf8, "plain").host.querySelector('[data-testid="language"]')?.textContent,
    ).toBe("Texte brut");
  });
});
