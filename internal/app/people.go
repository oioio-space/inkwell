package app

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

var (
	// errUnknownPerson is returned for a person or device that is not a member.
	errUnknownPerson = errors.New("no such person")
	// errSelf is returned when suspending a person one of whose devices is this one.
	errSelf = errors.New("you cannot disconnect yourself")
)

// Device is one device of a [Person].
type Device struct {
	// Peer identifies the device.
	Peer string `json:"peer"`
	// Suspended reports whether the device is disconnected by an admin.
	Suspended bool `json:"suspended"`
}

// Person is one line of the people list: the devices of a person, grouped.
type Person struct {
	// ID names the person in [Session.Suspend] and the other methods; a
	// device's Peer is accepted too.
	ID string `json:"id"`
	// Name is the name an admin gave the person; empty for the project's creator.
	Name string `json:"name"`
	// Role is "reader", "writer" or "admin" (the highest of its devices). A
	// suspended person keeps it.
	Role string `json:"role"`
	// SuspendedBy is the name of the admin who disconnected the person, the
	// first of its suspended devices that is.
	SuspendedBy string `json:"suspendedBy,omitzero"`
	// SuspendedByPeer identifies that admin's device.
	SuspendedByPeer string `json:"suspendedByPeer,omitzero"`
	// Devices lists the devices, in the order they joined. A person is shown
	// as still connected on some device when only some are suspended (an
	// admin may add a device while the others are being disconnected).
	Devices []Device `json:"devices"`
	// SuspendedAt is when this device received the news of that suspension.
	// Operations carry no clock: it is the local time of arrival.
	SuspendedAt time.Time `json:"suspendedAt,omitzero"`
	// Self reports whether one of the devices is this one.
	Self bool `json:"self"`
	// Suspended reports whether at least one device is suspended.
	Suspended bool `json:"suspended"`
}

func roleName(r trust.Role) string {
	switch r {
	case trust.RoleReader:
		return "reader"
	case trust.RoleWriter:
		return "writer"
	case trust.RoleAdmin:
		return "admin"
	}
	return ""
}

// people groups the members of the project by person.
func (inv *invites) people() []Person {
	members := inv.currentList().Members()
	self := inv.dev.Public()
	var out []Person
	index := make(map[trust.Hash]int)
	for _, m := range members {
		i, ok := index[m.Person]
		if !ok {
			i = len(out)
			index[m.Person] = i
			out = append(out, Person{ID: m.Person.String()})
		}
		p := &out[i]
		if p.Name == "" {
			p.Name = m.Name
		}
		if m.Role >= roleLevel(p.Role) {
			p.Role = roleName(m.Role)
		}
		p.Self = p.Self || m.Pub.Equal(self)
		p.Devices = append(p.Devices, Device{Peer: string(p2p.IDFromKey(m.Pub)), Suspended: m.Suspended})
		if m.Suspended && !p.Suspended {
			p.Suspended = true
			p.SuspendedByPeer = string(p2p.IDFromKey(m.SuspendedBy))
			for _, o := range members {
				if o.Pub.Equal(m.SuspendedBy) {
					p.SuspendedBy = o.Name
				}
			}
			p.SuspendedAt = inv.store.receivedAt(m.SuspendedOp)
		}
	}
	return out
}

// roleLevel returns the trust role named name, or 0.
func roleLevel(name string) trust.Role {
	r, err := parseRole(name)
	if err != nil {
		return 0
	}
	return r
}

// devicesOf returns the members that are devices of the person who names: a
// person ID, or the peer of one of its devices.
func (inv *invites) devicesOf(who string) ([]trust.Member, error) {
	members := inv.currentList().Members()
	i := slices.IndexFunc(members, func(m trust.Member) bool {
		return m.Person.String() == who || p2p.IDFromKey(m.Pub) == transport.PeerID(who)
	})
	if i < 0 {
		return nil, errUnknownPerson
	}
	person := members[i].Person
	return slices.DeleteFunc(members, func(m trust.Member) bool { return m.Person != person }), nil
}

// issueFor issues one op, built by op, for each device of the person who
// names that keep accepts, as a batch.
func (inv *invites) issueFor(who string, keep func(trust.Member) bool, op func(trust.Member) trust.Op) error {
	devices, err := inv.devicesOf(who)
	if err != nil {
		return err
	}
	var tmpls []trust.Op
	for _, m := range devices {
		if keep(m) {
			tmpls = append(tmpls, op(m))
		}
	}
	if len(tmpls) == 0 {
		return nil
	}
	if _, err := inv.wiring.IssueBatch(tmpls); err != nil {
		return fmt.Errorf("people: %w", err)
	}
	return nil
}

// Members returns the people of the project, the creator first, then in the
// order they joined.
func (s *Session) Members() []Person {
	if s.inv == nil {
		return nil
	}
	return s.inv.people()
}

// Suspend disconnects the person who, a person ID or the peer of one of its
// devices, on all its devices: the others refuse what it writes until
// [Session.Resume]. It keeps its role. Only an admin can, and not on a more
// senior admin, nor on oneself.
func (s *Session) Suspend(who string) error {
	if s.inv == nil {
		return errNoProject
	}
	devices, err := s.inv.devicesOf(who)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(devices, func(m trust.Member) bool { return m.Pub.Equal(s.inv.dev.Public()) }) {
		return errSelf
	}
	return s.inv.issueFor(who,
		func(m trust.Member) bool { return !m.Suspended },
		func(m trust.Member) trust.Op { return trust.Op{Kind: trust.OpSuspend, Subject: m.Pub} })
}

// Resume lets back the person who, on all its devices that are suspended. What
// it wrote meanwhile is accepted again, with the same role as before. Like
// [Session.Suspend], it refuses a person one of whose devices is this one.
func (s *Session) Resume(who string) error {
	if s.inv == nil {
		return errNoProject
	}
	devices, err := s.inv.devicesOf(who)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(devices, func(m trust.Member) bool { return m.Pub.Equal(s.inv.dev.Public()) }) {
		return errSelf // a sibling device that escaped the suspension must not lift it.
	}
	return s.inv.issueFor(who,
		func(m trust.Member) bool { return m.Suspended },
		func(m trust.Member) trust.Op { return trust.Op{Kind: trust.OpResume, Subject: m.Pub} })
}

// SetRole gives the person who, on all its devices, the role "reader",
// "writer" or "admin". Only an admin can, and not on a more senior admin.
func (s *Session) SetRole(who, role string) error {
	if s.inv == nil {
		return errNoProject
	}
	r, err := parseRole(role)
	if err != nil {
		return err
	}
	return s.inv.issueFor(who,
		func(m trust.Member) bool { return m.Role != r },
		func(m trust.Member) trust.Op { return trust.Op{Kind: trust.OpSetRole, Subject: m.Pub, Role: r} })
}

// Remove removes the person who from the project, on all its devices. It is
// final: a person who comes back is admitted anew. Only an admin can, and not
// on a more senior admin.
func (s *Session) Remove(who string) error {
	if s.inv == nil {
		return errNoProject
	}
	return s.inv.issueFor(who,
		func(trust.Member) bool { return true },
		func(m trust.Member) trust.Op { return trust.Op{Kind: trust.OpRemove, Subject: m.Pub} })
}
