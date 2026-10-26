// Every module registers its type when it is imported: importing this file is how the app learns
// the types. A new type is one new module and one new line here (ADR 0027).
import "./typst";
import "./markdown";
import "./csv";
import "./plaintext";
import "./code";
import "./image";
import "./pdf";
import "./binary";

export {
  all,
  BINARY,
  byFileName,
  get,
  PLAIN,
  register,
  resolve,
  type Subject,
  unregister,
} from "./registry";
export type * from "./types";
export { isViewable } from "./viewable";
