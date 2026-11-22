package p2p

import (
	"bytes"
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p/core/network"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// The host received the request and replied before the caller could half-close.
// QUIC's STOP_SENDING cancels the write half while the reply remains readable.
type answeredLobbyStream struct {
	network.MuxedStream
	request  bytes.Buffer
	response bytes.Buffer
}

func (s *answeredLobbyStream) Write(p []byte) (int, error) { return s.request.Write(p) }
func (s *answeredLobbyStream) Read(p []byte) (int, error)  { return s.response.Read(p) }
func (*answeredLobbyStream) CloseWrite() error             { return errors.New("close called for canceled stream") }
func (*answeredLobbyStream) Close() error                  { return nil }
func (*answeredLobbyStream) Reset() error                  { return nil }

func TestLobbyReadsReplyAfterHostStopsReceiving(t *testing.T) {
	var stream answeredLobbyStream
	if err := frame.Write(&stream.response, []byte("welcome")); err != nil {
		t.Fatal(err)
	}
	got, err := exchangeLobby(t.Context(), &stream, "host", []byte("redeem"))
	if err != nil || string(got) != "welcome" {
		t.Fatalf("exchangeLobby returned %q, %v; want welcome", got, err)
	}
	req, err := frame.Read(&stream.request, MaxLobbyRequest)
	if err != nil || string(req) != "redeem" {
		t.Fatalf("host received %q, %v; want redeem", req, err)
	}
}
