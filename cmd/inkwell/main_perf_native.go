//go:build perf_native && !production && linux && !server

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oioio-space/inkwell/internal/nativeperf"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func main() {
	output := "gates/L3/PERF-TYPE.json"
	for _, arg := range os.Args[1:] {
		if value, ok := strings.CutPrefix(arg, "--native-perf-out="); ok {
			output = value
		}
	}
	for _, arg := range os.Args[1:] {
		if directory, ok := strings.CutPrefix(arg, "--native-perf-merge="); ok {
			report := nativeperf.Merge(directory, output)
			if report.Status != "pass" {
				os.Exit(1)
			}
			return
		}
		if reasons, ok := strings.CutPrefix(arg, "--native-perf-invalid="); ok {
			// Written by the driver after three environment faults: invalid, never a pass.
			if err := nativeperf.WriteReport(output, nativeperf.Invalid(nativeperf.InvalidReasons(reasons))); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(1)
		}
		if reason, ok := strings.CutPrefix(arg, "--native-perf-unavailable="); ok {
			report := nativeperf.Validate(nil)
			report.Reasons = append(report.Reasons, reason)
			if err := nativeperf.WriteReport(output, report); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(1)
		}
	}
	if os.Getenv("INKWELL_NATIVE_PERF_RUN") != "1" {
		report := nativeperf.Validate(nil)
		report.Reasons = append(report.Reasons, "native collection requires explicitly authorized external session INKWELL_NATIVE_PERF_RUN=1")
		_ = nativeperf.WriteReport(output, report)
		os.Exit(1)
	}
	if os.Getenv("INKWELL_NATIVE_PERF_PHASE") == typingCheckPhase {
		runTypingCheck()
		return
	}
	var success atomic.Bool
	targetPath := os.Getenv("INKWELL_NATIVE_PERF_TARGET")
	run(func(app *application.App, window *application.WebviewWindow) {
		host := func() (nativeperf.Host, bool, bool) {
			webkit, gtk := nativeEngineVersions(app.Env.Info().PlatformInfo)
			target, _ := nativeperf.ReadTarget(targetPath, os.Getpid())
			return nativeperf.Host{Engine: "WebKitGTK", Native: true, GOOS: "linux", WebKitVersion: webkit, GTKVersion: gtk, Backend: target.Backend, XID: target.XID, RefreshHz: target.RefreshHz, InputPermissionVerified: target.PermissionVerified}, window.IsVisible(), window.IsFocused()
		}
		var markerMu sync.Mutex
		marker := func(value any) {
			markerMu.Lock()
			defer markerMu.Unlock()
			data, err := json.Marshal(value)
			if err == nil {
				fmt.Println("INKWELL_NATIVE_PERF " + string(data))
			}
		}
		raw := os.Getenv("INKWELL_NATIVE_PERF_OUT")
		if raw == "" {
			raw = filepath.Join("gates", "L3", "native-raw", "run.json")
		}
		config := nativeperf.Config{Phase: os.Getenv("INKWELL_NATIVE_PERF_PHASE"), Count: perfInt("INKWELL_NATIVE_PERF_COUNT"), Index: perfInt("INKWELL_NATIVE_PERF_INDEX"), Replicates: perfInt("INKWELL_NATIVE_PERF_REPLICATES"), RunID: os.Getenv("INKWELL_NATIVE_PERF_RUN_ID"), Output: raw, Host: host, Marker: marker, Done: func(ok bool) { success.Store(ok); app.Quit() }}
		config.Prepare = func() error {
			return prepareNativeWindow(window, marker, config.RunID, targetPath)
		}
		service, err := nativeperf.New(config)
		if err != nil {
			report := nativeperf.Validate(nil)
			report.Reasons = append(report.Reasons, err.Error())
			_ = nativeperf.WriteJSON(raw, report)
			app.Quit()
			return
		}
		app.HandleStream(nativeperf.StreamName, func(c *application.StreamConn) { service.Serve(c) })
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { window.Show(); window.Focus() })
		collectionTimeout := 90 * time.Second
		if config.Phase == "warm" {
			// The 90 s base covers the first 30 samples; every further trial (samples and NULL keystrokes) gets 3 s.
			collectionTimeout += time.Duration(max(config.Count-30, 0)+nativeperf.NullCount) * 3 * time.Second
		}
		time.AfterFunc(collectionTimeout, func() { service.Expire("native webview did not complete evidence collection") })
	})
	if !success.Load() {
		os.Exit(1)
	}
}

func prepareNativeWindow(window *application.WebviewWindow, marker func(any), runID, targetPath string) error {
	if os.Getenv("GDK_BACKEND") != "x11" {
		return fmt.Errorf("native target must use GDK_BACKEND=x11 for XTest")
	}
	marker(map[string]any{"type": "activate", "runId": runID, "pid": os.Getpid(), "backend": "x11"})
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		visible := window.IsVisible()
		focused := window.IsFocused()
		_, targetErr := nativeperf.ReadTarget(targetPath, os.Getpid())
		if visible && focused && targetErr == nil {
			fmt.Fprintln(os.Stderr, "native activation: visible and focused")
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("visible=%t focused=%t after X11 activation", window.IsVisible(), window.IsFocused())
}

func perfInt(name string) int { n, _ := strconv.Atoi(os.Getenv(name)); return n }
