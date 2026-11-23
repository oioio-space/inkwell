package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// foreignFolder returns a project folder whose member list was made by another
// device, with a recorded root document, some text, a file and a binary: what a
// second profile finds on a Documents folder shared with the first.
func foreignFolder(t *testing.T) (proj *project.Project, folder string, creator *identity.Device) {
	t.Helper()
	proj, folder = newProjectFolder(t)
	creator = newDevice(t)
	if err := proj.SetRootURL("automerge:root1"); err != nil {
		t.Fatal(err)
	}
	_, st, err := openTrust(proj, creator)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"main.typ":                 "= Titre\nle texte\n",
		"chapter.typ":              "un chapitre",
		".inkwell/blobs/ab/cdef01": "binary\x00bytes",
		".inkwell/repo/chunk":      "automerge chunk",
	} {
		path := filepath.Join(folder, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return proj, folder, creator
}

// startOn starts a runtime of a new device on proj.
func startOn(t *testing.T, proj *project.Project) *Runtime {
	t.Helper()
	home, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	rt, err := Start(t.Context(), home, Options{Project: proj, RendezvousURL: NoCodes, LANService: testLANService, CanRestart: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

// trustFiles returns the files of the trust directory of folder, by name.
func trustFiles(t *testing.T, folder string) map[string]string {
	t.Helper()
	return readDir(t, filepath.Join(folder, ".inkwell", trustDir))
}

func readDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = string(b)
	}
	return out
}

// opFiles returns the .op files of files, which is a directory listing.
func opFiles(files map[string]string) map[string]string {
	out := map[string]string{}
	for name, b := range files {
		if strings.HasSuffix(name, opExt) {
			out[name] = b
		}
	}
	return out
}

func rolesOf(t *testing.T, rt *Runtime) []bridge.MemberRole {
	t.Helper()
	for _, e := range rt.Repo.(bridge.Attester).Notices() {
		if e.Kind == bridge.Roles {
			rs, err := bridge.DecodeRoles(e.Payload)
			if err != nil {
				t.Fatal(err)
			}
			return rs
		}
	}
	return nil
}

func TestStandingFromList(t *testing.T) {
	admin, writer, removed, stranger := newDevice(t), newDevice(t), newDevice(t), newDevice(t)
	list := newProject(t, admin)
	for _, d := range []*identity.Device{writer, removed} {
		if _, err := list.Issue(admin, trust.OpAdd, d.Public(), trust.RoleWriter, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := list.Issue(admin, trust.OpRemove, removed.Public(), 0, ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		dev  *identity.Device
		want string
	}{
		{"creator", admin, StandingMember},
		{"writer", writer, StandingMember},
		{"removed", removed, StandingRemoved},
		{"never named", stranger, StandingNotNamed},
	} {
		if got := standingOf(list, tc.dev.Public()); got != tc.want {
			t.Errorf("N-1 standing of the %s = %q, want %q", tc.name, got, tc.want)
		}
	}
	proj, _, _ := foreignFolder(t)
	if got := startOn(t, proj).Session.Standing(); got != StandingNotNamed {
		t.Errorf("N-1 standing of a device on a foreign folder = %q, want %q", got, StandingNotNamed)
	}
	if got := NewSession(noNet{}).Standing(); got != StandingNotNamed {
		t.Errorf("N-1 standing of a session without project = %q, want %q", got, StandingNotNamed)
	}
}

func TestStandingMemberAfterJoin(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	proj, _, _ := foreignFolder(t)
	home := newHome(t)
	guest := startIn(t, home, proj, false)
	if got := guest.Session.Standing(); got != StandingNotNamed {
		t.Fatalf("N-1 standing before the join = %q, want %q", got, StandingNotNamed)
	}
	// The standing follows the loaded list: the join makes it "member" in the image it restarts
	// (PC1b, rule J-1).
	guest = joinedRuntime(t, guest, dirs{home, proj}, host)
	if got := guest.Session.Standing(); got != StandingMember {
		t.Errorf("N-1 standing after the join = %q, want %q", got, StandingMember)
	}
	self := p2p.IDFromKey(guest.inv.dev.Public())
	if !slices.ContainsFunc(rolesOf(t, guest), func(r bridge.MemberRole) bool { return r.Peer == self }) {
		t.Error("N-1 the roles notice after the join does not name this device")
	}
}

func TestFirstLaunchCreatorIsAdmin(t *testing.T) {
	rt, _ := startRuntime(t, nil, false)
	if got := rt.Session.Standing(); got != StandingMember {
		t.Errorf("N-1 standing of the creator = %q, want %q", got, StandingMember)
	}
	if got := rt.inv.currentList().Role(rt.inv.dev.Public()); got != trust.RoleAdmin {
		t.Errorf("N-1 role of the creator = %v, want admin", got)
	}
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// adm returns the admission of the runtime's wiring.
func (r *Runtime) adm() *Admission { return r.wiring.adm }

// N-1: Standing never answers from an older list. Since a join restarts Inkwell (PC1b, rule
// J-1), the running session keeps its list and its standing until the end, whatever the Welcome
// held, and the image the join restarts answers "member" from its first call, before any event.
func TestStandingNeverOldAfterRolesEvent(t *testing.T) {
	host, _ := startRuntime(t, nil, false)
	proj, _, _ := foreignFolder(t)
	home := newHome(t)
	rt := startIn(t, home, proj, false)
	holdJoin(t, rt, host)
	if got := rt.Session.Standing(); got != StandingNotNamed {
		t.Errorf("N-1 standing of the running session with a held Welcome = %q, want %q", got, StandingNotNamed)
	}
	next := restartAfterJoin(t, rt, home, proj)
	if got := next.Session.Standing(); got != StandingMember {
		t.Errorf("N-1 standing of the image the join restarted = %q, want %q", got, StandingMember)
	}
	self := p2p.IDFromKey(next.inv.dev.Public())
	if !slices.ContainsFunc(rolesOf(t, next), func(r bridge.MemberRole) bool { return r.Peer == self }) {
		t.Error("N-1 the roles notice of the restarted image does not name this device")
	}
}

// refusalCase is one way a device cannot invite, for N-2.
type refusalCase struct {
	name string
	want string
	// setup returns the runtime and the folder whose trust store must stay
	// byte-identical; the refusal is then asked of both entry points.
	setup func(t *testing.T) (*Runtime, string)
}

// assertRefusedByKey asks Invite and InviteDevice of rt for a refusal and checks
// the key, the absence of error and text, and the bytes of the trust store.
func assertRefusedByKey(t *testing.T, rt *Runtime, folder, want string) {
	t.Helper()
	before := trustFiles(t, folder)
	for name, invite := range map[string]func() (Invitation, error){
		"Invite":       func() (Invitation, error) { return rt.Session.Invite("writer") },
		"InviteDevice": rt.Session.InviteDevice,
	} {
		tag := "N-2 refused by key"
		if name == "InviteDevice" {
			tag = "N-2 device invitation refused by key"
		}
		got, err := invite()
		if err != nil {
			t.Errorf("%s: %s = error %v, want a result", tag, name, err)
		}
		if got.Refused != want || got.Text != "" || got.Code != "" {
			t.Errorf("%s: %s = %+v, want only Refused %q", tag, name, got, want)
		}
	}
	if after := trustFiles(t, folder); !equalMaps(before, after) {
		t.Errorf("N-2 refused by key: the trust directory changed: %d files before, %d after", len(before), len(after))
	}
}

func TestNotMemberInviteRefusedByKey(t *testing.T) {
	for _, tc := range []refusalCase{
		{"non-member", RefusedNotMember, func(t *testing.T) (*Runtime, string) {
			proj, folder, _ := foreignFolder(t)
			return startOn(t, proj), folder
		}},
		{"writer", RefusedNotAdmin, func(t *testing.T) (*Runtime, string) {
			host, _ := startRuntime(t, nil, false)
			inv, err := host.Session.Invite("writer")
			if err != nil {
				t.Fatal(err)
			}
			proj, _, _ := foreignFolder(t)
			home := newHome(t)
			rt := startIn(t, home, proj, false)
			if _, err := rt.Session.Join(t.Context(), inv.Text); err != nil {
				t.Fatal(err)
			}
			rt = restartAfterJoin(t, rt, home, proj) // a writer in the image the join restarted
			return rt, rt.inv.folder
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, folder := tc.setup(t)
			assertRefusedByKey(t, rt, folder, tc.want)
		})
	}
	// An admin is not refused.
	host, _ := startRuntime(t, nil, false)
	if got, err := host.Session.Invite("writer"); err != nil || got.Refused != "" || got.Text == "" {
		t.Errorf("N-2 refused by key: Invite of an admin = %+v, %v; want an invitation", got, err)
	}
}

// N-2: an admin demoted between the check of the session and the one of the
// host gets the same refusal key, not an error, on both entry points.
func TestInviteDemotedBetweenChecksRefusedByKey(t *testing.T) {
	rt, d := startRuntime(t, nil, false)
	other := newDevice(t)
	g, id := trust.Genesis(other, "demoted", "")
	list := trust.New(id)
	if err := list.Apply(g); err != nil {
		t.Fatal(err)
	}
	if _, err := list.Issue(other, trust.OpAdd, rt.inv.dev.Public(), trust.RoleWriter, "me"); err != nil {
		t.Fatal(err)
	}
	// The session still sees an admin; the host already sees the demotion.
	rt.inv.mu.Lock()
	rt.inv.host = rt.inv.newHost(list)
	rt.inv.mu.Unlock()
	if key := rt.inv.refusal(); key != "" {
		t.Fatalf("setup: the session refuses already: %q", key)
	}
	assertRefusedByKey(t, rt, d.proj.Path(), RefusedNotAdmin)
}

// N-16: a replacement that fails at any step (stage, swap, after the install,
// reopen, and a rollback that fails as well) leaves a store that opens on the
// old list or on the new one, and the same store saves the next op before any
// reopening, even after the double failure that left the trust directory closed.
func TestReplaceFailureAtEachStep(t *testing.T) {
	newList := func(t *testing.T) (*identity.Device, *trust.List) {
		t.Helper()
		d := newDevice(t)
		g, id := trust.Genesis(d, "replacement", "")
		l := trust.New(id)
		if err := l.Apply(g); err != nil {
			t.Fatal(err)
		}
		return d, l
	}
	open := func(t *testing.T) (*project.Project, string, *identity.Device, *trust.List, *opStore) {
		t.Helper()
		proj, folder, creator := foreignFolder(t)
		list, st, err := openTrust(proj, creator)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.close() })
		return proj, folder, creator, list, st
	}
	// A successful replacement, to learn the steps.
	var steps []string
	{
		_, _, _, _, st := open(t)
		_, nl := newList(t)
		st.step = func(name string) error { steps = append(steps, name); return nil }
		if err := st.replace(nl.Since(nil)); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{stepStage, stepSwap, stepInstalled} {
		if !slices.Contains(steps, want) {
			t.Fatalf("N-16 steps %v lack %q", steps, want)
		}
	}
	type failure struct {
		name string
		at   func(i int, name string) bool // which hook calls fail
		old  bool                          // the old list must be the one found again
	}
	var cases []failure
	for k, name := range steps {
		cases = append(cases, failure{name: fmt.Sprintf("%d-%s", k, name), at: func(i int, _ string) bool { return i == k }})
	}
	swap := slices.Index(steps, stepSwap)
	cases = append(cases, failure{"swap-and-rollback", func(i int, name string) bool { return i == swap || name == stepRollback }, true})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, folder, creator, oldList, st := open(t)
			newDev, nl := newList(t)
			oldID := oldList.Project()
			n := 0
			st.step = func(name string) error {
				n++
				if c.at(n-1, name) {
					return fmt.Errorf("injected failure at %s", name)
				}
				return nil
			}
			replaceErr := st.replace(nl.Since(nil))
			st.step = nil

			// What the next launch finds: a copy of the folder, opened as the app opens it.
			reopen := func(what string) *trust.List {
				t.Helper()
				snap := filepath.Join(t.TempDir(), "again")
				if err := os.CopyFS(snap, os.DirFS(folder)); err != nil {
					t.Fatal(err)
				}
				base, err := os.OpenRoot(filepath.Dir(snap))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = base.Close() })
				p, err := project.Open(base, "again")
				if err != nil {
					t.Fatalf("N-16 %s (replace error %v): the folder does not open: %v", what, replaceErr, err)
				}
				t.Cleanup(func() { _ = p.Close() })
				list, again, err := openTrust(p, creator)
				if err != nil {
					t.Fatalf("N-16 %s (replace error %v): the member list does not open: %v", what, replaceErr, err)
				}
				_ = again.close()
				return list
			}
			list := reopen("after the failure")
			switch got := list.Project(); {
			case got == oldID:
			case got == nl.Project() && !c.old:
			default:
				t.Fatalf("N-16 %s: the store opens on %v, want the old project %v or the new %v", c.name, got, oldID, nl.Project())
			}

			// The next save on the same store works, and is found again by the next launch.
			signer := creator
			if list.Project() != oldID {
				signer = newDev
			}
			third := newDevice(t)
			op, err := list.Issue(signer, trust.OpAdd, third.Public(), trust.RoleWriter, "third")
			if err != nil {
				t.Fatal(err)
			}
			if err := st.save(op); err != nil {
				t.Fatalf("N-16 store usable after a failed replacement: %s: the next save fails: %v", c.name, err)
			}
			if again := reopen("after a save"); again.Role(third.Public()) != trust.RoleWriter {
				t.Errorf("N-16 store usable after a failed replacement: %s: the op saved after the failure is not found by the next open", c.name)
			}
		})
	}
}
