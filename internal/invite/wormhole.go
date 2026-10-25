package invite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/psanford/wormhole-william/wormhole"
)

// AppID is the Wormhole application identifier of inkwell invitations: a
// client with another one cannot receive them.
const AppID = "inkwell.app/invite/v1"

// maxCodeLen bounds a Wormhole code: a nameplate, then words.
const maxCodeLen = 64

// codePattern matches a Wormhole code such as "7-guitarist-revenge".
var codePattern = regexp.MustCompile(`^[0-9]+(-[a-z]+)+$`)

// LooksLikeCode reports whether s has the shape of a Wormhole code, to tell it
// from invitation text.
func LooksLikeCode(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) <= maxCodeLen && codePattern.MatchString(s)
}

// Wormhole carries a [Ticket] over the magic-wormhole protocol: a short code
// that is typed in, authenticated by a PAKE, so that a ticket reaches the
// person who has the code and nobody else. The transfer is one-way and is
// text only: no transit relay is involved.
type Wormhole struct {
	// RendezvousURL is the WebSocket URL of the rendezvous (mailbox) server.
	// Empty means the public magic-wormhole server.
	RendezvousURL string
}

func (w Wormhole) client() *wormhole.Client {
	return &wormhole.Client{AppID: AppID, RendezvousURL: w.RendezvousURL}
}

// Offer publishes t and returns the code to give to the other person. done
// receives one value, nil once the other side has received the ticket, and is
// then closed; canceling ctx withdraws the offer (done then receives
// ctx.Err()). Offer returns as soon as the code is known.
func (w Wormhole) Offer(ctx context.Context, t Ticket) (code string, done <-chan error, err error) {
	code, status, err := w.client().SendText(ctx, t.Text())
	if err != nil {
		return "", nil, fmt.Errorf("invite: wormhole offer: %w", err)
	}
	out := make(chan error, 1)
	go func() {
		defer close(out)
		select {
		case res := <-status:
			switch {
			case res.Error != nil:
				out <- fmt.Errorf("invite: wormhole transfer: %w", res.Error)
			case !res.OK:
				out <- errors.New("invite: wormhole transfer failed")
			}
		case <-ctx.Done():
			out <- ctx.Err()
		}
	}()
	return code, out, nil
}

// Fetch receives the ticket offered with code. It does not check the expiry.
func (w Wormhole) Fetch(ctx context.Context, code string) (Ticket, error) {
	code = strings.TrimSpace(code)
	if !LooksLikeCode(code) {
		return Ticket{}, fmt.Errorf("%w: not a code", ErrMalformed)
	}
	msg, err := w.client().Receive(ctx, code)
	if err != nil {
		return Ticket{}, fmt.Errorf("invite: wormhole receive: %w", err)
	}
	if msg.Type != wormhole.TransferText {
		_ = msg.Reject()
		return Ticket{}, fmt.Errorf("%w: not text", ErrMalformed)
	}
	b, err := io.ReadAll(io.LimitReader(msg, MaxTextLen+1))
	if err != nil {
		return Ticket{}, fmt.Errorf("invite: wormhole read: %w", err)
	}
	return ParseText(string(b))
}
