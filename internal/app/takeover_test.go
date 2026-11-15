package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/trust"
)

// Tests of the take-over (plan docs/plans/2026-11-15-pc1a-take-over.md, rules N-5..N-8, and
// N-11, N-12 of docs/plans/2026-11-14-pc1-project-change-by-restart.md).

// foreignWithOps is [foreignFolder] whose member list holds a second op, so that an archive has
// more than the genesis to copy.
func foreignWithOps(t *testing.T) (proj *project.Project, folder string, creator *identity.Device) {
	t.Helper()
	proj, folder, creator = foreignFolder(t)
	list, st, err := openTrust(proj, creator)
	if err != nil {
		t.Fatal(err)
	}
	op, err := list.Issue(creator, trust.OpAdd, newDevice(t).Public(), trust.RoleWriter, "Ana")
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(st.save(op), st.close()); err != nil {
		t.Fatal(err)
	}
	return proj, folder, creator
}

// newHome returns a new profile folder: the same one gives the same device to each start.
func newHome(t *testing.T) *os.Root {
	t.Helper()
	home, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	return home
}

// startIn starts a runtime of the device of home on proj; afterChange is set for the image a
// project change restarted.
func startIn(t *testing.T, home *os.Root, proj *project.Project, afterChange bool) *Runtime {
	t.Helper()
	rt, err := Start(t.Context(), home, Options{Project: proj, RendezvousURL: NoCodes, LANService: testLANService, AfterChange: afterChange, CanRestart: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

// takeOver asks rt for a take-over that must be recorded.
func takeOver(t *testing.T, rt *Runtime) {
	t.Helper()
	got, err := rt.Session.TakeOver(t.Context())
	if err != nil || !got.Restarting {
		t.Fatalf("TakeOver of a non-member = %+v, %v; want restarting", got, err)
	}
}

// treeFiles returns the SHA-256 of every file under folder, by slash path, except those under a
// path skip accepts.
func treeFiles(t *testing.T, folder string, skip func(rel string) bool) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(folder, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(folder, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && rel != "." && skip(rel) {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if e.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// changed names the paths that differ between two [treeFiles].
func changed(a, b map[string]string) []string {
	var out []string
	for _, k := range slices.Sorted(maps.Keys(a)) {
		if b[k] != a[k] {
			out = append(out, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(b)) {
		if _, ok := a[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

func TestTakeOverRequestWritesNothing(t *testing.T) {
	proj, folder, _ := foreignWithOps(t)
	rt := startOn(t, proj)
	before := treeFiles(t, folder, nil)
	takeOver(t, rt)
	if diff := changed(before, treeFiles(t, folder, nil)); len(diff) > 0 {
		t.Errorf("N-5 nothing written at the request: changed %v", diff)
	}
}

func TestTakeOverThenNextStartInvites(t *testing.T) {
	proj, _, _ := foreignWithOps(t)
	home := newHome(t)
	rt := startIn(t, home, proj, false)
	quits := make(chan struct{}, 2)
	rt.SetQuit(func() { quits <- struct{}{} })
	takeOver(t, rt)
	select {
	case <-quits:
	case <-time.After(5 * time.Second):
		t.Error("N-5 the request asks to quit: no quit")
	}
	if len(quits) != 0 || !rt.RestartRequested() {
		t.Errorf("N-5 the request asks to quit once and restart: %d more quits, restart recorded %v", len(quits), rt.RestartRequested())
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	next := startIn(t, home, proj, true)
	if got, err := next.Session.Invite("writer"); err != nil || got.Refused != "" || got.Text == "" {
		t.Errorf("N-5 the next start invites: Invite = %+v, %v; want an invitation", got, err)
	}
}

func TestTakeOverArchivesOldList(t *testing.T) {
	proj, folder, _ := foreignWithOps(t)
	old := trustFiles(t, folder)
	rt := startOn(t, proj)
	takeOver(t, rt)
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(folder, ".inkwell", archiveDir)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !regexp.MustCompile(`^\d{8}T\d{6}\.\d{9}Z$`).MatchString(entries[0].Name()) {
		t.Fatalf("N-5 old list archived: %s holds %v (%v), want one folder named after the UTC time", dir, entries, err)
	}
	if got := readDir(t, filepath.Join(dir, entries[0].Name())); !equalMaps(got, old) {
		t.Errorf("N-5 old list archived: the archive differs from the old list: %v", changed(old, got))
	}
	ops := opFiles(trustFiles(t, folder))
	if len(ops) != 1 || equalMaps(ops, opFiles(old)) {
		t.Errorf("N-5 the list is replaced by the genesis: %d ops in trust, want the one genesis", len(ops))
	}
}

func TestTakeOverLeavesTextAndFilesIdentical(t *testing.T) {
	proj, folder, _ := foreignWithOps(t)
	home := newHome(t)
	rt := startIn(t, home, proj, false)
	skip := func(rel string) bool { return strings.HasPrefix(rel, ".inkwell/trust") }
	before := treeFiles(t, folder, skip)
	takeOver(t, rt)
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	next := startIn(t, home, proj, true)
	if got := next.Session.Standing(); got != StandingMember {
		t.Fatalf("standing after the take-over = %q, want member", got)
	}
	if diff := changed(before, treeFiles(t, folder, skip)); len(diff) > 0 {
		t.Errorf("N-6 folder unchanged: changed %v", diff)
	}
}

// takeOverCase is a member list a take-over is written on, outside a runtime.
type takeOverCase struct {
	folder         string
	creator, taker *identity.Device
	st             *opStore
	oldID, newID   trust.ProjectID
	oldOps         map[string]string
	g              trust.Op
}

func newTakeOverCase(t *testing.T) *takeOverCase {
	t.Helper()
	proj, folder, creator := foreignWithOps(t)
	list, st, err := openTrust(proj, creator)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.close() })
	c := &takeOverCase{folder: folder, creator: creator, taker: newDevice(t), st: st, oldID: list.Project(), oldOps: opFiles(trustFiles(t, folder))}
	if c.g, err = takeOverGenesis(c.taker, proj.Meta()); err != nil {
		t.Fatal(err)
	}
	c.newID = trust.ProjectID(c.g.Hash())
	return c
}

func (c *takeOverCase) finish() error {
	return finishTakeOver(c.st, c.g, time.Now(), slog.New(slog.DiscardHandler))
}

// crashCopy copies folder as a crash at this instant leaves it on disk.
func crashCopy(t *testing.T, folder string) string {
	t.Helper()
	snap := filepath.Join(t.TempDir(), "again")
	if err := os.CopyFS(snap, os.DirFS(folder)); err != nil {
		t.Fatal(err)
	}
	return snap
}

// opens opens the folder dir as the next launch of the taker does, checks that it opens on the
// old list, whole, or on the taken-over one, and returns that list with the device that may sign
// in it.
func (c *takeOverCase) opens(t *testing.T, what, dir string) (*trust.List, *identity.Device) {
	t.Helper()
	base, err := os.OpenRoot(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close() })
	p, err := project.Open(base, filepath.Base(dir))
	if err != nil {
		t.Fatalf("N-7 %s: the folder does not open: %v", what, err)
	}
	t.Cleanup(func() { _ = p.Close() })
	list, st, err := openTrust(p, c.taker)
	if err != nil {
		t.Fatalf("N-7 %s: the member list does not open: %v", what, err)
	}
	ops := opFiles(trustFiles(t, dir))
	if err := st.close(); err != nil {
		t.Fatal(err)
	}
	switch list.Project() {
	case c.oldID:
		for name, b := range c.oldOps {
			if ops[name] != b {
				t.Fatalf("N-7 archive is a copy: %s: the old list lost op %s", what, name)
			}
		}
		if got := standingOf(list, c.taker.Public()); got != StandingNotNamed {
			t.Fatalf("N-7 %s: on the old list the taker is %q, want not a member", what, got)
		}
		return list, c.creator
	case c.newID:
		if got := standingOf(list, c.taker.Public()); got != StandingMember {
			t.Fatalf("N-7 %s: on the taken-over list the taker is %q, want a member", what, got)
		}
		return list, c.taker
	}
	t.Fatalf("N-7 archive is a copy: %s: the folder opens on project %v, neither the old one nor the taken-over one", what, list.Project())
	return nil, nil
}

// saveNext saves on st an op of list signed by signer, and checks that the next launch finds it.
func (c *takeOverCase) saveNext(t *testing.T, what string, list *trust.List, signer *identity.Device) {
	t.Helper()
	third := newDevice(t)
	op, err := list.Issue(signer, trust.OpAdd, third.Public(), trust.RoleWriter, "third")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.st.save(op); err != nil {
		t.Fatalf("N-7 the next save works: %s: %v", what, err)
	}
	if again, _ := c.opens(t, what+", then a save", crashCopy(t, c.folder)); again.Role(third.Public()) != trust.RoleWriter {
		t.Errorf("N-7 the next save works: %s: the op saved after it is not found by the next open", what)
	}
}

// takeOverSteps returns the hook calls of a take-over that succeeds.
func takeOverSteps(t *testing.T) []string {
	t.Helper()
	c := newTakeOverCase(t)
	var steps []string
	c.st.step = func(name string) error { steps = append(steps, name); return nil }
	if err := c.finish(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{stepArchive, stepArchiveInstall, stepStage, stepAside, stepSwap, stepInstalled, stepCleanup} {
		if !slices.Contains(steps, want) {
			t.Fatalf("N-7 the steps %v lack %q", steps, want)
		}
	}
	if slices.Index(steps, stepArchiveInstall) > slices.Index(steps, stepStage) {
		t.Fatalf("N-7 the archive is whole before the replacement: steps %v", steps)
	}
	return steps
}

func TestTakeOverCrashAtEachStep(t *testing.T) {
	takeOverSteps(t) // the steps exist
	c := newTakeOverCase(t)
	type snap struct{ name, dir string }
	var snaps []snap
	c.st.step = func(name string) error {
		snaps = append(snaps, snap{fmt.Sprintf("%d-%s", len(snaps), name), crashCopy(t, c.folder)})
		return nil
	}
	if err := c.finish(); err != nil {
		t.Fatal(err)
	}
	c.st.step = nil
	snaps = append(snaps, snap{"end", crashCopy(t, c.folder)})
	for _, s := range snaps {
		t.Run(s.name, func(t *testing.T) {
			what := "crash at " + s.name
			list, signer := c.opens(t, what, s.dir)
			// The next save, in the folder the crash left.
			cc := *c
			cc.folder = s.dir
			base, err := os.OpenRoot(filepath.Dir(s.dir))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = base.Close() })
			p, err := project.Open(base, filepath.Base(s.dir))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = p.Close() })
			_, st, err := openTrust(p, c.taker)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.close() })
			cc.st = st
			cc.saveNext(t, what, list, signer)
		})
	}
}

func TestTakeOverFailureAtEachStep(t *testing.T) {
	steps := takeOverSteps(t)
	type failure struct {
		name string
		at   func(i int, name string) bool
	}
	var cases []failure
	for k, name := range steps {
		cases = append(cases, failure{fmt.Sprintf("%d-%s", k, name), func(i int, _ string) bool { return i == k }})
	}
	swap := slices.Index(steps, stepSwap)
	cases = append(cases, failure{"swap-and-rollback", func(i int, name string) bool { return i == swap || name == stepRollback }})
	for _, f := range cases {
		t.Run(f.name, func(t *testing.T) {
			c := newTakeOverCase(t)
			n := 0
			c.st.step = func(name string) error {
				n++
				if f.at(n-1, name) {
					return fmt.Errorf("injected failure at %s", name)
				}
				return nil
			}
			err := c.finish()
			c.st.step = nil
			what := fmt.Sprintf("failure at %s (error %v)", f.name, err)
			list, signer := c.opens(t, what, crashCopy(t, c.folder))
			if got, want := list.Project() == c.newID, err == nil; got != want {
				t.Errorf("N-7 %s: taken over %v, want %v (an error means the old list)", what, got, want)
			}
			c.saveNext(t, what, list, signer)
		})
	}
}

func TestTakeOverNoOpForMember(t *testing.T) {
	rt, d := startRuntime(t, nil, false)
	folder := d.proj.Path()
	before := trustFiles(t, folder)
	quits := 0
	rt.SetQuit(func() { quits++ })
	got, err := rt.Session.TakeOver(t.Context())
	if err != nil || got.Restarting {
		t.Errorf("N-8 a member's take-over does nothing: TakeOver = %+v, %v; want not restarting", got, err)
	}
	if quits != 0 || rt.RestartRequested() {
		t.Errorf("N-8 a member's take-over does nothing: quit %d times, restart recorded %v", quits, rt.RestartRequested())
	}
	if inv, err := rt.Session.Invite("writer"); err != nil || inv.Text == "" {
		t.Errorf("N-8 a member's take-over does nothing: Invite after it = %+v, %v", inv, err)
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if after := trustFiles(t, folder); !equalMaps(after, before) {
		t.Errorf("N-8 a member's take-over does nothing: the member list changed: %v", changed(before, after))
	}
}

func TestTakeOverIdDiffersFromOwnProject(t *testing.T) {
	home := newHome(t)
	own, _ := newProjectFolder(t)
	ownRT := startIn(t, home, own, false)
	ownID := ownRT.wiring.list().Project()
	if err := ownRT.Close(); err != nil {
		t.Fatal(err)
	}
	proj, _, _ := foreignWithOps(t)
	if proj.Meta().Title != own.Meta().Title {
		t.Fatalf("setup: titles %q and %q differ", proj.Meta().Title, own.Meta().Title)
	}
	rt := startIn(t, home, proj, false)
	oldID := rt.wiring.list().Project()
	takeOver(t, rt)
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	id := startIn(t, home, proj, true).wiring.list().Project()
	switch id {
	case ownID:
		t.Errorf("N-8 distinct project: the taken-over project has the id of this device's own project %v", ownID)
	case oldID:
		t.Errorf("N-8 distinct project: the next start is still on the old project %v", oldID)
	}
}

func TestTakeOverNeverAdoptsBeforeShutdown(t *testing.T) {
	proj, _, _ := foreignWithOps(t)
	rt := startOn(t, proj)
	oldID := rt.wiring.list().Project()
	var adoptions, steps []string
	adopted := func(when string) {
		if rt.wiring.list().Project() != oldID {
			adoptions = append(adoptions, when)
		}
	}
	rt.inv.store.step = func(name string) error {
		adopted("at " + name)
		if rt.wiring.ctx.Err() == nil {
			adoptions = append(adoptions, name+" before Wiring.Close")
		}
		steps = append(steps, name)
		return nil
	}
	takeOver(t, rt)
	adopted("after the request")
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	adopted("after Close")
	if len(adoptions) > 0 {
		t.Errorf("N-11 no adoption before shutdown: %v", adoptions)
	}
	a, s := slices.Index(steps, stepArchiveInstall), slices.Index(steps, stepStage)
	if a < 0 || s < 0 || a > s {
		t.Errorf("N-11 the archive, then the replacement, after Wiring.Close: steps %v", steps)
	}
}

func TestTakeOverIdentityOnlyAtNextStart(t *testing.T) {
	proj, _, _ := foreignWithOps(t)
	home := newHome(t)
	rt := startIn(t, home, proj, false)
	oldID := rt.wiring.list().Project()
	takeOver(t, rt)
	if rt.wiring.list().Project() != oldID || rt.Session.Standing() != StandingNotNamed {
		t.Errorf("N-11 identity only at the next start: the running session is on %v, standing %q", rt.wiring.list().Project(), rt.Session.Standing())
	}
	if _, err := rt.Session.Invite("writer"); !errors.Is(err, errLeaving) {
		t.Errorf("N-11 identity only at the next start: Invite in the running session = %v, want refused", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	next := startIn(t, home, proj, true)
	if next.wiring.list().Project() == oldID || next.Session.Standing() != StandingMember {
		t.Errorf("N-11 identity only at the next start: the next image is on %v, standing %q", next.wiring.list().Project(), next.Session.Standing())
	}
}

func TestTakeOverOutcomeFollowsTheLoadedList(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failAt   string
		standing string
		notice   string
	}{
		{"failure before the install", stepStage, StandingNotNamed, "foreign.takeOver.failed"},
		{"replace done", "", StandingMember, "foreign.takeOver.done"},
		{"finalisation error", stepInstalled, StandingMember, "foreign.takeOver.done"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj, _, _ := foreignWithOps(t)
			home := newHome(t)
			rt := startIn(t, home, proj, false)
			rt.inv.store.step = func(name string) error {
				if name == tc.failAt {
					return errors.New("injected failure at " + name)
				}
				return nil
			}
			takeOver(t, rt)
			_ = rt.Close() // the outcome is what the next start loads, whatever Close said
			next := startIn(t, home, proj, true)
			if got, notice := next.Session.Standing(), next.Session.StartupNotice(); got != tc.standing || notice != tc.notice {
				t.Errorf("N-12 outcome from the loaded list: %s: standing %q, notice %q; want %q, %q", tc.name, got, notice, tc.standing, tc.notice)
			}
		})
	}
}

// TestTakeOverRefusedWhereNoRestart: where this build cannot restart Inkwell (residual R-8: the
// restart is specified for Linux), the take-over is not offered and a request is refused before
// anything is signed: nothing is written, nothing quits, nothing restarts.
func TestTakeOverRefusedWhereNoRestart(t *testing.T) {
	proj, folder, _ := foreignWithOps(t)
	home := newHome(t)
	rt, err := Start(t.Context(), home, Options{Project: proj, RendezvousURL: NoCodes, LANService: testLANService})
	if err != nil {
		t.Fatal(err)
	}
	quits := 0
	rt.SetQuit(func() { quits++ })
	before := treeFiles(t, folder, nil)
	if rt.Session.CanTakeOver() {
		t.Error("R-8 no take-over where no restart: CanTakeOver = true without a restart")
	}
	got, err := rt.Session.TakeOver(t.Context())
	if !errors.Is(err, errNoRestart) || got.Restarting {
		t.Errorf("R-8 no take-over where no restart: TakeOver = %+v, %v; want refused", got, err)
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if diff := changed(before, treeFiles(t, folder, nil)); len(diff) > 0 || quits != 0 || rt.RestartRequested() {
		t.Errorf("R-8 no take-over where no restart: changed %v, quit %d times, restart recorded %v", diff, quits, rt.RestartRequested())
	}
	if !startIn(t, home, proj, false).Session.CanTakeOver() {
		t.Error("R-8 the take-over is offered where Inkwell can restart: CanTakeOver = false")
	}
}

// TestTakeOverRepliesBeforeQuit: the answer of the request leaves before the application is asked
// to quit: the quit never runs inside the call it would have to wait for (the server build's
// shutdown waits for the calls in flight), and the restart page gets « restarting » (rules N-5, N-9).
func TestTakeOverRepliesBeforeQuit(t *testing.T) {
	proj, _, _ := foreignWithOps(t)
	rt := startOn(t, proj)
	replied := make(chan struct{})
	afterReply := make(chan bool, 1)
	rt.SetQuit(func() {
		select {
		case <-replied:
			afterReply <- true
		case <-time.After(2 * time.Second):
			afterReply <- false
		}
	})
	takeOver(t, rt)
	close(replied)
	if !<-afterReply {
		t.Error("N-5 the reply leaves before the quit: the quit ran inside the request and waited for its own call")
	}
}
