//go:build perf_native && !production && linux && !server

package main

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
)

type fakeCheckConn struct {
	mu       sync.Mutex
	sent     []string
	received chan []byte
	gotSend  chan struct{}
}

func (f *fakeCheckConn) Send(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, string(data))
	close(f.gotSend)
	return nil
}

func (f *fakeCheckConn) Receive() ([]byte, error) {
	data, ok := <-f.received
	if !ok {
		<-f.gotSend // the webview closes only after the driver's command went through
		return nil, errors.New("closed")
	}
	return data, nil
}

func TestServeTypingCheckRelaysBothWays(t *testing.T) {
	conn := &fakeCheckConn{received: make(chan []byte, 2), gotSend: make(chan struct{})}
	conn.received <- []byte(`{"type":"ready"}`)
	conn.received <- []byte(`{"type":"state","doc":"ê"}`)
	close(conn.received)
	lines := readLines(strings.NewReader("{\"cmd\":\"state\"}\n"))
	var out bytes.Buffer
	serveTypingCheck(conn, lines, &out)

	want := typingCheckPrefix + `{"type":"ready"}` + "\n" + typingCheckPrefix + `{"type":"state","doc":"ê"}` + "\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.sent) != 1 || conn.sent[0] != `{"cmd":"state"}` {
		t.Errorf("sent = %q, want the stdin command", conn.sent)
	}
}
