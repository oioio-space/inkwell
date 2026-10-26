package app

import (
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
)

// RevealFile returns the function that shows a file in the system's file manager without
// running it. On Linux it asks the file manager over D-Bus (org.freedesktop.FileManager1), and
// falls back to opening the parent folder; elsewhere it is other, the toolkit's own.
func RevealFile(other func(path string) error) func(path string) error {
	if runtime.GOOS != "linux" {
		return other
	}
	return func(path string) error { return revealLinux(path, showItems, openFolder) }
}

// showItems asks the file manager, over the session bus, to select the file named by uri.
func showItems(uri string) error {
	return exec.Command("dbus-send", "--session", "--print-reply", // #nosec G204 -- fixed program; uri is a file URL of a validated project file
		"--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1",
		"org.freedesktop.FileManager1.ShowItems", "array:string:"+uri, "string:").Run()
}

// openFolder opens a folder (never a file) in the default program for folders.
func openFolder(dir string) error {
	return exec.Command("xdg-open", dir).Run() // #nosec G204 -- fixed program; dir is the folder of a validated project file
}

// revealLinux shows path in the file manager: ShowItems selects the file in its folder; if the
// bus has no file manager, the parent folder (never the file) is opened.
func revealLinux(path string, show func(uri string) error, open func(dir string) error) error {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	if err := show(uri); err == nil {
		return nil
	}
	return open(filepath.Dir(path))
}
