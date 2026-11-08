package nativeperf

import (
	"encoding/json"
	"fmt"
	"os"
)

// Target is the external driver's OS-verified X11 window and GdkMonitor.
// It is written after PID/XID/focus checks into the fresh process profile.
type Target struct {
	PermissionVerified bool               `json:"permissionVerified"`
	PID                int                `json:"pid"`
	XID                uint64             `json:"xid"`
	Backend            string             `json:"backend"`
	RefreshHz          float64            `json:"refreshHz"`
	Monitor            map[string]float64 `json:"monitor"`
}

// ReadTarget loads the Target JSON written by the external driver at path and
// checks it describes the measured X11 window of process pid, with an observed
// GdkMonitor refresh rate.
func ReadTarget(path string, pid int) (Target, error) {
	var t Target
	b, err := os.ReadFile(path) // #nosec G304 -- perf_native harness: path written by the local driver, never peer input
	if err != nil {
		return t, err
	}
	if err = json.Unmarshal(b, &t); err != nil {
		return t, err
	}
	if t.PID != pid || t.XID == 0 || t.Backend != "x11" || !finite(t.RefreshHz) || t.RefreshHz <= 0 {
		return t, fmt.Errorf("injection target is not the measured X11 window with an observed GdkMonitor refresh")
	}
	return t, nil
}
