package app

import (
	"errors"
	"strings"

	"github.com/oioio-space/inkwell/internal/project"
)

// maxRootURL bounds the document URL the webview may record.
const maxRootURL = 128

var errBadRootURL = errors.New("invalid document URL")

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
}

// NewProjects returns the service for the open project p. files, if not nil,
// is the blob side of the project (see [Files]); without it the project has no
// blobs to report.
func NewProjects(p *project.Project, files *Files) *Projects { return &Projects{p: p, files: files} }

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
