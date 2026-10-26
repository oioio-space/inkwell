import { describe, expect, it } from "vitest";
import { pageImage, pageSize } from "./svg";
import { svgPage } from "./testkit";

const decode = (src: string) =>
  new TextDecoder().decode(Uint8Array.from(atob(src.split(",")[1] ?? ""), (c) => c.charCodeAt(0)));

describe("page images", () => {
  it("PRV-53 a Typst page becomes a data image of the right size", () => {
    const img = pageImage(svgPage(595.28, 841.89));
    expect(img?.src.startsWith("data:image/svg+xml;base64,")).toBe(true);
    expect(img).toMatchObject({ width: 595.28, height: 841.89 });
    expect(decode(img?.src ?? "")).toBe(svgPage(595.28, 841.89));
  });

  it("PRV-53 non-ASCII text survives the round trip", () => {
    const svg = `${svgPage()}<!-- é à 日本語 😀 -->`;
    expect(decode(pageImage(svg)?.src ?? "")).toBe(svg);
  });

  it("PRV-53 the size comes from the viewBox when width and height are absent", () => {
    expect(pageSize('<svg viewBox="0 0 200 100" xmlns="http://www.w3.org/2000/svg"/>')).toEqual({
      width: 200,
      height: 100,
    });
    expect(pageSize('<?xml version="1.0"?><svg width="10" height="20"/>')).toEqual({
      width: 10,
      height: 20,
    });
  });

  it("PRV-54 nothing active gets through: scripts, handlers, foreign content, javascript: links", () => {
    const hostile = [
      '<svg width="10pt" height="10pt"><script>alert(1)</script></svg>',
      '<svg width="10pt" height="10pt" onload="alert(1)"/>',
      '<svg width="10pt" height="10pt"><rect onclick = "x()"/></svg>',
      '<svg width="10pt" height="10pt"><foreignObject><iframe src="x"/></foreignObject></svg>',
      '<svg width="10pt" height="10pt"><a xlink:href="javascript:alert(1)"><rect/></a></svg>',
      '<svg width="10pt" height="10pt"><animate attributeName="href" values="javascript:x"/></svg>',
      '<svg width="10pt" height="10pt"><SCRIPT>x</SCRIPT></svg>',
    ];
    for (const svg of hostile) expect(pageImage(svg), svg).toBeUndefined();
  });

  it("PRV-54 what is not an SVG page is refused", () => {
    for (const bad of [
      "",
      "hello",
      "<html><body/></html>",
      "<div><svg width='1' height='1'/></div>",
      "<svg>no size</svg>",
      '<svg width="0pt" height="10pt"/>',
      '<svg width="-5pt" height="10pt"/>',
    ]) {
      expect(pageImage(bad), bad).toBeUndefined();
    }
  });
});
