# Sidecar output limits by platform

Platform policy follows [ADR 0003](adr/0003-wails-v3-cgo-native-builds.md).
The proposed F5 backend is described in
[ADR 0033](adr/0033-sidecar-directory-bound.md); its fixed shares do not yet
implement the required shared remaining budget.

| Platform | Bounded disk output (`WatchDir` and positive `MaxDirBytes`) |
|---|---|
| Linux | Requires the internal launcher, Landlock ABI ≥ 3 and seccomp; rejects the launch if unavailable. Declared files alone are writable, with exact hard file-size limits. |
| macOS | Rejects bounded launches before executing the child; no native directory-bound backend is implemented. |
| Windows | Rejects bounded launches before executing the child; no native directory-bound backend is implemented. |

There is no polling-only fallback. The poller remains a backstop on bounded
Linux runs. The bound measures logical file sizes, not metadata, allocated
blocks, or cumulative I/O. The caller must keep the output directory private
and must not mutate it concurrently. Every launch requires an explicit project directory; home, configuration directories,
and their ancestors cannot be read authorities. Typst compilation, query, preview and
export are refused on macOS and Windows, including personal documents, until N35
proves confinement (architecture §9.11). Native platform CI must check this refusal.
