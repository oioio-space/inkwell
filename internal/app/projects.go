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
	p *project.Project
}

// NewProjects returns the service for the open project p.
func NewProjects(p *project.Project) *Projects { return &Projects{p: p} }

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
