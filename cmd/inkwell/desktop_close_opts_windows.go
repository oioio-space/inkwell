package main

import "github.com/wailsapp/wails/v3/pkg/application"

func closeWindowsOptions(closer *closeCoordinator) application.WindowsOptions {
	opts := application.WindowsOptions{}
	if closer != nil {
		opts.WndProcInterceptor = func(_ uintptr, message uint32, ending, _ uintptr) (uintptr, bool) {
			const queryEndSession, endSession = 0x0011, 0x0016
			switch message {
			case queryEndSession:
				closer.requestAll()
				return 0, true
			case endSession:
				if ending != 0 {
					closer.requestAll()
				}
				return 0, true
			default:
				return 0, false
			}
		}
	}
	return opts
}
