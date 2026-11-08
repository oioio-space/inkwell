// SPDX-License-Identifier: MPL-2.0

// Package repo implements the CBOR validity profile (§9.2) and the
// automerge-repo message schemas (§9.3-§9.6) of the public protocol.
package repo

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"
)

const (
	maxDepth = 8
	maxItems = 4096
	maxMap   = 64
	maxArray = 256
	maxSafe  = 1<<53 - 1
)

var errTruncated = errors.New("truncated")

type decoder struct {
	b     []byte
	i     int
	items int
}

// DecodeValue decodes exactly one item under the validity profile. It accepts
// every length width the profile admits (§9.2.1), including non-preferred ones.
func DecodeValue(b []byte) (any, error) {
	d := &decoder{b: b}
	v, err := d.item(1)
	if err != nil {
		return nil, err
	}
	if d.i != len(b) {
		return nil, errors.New("trailing bytes")
	}
	return v, nil
}

func (d *decoder) arg(ai byte) (uint64, error) {
	if ai < 24 {
		return uint64(ai), nil
	}
	width := 0
	switch ai {
	case 24:
		width = 1
	case 25:
		width = 2
	case 26:
		width = 4
	case 27:
		width = 8
	default:
		if ai == 31 {
			return 0, errors.New("indefinite length")
		}
		return 0, errors.New("reserved additional info")
	}
	if d.i+width > len(d.b) {
		return 0, errTruncated
	}
	var v uint64
	for k := 0; k < width; k++ {
		v = v<<8 | uint64(d.b[d.i+k])
	}
	d.i += width
	if v > maxSafe {
		return 0, errors.New("integer above 2^53-1")
	}
	return v, nil
}

func (d *decoder) item(depth int) (any, error) {
	if depth > maxDepth {
		return nil, errors.New("depth > 8")
	}
	d.items++
	if d.items > maxItems {
		return nil, errors.New("more than 4096 items")
	}
	if d.i >= len(d.b) {
		return nil, errTruncated
	}
	initial := d.b[d.i]
	d.i++
	major, ai := initial>>5, initial&31
	switch major {
	case 0:
		return d.arg(ai)
	case 1:
		return nil, errors.New("negative integer")
	case 2:
		return d.itemBytes(ai)
	case 3:
		return d.itemString(ai)
	case 4:
		return d.itemArray(ai, depth)
	case 5:
		return d.itemMap(ai, depth)
	case 6:
		return nil, errors.New("tag")
	default:
		return d.itemSimple(initial, ai)
	}
}

func (d *decoder) itemBytes(ai byte) ([]byte, error) {
	n, err := d.arg(ai)
	if err != nil {
		return nil, err
	}
	rest := len(d.b) - d.i // never negative: d.i only advances within d.b
	if n > uint64(rest) {  // #nosec G115 -- rest >= 0
		return nil, errTruncated
	}
	size := int(n) // #nosec G115 -- n <= rest, which is an int
	raw := d.b[d.i : d.i+size]
	d.i += size
	return raw, nil
}

func (d *decoder) itemString(ai byte) (string, error) {
	raw, err := d.itemBytes(ai)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", errors.New("invalid UTF-8")
	}
	return string(raw), nil
}

func (d *decoder) itemArray(ai byte, depth int) ([]any, error) {
	n, err := d.arg(ai)
	if err != nil {
		return nil, err
	}
	if n > maxArray {
		return nil, errors.New("array > 256")
	}
	out := make([]any, 0, int(n))
	for range n {
		v, err := d.item(depth + 1)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (d *decoder) itemMap(ai byte, depth int) (map[string]any, error) {
	n, err := d.arg(ai)
	if err != nil {
		return nil, err
	}
	if n > maxMap {
		return nil, errors.New("map > 64 pairs")
	}
	out := make(map[string]any, int(n))
	for range n {
		if d.i >= len(d.b) {
			return nil, errTruncated
		}
		if d.b[d.i]>>5 != 3 {
			return nil, errors.New("map key not a text string")
		}
		key, err := d.item(depth + 1)
		if err != nil {
			return nil, err
		}
		k, ok := key.(string)
		if !ok {
			return nil, errors.New("map key not a text string")
		}
		if _, exists := out[k]; exists {
			return nil, errors.New("duplicate map key")
		}
		value, err := d.item(depth + 1)
		if err != nil {
			return nil, err
		}
		out[k] = value
	}
	return out, nil
}

func (d *decoder) itemSimple(initial, ai byte) (any, error) {
	switch initial {
	case 0xf4:
		return false, nil
	case 0xf5:
		return true, nil
	case 0xf6:
		return nil, nil
	}
	if ai >= 25 && ai <= 27 {
		return nil, errors.New("float")
	}
	return nil, errors.New("simple value other than false/true/null")
}

// Encode writes one value in the core deterministic form (RFC 8949 §4.2.1):
// minimal heads, map keys sorted by their encoded bytes, no floats or tags.
func Encode(v any) ([]byte, error) {
	return encode(nil, v)
}

func encode(out []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(out, 0xf6), nil
	case bool:
		if x {
			return append(out, 0xf5), nil
		}
		return append(out, 0xf4), nil
	case string:
		out = appendHead(out, 3, uint64(len(x)))
		return append(out, x...), nil
	case []byte:
		out = appendHead(out, 2, uint64(len(x)))
		return append(out, x...), nil
	case []any:
		out = appendHead(out, 4, uint64(len(x)))
		for _, item := range x {
			var err error
			out, err = encode(out, item)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	case map[string]any:
		type pair struct{ key, value []byte }
		pairs := make([]pair, 0, len(x))
		for k, value := range x {
			key, err := encode(nil, k)
			if err != nil {
				return nil, err
			}
			encoded, err := encode(nil, value)
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, pair{key, encoded})
		}
		sort.Slice(pairs, func(i, j int) bool { return bytes.Compare(pairs[i].key, pairs[j].key) < 0 })
		out = appendHead(out, 5, uint64(len(pairs)))
		for _, p := range pairs {
			out = append(out, p.key...)
			out = append(out, p.value...)
		}
		return out, nil
	case int:
		return appendUint(out, uint64(max(x, 0)), x >= 0)
	case int64:
		return appendUint(out, uint64(max(x, 0)), x >= 0)
	case int32:
		return appendUint(out, uint64(max(x, 0)), x >= 0)
	case uint:
		return appendUint(out, uint64(x), true)
	case uint64:
		return appendUint(out, x, true)
	case uint32:
		return appendUint(out, uint64(x), true)
	default:
		return nil, fmt.Errorf("cannot encode %T", v)
	}
}

func appendUint(out []byte, n uint64, ok bool) ([]byte, error) {
	if !ok {
		return nil, errors.New("unsigned integer only")
	}
	if n > maxSafe {
		return nil, errors.New("integer above 2^53-1")
	}
	return appendHead(out, 0, n), nil
}

func appendHead(out []byte, major byte, n uint64) []byte {
	switch {
	// Each case bounds n to its width; byte(...) deliberately keeps one big-endian byte (RFC 8949 §3).
	case n < 24:
		return append(out, major<<5|byte(n)) // #nosec G115 -- n < 24
	case n < 0x100:
		return append(out, major<<5|24, byte(n)) // #nosec G115 -- n < 2^8
	case n < 0x10000:
		return append(out, major<<5|25, byte(n>>8), byte(n)) // #nosec G115 -- n < 2^16, byte extraction
	case n < 0x100000000:
		return append(out, major<<5|26, byte(n>>24), byte(n>>16), byte(n>>8), byte(n)) // #nosec G115 -- n < 2^32, byte extraction
	default:
		out = append(out, major<<5|27)
		for k := 7; k >= 0; k-- {
			out = append(out, byte(n>>(8*k))) // #nosec G115 -- byte extraction of a uint64
		}
		return out
	}
}
