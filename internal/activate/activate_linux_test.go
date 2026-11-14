//go:build linux && !server

package activate

import (
	"errors"
	"slices"
	"testing"
	"unsafe"
)

// Plan OI section 13, in the order a process lives it. A process that has not loaded GTK (this
// test binary at first) is refused: Present never loads a second GTK next to the application's
// own, and reports it instead of presenting nothing in silence. Once GTK is loaded, as Wails does
// at start, the two calls that carry the launch's token are found under the names and library
// Present uses, also through the no-load lookup the application makes.
func TestGTKCallsResolveOnlyInLoadedGTK(t *testing.T) {
	if _, err := load(rtldNoLoad); err == nil {
		t.Fatal("load(RTLD_NOLOAD) succeeded in a process without GTK, want an error")
	}
	for _, flags := range []int{0, rtldNoLoad} {
		gtk, err := load(flags)
		if err != nil {
			t.Fatalf("load(%#x) with GTK loaded: %v", flags, err)
		}
		if gtk.setStartupID == nil || gtk.present == nil {
			t.Fatalf("load(%#x) = %+v, want both calls bound", flags, gtk)
		}
	}
}

// A window that does not exist is never presented and is not an error.
func TestPresentNilWindow(t *testing.T) {
	if err := Present(nil, "token"); err != nil {
		t.Fatalf("Present(nil) = %v, want nil", err)
	}
}

// fakeGTK records the calls Present makes, in order.
func fakeGTK(calls *[]string) *toolkit {
	return &toolkit{
		setStartupID: func(_ unsafe.Pointer, id string) { *calls = append(*calls, "startup-id "+id) },
		present:      func(unsafe.Pointer) { *calls = append(*calls, "present") },
	}
}

// Plan OI section 13: the token goes to GTK before the window is presented (the order
// gtk_window_set_startup_id asks for), and a launch without a token presents the window without
// naming any. Dropping the token, or presenting first, is the defect of 2026-11-13.
func TestPresentGivesTheTokenThenPresents(t *testing.T) {
	window := unsafe.Pointer(new(int))
	for _, tc := range []struct {
		token string
		want  []string
	}{
		{"gnome-shell/Files/1234-5-host_TIME678", []string{"startup-id gnome-shell/Files/1234-5-host_TIME678", "present"}},
		{"", []string{"present"}},
	} {
		var calls []string
		restore := loaded
		loaded = func() (*toolkit, error) { return fakeGTK(&calls), nil }
		err := Present(window, tc.token)
		loaded = restore
		if err != nil || !slices.Equal(calls, tc.want) {
			t.Errorf("Present(window, %q) = %v with calls %v, want %v", tc.token, err, calls, tc.want)
		}
	}
}

// When GTK cannot be reached Present says so and does nothing: the caller shows the window its
// own way instead of believing it was raised.
func TestPresentReportsUnreachableGTK(t *testing.T) {
	unreachable := errors.New("no GTK")
	restore := loaded
	loaded = func() (*toolkit, error) { return nil, unreachable }
	defer func() { loaded = restore }()
	if err := Present(unsafe.Pointer(new(int)), "token"); !errors.Is(err, unreachable) {
		t.Fatalf("Present = %v, want %v", err, unreachable)
	}
}
