package sidecar

import (
	"embed"
	"io/fs"
)

//go:embed licenses/*
var licensesFS embed.FS

// Licenses returns the license texts of the bundled programs, to show in the
// application and to install next to it: typst-LICENSE and typst-NOTICE
// (Apache-2.0, with the notices of its dependencies) and tinymist-LICENSE
// (Apache-2.0). The licenses of the bundled Typst packages and of the Inter
// font travel with the files themselves.
func Licenses() fs.FS {
	sub, err := fs.Sub(licensesFS, "licenses")
	if err != nil { // "licenses" is a constant that exists.
		panic(err)
	}
	return sub
}
