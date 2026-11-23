//go:build linux && !server && !gtk3 && wc1native

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/sys/unix"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
)

const nativeProjection = "Committed prefix"
const sheetProjection = "a;b;c;d\n1;2;3;4\n5;6;7;8\n9;10;11;12\n"
const joinProjection = "Text of the person who invites"

type walkEvent struct {
	Event string          `json:"event"`
	Value json.RawMessage `json:"value,omitempty"`
}

type lifecycleEvent struct {
	Scenario string `json:"scenario"`
	Event    string `json:"event"`
	Request  string `json:"request"`
	Window   string `json:"window"`
	PID      int    `json:"pid"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

type walkCommand struct {
	Command string `json:"command"`
	Value   string `json:"value,omitempty"`
}

type nativeStoreFault struct {
	bridge.KV
	mode, marker string
	record       func(string, any)
}

func (f nativeStoreFault) Save(key []string, value []byte) error {
	if len(key) >= 2 && (key[1] == "snapshot" || key[1] == "incremental") {
		if _, err := os.Stat(f.marker); err == nil {
			switch f.mode {
			case "disk":
				f.record("fault-at-store", f.mode)
				return unix.ENOSPC
			case "store":
				f.record("fault-at-store", f.mode)
				<-make(chan struct{}) // the supervised process must exit with this write still stuck
			default:
				// A failing renderer leaves the store healthy: its last throttled
				// write may carry the typed cut before the renderer stops.
				err := f.KV.Save(key, value)
				if err == nil {
					f.record("cut-stored", f.mode)
				}
				return err
			}
		}
	}
	return f.KV.Save(key, value)
}

func init() {
	childModes["wc1-native"] = func() int { return runNativeChild(os.Getenv("WC1_NATIVE_CHILD")) }
}

func appendNativeJSON(file *os.File, row any) error {
	data, err := json.Marshal(row)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n')) // a single append per ordered observation
	return err
}

func nativeWalkEvent(scenario, event string) bool {
	switch event {
	case "native-error":
		return true
	case "input-accepted", "save-ack-durable":
		return scenario == "SAV-03b"
	case "goodbye-handed-off":
		return scenario == "SHEET-MU-01b"
	case "join-held":
		return scenario == "PC1b-join"
	case "close-authorized":
		return scenario == "SAV-03b" || scenario == "SHEET-MU-01b" || scenario == "PC1b-join"
	case "gtk-close-request", "wails-close-vetoed", "close-permit-consumed", "gtk-window-destroyed":
		return scenario == "SAV-03b" || scenario == "SHEET-MU-01b"
	default:
		return false
	}
}

func requireFixtureDir(path, fixture string) error {
	if path == "" || !filepath.IsAbs(path) || !strings.HasPrefix(path, fixture+string(os.PathSeparator)) {
		return fmt.Errorf("path %q is outside fixture %q", path, fixture)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("fixture directory %q is not populated: %v", path, err)
	}
	return nil
}

func runNativeChild(root string) int {
	for _, variable := range []string{"INKWELL_DOCS", "INKWELL_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME"} {
		if err := requireFixtureDir(os.Getenv(variable), root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	transcript, err := os.OpenFile(filepath.Join(root, "events.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer transcript.Close()
	mode, scenario, proofPath := os.Getenv("WC1_NATIVE_MODE"), os.Getenv("WC1_NATIVE_SCENARIO"), os.Getenv("WC1_NATIVE_PROOF")
	proof, err := os.OpenFile(proofPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer proof.Close()
	request := os.Getenv("WC1_NATIVE_REQUEST")
	// The image a project change restarts in place opens the folder the restart names (rule N-12).
	args := []string{filepath.Join(os.Getenv("INKWELL_DOCS"), "Project", "main.typ")}
	if os.Getenv(afterTakeOverEnv) == "1" {
		args = os.Args[1:]
	}
	var windowID string
	var observeMu sync.Mutex
	record := func(event string, value any) {
		observeMu.Lock()
		defer observeMu.Unlock()
		payload, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		if err := appendNativeJSON(transcript, walkEvent{event, payload}); err != nil {
			panic(err)
		}
		if nativeWalkEvent(scenario, event) {
			if err := appendNativeJSON(proof, lifecycleEvent{
				Scenario: scenario, Event: event, Request: request, Window: windowID, PID: os.Getpid(),
			}); err != nil {
				panic(err)
			}
		}
	}

	d := liveDeps(func(app *application.App, window *application.WebviewWindow) {
		windowID = fmt.Sprintf("%d/%d", os.Getpid(), window.ID())
		window.SetURL("/?wc1Native=" + mode)
		var installed bool
		window.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
			application.InvokeAsync(func() {
				if installed {
					return
				}
				installed = nativeWitness(window, func(event string) { record(event, nil) })
				if !installed {
					record("native-error", "GTK could not observe the native toplevel")
					return
				}
				record("gtk-ready", windowID)
			})
		})

		commands := make(chan walkCommand, 8)
		app.HandleStream("wc1-proof", func(c *application.StreamConn) {
			incoming := receive(c.Context(), c)
			for {
				select {
				case raw, ok := <-incoming:
					if !ok {
						return
					}
					var event walkEvent
					if err := json.Unmarshal(raw, &event); err != nil || event.Event == "" {
						record("native-error", "invalid proof page event")
						continue
					}
					record(event.Event, event.Value)
				case cmd := <-commands:
					if err := c.SendJSON(cmd); err != nil {
						record("native-error", err.Error())
						return
					}
				case <-c.Context().Done():
					return
				}
			}
		})
		go func() {
			in := bufio.NewScanner(os.Stdin)
			for in.Scan() {
				var cmd walkCommand
				if err := json.Unmarshal(in.Bytes(), &cmd); err != nil {
					record("native-error", err.Error())
					return
				}
				switch cmd.Command {
				case "focus":
					window.Focus()
					record("focus", nil)
				case "quit":
					app.Quit()
				case "native-state":
					status := application.InvokeSyncWithResult(func() int { return nativeViewStatus(window) })
					record("native-state", status)
				case "terminate-renderer":
					if !application.InvokeSyncWithResult(func() bool { return terminateRenderer(window) }) {
						record("native-error", "missing WebKitGTK renderer")
					}
				default:
					select {
					case commands <- cmd:
					case <-app.Context().Done():
						return
					}
				}
			}
		}()
	})
	realWindow := d.window
	d.window = func(ctx context.Context, s *session) int {
		record("image", nativeImage{
			PID: os.Getpid(), StartupID: s.rt.Session.StartupID(), Standing: s.rt.Session.Standing(),
			Members: len(s.rt.Session.Members()),
		})
		probe := closeProbe{observe: func(event string) { record(event, nil) }}
		if mode == "disk" || mode == "store" || mode == "hang" || mode == "crash" || mode == "auto" {
			probe.wrapKV = func(kv bridge.KV) bridge.KV {
				return nativeStoreFault{kv, mode, filepath.Join(root, "fault-armed"), record}
			}
		}
		return realWindow(context.WithValue(ctx, closeProbeKey{}, probe), s)
	}
	return startup(args, d)
}

// nativeImage is what each process image of a native child reports once its runtime runs (event
// « image »): an execution in place keeps the pid and changes the startup id (rule N-12).
type nativeImage struct {
	PID       int    `json:"pid"`
	StartupID string `json:"startupId"`
	Standing  string `json:"standing"`
	Members   int    `json:"members"`
}

// images returns every image the child reported so far, in order, whatever the transcript cursor.
func (p *nativeProcess) images(t *testing.T) []nativeImage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(p.root, "events.jsonl"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	var images []nativeImage
	for line := range strings.Lines(string(b)) {
		var event walkEvent
		if json.Unmarshal([]byte(line), &event) == nil && event.Event == "image" {
			images = append(images, eventValue[nativeImage](t, event.Value))
		}
	}
	return images
}

// joinRestarting has the guest p accept the invitation ticket into another project, which holds
// the Welcome and asks before Inkwell restarts (PC1b, rules J-1 and J-2); it confirms « Redémarrer
// et ouvrir » as a person does, then follows the process while it restarts in place on the joined
// folder (same pid, another startup id, rule N-12) and returns that image and its page once the
// joined document shows. The steps are recorded in the lifecycle trace of p's scenario, if any.
func (p *nativeProcess) joinRestarting(t *testing.T, ticket string) (nativeImage, pageWitness) {
	t.Helper()
	before := p.images(t)
	if len(before) == 0 {
		t.Fatal("the guest reported no process image before the join")
	}
	p.send(t, "join", ticket)
	p.until(t, 20*time.Second, "join-held")
	p.send(t, "confirm-join")
	p.until(t, 5*time.Second, "confirm-join")
	var after []nativeImage
	for end := time.Now().Add(30 * time.Second); len(after) <= len(before); after = p.images(t) {
		select {
		case <-p.exited:
			t.Fatalf("the guest exited instead of restarting in place: %v", p.waitErr)
		default:
		}
		if time.Now().After(end) {
			t.Fatal("the guest did not restart within 30 s of « Redémarrer et ouvrir »")
		}
		time.Sleep(20 * time.Millisecond)
	}
	old, image := before[len(before)-1], after[len(after)-1]
	if image.PID != p.cmd.Process.Pid || old.PID != image.PID || image.StartupID == old.StartupID {
		t.Fatalf("N-12 the join restarts in place: image %+v after %+v (process %d)", image, old, p.cmd.Process.Pid)
	}
	if log, err := os.ReadFile(p.log); err != nil || !strings.Contains(string(log), "level=INFO msg=restarting change=join ") {
		t.Fatalf("J-7 the restart of the join is not logged: %v", err)
	}
	if p.scenario != "" {
		p.lifecycle(t, "process-restarted", image.PID, nil)
	}
	p.window = "" // the new image opens a window of its own
	return image, p.ready(t)
}

type nativeProcess struct {
	cmd        *exec.Cmd
	root       string
	window     string
	proof      string
	scenario   string
	request    string
	log        string
	input      *json.Encoder
	exited     chan struct{}
	waitErr    error
	transcript int
	seen       map[string]int // events read from the transcript, by name
}

func makeNativeProject(t *testing.T, root, projection string) {
	t.Helper()
	for _, name := range []string{"docs", "profile", "cache", "config", "data"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".fixture"), []byte("WC1 fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	docs, err := os.OpenRoot(filepath.Join(root, "docs"))
	if err != nil {
		t.Fatal(err)
	}
	defer docs.Close()
	proj, err := project.Create(docs, "Project", "")
	if err != nil {
		t.Fatal(err)
	}
	defer proj.Close()
	if err := proj.WriteProjection(project.MainFile, []byte(projection)); err != nil {
		t.Fatal(err)
	}
}

func nativeFixture(t *testing.T) string {
	t.Helper()
	// Keep the instance socket address within Unix's sun_path limit even for
	// long subtest names and evidence directory paths.
	root, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "case-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func nativeProcessStart(t *testing.T, root, mode, scenario, request, proof string) *nativeProcess {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(),
		childEnv+"=wc1-native",
		"WC1_NATIVE_CHILD="+root, "WC1_NATIVE_MODE="+mode, "WC1_NATIVE_SCENARIO="+scenario,
		"WC1_NATIVE_REQUEST="+request, "WC1_NATIVE_PROOF="+proof, "INKWELL_RENDEZVOUS=off",
	)
	for key, suffix := range map[string]string{
		"INKWELL_DOCS": "docs", "INKWELL_HOME": "profile", "XDG_CACHE_HOME": "cache",
		"XDG_DATA_HOME": "data", "XDG_CONFIG_HOME": "config",
	} {
		dir := filepath.Join(root, suffix)
		if err := requireFixtureDir(dir, root); err != nil {
			t.Fatal(err)
		}
		cmd.Env = append(cmd.Env, key+"="+dir)
	}
	out, err := os.CreateTemp(root, "native-app-*.log")
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = out, out
	in, err := cmd.StdinPipe()
	if err != nil {
		out.Close()
		t.Fatal(err)
	}
	p := &nativeProcess{
		cmd: cmd, root: root, log: out.Name(), input: json.NewEncoder(in), exited: make(chan struct{}),
		scenario: scenario, proof: proof, request: request, seen: make(map[string]int),
	}
	// Do not reread events of the old process on relaunch.
	old, err := os.ReadFile(filepath.Join(root, "events.jsonl"))
	if err == nil {
		p.transcript = len(strings.Split(strings.TrimSpace(string(old)), "\n"))
	}
	if err := cmd.Start(); err != nil {
		out.Close()
		t.Fatal(err)
	}
	go func() {
		defer close(p.exited)
		p.waitErr = cmd.Wait()
		out.Close()
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill() // only the child this fixture started
		<-p.exited
		if t.Failed() {
			if log, err := os.ReadFile(p.log); err == nil {
				t.Logf("app %d (%s):\n%s", cmd.Process.Pid, mode, log[max(0, len(log)-9000):])
			}
			if log, err := os.ReadFile(filepath.Join(root, "events.jsonl")); err == nil {
				t.Logf("app evidence:\n%s", log[max(0, len(log)-5000):])
			}
		}
	})
	return p
}

func eventValue[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decoding app evidence %s: %v", raw, err)
	}
	return result
}

func (p *nativeProcess) until(t *testing.T, bound time.Duration, events string) walkEvent {
	t.Helper()
	end := time.Now().Add(bound)
	for time.Now().Before(end) {
		b, err := os.ReadFile(filepath.Join(p.root, "events.jsonl"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		lines := strings.Split(string(b), "\n")
		for p.transcript < len(lines)-1 {
			var event walkEvent
			if err := json.Unmarshal([]byte(lines[p.transcript]), &event); err != nil {
				t.Fatal(err)
			}
			p.transcript++
			p.seen[event.Event]++
			if event.Event == "page-error" || event.Event == "native-error" {
				t.Fatalf("app reported %s: %s", event.Event, event.Value)
			}
			if strings.Contains(" "+events+" ", " "+event.Event+" ") {
				return event
			}
		}
		select {
		case <-p.exited:
			t.Fatalf("app exited while waiting for %s: %v", events, p.waitErr)
		case <-time.After(5 * time.Millisecond):
		}
	}
	t.Fatalf("timed out waiting for %s", events)
	return walkEvent{}
}

func (p *nativeProcess) send(t *testing.T, name string, values ...string) {
	t.Helper()
	var value string
	if len(values) > 0 {
		value = values[0]
	}
	if err := p.input.Encode(walkCommand{name, value}); err != nil {
		t.Fatal(err)
	}
}

type pageWitness struct {
	Text    string
	Unsaved string
	Remote  int
	Peers   int
}

func (p *nativeProcess) ready(t *testing.T) pageWitness {
	t.Helper()
	var state pageWitness
	pageReady := false
	for p.window == "" || !pageReady {
		e := p.until(t, 20*time.Second, "gtk-ready page-ready")
		if e.Event == "gtk-ready" {
			p.window = eventValue[string](t, e.Value)
		} else {
			state = eventValue[pageWitness](t, e.Value)
			pageReady = true
		}
	}
	return state
}

func (p *nativeProcess) seed(t *testing.T, text string) {
	t.Helper()
	p.send(t, "write", text)
	p.until(t, 5*time.Second, "write")
	p.send(t, "durable")
	p.until(t, 5*time.Second, "durable")
}

func (p *nativeProcess) lifecycle(t *testing.T, event string, pid int, exit *int) {
	t.Helper()
	file, err := os.OpenFile(p.proof, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := appendNativeJSON(file, lifecycleEvent{
		Scenario: p.scenario, Event: event, Request: p.request, Window: p.window, PID: pid, ExitCode: exit,
	}); err != nil {
		t.Fatal(err)
	}
}

func (p *nativeProcess) nativeState(t *testing.T) int {
	t.Helper()
	p.send(t, "native-state")
	return eventValue[int](t, p.until(t, time.Second, "native-state").Value)
}

func (p *nativeProcess) awaitNative(t *testing.T, mask int, bound time.Duration) {
	t.Helper()
	end := time.Now().Add(bound)
	var got int
	for time.Now().Before(end) {
		got = p.nativeState(t)
		if got&mask == mask {
			return
		}
	}
	t.Fatalf("GTK view state = %d, expected mask %d", got, mask)
}

// closeFailed waits until the coordinator has aborted more than seen close
// attempts. Any wait may read the event first, so until counts it.
func (p *nativeProcess) closeFailed(t *testing.T, seen int, bound time.Duration) {
	t.Helper()
	end := time.Now().Add(bound)
	for p.seen["close-failed"] <= seen {
		p.until(t, time.Until(end), "close-failed")
	}
}

func (p *nativeProcess) cleanExit(t *testing.T, bound time.Duration) int {
	t.Helper()
	select {
	case <-p.exited:
		if p.waitErr != nil || p.cmd.ProcessState.ExitCode() != 0 {
			t.Fatalf("unclean exit: %v (%v)", p.waitErr, p.cmd.ProcessState)
		}
	case <-time.After(bound):
		t.Fatalf("native process %d did not exit within %s", p.cmd.Process.Pid, bound)
	}
	return p.cmd.ProcessState.ExitCode()
}

type wmKeys struct{ session dbus.BusObject }

func newWMKeys(t *testing.T) wmKeys {
	t.Helper()
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bus.Close() })
	const service = "org.gnome.Mutter.RemoteDesktop"
	root := bus.Object(service, "/org/gnome/Mutter/RemoteDesktop")
	var path dbus.ObjectPath
	if err := root.CallWithContext(t.Context(), service+".CreateSession", 0).Store(&path); err != nil {
		t.Fatal(err)
	}
	session := bus.Object(service, path)
	if err := session.CallWithContext(t.Context(), service+".Session.Start", 0).Err; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = session.CallWithContext(ctx, service+".Session.Stop", 0).Err
	})
	return wmKeys{session}
}

func (w wmKeys) key(t *testing.T, code uint32, down bool) {
	t.Helper()
	if err := w.session.CallWithContext(t.Context(),
		"org.gnome.Mutter.RemoteDesktop.Session.NotifyKeyboardKeycode", 0, code, down).Err; err != nil {
		t.Fatal(err)
	}
}

func (w wmKeys) press(t *testing.T, code uint32) {
	t.Helper()
	w.key(t, code, true)
	w.key(t, code, false)
}

func (w wmKeys) focus(t *testing.T, p *nativeProcess) {
	t.Helper()
	p.send(t, "focus")
	p.until(t, time.Second, "focus")
	if p.nativeState(t)&1 == 0 {
		// The private compositor starts with its pointer at the virtual monitor's
		// origin. Move to the middle of the mapped editor and give it seat focus.
		const iface = "org.gnome.Mutter.RemoteDesktop.Session."
		for _, step := range []struct {
			method string
			args   []any
		}{
			{"NotifyPointerMotionRelative", []any{-2000.0, -2000.0}},
			{"NotifyPointerMotionRelative", []any{640.0, 400.0}},
			{"NotifyPointerButton", []any{int32(272), true}},
			{"NotifyPointerButton", []any{int32(272), false}},
		} {
			if err := w.session.CallWithContext(t.Context(), iface+step.method, 0, step.args...).Err; err != nil {
				t.Fatal(err)
			}
		}
	}
	if p.nativeState(t)&1 == 0 {
		w.key(t, 56, true) // Alt-Tab in the private compositor selects the other editor.
		w.press(t, 15)
		w.key(t, 56, false)
	}
	p.awaitNative(t, 1, 3*time.Second)
}

func (w wmKeys) close(t *testing.T, p *nativeProcess, record bool) {
	t.Helper()
	if record {
		p.lifecycle(t, "wm-close-sent", p.cmd.Process.Pid, nil)
	}
	w.key(t, 56, true) // Alt
	w.press(t, 62)     // F4, handled by the compositor
	w.key(t, 56, false)
}

func (p *nativeProcess) inputCut(t *testing.T, keys wmKeys) string {
	t.Helper()
	keys.focus(t, p)
	p.send(t, "type")
	before := eventValue[string](t, p.until(t, time.Second, "type-ready").Value)
	keys.press(t, 45) // x
	after := eventValue[string](t, p.until(t, time.Second, "input-accepted").Value)
	if after != before+"x" {
		t.Fatalf("typed suffix was not accepted: %q → %q", before, after)
	}
	return after
}

func proofFile(t *testing.T) string {
	t.Helper()
	out := os.Getenv("WC1_NATIVE_OUT")
	if !filepath.IsAbs(out) {
		t.Fatal("WC1_NATIVE_OUT must be an absolute evidence directory")
	}
	return filepath.Join(out, "native-lifecycle.jsonl")
}

func assertProjection(t *testing.T, p *nativeProcess, want string) {
	t.Helper()
	awaitProjection(t, p, want, 0)
}

// awaitProjection fails unless the project file holds want within bound.
func awaitProjection(t *testing.T, p *nativeProcess, want string, bound time.Duration) {
	t.Helper()
	end := time.Now().Add(bound)
	for {
		b, err := os.ReadFile(filepath.Join(p.root, "docs/Project/main.typ"))
		if err == nil && string(b) == want {
			return
		}
		if !time.Now().Before(end) {
			t.Fatalf("project file holds %q, %v, want %q", b, err, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// renderers lists the WebKit web processes the app process started, through
// any sandbox launcher between them, as their /proc entries.
func (p *nativeProcess) renderers(t *testing.T) []int {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatal(err)
	}
	parents := map[int][]int{}
	for _, entry := range entries {
		var pid int
		if _, err := fmt.Sscan(entry.Name(), &pid); err != nil {
			continue
		}
		if _, parent, ok := procState(pid); ok {
			parents[parent] = append(parents[parent], pid)
		}
	}
	var found []int
	for queue := []int{p.cmd.Process.Pid}; len(queue) > 0; queue = queue[1:] {
		for _, child := range parents[queue[0]] {
			queue = append(queue, child)
			if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", child)); err == nil &&
				strings.HasPrefix(string(comm), "WebKitWebProc") {
				found = append(found, child)
			}
		}
	}
	return found
}

// procState reads a process's state letter and parent from /proc/<pid>/stat.
func procState(pid int) (state string, parent int, ok bool) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", 0, false
	}
	// The command name is in parentheses and may itself hold spaces or parentheses.
	fields := strings.Fields(string(b[strings.LastIndexByte(string(b), ')')+1:]))
	if len(fields) < 2 {
		return "", 0, false
	}
	if _, err := fmt.Sscan(fields[1], &parent); err != nil {
		return "", 0, false
	}
	return fields[0], parent, true
}

// renderersGone fails unless every listed web process has ended within bound
// of the app's exit: nothing the app gave up on may outlive it (W-6).
func renderersGone(t *testing.T, pids []int, bound time.Duration) {
	t.Helper()
	end := time.Now().Add(bound)
	for {
		var alive []string
		for _, pid := range pids {
			if state, _, ok := procState(pid); ok && state != "Z" {
				alive = append(alive, fmt.Sprintf("%d (%s)", pid, state))
			}
		}
		if len(alive) == 0 {
			return
		}
		if time.Now().After(end) {
			t.Fatalf("WebKit renderers outlived the app by %s: %v", bound, alive)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// signalRenderers stops or continues the app's web processes, as the person did
// in the R14 session (2026-11-23).
func signalRenderers(t *testing.T, pids []int, sig unix.Signal) {
	t.Helper()
	for _, pid := range pids {
		if err := unix.Kill(pid, sig); err != nil {
			t.Fatalf("signal %v to renderer %d: %v", sig, pid, err)
		}
	}
}

func TestWC1NativeClose(t *testing.T) {
	proof := proofFile(t)
	t.Run("SAV-03b", func(t *testing.T) {
		keys := newWMKeys(t)
		root := nativeFixture(t)
		makeNativeProject(t, root, nativeProjection)
		p := nativeProcessStart(t, root, "save", t.Name()[strings.LastIndex(t.Name(), "/")+1:], filepath.Base(root), proof)
		p.ready(t)
		p.seed(t, nativeProjection)
		want := p.inputCut(t, keys)
		keys.close(t, p, true)
		status := p.cleanExit(t, 10*time.Second)
		p.lifecycle(t, "process-exited", p.cmd.Process.Pid, &status)
		reopened := nativeProcessStart(t, root, "reopen", "", "", proof)
		p.lifecycle(t, "process-relaunched", reopened.cmd.Process.Pid, nil)
		got := reopened.ready(t)
		if got.Text != want || got.Unsaved != "Non enregistré" {
			t.Fatalf("reopened window shows %+v, expected unsaved %q", got, want)
		}
		assertProjection(t, p, nativeProjection)
		p.lifecycle(t, "project-reopened", reopened.cmd.Process.Pid, nil)
	})
	t.Run("SHEET-MU-01b", func(t *testing.T) {
		keys := newWMKeys(t)
		hostRoot, guestRoot := nativeFixture(t), nativeFixture(t)
		makeNativeProject(t, hostRoot, sheetProjection)
		makeNativeProject(t, guestRoot, nativeProjection)
		p := nativeProcessStart(t, hostRoot, "selection-hold", "SHEET-MU-01b", filepath.Base(hostRoot), proof)
		p.ready(t)
		p.seed(t, sheetProjection)
		q := nativeProcessStart(t, guestRoot, "guest", "", "", proof)
		q.ready(t)
		p.send(t, "invite")
		ticket := eventValue[string](t, p.until(t, 10*time.Second, "invite").Value)
		if _, got := q.joinRestarting(t, ticket); got.Text != sheetProjection {
			t.Fatalf("the joined guest shows %q, want the host's %q", got.Text, sheetProjection)
		}
		for _, person := range []*nativeProcess{p, q} {
			person.send(t, "sheet")
			person.until(t, 10*time.Second, "sheet")
		}
		keys.focus(t, p)
		p.send(t, "select")
		p.until(t, time.Second, "select")
		waitSelection(t, q, 4, 10*time.Second)
		p.lifecycle(t, "selection-visible", p.cmd.Process.Pid, nil)
		keys.close(t, p, true)
		p.until(t, 5*time.Second, "goodbye-handed-off")
		waitSelection(t, q, 0, 20*time.Second)
		p.lifecycle(t, "remote-selection-cleared", p.cmd.Process.Pid, nil)
		p.send(t, "release")
		status := p.cleanExit(t, 10*time.Second)
		p.lifecycle(t, "process-exited", p.cmd.Process.Pid, &status)
		reopened := nativeProcessStart(t, hostRoot, "reopen", "", "", proof)
		p.lifecycle(t, "process-relaunched", reopened.cmd.Process.Pid, nil)
		if got := reopened.ready(t); got.Text != sheetProjection {
			t.Fatalf("reopened sheet text = %q, want %q", got.Text, sheetProjection)
		}
		p.lifecycle(t, "project-reopened", reopened.cmd.Process.Pid, nil)
	})
	t.Run("PC1b-join", func(t *testing.T) {
		hostRoot, guestRoot := nativeFixture(t), nativeFixture(t)
		makeNativeProject(t, hostRoot, joinProjection)
		makeNativeProject(t, guestRoot, nativeProjection)
		host := nativeProcessStart(t, hostRoot, "host", "", "", proof)
		host.ready(t)
		host.seed(t, joinProjection)
		guest := nativeProcessStart(t, guestRoot, "guest", "PC1b-join", filepath.Base(guestRoot), proof)
		guest.ready(t)
		host.send(t, "invite")
		ticket := eventValue[string](t, host.until(t, 10*time.Second, "invite").Value)
		image, got := guest.joinRestarting(t, ticket)
		if got.Text != joinProjection || image.Standing != "member" || image.Members != 2 {
			t.Fatalf("J-2 the restarted guest shows %q as %s of %d members, want the host's %q as a member of 2", got.Text, image.Standing, image.Members, joinProjection)
		}
		guest.lifecycle(t, "project-joined", image.PID, nil)
		assertProjection(t, guest, nativeProjection) // J-6: the previous folder is never written
	})
	for _, mode := range []string{"Quit", "SIGTERM", "GNOMELogout"} {
		t.Run(mode, func(t *testing.T) {
			keys := newWMKeys(t)
			root := nativeFixture(t)
			makeNativeProject(t, root, nativeProjection)
			var logout *nativeSessionManager
			if mode == "GNOMELogout" {
				logout = newNativeSessionManager(t)
			}
			p := nativeProcessStart(t, root, "save", "", "", proof)
			p.ready(t)
			p.seed(t, nativeProjection)
			want := p.inputCut(t, keys)
			switch mode {
			case "Quit":
				p.send(t, "quit")
			case "SIGTERM":
				if err := p.cmd.Process.Signal(unix.SIGTERM); err != nil {
					t.Fatal(err)
				}
			case "GNOMELogout":
				logout.end(t)
			}
			p.cleanExit(t, 10*time.Second)
			if got := nativeProcessStart(t, root, "reopen", "", "", proof).ready(t); got.Text != want {
				t.Fatalf("%s recovered %q, expected %q", mode, got.Text, want)
			}
		})
	}
}

const logoutPath dbus.ObjectPath = "/org/gnome/SessionManager/ClientWC1"

type nativeSessionManager struct {
	bus        *dbus.Conn
	registered chan string
	answered   chan bool
}

func (s *nativeSessionManager) RegisterClient(client, _ string) (dbus.ObjectPath, *dbus.Error) {
	if client != appID {
		return "/org/gnome/SessionManager/OtherClient", nil
	}
	s.registered <- client
	return logoutPath, nil
}

func (s *nativeSessionManager) EndSessionResponse(ok bool, _ string) *dbus.Error {
	s.answered <- ok
	return nil
}

func newNativeSessionManager(t *testing.T) *nativeSessionManager {
	t.Helper()
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	status, err := bus.RequestName(gnomeManager, dbus.NameFlagDoNotQueue)
	if err != nil || status != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("private GNOME session manager: %v, %v", status, err)
	}
	s := &nativeSessionManager{bus: bus, registered: make(chan string, 8), answered: make(chan bool, 8)}
	if err := bus.Export(s, "/org/gnome/SessionManager", gnomeManager); err != nil {
		t.Fatal(err)
	}
	if err := bus.Export(s, logoutPath, gnomeClient); err != nil {
		t.Fatal(err)
	}
	return s
}

func (s *nativeSessionManager) end(t *testing.T) {
	t.Helper()
	select {
	case id := <-s.registered:
		if id != appID {
			t.Fatalf("unexpected logout client %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("app did not register for GNOME logout")
	}
	for _, step := range []struct {
		name  string
		bound time.Duration
	}{
		{name: "QueryEndSession", bound: time.Second},
		{name: "EndSession", bound: closeDeadline},
	} {
		if err := s.bus.Emit(logoutPath, gnomeClient+"."+step.name, uint32(0)); err != nil {
			t.Fatal(err)
		}
		select {
		case confirmed := <-s.answered:
			if !confirmed {
				t.Fatalf("healthy app objected to %s", step.name)
			}
		case <-time.After(step.bound):
			t.Fatalf("app did not answer %s within %s", step.name, step.bound)
		}
	}
}

func waitSelection(t *testing.T, person *nativeProcess, count int, bound time.Duration) {
	t.Helper()
	end := time.Now().Add(bound)
	for time.Now().Before(end) {
		person.send(t, "state")
		state := eventValue[pageWitness](t, person.until(t, time.Second, "state").Value)
		if state.Remote == count && state.Peers == 1 {
			return
		}
	}
	t.Fatalf("remote selection did not have %d cells on a healthy link", count)
}

func TestWC1NativeRecovery(t *testing.T) {
	cases := []struct {
		name, mode string
		stop       bool // SIGSTOP the renderer right after the keystroke, as in the R14 session
		retry      bool
	}{
		{name: "FullDiskRetry", mode: "disk", retry: true},
		{name: "HungStoreForce", mode: "store"},
		{name: "HungRendererForce", mode: "hang"},
		{name: "CrashedRendererForce", mode: "crash"},
		{name: "StoppedRendererRetry", mode: "auto", stop: true, retry: true},
		{name: "StoppedRendererForce", mode: "auto", stop: true},
	}
	proof := proofFile(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys := newWMKeys(t)
			root := nativeFixture(t)
			makeNativeProject(t, root, nativeProjection)
			p := nativeProcessStart(t, root, tc.mode, "", "", proof)
			p.ready(t)
			p.seed(t, nativeProjection)
			marker := filepath.Join(root, "fault-armed")
			if err := os.WriteFile(marker, []byte(tc.mode), 0o600); err != nil {
				t.Fatal(err)
			}
			want := p.inputCut(t, keys)
			var stopped []int
			switch {
			case tc.mode == "hang":
				p.send(t, "hang")
				p.until(t, time.Second, "hang-armed")
			case tc.mode == "crash":
				p.send(t, "terminate-renderer")
				p.until(t, time.Second, "renderer-crashed")
			case tc.stop:
				// Inside automatic saving's one-second pause: the file does not hold the key yet.
				if stopped = p.renderers(t); len(stopped) == 0 {
					t.Fatal("no WebKit renderer found under the app process")
				}
				signalRenderers(t, stopped, unix.SIGSTOP)
				t.Cleanup(func() {
					for _, pid := range stopped {
						if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil &&
							strings.HasPrefix(string(comm), "WebKitWebProc") {
							_ = unix.Kill(pid, unix.SIGKILL) // only the renderers this case stopped
						}
					}
				})
			}
			requestRecovery := func() {
				failures := p.seen["close-failed"]
				keys.close(t, p, false)
				if tc.mode == "crash" {
					p.awaitNative(t, 4|8, time.Second)
					if p.nativeState(t)&2 != 0 {
						t.Fatal("save progress was shown for a known renderer crash")
					}
					return
				}
				p.awaitNative(t, 2, time.Second)
				// W-4's deadline runs from the close request, so the abort is
				// awaited first; only the notice's presentation is then bounded.
				p.closeFailed(t, failures, closeDeadline+time.Second)
				p.awaitNative(t, 4|8, time.Second)
			}
			requestRecovery()
			keys.press(t, 1) // Escape dismisses; it carries no consent.
			p.awaitNative(t, 1, time.Second)
			requestRecovery()
			reopenMode := "reopen" // the Ctrl+S mode, whose file stays as it was
			if tc.mode == "auto" {
				reopenMode = "auto"
			}
			if tc.retry {
				switch {
				case tc.mode == "disk":
					if err := os.Remove(marker); err != nil {
						t.Fatal(err)
					}
				case tc.stop:
					signalRenderers(t, stopped, unix.SIGCONT)
				}
				keys.press(t, 28) // Enter activates the focused default Retry.
				p.cleanExit(t, 10*time.Second)
				if tc.mode == "auto" {
					// What the close kept is in the file too, not only in the store.
					assertProjection(t, p, want)
				}
			} else {
				keys.key(t, 42, true) // Shift-Tab moves from Retry to Force.
				keys.press(t, 15)
				keys.key(t, 42, false)
				p.awaitNative(t, 4|16, time.Second)
				renderers := p.renderers(t)
				if len(renderers) == 0 && tc.mode != "crash" {
					t.Fatal("no WebKit renderer found under the app process")
				}
				keys.press(t, 57) // Space activates the focused destructive action.
				p.cleanExit(t, 5*time.Second)
				renderersGone(t, renderers, time.Second)
				// Force keeps exactly what the store holds: the typed cut only if
				// the renderer stored it before it hung, stopped or crashed.
				if p.seen["cut-stored"] == 0 {
					want = nativeProjection
				} else {
					t.Logf("the %s renderer stored the typed cut before failing", tc.mode)
				}
			}
			reopen := nativeProcessStart(t, root, reopenMode, "", "", proof)
			if got := reopen.ready(t); got.Text != want {
				t.Fatalf("recovered %q, expected %q", got.Text, want)
			}
			if reopenMode == "auto" {
				// Opening in automatic saving writes the stored text to a file that lags it.
				awaitProjection(t, p, want, 2*time.Second)
			} else {
				assertProjection(t, p, nativeProjection)
			}
		})
	}
}
