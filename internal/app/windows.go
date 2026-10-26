package app

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// Screen is a display, in the pixels of the desktop.
type Screen struct {
	ID      string
	Name    string
	X, Y    int
	Width   int
	Height  int
	Primary bool
}

// Placement is where a window goes: on Screen, either filling it or in the
// rectangle (X, Y, Width, Height) of the desktop.
type Placement struct {
	// Screen is the ID of the screen.
	Screen string
	// Fullscreen is true for a window that fills its screen, without a frame.
	Fullscreen bool
	X, Y       int
	Width      int
	Height     int
}

// Windows is what [PresentService] needs of the desktop, so that the choice of
// screens can be tested without one. The Wails implementation is in
// cmd/inkwell; a build without windows (the end-to-end tests) passes nil.
type Windows interface {
	// Screens lists the displays, primary or not.
	Screens() []Screen
	// OpenPublic opens the public window at p, or moves it there if it is open.
	OpenPublic(p Placement) error
	// PlacePresenter moves the window that holds the presenter view.
	PlacePresenter(p Placement) error
	// ClosePublic closes the public window; it is not an error if there is none.
	ClosePublic() error
}

// PresentLayout is how the two views of a presentation sit on the screens.
type PresentLayout struct {
	// Native is true when Go opens the public window itself; false means the
	// webview has to open it (a browser tab).
	Native bool
	// Dual is true when the public view has a screen of its own.
	Dual bool
	// Screens lists the displays.
	Screens []Screen
	// Public is where the public window goes; it is the zero Placement if not Native.
	Public Placement
	// Presenter is where the presenter window goes; Move is false when it stays where it is.
	Presenter Placement
	Move      bool
	// Swapped is true when the public view is on the primary screen.
	Swapped bool
}

// publicSize is the size of the public window when it has no screen of its own:
// an ordinary window that can be moved, to drag onto a projector later.
var publicSize = struct{ w, h int }{960, 540}

// planLayout decides where the views go. With two displays or more the public
// view fills the first one that is not primary and the presenter view the
// primary one; swapped puts them the other way round. With one display the
// public view is an ordinary window in its middle and nothing else moves.
func planLayout(screens []Screen, swapped bool) PresentLayout {
	l := PresentLayout{Native: true, Screens: slices.Clone(screens)}
	if len(screens) == 0 {
		l.Public = Placement{Width: publicSize.w, Height: publicSize.h}
		return l
	}
	primary := screens[0]
	for _, s := range screens {
		if s.Primary {
			primary = s
			break
		}
	}
	others := slices.DeleteFunc(slices.Clone(screens), func(s Screen) bool { return s.ID == primary.ID })
	slices.SortFunc(others, func(a, b Screen) int {
		return cmp.Or(cmp.Compare(a.X, b.X), cmp.Compare(a.Y, b.Y), cmp.Compare(a.ID, b.ID))
	})
	if len(others) == 0 {
		l.Public = Placement{
			Screen: primary.ID,
			X:      primary.X + (primary.Width-publicSize.w)/2,
			Y:      primary.Y + (primary.Height-publicSize.h)/2,
			Width:  publicSize.w, Height: publicSize.h,
		}
		return l
	}
	l.Dual = true
	pub, pres := others[0], primary
	if swapped {
		pub, pres = primary, others[0]
		l.Swapped = true
	}
	l.Public = Placement{Screen: pub.ID, Fullscreen: true, X: pub.X, Y: pub.Y, Width: pub.Width, Height: pub.Height}
	l.Presenter = Placement{Screen: pres.ID, X: pres.X, Y: pres.Y, Width: pres.Width, Height: pres.Height}
	l.Move = true
	return l
}

// PresentService is the Wails service behind "Présenter": it opens the public
// window next to the presenter view, on the other screen when there is one.
// Its exported methods are bound to the frontend. Create one with
// [NewPresentService].
type PresentService struct {
	w  Windows
	mu sync.Mutex // one move at a time
}

// NewPresentService returns the service over w, which may be nil (no windows:
// the webview opens the public view itself).
func NewPresentService(w Windows) *PresentService { return &PresentService{w: w} }

// ErrNoPublicWindow is returned by [PresentService.Open] when the desktop could not open the window.
var ErrNoPublicWindow = errors.New("the public window could not be opened")

// Layout says where the views would go, without opening anything.
func (s *PresentService) Layout() PresentLayout {
	if s.w == nil {
		return PresentLayout{}
	}
	return planLayout(s.w.Screens(), false)
}

// Open opens the public window, or moves it and the presenter window if it is
// open, as planned for swapped (see [planLayout]). Without windows it only says
// so, and the webview opens the public view itself.
func (s *PresentService) Open(swapped bool) (PresentLayout, error) {
	if s.w == nil {
		return PresentLayout{}, nil
	}
	l := planLayout(s.w.Screens(), swapped)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.w.OpenPublic(l.Public); err != nil {
		return l, fmt.Errorf("%w: %w", ErrNoPublicWindow, err)
	}
	if l.Move {
		if err := s.w.PlacePresenter(l.Presenter); err != nil {
			return l, fmt.Errorf("moving the presenter view: %w", err)
		}
	}
	return l, nil
}

// Close closes the public window.
func (s *PresentService) Close() error {
	if s.w == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.ClosePublic()
}
