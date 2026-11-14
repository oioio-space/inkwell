// Package activate presents a native window on behalf of the launch that asked for it (ADR 0037,
// amendment of 2026-11-13; plan docs/plans/2026-11-11-oi-single-instance-open.md section 13).
//
// A desktop does not let a running application take the focus by itself: it gives the launcher
// an activation token for the application it starts ($XDG_ACTIVATION_TOKEN on Wayland,
// $DESKTOP_STARTUP_ID on X11), and raises a window only for a request that carries a token it
// knows. A second launch of Inkwell forwards its token to the running one (internal/instance),
// which presents its window with it: the window comes to the front and the launch feedback (busy
// cursor) ends. Without the token GNOME only says « "Inkwell" est prêt ».
//
// The token can only be given to the toolkit that owns the window (GTK 4 on Linux), and Wails
// wraps no such call. The package reaches GTK without cgo, through purego (the user's decision of
// 2026-11-13: no cgo exception): it looks the two calls up in the GTK the process has already
// loaded and never loads another. It starts no process and holds no state.
package activate
