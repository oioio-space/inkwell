import { vi } from "vitest";

// The tests read the French interface, as the e2e tests do (locale fr-FR).
Object.defineProperty(navigator, "language", { value: "fr-FR", configurable: true });

// No unit test runs inside a Wails webview, so none loads the real @wailsio/runtime: importing it
// arms webview side effects, among them drag.js's 50 ms `window.setInterval` poll, a Node timer
// that outlives the environment's teardown and then reads the deleted `window` global (CI run
// 45174333408, "window is not defined"). This boundary covers every test file; a file that needs
// a runtime behaviour mocks the module itself, which replaces this one. Only what the code reaches
// without a Go side exists: the bindings' import-time `Create.Events`, and binding calls, which
// reject as they would with no Go side; any other entry point (Stream…) fails on use with
// Vitest's "No export is defined on the mock" error, naming it.
vi.mock("@wailsio/runtime", () => ({
  Call: {
    ByID: (id: number) =>
      Promise.reject(new Error(`Call.ByID(${id}): no Wails runtime in unit tests`)),
  },
  Create: { Events: {} },
}));
