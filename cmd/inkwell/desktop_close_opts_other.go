//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func closeWindowsOptions(_ *closeCoordinator) application.WindowsOptions {
	return application.WindowsOptions{}
}
