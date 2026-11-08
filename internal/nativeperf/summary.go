package nativeperf

import (
	"reflect"
	"strings"
)

type (
	// Evidence names one raw evidence file and the SHA-256 of the bytes the
	// verdict was computed from.
	Evidence struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	// Summary is the compact PERF-TYPE verdict that the gate manifest embeds:
	// status, reasons, thresholds and every p95. The full diagnostics are in
	// the Detail file and the raw runs are referenced by path and SHA-256.
	Summary struct {
		ID                     string              `json:"id"`
		Status                 string              `json:"status"`
		Measurement            string              `json:"measurement"`
		Reasons                []string            `json:"reasons"`
		Thresholds             map[string]float64  `json:"thresholds"`
		ObjectiveWarmHandlerMS float64             `json:"objective_warm_handler_ms"`
		ObjectiveMet           bool                `json:"objective_met"`
		P95MS                  map[string]*float64 `json:"p95_ms"`
		MaxGapsMS              map[string]*float64 `json:"max_gaps_ms"`
		DroppedFrames          int                 `json:"dropped_frames"`
		Controls               []ControlValidation `json:"controls"`
		Overhead               *Overhead           `json:"instrument_overhead,omitempty"`
		MissedRefresh          *MissedRefresh      `json:"missed_refresh,omitempty"`
		Detail                 string              `json:"detail"`
		RawEvidence            []Evidence          `json:"raw_evidence"`
	}
)

// Summary keeps the verdict of r and, by JSON name, the p95 of every Metric
// field and every maximum gap, so a new metric cannot be left out.
func (r Report) Summary(detail string) Summary {
	s := Summary{
		ID: r.ID, Status: r.Status, Measurement: r.Measurement, Reasons: r.Reasons, Thresholds: r.Thresholds,
		ObjectiveWarmHandlerMS: r.ObjectiveWarmHandlerMS, ObjectiveMet: r.ObjectiveMet,
		P95MS: map[string]*float64{}, MaxGapsMS: map[string]*float64{}, DroppedFrames: r.DroppedFrames,
		Controls: r.Controls, Overhead: r.Overhead, MissedRefresh: r.MissedRefresh,
		Detail: detail, RawEvidence: r.RawEvidence,
	}
	for field, value := range reflect.ValueOf(r).Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		switch v := value.Interface().(type) {
		case Metric:
			s.P95MS[name] = v.P95MS
		case *float64:
			s.MaxGapsMS[name] = v
		}
	}
	return s
}

// DetailPath is where WriteReport puts the full diagnostics of the verdict at out.
func DetailPath(out string) string { return strings.TrimSuffix(out, ".json") + "-detail.json" }

// WriteReport writes the full report to DetailPath(out) and its compact
// Summary to out.
func WriteReport(out string, r Report) error {
	detail := DetailPath(out)
	if err := WriteJSON(detail, r); err != nil {
		return err
	}
	return WriteJSON(out, r.Summary(detail))
}
