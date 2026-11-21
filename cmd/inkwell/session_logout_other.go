//go:build !linux || server

package main

import (
	"context"
	"log/slog"
)

func watchSessionLogout(context.Context, *closeCoordinator, *slog.Logger) {}
