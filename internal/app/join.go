package app

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// A join into another project never changes the running session (plan
// docs/plans/2026-11-15-pc1b-join.md, rules J-1..J-6; ADR 0043, part 2). The verified Welcome is
// held in memory ([invites.hold]); [Session.FinishJoin] records it as the change of this image;
// [Runtime.Close] then writes the joined project in a folder of its own, beside the open one
// ([finishJoin]), and the process restarts on that folder. The next image waits for the root
// document and consumes the record once it is stored ([Session.CompleteJoin]).

const (
	// joinMarkerFile, in .inkwell of the open project, names the staging folder of a join from
	// before that folder exists: the one way the next start finds it again (rule J-6).
	joinMarkerFile = "join-pending.json"
	// joinFolderDefault names the folder of a joined document whose title leaves nothing usable
	// (catalogue key join.folderDefault).
	joinFolderDefault = "Document rejoint"
	// maxJoinFolder bounds the name of a joined folder, in characters.
	maxJoinFolder = 64
	// maxJoinFolderBytes bounds it in bytes of UTF-8 as well: file systems refuse a name over 255
	// bytes, and the suffix « (99) » must still fit. Only scripts of four bytes a letter reach it.
	maxJoinFolderBytes = 240
	// maxJoinSuffix is the last « (n) » tried when the name is taken.
	maxJoinSuffix = 99
	// joinFolderPunct are the characters kept in the name of a joined folder besides letters,
	// digits and spaces.
	joinFolderPunct = "-_.,()'"
	// joinRecordVersion is the version of the record and of the marker.
	joinRecordVersion = 1

	// Names of the steps of [finishJoin], given to the test hook of the member-list store before
	// each, as those of [opStore.replace] are.
	stepJoinMarker    = "join-marker"    // before the marker is written
	stepJoinStaging   = "join-staging"   // before the staging folder is created
	stepJoinRecord    = "join-record"    // before the record is written in the staging folder
	stepJoinList      = "join-list"      // before each op of the list is staged
	stepJoinInstall   = "join-install"   // before the rename that installs the folder
	stepJoinInstalled = "join-installed" // after it: the joined folder is what the restart opens
)

var (
	// errJoinFolderTaken: every name of the joined folder, up to « (99) », exists.
	errJoinFolderTaken = errors.New("join: no free folder name")
	// errNoJoinRecord refuses [Session.CompleteJoin] in a project that waits for no join.
	errNoJoinRecord = errors.New("join: no join waits for its document")
	// errJoinRoot refuses a root document other than the one the invitation stated.
	errJoinRoot = errors.New("join: not the document of the invitation")
	// errJoinInviter refuses, while the record holds no root, a root document announced by another
	// device than the inviter.
	errJoinInviter = errors.New("join: the document was not announced by the inviter")
	// errJoinPeer refuses a root document announced by a device that is not a member.
	errJoinPeer = errors.New("join: the document was not announced by a member")
	// errNoPrevious refuses [Session.ReturnToPrevious] when no join made the open folder, or when the
	// folder that was open before it is no longer a document of Inkwell.
	errNoPrevious = errors.New("join: no previous document to return to")
	// errJoinNotStored refuses to consume the record before the document is in the storage.
	errJoinNotStored = errors.New("join: the document is not stored yet")
)

// JoinResult is the answer of [Session.Join] and [Session.Ask].
type JoinResult struct {
	// Peer identifies the device that admitted this one.
	Peer string `json:"peer"`
	// Title names the joined document for the person: its title as [joinFolderName] keeps it (a
	// text supplied by a peer, bounded and made of letters, digits and plain punctuation only).
	Title string `json:"title,omitzero"`
	// Folder is the name the folder of the joined document gets, beside the open one: the first
	// free one when the invitation was accepted, « (2) »… included. The install checks again and
	// takes the next one when the name was taken meanwhile ([JoinPending.Folder] then says
	// which). It is empty without a restart.
	Folder string `json:"folder,omitzero"`
	// Current is the name of the folder of the open document, which a join never changes.
	Current string `json:"current,omitzero"`
	// Restart is set when the invitation leads to another project than the open one: nothing was
	// written; the page leaves its context, then calls [Session.FinishJoin], and Inkwell
	// restarts on the joined document. It is false for the open project: nothing to do.
	Restart bool `json:"restart"`
}

// JoinPending is the answer of [Session.PendingJoin].
type JoinPending struct {
	// Root is the automerge URL of the document to open when the invitation stated it; when
	// empty, the page waits for the root a member announces.
	Root string `json:"root"`
	// Inviter identifies the device that admitted this one.
	Inviter string `json:"inviter"`
	// Name is the name of the inviter in the member list; empty for the creator of the project.
	Name string `json:"name"`
	// Folder is the name of the open folder when it is not the one the person was told before
	// the restart (the name was taken meanwhile); empty otherwise.
	Folder string `json:"folder"`
	// Waiting is set while the open project waits for the root document of a join: the page
	// never creates a document then.
	Waiting bool `json:"waiting"`
	// CanReturn is set, while the join waits and after it completed (rules J-2 and J-9), while
	// the document that was open before the join is still in its folder:
	// [Session.ReturnToPrevious] then restarts Inkwell on it.
	CanReturn bool `json:"canReturn"`
}

// heldJoin is a verified Welcome into another project, held until the write.
type heldJoin struct {
	ops    []trust.Op // the member list, parents first
	name   string     // the name of the joined folder, normalised
	record project.PendingJoin
}

// joinFolderName returns the folder name of a joined document titled title, a text supplied by a
// peer (rule J-6): Unicode NFC; only letters, the combining marks that follow them (category M:
// Devanagari, Thai and other scripts are unreadable without), digits, spaces and -_.,()' are
// kept; leading dots, spaces and marks go, so that it is never a dot-name; at most 64
// characters and [maxJoinFolderBytes] bytes, cut on a character; trailing dots and spaces go
// (Windows and network shares strip them: the name made would not be the name asked for);
// [joinFolderDefault] when nothing is left. It can name nothing but a direct child of a folder.
func joinFolderName(title string) string {
	var b strings.Builder
	n := 0
	for _, r := range norm.NFC.String(title) {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsDigit(r) && r != ' ' && !strings.ContainsRune(joinFolderPunct, r):
			continue
		case n == 0 && (r == '.' || r == ' ' || unicode.IsMark(r)):
			continue
		}
		if n == maxJoinFolder || b.Len()+utf8.RuneLen(r) > maxJoinFolderBytes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return cmp.Or(strings.TrimRight(b.String(), ". "), joinFolderDefault)
}

// newHeldJoin returns the Welcome j, of a project titled title and reached at addrs, as it is
// held: with a new record id and the folder name of the title.
func newHeldJoin(j *invite.Joined, title string, addrs []string) (*heldJoin, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}
	root := j.RootURL
	if !validRootURL(root) {
		root = "" // a member announces it
	}
	return &heldJoin{
		ops:  j.List.Since(nil),
		name: joinFolderName(title),
		record: project.PendingJoin{
			V: joinRecordVersion, ID: hex.EncodeToString(id[:]), Project: j.List.Project().String(),
			Inviter: string(j.Host), Addrs: slices.Clone(addrs), Root: root,
		},
	}, nil
}

// hold keeps the verified Welcome j, received from a host reached at addrs, for the restart: for
// another project than the open one it is held as the change of this image and the result says
// Restart; nothing is written and nothing is adopted (rules J-1, J-3 and N-11). For the open
// project there is nothing to do: the member lists converge by themselves (rule J-5).
func (inv *invites) hold(j *invite.Joined, addrs []string) (JoinResult, error) {
	res := JoinResult{Peer: string(j.Host)}
	if j.List.Project() == inv.currentList().Project() {
		return res, nil
	}
	ops := j.List.Since(nil)
	title := ""
	if i := slices.IndexFunc(ops, func(o trust.Op) bool { return o.Kind == trust.OpGenesis }); i >= 0 {
		title = ops[i].Name
	}
	h, err := newHeldJoin(j, title, addrs)
	if err != nil {
		return JoinResult{}, err
	}
	// What the person reads before anything restarts (rule J-2): the name the install is about
	// to make, kept in the record with the folder open now, to which they can return.
	h.record.Previous = filepath.Base(inv.folder)
	h.record.Folder = freeJoinName(filepath.Dir(inv.folder), h.name)
	write := func(st *opStore) error {
		joined, err := finishJoin(st, inv.folder, h, inv.log)
		inv.chg.setJoined(joined)
		return err
	}
	if err := inv.chg.hold(write); err != nil {
		return JoinResult{}, err
	}
	res.Restart, res.Title, res.Folder, res.Current = true, h.name, h.record.Folder, h.record.Previous
	return res, nil
}

// suffixed returns the n-th name tried for a joined folder named name: name itself, then
// « name (2) », « name (3) », and so on.
func suffixed(name string, n int) string {
	if n <= 1 {
		return name
	}
	return fmt.Sprintf("%s (%d)", name, n)
}

// freeJoinName returns the first name [installJoin] would find free in the folder at parent for
// a joined folder named name, as far as can be told now; name itself when none is free or the
// folder cannot be read (the install then decides, and fails when none is).
func freeJoinName(parent, name string) string {
	root, err := os.OpenRoot(parent)
	if err != nil {
		return name
	}
	defer func() { _ = root.Close() }() // read-only handle
	for n := 1; n <= maxJoinSuffix; n++ {
		if _, err := root.Lstat(suffixed(name, n)); errors.Is(err, fs.ErrNotExist) {
			return suffixed(name, n)
		}
	}
	return name
}

// finishJoin writes the project of the held join h in a folder of its own beside folder, the
// open project, of which st is the member-list store, and returns the path of the folder it
// installed (rules J-1 and J-6). In this order: the marker in the open project; the staging
// folder, with exclusive create, in the parent; in it the record, then the list; one rename that
// never replaces, under the first free name. An error before the rename leaves the open project
// as it was but for its marker, and the next start removes what is left ([recoverJoin]); once
// the rename is done the join is installed, and a failure after it is only logged.
func finishJoin(st *opStore, folder string, h *heldJoin, log *slog.Logger) (joined string, err error) {
	parent, err := os.OpenRoot(filepath.Dir(folder))
	if err != nil {
		return "", fmt.Errorf("join: opening the parent folder: %w", err)
	}
	defer func() { _ = parent.Close() }() // read-only handle
	if err := writeJoinMarker(st, h.record.ID); err != nil {
		return "", err
	}
	staged, err := createStaging(st, parent, h)
	if err != nil {
		return "", err
	}
	err = stageJoin(st, staged, h)
	if err = errors.Join(err, staged.Close()); err != nil {
		return "", fmt.Errorf("join: staging the project: %w", err)
	}
	name, err := installJoin(st, parent, staged.Dir(), h.name)
	if err != nil {
		return "", err
	}
	if err := errors.Join(st.hook(stepJoinInstalled), store.SyncDir(parent, ".")); err != nil {
		log.Warn("project joined, with a follow-up failure", "err", err)
	}
	return filepath.Join(parent.Name(), name), nil
}

// joinMarker is the content of [joinMarkerFile].
type joinMarker struct {
	ID string `json:"id"`
	V  int    `json:"v"`
}

// writeJoinMarker names, in the open project, the staging folder of the record id.
func writeJoinMarker(st *opStore, id string) error {
	if err := st.hook(stepJoinMarker); err != nil {
		return err
	}
	b, err := json.Marshal(joinMarker{V: joinRecordVersion, ID: id})
	if err != nil {
		return fmt.Errorf("join: %w", err)
	}
	if err := store.WriteAtomic(st.parent, joinMarkerFile, b, 0o600); err != nil {
		return fmt.Errorf("join: writing the marker: %w", err)
	}
	return nil
}

// createStaging creates the staging folder of h in parent, holding a new project.
func createStaging(st *opStore, parent *os.Root, h *heldJoin) (*project.Project, error) {
	if err := st.hook(stepJoinStaging); err != nil {
		return nil, err
	}
	p, err := project.CreateStaging(parent, h.record.ID, h.name)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}
	return p, nil
}

// stageJoin writes the record of h, then its member list, in the staged project p, and makes
// them durable.
func stageJoin(st *opStore, p *project.Project, h *heldJoin) error {
	if err := stageRecord(st, p, h.record); err != nil {
		return err
	}
	return stageList(st, p, h.ops)
}

func stageRecord(st *opStore, p *project.Project, rec project.PendingJoin) error {
	if err := st.hook(stepJoinRecord); err != nil {
		return err
	}
	return p.SetPendingJoin(rec)
}

func stageList(st *opStore, p *project.Project, ops []trust.Op) (err error) {
	ink, err := p.OpenDir(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ink.Close()) }()
	root, err := p.OpenDir(trustDir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, op := range ops {
		if err := st.hook(stepJoinList); err != nil {
			return err
		}
		if err := saveIn(root, op); err != nil {
			return err
		}
	}
	return errors.Join(store.SyncDir(ink, trustDir), store.SyncDir(ink, "."))
}

// installJoin renames the folder staging of parent to name, or to « name (2) »… « name (99) »
// when the name is taken, by a rename that never replaces, and returns the name it got.
func installJoin(st *opStore, parent *os.Root, staging, name string) (string, error) {
	if err := errors.Join(st.hook(stepJoinInstall), store.SyncDir(parent, staging), store.SyncDir(parent, ".")); err != nil {
		return "", err
	}
	for n := 1; n <= maxJoinSuffix; n++ {
		dst := suffixed(name, n)
		err := project.RenameNoReplace(parent, staging, dst)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("join: installing the folder: %w", err)
		}
		return dst, nil
	}
	return "", errJoinFolderTaken
}

// recoverJoin removes what an unfinished join left beside the project p: the staging folder its
// marker names, when it is still there, then the marker (rules J-4 and J-6). failed reports that
// a staging folder was found: the join never installed, nothing changed, and the next notice
// says so. A marker that names no staging folder (the join installed, or never got that far) is
// removed alone. The staging folder is named from the marker only when its id is 128 bits in
// hex: it can be nothing but the direct child of the parent a join created.
func recoverJoin(p *project.Project) (failed bool, err error) {
	ink, err := p.OpenDir(".")
	if err != nil {
		return false, err
	}
	defer func() { _ = ink.Close() }() // read-only use but for the removal, checked below
	b, err := ink.ReadFile(joinMarkerFile)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("join: reading the marker: %w", err)
	}
	var m joinMarker
	if id, derr := hex.DecodeString(m.unmarshal(b)); derr == nil && len(id) == 16 {
		if failed, err = removeStaging(filepath.Dir(p.Path()), project.StagingPrefix+m.ID); err != nil {
			return failed, err
		}
	}
	if err := ink.Remove(joinMarkerFile); err != nil {
		return failed, fmt.Errorf("join: removing the marker: %w", err)
	}
	return failed, store.SyncDir(ink, ".")
}

// unmarshal reads the marker b and returns its id, "" for a marker that cannot be read.
func (m *joinMarker) unmarshal(b []byte) string {
	if err := json.Unmarshal(b, m); err != nil || m.V != joinRecordVersion || strings.ToLower(m.ID) != m.ID {
		m.ID = ""
	}
	return m.ID
}

// removeStaging removes the folder name of the folder at parent, and reports whether it existed.
func removeStaging(parent, name string) (existed bool, err error) {
	root, err := os.OpenRoot(parent)
	if err != nil {
		return false, fmt.Errorf("join: opening the parent folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	if _, err := root.Lstat(name); err != nil {
		return false, nil //nolint:nilerr // nothing by that name: nothing to remove
	}
	if err := root.RemoveAll(name); err != nil {
		return true, fmt.Errorf("join: removing %s: %w", name, err)
	}
	return true, store.SyncDir(root, ".")
}

// FinishJoin makes the held invitation the project change of this image, as [Session.TakeOver]
// does a take-over: the bridge is drained (rule N-16), the join is recorded, and the application
// is asked to quit; the joined project is written while the runtime closes, in a folder of its
// own, and Inkwell restarts on it (where it cannot restart itself, it closes and asks to be
// opened again). The page calls it from the restart page, once it left its context (rule N-15).
// It is refused when no verified Welcome is held.
func (s *Session) FinishJoin(ctx context.Context) error {
	if s.chg == nil {
		return errNoProject
	}
	return s.chg.finishHeld(ctx)
}

// CancelChange drops a held invitation: the page calls it when it could not save in time, so
// that nothing changes (rule N-15). The session then accepts requests again. It does nothing
// once [Session.FinishJoin] or [Session.TakeOver] was called.
func (s *Session) CancelChange() {
	if s.chg != nil {
		s.chg.cancelHeld()
	}
}

// PendingJoin reports whether the open project waits for the root document of a join, and what
// the page needs to wait for it: the root when the invitation stated one, and who invited; and,
// waiting or not, whether the way back to the document open before the join is offered (rules J-2
// and J-9).
func (s *Session) PendingJoin() JoinPending {
	name, back := s.previous()
	canReturn := back && s.previousFolder(name) != ""
	rec, ok := s.joinRecord()
	if !ok {
		return JoinPending{CanReturn: canReturn}
	}
	out := JoinPending{Waiting: true, Root: rec.Root, Inviter: rec.Inviter, CanReturn: canReturn}
	if dir := s.proj.Dir(); rec.Folder != "" && rec.Folder != dir {
		out.Folder = dir
	}
	if key, err := p2p.KeyFromID(transport.PeerID(rec.Inviter)); err == nil {
		for _, m := range s.inv.currentList().Members() {
			if m.Pub.Equal(key) {
				out.Name = m.Name
			}
		}
	}
	return out
}

// joinRecord returns the record of the join the open project waits for, when it is the record of
// the project of the loaded member list.
func (s *Session) joinRecord() (project.PendingJoin, bool) {
	if s.proj == nil || s.inv == nil {
		return project.PendingJoin{}, false
	}
	rec, ok := s.proj.PendingJoin()
	if !ok || rec.Project != s.inv.currentList().Project().String() {
		return project.PendingJoin{}, false
	}
	return rec, true
}

// CompleteJoin consumes the record of the join, once: it records url as the root document of the
// open project and removes the record, in one atomic rewrite of project.json. The page calls it
// when it holds the document and its storage flush returned; peer is the member that announced
// the root. It is refused without a record, for an invalid URL, for another root than the
// record's when it holds one, for another peer than the inviter when it holds none, for a peer
// that is not a member, and while the Go storage holds no chunk of the document: before the
// consumption no root is recorded, so no start can put another document under the joined list
// (rule J-1).
//
// The root is not signed yet (neither the Welcome nor the member list carries it): while the
// record holds none, only the inviter's word binds the project, for good, to a document. Another
// member of the joined list, even a removed one later, never does.
func (s *Session) CompleteJoin(url, peer string) error {
	rec, ok := s.joinRecord()
	if !ok {
		return errNoJoinRecord
	}
	if !validRootURL(url) {
		return errBadRootURL
	}
	if rec.Root != "" && url != rec.Root {
		return errJoinRoot
	}
	if rec.Root == "" && peer != rec.Inviter {
		return errJoinInviter
	}
	if key, err := p2p.KeyFromID(transport.PeerID(peer)); err != nil || s.inv.wiring.list().Role(key) == 0 {
		return errJoinPeer
	}
	if !s.stored(url) {
		return errJoinNotStored
	}
	return s.proj.CompleteJoin(url)
}

// stored reports whether the storage of the open project holds a chunk of the document url: a
// snapshot or an incremental change, as the storage adapter of the page saves them.
func (s *Session) stored(url string) bool {
	id := strings.TrimPrefix(url, "automerge:")
	for _, kind := range []string{"snapshot", "incremental"} {
		if entries, err := s.proj.LoadRange([]string{id, kind}); err == nil && len(entries) > 0 {
			return true
		}
	}
	return false
}

// returnedFile, in .inkwell of the folder a person returned to, tells its next start that the
// restart which opens it followed no change of that folder: it says nothing (rule J-2).
const returnedFile = "returned"

// previous returns the name of the folder that was open when the invitation that made the open
// folder was accepted: the record's while its join waits (rule J-2), the one [Session.CompleteJoin]
// kept after (rule J-9); ok is false when the open folder was not made by a join.
func (s *Session) previous() (name string, ok bool) {
	if rec, waits := s.joinRecord(); waits {
		name = rec.Previous
	} else if s.proj != nil {
		name = s.proj.Previous()
	}
	return name, name != ""
}

// previousFolder returns the path of the folder name beside the open one, when it still is a
// project; "" otherwise.
func (s *Session) previousFolder(name string) string {
	parent, err := os.OpenRoot(filepath.Dir(s.proj.Path()))
	if err != nil {
		return ""
	}
	defer func() { _ = parent.Close() }() // read-only handle
	if !project.IsProject(parent, name) {
		return ""
	}
	return filepath.Join(parent.Name(), name)
}

// ReturnToPrevious restarts Inkwell on the document that was open before the join that made the
// open folder, while that join waits (rule J-2) and after it completed (rule J-9): it is the
// project change of this image, as a take-over or a join is (the bridge is drained, rule N-16;
// what it writes is written while the runtime closes, rule N-13). It writes nothing in the open
// folder: a join that waits keeps waiting there and resumes whenever that folder is opened. It
// only dates the previous folder as the most recent one, so that the next launches open it too
// ([project.Prefer]). It is refused when no join made the open folder, when the previous folder
// is no longer a project, and once a change was requested.
func (s *Session) ReturnToPrevious(ctx context.Context) error {
	if err := s.leaving(); err != nil {
		return err
	}
	name, ok := s.previous()
	if !ok || s.previousFolder(name) == "" {
		return errNoPrevious
	}
	return s.requestChange(ctx, ChangeReturn, func(*opStore) error {
		previous, err := returnTo(filepath.Dir(s.proj.Path()), name)
		s.chg.setJoined(previous)
		return err
	})
}

// returnTo makes the project in the folder name of the folder at parent the one the next
// launches open, leaves it the word that its next start follows no change of its own, and
// returns its path; "" when it is no longer a project: the restart then opens the folder that
// waits again.
func returnTo(parent, name string) (string, error) {
	root, err := os.OpenRoot(parent)
	if err != nil {
		return "", fmt.Errorf("return: opening the parent folder: %w", err)
	}
	defer func() { _ = root.Close() }() // what is written is synced below
	if err := project.Prefer(root, name); err != nil {
		return "", fmt.Errorf("return: %w", err)
	}
	// Without it the previous document opens all the same, with the sentence of a take-over.
	err = store.WriteAtomic(root, name+"/"+project.PrivateDir+"/"+returnedFile, nil, 0o600)
	if err != nil {
		err = fmt.Errorf("return: leaving the word for the next start: %w", err)
	}
	return filepath.Join(root.Name(), name), err
}

// takeReturned reports whether the person returned to the project p from a join that waits, and
// forgets it.
func takeReturned(p *project.Project) bool {
	ink, err := p.OpenDir(".")
	if err != nil {
		return false
	}
	defer func() { _ = ink.Close() }()
	return ink.Remove(returnedFile) == nil
}
