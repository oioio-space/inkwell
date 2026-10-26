package app

import (
	"errors"
	"testing"
)

type fakeWindows struct {
	opened, closed int
	failOpen       error
}

func (f *fakeWindows) OpenPublic() error  { f.opened++; return f.failOpen }
func (f *fakeWindows) ClosePublic() error { f.closed++; return nil }

func TestPresentServiceOpensAndClosesThePublicWindow(t *testing.T) {
	w := &fakeWindows{}
	s := NewPresentService(w)
	got, err := s.Open()
	if err != nil || !got.Native || w.opened != 1 {
		t.Fatalf("Open() = %+v, %v; opened %d times", got, err, w.opened)
	}
	if err := s.Close(); err != nil || w.closed != 1 {
		t.Errorf("Close() = %v; closed %d times", err, w.closed)
	}
}

func TestPresentServiceReportsAWindowThatDoesNotOpen(t *testing.T) {
	s := NewPresentService(&fakeWindows{failOpen: errors.New("no display")})
	if _, err := s.Open(); !errors.Is(err, ErrNoPublicWindow) {
		t.Errorf("Open() error = %v, want ErrNoPublicWindow", err)
	}
}

func TestPresentServiceWithoutWindowsLeavesItToThePage(t *testing.T) {
	s := NewPresentService(nil)
	if got, err := s.Open(); err != nil || got.Native {
		t.Errorf("Open() = %+v, %v, want not native and no error", got, err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}
