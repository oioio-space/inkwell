//go:build server

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// newRaiser raises nothing in the server build: its window is a browser tab.
func newRaiser(*application.WebviewWindow) raiser {
	return raiser{invoke: func(func()) {}}
}
