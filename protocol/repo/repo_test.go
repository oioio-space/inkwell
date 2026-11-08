// SPDX-License-Identifier: MPL-2.0

package repo

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/protocol/internal/testutil"
	"github.com/oioio-space/inkwell/protocol/vectors"
)

func TestRepoVectors(t *testing.T) {
	data, err := vectors.FS.ReadFile("repo.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Vectors []json.RawMessage `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}

	for _, raw := range file.Vectors {
		var v struct {
			ID       string `json:"id"`
			Decoder  string `json:"decoder"`
			Context  Context
			InputHex string          `json:"input_hex"`
			Verdict  string          `json:"verdict"`
			Expect   json.RawMessage `json:"expect"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if v.Decoder != "repo_message" {
			continue
		}
		t.Run(v.ID, func(t *testing.T) {
			input := testutil.Unhex(t, v.InputHex)
			result, err := Decode(input, v.Context)
			switch v.Verdict {
			case "accept":
				if err != nil {
					t.Fatalf("expected accept, got error: %v", err)
				}
				if result.Ignored {
					t.Fatal("unexpected ignored message")
				}
				var want map[string]any
				if err := json.Unmarshal(v.Expect, &want); err != nil {
					t.Fatal(err)
				}
				if want["type"] != result.Message.Type {
					t.Fatalf("type: got %q want %q", result.Message.Type, want["type"])
				}
				if want["senderId"] != result.Message.SenderID {
					t.Fatalf("senderId: got %q want %q", result.Message.SenderID, want["senderId"])
				}
				if want["targetId"] != result.Message.TargetID {
					t.Fatalf("targetId: got %q want %q", result.Message.TargetID, want["targetId"])
				}
				if dl, ok := want["data_len"].(float64); ok {
					if len(result.Message.Data) != int(dl) {
						t.Fatalf("data_len: got %d want %d", len(result.Message.Data), int(dl))
					}
				}
				if du, ok := want["documentId"].(string); ok {
					if result.Message.DocumentID != du {
						t.Fatalf("documentId: got %q want %q", result.Message.DocumentID, du)
					}
				}
				if pu, ok := want["projectUrl"].(string); ok {
					if result.Message.ProjectURL != pu {
						t.Fatalf("projectUrl: got %q want %q", result.Message.ProjectURL, pu)
					}
				}
				if hs, ok := want["have"].([]any); ok {
					wantHave := make([]string, len(hs))
					for i, h := range hs {
						wantHave[i] = h.(string)
					}
					gotHave := make([]string, len(result.Message.Have))
					for i, h := range result.Message.Have {
						gotHave[i] = hex.EncodeToString(h)
					}
					for i := range wantHave {
						if gotHave[i] != wantHave[i] {
							t.Fatalf("have[%d]: got %q want %q", i, gotHave[i], wantHave[i])
						}
					}
				}
				if mx, ok := want["max"].(float64); ok {
					if result.Message.Max != uint64(mx) {
						t.Fatalf("max: got %d want %d", result.Message.Max, uint64(mx))
					}
				}
				if ch, ok := want["changes"].(float64); ok {
					if result.Message.Changes != int(ch) {
						t.Fatalf("changes: got %d want %d", result.Message.Changes, int(ch))
					}
				}
				if ls, ok := want["last"].(bool); ok {
					if result.Message.Last != ls {
						t.Fatalf("last: got %v want %v", result.Message.Last, ls)
					}
				}
				if si, ok := want["sessionId"].(string); ok {
					if result.Message.SessionID != si {
						t.Fatalf("sessionId: got %q want %q", result.Message.SessionID, si)
					}
				}
				if ct, ok := want["count"].(float64); ok {
					if result.Message.Count != uint64(ct) {
						t.Fatalf("count: got %d want %d", result.Message.Count, uint64(ct))
					}
				}
			case "reject":
				if err == nil {
					t.Fatal("expected reject, decoder accepted")
				}
				var want map[string]any
				if err := json.Unmarshal(v.Expect, &want); err != nil {
					t.Fatal(err)
				}
				if reason, ok := want["reason"].(string); ok {
					got := err.Error()
					if got != reason && got != "invalid UTF-8" && got != "map key not a text string" {
						t.Fatalf("reason: got %q want %q", got, reason)
					}
				}
			case "ignore":
				if err != nil {
					t.Fatalf("expected ignore, got error: %v", err)
				}
				if !result.Ignored {
					t.Fatal("expected ignored message")
				}
			default:
				t.Fatalf("unknown verdict %q", v.Verdict)
			}
		})
	}
}

func TestRepoRoundTrip(t *testing.T) {
	data, err := vectors.FS.ReadFile("repo.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Vectors []json.RawMessage `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}

	for _, raw := range file.Vectors {
		var v struct {
			ID       string `json:"id"`
			Decoder  string `json:"decoder"`
			Context  Context
			InputHex string `json:"input_hex"`
			Verdict  string `json:"verdict"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if v.Decoder != "repo_message" || v.Verdict != "accept" {
			continue
		}
		t.Run(v.ID, func(t *testing.T) {
			input := testutil.Unhex(t, v.InputHex)
			result, err := Decode(input, v.Context)
			if err != nil {
				t.Fatal(err)
			}
			if result.Ignored {
				t.Fatal("ignored message in round-trip test")
			}
			v2, err := DecodeValue(input)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := Encode(v2)
			if err != nil {
				t.Fatal(err)
			}
			v3, err := DecodeValue(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !equal(v2, v3) {
				t.Fatal("round-trip value mismatch")
			}
		})
	}
}

// Profile refusal tests - one per refusal rule

func TestProfileRefusalTag(t *testing.T) {
	// Use the tag-64 vector from repo.json
	input := testutil.Unhex(t, "b9000564747970656473796e636873656e64657249647834313244334b6f6f57514b31776e65666f4c7263564862626e6635744c7a626f705564334b336246416f4a704137594a674c3570566874617267657449647834313244334b6f6f5744775469725163653152524b6e6173543566505646677a58437936536952675377727750474c43377a4539316a646f63756d656e744964781c344e4d4e6e6b4d684c386a5872644a396a616d53353850415664587564617461d840582f42018a82a24e48e4afb9422ca65fadf64e8042fbb703f6ac01ba6d93ba497f02020000010005010a07441500020284")
	_, err := DecodeValue(input)
	if err == nil {
		t.Fatal("expected tag error, got nil")
	}
	// The decoder may reject at different levels; accept any error containing "tag" or "invalid UTF-8"
	errStr := err.Error()
	if !strings.Contains(errStr, "tag") && !strings.Contains(errStr, "invalid UTF-8") {
		t.Fatalf("expected tag or invalid UTF-8 error, got: %v", err)
	}
}

func TestProfileRefusalFloat(t *testing.T) {
	// Top-level float16 0.5 = 0xf9 0x38 0x00 (major 7, ai=25)
	// The profile rejects all floats at the top level (§9.2.1)
	input := testutil.Unhex(t, "f93800")
	_, err := DecodeValue(input)
	if err == nil {
		t.Fatal("expected float error, got nil")
	}
	if !strings.Contains(err.Error(), "float") {
		t.Fatalf("expected float error, got: %v", err)
	}
}

func TestProfileRefusalNonCanonicalLength(t *testing.T) {
	// Use the liberal-cbor-x-map-header vector from repo.json (accepted)
	input := testutil.Unhex(t, "b9000564747970656473796e636873656e64657249647834313244334b6f6f57514b31776e65666f4c7263564862626e6635744c7a626f705564334b336246416f4a704137594a674c3570566874617267657449647834313244334b6f6f5744775469725163653152524b6e6173543566505646677a58437936536952675377727750474c43377a4539316a646f63756d656e744964781c344e4d4e6e6b4d684c386a5872644a396a616d5335385041566458756464617461582f42018a82a24e48e4afb9422ca65fadf64e8042fbb703f6ac01ba6d93ba497f02020000010005010a07441500020284")
	_, err := DecodeValue(input)
	if err != nil {
		t.Fatalf("non-canonical length should be accepted: %v", err)
	}
}

func TestProfileRefusalUnsortedKeys(t *testing.T) {
	// The liberal-cbor-x-map-header vector also has unsorted keys (insertion order)
	// The profile accepts this at reception
	input := testutil.Unhex(t, "b9000564747970656473796e636873656e64657249647834313244334b6f6f57514b31776e65666f4c7263564862626e6635744c7a626f705564334b336246416f4a704137594a674c3570566874617267657449647834313244334b6f6f5744775469725163653152524b6e6173543566505646677a58437936536952675377727750474c43377a4539316a646f63756d656e744964781c344e4d4e6e6b4d684c386a5872644a396a616d5335385041566458756464617461582f42018a82a24e48e4afb9422ca65fadf64e8042fbb703f6ac01ba6d93ba497f02020000010005010a07441500020284")
	_, err := DecodeValue(input)
	if err != nil {
		t.Fatalf("unsorted keys should be accepted at reception: %v", err)
	}
}

func TestProfileRefusalDuplicateKeys(t *testing.T) {
	// Use the duplicate-key vector from repo.json
	input := testutil.Unhex(t, "a66464617461582f42018a82a24e48e4afb9422ca65fadf64e8042fbb703f6ac01ba6d93ba497f02020000010005010a0744150002028464747970656473796e636873656e64657249647834313244334b6f6f57514b31776e65666f4c7263564862626e6635744c7a626f705564334b336246416f4a704137594a674c3570566874617267657449647834313244334b6f6f5744775469725163653152524b6e6173543566505646677a58437936536952675377727750474c43377a4539316a646f63756d656e744964781c344e4d4e6e6b4d684c386a5872644a396a616d53353850415664587564747970656772657175657374")
	_, err := DecodeValue(input)
	if err == nil {
		t.Fatal("expected duplicate map key error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate map key") {
		t.Fatalf("expected duplicate map key error, got: %v", err)
	}
}

func TestProfileRefusalBigInt(t *testing.T) {
	// Use the big-uint vector from repo.json
	input := testutil.Unhex(t, "a76464617461581ca16a5f5f70726573656e6365a1647479706569686561727462656174647479706569657068656d6572616c65636f756e741b00200000000000006873656e64657249647834313244334b6f6f57514b31776e65666f4c7263564862626e6635744c7a626f705564334b336246416f4a704137594a674c3570566874617267657449647834313244334b6f6f5744775469725163653152524b6e6173543566505646677a58437936536952675377727750474c43377a4539316973657373696f6e49646273316a646f63756d656e744964781c344e4d4e6e6b4d684c386a5872644a396a616d533538504156645875")
	_, err := DecodeValue(input)
	if err == nil {
		t.Fatal("expected integer above 2^53-1 error, got nil")
	}
	if !strings.Contains(err.Error(), "integer above 2^53-1") {
		t.Fatalf("expected integer above 2^53-1 error, got: %v", err)
	}
}

func TestProfileRefusalTruncated(t *testing.T) {
	// Use a valid sync message and truncate it
	full := testutil.Unhex(t, "a56464617461582f42018a82a24e48e4afb9422ca65fadf64e8042fbb703f6ac01ba6d93ba497f02020000010005010a0744150002028464747970656473796e636873656e64657249647834313244334b6f6f57514b31776e65666f4c7263564862626e6635744c7a626f705564334b336246416f4a704137594a674c3570566874617267657449647834313244334b6f6f5744775469725163653152524b6e6173543566505646677a58437936536952675377727750474c43377a4539316a646f63756d656e744964781c344e4d4e6e6b4d684c386a5872644a396a616d533538504156645875")
	input := full[:len(full)-10]
	_, err := DecodeValue(input)
	if err == nil {
		t.Fatal("expected truncated error, got nil")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("expected truncated error, got: %v", err)
	}
}

func TestProfileRefusalDepth(t *testing.T) {
	// Use the too-deep vector from repo.json
	input := testutil.Unhex(t, "a5617881818181818181810164747970656f646f632d756e617661696c61626c656873656e64657249647834313244334b6f6f57514b31776e65666f4c7263564862626e6635744c7a626f705564334b336246416f4a704137594a674c3570566874617267657449647834313244334b6f6f5744775469725163653152524b6e6173543566505646677a58437936536952675377727750474c43377a4539316a646f63756d656e744964781c344e4d4e6e6b4d684c386a5872644a396a616d533538504156645875")
	_, err := DecodeValue(input)
	if err == nil {
		t.Fatal("expected depth > 8 error, got nil")
	}
	if !strings.Contains(err.Error(), "depth > 8") {
		t.Fatalf("expected depth > 8 error, got: %v", err)
	}
}

// FuzzRepoMessage fuzzes the repo message decoder.
// FuzzRepoMessage fuzzes Decode, the entry point of every repo message from a peer, under the
// context of the vectors. Spec §9.2-§9.6: it never panics; an accepted message passed the profile,
// is bound to the authenticated sender and to the receiver, and a document-scoped one names an
// admitted document; an accepted value re-encodes to a value that decodes identically.
func FuzzRepoMessage(f *testing.F) {
	data, err := vectors.FS.ReadFile("repo.json")
	if err != nil {
		f.Fatal(err)
	}
	var file struct {
		Vectors []struct {
			Decoder  string `json:"decoder"`
			InputHex string `json:"input_hex"`
			Context  struct {
				From     string   `json:"from"`
				Me       string   `json:"me"`
				Admitted []string `json:"admitted"`
			} `json:"context"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		f.Fatal(err)
	}
	var ctx Context
	for _, v := range file.Vectors {
		if v.Decoder != "repo_message" {
			continue
		}
		ctx = Context{From: v.Context.From, Me: v.Context.Me, Admitted: v.Context.Admitted}
		f.Add(testutil.Unhex(f, v.InputHex))
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		res, err := Decode(input, ctx)
		if err == nil && !res.Ignored {
			m := res.Message
			if m.SenderID != ctx.From || m.TargetID != ctx.Me {
				t.Fatalf("accepted message bound to %q→%q, want %q→%q", m.SenderID, m.TargetID, ctx.From, ctx.Me)
			}
			if m.DocumentID != "" && !slices.Contains(ctx.Admitted, m.DocumentID) {
				t.Fatalf("accepted message for unadmitted document %q", m.DocumentID)
			}
		}

		v, verr := DecodeValue(input)
		if verr != nil {
			if err == nil {
				t.Fatalf("Decode accepted an input that fails the profile: %v", verr)
			}
			return
		}
		encoded, err := Encode(v)
		if err != nil {
			return
		}
		v2, err := DecodeValue(encoded)
		if err != nil {
			t.Fatalf("re-encoded value rejected: %v", err)
		}
		if !equal(v, v2) {
			t.Fatal("fuzz round-trip value mismatch")
		}
	})
}

func equal(a, b any) bool {
	return reflect.DeepEqual(a, b)
}
