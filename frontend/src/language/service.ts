import type { SocketLike } from "../net/WailsNetworkAdapter";
import { LanguageClient, type LanguageDiagnostic } from "./client";

let open: () => SocketLike = () => {
  throw new Error("Language bridge not configured");
};
let publish: (file: string, version: number, items: LanguageDiagnostic[]) => void = () => {};
let file: () => string = () => "main.typ";
export function configureLanguage(
  bridge: () => SocketLike,
  diagnostics: typeof publish,
  current: () => string,
) {
  open = bridge;
  publish = diagnostics;
  file = current;
}
export const currentFile = () => file();
export const language = new LanguageClient(
  () => open(),
  (file, version, items) => publish(file, version, items),
);
