package invite_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the run if a test leaves a goroutine behind, except two that
// belong to wormhole-william: the goroutine that delivers the mailbox messages
// of a finished transfer never ends (it waits on a channel nothing closes),
// and neither does the handler of the test server that served it. They are
// bounded (one per transfer) and are not ours to stop.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(
		m,
		goleak.IgnoreAnyFunction("github.com/psanford/wormhole-william/rendezvous.(*Client).recvMailboxMsgs"),
		goleak.IgnoreAnyFunction("github.com/psanford/wormhole-william/rendezvous/rendezvousservertest.(*TestServer).handleWS.func5"),
	)
}
