//go:build !linux || server || gtk3

package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func watchCloseView(ctx context.Context, app *application.App, window *application.WebviewWindow, closer *closeCoordinator) {
	watchWailsCloseDialog(ctx, app, window, closer)
}
