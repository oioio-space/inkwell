import type { IncomingFile } from "../../project/import";

/** The marker of a drag that carries project files (ids), as opposed to files from outside. */
export const INTERNAL_DRAG = "application/x-inkwell-files";

/** True if the drag brings files from outside the window (the desktop, another program). */
export function bringsFiles(dt: DataTransfer | null): boolean {
  return dt !== null && Array.from(dt.types).includes("Files");
}

/** True if the drag carries files of the project, which it moves. */
export function bringsProjectFiles(dt: DataTransfer | null): boolean {
  return dt !== null && Array.from(dt.types).includes(INTERNAL_DRAG);
}

/** A picked or dropped `File`, as an incoming file; `path` is its place inside a dropped folder. */
export function fromFile(file: File, path = file.name): IncomingFile {
  return {
    path,
    size: file.size,
    read: async () => new Uint8Array(await file.arrayBuffer()),
  };
}

/** The files of an `<input type="file">`; with `webkitdirectory` each keeps its folder path. */
export function fromList(files: FileList | null | undefined): IncomingFile[] {
  return Array.from(files ?? [], (f) => fromFile(f, f.webkitRelativePath || f.name));
}

interface FsEntry {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  file?(ok: (f: File) => void, fail: (e: unknown) => void): void;
  createReader?(): { readEntries(ok: (e: FsEntry[]) => void, fail: (e: unknown) => void): void };
}

/** Walks a dropped folder, depth first. A reader returns its entries in batches until empty. */
async function walk(entry: FsEntry, prefix: string, out: IncomingFile[]): Promise<void> {
  if (entry.isFile && entry.file) {
    const file = await new Promise<File>((ok, fail) => entry.file?.(ok, fail));
    out.push(fromFile(file, `${prefix}${entry.name}`));
    return;
  }
  const reader = entry.createReader?.();
  if (!entry.isDirectory || !reader) return;
  for (;;) {
    const batch = await new Promise<FsEntry[]>((ok, fail) => reader.readEntries(ok, fail));
    if (batch.length === 0) break;
    for (const child of batch) await walk(child, `${prefix}${entry.name}/`, out);
  }
}

/**
 * The files of a drop, folders included. The entries must be taken before the first `await`:
 * the browser empties the data transfer when the event handler returns.
 */
export async function collectDropped(dt: DataTransfer): Promise<IncomingFile[]> {
  const entries: FsEntry[] = [];
  for (const item of Array.from(dt.items ?? [])) {
    const entry = (
      item as DataTransferItem & { webkitGetAsEntry?(): FsEntry | null }
    ).webkitGetAsEntry?.();
    if (entry) entries.push(entry);
  }
  // Plain files need no walk (and a walk is not reliable in every engine); only a folder does.
  const plain = fromList(dt.files);
  if (!entries.some((e) => e.isDirectory)) return plain;
  const out: IncomingFile[] = [];
  for (const entry of entries) await walk(entry, "", out);
  return out;
}
