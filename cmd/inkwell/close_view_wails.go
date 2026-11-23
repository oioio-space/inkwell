//go:build !linux || server || gtk3

package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// terminateRenderer has no portable Wails API; the macOS and Windows force
// paths are proved natively under PL1.
func terminateRenderer(*application.WebviewWindow) bool { return false }

func watchCloseView(ctx context.Context, app *application.App, window *application.WebviewWindow, closer *closeCoordinator) {
	watchWailsCloseDialog(ctx, app, window, closer)
}
