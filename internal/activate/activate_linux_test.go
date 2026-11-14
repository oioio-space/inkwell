//go:build linux && !server

package activate

import "testing"

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
