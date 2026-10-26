package app

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/oioio-space/inkwell/internal/project"
)

// maxRootURL bounds the document URL the webview may record.
const maxRootURL = 128

var (
	errBadRootURL = errors.New("invalid document URL")
	errNoOpener   = errors.New("this system cannot open files from here")
)

// ProjectInfo describes the open project to the webview.
type ProjectInfo struct {
	// Title is the project's name.
	Title string
	// RootURL is the automerge URL of the project's document; empty until the
	// webview has created it.
	RootURL string
}

// Projects is the Wails service giving the webview the open project, whose
// storage it reaches through the "store" stream. Its exported methods are
// bound to the frontend.
type Projects struct {
	p     *project.Project
	files *Files
	// open hands a file to the system; nil when the platform has no way to.
	open func(path string) error
	// reveal shows a file in the system file manager, without running it.
	reveal func(path string) error
}

// NewProjects returns the service for the open project p. files, if not nil,
// is the blob side of the project (see [Files]); without it the project has no
// blobs to report.
func NewProjects(p *project.Project, files *Files) *Projects { return &Projects{p: p, files: files} }

// viewable lists the extensions whose system program only shows the file: documents, images,
// fonts. A file comes from another person, so anything that could run (programs, scripts,
// shortcuts, macro documents, SVG, which can carry script) is never handed over.
var viewable = map[string]bool{
	".pdf": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".bmp": true, ".tif": true, ".tiff": true, ".ico": true, ".avif": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
	".docx": true, ".odt": true, ".xlsx": true, ".ods": true,
	".pptx": true, ".odp": true,
}

// SetOpener sets how a file is handed to the system: open runs the program the user chose for
// that kind of file (only for [viewable] types), reveal shows the file in the file manager.
// The service lives in the app package, which does not import the window toolkit, so the
// toolkit's functions come from main.
func (s *Projects) SetOpener(open, reveal func(path string) error) {
	s.open, s.reveal = open, reveal
}

// OpenWithSystem opens the project file rel in the system's own program for it when it is only
// viewed (a PDF, an image, a font, an office document); any other file is shown in the file
// manager instead, never run. Only a plain file of the project is ever handed over.
func (s *Projects) OpenWithSystem(rel string) error {
	if s.open == nil || s.reveal == nil {
		return errNoOpener
	}
	abs, err := s.p.FilePath(rel)
	if err != nil {
		return err
	}
	if viewable[strings.ToLower(filepath.Ext(abs))] {
		return s.open(abs)
	}
	return s.reveal(abs)
}

// BlobProgress reports the files of the project that are not text: how many
// the project lists, how many are here, and whether the rest is over the size
// the project may take, which needs the user's agreement ([Projects.ConfirmBlobs]).
func (s *Projects) BlobProgress() BlobStatus {
	if s.files == nil {
		return BlobStatus{}
	}
	return s.files.Status()
}

// ConfirmBlobs accepts a project over its size quota, after the user agreed to
// download the rest.
func (s *Projects) ConfirmBlobs() {
	if s.files != nil {
		s.files.Confirm()
	}
}

// Current describes the open project.
func (s *Projects) Current() ProjectInfo {
	m := s.p.Meta()
	return ProjectInfo{Title: m.Title, RootURL: m.RootURL}
}

// SetRootURL records the automerge URL of the project's document, once the
// webview has created it.
func (s *Projects) SetRootURL(url string) error {
	if !validRootURL(url) {
		return errBadRootURL
	}
	return s.p.SetRootURL(url)
}

// validRootURL accepts "automerge:" followed by a base58 document id.
func validRootURL(url string) bool {
	id, ok := strings.CutPrefix(url, "automerge:")
	if !ok || id == "" || len(url) > maxRootURL {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}
