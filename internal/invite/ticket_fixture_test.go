package invite_test

import (
	"crypto/sha256"
	"time"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

const testHost transport.PeerID = "12D3KooWExampleHostPeerIdentityForTicketTests1234"

func sampleTicket() invite.Ticket {
	person := trust.Hash(sha256.Sum256([]byte("person")))
	return invite.Ticket{
		Project: trust.ProjectID(sha256.Sum256([]byte("project"))),
		Host:    testHost,
		Addrs: []string{
			"/ip4/192.168.1.20/udp/7419/quic-v1/p2p/" + string(testHost),
			"/ip4/192.168.1.20/tcp/7419/p2p/" + string(testHost),
		},
		Secret:  [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Role:    trust.RoleWriter,
		Person:  &person,
		Expires: time.Unix(1_800_000_000, 0).UTC(),
	}
}
