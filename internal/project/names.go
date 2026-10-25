package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Limits of portable names (ADR 0023).
const (
	// MaxNameLen is the longest name of one path component, in UTF-8 bytes.
	MaxNameLen = 255
	// MaxPathLen is the longest whole path, in UTF-8 bytes.
	MaxPathLen = 1024
	// MaxDepth is the deepest folder nesting of a project tree. Deeper nodes
	// are ignored by the tree, so [ValidatePath] does not check it.
	MaxDepth = 32
)

// Reason says why a name or a path was refused. The values are stable codes,
// shared with the TypeScript validator through testdata/names.json.
type Reason string

// The reasons.
const (
	// ReasonEmpty is an empty name or path.
	ReasonEmpty Reason = "empty"
	// ReasonTooLong is a name over [MaxNameLen] bytes.
	ReasonTooLong Reason = "too_long"
	// ReasonBadUTF8 is a name that is not valid UTF-8.
	ReasonBadUTF8 Reason = "bad_utf8"
	// ReasonBadChar is a name with a separator, a control character or one
	// of <>:"|?*.
	ReasonBadChar Reason = "bad_char"
	// ReasonDot is "." or "..".
	ReasonDot Reason = "dot"
	// ReasonTrailing is a name that ends with a dot or a space.
	ReasonTrailing Reason = "trailing"
	// ReasonDevice is a Windows device name, with or without extension.
	ReasonDevice Reason = "device"
	// ReasonRootReserved is a path that starts with .inkwell or .typstdeps.
	ReasonRootReserved Reason = "root_reserved"
	// ReasonEmptyComponent is a path with an empty component: a leading,
	// trailing or doubled slash.
	ReasonEmptyComponent Reason = "empty_component"
	// ReasonPathTooLong is a path over [MaxPathLen] bytes.
	ReasonPathTooLong Reason = "path_too_long"
)

// ErrInvalidName is wrapped by every [NameError].
var ErrInvalidName = errors.New("project: invalid name")

// NameError reports a refused name or path. Its message never repeats the
// offending text, which comes from a peer and may hold control sequences.
type NameError struct {
	// Reason says which rule the name broke.
	Reason Reason
	// Len is the length in bytes of the refused text.
	Len int
}

func (e *NameError) Error() string {
	return fmt.Sprintf("project: invalid name (%s, %d bytes)", e.Reason, e.Len)
}

// Is reports that a NameError is an [ErrInvalidName].
func (e *NameError) Is(target error) bool { return target == ErrInvalidName }

func refuse(r Reason, s string) error { return &NameError{Reason: r, Len: len(s)} }

// reservedAtRoot lists, folded, the names no user file may have at the root.
var reservedAtRoot = []string{".inkwell", ".typstdeps"}

// ValidateName checks one path component against the portable-name rules of
// ADR 0023, which hold on every platform Inkwell runs on:
//
//   - 1 to [MaxNameLen] bytes of valid UTF-8;
//   - none of / \ NUL, the C0 controls, DEL and <>:"|?*;
//   - not "." or "..", and not ending with a dot or a space;
//   - not a Windows device name (CON, PRN, AUX, NUL, CLOCK$, CONIN$, CONOUT$,
//     COM0 to COM9, LPT0 to LPT9, COM¹²³, LPT¹²³), whatever the case and
//     with any extension: a device name is compared on the part before the
//     first dot, in ASCII case, ignoring spaces before that dot.
//
// It returns a [*NameError] wrapping [ErrInvalidName], or nil. Whether a name
// is reserved at the root of the project is a matter of [ValidatePath].
func ValidateName(name string) error {
	switch {
	case name == "":
		return refuse(ReasonEmpty, name)
	case len(name) > MaxNameLen:
		return refuse(ReasonTooLong, name)
	case !utf8.ValidString(name):
		return refuse(ReasonBadUTF8, name)
	case strings.ContainsFunc(name, forbiddenRune):
		return refuse(ReasonBadChar, name)
	case name == "." || name == "..":
		return refuse(ReasonDot, name)
	case name[len(name)-1] == '.' || name[len(name)-1] == ' ':
		return refuse(ReasonTrailing, name)
	case isDevice(name):
		return refuse(ReasonDevice, name)
	}
	return nil
}

// forbiddenRune reports whether r may not appear in a name.
func forbiddenRune(r rune) bool {
	return r < ' ' || r == 0x7f || strings.ContainsRune(`/\<>:"|?*`, r)
}

// isDevice reports whether name designates a Windows device.
func isDevice(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	stem = asciiUpper(strings.TrimRight(stem, " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$":
		return true
	}
	port, ok := strings.CutPrefix(stem, "COM")
	if !ok {
		port, ok = strings.CutPrefix(stem, "LPT")
	}
	return ok && slices.Contains(devicePorts, port)
}

// devicePorts are the numbers a COM or LPT device name ends with.
var devicePorts = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "\u00b9", "\u00b2", "\u00b3"}

// asciiUpper upper-cases the ASCII letters of s and leaves every other
// character alone, which is how Windows tells a device name and what a
// validator in any language can reproduce exactly.
func asciiUpper(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}, s)
}

// ValidatePath checks a project-relative path: components separated by "/"
// (so no leading, trailing or doubled slash), each accepted by
// [ValidateName], at most [MaxPathLen] bytes in all, and not under a name
// that is reserved at the root, .inkwell or .typstdeps in any case. It
// returns a [*NameError] wrapping [ErrInvalidName], or nil.
func ValidatePath(p string) error {
	switch {
	case p == "":
		return refuse(ReasonEmpty, p)
	case len(p) > MaxPathLen:
		return refuse(ReasonPathTooLong, p)
	}
	for i, c := range strings.Split(p, "/") {
		if c == "" {
			return refuse(ReasonEmptyComponent, p)
		}
		if err := ValidateName(c); err != nil {
			return err
		}
		if i == 0 && slices.Contains(reservedAtRoot, CollisionKey(c)) {
			return refuse(ReasonRootReserved, p)
		}
	}
	return nil
}

// CollisionKey returns the key under which two sibling names collide: the
// name in NFC, case-folded with the full Unicode case folding, then in NFC
// again. "Notes.md" and "notes.md" share a key, and so do the composed and
// the decomposed "é" (macOS writes the latter), and "Straße" and "STRASSE".
// It is meant for valid names.
func CollisionKey(name string) string {
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(name)))
}
