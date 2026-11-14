// Package launch turns the arguments of a launch into what the window does (plan
// docs/plans/2026-11-11-oi-single-instance-open.md, decision table R1-R9 and R16, section 4.2).
//
// [Resolve] classifies each argument against the filesystem: a file of the open project, its
// folder, a file or folder of another project, a file in no project, a plain folder, or refused
// with a reason. [Startup] picks the project a first launch opens (R2), and [Plan] reduces the
// classification to at most one file to show and at most one notice, which the webview turns
// into a toast from its own catalogue. Resolve only decides: every later access to a file goes
// through its project's os.Root. The package opens no network and writes no text for people.
//
//	intents := launch.Resolve(req.Args, int(req.Dropped), launch.Env{Cwd: req.Cwd, Home: home, Docs: docs, Current: dir})
//	out := launch.Plan(intents) // out.Show: a slash path in the project; out.Notices: what the window says
package launch

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/oioio-space/inkwell/internal/instance"
)

const (
	// maxPath is Linux PATH_MAX, the terminating NUL included: a path of maxPath bytes or more is
	// refused (and a launch cuts a longer argument to instance.MaxArg = maxPath bytes, so that it is
	// still refused here).
	maxPath = instance.MaxArg
	// maxLevels bounds the folders the walk up to a project examines.
	maxLevels = 64
	// internalDir is the private folder of a project.
	internalDir = ".inkwell"
	// manifest marks a folder as a project.
	manifest = internalDir + "/project.json"
)

// Kind is what an argument is to the window.
type Kind uint8

// The kinds of argument (decision table, rows R3-R9).
const (
	// Show is a file of the current project: the window shows it (R3).
	Show Kind = iota + 1
	// Current is the current project's folder, or a folder in it: the window is raised (R4).
	Current
	// OtherProject is a file or folder of another project (R5).
	OtherProject
	// Loose is a file in no project (R6).
	Loose
	// Folder is a folder that is not a project, $HOME and / included (R7).
	Folder
	// Refused is an argument the window cannot take, for its Reason (R9).
	Refused
)

var kindNames = [...]string{Show: "show", Current: "current", OtherProject: "otherProject", Loose: "loose", Folder: "folder", Refused: "refused"}

func (k Kind) String() string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return "unknown"
}

// MarshalText writes the kind by name, for the webview.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// Reason is why an argument is refused.
type Reason uint8

// The reasons of row R9.
const (
	// Missing: no such file (or a path through a file).
	Missing Reason = iota + 1
	// Unreadable: the person may not read it.
	Unreadable
	// Special: not a regular file or a folder (a fifo, a socket, a device).
	Special
	// Internal: inside a project's .inkwell folder.
	Internal
	// TooLong: a path of 4096 bytes or more.
	TooLong
	// TooMany: arguments beyond the 32 taken at once; Count says how many.
	TooMany
	// BadName: a file of the open project whose path is not UTF-8: the webview, which gets paths
	// as JSON text, would get another path.
	BadName
)

var reasonNames = [...]string{Missing: "missing", Unreadable: "unreadable", Special: "special", Internal: "internal", TooLong: "tooLong", TooMany: "tooMany", BadName: "badName"}

func (r Reason) String() string {
	if int(r) < len(reasonNames) && reasonNames[r] != "" {
		return reasonNames[r]
	}
	return ""
}

// MarshalText writes the reason by name, for the webview ("" for none).
func (r Reason) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// Intent is one argument, classified.
type Intent struct {
	Kind Kind
	// Name is the base name of the argument as given, for the toast.
	Name string
	// Project is the absolute, canonical folder of the project holding the argument, if any.
	Project string
	// Rel is the slash path of a file inside Project ("" for a folder).
	Rel string
	// Reason is set for Refused.
	Reason Reason
	// Count is, for TooMany, the number of arguments not taken.
	Count int
}

// Env is where the arguments are resolved: absolute folders, "" when unknown.
type Env struct {
	// Cwd is the working directory of the launch that gave the arguments (forwarded by a second
	// launch, never the primary's own).
	Cwd string
	// Home is the person's home folder: the walk to a project stops there.
	Home string
	// Docs is the folder of the projects Inkwell opens.
	Docs string
	// Current is the folder of the open project; "" before one is open.
	Current string

	// device returns the filesystem of a path; nil means stat's device number.
	device func(string) (uint64, bool)
}

// Resolve classifies args, in their order, against env: total (every non-empty argument gets an
// intent; the arguments beyond instance.MaxArgs, plus dropped, become one TooMany intent at the
// end), deterministic, and never an error.
func Resolve(args []string, dropped int, env Env) []Intent {
	env = env.canonical()
	var out []Intent
	extra := max(dropped, 0)
	for _, a := range args {
		switch {
		case a == "":
		case len(out) == instance.MaxArgs:
			extra++
		default:
			out = append(out, env.classify(a))
		}
	}
	if extra > 0 {
		out = append(out, Intent{Kind: Refused, Reason: TooMany, Count: extra})
	}
	return out
}

// canonical resolves the symlinks of env's folders, so that they compare with resolved paths.
func (e Env) canonical() Env {
	for _, p := range []*string{&e.Home, &e.Docs, &e.Current} {
		if *p == "" {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(*p); err == nil {
			*p = resolved
		} else {
			*p = filepath.Clean(*p)
		}
	}
	if e.device == nil {
		e.device = device
	}
	return e
}

func (e Env) classify(arg string) Intent {
	in := Intent{Name: filepath.Base(arg)}
	refuse := func(r Reason) Intent { return Intent{Kind: Refused, Name: in.Name, Reason: r} }
	path := arg
	if !filepath.IsAbs(path) {
		if !filepath.IsAbs(e.Cwd) {
			return refuse(Missing) // the launch could not tell its working directory
		}
		path = filepath.Join(e.Cwd, path)
	}
	path = filepath.Clean(path)
	if len(arg) >= maxPath || len(path) >= maxPath {
		return refuse(TooLong)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return refuse(reasonOf(err))
	}
	if internal(resolved) {
		return refuse(Internal)
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return refuse(reasonOf(err))
	}
	isDir := info.IsDir()
	if !isDir && !info.Mode().IsRegular() {
		return refuse(Special)
	}
	if err := readable(resolved); err != nil {
		return refuse(reasonOf(err))
	}
	in.Project = e.project(resolved, isDir)
	switch {
	case in.Project == "" && isDir:
		in.Kind = Folder
	case in.Project == "":
		in.Kind = Loose
	case in.Project == e.Current && isDir:
		in.Kind = Current
	default:
		in.Kind = OtherProject
		if in.Project == e.Current {
			in.Kind = Show
		}
		if !isDir {
			rel, err := filepath.Rel(in.Project, resolved)
			if err != nil {
				return refuse(Unreadable)
			}
			in.Rel = filepath.ToSlash(rel)
			if in.Kind == Show && !utf8.ValidString(in.Rel) {
				return refuse(BadName)
			}
		}
	}
	return in
}

// reasonOf maps a filesystem error to the reason of the toast: anything but a refused access or
// an over-long path (a symlink loop, for instance) leads nowhere, so it is missing.
func reasonOf(err error) Reason {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return Unreadable
	case errors.Is(err, syscall.ENAMETOOLONG):
		return TooLong
	}
	return Missing
}

// internal reports whether path is, or is inside, a .inkwell folder.
func internal(path string) bool {
	for part := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		if part == internalDir {
			return true
		}
	}
	return false
}

// readable opens path for reading and closes it. Only regular files and folders get here, so the
// open cannot block (no fifo).
func readable(path string) error {
	f, err := os.Open(path) // #nosec G304 -- the person's own argument, opened only to test access
	if err != nil {
		return err
	}
	return f.Close()
}

// project walks up from path (a folder, or the folder of a file) to the nearest folder holding
// .inkwell/project.json and returns it, or "". The walk examines at most maxLevels folders and
// stops before $HOME and the filesystem root, and at a mount point (a change of device), whose
// own folder is still examined.
func (e Env) project(path string, isDir bool) string {
	dir := path
	if !isDir {
		dir = filepath.Dir(path)
	}
	for range maxLevels {
		parent := filepath.Dir(dir)
		if dir == e.Home || parent == dir {
			return ""
		}
		if info, err := os.Stat(filepath.Join(dir, manifest)); err == nil && info.Mode().IsRegular() {
			return dir
		}
		d, ok := e.device(dir)
		pd, pok := e.device(parent)
		if !ok || !pok || d != pd {
			return ""
		}
		dir = parent
	}
	return ""
}

// Startup returns the folder, in docs, of the project a first launch opens: the project under
// docs (a direct sub-folder) holding the first argument (R2); "" means the most recent one (R1).
func Startup(intents []Intent, docs string) string {
	if len(intents) == 0 || intents[0].Project == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(docs); err == nil {
		docs = resolved
	}
	if filepath.Dir(intents[0].Project) != filepath.Clean(docs) {
		return ""
	}
	return filepath.Base(intents[0].Project)
}

// Notice is the toast of a request: the first argument the window did not show, and how many
// others it did not show either. The webview writes its text.
type Notice struct {
	Kind   Kind   `json:"kind"`
	Reason Reason `json:"reason,omitzero"`
	Name   string `json:"name,omitzero"`
	// Count is, for TooMany, the number of arguments not taken.
	Count int `json:"count,omitzero"`
}

// Outcome is what the window does with a request, besides coming to the front: at most one file
// to show, and a notice for every other argument, in order (at most 33: 32 arguments and the
// TooMany of the rest). The window says them in one toast per kind, naming the files.
type Outcome struct {
	// Show is the slash path, in the current project, of the file to show; "" for none.
	Show    string   `json:"show,omitzero"`
	Notices []Notice `json:"notices,omitzero"`
}

// Plan reduces the intents of one request (R3-R9, R16): the first file of the current project is
// shown (one document at a time), the current project's folder only raises the window, and every
// other argument gets its notice, in order (TooMany counting the arguments not taken).
func Plan(intents []Intent) Outcome {
	var out Outcome
	for _, in := range intents {
		switch {
		case in.Kind == Current:
			continue
		case in.Kind == Show && (out.Show == "" || out.Show == in.Rel):
			out.Show = in.Rel
			continue
		}
		out.Notices = append(out.Notices, Notice{Kind: in.Kind, Reason: in.Reason, Name: in.Name, Count: in.Count})
	}
	return out
}
