package app

import (
	"errors"
	"testing"
)

var (
	laptop   = Screen{ID: "laptop", X: 0, Y: 0, Width: 1920, Height: 1080, Primary: true}
	beamer   = Screen{ID: "beamer", X: 1920, Y: 0, Width: 1280, Height: 720}
	tv       = Screen{ID: "tv", X: 3200, Y: 0, Width: 3840, Height: 2160}
	leftSide = Screen{ID: "left", X: -1280, Y: 0, Width: 1280, Height: 1024}
)

func TestPlanLayout(t *testing.T) {
	tests := []struct {
		name       string
		screens    []Screen
		swapped    bool
		dual       bool
		public     string // screen of the public window
		fullscreen bool
		presenter  string // screen of the presenter, "" if it does not move
	}{
		{name: "PRS-30 two screens: public on the external one", screens: []Screen{laptop, beamer}, dual: true, public: "beamer", fullscreen: true, presenter: "laptop"},
		{name: "PRS-30 swapped", screens: []Screen{laptop, beamer}, swapped: true, dual: true, public: "laptop", fullscreen: true, presenter: "beamer"},
		{name: "primary listed last", screens: []Screen{beamer, laptop}, dual: true, public: "beamer", fullscreen: true, presenter: "laptop"},
		{name: "three screens: the first external one by position", screens: []Screen{tv, laptop, leftSide}, dual: true, public: "left", fullscreen: true, presenter: "laptop"},
		{name: "PRS-32 one screen: a window", screens: []Screen{laptop}, public: "laptop"},
		{name: "no screen known: a window", screens: nil},
		{name: "no primary flagged: the first is it", screens: []Screen{{ID: "a", Width: 800, Height: 600}, {ID: "b", X: 800, Width: 800, Height: 600}}, dual: true, public: "b", fullscreen: true, presenter: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planLayout(tt.screens, tt.swapped)
			if got.Dual != tt.dual || got.Swapped != (tt.swapped && tt.dual) {
				t.Errorf("planLayout() dual = %v swapped = %v, want %v", got.Dual, got.Swapped, tt.dual)
			}
			if got.Public.Screen != tt.public || got.Public.Fullscreen != tt.fullscreen {
				t.Errorf("planLayout() public = %+v, want screen %q fullscreen %v", got.Public, tt.public, tt.fullscreen)
			}
			if tt.presenter == "" && got.Move || tt.presenter != "" && (!got.Move || got.Presenter.Screen != tt.presenter) {
				t.Errorf("planLayout() presenter = %+v move %v, want screen %q", got.Presenter, got.Move, tt.presenter)
			}
			if got.Public.Width <= 0 || got.Public.Height <= 0 {
				t.Errorf("planLayout() public window has no size: %+v", got.Public)
			}
		})
	}
}

func TestPlanLayoutPutsTheWindowInTheMiddleOfTheOnlyScreen(t *testing.T) {
	got := planLayout([]Screen{{ID: "s", X: 100, Y: 50, Width: 1920, Height: 1080, Primary: true}}, false).Public
	if got.X != 100+(1920-960)/2 || got.Y != 50+(1080-540)/2 {
		t.Errorf("window at (%d, %d), want it centred", got.X, got.Y)
	}
}

func TestPlanLayoutFillsTheExternalScreen(t *testing.T) {
	got := planLayout([]Screen{laptop, beamer}, false).Public
	if got.X != 1920 || got.Width != 1280 || got.Height != 720 {
		t.Errorf("public = %+v, want the whole of the beamer", got)
	}
}

type fakeWindows struct {
	screens   []Screen
	public    []Placement
	presenter []Placement
	closed    int
	failOpen  error
}

func (f *fakeWindows) Screens() []Screen { return f.screens }
func (f *fakeWindows) OpenPublic(p Placement) error {
	f.public = append(f.public, p)
	return f.failOpen
}

func (f *fakeWindows) PlacePresenter(p Placement) error {
	f.presenter = append(f.presenter, p)
	return nil
}
func (f *fakeWindows) ClosePublic() error { f.closed++; return nil }

func TestPresentServiceOpensAndSwaps(t *testing.T) {
	w := &fakeWindows{screens: []Screen{laptop, beamer}}
	s := NewPresentService(w)
	l, err := s.Open(false)
	if err != nil || !l.Native || !l.Dual {
		t.Fatalf("Open(false) = %+v, %v", l, err)
	}
	if got := w.public[0]; got.Screen != "beamer" || !got.Fullscreen {
		t.Errorf("first public = %+v", got)
	}
	if got := w.presenter[0]; got.Screen != "laptop" {
		t.Errorf("first presenter = %+v", got)
	}
	if _, err := s.Open(true); err != nil {
		t.Fatalf("Open(true) = %v", err)
	}
	if got, want := w.public[1].Screen, "laptop"; got != want {
		t.Errorf("swapped public screen = %q, want %q", got, want)
	}
	if got, want := w.presenter[1].Screen, "beamer"; got != want {
		t.Errorf("swapped presenter screen = %q, want %q", got, want)
	}
	if err := s.Close(); err != nil || w.closed != 1 {
		t.Errorf("Close() = %v, closed %d times", err, w.closed)
	}
}

func TestPresentServiceSingleScreenLeavesThePresenterAlone(t *testing.T) {
	w := &fakeWindows{screens: []Screen{laptop}}
	l, err := NewPresentService(w).Open(false)
	if err != nil || l.Dual || len(w.presenter) != 0 {
		t.Errorf("Open() = %+v, %v; presenter moves %d", l, err, len(w.presenter))
	}
	if w.public[0].Fullscreen {
		t.Error("the public window is fullscreen on the only screen")
	}
}

func TestPresentServiceReportsAWindowThatDoesNotOpen(t *testing.T) {
	w := &fakeWindows{screens: []Screen{laptop}, failOpen: errors.New("no display")}
	if _, err := NewPresentService(w).Open(false); !errors.Is(err, ErrNoPublicWindow) {
		t.Errorf("Open() error = %v, want ErrNoPublicWindow", err)
	}
}

func TestPresentServiceWithoutWindows(t *testing.T) {
	s := NewPresentService(nil)
	if l := s.Layout(); l.Native {
		t.Errorf("Layout() = %+v, want not native", l)
	}
	if l, err := s.Open(false); err != nil || l.Native {
		t.Errorf("Open() = %+v, %v, want not native and no error", l, err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}
