package typst

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Severity tells an error, which stops the export, from a warning, which does not.
type Severity string

// The severities Typst reports.
const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Diagnostic is one error or warning Typst reported.
type Diagnostic struct {
	// Severity is [Error] or [Warning].
	Severity Severity `json:"severity"`
	// File is the file the message is about: relative to the project with
	// forward slashes, or "@preview/name:version/path" for a file of a package.
	// Empty when the message is about no file.
	File string `json:"file,omitzero"`
	// Message is what is wrong, one or more lines.
	Message string `json:"message"`
	// Package is "@namespace/name:version" when the message is about a package
	// that is not available offline (see [ErrMissingPackage]).
	Package string `json:"package,omitzero"`
	// Line is the 1-based line, 0 when the message has no position.
	Line int `json:"line,omitzero"`
	// Column is the 1-based column counted in Unicode characters (Typst prints
	// it 0-based; Inkwell adds one), 0 when the message has no position.
	Column int `json:"column,omitzero"`
}

// limits bounds what is kept of the sidecar's output.
type limits struct {
	maxDiagnostics int
	maxMessage     int
	maxInput       int // bytes of stderr looked at
}

var (
	// "path:line:column: error: message", the format of --diagnostic-format short.
	positionedRE = regexp.MustCompile(`^(.+?):(\d+):(\d+): (error|warning): (.*)$`)
	// "error: message", for what has no position.
	bareRE = regexp.MustCompile(`^(error|warning): (.*)$`)
	// The download Typst attempted: .../<namespace>/<name>-<version>.tar.gz .
	downloadRE = regexp.MustCompile(`https://packages\.typst\.org/([a-z]+)/([A-Za-z0-9_-]+)-(\d+\.\d+\.\d+)\.tar\.gz`)
	// "package not found (searched for <dir>/<namespace>/<name>/<version>)" .
	notFoundRE = regexp.MustCompile(`package not found \(searched for .*/([a-z]+)/([A-Za-z0-9_-]+)/(\d+\.\d+\.\d+)\)`)
)

// parseDiagnostics reads the standard error of a Typst run made with
// --diagnostic-format short. root and pkgs are the project folder and the
// package cache, so that paths can be shown the way a person writes them.
// It never keeps more than lim.maxDiagnostics, and reports when it dropped some.
// Text that is not a diagnostic is ignored.
func parseDiagnostics(stderr []byte, root, pkgs string, lim limits) (diags []Diagnostic, truncated bool) {
	if lim.maxInput > 0 && len(stderr) > lim.maxInput {
		stderr, truncated = stderr[:lim.maxInput], true
	}
	text := strings.ReplaceAll(string(stderr), "\r\n", "\n")
	var last *Diagnostic
	for line := range strings.SplitSeq(text, "\n") {
		if d, ok := parseLine(line, root, pkgs); ok {
			if len(diags) >= lim.maxDiagnostics {
				truncated, last = true, nil
				continue
			}
			diags = append(diags, d)
			last = &diags[len(diags)-1]
			continue
		}
		// A message can span lines (panic("a\nb")): the next lines continue it.
		// Bounded: a hostile document can print megabytes of them.
		if last != nil && line != "" && len(last.Message) <= lim.maxMessage {
			last.Message += "\n" + line
		}
	}
	for i := range diags {
		finish(&diags[i], root, lim.maxMessage)
	}
	return diags, truncated
}

func parseLine(line, root, pkgs string) (Diagnostic, bool) {
	if m := positionedRE.FindStringSubmatch(line); m != nil {
		ln, err1 := strconv.Atoi(m[2])
		col, err2 := strconv.Atoi(m[3])
		if err1 == nil && err2 == nil && ln >= 0 && col >= 0 && col < 1<<30 {
			return Diagnostic{
				Severity: Severity(m[4]), File: displayPath(m[1], root, pkgs),
				Line: ln, Column: col + 1, Message: m[5],
			}, true
		}
	}
	if m := bareRE.FindStringSubmatch(line); m != nil {
		return Diagnostic{Severity: Severity(m[1]), Message: m[2]}, true
	}
	return Diagnostic{}, false
}

// finish cleans the message and recognises a package that could not be loaded.
func finish(d *Diagnostic, root string, maxMessage int) {
	msg := strings.ToValidUTF8(d.Message, "�")
	msg = strings.ReplaceAll(msg, root+"/", "")
	if m := downloadRE.FindStringSubmatch(msg); m != nil {
		d.Package = "@" + m[1] + "/" + m[2] + ":" + m[3]
	} else if m := notFoundRE.FindStringSubmatch(msg); m != nil {
		d.Package = "@" + m[1] + "/" + m[2] + ":" + m[3]
	}
	if d.Package != "" {
		msg = "package " + d.Package + " is not available: Inkwell never downloads packages " +
			"(it works offline); add it to the project's .typstdeps folder"
	}
	d.Message = truncate(msg, maxMessage)
}

// truncate cuts s to at most n bytes, on a character boundary, adding an ellipsis.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

// displayPath shows a file the way the person who wrote the project would.
func displayPath(file, root, pkgs string) string {
	file = filepath.Clean(file)
	if filepath.IsAbs(file) {
		if rel, ok := within(pkgs, file); ok {
			if parts := strings.SplitN(rel, "/", 4); len(parts) == 4 {
				return "@" + parts[0] + "/" + parts[1] + ":" + parts[2] + "/" + parts[3]
			}
		}
		if rel, ok := within(root, file); ok {
			return rel
		}
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(file)
}

// within returns file relative to dir, slash-separated, when it is inside it.
func within(dir, file string) (string, bool) {
	rel, err := filepath.Rel(dir, file)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
