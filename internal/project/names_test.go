package project

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path"
	"strings"
	"testing"
	"unicode/utf8"
)

// nameVector is one case of testdata/names.json, shared with the TypeScript
// validator.
type nameVector struct {
	Name     string `json:"name"`
	Input    string `json:"input"`
	InputHex string `json:"inputHex"`
	Reason   string `json:"reason"`
	Valid    bool   `json:"valid"`
}

func (v nameVector) input(t *testing.T) string {
	t.Helper()
	if v.InputHex == "" {
		return v.Input
	}
	b, err := hex.DecodeString(v.InputHex)
	if err != nil {
		t.Fatalf("vector %q: bad inputHex: %v", v.Name, err)
	}
	return string(b)
}

type keyVector struct {
	Input string `json:"input"`
	Key   string `json:"key"`
}

func loadNameVectors(t *testing.T) (names, paths []nameVector, keys []keyVector) {
	t.Helper()
	raw, err := os.ReadFile("testdata/names.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Names []nameVector `json:"names"`
		Paths []nameVector `json:"paths"`
		Keys  []keyVector  `json:"keys"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f.Names, f.Paths, f.Keys
}

// checkVector compares the outcome of validate on v with the vector.
func checkVector(t *testing.T, v nameVector, validate func(string) error) {
	t.Helper()
	in := v.input(t)
	err := validate(in)
	if v.Valid {
		if err != nil {
			t.Errorf("validate(%q) = %v, want valid", in, err)
		}
		return
	}
	ne, ok := errors.AsType[*NameError](err)
	switch {
	case err == nil:
		t.Errorf("validate(%q) = nil, want reason %q", in, v.Reason)
	case !ok || !errors.Is(err, ErrInvalidName):
		t.Errorf("validate(%q) = %v, want a *NameError wrapping ErrInvalidName", in, err)
	case string(ne.Reason) != v.Reason:
		t.Errorf("validate(%q) reason = %q, want %q", in, ne.Reason, v.Reason)
	}
}

// PRJ-10, PRJ-15: the portable-name rules, on the vectors the TS side shares.
func TestValidateNameVectors(t *testing.T) {
	names, _, _ := loadNameVectors(t)
	for _, v := range names {
		t.Run("PRJ-15 "+v.Name, func(t *testing.T) { checkVector(t, v, ValidateName) })
	}
}

func TestValidatePathVectors(t *testing.T) {
	_, paths, _ := loadNameVectors(t)
	for _, v := range paths {
		t.Run("PRJ-10 "+v.Name, func(t *testing.T) { checkVector(t, v, ValidatePath) })
	}
}

// PRJ-17: names that differ only by case or by NFC/NFD share a key.
func TestCollisionKeyVectors(t *testing.T) {
	_, _, keys := loadNameVectors(t)
	for _, v := range keys {
		t.Run("PRJ-17 "+v.Input, func(t *testing.T) {
			if got := CollisionKey(v.Input); got != v.Key {
				t.Errorf("CollisionKey(%q) = %q, want %q", v.Input, got, v.Key)
			}
		})
	}
}

func TestCollisionKeyDistinguishes(t *testing.T) {
	for _, tc := range [][2]string{{"a.md", "b.md"}, {"notes.md", "notes.mdx"}, {"é", "e"}} {
		if CollisionKey(tc[0]) == CollisionKey(tc[1]) {
			t.Errorf("CollisionKey(%q) == CollisionKey(%q), want different", tc[0], tc[1])
		}
	}
}

func TestNameErrorHidesInput(t *testing.T) {
	err := ValidateName("evil\x00\x1b[31m<name>")
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.ContainsAny(err.Error(), "\x00\x1b<>") {
		t.Errorf("error %q repeats the hostile name", err)
	}
}

// FuzzValidateName: the validator never panics and what it accepts is safe
// to create under a root, on every platform.
func FuzzValidateName(f *testing.F) {
	for _, s := range []string{
		"", "a", ".", "..", "a/b", "a\\b", "CON", "con.txt", "COM¹", "a.", "a ", "é", "é",
		"\xff", "a\x00b", ".inkwell", strings.Repeat("a", 255), strings.Repeat("é", 128), "C:",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		err := ValidateName(name)
		if err != nil {
			if !errors.Is(err, ErrInvalidName) {
				t.Fatalf("ValidateName(%q) = %v, want ErrInvalidName", name, err)
			}
			if perr := ValidatePath(name); perr == nil && !strings.Contains(name, "/") {
				t.Fatalf("ValidatePath(%q) accepts a single component ValidateName refuses", name)
			}
			return
		}
		if !utf8.ValidString(name) || len(name) == 0 || len(name) > MaxNameLen {
			t.Fatalf("ValidateName(%q) accepted a bad length or encoding", name)
		}
		if strings.ContainsAny(name, "/\\<>:\"|?*\x00\x7f") || strings.ContainsFunc(name, func(r rune) bool { return r < ' ' }) {
			t.Fatalf("ValidateName(%q) accepted a forbidden character", name)
		}
		if name == "." || name == ".." || path.Clean(name) != name || path.Base(name) != name {
			t.Fatalf("ValidateName(%q) accepted a name that is not a plain component", name)
		}
		if last := name[len(name)-1]; last == '.' || last == ' ' {
			t.Fatalf("ValidateName(%q) accepted a trailing dot or space", name)
		}
		key := CollisionKey(name)
		if CollisionKey(key) != key {
			t.Fatalf("CollisionKey is not idempotent on %q: %q then %q", name, key, CollisionKey(key))
		}
		// A valid name is a valid one-component path, except where it is reserved at the root.
		if perr := ValidatePath(name); perr != nil {
			if ne, ok := errors.AsType[*NameError](perr); !ok || ne.Reason != ReasonRootReserved {
				t.Fatalf("ValidatePath(%q) = %v after ValidateName accepted it", name, perr)
			}
		}
	})
}

// FuzzValidatePath: a valid path has only valid components and fits the limits.
func FuzzValidatePath(f *testing.F) {
	for _, s := range []string{"", "a/b", "../a", "/a", "a//b", ".inkwell/x", "a/NUL", strings.Repeat("a/", 600)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if ValidatePath(p) != nil {
			return
		}
		if len(p) > MaxPathLen {
			t.Fatalf("ValidatePath(%q) accepted %d bytes", p, len(p))
		}
		for c := range strings.SplitSeq(p, "/") {
			if err := ValidateName(c); err != nil {
				t.Fatalf("ValidatePath(%q) accepted the component %q: %v", p, c, err)
			}
		}
		if path.Clean(p) != p {
			t.Fatalf("ValidatePath(%q) accepted a path that is not clean", p)
		}
	})
}
