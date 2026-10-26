/**
 * Symbolic 16×16 icons of the Adwaita mockup: one stroke, `currentColor`, round caps. The
 * markup is constant (never built from input), so it is safe to render with `{@html}`.
 */
export const ICONS = {
  side: '<rect x="1.5" y="2.5" width="13" height="11" rx="2.5"/><path d="M6 2.5v11"/>',
  menu: '<path d="M2.5 4h11M2.5 8h11M2.5 12h11"/>',
  plus: '<path d="M8 2.5v11M2.5 8h11"/>',
  minus: '<path d="M2.5 8h11"/>',
  eye: '<path d="M1.5 8S4 3.5 8 3.5 14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z"/><circle cx="8" cy="8" r="2"/>',
  target: '<circle cx="8" cy="8" r="4.5"/><path d="M8 1.5v3M8 11.5v3M1.5 8h3M11.5 8h3"/>',
  fail: '<circle cx="8" cy="8" r="6.2"/><path d="M5.6 5.6l4.8 4.8M10.4 5.6l-4.8 4.8"/>',
  down: '<path d="M4 6l4 4 4-4"/>',
  right: '<path d="M6 4l4 4-4 4"/>',
  back: '<path d="M10 4L6 8l4 4"/>',
  close: '<path d="M4 4l8 8M12 4l-8 8"/>',
  more: '<circle cx="3.5" cy="8" r="1" fill="currentColor"/><circle cx="8" cy="8" r="1" fill="currentColor"/><circle cx="12.5" cy="8" r="1" fill="currentColor"/>',
  check: '<path d="M3 8.5l3 3 7-7"/>',
  link: '<path d="M7 9a3 3 0 0 0 4.2 0l2-2a3 3 0 0 0-4.2-4.2l-.8.8M9 7a3 3 0 0 0-4.2 0l-2 2a3 3 0 0 0 4.2 4.2l.8-.8"/>',
  code: '<path d="M5.5 4.5L2 8l3.5 3.5M10.5 4.5L14 8l-3.5 3.5"/>',
  codeblock:
    '<rect x="1.5" y="2.5" width="13" height="11" rx="2.5"/><path d="M6.3 6.2L4.5 8l1.8 1.8M9.7 6.2L11.5 8 9.7 9.8"/>',
  ul: '<path d="M6 4h8M6 8h8M6 12h8"/><circle cx="2.8" cy="4" r=".9" fill="currentColor"/><circle cx="2.8" cy="8" r=".9" fill="currentColor"/><circle cx="2.8" cy="12" r=".9" fill="currentColor"/>',
  ol: '<path d="M6.5 4h7.5M6.5 8h7.5M6.5 12h7.5M2.5 2.8l1-.6v3.6M2 9.2c.4-.6 2-.7 1.9.3-.1.7-1.9 1.5-1.9 2.2h2"/>',
  task: '<rect x="1.8" y="2.3" width="4.4" height="4.4" rx="1"/><path d="M2.8 4.5l1 1 1.6-2M8.5 4.5h5.5M8.5 11.5h5.5"/><rect x="1.8" y="9.3" width="4.4" height="4.4" rx="1"/>',
  quote: '<path d="M3 3v10M6.5 5h7M6.5 8h7M6.5 11h4.5"/>',
  table:
    '<rect x="1.5" y="2.5" width="13" height="11" rx="2"/><path d="M1.5 6.5h13M1.5 10h13M6 6.5v7"/>',
  image:
    '<rect x="1.5" y="2.5" width="13" height="11" rx="2"/><circle cx="5.5" cy="6" r="1.2"/><path d="M1.8 12l4-3.5 3 2.5 2-1.5 3.2 2.5"/>',
  help: '<circle cx="8" cy="8" r="6.5"/><path d="M6.2 6.2a1.9 1.9 0 1 1 2.6 1.8c-.5.2-.8.6-.8 1.1v.4M8 11.6v.2"/>',
  warn: '<path d="M8 2l6.5 11.5h-13z"/><path d="M8 6.5v3M8 11.6v.2"/>',
  copy: '<rect x="5" y="5" width="9" height="9" rx="2"/><path d="M11 5V3.5A1.5 1.5 0 0 0 9.5 2h-6A1.5 1.5 0 0 0 2 3.5v6A1.5 1.5 0 0 0 3.5 11H5"/>',
  paperclip:
    '<path d="M13 7.5l-5.3 5.3a3.2 3.2 0 0 1-4.5-4.5L8.7 2.8a2.1 2.1 0 0 1 3 3L6.4 11.1a1 1 0 0 1-1.5-1.5L9.8 4.7"/>',
  include: '<path d="M5 1.5h6l3 3v10H5z" opacity=".55"/><path d="M1.5 8h7M6 5.5L8.5 8 6 10.5"/>',
  figure:
    '<rect x="2" y="1.5" width="12" height="9" rx="2"/><path d="M2.5 9l3.5-3 3 2.5 2-1.5 2.5 2M4 13.5h8"/>',
  csvtable:
    '<rect x="1.5" y="2.5" width="13" height="11" rx="2"/><path d="M1.5 6.5h13M1.5 10h13M6 2.5v11M10 2.5v11"/>',
  data: '<path d="M5.5 2.5c-1.5 0-2 .7-2 2v1.6c0 .9-.5 1.4-1.5 1.9 1 .5 1.5 1 1.5 1.9v1.6c0 1.3.5 2 2 2M10.5 2.5c1.5 0 2 .7 2 2v1.6c0 .9.5 1.4 1.5 1.9-1 .5-1.5 1-1.5 1.9v1.6c0 1.3-.5 2-2 2"/>',
  book: '<path d="M2.5 2.5h4a1.5 1.5 0 0 1 1.5 1.5v9.5a1.5 1.5 0 0 0-1.5-1.5h-4zM13.5 2.5h-4A1.5 1.5 0 0 0 8 4v9.5A1.5 1.5 0 0 1 9.5 12h4z"/>',
  cite: '<path d="M3 7.5c-.8.6-1 1.4-1 2.3V12h3V9H3.4M9 7.5c-.8.6-1 1.4-1 2.3V12h3V9H9.4"/><path d="M12.5 3.5h2M12.5 6h2" opacity=".6"/>',
  at: '<circle cx="8" cy="8" r="2.6"/><path d="M10.6 8v1.2a1.9 1.9 0 0 0 3.8 0V8A6.4 6.4 0 1 0 11.6 13.3"/>',
  footnote:
    '<path d="M2 5h8M2 8h8M2 11h5"/><path d="M12 2.5l1.2-.7v4" stroke-width="1.3"/><path d="M2 14h12" opacity=".5"/>',
  outline:
    '<path d="M2 3.5h7M4 6.5h6M4 9.5h6M2 12.5h7"/><path d="M12.5 3.5h1.5M12.5 6.5h1.5M12.5 9.5h1.5M12.5 12.5h1.5" opacity=".6"/>',
  pagebreak: '<path d="M3.5 1.5v4h9v-4M3.5 14.5v-4h9v4"/><path d="M1.5 8h2M6 8h4M12.5 8h2"/>',
  gear: '<circle cx="8" cy="8" r="2.2"/><path d="M8 1.5v2M8 12.5v2M1.5 8h2M12.5 8h2M3.4 3.4l1.4 1.4M11.2 11.2l1.4 1.4M3.4 12.6l1.4-1.4M11.2 4.8l1.4-1.4"/>',
  folder: '<path d="M1.5 4.5V3h5l1.5 1.5h6.5v9h-13z"/>',
} as const;

export type IconName = keyof typeof ICONS;

const DOC = '<path d="M3.5 1.5h6l3 3v10h-9z"/><path d="M9.5 1.5v3h3"/>';

/** The page outline shared by every file icon, and the coloured mark inside it. */
export const FILE_MARKS = {
  typst: `${DOC}<path d="M7 7v5.2c0 .6.4.9 1 .9h.8M5.6 8.8h3" class="mk-typst" stroke-width="1.7"/>`,
  markdown: `${DOC}<path d="M5.3 12.6V8.4l1.4 1.8 1.4-1.8v4.2M10.4 8.4v4.1M9.2 11.3l1.2 1.3 1.2-1.3" class="mk-markdown" stroke-width="1.3"/>`,
  text: `${DOC}<path d="M5.8 8.2h4.4M5.8 10.4h4.4M5.8 12.6h2.6" opacity=".7"/>`,
  csv: `${DOC}<path d="M5.5 7.8h5v5h-5zM5.5 10.3h5M8 7.8v5" class="mk-csv" stroke-width="1.2"/>`,
  bib: `${DOC}<path d="M6.2 9.2c-.6.4-.8 1-.8 1.7v1h1.4v-1.3h-.9M9.3 9.2c-.6.4-.8 1-.8 1.7v1h1.4v-1.3H9" class="mk-bib" stroke-width="1.2"/>`,
  image: `${DOC}<path d="M5.5 12.8l1.8-2 1.3 1.2 1-.9 1.4 1.7" class="mk-image" stroke-width="1.3"/><circle cx="6.6" cy="8.5" r=".8" class="mk-image"/>`,
  pdf: `${DOC}<path d="M5.5 12.6V8.6h1.3a1 1 0 0 1 0 2H5.5M9.2 8.6v4h.8a1.3 1.3 0 0 0 0-4z" class="mk-pdf" stroke-width="1.1"/>`,
  data: `${DOC}<path d="M6.6 8.2c-.8 0-1 .4-1 1v.6c0 .4-.2.6-.6.8.4.2.6.4.6.8v.6c0 .6.2 1 1 1M9.4 8.2c.8 0 1 .4 1 1v.6c0 .4.2.6.6.8-.4.2-.6.4-.6.8v.6c0 .6-.2 1-1 1" class="mk-data" stroke-width="1.1"/>`,
  other: DOC,
  folder: ICONS.folder,
} as const;

export type FileMark = keyof typeof FILE_MARKS;
