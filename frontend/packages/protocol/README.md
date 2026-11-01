# @inkwell/protocol

MPL-2.0, version 0.1.0. A pnpm workspace package containing Inkwell's existing
webview envelope codec and an implementation-neutral conformance runner.

Import `decodeEnvelope`, `encodeEnvelope` and the associated payload codecs from
`@inkwell/protocol`. These are the existing codecs with unchanged behavior.

`runConformance(vectors, check, 1)` visits every vector; your check must verify its
verdict and expectations. Results identify required cases separately from future
cases. `conformance.test.ts` reads the canonical `protocol/vectors/*.json` files
and tests the current binary formats; proposed formats remain informational.
Run from `frontend`: `pnpm exec vitest run packages/protocol/conformance.test.ts`.
