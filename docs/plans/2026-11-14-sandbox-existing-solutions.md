# Sandboxing Typst and tinymist in inkwell: what already exists (research, 2026-11-14)

Method: pages fetched on 2026-11-14 (WebFetch, GitHub/npm/jsDelivr APIs). Every claim carries a source. "UNVERIFIED" marks
what I did not read on a fetched page, or only saw in a search snippet. No repo changes. Sizes marked "estimate" are mine.

## Summary (20 lines)

1. A ready-made, maintained, cross-platform sandbox that wraps a native child does NOT exist for Go. Closest: Anthropic
   `sandbox-runtime` (Apache-2.0, bubblewrap + sandbox-exec + Windows alpha) but it is an npm/TypeScript package, beta,
   and on Linux it needs bubblewrap, so it inherits the Ubuntu 24.04 userns problem (its README says so).
2. Ubuntu 24.04: Ubuntu's own bug says bwrap gets NO dedicated AppArmor profile (2046477); a profile for `/usr/bin/bwrap`
   is a per-machine admin fix. Anything that needs a user namespace (bwrap, nsjail, gVisor rootless, systemd `Private*` in
   user units, Birdcage) hits it. Landlock + seccomp + rlimits need NO user namespace and are not touched by that sysctl.
3. The cheapest real fix for the stated Ubuntu/arm64/CI breakage is a TIER, not a new product: Landlock (go-landlock, MIT,
   Google-adjacent, 287 commits) + seccomp + rlimit as the default tier, namespaces only as an optional extra tier.
4. WASM option: the Typst compiler does run as WASM (typst.ts compiler wasm = 28.3 MB in 0.7.0, 30.2 MB in 0.8.0-rc4,
   Apache-2.0, `latest` tag 0.7.0 of 2026-06-01, rc4 of 2026-11-09, never 1.0). Typst itself documents "no arbitrary file
   access, no network, no OS commands" as its model (community statement, not an official page).
5. In WASM, system fonts do not exist (supply them), `@preview` packages need your own resolver (typst.ts has an
   undocumented one), no threads (UNVERIFIED for wazero; browser threads need COOP/COEP), 4 GiB memory ceiling per module.
6. Go-side WASI: a Go library `typst-go-wasm` exists (MIT, 7 commits, 26 stars, no releases, ~48 MB RAM per compile, fonts
   passed in, packages not mentioned). It proves feasibility, not production quality. wazero itself is mature (1.0 in 2023,
   Apache-2.0, pure Go, compiler on amd64/arm64 Linux/macOS/Windows, interpreter elsewhere).
7. tinymist in WASM exists (`tinymist-web.vsix` 15 MB in release v0.15.12 of 2026-11-03, built since 2025) but is a
   VS Code Web extension; the gap list vs the native server is NOT documented (open issue #2781 says packages and fonts
   must be bundled by hand). Treat as a research item, not a drop-in.
8. No benchmark WASM-vs-native Typst was found. Do not assume a number; the spike must measure it.
9. Peer tools: nobody sandboxes the Typst/LaTeX compiler the way inkwell does for desktop. Overleaf uses a Docker container
   per compile (server side); typst.app and typst.ts run the compiler as WASM in the browser; VS Code + tinymist runs
   native unsandboxed (VS Code docs fetch failed: UNVERIFIED). "Compiler in WASM" is the norm for browser products, not for
   desktop editors.
10. Ranking (section D): (1) tiered native sandbox rebuilt on Landlock+seccomp+rlimit (+ job object / seatbelt) with
    the existing namespace code demoted to optional; (2) WASM compiler as the portable fallback tier chosen by a spike;
    (3) bwrap-based wrappers only as an optional tier. Keep tinymist native.
11. Decisive facts: Ubuntu gives bwrap no profile; Landlock/seccomp are unaffected by it; the WASM compiler is real but
    packages/fonts/threads/tinymist parity are unproven for inkwell's fixtures.
12. Honest sizes (estimates): tier rework 1 500-2 500 lines of Go deleted/changed; WASM spike 300-600 lines + 1-2 days;
    full WASM tier 1 500-3 000 lines plus a Rust wrapper crate to own.

---

## A. Compiler as WebAssembly

### A.1 Typst and tinymist as WASM: facts

**Official Typst.**
- Typst's plugin system executes WASM modules "in isolation from your system"; printing and reading files are not supported;
  functions are meant to be pure. Source: https://typst.app/docs/reference/foundations/plugin/ (read).
- Typst executes those plugins with the `wasmi` runtime (v0.14.2 release notes: "Updated the WebAssembly runtime used for
  executing plugins"; wasmi memory bug in v0.14.0/0.14.1) and, since v0.13.0, plugins run in multiple threads. Source:
  https://github.com/typst/typst/releases (read). Meaning: Typst itself is a WASM host; that is a different thing from
  Typst-as-WASM-guest.
- Latest Typst release seen: v0.15.1, 2026-07-17 (same page). I did not verify that the official CLI crates build for
  `wasm32-wasip1` out of the box: UNVERIFIED. The CLI (`typst-cli`) depends on system fonts, network package download and
  file watching, so a wrapper crate is needed in practice (typst-go-wasm and typst.ts are exactly such wrappers).
- Typst architecture: the compiler is parameterised by a `World` interface so it can run in CLI and web app (search summary
  of Typst dev docs: UNVERIFIED, page not fetched). The Typst web app compiles in the browser as WASM, React front end,
  backend on Azure Functions (only from a dev.to profile of the Typst account in a search snippet: UNVERIFIED; no official
  architecture write-up found).
- Security model (community statement, maintainer agrees docs should say it): no arbitrary file access, no arbitrary
  network requests, no OS command execution, plugins only as sandboxed WASM; "not uniformly applied" (compiler and HTML
  export are named as exceptions). Source: https://forum.typst.app/t/typst-documentation-should-mention-security-model/8521
  (read). It is not a formal guarantee; a parser bug or a resource bomb (huge loops, huge images) is still possible.
  A past path-traversal CVE exists in the Python wrapper (Snyk, versions before 0.11.1: search snippet, UNVERIFIED).

**typst.ts (Myriad-Dreamin).**
- What: `typst::World` implementation and exporters to compile and render Typst in the browser, Node, or native. Licence
  Apache-2.0 (GitHub API). Source: https://github.com/Myriad-Dreamin/typst.ts (read).
- Forms: server-compiled vector artifact rendered client-side; or whole compiler in client WASM with canvas/SVG rendering.
  Incremental rendering exists via a streamed "compiler service" (typst-preview uses it). README. 
- Maturity: npm `@myriaddreamin/typst-ts-web-compiler` dist-tags: `latest` 0.7.0 (2026-06-01), `rc` 0.8.0-rc4 (2026-11-09);
  rc1..rc3 on 2026-06-18, 06-22, 06-29 (npm registry API, run by me). Rust crates are not on crates.io (git dependency on
  typst, README). No 1.0, no stability statement. Commit count 1 442, 1.3k stars.
- Bundle size (measured from jsDelivr file listing): compiler `typst_ts_web_compiler_bg.wasm` = 28 325 178 bytes (0.7.0),
  30 167 169 bytes (0.8.0-rc4); renderer wasm = 972 425 bytes (0.7.0). Uncompressed; compressed size not measured.
- Fonts: the all-in-one bundle fetches font assets from GitHub by default (needs internet); fonts can be supplied by the
  host. README. Packages: README says nothing; a Typst forum thread says typst.ts already implements `@preview` fetching but
  "isn't documented" and the user still had to add a font cache and a registry fetch by hand
  (https://forum.typst.app/t/webassembly-auto-download-and-build-typst-packages/7655, read). So offline packages from a
  local store: feasible by implementing the access model yourself, but not a documented, supported path.
- PDF export in WASM: not stated on any page I read (README, cookery, web-compiler README). UNVERIFIED for typst.ts.
  (The Go lib typst-go-wasm does produce PDFs, including tagged PDF, from WASM: see A.2.)
- Multi-threading / 4 GiB: not discussed on the pages read. WASM32 linear memory is capped at 4 GiB by the format;
  wazero's default maximum is 65 536 pages = 4 GB (pkg.go.dev, read). Browser threads need SharedArrayBuffer + cross-origin
  isolation: general knowledge, UNVERIFIED here.
- Performance WASM vs native for Typst: NO published figure found (two searches). Do not quote any. General WASM overhead
  numbers found are unrelated workloads (e.g. ~32 % slower sequential memory access, 1.75x-2.5x in a Node module test:
  search snippets, UNVERIFIED relevance).

**tinymist (language server).**
- Licence Apache-2.0; native release v0.15.12 on 2026-11-03; cadence of ~2-6 weeks with rc builds; v0.15.8 bumped Typst to
  0.15.1 (GitHub releases API/page, read). Native Linux binary assets: x86_64 77.5 MB, arm64 64 MB raw; tarballs ~30-33 MB.
- Web: release assets include `tinymist-web.vsix` (15.1 MB) and `tinymist-web.tar.gz` (12.4 MB). Issues: #970 "Support VS
  Code Web" (closed Jan 2025: the maintainer said all components "have potential to run as wasm" and early web support
  only bootstrapped features not needing the language server), #1945 (Jul 2025, build wasm into web extension), #2066
  (Aug 2025), #2431 (npm publication of tinymist-web, closed Apr 2026). Source: GitHub issues API (read by me).
- Issue #2781 (opened 2026-11-11 by a third party) states "Tinymist actually already has a WASM build of its language
  server, bundled into a dedicated VS Code Web extension" but it targets browser VS Code, and packages and fonts must be
  bundled by hand for offline use. That is the only gap statement I found. A full feature gap list (what the native server
  does that the web one does not) was NOT found: UNVERIFIED. Expect gaps in file watching, package resolution and fonts.

### A.2 Where to run it

**(i) In the Wails webview, in a Web Worker.**
- The browser WASM sandbox gives: no file, no network, no syscalls unless the host provides them. A Worker can be killed
  with `worker.terminate()` (time bound) and `WebAssembly.Memory({maximum})` bounds linear memory (general web platform
  knowledge: MDN not fetched, UNVERIFIED here). Compare rlimits: the same two bounds, but enforced by the engine inside the
  webview process, not by the kernel; a bug in the JS engine's WASM implementation is the attack surface (it is also what
  protects every web page, and WebKit/Chromium bugs there are actively fixed).
- Cost: it moves compile work into the webview process; a hostile document that exhausts memory can crash the webview
  (the UI) instead of a throwaway child. A bound via `maximum` makes it fail inside the worker; verify in the spike.
- WebKitGTK's own sandbox: opt-in via `webkit_web_context_set_sandbox_enabled()` (called before the first web process);
  Linux only, no-op elsewhere. Sources: https://webkitgtk.org/reference/webkit2gtk/stable/method.WebContext.set_sandbox_enabled.html
  and Catanzaro 2020 https://blogs.gnome.org/mcatanzaro/2020/03/31/sandboxing-webkitgtk-apps/ (both read; the blog says the
  sandbox is not on by default for apps, apps must opt in). Whether Wails v3 enables it: UNVERIFIED (check Wails source).
  The sandbox is built on bubblewrap (search snippets on WebKit build options, `-DENABLE_BUBBLEWRAP_SANDBOX`: not read in
  full). So WebKitGTK's web process itself is hit by the Ubuntu 24.04 restriction: Launchpad bugs show WebKit's
  xdg-dbus-proxy failing with "bwrap: setting up uid map: Permission denied" then a crash (bug 2051574, Wike bug 2060810:
  search snippets, UNVERIFIED in detail). Consequence: relying on the webview for isolation does not remove the Ubuntu
  problem for WebKit's own sandbox; it only removes it for YOUR child process.
- Packages offline: serve them via a custom `fetch` handler/service worker or an in-memory access model backed by Go over
  the Wails bridge (feasible by design, not documented by typst.ts).

**(ii) In the Go process with wazero (pure Go, no cgo).**
- wazero: Apache-2.0; "zero dependency"; 1.0 March 2023; implements Core spec 1.0 and 2.0; compiler engine tested on
  Linux amd64 and arm64, macOS arm64, Windows amd64 (and BSDs); interpreter on any Go target; compiler "often 10x or more"
  faster than the interpreter (wazero.io, GitHub README, Arcjet blog: all read). Latest module version on pkg.go.dev
  v1.12.0 of 2026-05-28 (but the page said a newer module version may exist).
- Limits you set: `WithMemoryLimitPages` (default 65 536 = 4 GB, caps per memory), `WithCloseOnContextDone` (stops
  execution on cancel/timeout; "disabled by default because it adds runtime overhead"), `WithCompilationCache` (in memory or
  on disk), `WithCoreFeatures` (threads not mentioned on the page; I did not verify threads support: UNVERIFIED, plan on
  single-thread). WASI via `wasi_snapshot_preview1`; file access only through the FS you mount (virtual FS) (wazero.io).
- Cost: compile of a 30 MB module at start takes seconds (Arcjet: "several seconds" at startup for their modules; use the
  on-disk compilation cache). Windows arm64 not in the tested list (macOS tested only on arm64; Windows amd64 only).
- Prior art: `varunbpatil/typst-go-wasm`: Go library, Typst through `typst-as-lib` compiled to `wasm32-wasip1`, embedded via
  `//go:embed`, MIT, 7 commits, 26 stars, no releases; no embedded fonts (caller passes TTF/OTF as base64 JSON); extra files
  by a `Files` map; packages not mentioned; fresh module per call, ~48 MB linear memory per call; semaphore suggested for
  concurrency; tagged PDF and page ranges exist; the README does not name the runtime (UNVERIFIED that it is wazero);
  "Rust WASM builds are not reproducible across toolchains" (a point for inkwell's SHA-256 pinning: pin the artifact, not the
  build). Source: https://github.com/varunbpatil/typst-go-wasm (read). Another Typst-WASM wrapper: `typst-wasm` (forum post,
  multi-runtime, fonts added explicitly; https://forum.typst.app/t/typst-wasm-compile-typst-in-browsers-node-and-serverless-runtimes/9399, read).
- "Official `typst` CLI under wazero": nobody found doing exactly that (searches "typst wasi", "typst wazero"). The real
  route is a thin Rust wrapper (like typst-as-lib) built for wasm32-wasip1, which inkwell would own and pin.

**(iii) wasmtime via cgo**: mentioned only. Rejected a priori (more cgo; Zed uses a Rust host for its WASM extensions, but
the page read did not name wasmtime: UNVERIFIED).

### A.3 Lost or harder versus the native sidecar

- Speed: unknown, to measure. Expect slower than native (no SIMD guarantees, single thread, bounds checks); Typst's own
  parallelism (comemo/rayon) is lost in single-thread WASM (inference).
- Exact parity: the wrapper crate reimplements the World (fonts, packages, file reads). Fonts: you supply them; Typst
  output depends on them, so parity with the CLI requires the same font set (the native sidecar already needs this for
  reproducibility). Package resolution is yours.
- Features: HTML export, `typst watch`, `--root` semantics, Typst's own wasmi plugin host inside WASM (a WASM runtime in a
  WASM runtime: works, slow): all unverified.
- tinymist: the web build is a different product surface; completion on `@preview` packages and diagnostics across files
  depend on your package/file access layer. Debugging is harder (no gdb; wazero stack traces with DWARF if enabled).
- Rule "pinned, embedded, SHA-256 verified, never downloaded at run time": equivalent. Embed the .wasm (28-30 MB for
  typst.ts; typst-go-wasm size unknown), verify the hash before instantiation, compile with a pinned toolchain. It is in
  fact easier than native: one artifact for all OSes and architectures instead of per-target binaries.

---

## B. Existing sandboxing tools for native children

Context common to Linux: the Ubuntu restriction is `kernel.apparmor_restrict_unprivileged_userns`, introduced in 23.10/24.04.
Unprivileged processes can create a user namespace but lose capabilities inside it unless an AppArmor profile with `userns`
allows it (https://discourse.ubuntu.com/t/understanding-apparmor-user-namespace-restriction/58007, read). Ubuntu's team
decided at the time that bwrap, installed by default on Ubuntu Desktop 24.04, "can be used to launch arbitrary binaries within
a new user namespace" so "no profile is planned" (https://bugs.launchpad.net/bugs/2046477, read); profiles shipped then were
for named apps (1password, Discord, code, firefox, obsidian, signal-desktop, slack, steam...). Later reports mention
`bwrap-userns-restrict` shipped with the apparmor package and flatpak/libgnome-desktop/bubblejail as intended users (the
Discourse page, read) and that the profile may break nested confinement (Claude Code issue 87680: search snippet,
UNVERIFIED). Qualys published three bypasses of the restriction in 2025 (https://www.qualys.com/2025/three-bypasses-of-Ubuntu-unprivileged-user-namespace-restrictions.txt,
only listed in search: UNVERIFIED). Conclusion: you cannot rely on userns on stock Ubuntu desktop for an app without its own profile.

### B.1 Linux

| Tool | What / licence / maintained | Go use | Isolates | Limits | Ubuntu 24.04 userns | Could replace inkwell's sandbox? |
|---|---|---|---|---|---|---|
| bubblewrap | LGPL-2.1; 9.5k stars; setuid mode removed (README read) | external binary to ship/depend on | mount/PID/net/user ns, seccomp via args | "a tool for building sandboxes, not a ready-made one"; needs unprivileged userns; DoS possible | FAILS without an AppArmor profile for the exact path; a bundled copy at another path does not match `/usr/bin/bwrap` profiles (inference: UNVERIFIED) | Partly, same problem as today |
| Landlock (go-landlock) | MIT; 287 commits; tests on kernels (README read) | pure Go library, in-process: restrict then exec, no ns | filesystem, some network, some IPC (ABI-dependent) | Kernel >= 5.13 for FS; TCP rules need kernel 6.7+, UDP 7.2+ (landrun README, read); no process/memory limits; MPTCP bypass with Go >= 1.24 listeners | NOT affected (no userns) | Yes for the FS and net part; add seccomp+rlimit |
| landrun | MIT; CLI wrapper over go-landlock 0.10.0 (README read) | external binary via os/exec | as above | CLI only, maintenance unclear | not affected | Not needed: use the library |
| sandboxec | Landlock CLI/library: not fetched (UNVERIFIED) | | | | | |
| firejail | GPL-2.0; SUID program; ~508 open issues; CVE-2021-26910 cited (README read) | external setuid binary | ns, seccomp, caps, optional AppArmor/Landlock | setuid root = large attack surface; GPL; can't be bundled unprivileged | setuid path avoids userns, but needs install as root: not for a self-contained app | No |
| nsjail | Apache-2.0; "not an official Google product"; needs CLONE_NEWUSER or root (README read) | external binary | ns, cgroups, rlimits, seccomp (Kafel), pasta net | same userns dependency | FAILS the same way | No (it is what inkwell already approximates) |
| minijail | ChromeOS: not fetched (UNVERIFIED) | | | | | |
| gVisor runsc | Apache-2.0; rootless needs user namespaces; `runsc --rootless` unsupported for `create`, no netstack (rootless docs, read) | external binary (OCI bundle) | user-space kernel | big (tens of MB: UNVERIFIED), OCI model, userns again | FAILS (userns) | No |
| systemd-run --user | LGPL; user units: `ProtectSystem=` not available; `PrivateTmp/ProtectHome` need `PrivateUsers=` = unprivileged userns (systemd.exec man, read) | `systemd-run` command, D-Bus | cgroup limits (MemoryMax, TasksMax) work when the user manager has delegation; `NoNewPrivileges` | no mount ns without userns; absent in containers/CI (inkwell already saw no delegated cgroup) | `Private*` FAILS; cgroup limits unaffected | Only for memory/CPU limits where delegation exists |
| Flatpak / Snap (whole app) | Flatpak sandbox: no network, filtered D-Bus, limited syscalls, files only app dir, portals (docs.flatpak.org, read); Snap strict confinement: page not read (UNVERIFIED) | N/A: packaging change; `flatpak-spawn --sandbox [--no-network]` runs a command "fully sandboxed" (man7, read; `--host` needs the org.freedesktop.Flatpak interface) | whole app and child | Changes distribution: inkwell promises deb/rpm self-contained; Flatpak needs its runtime; the Flatpak sandbox uses bubblewrap and is exempt from the Ubuntu restriction through its own profile (inference from the Discourse page naming flatpak: UNVERIFIED detail) | Fine inside Flatpak | Not for deb/rpm; maybe an extra channel |
| seccomp profile generators | e.g. Docker default, libseccomp: not fetched | | | | | seccomp is already in inkwell: the arm64 list bug is a table error to fix by test |

### B.2 macOS

- App Sandbox (entitlements) applies to the whole app and to children that opt in with the inherit entitlement; a sandboxed
  Wails app would lose the file access model inkwell needs unless it uses user-selected file entitlements. Details of
  `com.apple.security.inherit`: not found in this research (UNVERIFIED).
- `sandbox-exec` / Seatbelt: man page marks it deprecated, but `sandbox_init` still works on macOS Sequoia 15 per a
  developer report; children inherit the profile (search snippets, UNVERIFIED depth). Chromium uses Seatbelt with SBPL
  profiles, `(deny default)`, applied in the helper before loading the framework (Chromium design doc, read). Bazel uses
  sandbox-exec (jmmv.dev 2019, snippet). Anthropic sandbox-runtime generates profiles dynamically (README read). Practical
  for a Go child: spawn `/usr/bin/sandbox-exec -p <profile> typst ...`, no cgo, no entitlement. Risk: Apple can remove it
  (no stated date: UNVERIFIED).
- Endpoint Security: a system-extension monitoring framework for security products, needs special entitlement and
  notarisation; not for sandboxing a child (general knowledge, not fetched).

### B.3 Windows

- AppContainer: isolates files, registry, network (capabilities), devices, processes, windows (Microsoft Learn page,
  read); LPAC is stricter. A desktop app creates the child with a security-capabilities attribute at process creation.
  Chromium composes restricted token + job object + alternate desktop + integrity level + AppContainer, as a static C++
  library, user-mode only, no admin (Chromium design doc, read). Not usable as a Go library.
- Job Objects: memory and process limits, kill-on-close: Go options: `golang.org/x/sys/windows`, `aoldershaw/proclimit`
  (Linux cgroups + Windows job objects), hcsshim's `internal/jobobject` (internal, cannot be imported) (search results,
  UNVERIFIED in detail). I found NO maintained Go library that creates AppContainer processes: it would be ~300-500 lines
  of direct Win32 calls (estimate).
- Windows Sandbox: a Hyper-V based disposable VM feature of Windows Pro/Enterprise; not embeddable (general knowledge,
  not fetched).
- Anthropic sandbox-runtime on Windows is an alpha: a dedicated local user, write-restricted token, job object, WFP
  network filter, one-time ELEVATED install (README read): not suitable for a no-admin install.

### B.4 Cross-platform wrappers

| Product | Facts | Fit |
|---|---|---|
| Anthropic `sandbox-runtime` | Apache-2.0, beta research preview; npm package/CLI `srt`; Linux = bubblewrap + socat + ripgrep + seccomp, whole netns removed, proxies; macOS = Seatbelt; Windows alpha; README says on Ubuntu 24.04+ `apparmor_restrict_unprivileged_userns` "breaks bubblewrap" and gives sysctl or AppArmor profile as the fix; nested userns sandboxes (Chromium, podman) fail by default. | Not embeddable in Go without Node; the design (deny by default, bwrap+Seatbelt) is what inkwell reimplements. Does NOT solve Ubuntu. |
| `birdcage` (Phylum) | GPL-3.0; Linux namespaces + macOS `sandbox_init`; no Windows; ARCHIVED 2026-07-06 (README read) | No: GPL, archived, Rust, userns |
| `extrasafe` | Rust, seccomp+Landlock: not fetched (UNVERIFIED) | Rust only; ideas only |
| Chromium sandbox as a library | static C++ library, Windows-only design page; Linux/macOS have separate designs (page read for Windows) | Not feasible from Go without large cgo |
| Deno permission model | A runtime's flags for JS code it runs, not a sandbox for foreign children (docs.deno.com listed, not fetched) | No |
| Docker/Podman | Needs a daemon and/or root or rootless userns/newuidmap, image storage, GBs, not installable by deb alone, absent on macOS/Windows without a VM: unacceptable for a desktop editor. Overleaf uses it server side (docs read). | No |
| wasm runtimes (wazero) | See A.2 | It is the "no OS sandbox needed" alternative |

No cross-platform Go library wrapping Landlock+Seatbelt+Job Objects was found in my searches. UNVERIFIED that none exists:
search was shallow (no GitHub code search done).

---

## C. How comparable tools do it (documented only)

| Tool | Approach | Source |
|---|---|---|
| Typst web app | Compiler as WASM in the browser (dev.to profile snippet; no official architecture page found: UNVERIFIED) | search |
| typst.ts, typst-wasm | Same compiler as WASM for browser/Node/serverless; typst-wasm mentions "sandboxes for AI agents" | repo, forum |
| Overleaf Server Pro | One Docker container per compile via Docker socket; rationale "LaTeX can run arbitrary shell commands"; without it users can reach the main container's filesystem, network, env ("danger-level") | docs.overleaf.com (read) |
| Pandoc | `--sandbox`: readers/writers read only command-line files; does NOT limit filters or PDF production; data files must be embedded | pandoc manual (read) |
| Figma plugins | Realms shim replaced by QuickJS compiled to WASM, "somewhat slower, intrinsically more secure" | figma.com blog (read) |
| Zed | Extensions are Rust compiled to `wasm32-wasip2` (runtime not named on the page); language servers are separate native processes, sandboxing not stated | zed.dev docs (read) |
| VS Code + tinymist | Workspace Trust docs fetch 404: UNVERIFIED. Known: the native tinymist runs as a normal process (general knowledge). Web build exists for vscode.dev | - |
| Obsidian, Jupyter | not researched: UNVERIFIED | - |

Is "compiler in WASM" the norm? For web products and for plugin systems (Figma, Zed, Typst's own plugins): yes. For
desktop editors that run a compiler or language server as a child (VS Code, Zed LSPs): the norm is no sandbox. inkwell is
stricter than all of them; that is a product decision (documents received from other people), not an industry baseline.

---

## D. Recommendation

### Ranking

1. **Rebuild the native sandbox as tiers, Landlock+seccomp+rlimit first, namespaces optional (recommended default).**
   Why: it removes the actual cause of three of your four breakages (userns on Ubuntu; GitHub runners without cgroup scope
   if cgroup becomes best-effort; the arm64 seccomp table becomes a tested table). `go-landlock` is MIT and maintained;
   the Ubuntu restriction does not apply to Landlock/seccomp. Windows gets a Job Object (memory, process count,
   kill-on-close) + restricted token (+ AppContainer later); macOS keeps Seatbelt via `sandbox-exec` with a deny-default profile.
   Risks: Landlock network rules need kernel >= 6.7 (so also seccomp deny of `socket` for AF_INET*, i.e. fix inside the
   syscall list) and Landlock FS needs >= 5.13; no memory cap without cgroup (use `RLIMIT_AS`, which you have); you still
   own ~1 500-2 500 lines (estimate: it is mostly deleting the namespace and mount code, not adding).
2. **Typst compiler as WASM under wazero as the portable tier (decide after the spike).** It gives one implementation for
   Linux/macOS/Windows and amd64/arm64 with a hard capability model (no syscalls except those you mount), a memory page
   cap and a context timeout. Risks: no proof of speed, you own a Rust wrapper crate and its build, fonts and `@preview`
   packages become your resolver, no threads, big embedded artifact, wazero compile-time at startup. Keep tinymist native
   (tier 1) until the web build's gaps are known; if tinymist runs native only, the sandbox problem for it remains, so
   WASM alone does not remove the OS problem for the language server.
3. **WASM in the webview Worker.** Elegant for the preview (the UI already lives there) but it puts hostile-document CPU/memory
   into the UI process, depends on typst.ts rc maturity (0.8.0-rc4), and WebKitGTK's own bwrap sandbox is exposed to the same
   Ubuntu problem. Use it only if the wazero path fails the spike.
4. **bwrap/sandbox-runtime wrappers as an optional extra tier** (stronger isolation where available and permitted); never
   the only tier. On Ubuntu the user or package would have to ship an AppArmor profile (an admin step, and for a deb
   possibly a package-shipped profile: UNVERIFIED how Ubuntu treats profiles for third-party debs).
5. Not recommended: firejail (setuid GPL), gVisor, nsjail, Docker/Podman, birdcage (archived, GPL), Flatpak/Snap as the only
   delivery (conflicts with deb/rpm promise).

### What to measure first (spike, in this order)

1. Ubuntu 24.04 stock VM: run inkwell's current native sidecar under Landlock + seccomp + rlimit only (no namespaces):
   does it run the fixture, are all calls blocked as expected (use inkwell's escape tests), on arm64 too.
2. `typst-go-wasm`-style wrapper (or typst.ts node bundle) under wazero compiler mode: compile the fixture project; record
   wall time (cold and warm, with compilation cache), peak RSS, PDF byte-equality or visual diff versus the pinned CLI,
   and behaviour when the fixture loops or allocates until the page cap. Repeat on arm64 and Windows amd64.
3. typst.ts in WebKitGTK 6 (inside a Wails window): same fixture, plus Worker kill and `Memory maximum` test.
4. tinymist-web in a Worker or wazero: feature-gap list: completion in `#import "@preview/..."`, diagnostics across
   multiple files, hover, formatting; startup time and RSS versus native tinymist.
5. Package path: resolve `@preview` from a local store through the host (Go over the bridge, or wazero FS mount).

### Honest size estimates (mine, not sourced)

| Option | Work | Lines (Go/Rust) | Ongoing cost |
|---|---|---|---|
| Tier rework (Landlock/seccomp/rlimit/Job Object/Seatbelt) | 1-2 weeks | -1 500 to +1 000 net | seccomp tables per arch kept under test |
| WASM spike | 1-2 days | 300-600 | throwaway |
| wazero compiler tier, Typst only | 2-4 weeks | 1 500-3 000 Go + 500-1 000 Rust | own the Rust wrapper, track Typst releases, fonts/packages |
| tinymist in WASM | open-ended | unknown | depends on spike item 4 |
| Webview Worker tier | 2-3 weeks | 1 000-2 000 TS | typst.ts rc churn |

### What I could not verify (listed so nothing is taken as fact)

typst.ts PDF export support and package-resolution API; any WASM-vs-native Typst timing; wazero threads support; whether the
official typst CLI builds as wasm32-wasip1 unmodified; typst-go-wasm's runtime; WebKitGTK default sandbox state inside Wails
v3 and its exact interaction with Ubuntu's profile; whether a bundled bwrap matches Ubuntu's `bwrap` profile; Snap strict
confinement details; macOS `com.apple.security.inherit`; sandboxec, minijail, extrasafe, Obsidian, Jupyter, VS Code
Workspace Trust; gVisor's binary size; Landlock network ABI on the Ubuntu 24.04 kernel (6.8 assumed, not checked).

### Sources read (main ones)

typst.ts https://github.com/Myriad-Dreamin/typst.ts ; npm registry and jsDelivr API for `@myriaddreamin/typst-ts-web-compiler` ;
tinymist releases/issues via GitHub API ; https://github.com/varunbpatil/typst-go-wasm ; https://wazero.io/ ,
https://github.com/wazero/wazero , https://pkg.go.dev/github.com/tetratelabs/wazero ; https://arcjet.com/blog/lessons-from-running-webassembly-in-production-with-go-wazero ;
https://github.com/anthropic-experimental/sandbox-runtime ; https://discourse.ubuntu.com/t/understanding-apparmor-user-namespace-restriction/58007 ;
https://bugs.launchpad.net/bugs/2046477 ; https://etbe.coker.com.au/2024/04/24/ubuntu-24-04-bubblewrap/ ;
https://github.com/containers/bubblewrap ; https://github.com/landlock-lsm/go-landlock ; https://github.com/Zouuup/landrun ;
https://github.com/netblue30/firejail ; https://github.com/google/nsjail ; https://gvisor.dev/docs/user_guide/rootless/ ;
https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html ; https://man7.org/linux/man-pages/man1/flatpak-spawn.1.html ;
https://docs.flatpak.org/en/latest/sandbox-permissions.html ; https://github.com/phylum-dev/birdcage ;
https://chromium.googlesource.com/chromium/src/+/main/docs/design/sandbox.md ; https://chromium.googlesource.com/chromium/src/+/main/sandbox/mac/seatbelt_sandbox_design.md ;
https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-isolation ; https://typst.app/docs/reference/foundations/plugin/ ;
https://github.com/typst/typst/releases ; https://forum.typst.app/t/typst-documentation-should-mention-security-model/8521 ;
https://forum.typst.app/t/webassembly-auto-download-and-build-typst-packages/7655 ; https://docs.overleaf.com/on-premises/configuration/overleaf-toolkit/server-pro-only-configuration/sandboxed-compiles ;
https://pandoc.org/MANUAL.html ; https://www.figma.com/blog/an-update-on-plugin-security/ ; https://zed.dev/docs/extensions/developing-extensions ;
https://blogs.gnome.org/mcatanzaro/2020/03/31/sandboxing-webkitgtk-apps/ ; https://webkitgtk.org/reference/webkit2gtk/stable/method.WebContext.set_sandbox_enabled.html
