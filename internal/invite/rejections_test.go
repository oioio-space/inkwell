package invite_test

import (
	"errors"
	"testing"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/trust"
)

func TestInviteInvalidRoleAndPerson(t *testing.T) {
	p := newProject(t)
	for _, role := range []trust.Role{0, 4, 255} {
		if _, err := p.host.Invite(role, nil); !errors.Is(err, invite.ErrMalformed) {
			t.Fatalf("role %d: %v", role, err)
		}
		if err := p.host.Answer(0, true, role); !errors.Is(err, invite.ErrMalformed) {
			t.Fatalf("answer role %d: %v", role, err)
		}
	}
	person := trust.Hash{255}
	if _, err := p.host.Invite(trust.RoleWriter, &person); !errors.Is(err, invite.ErrUnknownPerson) {
		t.Fatal(err)
	}
}

func TestCancelledTicketCannotBeRedeemed(t *testing.T) {
	p := newProject(t)
	ticket, err := p.host.Invite(trust.RoleWriter, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.host.Cancel(ticket)
	p.host.Cancel(ticket) // cancellation is idempotent
	bob := testDevice(t)
	_, err = p.joiner(bob, "Bob").Redeem(t.Context(), ticket, p.clock.Now())
	wantRefused(t, err, invite.ReasonInvalid)
	if len(p.list.Members()) != 1 {
		t.Fatal("cancelled ticket admitted a member")
	}
}
