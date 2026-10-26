package app

import (
	"errors"
	"fmt"
)

// Windows is what [PresentService] needs of the desktop: one extra window for the audience. The
// person moves it to the screen they want and makes it full screen themselves; nothing here
// guesses screens. The Wails implementation is in cmd/inkwell; a build without windows (the
// end-to-end tests) passes nil.
type Windows interface {
	// OpenPublic opens the public window, or brings it to the front if it is open.
	OpenPublic() error
	// ClosePublic closes the public window; it is not an error if there is none.
	ClosePublic() error
}

// PublicWindow says how the public window was opened.
type PublicWindow struct {
	// Native is true when Go opened the window itself; false means the webview has to open it
	// (a browser pop-up).
	Native bool
}

// PresentService is the Wails service behind "Présenter": it opens the public window next to the
// presenter view. Its exported methods are bound to the frontend. Create one with
// [NewPresentService].
type PresentService struct {
	w Windows
}

// NewPresentService returns the service over w, which may be nil (no windows: the webview opens
// the public view itself).
func NewPresentService(w Windows) *PresentService { return &PresentService{w: w} }

// ErrNoPublicWindow is returned by [PresentService.Open] when the desktop could not open the window.
var ErrNoPublicWindow = errors.New("the public window could not be opened")

// Open opens the public window. Without windows it only says so, and the webview opens the public
// view itself.
func (s *PresentService) Open() (PublicWindow, error) {
	if s.w == nil {
		return PublicWindow{}, nil
	}
	if err := s.w.OpenPublic(); err != nil {
		return PublicWindow{}, fmt.Errorf("%w: %w", ErrNoPublicWindow, err)
	}
	return PublicWindow{Native: true}, nil
}

// Close closes the public window.
func (s *PresentService) Close() error {
	if s.w == nil {
		return nil
	}
	return s.w.ClosePublic()
}
