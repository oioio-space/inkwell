# WC1 — Save before the desktop window closes

An ordinary close immediately after typing preserves every edit the editor accepted, including
input buffered for the next paint. Reopening recovers it from the project’s Automerge store, even
in manual save mode. When saving cannot be confirmed, the person can retry or explicitly close
after seeing what may be lost. Complete rigour covers the native window, webview and Go store sharing durable
state. The concrete failures are R-E2E1-1 (last keys lost) and R-E2E2-1 (presence stranded).
[Existing witnesses](2026-11-17-e2e1.md) · [D-DRAIN](../protocol/tla/OISingleInstance.tla)

## Evidence and choice

- Wails v3.0.0 calls cancellable window hooks before dispatching observers in separate goroutines,
  including its destructor. Linux then uses `gtk_window_destroy`. The separate WebKitGTK
  `try_close` path checks `beforeunload` and emits a close signal for the embedder. The HTML
  unload algorithm dispatches `pagehide` without waiting for asynchronous application work.
  The pinned `Close()` emits `WindowClosing` with no payload. WC1 uses the existing Wails API
  and keeps approval in the native coordinator.
  [Wails index](https://v3.wails.io/llms.txt) · [Window hooks](https://wails.io/features/windows/events/#windowclosing)
  · [Pinned Go source](https://github.com/wailsapp/wails/blob/v3.0.0/v3/pkg/application/webview_window.go)
  · [GTK backing](https://github.com/wailsapp/wails/blob/v3.0.0/v3/pkg/application/linux_cgo.go)
  · [GTK close-request](https://docs.gtk.org/gtk4/signal.Window.close-request.html)
  · [WebKitGTK try_close](https://webkitgtk.org/reference/webkit2gtk/2.24.0/WebKitWebView.html#webkit-web-view-try-close)
  · [HTML unload](https://html.spec.whatwg.org/multipage/document-lifecycle.html#unloading-documents)
- VS Code lets the renderer veto shutdown while its working copies are saved or backed up.
  IntelliJ likewise saves on closing a project or exiting and autosaves during use. WC1 uses
  the same veto-and-save principle and the existing project store as recovery, with
  `open` as the control stream and the existing `store` and `net` data paths.
  [VS Code lifecycle](https://github.com/microsoft/vscode/blob/main/src/vs/platform/lifecycle/electron-main/lifecycleMainService.ts)
  · [VS Code backup tracker](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/services/workingCopy/electron-browser/workingCopyBackupTracker.ts)
  · [IntelliJ save triggers](https://www.jetbrains.com/help/idea/saving-and-reverting-changes.html)

## Rules

- **W-1 — Hold the native window.** Go cancels the first closing event synchronously with
  `RegisterHook(WindowClosing)` and starts an asynchronous attempt. The hook returns immediately.
  Duplicate close requests join the same attempt. Go sends Prepare over the window’s existing
  Streams connection, bound to `StreamConn.Window()`, its current generation and a fresh attempt ID.
  The page’s confirmation of the durable cut and goodbye, or the explicit W-6 force choice,
  approves release. Input stays frozen through release. Go installs a one-use permit for the
  approved attempt, window and generation, then calls Wails’ `Close()` to ensure a closing event
  arrives. The first closing event for that window while the permit is valid consumes it
  atomically and may proceed, including a repeated OS request arriving before the event from
  `Close()`. The hook vetoes every closing event that lacks a valid permit. The coordinator
  invalidates the permit on abort and before any new edit becomes possible, including when a
  replacement page can accept input. Every new attempt requires its own approval.
  Keep the webview, store and network alive through the handshake. For an app exit, pause
  new OI launches and drain accepted opens on the live
  page under D-DRAIN's existing 2 s bound. Log the stuck-open exception. A save ACK is still
  required. Resume OI launches on abort and give unhandled opens back when another editor survives.
- **W-2 — Save the whole editing cut.** Prepare stops accepting new input, finishes accepted
  composition and editor or sheet buffers, and flushes every dirty Automerge document. The
  close-specific flush propagates failures that the best-effort `saveNow` currently swallows.
  The page sends a save ACK of either success or failure. Success follows successful Go store
  replies covering the cut and outstanding writes. `GoStorageAdapter.save` awaits the `OpSave`
  reply sent after `KV.Save` and `WriteAtomic` with fsync. Failure identifies the affected
  documents when known. Go accepts replies only from the matching window, generation and
  attempt. On a matching failure Go immediately aborts under W-4 and issues no close permit.
  Manual save still leaves the text-file projection untouched and reopens with the unsaved marker.
  Controlled reload or navigation of the editable webview uses the same save barrier before
  replacing the page. [Page save](../../frontend/src/workspace/workspace.svelte.ts#L371)
  · [Adapter ACK](../../frontend/src/store/GoStorageAdapter.ts#L103)
  · [Store commit](../../internal/store/atomic.go)
- **W-3 — Finish presence while both paths are live.** After the relevant save ACKs, the page
  stops its active presence instances, including the sheet’s, and emits their ordinary goodbye.
  An ordered barrier on the same `net` stream covers all earlier goodbye frames and their outbound
  handoff on healthy links. Go releases only after this barrier and the matching page completion ACK.
  An unavailable peer falls back to existing link-loss/expiry semantics. On abort after a goodbye,
  restore publication for any page staying open.
- **W-4 — Resolve within 5 s.** One monotonic deadline runs from the first request, including
  all phases and target windows. Timeout, save failure or renderer loss aborts the whole attempt,
  invalidates its authorization and sends Abort to surviving pages. Keep the native windows
  and live editor state, restore input and publication on surviving pages, and show the W-6
  notice. Late ACKs cannot authorize release or a retry. Quit, intercepted SIGTERM and
  vetoable logout use this coordinator before Wails teardown. Gate Wails Quit with a fast
  `ShouldQuit` callback. Disable Wails’ default desktop signal handler and route SIGTERM to
  the same coordinator. A non-vetoable logout uses the available grace period up to 5 s and
  records an unconfirmed save if it can. An externally forced kill can recover only committed data. App-initiated
  project changes obtain the same close approval before PC1’s irreversible bridge drain and
  keep input frozen until restart.
  For app quit, drain editable windows in parallel. Issue their permits as a group only after
  all confirm before the deadline.
  A presentation-only window closes independently and has no save requirement. Closing the
  last editor also closes its dependent presentation and ends the shared runtime.
  On GNOME, answer the session query within its 1 s budget and use the end-session signal for
  the flush. The session manager can override objections. Map the equivalent OS hooks where
  available. [Pinned Wails signal handler](https://github.com/wailsapp/wails/blob/v3.0.0/v3/pkg/application/signal_handler_desktop.go)
  · [GNOME session protocol](https://gnome.pages.gitlab.gnome.org/gnome-session/re05.html)
  · [Windows session notifications](https://learn.microsoft.com/en-us/windows/win32/shutdown/wm-queryendsession)
  · [AppKit deferred termination](https://developer.apple.com/documentation/appkit/nsapplication/reply%28toapplicationshouldterminate%3A%29) → WC1
- **W-5 — Preserve received store requests across reload.** Once `Receive` yields a valid
  request, process it exactly once under the current project’s admission rules, even if the
  requesting page is replaced. Preserve FIFO order, and make a replacement page’s reads and writes
  observe the predecessor’s accepted operations before acting. Route each reply only to its
  original connection and retire old handlers without waiting on abandoned replies or the
  transport’s idle TTL. External-edit notices and N-16 leaving/refusal rules still apply.
  Wails can discard frames it still owns on teardown, so the W-2 barrier must precede app-owned
  navigation. [Pinned Wails Streams source](https://github.com/wailsapp/wails/blob/v3.0.0/v3/pkg/application/stream.go)
- **W-6 — Show progress and give the person a way out.** During the hold show
  « Enregistrement… » with visible activity in an Adwaita-style banner. Go owns the close state
  and shows native progress if the target page cannot display it, including a hung or crashed
  renderer. On abort replace progress with a persistent Adwaita-style dialog offering
  « Réessayer » and « Fermer quand même ». Retry is the safe default and starts a fresh attempt
  with its own deadline. Retry resumes the normal handshake if the original editing state
  is available and otherwise returns to the notice. It cannot reconstruct changes lost with
  the page.
  Dismissing the dialog keeps the window open and lets the person return to the editor and copy
  their text when it responds. The notice lists documents with unconfirmed changes when known,
  otherwise it says « Les dernières modifications de ce projet n’ont pas pu être vérifiées. »
  It explains that reopening the same project on this device recovers changes already saved,
  and that changes still visible can be copied from the editor. For an unresponsive page,
  copying depends on its recovery. For a crashed page, explain that changes kept only there
  cannot be recovered. For a full disk, tell the person to free space and retry.
  Reuse the existing banner and dialog patterns, with keyboard focus, an accessible progress
  label and destructive styling for « Fermer quand même ». Only explicitly activating that action
  authorizes force for the scope named in the dialog (one window or all on quit). Go issues
  distinct one-use authorization through W-1 without waiting for save, presence, renderer,
  store or network. Within 5 s of that action, destroy the target windows and, on quit or
  last-editor close, exit the process even if a synchronous store operation is stuck.
  Duplicate close, deadline, dismissal and late ACK carry no consent. If this began as a
  project change, force closes the window or app and cancels the change.
  [UX rule 5](../ux-principles.md) · [Glossary](../glossary.md)
  · [Adwaita alert dialog](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.AlertDialog.html)
  · [WebKitGTK renderer termination](https://webkitgtk.org/reference/webkit2gtk/stable/class.WebView.html)

## Acceptance

| Rules | Independent oracle |
|---|---|
| W-1 | `TestWC1W1CloseHold` uses a controlled clock. Without approval every close is vetoed. With approval the first event for the window consumes the permit, including an OS close arriving before the reissued close. The permit is single-use and expires on abort or before editing can resume. A retry requires fresh approval. |
| W-2, W-4 | `TestWC1W2SaveAck` and `TestWC1W4ExitPaths` exercise failed writes, stale and cross-window replies, timeout, the app-quit group and PC1 with a controlled clock. |
| W-3 | `TestWC1W3PresenceBarrier` checks the ordered handoff, abort and healthy-link departure. |
| W-5 | `TestWC1W5StoreAfterReload` gives the retired handler a valid write after replacement and reads it from the new connection. Also test blocked old write before a fresh load, FIFO and abandoned reply. |
| W-6 | `TestWC1W6CloseRecovery` checks failure, fresh retry, dismissal, explicit force and bounded teardown independently of the failing participants. Native tests exercise visible progress and both choices with a full disk, hung store, and hung or crashed renderer. After force verify recovery of only the durable prefix in the controlled fault fixture. |
| W-1–W-6 | Keep the Chromium regressions SAV-03b and SHEET-MU-01b. In Playwright WebKit, start the app-owned reload or departure immediately after editing and exercise the page-side barrier. Separately run both scenarios in the built Linux Wails app on WebKitGTK, with native GTK close-request and hook veto, destruction, process exit and relaunch. Verify text recovery, the unchanged manual projection and the selection disappearing while the other person’s window stays open. [Gate contract](../../gates/WC1.toml) · [Playwright browser builds](https://playwright.dev/docs/browsers) |
| W-1–W-4, W-6 | `WindowClose` explores two editable windows, group authorization, input, crashes, retry, deadlines and explicit force. Separate liveness runs show a healthy quit succeeds and an accepted force closes a dead renderer’s window. |

## Open items

- Linux uses the pinned Wails window hook and the application-level quit veto. The GTK fallback
  attaches progress and recovery controls to the native window so a renderer failure leaves both
  actions available. It resolves the public APIs in Wails' already loaded GTK and WebKitGTK via
  `RTLD_NOLOAD`, as the existing native activation code does. The native walk uses a private Mutter session and independent GTK signal and
  toplevel observations. Its private GNOME session manager exercises the one-second query and the
  end-session flush. A session manager can still override an objection, and an externally forced
  termination can recover only the committed prefix.
  [GTK close-request](https://docs.gtk.org/gtk4/signal.Window.close-request.html)
  · [Mutter remote desktop test driver](https://github.com/GNOME/mutter/blob/main/data/dbus-interfaces/org.gnome.Mutter.RemoteDesktop.xml)
  · [GNOME session protocol](https://gnome.pages.gitlab.gnome.org/gnome-session/re05.html)
- → PL1 Prove the macOS and Windows cancellation and reissue paths on their native desktops.
  The integrator approved cancelling the OS termination, running the handshake, and reissuing the
  application quit under its permit. R1 requires native proof on Linux. PL1 also owns native
  progress presentation and replacement of the Wails fallback recovery dialog.
- → REL1 A retired store request that never returns holds the replacement page's store queue.
  Preserve the accepted-request ordering in W-5 when adding recovery. The native
  « Fermer quand même » action still exits an app with a blocked store.
- → WC1 Record the owner's recovery-dialog walkthrough before integration (R14).

## Post-review implementation

- The Ready grant and close attempt share the coordinator lock. A window without a grant is
  empty, and a granted page can expressly confirm that it froze input before starting its
  store. A granted window with a missing stream still requires confirmation. When no target
  could have received an open request, the existing OI drain logs pending opens immediately.
- Abort uses the pinned stream's blocking send on one writer per connection. The writer
  keeps the latest pending abort, since a newer Prepare replaces the old page hold. A
  refused take-over clears the close state and resumes launches. The project-specific
  refusal remains visible through the existing take-over path. [Pinned Wails Streams](https://github.com/wailsapp/wails/blob/v3.0.0/v3/pkg/application/stream.go)
- The WC1 frontend replay found that the after-paint exception test used the real slice clock.
  The witness now controls that clock and still rejects a drain that exits on the exception.
- The integrator's Fedora gate found missing Playwright WebKit libraries at browser launch.
  WC1 uses the Podman-backed `e2e:webkit` task locally and host WebKit with Playwright's system
  dependencies on GitHub's Ubuntu runner. Each browser gets fresh document and profile
  directories and zero retries. The native GTK witness runs on the host.
  [Mise tasks](../../mise.toml) · [CI workflow](../../.github/workflows/ci.yml)
- In the next Fedora gate, both browser legs passed. The native crash case still showed
  the saving banner when its second close attempt reached the five-second dialog check.
  The native launcher now owns a short XDG runtime, a headless Mutter and a D-Bus session
  with no service activation directories. The fixture registers its own session manager
  and uses GTK widget APIs for the role and focus assertions. GTK's AT-SPI bridge can
  therefore be disabled for this run. The launcher clears inherited display and
  activation state, selects software rendering and checks the bus, the Wayland socket and
  Mutter's lifetime. The recovery assertions and their deadlines are preserved.
  [D-Bus session lifetime](https://dbus.freedesktop.org/doc/dbus-run-session.1.html)
  · [D-Bus activation configuration](https://dbus.freedesktop.org/doc/dbus-daemon.1.html)
  · [GTK accessibility backend](https://docs.gtk.org/gtk4/running.html#gtk_a11y)
  · [Mesa software rendering](https://docs.mesa3d.org/envvars.html)
- A crash already recorded for a target window starts directly in recovery. The coordinator
  skips the open-request drain and the save timer, so the view can only show the recovery
  dialog. A crash during a pending attempt ends the wait immediately. Both paths keep the
  attempt in recovery until the user acts, so another close request adds no dialog and
  closing still requires explicit consent. The native crash case waits for the coordinator
  to observe the WebKit termination before sending Alt+F4. It checks both close attempts
  against a one-second dialog bound. The other fault cases still check the saving banner
  and use the existing dialog bound. [WindowClose model](../protocol/tla/WindowClose.tla)
