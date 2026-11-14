package main

import "unsafe"

// raiser brings the window to the front for an open request (plan section 13): on the main thread,
// without waiting for it, it presents the native window as the launch whose activation token came
// with the request. The token is what lets a desktop raise the window instead of only saying that
// it is ready; a raiser that drops it is the defect of 2026-11-13.
type raiser struct {
	// invoke runs f on the main thread, without waiting for it.
	invoke func(f func())
	// window is the native window: nil before it exists and once it is destroyed.
	window func() unsafe.Pointer
	// present brings window to the front with the launch's token, "" when it gave none.
	present func(window unsafe.Pointer, token string)
}

func (r raiser) raise(token string) {
	r.invoke(func() {
		if w := r.window(); w != nil {
			r.present(w, token)
		}
	})
}
