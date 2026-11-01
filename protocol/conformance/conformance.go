// SPDX-License-Identifier: MPL-2.0

// Package conformance runs the shared corpus against reference or third-party implementations.
package conformance

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/oioio-space/inkwell/protocol/vectors"
)

// Vector is a corpus case. Fields retains the complete schema for implementation-specific checks.
type Vector struct {
	// File is the corpus filename.
	File string
	// ID identifies the case within its file.
	ID string
	// Decoder names the format or state machine under test.
	Decoder string
	// Status is normative or proposed.
	Status string
	// Enforce is the first required migration step, when specified.
	Enforce string
	// Verdict is the expected accept or reject result.
	Verdict string
	// Fields retains every original JSON field.
	Fields map[string]json.RawMessage
}

// Decode reads a field from the vector without losing future schema extensions.
func (v Vector) Decode(name string, dst any) error { return json.Unmarshal(v.Fields[name], dst) }

// Required reports whether the case is enforced at the supplied migration step.
func (v Vector) Required(step int) bool {
	if v.Enforce != "" {
		n, err := strconv.Atoi(strings.TrimPrefix(v.Enforce, "step-"))
		return err == nil && n <= step
	}
	return v.Status == "normative"
}

// Result reports one case, including failures that are only informational at this step.
type Result struct {
	// Vector is the original case.
	Vector Vector
	// Required reports whether this case gates the selected step.
	Required bool
	// Passed reports whether the implementation matched the case.
	Passed bool
	// Err explains a mismatch or an unsupported future format.
	Err error
}

// Check executes a vector and returns nil only when its verdict and expectations match.
// Rejections expected by the corpus are successful checks, not returned errors.
type Check func(Vector) error

// Run visits every embedded vector in filename order. A nil check uses the Go
// reference implementation. Future cases are reported but do not become required early.
func Run(step int, check Check) ([]Result, error) {
	if check == nil {
		check = Reference
	}
	names, err := fs.Glob(vectors.FS, "*.json")
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, name := range names {
		data, err := vectors.FS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var file struct {
			Vectors []map[string]json.RawMessage `json:"vectors"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, fields := range file.Vectors {
			v := Vector{File: name, Fields: fields}
			for k, dst := range map[string]*string{"id": &v.ID, "decoder": &v.Decoder, "status": &v.Status, "enforce": &v.Enforce, "verdict": &v.Verdict} {
				if raw, ok := fields[k]; ok {
					if err := json.Unmarshal(raw, dst); err != nil {
						return nil, err
					}
				}
			}
			err := check(v)
			results = append(results, Result{Vector: v, Required: v.Required(step), Passed: err == nil, Err: err})
		}
	}
	return results, nil
}
