// SPDX-License-Identifier: MPL-2.0

// Package vectors embeds the canonical, shared conformance corpus.
package vectors

import "embed"

// FS contains the JSON vector files, unchanged from the specification corpus.
//
//go:embed *.json
var FS embed.FS
