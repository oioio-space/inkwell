//go:build unix

package launch

import (
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// tree is the test filesystem of the decision table:
//
//	home/                      Env.Home
//	  Documents/Inkwell/       Env.Docs
//	    Cur/                   the current project: main.typ, chapters/a.typ, Nested/ (a project)
//	    Other/                 another project under Docs: o.typ
//	    plain/                 not a project: p.typ
//	  elsewhere/Proj/          a project outside Docs: e.typ
//	  loose.typ, folder/
//	outside/x.typ              outside home
type tree struct {
	root, home, docs, cur, other, nested, plain, proj, outside string
}

func newTree(t *testing.T) tree {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tr := tree{root: root, home: filepath.Join(root, "home")}
	tr.docs = filepath.Join(tr.home, "Documents", "Inkwell")
	tr.cur = filepath.Join(tr.docs, "Cur")
	tr.other = filepath.Join(tr.docs, "Other")
	tr.nested = filepath.Join(tr.cur, "Nested")
	tr.plain = filepath.Join(tr.docs, "plain")
	tr.proj = filepath.Join(tr.home, "elsewhere", "Proj")
	tr.outside = filepath.Join(root, "outside")
	for _, p := range []string{tr.cur, tr.other, tr.nested, tr.proj} {
		mkProject(t, p)
	}
	for _, f := range []string{
		filepath.Join(tr.cur, "main.typ"), filepath.Join(tr.cur, "chapters", "a.typ"), filepath.Join(tr.nested, "n.typ"),
		filepath.Join(tr.other, "o.typ"), filepath.Join(tr.plain, "p.typ"), filepath.Join(tr.proj, "e.typ"),
		filepath.Join(tr.home, "loose.typ"), filepath.Join(tr.home, "folder", "f.md"), filepath.Join(tr.outside, "x.typ"),
	} {
		mkFile(t, f)
	}
	return tr
}

func (tr tree) env() Env {
	return Env{Cwd: tr.home, Home: tr.home, Docs: tr.docs, Current: tr.cur}
}

func mkProject(t *testing.T, dir string) {
	t.Helper()
	mkFile(t, filepath.Join(dir, ".inkwell", "project.json"))
}

func mkFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// one resolves a single argument.
func one(t *testing.T, arg string, env Env) Intent {
	t.Helper()
	got := Resolve([]string{arg}, 0, env)
	if len(got) != 1 {
		t.Fatalf("Resolve(%q) = %d intents, want 1", arg, len(got))
	}
	return got[0]
}

func check(t *testing.T, arg string, got, want Intent) {
	t.Helper()
	if got != want {
		t.Errorf("Resolve(%q)\n got %+v\nwant %+v", arg, got, want)
	}
}

// TestResolveNoArgs: no argument, or only empty ones, is a plain launch: nothing to show, no toast
// (R1 at first launch, R16 for a second launch).
func TestResolveNoArgs(t *testing.T) {
	tr := newTree(t)
	for _, args := range [][]string{nil, {}, {""}, {"", ""}} {
		got := Resolve(args, 0, tr.env())
		if len(got) != 0 {
			t.Errorf("Resolve(%q) = %+v, want none", args, got)
		}
		if out := Plan(got); !reflect.DeepEqual(out, Outcome{}) {
			t.Errorf("Plan(Resolve(%q)) = %+v, want nothing", args, out)
		}
		if p := Startup(got, tr.docs); p != "" {
			t.Errorf("Startup = %q, want the most recent project", p)
		}
	}
}

// TestResolveFileInCurrentProject: a file of the open project is shown (R3).
func TestResolveFileInCurrentProject(t *testing.T) {
	tr := newTree(t)
	for arg, rel := range map[string]string{
		filepath.Join(tr.cur, "main.typ"):          "main.typ",
		filepath.Join(tr.cur, "chapters", "a.typ"): "chapters/a.typ",
		tr.cur + "/chapters/../main.typ":           "main.typ",
	} {
		got := one(t, arg, tr.env())
		check(t, arg, got, Intent{Kind: Show, Name: filepath.Base(arg), Project: tr.cur, Rel: rel})
		if out := Plan([]Intent{got}); out.Show != rel || out.Notices != nil {
			t.Errorf("Plan(%q) = %+v, want show %q and no toast", arg, out, rel)
		}
	}
}

// TestResolveCurrentProjectFolder: the open project's folder, or a folder in it, only raises the
// window (R4).
func TestResolveCurrentProjectFolder(t *testing.T) {
	tr := newTree(t)
	for _, arg := range []string{tr.cur, tr.cur + "/", filepath.Join(tr.cur, "chapters")} {
		got := one(t, arg, tr.env())
		check(t, arg, got, Intent{Kind: Current, Name: filepath.Base(arg), Project: tr.cur})
		if out := Plan([]Intent{got}); !reflect.DeepEqual(out, Outcome{}) {
			t.Errorf("Plan(%q) = %+v, want nothing", arg, out)
		}
	}
}

// TestResolveFileInOtherProject: a file or folder of another project under Docs, or of a project
// nested in the current one, is another project (R5).
func TestResolveFileInOtherProject(t *testing.T) {
	tr := newTree(t)
	cases := []struct {
		arg  string
		want Intent
	}{
		{filepath.Join(tr.other, "o.typ"), Intent{Kind: OtherProject, Name: "o.typ", Project: tr.other, Rel: "o.typ"}},
		{tr.other, Intent{Kind: OtherProject, Name: "Other", Project: tr.other}},
		{filepath.Join(tr.nested, "n.typ"), Intent{Kind: OtherProject, Name: "n.typ", Project: tr.nested, Rel: "n.typ"}},
		{tr.nested, Intent{Kind: OtherProject, Name: "Nested", Project: tr.nested}},
	}
	for _, tc := range cases {
		check(t, tc.arg, one(t, tc.arg, tr.env()), tc.want)
	}
	// Before a project is open, the current project is just another one.
	env := tr.env()
	env.Current = ""
	arg := filepath.Join(tr.cur, "main.typ")
	check(t, arg, one(t, arg, env), Intent{Kind: OtherProject, Name: "main.typ", Project: tr.cur, Rel: "main.typ"})
}

// TestResolveProjectOutsideDocs: a project outside Docs is another project, and a first launch
// cannot open it (R5).
func TestResolveProjectOutsideDocs(t *testing.T) {
	tr := newTree(t)
	file := filepath.Join(tr.proj, "e.typ")
	got := one(t, file, tr.env())
	check(t, file, got, Intent{Kind: OtherProject, Name: "e.typ", Project: tr.proj, Rel: "e.typ"})
	check(t, tr.proj, one(t, tr.proj, tr.env()), Intent{Kind: OtherProject, Name: "Proj", Project: tr.proj})
	if p := Startup([]Intent{got}, tr.docs); p != "" {
		t.Errorf("Startup(project outside Docs) = %q, want the most recent", p)
	}
}

// TestResolveLooseFile: a file in no project, inside or outside home, or in a plain folder of
// Docs (R6).
func TestResolveLooseFile(t *testing.T) {
	tr := newTree(t)
	for _, arg := range []string{filepath.Join(tr.home, "loose.typ"), filepath.Join(tr.outside, "x.typ"), filepath.Join(tr.plain, "p.typ")} {
		check(t, arg, one(t, arg, tr.env()), Intent{Kind: Loose, Name: filepath.Base(arg)})
	}
}

// TestResolvePlainFolder: a folder that is not a project, $HOME, / and Docs included (R7).
func TestResolvePlainFolder(t *testing.T) {
	tr := newTree(t)
	for _, arg := range []string{filepath.Join(tr.home, "folder"), tr.home, "/", tr.docs, tr.plain} {
		check(t, arg, one(t, arg, tr.env()), Intent{Kind: Folder, Name: filepath.Base(arg)})
	}
	// A mount root that is no project: the walk does not cross into the folder that holds it.
	mnt := filepath.Join(tr.cur, "mnt")
	if err := os.Mkdir(mnt, 0o700); err != nil {
		t.Fatal(err)
	}
	env := tr.env()
	env.device = mounted(mnt)
	check(t, mnt, one(t, mnt, env), Intent{Kind: Folder, Name: "mnt"})
}

// mounted is a device function where the folder mnt and everything below it is another
// filesystem.
func mounted(mnt string) func(string) (uint64, bool) {
	return func(p string) (uint64, bool) {
		if p == mnt || strings.HasPrefix(p, mnt+string(filepath.Separator)) {
			return 2, true
		}
		return 1, true
	}
}

// TestResolveSeveralPaths: arguments are classified one by one, in order; the first file of the
// current project is shown; every other argument has its notice, in order, so the window can name
// each one (R8, amended 2026-11-13: UX-OI-2).
func TestResolveSeveralPaths(t *testing.T) {
	tr := newTree(t)
	args := []string{
		filepath.Join(tr.cur, "chapters", "a.typ"),
		tr.cur,
		filepath.Join(tr.other, "o.typ"),
		filepath.Join(tr.home, "nothing.typ"),
		filepath.Join(tr.cur, "main.typ"),
		filepath.Join(tr.home, "loose.typ"),
	}
	got := Resolve(args, 0, tr.env())
	kinds := make([]Kind, len(got))
	for i, in := range got {
		kinds[i] = in.Kind
	}
	if want := []Kind{Show, Current, OtherProject, Refused, Show, Loose}; !slices.Equal(kinds, want) {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	out := Plan(got)
	want := Outcome{Show: "chapters/a.typ", Notices: []Notice{
		{Kind: OtherProject, Name: "o.typ"},
		{Kind: Refused, Reason: Missing, Name: "nothing.typ"},
		{Kind: Show, Name: "main.typ"},
		{Kind: Loose, Name: "loose.typ"},
	}}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("Plan = %+v, want %+v", out, want)
	}
}

// TestResolveRefused: every reason of R9, and the bounds: 32 arguments taken and the rest counted,
// paths of 4095 bytes looked up and of 4096 or more refused.
func TestResolveRefused(t *testing.T) {
	tr := newTree(t)
	env := tr.env()
	fifo := filepath.Join(tr.home, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(tr.home, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	loop := filepath.Join(tr.home, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		arg    string
		reason Reason
	}{
		{filepath.Join(tr.home, "missing.typ"), Missing},
		{filepath.Join(tr.home, "loose.typ", "x.typ"), Missing}, // through a file
		{fifo, Special},
		{sock, Special},
		{"/dev/null", Special},
		{loop, Missing},
		{filepath.Join(tr.cur, ".inkwell"), Internal},
		{filepath.Join(tr.cur, ".inkwell", "project.json"), Internal},
		{"/nope" + strings.Repeat("/a", (maxPath-5)/2), Missing}, // 4095 bytes: looked up
		{"/nope" + strings.Repeat("/a", (maxPath-5)/2) + "b", TooLong},
		{"/" + strings.Repeat("a", maxPath), TooLong},
		{"/" + strings.Repeat("a", 300), TooLong}, // a name over NAME_MAX
		// 4096 bytes as given, though shorter once cleaned: a launch cut it there, so it is refused.
		{"/" + strings.Repeat("./", (maxPath-2)/2) + "x", TooLong},
		{strings.Repeat("a/", maxPath/2), TooLong},
	}
	for _, tc := range cases {
		check(t, tc.arg, one(t, tc.arg, env), Intent{Kind: Refused, Name: filepath.Base(tc.arg), Reason: tc.reason})
	}
	// Relative arguments are measured once joined.
	if got := one(t, strings.Repeat("a", 200)+"/../"+strings.Repeat("b", 200), env); got.Reason != Missing {
		t.Errorf("short relative path: %+v, want missing", got)
	}
	deep := Env{Cwd: "/" + strings.Repeat("c", maxPath-10), Home: tr.home}
	if got := one(t, "0123456789", deep); got.Reason != TooLong {
		t.Errorf("relative path over 4096 once joined: %+v, want tooLong", got)
	}
	// Short folder names, so that only the length can refuse: 4096 bytes once joined is too long,
	// 4095 is looked up (and missing).
	deep.Cwd = "/nope" + strings.Repeat("/c", (maxPath-16)/2) // 4085 bytes
	if got := one(t, "0123456789", deep); got.Reason != TooLong {
		t.Errorf("relative path of 4096 bytes once joined: %+v, want tooLong", got)
	}
	if got := one(t, "012345678", deep); got.Reason != Missing {
		t.Errorf("relative path of 4095 bytes once joined: %+v, want missing", got)
	}
	if os.Getuid() != 0 {
		secret := filepath.Join(tr.home, "secret.typ")
		mkFile(t, secret)
		if err := os.Chmod(secret, 0); err != nil {
			t.Fatal(err)
		}
		closed := filepath.Join(tr.home, "closed")
		if err := os.Mkdir(closed, 0); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(closed, 0o700) }()
		for _, arg := range []string{secret, closed, filepath.Join(closed, "x.typ")} {
			check(t, arg, one(t, arg, env), Intent{Kind: Refused, Name: filepath.Base(arg), Reason: Unreadable})
		}
	}
	file := filepath.Join(tr.cur, "main.typ")
	for _, tc := range []struct {
		args, dropped, want int
	}{{32, 0, 0}, {33, 0, 1}, {40, 0, 8}, {32, 8, 8}, {3, 5, 5}, {0, 2, 2}, {1, -1, 0}} {
		got := Resolve(slices.Repeat([]string{file}, tc.args), tc.dropped, env)
		wantLen := min(tc.args, 32)
		if tc.want > 0 {
			wantLen++
		}
		if len(got) != wantLen {
			t.Errorf("%d args, %d dropped: %d intents, want %d", tc.args, tc.dropped, len(got), wantLen)
			continue
		}
		if tc.want > 0 {
			if last := got[len(got)-1]; last != (Intent{Kind: Refused, Reason: TooMany, Count: tc.want}) {
				t.Errorf("%d args, %d dropped: last %+v, want TooMany %d", tc.args, tc.dropped, last, tc.want)
			}
		}
	}
	out := Plan(Resolve(slices.Repeat([]string{filepath.Join(tr.home, "loose.typ")}, 40), 0, env))
	want := append(slices.Repeat([]Notice{{Kind: Loose, Name: "loose.typ"}}, 32), Notice{Kind: Refused, Reason: TooMany, Count: 8})
	if !reflect.DeepEqual(out.Notices, want) {
		t.Errorf("40 loose files: %+v, want %+v", out.Notices, want)
	}
}

// TestResolveNonUTF8Name: a file of the open project whose path is not UTF-8 is refused with its
// own reason before anything is shown: the webview could only get a different path (JSON
// replaces the invalid bytes).
func TestResolveNonUTF8Name(t *testing.T) {
	tr := newTree(t)
	bad := filepath.Join(tr.cur, "chapters", "r\xe9sum\xe9.md")
	mkFile(t, bad)
	check(t, bad, one(t, bad, tr.env()), Intent{Kind: Refused, Name: "r\xe9sum\xe9.md", Reason: BadName})
	link := filepath.Join(tr.home, "lien.md")
	if err := os.Symlink(bad, link); err != nil {
		t.Fatal(err)
	}
	check(t, link, one(t, link, tr.env()), Intent{Kind: Refused, Name: "lien.md", Reason: BadName})
	// Elsewhere nothing is shown, so nothing is refused for its name.
	loose := filepath.Join(tr.home, "\xff.typ")
	mkFile(t, loose)
	check(t, loose, one(t, loose, tr.env()), Intent{Kind: Loose, Name: "\xff.typ"})
}

// TestResolveRelativeToForwardedCwd: a relative argument is resolved against the working
// directory of the launch that gave it, never against this process's (rule D-RESOLVE-HERE).
func TestResolveRelativeToForwardedCwd(t *testing.T) {
	tr := newTree(t)
	env := tr.env()
	env.Cwd = filepath.Join(tr.cur, "chapters")
	check(t, "a.typ", one(t, "a.typ", env), Intent{Kind: Show, Name: "a.typ", Project: tr.cur, Rel: "chapters/a.typ"})
	check(t, "../main.typ", one(t, "../main.typ", env), Intent{Kind: Show, Name: "main.typ", Project: tr.cur, Rel: "main.typ"})
	check(t, ".", one(t, ".", env), Intent{Kind: Current, Name: ".", Project: tr.cur})
	env.Cwd = tr.other
	check(t, "o.typ", one(t, "o.typ", env), Intent{Kind: OtherProject, Name: "o.typ", Project: tr.other, Rel: "o.typ"})
	// This process's own working directory has no such file: resolving against it would miss.
	if wd, _ := os.Getwd(); wd == tr.other {
		t.Fatal("test runs in the forwarded directory")
	}
	// A launch that could not tell its directory: relative arguments are missing, absolute ones work.
	env.Cwd = "."
	check(t, "o.typ", one(t, "o.typ", env), Intent{Kind: Refused, Name: "o.typ", Reason: Missing})
	abs := filepath.Join(tr.other, "o.typ")
	check(t, abs, one(t, abs, env), Intent{Kind: OtherProject, Name: "o.typ", Project: tr.other, Rel: "o.typ"})
}

// TestResolveSymlinkResolved: a symlink argument is resolved and its target classified, under the
// name it was given (rule D-SYMLINK); the folders of Env may be symlinks too.
func TestResolveSymlinkResolved(t *testing.T) {
	tr := newTree(t)
	link := func(name, target string) string {
		p := filepath.Join(tr.home, name)
		if err := os.Symlink(target, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		arg  string
		want Intent
	}{
		{link("main-link.typ", filepath.Join(tr.cur, "main.typ")), Intent{Kind: Show, Name: "main-link.typ", Project: tr.cur, Rel: "main.typ"}},
		{link("cur-link", tr.cur), Intent{Kind: Current, Name: "cur-link", Project: tr.cur}},
		{link("other-link.typ", filepath.Join(tr.other, "o.typ")), Intent{Kind: OtherProject, Name: "other-link.typ", Project: tr.other, Rel: "o.typ"}},
		{link("dangling.typ", filepath.Join(tr.home, "gone.typ")), Intent{Kind: Refused, Name: "dangling.typ", Reason: Missing}},
		{link("inside", filepath.Join(tr.cur, ".inkwell", "project.json")), Intent{Kind: Refused, Name: "inside", Reason: Internal}},
	}
	for _, tc := range cases {
		check(t, tc.arg, one(t, tc.arg, tr.env()), tc.want)
	}
	// The open project named through a symlink is still the open project.
	alias := link("alias", tr.cur)
	env := tr.env()
	env.Current = alias
	arg := filepath.Join(tr.cur, "main.typ")
	check(t, arg, one(t, arg, env), Intent{Kind: Show, Name: "main.typ", Project: tr.cur, Rel: "main.typ"})
	env.Current = filepath.Join(tr.home, "no-such-project")
	check(t, arg, one(t, arg, env), Intent{Kind: OtherProject, Name: "main.typ", Project: tr.cur, Rel: "main.typ"})
}

// TestResolveWalkStops: the walk to a project does not examine $HOME or /, does not cross a mount
// point (whose own folder is examined), and gives up after 64 folders.
func TestResolveWalkStops(t *testing.T) {
	tr := newTree(t)
	// A project marker in $HOME itself is never found.
	mkProject(t, tr.home)
	arg := filepath.Join(tr.home, "loose.typ")
	check(t, arg, one(t, arg, tr.env()), Intent{Kind: Loose, Name: "loose.typ"})
	check(t, tr.home, one(t, tr.home, tr.env()), Intent{Kind: Folder, Name: "home"})
	// Without a home, the walk ends at / without examining it.
	noHome := tr.env()
	noHome.Home = ""
	arg = filepath.Join(tr.outside, "x.typ")
	check(t, arg, one(t, arg, noHome), Intent{Kind: Loose, Name: "x.typ"})

	// Mount points: a project above a mount is not reached; a project at a mount root is.
	mnt := filepath.Join(tr.cur, "mnt")
	mkFile(t, filepath.Join(mnt, "deep", "m.typ"))
	env := tr.env()
	env.device = mounted(mnt)
	arg = filepath.Join(mnt, "deep", "m.typ")
	check(t, arg, one(t, arg, env), Intent{Kind: Loose, Name: "m.typ"})
	mkProject(t, mnt)
	check(t, arg, one(t, arg, env), Intent{Kind: OtherProject, Name: "m.typ", Project: mnt, Rel: "deep/m.typ"})
	// A folder whose device cannot be read stops the walk.
	env.device = func(string) (uint64, bool) { return 0, false }
	arg = filepath.Join(tr.cur, "chapters", "a.typ")
	check(t, arg, one(t, arg, env), Intent{Kind: Loose, Name: "a.typ"})

	// 64 folders are examined: the project folder itself and its 63 nearest sub-folders.
	base := filepath.Join(tr.outside, "levels")
	mkProject(t, base)
	path := base
	for range maxLevels - 1 {
		path = filepath.Join(path, "d")
	}
	mkFile(t, filepath.Join(path, "in.typ"))
	mkFile(t, filepath.Join(path, "d", "out.typ"))
	arg = filepath.Join(path, "in.typ")
	got := one(t, arg, tr.env())
	if got.Kind != OtherProject || got.Project != base {
		t.Errorf("63 folders below the project: %+v, want found", got)
	}
	arg = filepath.Join(path, "d", "out.typ")
	check(t, arg, one(t, arg, tr.env()), Intent{Kind: Loose, Name: "out.typ"})
}

// TestResolveStartupPicksProject: a first launch opens the project under Docs of its first
// argument (R2), else the most recent one (R1).
func TestResolveStartupPicksProject(t *testing.T) {
	tr := newTree(t)
	env := tr.env()
	env.Current = ""
	cases := []struct {
		args []string
		want string
	}{
		{[]string{filepath.Join(tr.other, "o.typ")}, "Other"},
		{[]string{tr.other}, "Other"},
		{[]string{filepath.Join(tr.cur, "chapters", "a.typ"), filepath.Join(tr.other, "o.typ")}, "Cur"},
		{[]string{filepath.Join(tr.home, "missing.typ"), filepath.Join(tr.other, "o.typ")}, ""},
		{[]string{filepath.Join(tr.home, "loose.typ")}, ""},
		{[]string{filepath.Join(tr.nested, "n.typ")}, ""}, // nested: not a direct sub-folder of Docs
		{[]string{filepath.Join(tr.proj, "e.typ")}, ""},
		{[]string{tr.docs}, ""},
	}
	for _, tc := range cases {
		if got := Startup(Resolve(tc.args, 0, env), tr.docs); got != tc.want {
			t.Errorf("Startup(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
	// Docs named through a symlink.
	alias := filepath.Join(tr.root, "docs-link")
	if err := os.Symlink(tr.docs, alias); err != nil {
		t.Fatal(err)
	}
	if got := Startup(Resolve([]string{tr.other}, 0, env), alias); got != "Other" {
		t.Errorf("Startup through a symlinked Docs = %q", got)
	}
}

// TestPlanOneToastPerRequest: at most one file per request, and one notice for every other
// argument, in order (a TooMany notice counts the arguments not taken), so that the window says in
// one toast per kind what each one is, naming them (R8, R9; amended 2026-11-13: UX-OI-2).
func TestPlanOneToastPerRequest(t *testing.T) {
	show := func(rel string) Intent { return Intent{Kind: Show, Name: filepath.Base(rel), Rel: rel} }
	cases := []struct {
		name string
		in   []Intent
		want Outcome
	}{
		{"nothing", nil, Outcome{}},
		{"one file", []Intent{show("a.typ")}, Outcome{Show: "a.typ"}},
		{"folder only raises", []Intent{{Kind: Current}}, Outcome{}},
		{"the same file twice", []Intent{show("a.typ"), show("a.typ")}, Outcome{Show: "a.typ"}},
		{"a second file is not shown", []Intent{show("a.typ"), show("b.typ")}, Outcome{Show: "a.typ", Notices: []Notice{{Kind: Show, Name: "b.typ"}}}},
		{"notice before the shown file", []Intent{{Kind: Folder, Name: "f"}, {Kind: Current}, show("a.typ")}, Outcome{Show: "a.typ", Notices: []Notice{{Kind: Folder, Name: "f"}}}},
		{"each other argument named", []Intent{{Kind: Refused, Reason: Missing, Name: "m"}, {Kind: Loose, Name: "l"}, show("a.typ"), show("b.typ"), {Kind: Current}}, Outcome{Show: "a.typ", Notices: []Notice{{Kind: Refused, Reason: Missing, Name: "m"}, {Kind: Loose, Name: "l"}, {Kind: Show, Name: "b.typ"}}}},
		{"too many counts its arguments", []Intent{{Kind: Loose, Name: "l"}, {Kind: Refused, Reason: TooMany, Count: 7}}, Outcome{Notices: []Notice{{Kind: Loose, Name: "l"}, {Kind: Refused, Reason: TooMany, Count: 7}}}},
		{"too many alone", []Intent{show("a.typ"), {Kind: Refused, Reason: TooMany, Count: 3}}, Outcome{Show: "a.typ", Notices: []Notice{{Kind: Refused, Reason: TooMany, Count: 3}}}},
	}
	for _, tc := range cases {
		if got := Plan(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: Plan = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// TestPlanOutsideProjectNotice: a file of another project (R5) or in no project (R6) shows
// nothing and gives the interim toast naming it (block OI3 replaces it with the light window).
func TestPlanOutsideProjectNotice(t *testing.T) {
	tr := newTree(t)
	for arg, kind := range map[string]Kind{
		filepath.Join(tr.other, "o.typ"):    OtherProject,
		filepath.Join(tr.proj, "e.typ"):     OtherProject,
		tr.other:                            OtherProject,
		filepath.Join(tr.home, "loose.typ"): Loose,
	} {
		out := Plan(Resolve([]string{arg}, 0, tr.env()))
		want := Outcome{Notices: []Notice{{Kind: kind, Name: filepath.Base(arg)}}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("Plan(%q) = %+v, want %+v", arg, out, want)
		}
	}
}

// TestResolveNames pins the names the webview reads (frontend/src/workspace/openRequests.ts).
func TestResolveNames(t *testing.T) {
	kinds := map[Kind]string{Show: "show", Current: "current", OtherProject: "otherProject", Loose: "loose", Folder: "folder", Refused: "refused", 0: "unknown", 7: "unknown", 99: "unknown"}
	for k, want := range kinds {
		if b, _ := k.MarshalText(); string(b) != want {
			t.Errorf("Kind(%d) = %q, want %q", k, b, want)
		}
	}
	reasons := map[Reason]string{Missing: "missing", Unreadable: "unreadable", Special: "special", Internal: "internal", TooLong: "tooLong", TooMany: "tooMany", BadName: "badName", 0: "", 8: "", 99: ""}
	for r, want := range reasons {
		if b, _ := r.MarshalText(); string(b) != want {
			t.Errorf("Reason(%d) = %q, want %q", r, b, want)
		}
	}
}

// TestResolveProperty: over random trees and argument lists (seeds 1-10, 500 each), Resolve is
// total (one intent per non-empty argument up to 32, then one TooMany), deterministic, never
// panics, and every file it shows opens through its project's os.Root.
func TestResolveProperty(t *testing.T) {
	for seed := uint64(1); seed <= 10; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
		for i := range 500 {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			paths := randomTree(t, rng, root)
			n := rng.IntN(40)
			args := make([]string, n)
			for j := range args {
				args[j] = randomArg(rng, root, paths)
			}
			env := Env{Cwd: root, Home: filepath.Dir(root), Docs: root, Current: paths[rng.IntN(len(paths))]}
			dropped := rng.IntN(3)
			got := Resolve(args, dropped, env)
			if again := Resolve(args, dropped, env); !reflect.DeepEqual(got, again) {
				t.Fatalf("seed %d tree %d: not deterministic", seed, i)
			}
			nonEmpty := 0
			for _, a := range args {
				if a != "" {
					nonEmpty++
				}
			}
			want := min(nonEmpty, 32)
			if nonEmpty > 32 || dropped > 0 {
				want++
			}
			if len(got) != want {
				t.Fatalf("seed %d tree %d: %d intents for %d arguments, %d dropped", seed, i, len(got), nonEmpty, dropped)
			}
			for _, in := range got {
				checkIntent(t, in, env)
			}
			_ = Plan(got)
			_ = Startup(got, root)
		}
	}
}

func checkIntent(t *testing.T, in Intent, env Env) {
	t.Helper()
	if in.Kind < Show || in.Kind > Refused || (in.Kind == Refused) != (in.Reason != 0) {
		t.Fatalf("intent %+v", in)
	}
	if in.Kind != Show {
		return
	}
	if in.Project != env.Current {
		t.Fatalf("shown file outside the current project: %+v", in)
	}
	r, err := os.OpenRoot(in.Project)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	info, err := r.Stat(in.Rel)
	if err != nil || !info.Mode().IsRegular() || internal(in.Rel) {
		t.Fatalf("shown file %+v does not open through its project's root: %v", in, err)
	}
}

// randomTree makes up to 12 folders and files under root, some of them projects, some symlinks
// (in or out of the tree), and returns the folders.
func randomTree(t *testing.T, rng *rand.Rand, root string) []string {
	t.Helper()
	dirs := []string{root}
	var all []string
	for i := range 1 + rng.IntN(12) {
		parent := dirs[rng.IntN(len(dirs))]
		p := filepath.Join(parent, fmt.Sprintf("n%d", i))
		switch rng.IntN(5) {
		case 0, 1:
			if err := os.Mkdir(p, 0o700); err != nil {
				t.Fatal(err)
			}
			dirs = append(dirs, p)
			if rng.IntN(2) == 0 {
				mkProject(t, p)
			}
		case 2, 3:
			mkFile(t, p+".typ")
			all = append(all, p+".typ")
		case 4:
			targets := append(slices.Clone(dirs), all...)
			target := targets[rng.IntN(len(targets))]
			if rng.IntN(4) == 0 {
				target = filepath.Join(root, "absent")
			}
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
			all = append(all, p)
		}
	}
	return dirs
}

func randomArg(rng *rand.Rand, root string, dirs []string) string {
	switch rng.IntN(8) {
	case 0:
		return ""
	case 1:
		return "missing.typ"
	case 2:
		return "../" + filepath.Base(root)
	case 3:
		return filepath.Join(dirs[rng.IntN(len(dirs))], ".inkwell", "project.json")
	}
	var entries []string
	_ = filepath.WalkDir(root, func(p string, _ os.DirEntry, err error) error {
		if err == nil {
			entries = append(entries, p)
		}
		return nil
	})
	p := entries[rng.IntN(len(entries))]
	if rel, err := filepath.Rel(root, p); err == nil && rng.IntN(2) == 0 {
		return rel
	}
	return p
}
