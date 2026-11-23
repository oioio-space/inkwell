package project

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

// A project joined by a restart (plan docs/plans/2026-11-15-pc1b-join.md, rules J-1 and J-6) is
// written whole in a staging folder beside the current project, then installed under its name by
// a rename that never replaces ([RenameNoReplace]). Its project.json holds the record of the join
// until the root document is stored.

// StagingPrefix starts the name of a staging folder, followed by the id of its record. Such a
// folder is never a project to open: [Latest] ignores it.
const StagingPrefix = ".inkwell-join-"

// ErrPendingJoin is returned by [Project.SetRootURL] while the project waits for the root
// document of a join.
var ErrPendingJoin = errors.New("project: waiting for the document of a join")

// PendingJoin is the record of a join that waits for its root document: the field pendingJoin
// of project.json, from the staging of the joined project until [Project.CompleteJoin].
type PendingJoin struct {
	// V is the version of the record: 1.
	V int `json:"v"`
	// ID is 128 random bits in hex; the staging folder was named after it.
	ID string `json:"id"`
	// Project is the id of the joined project: the record counts only under a member list of
	// that project.
	Project string `json:"project"`
	// Inviter is the peer identity of the device that admitted this one.
	Inviter string `json:"inviter"`
	// Addrs are the addresses the inviter was reached at.
	Addrs []string `json:"addrs"`
	// Root is the automerge URL of the root document when the invitation stated one; empty when
	// a member announces it later.
	Root string `json:"root"`
	// Previous is the name of the folder that was open when the invitation was accepted, beside
	// this one: the person can return to it while the join waits. Empty when unknown.
	Previous string `json:"previous,omitzero"`
	// Folder is the name the person was told this folder would get; the install may have taken
	// another one, when the name was taken meanwhile.
	Folder string `json:"folder,omitzero"`
}

// CreateStaging makes the staging folder of the join record id in base, with exclusive create,
// holding a new project titled title, and returns it open. It fails with an error wrapping
// [fs.ErrExist] when the folder exists. The project has no record yet: see
// [Project.SetPendingJoin].
func CreateStaging(base *os.Root, id, title string) (*Project, error) {
	dir := StagingPrefix + id
	if err := base.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("project: create folder %q: %w", dir, err)
	}
	return createIn(base, dir, title, "")
}

// SetPendingJoin writes rec as the record of the join this project waits for, atomically.
func (p *Project) SetPendingJoin(rec PendingJoin) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	old := p.m.PendingJoin
	rec.Addrs = slices.Clone(rec.Addrs)
	p.m.PendingJoin = &rec
	if err := p.saveLocked(); err != nil {
		p.m.PendingJoin = old
		return err
	}
	return nil
}

// PendingJoin returns the record of the join this project waits for; ok is false when it waits
// for none.
func (p *Project) PendingJoin() (rec PendingJoin, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m.PendingJoin == nil {
		return PendingJoin{}, false
	}
	rec = *p.m.PendingJoin
	rec.Addrs = slices.Clone(rec.Addrs)
	return rec, true
}

// CompleteJoin consumes the record of a join: one atomic rewrite of project.json records rootURL
// as the root document, keeps the record's previous folder ([Project.Previous], rule J-9) and
// removes the record. The caller checks first that there is a record and that the document is
// stored.
func (p *Project) CompleteJoin(rootURL string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	oldRoot, oldRec, oldPrevious := p.m.RootURL, p.m.PendingJoin, p.m.Previous
	if oldRec != nil {
		p.m.Previous = oldRec.Previous
	}
	p.m.RootURL = rootURL
	p.m.PendingJoin = nil
	if err := p.saveLocked(); err != nil {
		p.m.RootURL, p.m.PendingJoin, p.m.Previous = oldRoot, oldRec, oldPrevious
		return err
	}
	return nil
}

// Previous returns the name of the folder that was open when the invitation that made this
// folder was accepted, beside this one, as [Project.CompleteJoin] kept it; "" before the join
// completed (the record holds it then, [PendingJoin.Previous]) and when no join made this folder.
func (p *Project) Previous() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Previous
}

// errNotAChild refuses a name that is not one plain entry of the folder.
var errNotAChild = errors.New("project: not a direct child name")

// childName refuses a name that could designate anything but a direct child of a folder.
func childName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`+"\x00") {
		return fmt.Errorf("%w: %q", errNotAChild, name)
	}
	return nil
}

// RenameNoReplace renames the direct child src of base to the direct child dst in one system
// call that never replaces: it fails with [fs.ErrExist] when dst exists, whatever it is (a
// folder, a file, a symbolic link, which is not followed). Where the system or the file system
// has no such call it fails and renames nothing.
func RenameNoReplace(base *os.Root, src, dst string) error {
	if err := errors.Join(childName(src), childName(dst)); err != nil {
		return err
	}
	return renameChild(base, src, dst)
}

// PrivateDir is the name of the private directory of a project folder, for code that leaves a
// word in a project it has not open.
const PrivateDir = inkwellDir

// IsProject reports whether the direct child dir of base is a project folder.
func IsProject(base *os.Root, dir string) bool {
	if childName(dir) != nil {
		return false
	}
	_, err := base.Stat(dir + "/" + inkwellDir + "/" + manifestFile)
	return err == nil
}

// preferStep is how far past the most recent other project [Prefer] dates a project when the
// clock is not past it already: more than the coarsest time stamp of a file system (2 s, FAT).
const preferStep = 3 * time.Second

// Prefer makes the project in the direct child dir of base the one [Latest] returns: its
// project.json is dated now, or past every other project of base when one is dated later than
// now. The person chose to return to it, and the next launch opens it (plan
// docs/plans/2026-11-15-pc1b-join.md, rule J-2). It fails with an error wrapping [ErrNoProject]
// when dir is no project.
func Prefer(base *os.Root, dir string) error {
	if !IsProject(base, dir) {
		return fmt.Errorf("project: prefer %q: %w", dir, ErrNoProject)
	}
	at := time.Now()
	// One turn dates it and the next sees it; one more for a project written in between.
	for range 3 {
		latest, err := Latest(base)
		if err != nil {
			return err
		}
		if latest == dir {
			return nil
		}
		if info, err := base.Stat(latest + "/" + inkwellDir + "/" + manifestFile); err == nil && !at.After(info.ModTime()) {
			at = info.ModTime().Add(preferStep)
		}
		if err := base.Chtimes(dir+"/"+inkwellDir+"/"+manifestFile, at, at); err != nil {
			return fmt.Errorf("project: prefer %q: %w", dir, err)
		}
		at = at.Add(preferStep)
	}
	return fmt.Errorf("project: prefer %q: another project stays the most recent", dir)
}
