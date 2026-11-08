//go:build perf_native && !production && linux && !server

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The typing checks (scripts/perf/typing_checks.py) drive the real perf-build window with real
// keys and read CodeMirror's state back: the driver's stdin lines go to the webview, the
// webview's messages come out as stdout lines with this prefix. Perf builds only; no measurement
// runs in this phase.
const (
	typingCheckPhase  = "typing-check"
	typingCheckStream = "typing-check"
	typingCheckPrefix = "INKWELL_TYPING_CHECK "
)

type checkConn interface {
	Send([]byte) error
	Receive() ([]byte, error)
}

func runTypingCheck() {
	lines := readLines(os.Stdin)
	run(func(app *application.App, window *application.WebviewWindow) {
		window.SetURL("/?typing-check=1")
		app.HandleStream(typingCheckStream, func(c *application.StreamConn) { serveTypingCheck(c, lines, os.Stdout) })
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { window.Show(); window.Focus() })
	})
}

// readLines reads the driver's commands once for the whole process, so a reconnected stream
// takes over the same queue.
func readLines(in io.Reader) <-chan []byte {
	lines := make(chan []byte)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for scanner.Scan() {
			lines <- slices.Clone(scanner.Bytes())
		}
	}()
	return lines
}

// serveTypingCheck relays one stream until either side closes.
func serveTypingCheck(c checkConn, lines <-chan []byte, out io.Writer) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			data, err := c.Receive()
			if err != nil {
				return
			}
			fmt.Fprintf(out, "%s%s\n", typingCheckPrefix, data)
		}
	}()
	for {
		select {
		case <-done:
			return
		case line, ok := <-lines:
			if !ok || c.Send(line) != nil {
				return
			}
		}
	}
}
