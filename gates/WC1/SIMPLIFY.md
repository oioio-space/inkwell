# Diff : 1ff5740..

Reviewer: Codex, implementation-family self-review, 2026-11-21.

The implementation starts at the corrected freeze, `02475f6`. This review covers the full WC1
diff from `1ff5740`, including the freeze restored on main. The independent Opus code review
returned MERGEABLE with no major findings. Codex checked the fixes below against its minor
findings. The integrator owns the staged-diff attestation.

## Applied

- **Reuse.** Carry close decisions on the existing `open` stream. The independent handoff goes
  through the established per-peer `net` writers. Keep Automerge flushing and the store's atomic
  write on their existing paths. Reuse the OI queue, shutdown callbacks, sheet draft commit, and
  the test binary's child-mode supervisor.
- **Simplification.** Keep attempts, generations, and permits under one lock. The UI reads the
  Go state, while the renderer only saves, hands off presence, and responds to abort. Share the
  bounded PC1 page teardown and switch off store replies by retiring the old connection.
- **Efficiency.** Stop an abandoned store send at the transport, then wait only for the retired
  handler's received request. Reuse the outgoing peer queue for ordered markers. The Wails
  hook never drains storage or dispatches GTK synchronously. Finalize fixture JSON with the
  supervisor's observed wait status and PIDs.
- **Altitude.** Keep the desktop owner in `cmd/inkwell`, the FIFO guarantees in the bridge,
  and the CRDT cut in the renderer. Prevent an editable replacement page from starting until
  Go has registered its stream generation. Separate a PC1 save approval from the release to
  close after the project change is recorded. Use the activation code's `RTLD_NOLOAD` approach
  for GTK calls. The architecture check rejected direct C imports outside the sidecar launcher.
- **Opus findings addressed.** Give each close stream a bounded Abort writer that waits for capacity
  outside the coordinator lock. The renderer restores input and presence on the matching
  Abort. Skip removed editors when walking a close attempt's cuts. Count a cut as empty before
  Ready or when the live page confirms its store has never started. Clear the close view and
  resume launches when the take-over is refused. Add the three specified labels to the glossary.
- **Frontend test found by WC1.** Control `performance.now()` in the after-paint exception test.
  Keep its two assertions for error reporting and continuation of the queue.

## Set aside

- A new peer-wire message or a Wails fork would enlarge the public contract for a local
  synchronization point. The barrier is carried only on the existing local bridge.
- A renderer-only recovery dialogue cannot offer an action when WebKit has stopped. Linux
  uses GTK widgets and standard destructive/default styling, with the platform work for
  macOS and Windows, including the Wails fallback dialog. → PL1
- A retired store request that never returns still blocks the replacement page's store.
  Explicit force still exits. Recovery must preserve W-5 ordering. → REL1
- A general desktop automation framework would add a second job to this block. The fixture
  drives the existing application under a private compositor with small, build-gated probes.

## Verified

- Red witnesses cover the dropped old store write, the overtaking replacement load, the
  stale reply buffer, the missing presence handoff acknowledgement, and the legacy close
  coordinator behavior. The native baseline lost the asynchronous pagehide response.
- The post-review red witnesses cover a full close stream on Abort, a replacement after
  the first of two editors closed, startup with or without an attached stream, a live
  page's empty cut and a refused take-over. The stop-on-throw mutant failed the after-paint
  continuation assertion with an empty callback trace. Focused Go and frontend replays
  passed. See `.task/wc1/review-fixes/`.
- The exact frozen `go`, `webkit-native-e2e`, and `mutants` commands passed after those fixes.
  See `.task/wc1/review-fixes/criteria/results.json`.
- `mise run fe:check` passed at default parallelism with 122 files and 1,383 passing tests
  plus one existing todo. `mise run lint` passed and inventory reported zero unfiled items.
  The resumption logs are in `.task/wc1/review-fixes/`.
- Native smoke observed window-manager close, GTK's signal, the Wails veto and consumed permit,
  toplevel destruction, a clean OS wait status, and a distinct relaunch. It also exercised disk
  error retry, blocked-store force, hung-renderer force, and crashed-renderer force.
- The close control accepts bounded messages from the registered window generation. Document
  bytes and invitation material do not appear in the product's observations. Native test hooks
  run only in the instrumented fixture. No peer-wire or frozen gate change is included.

## Guard entries for the integrator

The WebKit runner correction changes the script fingerprint. The Playwright configuration
fingerprint still matches its recorded override.

- `7069e3bdec4531c664ad45ca66d54d4f14b4dd7bcdf734749612f67e9272850a` (`frontend/playwright.config.ts`)
  accepts WC1's per-run profile root and checks that the fixture directories are populated.
  The original browser project settings, retries for the general suite, and timeouts keep
  their previous values.
- `b153e3584a83f2d2d4632898883ca97f2d874a86c9398144cccf045c24b436bd` (`scripts/wc1-webkit.sh`)
  supersedes `eaeea1fc3b8682a673ea5b2537965e1221ed8bf03b17494abff24e85ca4d3728`.
  The integrator owns review of the updated override.

## Fedora WebKit launch correction

Codex self-reviewed the runner diff on 2026-11-21. In the integrator's Fedora run,
both WebKit scenarios failed at browser launch because of missing shared libraries.
The runner now reuses `e2e:webkit` and its version-matched Playwright image locally.
On GitHub's Ubuntu runner it uses host WebKit with Playwright's dependencies, following
the CI workflow. Both browser legs keep fresh populated document and profile folders,
the scenario filter, zero retries, and the same JSON report. Native GTK runs on the host.

Shell syntax, ShellCheck and mocked Fedora and CI routing passed. The routing check
also verified that a failed WebKit command stops the criterion. Podman could not start
in the sandbox because its runtime directory is read-only. An isolated runtime also failed
at `newuidmap`. The frozen criterion still needs a real Fedora replay on the corrected runner.

## Fedora native session correction

Codex self-reviewed the runner change on 2026-11-21. The later Fedora run passed both
WebKit/Podman cases and both Chromium cases. Its first red assertion is in
`gates/WC1/native.jsonl` at 17:57:19.314, `CrashedRendererForce` at the second Alt-F4
(`wc1_native_test.go:811`). GTK showed an active parent and saving banner (mask 3) when the
test expected a visible notice with Retry focused (mask 12). The five-second observation
window runs close to the coordinator's five-second timer. The earlier native pass and this
failure show that the boundary is sensitive to scheduling. Similar mask failures were
already recorded in `.task/wc1/native-smoke-v2.log`. The test and product deadlines are
preserved. Mutter removed its virtual monitor at 17:57:19.346, after Go reported the failure.

The old bus used the host's activation directories. Fedora attempted AT-SPI activation and
ran desktop portals throughout the leg. The executor's older passing runs also show portal
and systemd activation warnings, and used a surfaceless renderer while Fedora opened a DRM
render node. `heavy.sh` sets the memory and swap caps in a systemd scope that inherits the
caller's environment. The recorded GTK assertion and normal exit 1 provide no evidence of an
OOM kill or supervisor kill. The app's Typst launcher applies its confinement to the sidecar, outside the
ancestry of Mutter and the session bus. No evidence ties the missed observation to any one
activation warning, the GPU, or the memory cap.
[systemd-run scope environment](https://www.freedesktop.org/software/systemd/man/latest/systemd-run.html)

The runner scopes its desktop variables and configuration to the native leg and uses a
short temporary directory to keep all sockets in range. The bus accepts services registered
by the fixture and has no desktop service activation. The GTK witness still checks the
widget accessibility role, labels, focus, and destructive styling directly. Mutter stays
under the session shell through the native tests. Startup checks both bus readiness and
the Wayland socket, and failures identify the native log. Shell syntax and ShellCheck passed.
A fake Mutter exit 17 was rejected before Go started, with the expected diagnostic.
A separate negative control stopped Mutter after both readiness checks. The runner
rejected the exit even though the simulated test command returned zero.
The original local native leg passed in 47.264 s and one isolated leg passed in 46.132 s.
A second isolated run with deliberately invalid caller desktop variables reached native
tests and reproduced mask 3 in the first `CrashedRendererForce` close. Mutter remained
alive and no AT-SPI or portal activation appeared. Session isolation removes the host
service dependency. The missed GTK observation can still occur on the isolated stack.
A virtual-time probe confirmed that an already recorded renderer crash still has no
failure reason after the open-request drain returns. The coordinator's expiry timer
was the next event that could resolve that attempt.
The exact reproduction attempt stopped at the read-only heavy slot lock. With writable
slot locks, the exact criterion built the browser server then stopped at Podman's read-only
runtime lock, exit 125. The host systemd scope requires a Fedora graphical-session replay.
See `STATE.local.md` for evidence.

## Renderer loss before or during close

Codex reviewed the narrow coordinator and witness changes on 2026-11-21. The original
native red was the second Alt+F4 in `CrashedRendererForce`. GTK showed the saving banner
(mask 3) while the test was waiting for the dialog (mask 12). Before changing the coordinator,
the new controlled-clock test reproduced the progress view at zero elapsed time. The
strengthened GTK walk also failed with mask 3 after observing that Go had recorded the crash.

The coordinator reuses its abort and view transitions. A recorded crash in the close scope
enters recovery before scheduling the timer or the drain. A crash during the wait ends the
active attempt and stops its timer. The recovery view has the same attempt ID on repeated
close requests and stays open until the person acts. Explicit force still grants one permit.
The only added probe observation is the crash event name, with no document or user data.

The virtual-clock tests assert immediate recovery, no timer for a known crash, the transition
from saving to recovery for a crash during the wait, and no extra dialog or permit from a
repeated request. The GTK walk waits for the crash observation and requires the dialog within
one second on both Alt+F4 requests. It checks the absence of the banner, focused Retry,
destructive Force, clean exit and durable-prefix recovery. The other fault cases retain
their progress assertions and bounds.

The focused test passed 20 repetitions. The exact frozen `go` and `mutants` commands passed,
including all ten expected violations. Lint passed and inventory reported zero unfiled items.
Five fresh private GTK sessions passed the full native walk and the frozen native evidence
checks through `scripts/heavy.sh` (5/5, 38–44 s each). See
`.task/wc1/renderer-loss/campaign/` for the transcripts and trace for each run. The sandbox
has no user systemd scope. The full Fedora gate replay and validation of the runner
fingerprint listed above remain with the integrator → WC1.
