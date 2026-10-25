import type { ProjectFile } from "../../store/files";

export interface FolderNode {
  type: "folder";
  /** The folder's path, without a trailing slash. */
  path: string;
  name: string;
  depth: number;
  children: TreeNode[];
}

export interface FileNode {
  type: "file";
  path: string;
  name: string;
  depth: number;
  file: ProjectFile;
}

export type TreeNode = FolderNode | FileNode;

const natural = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

/** Folders first, then files, each in natural order ("2.typ" before "10.typ"). */
function sortNodes(nodes: TreeNode[]): void {
  nodes.sort(
    (a, b) =>
      (a.type === b.type ? 0 : a.type === "folder" ? -1 : 1) || natural.compare(a.name, b.name),
  );
  for (const n of nodes) if (n.type === "folder") sortNodes(n.children);
}

/** Builds the folder tree the sidebar shows from the flat, slash-separated file list. */
export function buildTree(files: readonly ProjectFile[]): TreeNode[] {
  const root: TreeNode[] = [];
  const folders = new Map<string, FolderNode>();
  for (const file of files) {
    const parts = file.path.split("/");
    let siblings = root;
    for (let i = 0; i < parts.length - 1; i++) {
      const path = parts.slice(0, i + 1).join("/");
      let folder = folders.get(path);
      if (!folder) {
        folder = { type: "folder", path, name: parts[i] ?? path, depth: i, children: [] };
        folders.set(path, folder);
        siblings.push(folder);
      }
      siblings = folder.children;
    }
    siblings.push({
      type: "file",
      path: file.path,
      name: parts[parts.length - 1] ?? file.path,
      depth: parts.length - 1,
      file,
    });
  }
  sortNodes(root);
  return root;
}

/** The nodes in display order, skipping the content of collapsed folders. */
export function visibleNodes(
  tree: readonly TreeNode[],
  collapsed: ReadonlySet<string>,
): TreeNode[] {
  const out: TreeNode[] = [];
  const walk = (nodes: readonly TreeNode[]) => {
    for (const n of nodes) {
      out.push(n);
      if (n.type === "folder" && !collapsed.has(n.path)) walk(n.children);
    }
  };
  walk(tree);
  return out;
}
