# P1 unit evidence (2026-11-13)

Observed before the fixes, with plain go test -run of pure tests:
- TestCoreSentinelSeccompPolicy: FAIL; prlimit64 resource 4 returned 0x7fff0000
  (ALLOW) for non-null new-limit pointers, including a high-word-only pointer.
- TestBlockedSecondRunDiscardsAllDiagnostics: FAIL; PDF error text and CANARY
  diagnostics survived a later SVG SIGSYS.

Observed after the fixes, uncached:
`go test -count=1 -run '^(TestCoreSentinelSeccompPolicy|TestSeccompPolicyVM|TestBlockedSecondRunDiscardsAllDiagnostics|TestBlockedErrorCarriesNoDetail|TestFailureBlocked|TestMemoryCauseFromEvents|TestMemoryTimelineSameReadAndFailure|TestFailureMemoryCause|TestNamespaceDeathIdentityAndManifest)$' ./internal/sidecar ./internal/typst`
Both packages PASS. Build ./... and vet of sidecar/... and typst/... PASS.
No heavy, cgroup, systemd, TLC or mutation runner was invoked in this continuation.
The host session must validate the target integration tests and all criteria.
