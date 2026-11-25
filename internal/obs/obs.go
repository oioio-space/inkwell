// Package obs keeps the person's local statistics and debug state (plan
// docs/plans/2026-11-24-obs1-observability.md): a metric is declared once with
// a [Descriptor], its values are numbers that pass the value rule V, they are
// summed per local day in files under INKWELL_HOME/stats, and nothing ever
// leaves the device. The package reaches no network code.
//
// Go declares its own instruments on a [Meter]; the webview declares its
// metrics through [Store.Declare] and sends its deltas through [Store.Add],
// both checked here and never trusted.
package obs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
)

// The kinds of a metric: what is counted, and so how a day sums it.
const (
	// KindCounter counts events: a day holds their sum.
	KindCounter = "counter"
	// KindDuration measures time in milliseconds: a day holds the sum and
	// the number of measures.
	KindDuration = "duration"
	// KindVolume counts an amount (bytes, words): a day holds the sum.
	KindVolume = "volume"
)

// The scopes of a metric.
const (
	// ScopeApp is a metric of the application as a whole.
	ScopeApp = "app"
	// ScopeDocument is a metric of one document, stored under its pseudonym.
	ScopeDocument = "document"
)

// units gives the units each kind may be measured in.
var units = map[string][]string{
	KindCounter:  {"count"},
	KindDuration: {"ms"},
	KindVolume:   {"bytes", "words"},
}

// Bounds of what a declaration and a batch may hold.
const (
	maxID          = 64
	maxLabel       = 128
	maxValues      = 64
	maxValue       = 32
	maxDescriptors = 256
	maxBatch       = 1024
	// maxSafe is the largest integer a webview number holds exactly.
	maxSafe = 1<<53 - 1
)

var (
	metricID = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$`)
	labelKey = regexp.MustCompile(`^[a-z][A-Za-z0-9]*(\.[A-Za-z0-9]+)*$`)
	enumWord = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	// docID is the shape of an Automerge document id (bs58check): it is only
	// keyed into a pseudonym, never stored.
	docID = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{16,64}$`)
)

// Descriptor declares a metric (rule M, plan §2). Every field is data: the
// label is an i18n key, the values an enum, the bound a number.
type Descriptor struct {
	// ID is `<extension>.<name>`, unique among all metrics.
	ID string `json:"id"`
	// Kind is [KindCounter], [KindDuration] or [KindVolume].
	Kind string `json:"kind"`
	// Unit is consistent with the kind: count, ms, bytes or words.
	Unit string `json:"unit"`
	// Scope is [ScopeApp] or [ScopeDocument].
	Scope string `json:"scope"`
	// Label is the i18n key of the metric's name.
	Label string `json:"label"`
	// Max is the most one delta may add, at least 1.
	Max uint64 `json:"max"`
	// Values, if any, is the closed set of values a delta may be split by
	// (a failure's class, for example).
	Values []string `json:"values,omitempty"`
}

// ErrRefused wraps every refusal of a declaration or a batch under rule V.
var ErrRefused = errors.New("obs: refused")

func refuse(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrRefused}, args...)...)
}

// Validate reports whether d is a well-formed declaration.
func (d Descriptor) Validate() error {
	switch {
	case len(d.ID) > maxID || !metricID.MatchString(d.ID):
		return refuse("metric id %q", d.ID)
	case !slices.Contains(units[d.Kind], d.Unit):
		return refuse("%s: kind %q with unit %q", d.ID, d.Kind, d.Unit)
	case d.Scope != ScopeApp && d.Scope != ScopeDocument:
		return refuse("%s: scope %q", d.ID, d.Scope)
	case len(d.Label) > maxLabel || !labelKey.MatchString(d.Label):
		return refuse("%s: label is not an i18n key", d.ID)
	case d.Max < 1 || d.Max > maxSafe:
		return refuse("%s: bound %d", d.ID, d.Max)
	case len(d.Values) > maxValues:
		return refuse("%s: %d values", d.ID, len(d.Values))
	}
	for i, v := range d.Values {
		if len(v) > maxValue || !enumWord.MatchString(v) || slices.Contains(d.Values[:i], v) {
			return refuse("%s: value %d is not an enum word", d.ID, i)
		}
	}
	return nil
}

// equal reports whether d and e declare the same metric.
func (d Descriptor) equal(e Descriptor) bool {
	return d.ID == e.ID && d.Kind == e.Kind && d.Unit == e.Unit && d.Scope == e.Scope &&
		d.Label == e.Label && d.Max == e.Max && slices.Equal(d.Values, e.Values)
}

// Delta is what one flush adds to one metric: the webview's batches are made
// of them.
type Delta struct {
	// M is the id of a declared metric.
	M string `json:"m"`
	// Doc is the Automerge document id of a document metric, empty for an
	// application one. It is turned into a pseudonym at once.
	Doc string `json:"doc,omitempty"`
	// By is one of the metric's declared values, if it has some.
	By string `json:"by,omitempty"`
	// V is added to the day's sum: an integer from 0 to the metric's bound.
	V float64 `json:"v"`
	// N is the number of measures in V, for a duration; 0 means 1.
	N float64 `json:"n,omitempty"`
}

// count reports whether x is an integer from 0 to limit (rule V for a metric
// value: finite, not negative, bounded).
func count(x float64, limit uint64) bool {
	return !math.IsNaN(x) && x >= 0 && x <= float64(limit) && x == math.Trunc(x)
}

// check applies rule V to one delta of a metric described by d.
func (x Delta) check(d Descriptor) error {
	switch {
	case !count(x.V, d.Max):
		return refuse("%s: value out of rule V", d.ID)
	case !count(x.N, d.Max) || (x.N != 0 && d.Kind != KindDuration):
		return refuse("%s: measure count out of rule V", d.ID)
	case (d.Scope == ScopeDocument) != (x.Doc != ""):
		return refuse("%s: document key does not match the scope", d.ID)
	case x.Doc != "" && !docID.MatchString(x.Doc):
		return refuse("%s: not a document id", d.ID)
	case x.By != "" && !slices.Contains(d.Values, x.By):
		return refuse("%s: value is not one of the declared ones", d.ID)
	}
	return nil
}

// pseudonym is HMAC-SHA-256 of id under key, truncated to 8 bytes, in hex: the
// local pseudonym of spec O-2, a document's under the stats key, a peer's
// under the session's salt.
func pseudonym(key []byte, id string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}
