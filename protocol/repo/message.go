// SPDX-License-Identifier: MPL-2.0

package repo

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

// HistoryMax is the fixed `max` of a history request: r = 4 MiB (§9.6).
const HistoryMax = 4194304

var (
	peerRE    = regexp.MustCompile(`^[!-~]{1,64}$`)
	docRE     = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{1,64}$`)
	urlRE     = regexp.MustCompile(`^automerge:[1-9A-HJ-NP-Za-km-z]{1,70}$`)
	sessionRE = regexp.MustCompile(`^[0-9a-z]{1,16}$`)
)

var knownTypes = map[string]bool{
	"sync":            true,
	"request":         true,
	"doc-unavailable": true,
	"ephemeral":       true,
	"history-request": true,
	"history-page":    true,
}

// Context is what the transport authenticated and the receiver admits.
type Context struct {
	From     string
	Me       string
	Admitted []string
}

// Message is a message that passed the profile and its type schema.
type Message struct {
	Type       string
	SenderID   string
	TargetID   string
	ProjectURL string
	DocumentID string
	Data       []byte
	Have       [][]byte
	Max        uint64
	Changes    int
	Last       bool
	SessionID  string
	Count      uint64
}

// Result reports either a message or an ignored unknown type (N14).
type Result struct {
	Message Message
	Ignored bool
}

// Decode applies the §9.2 profile, the authenticated-sender/receiver binding
// and the §9.3-§9.6 schema. Unknown types are ignored without error.
func Decode(b []byte, ctx Context) (Result, error) {
	value, err := DecodeValue(b)
	if err != nil {
		return Result{}, err
	}
	m, ok := value.(map[string]any)
	if !ok {
		return Result{}, errors.New("message is not a map")
	}
	if critical(m) {
		return Result{}, errors.New("unknown must-understand key")
	}
	rawType, _ := m["type"]
	t, _ := rawType.(string)
	sender, ok := m["senderId"].(string)
	if !ok || sender != ctx.From {
		return Result{}, errors.New("senderId is not the authenticated sender")
	}
	target, ok := m["targetId"].(string)
	if !ok || target != ctx.Me {
		return Result{}, errors.New("targetId is not the receiver")
	}
	out := Message{Type: t, SenderID: sender, TargetID: target}

	if t == "arrive" || t == "welcome" {
		return handshake(m, out, len(b))
	}
	if t == "remote-subscription-change" || t == "remote-heads-changed" {
		return Result{}, errors.New("type not admitted")
	}
	if !knownTypes[t] {
		return Result{Ignored: true}, nil
	}

	doc, ok := m["documentId"].(string)
	if !ok || !docRE.MatchString(doc) {
		return Result{}, errors.New("documentId")
	}
	if !slices.Contains(ctx.Admitted, doc) {
		return Result{}, errors.New("document not in the admitted set")
	}
	out.DocumentID = doc
	if t == "doc-unavailable" {
		return Result{Message: out}, nil
	}

	switch t {
	case "history-request":
		return decodeHistoryRequest(m, out)
	case "history-page":
		return decodeHistoryPage(m, out)
	case "sync", "request":
		return decodeSyncOrRequest(m, out)
	case "ephemeral":
		return decodeEphemeral(m, out)
	default:
		return Result{Message: out}, nil
	}
}

func decodeHistoryRequest(m map[string]any, out Message) (Result, error) {
	have, ok := m["have"].([]any)
	if !ok || len(have) > maxArray {
		return Result{}, errors.New("have")
	}
	out.Have = make([][]byte, 0, len(have))
	for _, h := range have {
		hash, ok := h.([]byte)
		if !ok || len(hash) != 32 {
			return Result{}, errors.New("have")
		}
		out.Have = append(out.Have, hash)
	}
	maxVal, ok := m["max"].(uint64)
	if !ok || maxVal != HistoryMax {
		return Result{}, errors.New("max must be r = 4194304")
	}
	out.Max = HistoryMax
	return Result{Message: out}, nil
}

func decodeHistoryPage(m map[string]any, out Message) (Result, error) {
	changes, ok := m["changes"].([]any)
	if !ok || len(changes) > maxArray {
		return Result{}, errors.New("changes")
	}
	for _, change := range changes {
		raw, ok := change.([]byte)
		if !ok || len(raw) < 1 {
			return Result{}, errors.New("changes")
		}
	}
	last, ok := m["last"].(bool)
	if !ok {
		return Result{}, errors.New("last")
	}
	if len(changes) == 0 && !last {
		return Result{}, errors.New("empty page must be the last (terminal answer)")
	}
	out.Changes = len(changes)
	out.Last = last
	return Result{Message: out}, nil
}

func decodeSyncOrRequest(m map[string]any, out Message) (Result, error) {
	data, ok := m["data"].([]byte)
	if !ok {
		return Result{}, errors.New("data is not a byte string")
	}
	out.Data = data
	return Result{Message: out}, nil
}

func decodeEphemeral(m map[string]any, out Message) (Result, error) {
	data, ok := m["data"].([]byte)
	if !ok {
		return Result{}, errors.New("data is not a byte string")
	}
	out.Data = data
	session, ok := m["sessionId"].(string)
	if !ok || !sessionRE.MatchString(session) {
		return Result{}, errors.New("sessionId")
	}
	count, ok := m["count"].(uint64)
	if !ok || count < 1 {
		return Result{}, errors.New("count")
	}
	if len(data) > 1024 {
		return Result{}, errors.New("ephemeral data > 1024 bytes")
	}
	envelope, err := DecodeValue(data)
	if err != nil {
		return Result{}, err
	}
	e, ok := envelope.(map[string]any)
	if !ok {
		return Result{}, errors.New("ephemeral payload")
	}
	if critical(e) {
		return Result{}, errors.New("unknown must-understand key (inner payload)")
	}
	if len(e) != 1 {
		return Result{}, errors.New("envelope is a closed map with one key")
	}
	if raw, ok := e["__present"]; ok {
		p, ok := raw.(map[string]any)
		if !ok || !present(p) {
			return Result{}, errors.New("__present")
		}
	} else {
		p, ok := e["__presence"].(map[string]any)
		if !ok || !presence(p) {
			return Result{}, errors.New("__presence")
		}
	}
	out.SessionID = session
	out.Count = count
	return Result{Message: out}, nil
}

func handshake(m map[string]any, out Message, size int) (Result, error) {
	if size > 1024 {
		return Result{}, errors.New("handshake > 1024 bytes")
	}
	if metadata, ok := m["peerMetadata"]; ok {
		if _, ok := metadata.(map[string]any); !ok {
			return Result{}, errors.New("peerMetadata")
		}
	}
	if raw, ok := m["projectUrl"]; ok {
		url, ok := raw.(string)
		if !ok || !urlRE.MatchString(url) || len(url) > 80 {
			return Result{}, errors.New("projectUrl")
		}
		out.ProjectURL = url
	}
	return Result{Message: out}, nil
}

func critical(v any) bool {
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			if critical(item) {
				return true
			}
		}
	case map[string]any:
		for key, item := range x {
			if strings.HasPrefix(key, "!") || critical(item) {
				return true
			}
		}
	}
	return false
}

func channel(key string, value any) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	limit := 64
	if key == "name" {
		limit = 128
	}
	switch key {
	case "name", "anchor", "head":
		return len(s) <= limit
	default:
		return false
	}
}

func presence(p map[string]any) bool {
	switch t, _ := p["type"].(string); t {
	case "heartbeat", "goodbye":
		return true
	case "update":
		key, _ := p["channel"].(string)
		return channel(key, p["value"])
	case "snapshot":
		state, ok := p["state"].(map[string]any)
		if !ok || len(state) > 3 {
			return false
		}
		for key, value := range state {
			if !channel(key, value) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func present(m map[string]any) bool {
	switch t, _ := m["t"].(string); t {
	case "state":
		return closed(m, "epoch", "screen", "seq", "slide", "step", "t") &&
			epoch(m["epoch"]) && uintAtMost(m["seq"], maxSafe) &&
			uintAtMost(m["slide"], 100000) && uintAtMost(m["step"], 100000) &&
			nonZero(m["slide"]) && screen(m["screen"])
	case "laser":
		if !closed(m, "epoch", "t", "x", "y") || !epoch(m["epoch"]) {
			return false
		}
		if m["x"] == nil && m["y"] == nil {
			return true
		}
		return uintAtMost(m["x"], 10000) && uintAtMost(m["y"], 10000)
	case "take", "end":
		return closed(m, "epoch", "t") && epoch(m["epoch"])
	case "hello", "ask":
		return closed(m, "t")
	case "give":
		return closed(m, "epoch", "t", "to") && epoch(m["epoch"]) && peer(m["to"])
	case "deny":
		return closed(m, "t", "to") && peer(m["to"])
	default:
		return false
	}
}

func closed(m map[string]any, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			return false
		}
	}
	return true
}

func epoch(v any) bool { return uintAtMost(v, 2147483648) }

func uintAtMost(v any, maxVal uint64) bool {
	n, ok := v.(uint64)
	return ok && n <= maxVal
}

func nonZero(v any) bool {
	n, ok := v.(uint64)
	return ok && n >= 1
}

func screen(v any) bool {
	s, ok := v.(string)
	return ok && (s == "show" || s == "black" || s == "white")
}

func peer(v any) bool {
	s, ok := v.(string)
	return ok && peerRE.MatchString(s)
}
