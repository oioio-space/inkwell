package app

import (
	"errors"
	"testing"
)

func TestRevealLinuxAsksTheFileManagerOverDBus(t *testing.T) {
	var shown, opened []string
	err := revealLinux("/home/u/p/a b.exe",
		func(uri string) error { shown = append(shown, uri); return nil },
		func(dir string) error { opened = append(opened, dir); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 || shown[0] != "file:///home/u/p/a%20b.exe" || len(opened) != 0 {
		t.Fatalf("shown %v, opened %v", shown, opened)
	}
}

func TestRevealLinuxFallsBackToTheParentFolder(t *testing.T) {
	var opened []string
	err := revealLinux("/home/u/p/a.exe",
		func(string) error { return errors.New("no file manager on the bus") },
		func(dir string) error { opened = append(opened, dir); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0] != "/home/u/p" {
		t.Fatalf("opened %v: the folder, never the file, goes to xdg-open", opened)
	}
}
